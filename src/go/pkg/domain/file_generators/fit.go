package file_generators

import (
	"bytes"
	"fmt"
	"log/slog"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// GenerateFitFile creates a FIT file from StandardizedActivity
// Supports multiple sport types and rich record data
func GenerateFitFile(activity *pb.StandardizedActivity) ([]byte, error) {
	if activity == nil {
		return nil, fmt.Errorf("activity cannot be nil")
	}

	if len(activity.Sessions) == 0 {
		return nil, fmt.Errorf("activity must have at least one session")
	}

	// Parse start time
	startTime := activity.StartTime.AsTime()
	if startTime.IsZero() {
		return nil, fmt.Errorf("invalid start time: zero")
	}

	// Strict Single Session Enforcement
	session := activity.Sessions[0]

	// Create proto.FIT struct
	fit := &proto.FIT{
		Messages: []proto.Message{},
	}

	// 1. FileId message
	fileId := mesgdef.NewFileId(nil).
		SetType(typedef.FileActivity).
		SetManufacturer(typedef.ManufacturerDevelopment).
		SetProduct(1). // FitGlue product ID
		SetTimeCreated(startTime)
	fit.Messages = append(fit.Messages, fileId.ToMesg(nil))

	// Map Sport
	sport, subSport := mapSport(activity.Type)

	// 2. Activity message (Appended last)
	activityMsg := mesgdef.NewActivity(nil).
		SetTimestamp(startTime).
		SetType(typedef.ActivityManual).
		SetNumSessions(1)

	// 3a. DeviceInfo: Source App (e.g. Hevy)
	manuf, product := mapSourceToDevice(activity.Source)
	sourceDeviceMsg := mesgdef.NewDeviceInfo(nil).
		SetTimestamp(startTime).
		SetManufacturer(manuf).
		SetProduct(0).
		SetProductName(product).
		SetDeviceIndex(0) // Primary device
	fit.Messages = append(fit.Messages, sourceDeviceMsg.ToMesg(nil))

	// 3b. DeviceInfo: FitGlue (Enricher/Aggregator)
	fitGlueDeviceMsg := mesgdef.NewDeviceInfo(nil).
		SetTimestamp(startTime).
		SetManufacturer(typedef.ManufacturerDevelopment).
		SetProduct(1).
		SetProductName("FitGlue").
		SetDeviceIndex(1) // Secondary device
	fit.Messages = append(fit.Messages, fitGlueDeviceMsg.ToMesg(nil))

	// 4. Session message (Appended last)
	sessionMsg := mesgdef.NewSession(nil).
		SetTimestamp(startTime).
		SetSport(sport).
		SetSubSport(subSport).
		SetStartTime(startTime)

	if session.TotalElapsedTime > 0 {
		sessionMsg.SetTotalElapsedTime(uint32(session.TotalElapsedTime * 1000))
		sessionMsg.SetTotalTimerTime(uint32(session.TotalElapsedTime * 1000))
	}
	if session.TotalDistance > 0 {
		sessionMsg.SetTotalDistance(uint32(session.TotalDistance * 100)) // cm? No, FIT uses meters usually but scaled?
		// meters, Type: uint32, Scale: 100, Offset: 0, Units: m
		sessionMsg.SetTotalDistance(uint32(session.TotalDistance * 100))
	}

	// 5. Lap message (One per session for now)
	lapMsg := mesgdef.NewLap(nil).
		SetTimestamp(startTime).
		SetStartTime(startTime).
		SetSport(sport).
		SetSubSport(subSport).
		SetMessageIndex(0)

	if session.TotalElapsedTime > 0 {
		lapMsg.SetTotalElapsedTime(uint32(session.TotalElapsedTime * 1000))
		lapMsg.SetTotalTimerTime(uint32(session.TotalElapsedTime * 1000))
	}
	if session.TotalDistance > 0 {
		lapMsg.SetTotalDistance(uint32(session.TotalDistance * 100))
	}

	// 6. Records
	// We iterate through laps in the session (though we only created one Lap msg above,
	// ideally we'd map session.Laps to FIT Laps, but enforcing single Lap for robust uploads first)
	// We'll flatten all records from all laps into this single FIT Lap/Session for safety.

	recordCount := 0
	for _, lap := range session.Laps {
		for _, record := range lap.Records {
			ts := record.Timestamp.AsTime()
			if ts.IsZero() {
				slog.Warn("Skipping record with invalid timestamp", "timestamp", record.Timestamp)
				continue // Skip invalid records
			}

			recordMsg := mesgdef.NewRecord(nil).SetTimestamp(ts)

			if record.HeartRate > 0 {
				recordMsg.SetHeartRate(uint8(record.HeartRate))
			}
			if record.Power > 0 {
				recordMsg.SetPower(uint16(record.Power))
			}
			if record.Cadence > 0 {
				recordMsg.SetCadence(uint8(record.Cadence))
			}
			if record.Speed > 0 {
				recordMsg.SetSpeed(uint16(record.Speed * 1000)) // m/s, scale 1000
			}
			if record.Altitude != 0 {
				// Altitude: scale 5, offset 500
				// Using SetAltitude which takes uint16 (scaled)
				// Formula: scaled = (altitude + 500) * 5
				alt := (record.Altitude + 500) * 5
				if alt >= 0 {
					recordMsg.SetAltitude(uint16(alt))
				}
			}

			// Location (Semicircles)
			// lat * (2^31 / 180)
			if record.PositionLat != 0 || record.PositionLong != 0 {
				const semicircleConst = 11930464.7111 // 2^31 / 180
				lat := int32(record.PositionLat * semicircleConst)
				long := int32(record.PositionLong * semicircleConst)
				recordMsg.SetPositionLat(lat)
				recordMsg.SetPositionLong(long)
			}

			if recordCount == 0 {
				// Start Lat/Long for Lap/Session
				if record.PositionLat != 0 || record.PositionLong != 0 {
					const semicircleConst = 11930464.7111
					lat := int32(record.PositionLat * semicircleConst)
					long := int32(record.PositionLong * semicircleConst)
					lapMsg.SetStartPositionLat(lat)
					lapMsg.SetStartPositionLong(long)
					sessionMsg.SetStartPositionLat(lat)
					sessionMsg.SetStartPositionLong(long)
				}
			}

			fit.Messages = append(fit.Messages, recordMsg.ToMesg(nil))
			recordCount++
		}
	}

	// Fallback: Synthesize records if none exist
	if recordCount == 0 && session.TotalElapsedTime > 0 {
		duration := int(session.TotalElapsedTime)
		for i := 0; i < duration; i++ {
			ts := startTime.Add(time.Duration(i) * time.Second)
			recordMsg := mesgdef.NewRecord(nil).SetTimestamp(ts)
			fit.Messages = append(fit.Messages, recordMsg.ToMesg(nil))
		}
	}

	// 7. Strength Sets (Only for training)
	if sport == typedef.SportTraining {
		for i, set := range session.StrengthSets {
			setStartTime := startTime
			if set.StartTime != nil {
				setStartTime = set.StartTime.AsTime()
			}

			category := MapExerciseToCategory(set.ExerciseName)
			setMsg := mesgdef.NewSet(nil).
				SetTimestamp(setStartTime).
				SetStartTime(setStartTime).
				SetCategory([]typedef.ExerciseCategory{category}).
				SetSetType(typedef.SetTypeActive).
				SetMessageIndex(typedef.MessageIndex(i))

			if set.Reps > 0 {
				setMsg.SetRepetitions(uint16(set.Reps))
			}
			if set.WeightKg > 0 {
				setMsg.SetWeightScaled(set.WeightKg)
			}
			if set.DurationSeconds > 0 {
				setMsg.SetDuration(uint32(set.DurationSeconds * 1000))
			}
			fit.Messages = append(fit.Messages, setMsg.ToMesg(nil))
		}
	}

	// Append Summary
	fit.Messages = append(fit.Messages, lapMsg.ToMesg(nil))
	fit.Messages = append(fit.Messages, sessionMsg.ToMesg(nil))
	fit.Messages = append(fit.Messages, activityMsg.ToMesg(nil))

	// Encode
	var buf bytes.Buffer
	enc := encoder.New(&buf)
	if err := enc.Encode(fit); err != nil {
		return nil, fmt.Errorf("failed to encode FIT file: %w", err)
	}

	return buf.Bytes(), nil
}

func mapSport(activityType pb.ActivityType) (typedef.Sport, typedef.SubSport) {
	switch activityType {
	// Running
	case pb.ActivityType_ACTIVITY_TYPE_RUN, pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RUN, pb.ActivityType_ACTIVITY_TYPE_TRAIL_RUN:
		return typedef.SportRunning, typedef.SubSportGeneric

	// Cycling
	case pb.ActivityType_ACTIVITY_TYPE_RIDE, pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_GRAVEL_RIDE, pb.ActivityType_ACTIVITY_TYPE_MOUNTAIN_BIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_EMOUNTAIN_BIKE_RIDE, pb.ActivityType_ACTIVITY_TYPE_EBIKE_RIDE,
		pb.ActivityType_ACTIVITY_TYPE_VELOMOBILE, pb.ActivityType_ACTIVITY_TYPE_HANDCYCLE:
		return typedef.SportCycling, typedef.SubSportGeneric

	// Swimming
	case pb.ActivityType_ACTIVITY_TYPE_SWIM:
		return typedef.SportSwimming, typedef.SubSportLapSwimming

	// Walking/Hiking
	case pb.ActivityType_ACTIVITY_TYPE_WALK:
		return typedef.SportWalking, typedef.SubSportGeneric
	case pb.ActivityType_ACTIVITY_TYPE_HIKE, pb.ActivityType_ACTIVITY_TYPE_SNOWSHOE:
		return typedef.SportHiking, typedef.SubSportGeneric

	// Training / Gym
	case pb.ActivityType_ACTIVITY_TYPE_WEIGHT_TRAINING:
		return typedef.SportTraining, typedef.SubSportStrengthTraining
	case pb.ActivityType_ACTIVITY_TYPE_WORKOUT, pb.ActivityType_ACTIVITY_TYPE_CROSSFIT,
		pb.ActivityType_ACTIVITY_TYPE_ELLIPTICAL, pb.ActivityType_ACTIVITY_TYPE_STAIR_STEPPER,
		pb.ActivityType_ACTIVITY_TYPE_PILATES:
		return typedef.SportTraining, typedef.SubSportGeneric
	case pb.ActivityType_ACTIVITY_TYPE_YOGA:
		return typedef.SportTraining, typedef.SubSportYoga
	case pb.ActivityType_ACTIVITY_TYPE_HIGH_INTENSITY_INTERVAL_TRAINING:
		return typedef.SportTraining, typedef.SubSportHiit

	// Water Sports
	case pb.ActivityType_ACTIVITY_TYPE_ROWING, pb.ActivityType_ACTIVITY_TYPE_VIRTUAL_ROW,
		pb.ActivityType_ACTIVITY_TYPE_CANOEING, pb.ActivityType_ACTIVITY_TYPE_KAYAKING,
		pb.ActivityType_ACTIVITY_TYPE_STAND_UP_PADDLING, pb.ActivityType_ACTIVITY_TYPE_SURFING,
		pb.ActivityType_ACTIVITY_TYPE_WINDSURF, pb.ActivityType_ACTIVITY_TYPE_KITESURF,
		pb.ActivityType_ACTIVITY_TYPE_SAIL:
		return typedef.SportRowing, typedef.SubSportGeneric

	// Winter Sports
	case pb.ActivityType_ACTIVITY_TYPE_ALPINE_SKI, pb.ActivityType_ACTIVITY_TYPE_BACKCOUNTRY_SKI,
		pb.ActivityType_ACTIVITY_TYPE_NORDIC_SKI, pb.ActivityType_ACTIVITY_TYPE_ROLLER_SKI,
		pb.ActivityType_ACTIVITY_TYPE_SNOWBOARD, pb.ActivityType_ACTIVITY_TYPE_ICE_SKATE:
		return typedef.SportGeneric, typedef.SubSportGeneric

	// Team / Racket Sports
	case pb.ActivityType_ACTIVITY_TYPE_SOCCER, pb.ActivityType_ACTIVITY_TYPE_GOLF,
		pb.ActivityType_ACTIVITY_TYPE_TENNIS, pb.ActivityType_ACTIVITY_TYPE_SQUASH,
		pb.ActivityType_ACTIVITY_TYPE_RACQUETBALL, pb.ActivityType_ACTIVITY_TYPE_BADMINTON,
		pb.ActivityType_ACTIVITY_TYPE_TABLE_TENNIS, pb.ActivityType_ACTIVITY_TYPE_PICKLEBALL:
		return typedef.SportGeneric, typedef.SubSportGeneric

	// Other
	case pb.ActivityType_ACTIVITY_TYPE_ROCK_CLIMBING, pb.ActivityType_ACTIVITY_TYPE_SKATEBOARD,
		pb.ActivityType_ACTIVITY_TYPE_WHEELCHAIR, pb.ActivityType_ACTIVITY_TYPE_INLINE_SKATE:
		return typedef.SportGeneric, typedef.SubSportGeneric

	default:
		return typedef.SportGeneric, typedef.SubSportGeneric
	}
}

func mapSourceToDevice(source string) (typedef.Manufacturer, string) {
	// 255 is ManufacturerDevelopment
	// We use this because we don't have official Manufacturer IDs for these apps
	const manufacturerDevelopment = typedef.Manufacturer(255)

	switch source {
	case "SOURCE_HEVY":
		return manufacturerDevelopment, "Hevy"
	case "SOURCE_TEST":
		return manufacturerDevelopment, "FitGlue Test"
	default:
		return manufacturerDevelopment, "FitGlue"
	}
}
