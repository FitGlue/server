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
	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/database"
	infrapubsub "github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/pubsub"
	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/secrets"
	infrastorage "github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/storage"
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
	Store   shared.BlobStore
	Pub     shared.Publisher
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

// GetSlogHandlerOptions returns standard handler options for GCP
func GetSlogHandlerOptions(level slog.Level) *slog.HandlerOptions {
	return &slog.HandlerOptions{
		Level: level,
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
}

// InitLogger configures structured logging with Cloud Logging compatible keys
func InitLogger() {
	opts := GetSlogHandlerOptions(slog.LevelInfo)
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
		pubAdapter = &infrapubsub.PubSubAdapter{Client: psClient}
		slog.Info("Pub/Sub: REAL (ENABLE_PUBLISH=true)")
	} else {
		pubAdapter = &infrapubsub.LogPublisher{}
		slog.Info("Pub/Sub: MOCK (LogPublisher)")
	}

	// Storage
	gcsClient, err := storage.NewClient(ctx)
	if err != nil {
		slog.Error("Storage init failed", "error", err)
		return nil, fmt.Errorf("storage init: %w", err)
	}

	return &Service{
		DB:      database.NewFirestoreAdapter(fsClient),
		Pub:     pubAdapter,
		Store:   &infrastorage.StorageAdapter{Client: gcsClient},
		Secrets: &secrets.SecretsAdapter{},
		Config:  cfg,
	}, nil
}
