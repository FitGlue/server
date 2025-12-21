package main

import (
	"log"
	"os"

	_ "github.com/ripixel/fitglue-server/src/go/functions/enricher" // Import function/init

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func main() {
	port := "8081"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
