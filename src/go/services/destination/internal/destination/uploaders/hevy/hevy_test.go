package hevy

import (
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestHevyUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "hevy", u.Name())
}
