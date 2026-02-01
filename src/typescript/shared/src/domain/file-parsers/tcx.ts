import { XMLParser } from 'fast-xml-parser';
import { StandardizedActivity, Lap, Record, Session, ActivityType } from '../../types/pb/standardized_activity';

// Helper to safely get array even if XML parser returns single object or undefined
const asArray = (val: unknown) => {
  if (!val) return [];
  return Array.isArray(val) ? val : [val];
};

// Map TCX Sport_t (spec: Running | Biking | Other) to ActivityType
// This is provider-agnostic and follows the Garmin TCX specification.
function mapTcxSportToActivityType(sport: string): ActivityType {
  switch (sport?.toLowerCase()) {
    case 'running':
      return ActivityType.ACTIVITY_TYPE_RUN;
    case 'biking':
      return ActivityType.ACTIVITY_TYPE_RIDE;
    default:
      return ActivityType.ACTIVITY_TYPE_WORKOUT; // "Other" or unknown
  }
}

// TCX Interfaces
interface TcxTrackpoint {
  Time: string;
  Position?: {
    LatitudeDegrees?: string;
    LongitudeDegrees?: string;
  };
  AltitudeMeters?: string;
  HeartRateBpm?: { Value?: string };
  Cadence?: string;
  Extensions?: {
    TPX?: {
      Speed?: string;
      Watts?: string;
    };
  };
}

interface TcxTrack {
  Trackpoint?: TcxTrackpoint | TcxTrackpoint[];
}

interface TcxLap {
  Track?: TcxTrack;
  '@_StartTime': string;
  TotalTimeSeconds?: string;
  DistanceMeters?: string;
}

interface TcxActivity {
  Id: string;
  '@_Sport'?: string;
  Lap?: TcxLap | TcxLap[];
}

interface TcxDatabase {
  TrainingCenterDatabase?: {
    Activities?: {
      Activity?: TcxActivity;
    };
  };
}

interface LogData {
  duration?: number;
  logId?: string | number;
  activityName?: string;
  description?: string;
}

export const mapTCXToStandardized = (tcxXml: string, logData: LogData, userId: string, source: string): StandardizedActivity => {
  const data = logData as LogData;
  const parser = new XMLParser({
    ignoreAttributes: false,
    attributeNamePrefix: '@_'
  });
  const parsed = parser.parse(tcxXml) as TcxDatabase;

  const activity = parsed?.TrainingCenterDatabase?.Activities?.Activity;
  if (!activity) {
    throw new Error('Invalid TCX: Missing Activity');
  }

  const tcxId = activity.Id;
  const sport = activity['@_Sport']; // e.g. Biking, Running, Other

  const generatedLaps: Lap[] = [];
  let totalDistanceToCheck = 0;
  let totalElapsedTime = 0;

  const tcxLaps = asArray(activity.Lap);

  tcxLaps.forEach((tcxLap: TcxLap) => {
    const records: Record[] = [];
    const track = tcxLap.Track;

    if (track) {
      const trackpoints = asArray(track.Trackpoint);
      trackpoints.forEach((tp: TcxTrackpoint) => {
        const record: Record = {
          timestamp: new Date(tp.Time),
          // GPS
          positionLat: tp.Position?.LatitudeDegrees ? parseFloat(tp.Position.LatitudeDegrees) : 0,
          positionLong: tp.Position?.LongitudeDegrees ? parseFloat(tp.Position.LongitudeDegrees) : 0,
          // Metrics
          altitude: tp.AltitudeMeters ? parseFloat(tp.AltitudeMeters) : 0,
          speed: 0, // TCX standard doesn't always have speed in TP, often in extensions
          heartRate: tp.HeartRateBpm?.Value ? parseInt(tp.HeartRateBpm.Value) : 0,
          cadence: tp.Cadence ? parseInt(tp.Cadence) : 0,
          power: 0 // Watts often in extensions
        };

        // Parse Extensions for Speed/Watts if available (TPX)
        if (tp.Extensions?.TPX) {
          if (tp.Extensions.TPX.Speed) record.speed = parseFloat(tp.Extensions.TPX.Speed);
          if (tp.Extensions.TPX.Watts) record.power = parseFloat(tp.Extensions.TPX.Watts);
        }

        records.push(record);
      });
    }

    const lap: Lap = {
      startTime: new Date(tcxLap['@_StartTime']),
      totalElapsedTime: parseFloat(tcxLap.TotalTimeSeconds || '0'),
      totalDistance: parseFloat(tcxLap.DistanceMeters || '0'),
      records: records,
      exerciseName: '' // Default empty - can be set by hybrid race tagger
    };

    totalDistanceToCheck += lap.totalDistance;
    totalElapsedTime += lap.totalElapsedTime;
    generatedLaps.push(lap);
  });

  // Use Log Data for high-level metadata if available
  // note: logData.duration is usually milliseconds
  const durationSeconds = data?.duration ? data.duration / 1000 : totalElapsedTime;

  // Create Session (Fitbit activities usually are single session)
  const session: Session = {
    startTime: new Date(tcxId),
    totalElapsedTime: durationSeconds,
    totalDistance: totalDistanceToCheck,
    laps: generatedLaps,
    strengthSets: [] // TCX doesn't have strength sets
  };

  // FitGlue Standardized Activity
  const standardized: StandardizedActivity = {
    source,
    externalId: data?.logId?.toString() || tcxId,
    userId: userId,
    startTime: new Date(tcxId), // TCX Id is usually ISO timestamp
    name: data?.activityName || `${sport || 'Unknown'} Activity`,
    type: mapTcxSportToActivityType(sport || ''),
    description: data?.description || '',
    sessions: [session],
    timeMarkers: [],
    tags: [],
    notes: ''
  };

  return standardized;
};
