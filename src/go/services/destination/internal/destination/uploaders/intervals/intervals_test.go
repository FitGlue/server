package intervals

import (
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestIntervalsUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "intervals", u.Name())
}
