package googlesheets

import (
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestGoogleSheetsUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "googlesheets", u.Name())
}
