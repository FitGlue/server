// nolint:proto-json
package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/fitglue/server/src/go/internal/infra"
	"github.com/fitglue/server/src/go/internal/pipeline"
	"github.com/fitglue/server/src/go/internal/pipeline/enricher"
	"github.com/fitglue/server/src/go/internal/pipeline/router"
	"github.com/fitglue/server/src/go/internal/pipeline/splitter"
	infrapubsub "github.com/fitglue/server/src/go/pkg/infrastructure/pubsub"
	pb "github.com/fitglue/server/src/go/pkg/types/pb/services/pipeline"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type stubPublisher struct{}

func (p *stubPublisher) PublishCloudEvent(ctx context.Context, topic string, ce cloudevents.Event) (string, error) {
	return "stub-id", nil
}

type stubBlobStore struct{}

func (s *stubBlobStore) Get(ctx context.Context, uri string) ([]byte, error) {
	return []byte{}, nil
}

func (s *stubBlobStore) Write(ctx context.Context, bucket, path string, data []byte) error {
	return nil
}

func main() {
	logger := infra.NewLogger()
	ctx := context.Background()

	// Initialize dependencies
	fsClient, err := firestore.NewClient(ctx, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Fatalf("failed to init firestore: %v", err)
	}
	defer fsClient.Close()

	store := pipeline.NewFirestoreStore(fsClient)

	// In the new architecture, we use a real publisher and blob store
	rawPubClient, err := pubsub.NewClient(ctx, os.Getenv("PROJECT_ID"))
	if err != nil {
		log.Fatalf("failed to init pubsub: %v", err)
	}
	pubClient := &infrapubsub.PubSubAdapter{Client: rawPubClient}

	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("failed to init gcs: %v", err)
	}

	blobStore := NewGCSBlobStore(gcsClient)
	bucketName := os.Getenv("ARTIFACT_BUCKET")
	if bucketName == "" {
		bucketName = "fitglue-server-dev-artifacts"
	}

	// 1. gRPC Service (CRUD) — internal port for service-to-service calls
	svc := pipeline.NewService(store, pubClient, blobStore, logger)

	server := grpc.NewServer()
	pb.RegisterPipelineServiceServer(server, svc)

	healthcheck := health.NewServer()
	grpc_health_v1.RegisterHealthServer(server, healthcheck)

	grpcPort := os.Getenv("GRPC_PORT")
	if grpcPort == "" {
		grpcPort = "50053"
	}

	lis, err := net.Listen("tcp", ":"+grpcPort)
	if err != nil {
		log.Fatalf("failed to listen on gRPC port: %v", err)
	}

	// Start gRPC server in a goroutine
	go func() {
		logger.Info(ctx, "Starting service.pipeline gRPC", "port", grpcPort)
		if err := server.Serve(lis); err != nil {
			log.Fatalf("failed to serve gRPC: %v", err)
		}
	}()

	// 2. HTTP Server for Pub/Sub Pushes (Cloud Run PORT)
	// Instantiate domain components
	splitterSvc := splitter.NewSplitter(store, pubClient, logger)
	routerSvc := router.NewRouter(store, pubClient, blobStore, bucketName, logger)

	mux := http.NewServeMux()

	// 2a. Splitter (consumes topic-raw-activity)
	mux.HandleFunc("/pubsub/raw", handlePubSubPush(logger, splitterSvc.SplitByPipeline))

	// 2b. Enricher (consumes topic-pipeline-activity)
	// enricher.EnrichActivityHTTP already exists to handle the lag retries and HTTP boilerplate
	mux.HandleFunc("/pubsub/run", enricher.EnrichActivityHTTP)

	// 2c. Router (consumes topic-enriched-activity)
	mux.HandleFunc("/pubsub/enriched", handlePubSubPush(logger, routerSvc.RouteActivity))

	// Basic healthcheck for HTTP
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8080"
	}

	logger.Info(ctx, "Starting service.pipeline HTTP for Pub/Sub pushes", "port", httpPort)
	if err := http.ListenAndServe(":"+httpPort, mux); err != nil {
		log.Fatalf("failed to serve HTTP: %v", err)
	}
}

// handlePubSubPush wraps a domain function taking (ctx, cloudevents.Event) with HTTP Pub/Sub Push unmarshalling
func handlePubSubPush(logger infra.Logger, handler func(context.Context, cloudevents.Event) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Try standard CloudEvents HTTP first
		event, err := cehttp.NewEventFromHTTPRequest(r)
		if err != nil {
			// Fallback: This might be wrapped in a Pub/Sub push payload {"message": {"data": "...", ...}}
			body, ioErr := io.ReadAll(r.Body)
			if ioErr != nil {
				logger.Error(ctx, "Failed to read request body", "error", ioErr)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}
			defer r.Body.Close()

			var pushMsg struct {
				Message struct {
					Data       []byte            `json:"data"`
					MessageID  string            `json:"messageId"`
					Attributes map[string]string `json:"attributes"`
				} `json:"message"`
			}
			if unmarshalErr := json.Unmarshal(body, &pushMsg); unmarshalErr != nil || len(pushMsg.Message.Data) == 0 {
				logger.Error(ctx, "Failed to parse Pub/Sub push payload", "error", unmarshalErr)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			// The inner data should be our serialized CloudEvent
			parsedEvent := cloudevents.NewEvent()
			if jsonErr := json.Unmarshal(pushMsg.Message.Data, &parsedEvent); jsonErr != nil || parsedEvent.Type() == "" {
				// Very raw payload, encapsulate it
				parsedEvent.SetID(pushMsg.Message.MessageID)
				parsedEvent.SetSource("pubsub-push")
				parsedEvent.SetType("com.fitglue.unknown")
				parsedEvent.SetData(cloudevents.ApplicationJSON, pushMsg.Message.Data)
			}
			event = &parsedEvent
		}

		if err := handler(ctx, *event); err != nil {
			logger.Error(ctx, "Handler failed processing event", "error", err, "eventId", event.ID())
			// Return 500 so Pub/Sub retries NACKs
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}
}
