package strava

import (
	"context"
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pbevents "github.com/fitglue/server/src/go/pkg/types/pb/models/events"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	"github.com/stretchr/testify/assert"
)

func TestStravaUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "strava", u.Name())
}

func TestStravaUploader_Create_NilUser(t *testing.T) {
	u := New(&bootstrap.Service{})
	_, err := u.Create(context.Background(), &pbevents.ActivityPayload{}, nil)
	assert.Error(t, err)
}

func TestStravaUploader_Update_NilUser(t *testing.T) {
	u := New(&bootstrap.Service{})
	err := u.Update(context.Background(), &pbevents.ActivityPayload{}, nil, &pbpipeline.PipelineRun{})
	assert.Error(t, err)
}
