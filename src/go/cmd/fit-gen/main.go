package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/ripixel/fitglue-server/src/go/pkg/domain/file_generators"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func main() {
	inputFile := flag.String("input", "", "Path to input JSON file (StandardizedActivity)")
	outputFile := flag.String("output", "output.fit", "Path to output FIT file")
	flag.Parse()

	if *inputFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	// 1. Read JSON
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("Failed to read input file: %v", err)
	}

	// 2. Unmarshal to Proto
	var activity pb.StandardizedActivity
	unmarshalOpts := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshalOpts.Unmarshal(data, &activity); err != nil {
		log.Fatalf("Failed to parse JSON: %v", err)
	}

	// 3. Extract HR Stream
	var hrStream []int
	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				hrStream = append(hrStream, int(record.HeartRate))
			}
		}
	}



	// If stream is empty, user might be testing "empty file" scenario, or wants dummy data.
	// For now, we proceed. If generator fails, it fails.
	fmt.Printf("Extracted %d heart rate points\n", len(hrStream))

	// 4. Generate FIT
	fitData, err := file_generators.GenerateFitFile(&activity, hrStream)
	if err != nil {
		log.Fatalf("Failed to generate FIT file: %v", err)
	}

	// 5. Write Output
	if err := os.WriteFile(*outputFile, fitData, 0644); err != nil {
		log.Fatalf("Failed to write output file: %v", err)
	}

	fmt.Printf("Successfully wrote FIT file to %s (%d bytes)\n", *outputFile, len(fitData))
}

