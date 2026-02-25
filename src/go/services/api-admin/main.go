package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/fitglue/server/src/go/internal/infra"
	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"github.com/fitglue/server/src/go/services/api-admin/internal/server"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := infra.NewLogger()
	ctx := context.Background()

	logger.Info(ctx, "Starting FitGlue Admin API Gateway", "version", "v1")

	// 1. Initialize Firebase Auth for verifying Admin JWTs
	var fbApp *firebase.App
	var err error
	if creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); creds != "" {
		fbApp, err = firebase.NewApp(ctx, nil, option.WithCredentialsFile(creds))
	} else {
		fbApp, err = firebase.NewApp(ctx, nil)
	}

	if err != nil {
		logger.Error(ctx, "Failed to initialize Firebase App", "error", err)
		os.Exit(1)
	}

	authClient, err := fbApp.Auth(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to initialize Firebase Auth client", "error", err)
		os.Exit(1)
	}

	// 2. Setup gRPC Connections to dependent Domain Services
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		userServiceURL = "localhost:50051" // Default for local dev
	}
	userConn, err := grpc.NewClient(userServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error(ctx, "Failed to connect to User Service", "url", userServiceURL, "error", err)
		os.Exit(1)
	}
	defer userConn.Close()
	userClient := userpb.NewUserServiceClient(userConn)

	pipelineServiceURL := os.Getenv("PIPELINE_SERVICE_URL")
	if pipelineServiceURL == "" {
		pipelineServiceURL = "localhost:50053"
	}
	pipelineConn, err := grpc.NewClient(pipelineServiceURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Error(ctx, "Failed to connect to Pipeline Service", "url", pipelineServiceURL, "error", err)
		os.Exit(1)
	}
	defer pipelineConn.Close()
	pipelineClient := pipelinepb.NewPipelineServiceClient(pipelineConn)

	// 3. Initialize the HTTP Gateway Server
	apiServer := server.NewAPIServer(
		logger,
		authClient,
		userClient,
		pipelineClient,
	)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info(ctx, "Admin API Gateway listening", "port", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), apiServer); err != nil {
		logger.Error(ctx, "HTTP server failed", "error", err)
		os.Exit(1)
	}
}
