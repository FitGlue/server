package main

import (
	"context"
	"log"
	"net"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/fitglue/server/src/go/internal/activity"
	"github.com/fitglue/server/src/go/internal/infra"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	gcsstorage "github.com/fitglue/server/src/go/pkg/infrastructure/storage"
	pb "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084" // Default port for activity service
	}

	ctx := context.Background()
	logger := infra.NewLogger()

	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		projectID = "fitglue-server-dev"
	}

	// Firestore
	fsClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("failed to init firestore: %v", err)
	}
	defer fsClient.Close()
	store := activity.NewFirestoreStore(fsClient)

	// Google Cloud Storage
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to init gcs: %v", err)
	}
	defer gcsClient.Close()
	blobStore := &GCSBlobStore{adapter: &gcsstorage.StorageAdapter{Client: gcsClient}}

	// Pub/Sub
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("failed to init pubsub: %v", err)
	}
	defer pubsubClient.Close()
	pub := &infrapubsub.PubSubAdapter{Client: pubsubClient}

	bucketName := os.Getenv("ARTIFACT_BUCKET")
	if bucketName == "" {
		bucketName = "fitglue-server-dev-artifacts"
	}

	svc := activity.NewService(store, blobStore, pub, bucketName, logger)

	server := grpc.NewServer()
	pb.RegisterActivityServiceServer(server, svc)

	healthcheck := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthcheck)

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	logger.Info(ctx, "Starting service.activity", "port", port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
