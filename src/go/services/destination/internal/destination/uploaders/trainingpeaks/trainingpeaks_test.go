package trainingpeaks

import (
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestTrainingPeaksUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "trainingpeaks", u.Name())
}
