package main

import (
	"bytes"
	"flag"
	"fmt"
	"math"
	"os"
	"reflect"
	"text/tabwriter"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
)

type FieldStats struct {
	Name  string
	Count int
	Min   float64
	Max   float64
	Sum   float64
}

func NewFieldStats(name string) *FieldStats {
	return &FieldStats{
		Name: name,
		Min:  math.MaxFloat64,
		Max:  -math.MaxFloat64,
	}
}

func (fs *FieldStats) Update(val interface{}) {
	var v float64
	switch t := val.(type) {
	case uint8:
		v = float64(t)
	case uint16:
		v = float64(t)
	case uint32:
		v = float64(t)
	case int8:
		v = float64(t)
	case int16:
		v = float64(t)
	case int32:
		v = float64(t)
	case float32:
		v = float64(t)
	case float64:
		v = t
	default:
		// Try reflection for other numeric types or custom types
		rv := reflect.ValueOf(val)
		switch rv.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			v = float64(rv.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			v = float64(rv.Uint())
		case reflect.Float32, reflect.Float64:
			v = rv.Float()
		case reflect.Struct:
			// Fallback: usage fmt.Sprint to get value string and parse
			// Because fit proto.Value is unexported, we can't access fields directly via reflection without unsafe or hacking.
			// But fmt.Sprint works via its own deep reflection.
			strVal := fmt.Sprint(val)
			var floatVal float64
			if n, err := fmt.Sscanf(strVal, "%f", &floatVal); err == nil && n == 1 {
				fs.Update(floatVal)
				return
			}
			return
		default:
			return // Ignore non-numeric
		}
	}

	fs.Count++
	fs.Sum += v
	if v < fs.Min {
		fs.Min = v
	}
	if v > fs.Max {
		fs.Max = v
	}
}

func (fs *FieldStats) Avg() float64 {
	if fs.Count == 0 {
		return 0
	}
	return fs.Sum / float64(fs.Count)
}

func main() {
	inputPath := flag.String("input", "", "Path to FIT file")
	verbose := flag.Bool("detailed-dump", false, "Print detailed record info")
	flag.Parse()

	if *inputPath == "" {
		fmt.Println("Please provide input file with -input")
		os.Exit(1)
	}

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Printf("Failed to read file: %v\n", err)
		os.Exit(1)
	}

	fitDec := decoder.New(bytes.NewReader(data))
	fitData, err := fitDec.Decode()
	if err != nil {
		fmt.Printf("Failed to decode FIT file: %v\n", err)
		os.Exit(1)
	}

	stats := map[string]*FieldStats{
		"heart_rate":           NewFieldStats("HeartRate"),
		"power":                NewFieldStats("Power"),
		"cadence":              NewFieldStats("Cadence"),
		"speed":                NewFieldStats("Speed"),
		"enhanced_speed":       NewFieldStats("EnhancedSpeed"),
		"distance":             NewFieldStats("Distance"),
		"altitude":             NewFieldStats("Altitude"),
		"enhanced_altitude":    NewFieldStats("EnhancedAltitude"),
		"position_lat":         NewFieldStats("PositionLat"),
		"position_long":        NewFieldStats("PositionLong"),
		"stance_time":          NewFieldStats("GroundContactTime"),
		"vertical_oscillation": NewFieldStats("VerticalOscillation"),
		"vertical_ratio":       NewFieldStats("VerticalRatio"),
		"step_length":          NewFieldStats("StepLength"),
		"accumulated_power":    NewFieldStats("AccumulatedPower"),
	}

	recordCount := 0
	sessionCount := 0
	lapCount := 0

	type sessionInfo struct {
		startTime time.Time
		duration  float64
		distance  float64
		sport     string
		subSport  string
		name      string
	}
	var sessions []sessionInfo

	type lapInfo struct {
		startTime time.Time
		duration  float64
		distance  float64
	}
	var laps []lapInfo

	fmt.Println("Analyzing FIT file...")
	for _, msg := range fitData.Messages {

		if msg.Num == typedef.MesgNumSession {
			sessionCount++
			sessionMsg := mesgdef.NewSession(&msg)
			sessions = append(sessions, sessionInfo{
				startTime: sessionMsg.StartTime.UTC(),
				duration:  float64(sessionMsg.TotalElapsedTime) / 1000,
				distance:  float64(sessionMsg.TotalDistance) / 100,
				sport:     sessionMsg.Sport.String(),
				subSport:  sessionMsg.SubSport.String(),
				name:      sessionMsg.SportProfileName,
			})
		}

		if msg.Num == typedef.MesgNumLap {
			lapCount++
			lapMsg := mesgdef.NewLap(&msg)
			laps = append(laps, lapInfo{
				startTime: lapMsg.StartTime.UTC(),
				duration:  float64(lapMsg.TotalElapsedTime) / 1000,
				distance:  float64(lapMsg.TotalDistance) / 100,
			})
		}

		if msg.Num == typedef.MesgNumRecord {
			recordCount++
			for _, field := range msg.Fields {
				if *verbose {
					// Dump all fields to see what's actually there
					fmt.Printf("Record %d: %q (Num: %d) = %v (Type: %T)\n", recordCount, field.Name, field.Num, field.Value, field.Value)
				}
				if s, ok := stats[field.Name]; ok {
					s.Update(field.Value)
				} else if *verbose {
					fmt.Printf("Field %q not found in stats map (Keys: %v)\n", field.Name, reflect.ValueOf(stats).MapKeys())
				}
			}
		}
	}

	// Print session summary
	fmt.Printf("\n=== SESSIONS: %d ===\n", sessionCount)
	if len(sessions) > 0 {
		sw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(sw, "#\tStart Time\tDuration\tDistance\tSport\tSubSport\tName")
		fmt.Fprintln(sw, "-\t----------\t--------\t--------\t-----\t--------\t----")
		for i, s := range sessions {
			durationStr := fmt.Sprintf("%.0fm%.0fs", s.duration/60, float64(int(s.duration)%60))
			distanceStr := fmt.Sprintf("%.2f km", s.distance/1000)
			fmt.Fprintf(sw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				i+1, s.startTime.Format("15:04:05"), durationStr, distanceStr, s.sport, s.subSport, s.name)
		}
		sw.Flush()
	}

	// Print lap summary
	fmt.Printf("\n=== LAPS: %d ===\n", lapCount)
	if len(laps) > 0 && len(laps) <= 20 { // Only show if reasonable number
		lw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(lw, "#\tStart Time\tDuration\tDistance")
		fmt.Fprintln(lw, "-\t----------\t--------\t--------")
		for i, l := range laps {
			durationStr := fmt.Sprintf("%.0fm%.0fs", l.duration/60, float64(int(l.duration)%60))
			distanceStr := fmt.Sprintf("%.2f km", l.distance/1000)
			fmt.Fprintf(lw, "%d\t%s\t%s\t%s\n", i+1, l.startTime.Format("15:04:05"), durationStr, distanceStr)
		}
		lw.Flush()
	} else if len(laps) > 20 {
		fmt.Printf("(Too many laps to display - showing first and last)\n")
		fmt.Printf("  Lap 1: %s, %.0fs, %.2fkm\n", laps[0].startTime.Format("15:04:05"), laps[0].duration, laps[0].distance/1000)
		fmt.Printf("  Lap %d: %s, %.0fs, %.2fkm\n", len(laps), laps[len(laps)-1].startTime.Format("15:04:05"), laps[len(laps)-1].duration, laps[len(laps)-1].distance/1000)
	}

	fmt.Printf("\n=== RECORDS: %d ===\n", recordCount)
	fmt.Println("\nField Statistics:")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "Field\tCount\tCoverage\tMin\tMax\tAvg")
	fmt.Fprintln(w, "-----\t-----\t--------\t---\t---\t---")

	for name, s := range stats {
		if s.Count > 0 {
			coverage := float64(s.Count) / float64(recordCount) * 100
			fmt.Fprintf(w, "%s\t%d\t%.1f%%\t%.2f\t%.2f\t%.2f\n",
				name, s.Count, coverage, s.Min, s.Max, s.Avg())
		}
	}
	w.Flush()
}
