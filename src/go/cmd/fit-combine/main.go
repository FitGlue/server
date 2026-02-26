package main

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
	"github.com/muktihari/fit/proto"
)

func main() {
	input1 := flag.String("input1", "", "Path to first FIT file")
	input2 := flag.String("input2", "", "Path to second FIT file")
	output := flag.String("output", "combined.fit", "Path to output FIT file")
	flag.Parse()

	if *input1 == "" || *input2 == "" {
		fmt.Println("Usage: fit-combine -input1 <file1.fit> -input2 <file2.fit> [-output <out.fit>]")
		os.Exit(1)
	}

	fit1, err := decodeFitFile(*input1)
	if err != nil {
		fmt.Printf("Failed to decode %s: %v\n", *input1, err)
		os.Exit(1)
	}

	fit2, err := decodeFitFile(*input2)
	if err != nil {
		fmt.Printf("Failed to decode %s: %v\n", *input2, err)
		os.Exit(1)
	}

	combined, err := combineFitFiles(fit1, fit2)
	if err != nil {
		fmt.Printf("Failed to combine FIT files: %v\n", err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	enc := encoder.New(&buf)
	if err := enc.Encode(combined); err != nil {
		fmt.Printf("Failed to encode combined FIT file: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, buf.Bytes(), 0644); err != nil {
		fmt.Printf("Failed to write output file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully combined FIT files into %s (%d bytes)\n", *output, buf.Len())
}

func decodeFitFile(path string) (*proto.FIT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	dec := decoder.New(bytes.NewReader(data))
	fit, err := dec.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return fit, nil
}

func combineFitFiles(fit1, fit2 *proto.FIT) (*proto.FIT, error) {
	combined := &proto.FIT{
		Messages: []proto.Message{},
	}

	var (
		fileIds1     []proto.Message
		deviceInfos1 []proto.Message
		records      []proto.Message
		laps         []proto.Message
		sessions     []proto.Message
		sets         []proto.Message
		activities   []proto.Message
		events       []proto.Message
		other        []proto.Message
	)

	categorise := func(msgs []proto.Message, isFirst bool) {
		for _, msg := range msgs {
			switch msg.Num {
			case typedef.MesgNumFileId:
				if isFirst {
					fileIds1 = append(fileIds1, msg)
				}
			case typedef.MesgNumDeviceInfo:
				if isFirst {
					deviceInfos1 = append(deviceInfos1, msg)
				}
			case typedef.MesgNumRecord:
				records = append(records, msg)
			case typedef.MesgNumLap:
				laps = append(laps, msg)
			case typedef.MesgNumSession:
				sessions = append(sessions, msg)
			case typedef.MesgNumSet:
				sets = append(sets, msg)
			case typedef.MesgNumActivity:
				activities = append(activities, msg)
			case typedef.MesgNumEvent:
				events = append(events, msg)
			default:
				other = append(other, msg)
			}
		}
	}

	categorise(fit1.Messages, true)
	categorise(fit2.Messages, false)

	// Sort records by timestamp
	sortByTimestamp(records)
	sortByTimestamp(events)

	// Interpolate gaps between records (fills with 1-second synthetic records)
	records = interpolateGaps(records)

	// Sort laps by start time and re-index
	sortByTimestamp(laps)
	for i := range laps {
		lapMsg := mesgdef.NewLap(&laps[i])
		lapMsg.SetMessageIndex(typedef.MessageIndex(i))
		laps[i] = lapMsg.ToMesg(nil)
	}

	// Sort sessions by start time and re-index
	sortByTimestamp(sessions)
	for i := range sessions {
		sessionMsg := mesgdef.NewSession(&sessions[i])
		sessionMsg.SetMessageIndex(typedef.MessageIndex(i))
		sessions[i] = sessionMsg.ToMesg(nil)
	}

	// Sort sets by timestamp and re-index
	sortByTimestamp(sets)
	for i := range sets {
		setMsg := mesgdef.NewSet(&sets[i])
		setMsg.SetMessageIndex(typedef.MessageIndex(i))
		sets[i] = setMsg.ToMesg(nil)
	}

	// Build a single Activity message
	activityMsg := mesgdef.NewActivity(nil).
		SetType(typedef.ActivityManual).
		SetNumSessions(uint16(len(sessions)))
	if len(records) > 0 {
		rec := mesgdef.NewRecord(&records[0])
		activityMsg.SetTimestamp(rec.Timestamp)
	} else if len(activities) > 0 {
		act := mesgdef.NewActivity(&activities[0])
		activityMsg.SetTimestamp(act.Timestamp)
	}

	// Assemble: FileId → DeviceInfo → Other → Events → Records → Sets → Laps → Sessions → Activity
	combined.Messages = append(combined.Messages, fileIds1...)
	combined.Messages = append(combined.Messages, deviceInfos1...)
	combined.Messages = append(combined.Messages, other...)
	combined.Messages = append(combined.Messages, events...)
	combined.Messages = append(combined.Messages, records...)
	combined.Messages = append(combined.Messages, sets...)
	combined.Messages = append(combined.Messages, laps...)
	combined.Messages = append(combined.Messages, sessions...)
	combined.Messages = append(combined.Messages, activityMsg.ToMesg(nil))

	return combined, nil
}

// interpolateGaps fills time gaps between consecutive records with synthetic
// records at 1-second intervals. HR, cadence, power, speed, and altitude are
// linearly interpolated between the boundary values.
func interpolateGaps(records []proto.Message) []proto.Message {
	if len(records) < 2 {
		return records
	}

	result := make([]proto.Message, 0, len(records))
	interpolatedCount := 0

	for i := 0; i < len(records); i++ {
		result = append(result, records[i])

		if i+1 >= len(records) {
			break
		}

		recA := mesgdef.NewRecord(&records[i])
		recB := mesgdef.NewRecord(&records[i+1])

		tA := recA.Timestamp
		tB := recB.Timestamp

		gapSeconds := int(tB.Sub(tA).Seconds())
		if gapSeconds <= 5 {
			continue // Only interpolate significant gaps, not normal recording cadence
		}

		for s := 1; s < gapSeconds; s++ {
			fraction := float64(s) / float64(gapSeconds)
			ts := tA.Add(time.Duration(s) * time.Second)

			synthRecord := mesgdef.NewRecord(nil).SetTimestamp(ts)

			// Heart Rate
			if recA.HeartRate != 0xFF && recB.HeartRate != 0xFF {
				synthRecord.SetHeartRate(lerpUint8(recA.HeartRate, recB.HeartRate, fraction))
			} else if recA.HeartRate != 0xFF {
				synthRecord.SetHeartRate(recA.HeartRate)
			} else if recB.HeartRate != 0xFF {
				synthRecord.SetHeartRate(recB.HeartRate)
			}

			// Cadence
			if recA.Cadence != 0xFF && recB.Cadence != 0xFF {
				synthRecord.SetCadence(lerpUint8(recA.Cadence, recB.Cadence, fraction))
			}

			// Power
			if recA.Power != 0xFFFF && recB.Power != 0xFFFF {
				synthRecord.SetPower(lerpUint16(recA.Power, recB.Power, fraction))
			}

			// Speed
			if recA.Speed != 0xFFFF && recB.Speed != 0xFFFF {
				synthRecord.SetSpeed(lerpUint16(recA.Speed, recB.Speed, fraction))
			}

			// Altitude
			if recA.Altitude != 0xFFFF && recB.Altitude != 0xFFFF {
				synthRecord.SetAltitude(lerpUint16(recA.Altitude, recB.Altitude, fraction))
			}

			// Position (lat/long)
			if recA.PositionLat != 0x7FFFFFFF && recB.PositionLat != 0x7FFFFFFF {
				synthRecord.SetPositionLat(lerpInt32(recA.PositionLat, recB.PositionLat, fraction))
				synthRecord.SetPositionLong(lerpInt32(recA.PositionLong, recB.PositionLong, fraction))
			}

			result = append(result, synthRecord.ToMesg(nil))
			interpolatedCount++
		}
	}

	if interpolatedCount > 0 {
		fmt.Printf("Interpolated %d records across gaps\n", interpolatedCount)
	}

	return result
}

func lerpUint8(a, b uint8, t float64) uint8 {
	return uint8(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

func lerpUint16(a, b uint16, t float64) uint16 {
	return uint16(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

func lerpInt32(a, b int32, t float64) int32 {
	return int32(math.Round(float64(a) + (float64(b)-float64(a))*t))
}

// sortByTimestamp sorts messages by their Timestamp field (field num 253).
func sortByTimestamp(msgs []proto.Message) {
	sort.SliceStable(msgs, func(i, j int) bool {
		ti := getTimestamp(msgs[i])
		tj := getTimestamp(msgs[j])
		return ti < tj
	})
}

// getTimestamp extracts the raw timestamp uint32 value from a message.
func getTimestamp(msg proto.Message) uint32 {
	for _, field := range msg.Fields {
		if field.Num == 253 { // timestamp field number in FIT protocol
			switch v := field.Value.Any().(type) {
			case uint32:
				return v
			}
		}
	}
	return 0
}
