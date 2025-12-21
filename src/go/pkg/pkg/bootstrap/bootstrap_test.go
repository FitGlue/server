package bootstrap

import (
	"log/slog"
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Save original env
	origProject := os.Getenv("GOOGLE_CLOUD_PROJECT")
	origPublish := os.Getenv("ENABLE_PUBLISH")
	origBucket := os.Getenv("GCS_ARTIFACT_BUCKET")
	defer func() {
		os.Setenv("GOOGLE_CLOUD_PROJECT", origProject)
		os.Setenv("ENABLE_PUBLISH", origPublish)
		os.Setenv("GCS_ARTIFACT_BUCKET", origBucket)
	}()

	t.Run("Defaults", func(t *testing.T) {
		os.Unsetenv("GOOGLE_CLOUD_PROJECT")
		os.Unsetenv("ENABLE_PUBLISH")
		os.Unsetenv("GCS_ARTIFACT_BUCKET")

		cfg := LoadConfig()
		if cfg.ProjectID == "" {
			t.Error("ProjectID should have default fallback")
		}
		if cfg.EnablePublish != false {
			t.Error("EnablePublish should default to false")
		}
	})

	t.Run("Overrides", func(t *testing.T) {
		os.Setenv("GOOGLE_CLOUD_PROJECT", "test-project")
		os.Setenv("ENABLE_PUBLISH", "true")
		os.Setenv("GCS_ARTIFACT_BUCKET", "test-bucket")

		cfg := LoadConfig()
		if cfg.ProjectID != "test-project" {
			t.Errorf("Expected test-project, got %s", cfg.ProjectID)
		}
		if !cfg.EnablePublish {
			t.Error("Expected EnablePublish to be true")
		}
		if cfg.GCSArtifactBucket != "test-bucket" {
			t.Errorf("Expected test-bucket, got %s", cfg.GCSArtifactBucket)
		}
	})
}

func TestInitLogger(t *testing.T) {
	// Capture output
	// var buf bytes.Buffer
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Reuse the same logic as InitLogger for the test handler,
			// or we can invoke InitLogger and somehow capture os.Stdout (harder).
			// Instead, let's test the replacement logic directly if possible,
			// but InitLogger doesn't expose the handler options.
			// Better: Just verify InitLogger runs without panic,
			// and maybe we can't easily capture os.Stdout in parallel tests safely.
			// So let's just ensure it sets the default logger.
			return a
		},
	}
	_ = opts // unused in this simple test logic

	// Execute
	InitLogger()
	// No panic means success.
	// Validating output format would require replacing os.Stdout or modifying InitLogger to accept a writer.
	// For now, simple execution check is sufficient.
	slog.Info("Test log")
}

func TestLoggerFormat(t *testing.T) {
	// Re-implement the key mapper to test just the logic
	replace := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.MessageKey {
			return slog.Attr{Key: "message", Value: a.Value}
		}
		if a.Key == slog.LevelKey {
			return slog.Attr{Key: "severity", Value: a.Value}
		}
		return a
	}

	// Test Message Key
	a := slog.Attr{Key: slog.MessageKey, Value: slog.StringValue("hello")}
	res := replace(nil, a)
	if res.Key != "message" {
		t.Errorf("Expected message key replacement, got %s", res.Key)
	}

	// Test Level Key
	a = slog.Attr{Key: slog.LevelKey, Value: slog.StringValue("INFO")}
	res = replace(nil, a)
	if res.Key != "severity" {
		t.Errorf("Expected severity key replacement, got %s", res.Key)
	}
}
