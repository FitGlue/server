package fit_parser

import (
	"bytes"
	"fmt"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// FIT message order: FileId -> DeviceInfo -> Records -> Lap -> Session -> Activity
// Records come BEFORE Lap/Session summaries, so we need to collect everything first
// and then organize into the proper hierarchy.

// ParseFitFile parses a FIT file and returns a StandardizedActivity.
// If the file contains multiple sessions, they are merged into a single session.
func ParseFitFile(data []byte) (*pb.StandardizedActivity, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty FIT data")
	}

	fitDec := decoder.New(bytes.NewReader(data))

	// Collect all data first, then organize
	var allRecords []*pb.Record
	var lapInfos []lapInfo
	var sessionInfos []sessionInfo

	var activityType pb.ActivityType
	var activityName string
	var startTime time.Time

	for fitDec.Next() {
		fitData, err := fitDec.Decode()
		if err != nil {
			return nil, fmt.Errorf("failed to decode FIT file: %w", err)
		}

		for _, msg := range fitData.Messages {
			switch msg.Num {
			case typedef.MesgNumFileId:
				fileId := mesgdef.NewFileId(&msg)
				if startTime.IsZero() && !fileId.TimeCreated.IsZero() {
					startTime = fileId.TimeCreated.UTC()
				}

			case typedef.MesgNumRecord:
				record := parseRecord(&msg)
				if record != nil {
					allRecords = append(allRecords, record)
					if startTime.IsZero() && record.Timestamp != nil {
						startTime = record.Timestamp.AsTime()
					}
				}

			case typedef.MesgNumLap:
				lapMsg := mesgdef.NewLap(&msg)
				lapInfos = append(lapInfos, lapInfo{
					startTime:        lapMsg.StartTime.UTC(),
					totalElapsedTime: float64(lapMsg.TotalElapsedTime) / 1000,
					totalDistance:    float64(lapMsg.TotalDistance) / 100,
				})

			case typedef.MesgNumSession:
				sessionMsg := mesgdef.NewSession(&msg)
				sessionInfos = append(sessionInfos, sessionInfo{
					startTime:        sessionMsg.StartTime.UTC(),
					totalElapsedTime: float64(sessionMsg.TotalElapsedTime) / 1000,
					totalDistance:    float64(sessionMsg.TotalDistance) / 100,
					sport:            sessionMsg.Sport,
					subSport:         sessionMsg.SubSport,
					sportProfileName: sessionMsg.SportProfileName,
				})

				// Set activity type from first session
				if activityType == pb.ActivityType_ACTIVITY_TYPE_UNSPECIFIED {
					activityType = mapFitSportToActivityType(sessionMsg.Sport, sessionMsg.SubSport)
				}
				if activityName == "" && sessionMsg.SportProfileName != "" {
					activityName = sessionMsg.SportProfileName
				}
			}
		}
	}

	// Build sessions from collected data
	sessions := buildSessions(allRecords, lapInfos, sessionInfos)

	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions found in FIT file")
	}

	// Merge multiple sessions into one
	var mergedSession *pb.Session
	if len(sessions) > 1 {
		mergedSession = MergeSessions(sessions)
	} else {
		mergedSession = sessions[0]
	}

	// Generate activity name if not set
	if activityName == "" {
		activityName = generateActivityName(activityType, startTime)
	}

	activity := &pb.StandardizedActivity{
		Source:    "SOURCE_FILE_UPLOAD",
		StartTime: timestamppb.New(startTime),
		Name:      activityName,
		Type:      activityType,
		Sessions:  []*pb.Session{mergedSession},
	}

	return activity, nil
}

type lapInfo struct {
	startTime        time.Time
	totalElapsedTime float64
	totalDistance    float64
}

type sessionInfo struct {
	startTime        time.Time
	totalElapsedTime float64
	totalDistance    float64
	sport            typedef.Sport
	subSport         typedef.SubSport
	sportProfileName string
}

// buildSessions organizes records into laps and sessions based on timestamps
func buildSessions(records []*pb.Record, lapInfos []lapInfo, sessionInfos []sessionInfo) []*pb.Session {
	if len(records) == 0 {
		return nil
	}

	// If no session info, create a synthetic session with all records
	if len(sessionInfos) == 0 {
		var duration float64
		if len(records) > 1 {
			first := records[0].Timestamp.AsTime()
			last := records[len(records)-1].Timestamp.AsTime()
			duration = last.Sub(first).Seconds()
		}

		lap := &pb.Lap{
			StartTime:        records[0].Timestamp,
			TotalElapsedTime: duration,
			Records:          records,
		}

		return []*pb.Session{{
			StartTime:        records[0].Timestamp,
			TotalElapsedTime: duration,
			Laps:             []*pb.Lap{lap},
		}}
	}

	// If no lap info, create one lap per session containing all records
	if len(lapInfos) == 0 {
		session := &sessionInfos[0]
		lap := &pb.Lap{
			StartTime:        timestamppb.New(session.startTime),
			TotalElapsedTime: session.totalElapsedTime,
			TotalDistance:    session.totalDistance,
			Records:          records,
		}

		return []*pb.Session{{
			StartTime:        timestamppb.New(session.startTime),
			TotalElapsedTime: session.totalElapsedTime,
			TotalDistance:    session.totalDistance,
			Laps:             []*pb.Lap{lap},
		}}
	}

	// Assign records to laps based on timestamps
	laps := make([]*pb.Lap, len(lapInfos))
	for i, li := range lapInfos {
		laps[i] = &pb.Lap{
			StartTime:        timestamppb.New(li.startTime),
			TotalElapsedTime: li.totalElapsedTime,
			TotalDistance:    li.totalDistance,
			Records:          []*pb.Record{},
		}
	}

	// Assign each record to the appropriate lap
	for _, record := range records {
		rt := record.Timestamp.AsTime()
		assigned := false

		for i := len(lapInfos) - 1; i >= 0; i-- {
			lapStart := lapInfos[i].startTime
			lapEnd := lapStart.Add(time.Duration(lapInfos[i].totalElapsedTime) * time.Second)

			// Record belongs to this lap if it falls within its time range
			if (rt.Equal(lapStart) || rt.After(lapStart)) && (rt.Before(lapEnd) || rt.Equal(lapEnd)) {
				laps[i].Records = append(laps[i].Records, record)
				assigned = true
				break
			}
		}

		// If not assigned (edge case), put in first lap that starts before this record
		if !assigned {
			for i := len(lapInfos) - 1; i >= 0; i-- {
				if !rt.Before(lapInfos[i].startTime) {
					laps[i].Records = append(laps[i].Records, record)
					assigned = true
					break
				}
			}
			// Last resort: first lap
			if !assigned && len(laps) > 0 {
				laps[0].Records = append(laps[0].Records, record)
			}
		}
	}

	// Build sessions with laps
	sessions := make([]*pb.Session, len(sessionInfos))
	for i, si := range sessionInfos {
		sessions[i] = &pb.Session{
			StartTime:        timestamppb.New(si.startTime),
			TotalElapsedTime: si.totalElapsedTime,
			TotalDistance:    si.totalDistance,
			Laps:             []*pb.Lap{},
		}
	}

	// Assign laps to sessions based on lap start time
	for _, lap := range laps {
		lapStart := lap.StartTime.AsTime()
		assigned := false

		for i := len(sessionInfos) - 1; i >= 0; i-- {
			sessionStart := sessionInfos[i].startTime
			sessionEnd := sessionStart.Add(time.Duration(sessionInfos[i].totalElapsedTime) * time.Second)

			if (lapStart.Equal(sessionStart) || lapStart.After(sessionStart)) &&
				(lapStart.Before(sessionEnd) || lapStart.Equal(sessionEnd)) {
				sessions[i].Laps = append(sessions[i].Laps, lap)
				assigned = true
				break
			}
		}

		// Fallback: assign to last session that starts before this lap
		if !assigned {
			for i := len(sessionInfos) - 1; i >= 0; i-- {
				if !lapStart.Before(sessionInfos[i].startTime) {
					sessions[i].Laps = append(sessions[i].Laps, lap)
					assigned = true
					break
				}
			}
			if !assigned && len(sessions) > 0 {
				sessions[0].Laps = append(sessions[0].Laps, lap)
			}
		}
	}

	return sessions
}

// parseRecord extracts record data from a FIT message
func parseRecord(msg *proto.Message) *pb.Record {
	recordMsg := mesgdef.NewRecord(msg)

	ts := recordMsg.Timestamp
	if ts.IsZero() {
		return nil
	}

	record := &pb.Record{
		Timestamp: timestamppb.New(ts.UTC()),
	}

	// Heart rate
	if recordMsg.HeartRate != 0xFF { // 0xFF is invalid
		record.HeartRate = int32(recordMsg.HeartRate)
	}

	// Power
	if recordMsg.Power != 0xFFFF { // 0xFFFF is invalid
		record.Power = int32(recordMsg.Power)
	}

	// Cadence
	if recordMsg.Cadence != 0xFF {
		record.Cadence = int32(recordMsg.Cadence)
	}

	// Speed (FIT uses mm/s, we want m/s)
	if recordMsg.Speed != 0xFFFF {
		record.Speed = float64(recordMsg.Speed) / 1000
	}

	// Altitude (FIT uses 5 * (altitude + 500) scale)
	if recordMsg.Altitude != 0xFFFF {
		record.Altitude = (float64(recordMsg.Altitude) / 5) - 500
	}

	// Position (FIT uses semicircles, convert to decimal degrees)
	if recordMsg.PositionLat != 0x7FFFFFFF && recordMsg.PositionLong != 0x7FFFFFFF {
		const semicircleConst = 11930464.7111 // 2^31 / 180
		record.PositionLat = float64(recordMsg.PositionLat) / semicircleConst
		record.PositionLong = float64(recordMsg.PositionLong) / semicircleConst
	}

	return record
}

// MergeSessions merges multiple sessions into a single session.
// This is useful for FIT files that contain multiple sessions from device auto-pause.
func MergeSessions(sessions []*pb.Session) *pb.Session {
	if len(sessions) == 0 {
		return nil
	}
	if len(sessions) == 1 {
		return sessions[0]
	}

	merged := &pb.Session{
		StartTime:        sessions[0].StartTime,
		TotalElapsedTime: 0,
		TotalDistance:    0,
		Laps:             make([]*pb.Lap, 0),
		StrengthSets:     make([]*pb.StrengthSet, 0),
	}

	for _, session := range sessions {
		merged.TotalElapsedTime += session.TotalElapsedTime
		merged.TotalDistance += session.TotalDistance
		merged.Laps = append(merged.Laps, session.Laps...)
		merged.StrengthSets = append(merged.StrengthSets, session.StrengthSets...)
	}

	return merged
}

// mapFitSportToActivityType converts FIT SDK sport types to our ActivityType enum
func mapFitSportToActivityType(sport typedef.Sport, subSport typedef.SubSport) pb.ActivityType {
	switch sport {
	case typedef.SportRunning:
		switch subSport {
		case typedef.SubSportTrail:
			return pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN
		case typedef.SubSportVirtualActivity:
			return pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN
		default:
			return pb.ActivityType_ACTIVITY_TYPE_RUN
		}

	case typedef.SportCycling:
		switch subSport {
		case typedef.SubSportVirtualActivity:
			return pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE
		case typedef.SubSportMountain:
			return pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE
		case typedef.SubSportGravelCycling:
			return pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE
		case typedef.SubSportEBikeMountain:
			return pb.ActivityType_ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE
		case typedef.SubSportEBikeFitness:
			return pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE
		default:
			return pb.ActivityType_ACTIVITY_TYPE_RIDE
		}

	case typedef.SportSwimming:
		return pb.ActivityType_ACTIVITY_TYPE_SWIM

	case typedef.SportWalking:
		return pb.ActivityType_ACTIVITY_TYPE_WALK

	case typedef.SportHiking:
		return pb.ActivityType_ACTIVITY_TYPE_HIKE

	case typedef.SportTraining:
		switch subSport {
		case typedef.SubSportStrengthTraining:
			return pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING
		case typedef.SubSportYoga:
			return pb.ActivityType_ACTIVITY_TYPE_YOGA
		case typedef.SubSportHiit:
			return pb.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING
		case typedef.SubSportPilates:
			return pb.ActivityType_ACTIVITY_TYPE_PILATES
		case typedef.SubSportElliptical:
			return pb.ActivityType_ACTIVITY_TYPE_ELLIPTICAL
		case typedef.SubSportStairClimbing:
			return pb.ActivityType_ACTIVITY_TYPE_STAIR_STEPPER
		default:
			return pb.ActivityType_ACTIVITY_TYPE_WORKOUT
		}

	case typedef.SportRowing:
		return pb.ActivityType_ACTIVITY_TYPE_ROWING

	case typedef.SportAlpineSkiing:
		return pb.ActivityType_ACTIVITY_TYPE_ALPINE_SKI

	case typedef.SportCrossCountrySkiing:
		return pb.ActivityType_ACTIVITY_TYPE_NORDIC_SKI

	case typedef.SportSnowboarding:
		return pb.ActivityType_ACTIVITY_TYPE_SNOWBOARD

	case typedef.SportSoccer:
		return pb.ActivityType_ACTIVITY_TYPE_SOCCER

	case typedef.SportTennis:
		return pb.ActivityType_ACTIVITY_TYPE_TENNIS

	case typedef.SportGolf:
		return pb.ActivityType_ACTIVITY_TYPE_GOLF

	case typedef.SportPaddling:
		return pb.ActivityType_ACTIVITY_TYPE_KAYAKING

	case typedef.SportStandUpPaddleboarding:
		return pb.ActivityType_ACTIVITY_TYPE_STAND_UP_PADDLING

	case typedef.SportSurfing:
		return pb.ActivityType_ACTIVITY_TYPE_SURFING

	case typedef.SportSailing:
		return pb.ActivityType_ACTIVITY_TYPE_SAIL

	case typedef.SportIceSkating:
		return pb.ActivityType_ACTIVITY_TYPE_ICE_SKATE

	case typedef.SportInlineSkating:
		return pb.ActivityType_ACTIVITY_TYPE_INLINE_SKATE

	case typedef.SportRockClimbing:
		return pb.ActivityType_ACTIVITY_TYPE_ROCK_CLIMBING

	default:
		return pb.ActivityType_ACTIVITY_TYPE_WORKOUT
	}
}

// generateActivityName creates a default activity name based on type and time
func generateActivityName(activityType pb.ActivityType, startTime time.Time) string {
	hour := startTime.Hour()
	var timeOfDay string
	switch {
	case hour < 12:
		timeOfDay = "Morning"
	case hour < 17:
		timeOfDay = "Afternoon"
	case hour < 21:
		timeOfDay = "Evening"
	default:
		timeOfDay = "Night"
	}

	var activityName string
	switch activityType {
	case pb.ActivityType_ACTIVITY_TYPE_RUN, pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN, pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN:
		activityName = "Run"
	case pb.ActivityType_ACTIVITY_TYPE_RIDE, pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE, pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE, pb.ActivityType_ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE, pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE:
		activityName = "Ride"
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		activityName = "Swim"
	case pb.ActivityType_ACTIVITY_TYPE_WALK:
		activityName = "Walk"
	case pb.ActivityType_ACTIVITY_TYPE_HIKE:
		activityName = "Hike"
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		activityName = "Workout"
	case pb.ActivityType_ACTIVITY_TYPE_YOGA:
		activityName = "Yoga"
	default:
		activityName = "Activity"
	}

	return fmt.Sprintf("%s %s", timeOfDay, activityName)
}
