package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	shared "github.com/ripixel/fitglue-server/src/go/pkg"
	"github.com/ripixel/fitglue-server/src/go/pkg/adapters"
)

// Config holds standard configuration for all services
type Config struct {
	ProjectID         string
	EnablePublish     bool
	GCSArtifactBucket string
}

// Service holds initialized dependencies
type Service struct {
	DB      shared.Database
	Pub     shared.Publisher
	Store   shared.BlobStore
	Secrets shared.SecretStore
	Config  *Config
}

// LoadConfig reads configuration from environment variables
func LoadConfig() *Config {
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if projectID == "" {
		projectID = shared.ProjectID // Fallback
	}

	return &Config{
		ProjectID:         projectID,
		EnablePublish:     os.Getenv("ENABLE_PUBLISH") == "true",
		GCSArtifactBucket: os.Getenv("GCS_ARTIFACT_BUCKET"),
	}
}

// InitLogger configures structured logging with Cloud Logging compatible keys
func InitLogger() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Map standard keys to Cloud Logging keys
			if a.Key == slog.MessageKey {
				return slog.Attr{Key: "message", Value: a.Value}
			}
			if a.Key == slog.LevelKey {
				return slog.Attr{Key: "severity", Value: a.Value}
			}
			return a
		},
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts))
	slog.SetDefault(logger)
}

// NewService initializes all standard dependencies
func NewService(ctx context.Context) (*Service, error) {
	InitLogger()
	cfg := LoadConfig()

	slog.Info("Initializing service", "project_id", cfg.ProjectID)

	// Firestore
	fsClient, err := firestore.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		slog.Error("Firestore init failed", "error", err)
		return nil, fmt.Errorf("firestore init: %w", err)
	}

	// Pub/Sub
	var pubAdapter shared.Publisher
	if cfg.EnablePublish {
		psClient, err := pubsub.NewClient(ctx, cfg.ProjectID)
		if err != nil {
			slog.Error("PubSub init failed", "error", err)
			return nil, fmt.Errorf("pubsub init: %w", err)
		}
		pubAdapter = &adapters.PubSubAdapter{Client: psClient}
		slog.Info("Pub/Sub: REAL (ENABLE_PUBLISH=true)")
	} else {
		pubAdapter = &adapters.LogPublisher{}
		slog.Info("Pub/Sub: MOCK (LogPublisher)")
	}

	// Storage
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		slog.Error("Storage init failed", "error", err)
		return nil, fmt.Errorf("storage init: %w", err)
	}

	return &Service{
		DB:      &adapters.FirestoreAdapter{Client: fsClient},
		Pub:     pubAdapter,
		Store:   &adapters.StorageAdapter{Client: gcsClient},
		Secrets: &adapters.SecretsAdapter{},
		Config:  cfg,
	}, nil
}
