package shared

const (
	ProjectID = "fitglue-project" // Can be overridden by env var in main if needed

	TopicRawActivity           = "topic-raw-activity"
	TopicActivityEnrichment    = "topic-activity-enrichment" // Enricher pipeline entry point
	TopicEnrichedActivity      = "topic-enriched-activity"
	TopicJobUploadStrava       = "topic-job-upload-strava"
	TopicFitbitUpdates         = "topic-fitbit-updates"
	TopicEnrichmentLag         = "topic-enrichment-lag"
	TopicParkrunResultsTrigger = "topic-parkrun-results-trigger"

	CollectionUsers      = "users"
	CollectionCursors    = "cursors"
	CollectionExecutions = "executions"
)
