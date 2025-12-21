package shared

const (
	ProjectID = "fitglue-project" // Can be overridden by env var in main if needed

	TopicRawActivity      = "topic-raw-activity"
	TopicEnrichedActivity = "topic-enriched-activity"
	TopicJobUploadStrava  = "topic-job-upload-strava"

	CollectionUsers      = "users"
	CollectionCursors    = "cursors"
	CollectionExecutions = "executions"
	CollectionCache      = "cache"
)
