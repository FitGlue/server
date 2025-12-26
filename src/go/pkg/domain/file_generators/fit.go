package file_generators

import (
	"bytes"
	"fmt"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

// GenerateFitFile creates a FIT file from StandardizedActivity
// Currently supports strength training activities only
func GenerateFitFile(activity *pb.StandardizedActivity, hrStream []int) ([]byte, error) {
	if activity == nil {
		return nil, fmt.Errorf("activity cannot be nil")
	}

	if len(activity.Sessions) == 0 {
		return nil, fmt.Errorf("activity must have at least one session")
	}

	// Parse start time
	startTime, err := time.Parse(time.RFC3339, activity.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	// Use first session (simplified for now)
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

	// 2. Activity message
	activityMsg := mesgdef.NewActivity(nil).
		SetTimestamp(startTime).
		SetType(typedef.ActivityManual).
		SetNumSessions(1)
	fit.Messages = append(fit.Messages, activityMsg.ToMesg(nil))

	// 3. Session message
	sessionMsg := mesgdef.NewSession(nil).
		SetTimestamp(startTime).
		SetSport(typedef.SportTraining).
		SetStartTime(startTime)

	// Set session duration if available
	if session.TotalElapsedTime > 0 {
		sessionMsg.SetTotalElapsedTime(uint32(session.TotalElapsedTime * 1000)) // milliseconds
		sessionMsg.SetTotalTimerTime(uint32(session.TotalElapsedTime * 1000))
	}

	fit.Messages = append(fit.Messages, sessionMsg.ToMesg(nil))

	// 4. Set messages for each strength set
	for i, set := range session.StrengthSets {
		setStartTime := startTime
		if set.StartTime != "" {
			var err error
			setStartTime, err = time.Parse(time.RFC3339, set.StartTime)
			if err != nil {
				// Use activity start time if parse fails
				setStartTime = startTime
			}
		}

		// Map exercise name to FIT category
		category := MapExerciseToCategory(set.ExerciseName)

		// Create Set message
		setMsg := mesgdef.NewSet(nil).
			SetTimestamp(setStartTime).
			SetStartTime(setStartTime).
			SetCategory([]typedef.ExerciseCategory{category}).
			SetSetType(typedef.SetTypeActive).
			SetMessageIndex(typedef.MessageIndex(i))

		// Add repetitions
		if set.Reps > 0 {
			setMsg.SetRepetitions(uint16(set.Reps))
		}

		// Add weight (FIT uses SetWeightScaled which handles the scaling)
		if set.WeightKg > 0 {
			setMsg.SetWeightScaled(set.WeightKg)
		}

		// Add duration (FIT uses milliseconds)
		if set.DurationSeconds > 0 {
			setMsg.SetDuration(uint32(set.DurationSeconds * 1000))
		}

		fit.Messages = append(fit.Messages, setMsg.ToMesg(nil))
	}

	// Encode to FIT file
	var buf bytes.Buffer
	enc := encoder.New(&buf)

	if err := enc.Encode(fit); err != nil {
		return nil, fmt.Errorf("failed to encode FIT file: %w", err)
	}

	return buf.Bytes(), nil
}
