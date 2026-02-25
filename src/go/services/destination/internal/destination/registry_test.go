package destination

import (
	"context"
	"testing"

	"github.com/fitglue/server/src/go/pkg/domain/user"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
	"github.com/stretchr/testify/assert"
)

type mockUploader struct {
	name string
	err  error
	id   string
}

func (m *mockUploader) Name() string { return m.name }

func (m *mockUploader) Create(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record) (string, error) {
	return m.id, m.err
}

func (m *mockUploader) Update(ctx context.Context, payload *pbevents.ActivityPayload, userRec *user.Record, pipelineRun *pbpipeline.PipelineRun) error {
	return m.err
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// Initially empty
	_, ok := r.Get(pbplugin.DestinationType_DESTINATION_STRAVA)
	assert.False(t, ok)

	// Register
	r.Register(pbplugin.DestinationType_DESTINATION_STRAVA, &mockUploader{name: "strava-mock"})

	// Now it should exist
	uploader, ok := r.Get(pbplugin.DestinationType_DESTINATION_STRAVA)
	assert.True(t, ok)
	assert.Equal(t, "strava-mock", uploader.Name())

	// Other shouldn't
	_, ok = r.Get(pbplugin.DestinationType_DESTINATION_HEVY)
	assert.False(t, ok)
}
