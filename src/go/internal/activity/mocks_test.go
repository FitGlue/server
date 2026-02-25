package activity

import (
	"context"

	pbactivity "github.com/fitglue/server/src/go/pkg/types/pb/models/activity"
	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
)

// MockActivityStore implements ActivityStore for testing
type MockActivityStore struct {
	GetPipelineRunFunc    func(ctx context.Context, userID, runID string) (*pbpipeline.PipelineRun, error)
	ListPipelineRunsFunc  func(ctx context.Context, userID string, limit int32, pageToken string) ([]*pbpipeline.PipelineRun, string, error)
	DeletePipelineRunFunc func(ctx context.Context, userID, runID string) error

	GetShowcaseFunc    func(ctx context.Context, userID, showcaseID string) (*pbactivity.ShowcasedActivity, error)
	ListShowcasesFunc  func(ctx context.Context, userID string) ([]*pbactivity.ShowcaseProfileEntry, error)
	CreateShowcaseFunc func(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error)
	UpdateShowcaseFunc func(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error)
	DeleteShowcaseFunc func(ctx context.Context, userID, showcaseID string) error

	GetShowcasePreferencesFunc    func(ctx context.Context, userID string) (*pbactivity.ShowcaseProfile, error)
	UpdateShowcasePreferencesFunc func(ctx context.Context, userID string, prefs *pbactivity.ShowcaseProfile) (*pbactivity.ShowcaseProfile, error)
	GetPublicShowcaseFunc         func(ctx context.Context, showcaseID string) (*pbactivity.ShowcasedActivity, error)
}

func (m *MockActivityStore) GetPipelineRun(ctx context.Context, userID, runID string) (*pbpipeline.PipelineRun, error) {
	if m.GetPipelineRunFunc != nil {
		return m.GetPipelineRunFunc(ctx, userID, runID)
	}
	return nil, nil
}

func (m *MockActivityStore) ListPipelineRuns(ctx context.Context, userID string, limit int32, pageToken string) ([]*pbpipeline.PipelineRun, string, error) {
	if m.ListPipelineRunsFunc != nil {
		return m.ListPipelineRunsFunc(ctx, userID, limit, pageToken)
	}
	return nil, "", nil
}

func (m *MockActivityStore) DeletePipelineRun(ctx context.Context, userID, runID string) error {
	if m.DeletePipelineRunFunc != nil {
		return m.DeletePipelineRunFunc(ctx, userID, runID)
	}
	return nil
}

func (m *MockActivityStore) GetShowcase(ctx context.Context, userID, showcaseID string) (*pbactivity.ShowcasedActivity, error) {
	if m.GetShowcaseFunc != nil {
		return m.GetShowcaseFunc(ctx, userID, showcaseID)
	}
	return nil, nil
}

func (m *MockActivityStore) ListShowcases(ctx context.Context, userID string) ([]*pbactivity.ShowcaseProfileEntry, error) {
	if m.ListShowcasesFunc != nil {
		return m.ListShowcasesFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockActivityStore) CreateShowcase(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error) {
	if m.CreateShowcaseFunc != nil {
		return m.CreateShowcaseFunc(ctx, userID, showcase)
	}
	return showcase, nil
}

func (m *MockActivityStore) UpdateShowcase(ctx context.Context, userID string, showcase *pbactivity.ShowcasedActivity) (*pbactivity.ShowcasedActivity, error) {
	if m.UpdateShowcaseFunc != nil {
		return m.UpdateShowcaseFunc(ctx, userID, showcase)
	}
	return showcase, nil
}

func (m *MockActivityStore) DeleteShowcase(ctx context.Context, userID, showcaseID string) error {
	if m.DeleteShowcaseFunc != nil {
		return m.DeleteShowcaseFunc(ctx, userID, showcaseID)
	}
	return nil
}

func (m *MockActivityStore) GetShowcasePreferences(ctx context.Context, userID string) (*pbactivity.ShowcaseProfile, error) {
	if m.GetShowcasePreferencesFunc != nil {
		return m.GetShowcasePreferencesFunc(ctx, userID)
	}
	return nil, nil
}

func (m *MockActivityStore) UpdateShowcasePreferences(ctx context.Context, userID string, prefs *pbactivity.ShowcaseProfile) (*pbactivity.ShowcaseProfile, error) {
	if m.UpdateShowcasePreferencesFunc != nil {
		return m.UpdateShowcasePreferencesFunc(ctx, userID, prefs)
	}
	return prefs, nil
}

func (m *MockActivityStore) GetPublicShowcase(ctx context.Context, showcaseID string) (*pbactivity.ShowcasedActivity, error) {
	if m.GetPublicShowcaseFunc != nil {
		return m.GetPublicShowcaseFunc(ctx, showcaseID)
	}
	return nil, nil
}

// MockBlobStore implements BlobStore for testing
type MockBlobStore struct {
	GetFunc    func(ctx context.Context, bucket, object string) ([]byte, error)
	DeleteFunc func(ctx context.Context, bucket, object string) error
	WriteFunc  func(ctx context.Context, bucket, object string, data []byte) error
}

func (m *MockBlobStore) Get(ctx context.Context, bucket, object string) ([]byte, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, bucket, object)
	}
	return nil, nil
}

func (m *MockBlobStore) Delete(ctx context.Context, bucket, object string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, bucket, object)
	}
	return nil
}

func (m *MockBlobStore) Write(ctx context.Context, bucket, object string, data []byte) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(ctx, bucket, object, data)
	}
	return nil
}

// MockPublisher implements Publisher for testing
type MockPublisher struct {
	PublishCloudEventFunc func(ctx context.Context, topic string, e interface{}) (string, error)
}

func (m *MockPublisher) PublishCloudEvent(ctx context.Context, topic string, e interface{}) (string, error) {
	if m.PublishCloudEventFunc != nil {
		return m.PublishCloudEventFunc(ctx, topic, e)
	}
	return "test-msg-id", nil
}
