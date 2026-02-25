package github

import (
	"testing"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/stretchr/testify/assert"
)

func TestGitHubUploader_Name(t *testing.T) {
	u := New(&bootstrap.Service{})
	assert.Equal(t, "github", u.Name())
}

func TestGitHubUploader_MergeUserContent(t *testing.T) {
	existing := "Some content\n<!-- fitglue:end -->\n\nUser edit!"
	newContent := "New generated content\n<!-- fitglue:end -->"
	merged := mergeWithUserContent(newContent, existing)

	expected := "New generated content\n<!-- fitglue:end -->\n\nUser edit!"
	assert.Equal(t, expected, merged)
}
