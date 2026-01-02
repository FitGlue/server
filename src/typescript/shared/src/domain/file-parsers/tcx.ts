import { XMLParser } from 'fast-xml-parser';
import { StandardizedActivity, Lap, Record, Session } from '../../types/pb/standardized_activity';

// Helper to safely get array even if XML parser returns single object or undefined
const asArray = (val: any) => {
  if (!val) return [];
  return Array.isArray(val) ? val : [val];
};

export const mapTCXToStandardized = (tcxXml: string, logData: any, userId: string, source: string): StandardizedActivity => {
  const parser = new XMLParser({
    ignoreAttributes: false,
    attributeNamePrefix: "@_"
  });
  const parsed = parser.parse(tcxXml);

  const activity = parsed?.TrainingCenterDatabase?.Activities?.Activity;
  if (!activity) {
    throw new Error('Invalid TCX: Missing Activity');
  }

  const tcxId = activity.Id;
  const sport = activity['@_Sport']; // e.g. Biking, Running

  const generatedLaps: Lap[] = [];
  let totalDistanceToCheck = 0;
  let totalElapsedTime = 0;

  const tcxLaps = asArray(activity.Lap);

  tcxLaps.forEach((tcxLap: any) => {
    const records: Record[] = [];
    const track = tcxLap.Track;

    if (track) {
      const trackpoints = asArray(track.Trackpoint);
      trackpoints.forEach((tp: any) => {
        const record: Record = {
          timestamp: new Date(tp.Time),
          // GPS
          positionLat: tp.Position?.LatitudeDegrees ? parseFloat(tp.Position.LatitudeDegrees) : 0,
          positionLong: tp.Position?.LongitudeDegrees ? parseFloat(tp.Position.LongitudeDegrees) : 0,
          // Metrics
          altitude: tp.AltitudeMeters ? parseFloat(tp.AltitudeMeters) : 0,
          // In TCX, DistanceMeters is cumulative
          // We don't have a "distance" field in Record (only altitude, speed etc in simple view)
          // But looking at proto: Record has "speed" (m/s), "cadence", "heartRate", "power"
          // It does not explicitly have "distance" per point, usually calculated or implied.
          // Wait, looking at proto definition I saw earlier:
          /*
          export interface Record {
            timestamp: string;
            heartRate: number;
            power: number;
            cadence: number;
            speed: number;
            altitude: number;
            positionLat: number;
            positionLong: number;
          }
          */
          // TCX "DistanceMeters" at a point is valid, but our Record proto doesn't seem to store it?
          // Actually, let's double check proto. I see `altitude`, `speed`.
          // We can derive speed if needed, or if TCX has extensions.
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
      records: records
    };

    totalDistanceToCheck += lap.totalDistance;
    totalElapsedTime += lap.totalElapsedTime;
    generatedLaps.push(lap);
  });

  // Use Log Data for high-level metadata if available
  // note: logData.duration is usually milliseconds
  const durationSeconds = logData?.duration ? logData.duration / 1000 : totalElapsedTime;

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
    externalId: logData?.logId?.toString() || tcxId,
    userId: userId,
    startTime: new Date(tcxId), // TCX Id is usually ISO timestamp
    name: logData?.activityName || `Fitbit ${sport} Activity`,
    type: sport.toUpperCase(), // e.g. BIKING, RUNNING
    description: logData?.description || '',
    sessions: [session],
    tags: [],
    notes: ''
  };

  return standardized;
};
