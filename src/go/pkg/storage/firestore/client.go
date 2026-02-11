package firestore

import (
	"cloud.google.com/go/firestore"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type Client struct {
	fs *firestore.Client
}

func NewClient(client *firestore.Client) *Client {
	return &Client{fs: client}
}

func (c *Client) Close() error {
	return c.fs.Close()
}

func (c *Client) Users() *Collection[pb.UserRecord] {
	return &Collection[pb.UserRecord]{
		Ref:           c.fs.Collection("users"),
		ToFirestore:   UserToFirestore,
		FromFirestore: FirestoreToUser,
	}
}

// Executions returns the legacy root-level collection.
// DEPRECATED: Use UserExecutions(userId) for new code.
// This remains for backward compatibility during migration.
func (c *Client) Executions() *Collection[pb.ExecutionRecord] {
	return &Collection[pb.ExecutionRecord]{
		Ref:           c.fs.Collection("executions"),
		ToFirestore:   ExecutionToFirestore,
		FromFirestore: FirestoreToExecution,
	}
}

// UserExecutions are sub-collections of Users: users/{uid}/executions/{id}
// PREFERRED: Use this instead of Executions() for new code
func (c *Client) UserExecutions(userId string) *Collection[pb.ExecutionRecord] {
	return &Collection[pb.ExecutionRecord]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("executions"),
		ToFirestore:   ExecutionToFirestore,
		FromFirestore: FirestoreToExecution,
	}
}

// OrphanedExecutions stores executions without a userId.
// These are code smells and should be investigated.
// Consider setting up alerts on this collection's write activity.
func (c *Client) OrphanedExecutions() *Collection[pb.ExecutionRecord] {
	return &Collection[pb.ExecutionRecord]{
		Ref:           c.fs.Collection("orphaned_executions"),
		ToFirestore:   ExecutionToFirestore,
		FromFirestore: FirestoreToExecution,
	}
}

func (c *Client) PendingInputs() *Collection[pb.PendingInput] {
	return &Collection[pb.PendingInput]{
		Ref:           c.fs.Collection("pending_inputs"),
		ToFirestore:   PendingInputToFirestore,
		FromFirestore: FirestoreToPendingInput,
	}
}

// UserPendingInputs are sub-collections of Users: users/{uid}/pending_inputs/{id}
// PREFERRED: Use this instead of PendingInputs() for new code
func (c *Client) UserPendingInputs(userId string) *Collection[pb.PendingInput] {
	return &Collection[pb.PendingInput]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("pending_inputs"),
		ToFirestore:   PendingInputToFirestore,
		FromFirestore: FirestoreToPendingInput,
	}
}

// Counters are sub-collections of Users: users/{uid}/counters/{id}
func (c *Client) Counters(userId string) *Collection[pb.Counter] {
	return &Collection[pb.Counter]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("counters"),
		ToFirestore:   CounterToFirestore,
		FromFirestore: FirestoreToCounter,
	}
}

// PersonalRecords are sub-collections of Users: users/{uid}/personal_records/{recordType}
func (c *Client) PersonalRecords(userId string) *Collection[pb.PersonalRecord] {
	return &Collection[pb.PersonalRecord]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("personal_records"),
		ToFirestore:   PersonalRecordToFirestore,
		FromFirestore: FirestoreToPersonalRecord,
	}
}

// ShowcasedActivities is a top-level collection: showcased_activities/{showcase_id}
func (c *Client) ShowcasedActivities() *Collection[pb.ShowcasedActivity] {
	return &Collection[pb.ShowcasedActivity]{
		Ref:           c.fs.Collection("showcased_activities"),
		ToFirestore:   ShowcasedActivityToFirestore,
		FromFirestore: FirestoreToShowcasedActivity,
	}
}

// ShowcaseProfiles is a top-level collection: showcase_profiles/{slug}
func (c *Client) ShowcaseProfiles() *Collection[pb.ShowcaseProfile] {
	return &Collection[pb.ShowcaseProfile]{
		Ref:           c.fs.Collection("showcase_profiles"),
		ToFirestore:   ShowcaseProfileToFirestore,
		FromFirestore: FirestoreToShowcaseProfile,
	}
}

// UploadedActivities are sub-collections of Users: users/{uid}/uploaded_activities/{id}
// Used for loop prevention to track activities we've posted to destinations
func (c *Client) UploadedActivities(userId string) *Collection[pb.UploadedActivityRecord] {
	return &Collection[pb.UploadedActivityRecord]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("uploaded_activities"),
		ToFirestore:   UploadedActivityToFirestore,
		FromFirestore: FirestoreToUploadedActivity,
	}
}

// PipelineRuns are sub-collections of Users: users/{uid}/pipeline_runs/{id}
// Tracks complete pipeline execution lifecycle including boosters and destinations
func (c *Client) PipelineRuns(userId string) *Collection[pb.PipelineRun] {
	return &Collection[pb.PipelineRun]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("pipeline_runs"),
		ToFirestore:   PipelineRunToFirestore,
		FromFirestore: FirestoreToPipelineRun,
	}
}

// PluginDefaults are sub-collections of Users: users/{uid}/plugin_defaults/{pluginId}
// Stores user-level default config for source and destination plugins
func (c *Client) PluginDefaults(userId string) *Collection[pb.PluginDefault] {
	return &Collection[pb.PluginDefault]{
		Ref:           c.fs.Collection("users").Doc(userId).Collection("plugin_defaults"),
		ToFirestore:   PluginDefaultToFirestore,
		FromFirestore: FirestoreToPluginDefault,
	}
}
