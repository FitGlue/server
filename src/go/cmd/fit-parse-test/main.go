package main

import (
	"fmt"
	"os"

	"github.com/fitglue/server/src/go/pkg/domain/fit_parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: fit-parse-test <fit-file>")
		os.Exit(1)
	}

	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("Failed to read file: %v\n", err)
		os.Exit(1)
	}

	activity, err := fit_parser.ParseFitFile(data)
	if err != nil {
		fmt.Printf("Failed to parse FIT file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Activity: %s\n", activity.Name)
	fmt.Printf("Sessions: %d\n", len(activity.Sessions))

	if len(activity.Sessions) > 0 {
		session := activity.Sessions[0]
		fmt.Printf("Laps: %d\n\n", len(session.Laps))

		for i, lap := range session.Laps {
			mins := int(lap.TotalElapsedTime) / 60
			secs := int(lap.TotalElapsedTime) % 60
			fmt.Printf("Lap %d: %d:%02d (%.0fm)\n", i+1, mins, secs, lap.TotalDistance)
		}
	}
}
