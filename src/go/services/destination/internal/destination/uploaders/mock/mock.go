package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
)

// Uploader implements destination.Destination for mock purposes.
type Uploader struct{}

// New returns a new Mock uploader
func New() *Uploader {
	return &Uploader{}
}

// Name returns the identifier for this uploader
func (u *Uploader) Name() string {
	return "mock"
}

// Create simulates uploading a new activity.
func (u *Uploader) Create(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record) (string, error) {
	// Generate a mock external ID
	mockExternalID := fmt.Sprintf("mock-%s-%d", *payload.ActivityId, time.Now().UnixNano())

	// Simulate work
	time.Sleep(100 * time.Millisecond)

	// In a real uploader, we would log via the injected logger or UpdateStatus directly.
	// Since executor calls the methods, it handles logging success.
	return mockExternalID, nil
}

// Update simulates modifying an existing activity.
func (u *Uploader) Update(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record, pipelineRun *pbpipeline.PipelineRun) error {
	// Simulate work
	time.Sleep(100 * time.Millisecond)
	return nil
}
