package activity

import (
	"context"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
)

// ActivityStore defines the data access contract for activity records and showcases.
type ActivityStore interface {
	GetPipelineRun(ctx context.Context, userID, runID string) (*pbpipeline.PipelineRun, error)
	ListPipelineRuns(ctx context.Context, userID string, limit int32, pageToken string) ([]*pbpipeline.PipelineRun, string, error)
	DeletePipelineRun(ctx context.Context, userID, runID string) error

	GetShowcase(ctx context.Context, userID, showcaseID string) (*pbactivity.ShowcasedActivity, error)
	ListShowcases(ctx context.Context, userID string) ([]*pbactivity.ShowcaseProfileEntry, error)
	CreateShowcase(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error)
	UpdateShowcase(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error)
	DeleteShowcase(ctx context.Context, userID, showcaseID string) error

	// New Showcase Management RPCs
	GetShowcasePreferences(ctx context.Context, userID string) (*pbactivity.ShowcaseProfile, error)
	UpdateShowcasePreferences(ctx context.Context, userID string, prefs *pbactivity.ShowcaseProfile) (*pbactivity.ShowcaseProfile, error)
	GetPublicShowcase(ctx context.Context, showcaseID string) (*pbactivity.ShowcasedActivity, error)
}
