package main

import (
	"log"
	"os"

	_ "github.com/ripixel/fitglue-server/src/go/functions/strava-uploader" // Import function/init

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func main() {
	port := "8083"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
