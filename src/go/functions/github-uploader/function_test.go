package githubuploader

import (
	"testing"
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBuildMarkdownContent(t *testing.T) {
	event := createTestEvent()
	content := buildMarkdownContent(event, "")

	if content == "" {
		t.Error("Expected non-empty markdown content")
	}

	// Check YAML frontmatter
	if !contains(content, "---") {
		t.Error("Expected YAML frontmatter delimiters")
	}
	if !contains(content, "title:") {
		t.Error("Expected title in frontmatter")
	}
	if !contains(content, "type:") {
		t.Error("Expected type in frontmatter")
	}
	if !contains(content, "source:") {
		t.Error("Expected source in frontmatter")
	}

	// Check content sections
	if !contains(content, "# Morning Run") {
		t.Error("Expected Markdown heading with activity name")
	}
	if !contains(content, "<!-- fitglue:end -->") {
		t.Error("Expected fitglue:end marker")
	}
}

func TestBuildMarkdownContent_WithFitFile(t *testing.T) {
	event := createTestEvent()
	content := buildMarkdownContent(event, "activity.fit")

	if !contains(content, "fit_file: activity.fit") {
		t.Error("Expected fit_file in frontmatter when FIT file is provided")
	}
}

func TestBuildMarkdownContent_WithoutFitFile(t *testing.T) {
	event := createTestEvent()
	content := buildMarkdownContent(event, "")

	if contains(content, "fit_file") {
		t.Error("Expected no fit_file in frontmatter when FIT file is not provided")
	}
}

func TestBuildFilePath(t *testing.T) {
	event := createTestEvent()
	activityDate := event.StartTime.AsTime()
	path := buildFilePath("workouts/", event, activityDate)

	if path == "" {
		t.Error("Expected non-empty file path")
	}
	if !contains(path, "workouts/") {
		t.Error("Expected path to start with configured folder")
	}
	if !contains(path, "activity.md") {
		t.Error("Expected path to end with activity.md")
	}
}

func TestSanitizeFileName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Morning Run", "morning-run"},
		{"Run/Walk", "run-walk"},
		{"Test's Activity", "tests-activity"},
		{"Hello  World", "hello-world"},
	}

	for _, tc := range tests {
		result := sanitizeFileName(tc.input)
		if result != tc.expected {
			t.Errorf("sanitizeFileName(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestMergeWithUserContent(t *testing.T) {
	newContent := "# New Activity\n\nNew description\n\n<!-- fitglue:end -->\n"
	existingContent := "# Old Activity\n\nOld description\n\n<!-- fitglue:end -->\n\n## My Notes\nUser wrote this"

	merged := mergeWithUserContent(newContent, existingContent)

	if !contains(merged, "New description") {
		t.Error("Expected new description in merged content")
	}
	if !contains(merged, "My Notes") {
		t.Error("Expected user content to be preserved")
	}
	if !contains(merged, "User wrote this") {
		t.Error("Expected user notes to be preserved")
	}
}

func TestMergeWithUserContent_NoMarker(t *testing.T) {
	newContent := "# New Activity\n\nNew description\n\n<!-- fitglue:end -->\n"
	existingContent := "Just some content without marker"

	merged := mergeWithUserContent(newContent, existingContent)

	if merged != newContent {
		t.Error("Expected new content when no marker in existing")
	}
}

func TestLoadGitHubConfig(t *testing.T) {
	event := createTestEvent()
	event.EnrichmentMetadata = map[string]string{
		"github_repo":   "user/fitness-log",
		"github_folder": "workouts/",
	}

	config, err := loadGitHubConfig(event)
	if err != nil {
		t.Fatalf("loadGitHubConfig failed: %v", err)
	}
	if config.Owner != "user" {
		t.Errorf("Expected owner 'user', got %q", config.Owner)
	}
	if config.Name != "fitness-log" {
		t.Errorf("Expected name 'fitness-log', got %q", config.Name)
	}
	if config.Folder != "workouts/" {
		t.Errorf("Expected folder 'workouts/', got %q", config.Folder)
	}
}

func TestLoadGitHubConfig_MissingRepo(t *testing.T) {
	event := createTestEvent()
	event.EnrichmentMetadata = map[string]string{}

	_, err := loadGitHubConfig(event)
	if err == nil {
		t.Error("Expected error for missing repo")
	}
}

func TestLoadGitHubConfig_InvalidRepoFormat(t *testing.T) {
	event := createTestEvent()
	event.EnrichmentMetadata = map[string]string{
		"github_repo": "invalid-no-slash",
	}

	_, err := loadGitHubConfig(event)
	if err == nil {
		t.Error("Expected error for invalid repo format")
	}
}

func TestLoadGitHubConfig_DefaultFolder(t *testing.T) {
	event := createTestEvent()
	event.EnrichmentMetadata = map[string]string{
		"github_repo": "user/repo",
	}

	config, err := loadGitHubConfig(event)
	if err != nil {
		t.Fatalf("loadGitHubConfig failed: %v", err)
	}
	if config.Folder != "workouts/" {
		t.Errorf("Expected default folder 'workouts/', got %q", config.Folder)
	}
}

// --- Test helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func createTestEvent() *pb.EnrichedActivityEvent {
	return &pb.EnrichedActivityEvent{
		ActivityId:   "test-activity-123",
		UserId:       "test-user",
		PipelineId:   "test-pipeline",
		Name:         "Morning Run",
		Description:  "A great morning run",
		ActivityType: pb.ActivityType_ACTIVITY_TYPE_RUN,
		Source:       pb.ActivitySource_SOURCE_STRAVA,
		StartTime:    timestamppb.New(time.Date(2026, 2, 8, 7, 30, 0, 0, time.UTC)),
		EnrichmentMetadata: map[string]string{
			"github_repo":   "user/fitness-log",
			"github_folder": "workouts/",
		},
	}
}
