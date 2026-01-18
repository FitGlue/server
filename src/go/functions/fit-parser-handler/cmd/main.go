package main

import (
	"log"
	"os"

	// Blank import to register the function
	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
	_ "github.com/fitglue/server/src/go/functions/fit-parser-handler"
)

func main() {
	port := "8081"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}
	if err := funcframework.Start(port); err != nil {
		log.Fatalf("funcframework.Start: %v\n", err)
	}
}
