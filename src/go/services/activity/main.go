package main

import (
	"context"
	"log"
	"net"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/fitglue/server/src/go/internal/activity"
	"github.com/fitglue/server/src/go/internal/infra"
	pb "github.com/fitglue/server/src/go/pkg/types/pb/services/activity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type stubBlobStore struct{}

func (s *stubBlobStore) Get(ctx context.Context, bucket, path string) ([]byte, error) {
	return []byte{}, nil
}

func (s *stubBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	return nil
}

func (s *stubBlobStore) Delete(ctx context.Context, bucket, path string) error {
	return nil
}

func (s *stubBlobStore) SignedURL(ctx context.Context, bucket, path, contentType string, expiry time.Duration) (string, error) {
	return "", nil
}

type stubPublisher struct{}

func (p *stubPublisher) PublishCloudEvent(ctx context.Context, topic string, ce cloudevents.Event) (string, error) {
	return "stub-id", nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8084" // Default port for activity service
	}

	logger := infra.NewLogger()

	// TODO: Replace with real Firestore adapter initialization when implemented
	var fsClient *firestore.Client
	store := activity.NewFirestoreStore(fsClient)

	blobStore := &stubBlobStore{}
	pub := &stubPublisher{}
	bucketName := os.Getenv("BUCKET_NAME")
	if bucketName == "" {
		bucketName = "fitglue-staging.appspot.com"
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

	logger.Info(context.Background(), "Starting service.activity", "port", port)
	if err := server.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
