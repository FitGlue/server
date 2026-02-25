package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/pubsub"
	"github.com/fitglue/server/src/go/internal/infra"
	infraps "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	"github.com/fitglue/server/src/go/services/api-client/internal/server"

	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"

	activitypb "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	billingpb "github.com/fitglue/server/src/go/pkg/types/pb/services/billing"
	pipelinepb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
	registrypb "github.com/fitglue/server/src/go/pkg/types/pb/services/registry"
	userpb "github.com/fitglue/server/src/go/pkg/types/pb/services/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger := infra.NewLogger()
	ctx := context.Background()

	// Firebase Auth Setup
	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")))
	if err != nil {
		logger.Error(ctx, "failed to initialize firebase app", "err", err)
		os.Exit(1)
	}

	authClient, err := app.Auth(ctx)
	if err != nil {
		logger.Error(ctx, "failed to initialize firebase auth client", "err", err)
		os.Exit(1)
	}

	// Initialize gRPC clients
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// Helper to dial a service or panic if connection starts failing severely format (lazy dialing)
	connect := func(targetEnv string, defaultTarget string) *grpc.ClientConn {
		target := os.Getenv(targetEnv)
		if target == "" {
			target = defaultTarget
		}
		conn, err := grpc.NewClient(target, opts...)
		if err != nil {
			logger.Error(ctx, "failed to configure grpc client", "target", target, "error", err)
			os.Exit(1)
		}
		return conn
	}

	userConn := connect("USER_SERVICE_URL", "localhost:50051")
	defer userConn.Close()
	userClient := userpb.NewUserServiceClient(userConn)

	billingConn := connect("BILLING_SERVICE_URL", "localhost:50052")
	defer billingConn.Close()
	billingClient := billingpb.NewBillingServiceClient(billingConn)

	pipelineConn := connect("PIPELINE_SERVICE_URL", "localhost:50053")
	defer pipelineConn.Close()
	pipelineClient := pipelinepb.NewPipelineServiceClient(pipelineConn)

	activityConn := connect("ACTIVITY_SERVICE_URL", "localhost:50054")
	defer activityConn.Close()
	activityClient := activitypb.NewActivityServiceClient(activityConn)

	registryConn := connect("REGISTRY_SERVICE_URL", "localhost:50055")
	defer registryConn.Close()
	registryClient := registrypb.NewRegistryServiceClient(registryConn)

	// Setup Pub/Sub Client
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = "fitglue" // Fallback or development default
	}
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		logger.Error(ctx, "Failed to initialize Pub/Sub client", "error", err)
		os.Exit(1)
	}
	defer pubsubClient.Close()
	publisher := &infraps.PubSubAdapter{Client: pubsubClient}

	// Build API Gateway router
	apiServer := server.NewAPIServer(
		logger,
		authClient,
		publisher,
		userClient,
		billingClient,
		pipelineClient,
		activityClient,
		registryClient,
	)

	logger.Info(ctx, "Starting service.api.client", "port", port)

	if err := http.ListenAndServe(":"+port, apiServer); err != nil {
		log.Fatalf("failed to serve HTTP: %v", err)
	}
}
