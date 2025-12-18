package fit

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tormoder/fit"
	"github.com/tormoder/fit/filedef"
)

// GenerateFitFile creates a binary FIT file from streams
func GenerateFitFile(startTime time.Time, durationSec int, powerStream []int, hrStream []int) ([]byte, error) {
	// Initialize FIT file
	f, err := fit.NewFile(filedef.Activity, fit.WithProtocolVersion(fit.V20))
	if err != nil {
		return nil, fmt.Errorf("factory error: %v", err)
	}

	// Create Activity ID (timestamp)
	// Real implementation involves setting FileId, Activity, Session, Lap, and Records.

	// Create Session
	session, err := f.Activity.AddSession()
	if err != nil { return nil, err }
	session.Sport = filedef.SportCycling
	session.SubSport = filedef.SubSportIndoorCycling
	session.StartTime = fit.NewDateTime(startTime)
	session.TotalElapsedTime = uint32(durationSec) * 1000 // ms
	session.TotalTimerTime = uint32(durationSec) * 1000

	// Add Records (1Hz)
	for i := 0; i < durationSec; i++ {
		rec, err := f.Activity.AddRecord()
		if err != nil { return nil, err }
		rec.Timestamp = fit.NewDateTime(startTime.Add(time.Duration(i) * time.Second))

		// Power
		if i < len(powerStream) {
			p := uint16(powerStream[i])
			rec.Power = p
		}

		// HR
		if i < len(hrStream) {
			h := uint8(hrStream[i])
			rec.HeartRate = h
		}
	}

	buf := new(bytes.Buffer)
	if err := fit.Encode(buf, f, fit.WithBigEndian()); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
