package main

import (
	"bytes"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fit-lap-debug <fit-file>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
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

	fmt.Println("=== LAP ANALYSIS ===")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tStart\tDuration\tRawDur\tDistance\tWktStepIdx\tLapTrigger\tSport")
	fmt.Fprintln(w, "-\t-----\t--------\t------\t--------\t----------\t----------\t-----")

	lapNum := 0
	for _, msg := range fitData.Messages {
		if msg.Num == typedef.MesgNumLap {
			lapNum++
			lapMsg := mesgdef.NewLap(&msg)

			duration := float64(lapMsg.TotalElapsedTime) / 1000
			distance := float64(lapMsg.TotalDistance) / 100

			// Format duration as M:SS
			mins := int(duration) / 60
			secs := int(duration) % 60
			durationStr := fmt.Sprintf("%d:%02d", mins, secs)
			rawDurStr := fmt.Sprintf("%.3f", duration)

			// Format distance
			distanceStr := fmt.Sprintf("%.0fm", distance)

			// Workout step index
			wktStepStr := "nil"
			if lapMsg.WktStepIndex != typedef.MessageIndexInvalid {
				wktStepStr = fmt.Sprintf("%d", lapMsg.WktStepIndex)
			}

			// Lap trigger
			lapTriggerStr := "nil"
			if lapMsg.LapTrigger != typedef.LapTriggerInvalid {
				lapTriggerStr = lapMsg.LapTrigger.String()
			}

			// Sport (if different from session)
			sportStr := ""
			if lapMsg.Sport != typedef.SportInvalid {
				sportStr = lapMsg.Sport.String()
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				lapNum,
				lapMsg.StartTime.UTC().Format("15:04:05"),
				durationStr,
				rawDurStr,
				distanceStr,
				wktStepStr,
				lapTriggerStr,
				sportStr,
			)
		}
	}
	w.Flush()

	fmt.Printf("\nTotal laps: %d\n", lapNum)
}
