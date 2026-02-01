package database

import (
	"context"
	"strings"

	"cloud.google.com/go/firestore"
	storage "github.com/fitglue/server/src/go/pkg/storage/firestore"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// FirestoreAdapter provides database operations using Firestore
// It wraps our typed storage client
type FirestoreAdapter struct {
	Client  *firestore.Client
	storage *storage.Client // internal typed wrapper
}

func NewFirestoreAdapter(client *firestore.Client) *FirestoreAdapter {
	return &FirestoreAdapter{
		Client:  client, // Keep raw client accessible if needed? OR remove it if unused.
		storage: storage.NewClient(client),
	}
}

func (a *FirestoreAdapter) SetExecution(ctx context.Context, record *pb.ExecutionRecord) error {
	userId := record.GetUserId()
	if userId == "" {
		// ORPHANED: No userId - this is a code smell that should be investigated
		// Store in orphaned_executions collection for alerting
		return a.storage.OrphanedExecutions().Doc(record.ExecutionId).Set(ctx, record)
	}
	// Use typed storage with user sub-collection for direct Firestore client access
	return a.storage.UserExecutions(userId).Doc(record.ExecutionId).Set(ctx, record)
}

func (a *FirestoreAdapter) UpdateExecution(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	if userId == "" {
		// ORPHANED: No userId - update in orphaned_executions collection
		return a.storage.OrphanedExecutions().Doc(id).Update(ctx, data)
	}
	// Use user sub-collection for direct Firestore client access
	return a.storage.UserExecutions(userId).Doc(id).Update(ctx, data)
}

func (a *FirestoreAdapter) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	doc, err := a.storage.Users().Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Manually populate ID since it's the doc key
	doc.UserId = id
	return doc, nil
}

func (a *FirestoreAdapter) UpdateUser(ctx context.Context, id string, data map[string]interface{}) error {
	return a.storage.Users().Doc(id).Update(ctx, data)
}

// --- Sync Count (for tier limits) ---

func (a *FirestoreAdapter) IncrementSyncCount(ctx context.Context, userID string) error {
	_, err := a.Client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "sync_count_this_month", Value: firestore.Increment(1)},
	})
	return err
}

func (a *FirestoreAdapter) IncrementPreventedSyncCount(ctx context.Context, userID string) error {
	_, err := a.Client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "prevented_sync_count", Value: firestore.Increment(1)},
	})
	return err
}

func (a *FirestoreAdapter) ResetSyncCount(ctx context.Context, userID string) error {
	_, err := a.Client.Collection("users").Doc(userID).Update(ctx, []firestore.Update{
		{Path: "sync_count_this_month", Value: 0},
		{Path: "sync_count_reset_at", Value: firestore.ServerTimestamp},
	})
	return err
}

// --- Pending Inputs ---

func (a *FirestoreAdapter) GetPendingInput(ctx context.Context, userId string, id string) (*pb.PendingInput, error) {
	doc, err := a.storage.UserPendingInputs(userId).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *FirestoreAdapter) CreatePendingInput(ctx context.Context, userId string, input *pb.PendingInput) error {
	// Use Set to handle potential retries/race conditions
	// Store in user sub-collection for direct Firestore client access
	return a.storage.UserPendingInputs(userId).Doc(input.ActivityId).Set(ctx, input)
}

func (a *FirestoreAdapter) UpdatePendingInput(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	return a.storage.UserPendingInputs(userId).Doc(id).Update(ctx, data)
}

func (a *FirestoreAdapter) ListPendingInputs(ctx context.Context, userID string) ([]*pb.PendingInput, error) {
	// Query user sub-collection directly - no need for where clause on user_id
	iter := a.Client.Collection("users").Doc(userID).Collection("pending_inputs").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var results []*pb.PendingInput
	for _, d := range docs {
		// Manually convert using our converter
		m := d.Data()
		p := storage.FirestoreToPendingInput(m)
		// Ensure ActivityID is set from doc ID if missing (though it should be in data)
		if p.ActivityId == "" {
			p.ActivityId = d.Ref.ID
		}
		results = append(results, p)
	}
	return results, nil
}

// --- Counters ---

func (a *FirestoreAdapter) GetCounter(ctx context.Context, userId string, id string) (*pb.Counter, error) {
	doc, err := a.storage.Counters(userId).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	doc.Id = id
	return doc, nil
}

func (a *FirestoreAdapter) SetCounter(ctx context.Context, userId string, counter *pb.Counter) error {
	// Set (overwrite/create)
	return a.storage.Counters(userId).Doc(counter.Id).Set(ctx, counter)
}

// ListCounters returns all counters for a user
func (a *FirestoreAdapter) ListCounters(ctx context.Context, userId string) ([]*pb.Counter, error) {
	iter := a.Client.Collection("users").Doc(userId).Collection("counters").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var counters []*pb.Counter
	for _, d := range docs {
		m := d.Data()
		counter := storage.FirestoreToCounter(m)
		if counter.Id == "" {
			counter.Id = d.Ref.ID
		}
		counters = append(counters, counter)
	}
	return counters, nil
}

// DeleteCounter removes a counter by ID
func (a *FirestoreAdapter) DeleteCounter(ctx context.Context, userId string, id string) error {
	_, err := a.Client.Collection("users").Doc(userId).Collection("counters").Doc(id).Delete(ctx)
	return err
}

// --- Personal Records ---

// GetPersonalRecord retrieves a personal record by type
func (a *FirestoreAdapter) GetPersonalRecord(ctx context.Context, userId string, recordType string) (*pb.PersonalRecord, error) {
	doc, err := a.storage.PersonalRecords(userId).Doc(recordType).Get(ctx)
	if err != nil {
		return nil, err
	}
	doc.RecordType = recordType
	return doc, nil
}

// SetPersonalRecord creates or updates a personal record
func (a *FirestoreAdapter) SetPersonalRecord(ctx context.Context, userId string, record *pb.PersonalRecord) error {
	return a.storage.PersonalRecords(userId).Doc(record.RecordType).Set(ctx, record)
}

// ListPersonalRecords returns all personal records for a user
func (a *FirestoreAdapter) ListPersonalRecords(ctx context.Context, userId string) ([]*pb.PersonalRecord, error) {
	iter := a.Client.Collection("users").Doc(userId).Collection("personal_records").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var records []*pb.PersonalRecord
	for _, d := range docs {
		m := d.Data()
		record := storage.FirestoreToPersonalRecord(m)
		if record.RecordType == "" {
			record.RecordType = d.Ref.ID
		}
		records = append(records, record)
	}
	return records, nil
}

// DeletePersonalRecord removes a personal record by type
func (a *FirestoreAdapter) DeletePersonalRecord(ctx context.Context, userId string, recordType string) error {
	_, err := a.Client.Collection("users").Doc(userId).Collection("personal_records").Doc(recordType).Delete(ctx)
	return err
}

// --- Activities ---

func (a *FirestoreAdapter) SetSynchronizedActivity(ctx context.Context, userId string, activity *pb.SynchronizedActivity) error {
	// Set the synchronized activity document
	if err := a.storage.Activities(userId).Doc(activity.ActivityId).Set(ctx, activity); err != nil {
		return err
	}

	// PHASE 2 OPTIMIZATION: Atomically increment activity counters for O(1) stats access
	// This enables the frontend to show activity counts without expensive count() queries
	_, err := a.Client.Collection("users").Doc(userId).Update(ctx, []firestore.Update{
		{Path: "activityCounts.synchronized", Value: firestore.Increment(1)},
		{Path: "activityCounts.weeklySync", Value: firestore.Increment(1)},
		{Path: "activityCounts.lastUpdated", Value: firestore.ServerTimestamp},
	})
	// Don't fail if counters update fails - the activity was already created
	if err != nil {
		// Log but don't return error - activity creation succeeded
		// The counters can be backfilled later if needed
	}

	return nil
}

// ListPendingParkrunActivities queries all users' activities with pending Parkrun results
func (a *FirestoreAdapter) ListPendingParkrunActivities(ctx context.Context) ([]*pb.SynchronizedActivity, []string, error) {
	// Query across ALL users' activities subcollections using collection group query
	// Filter by parkrun_results_state == PARKRUN_RESULTS_STATE_PENDING (2)
	iter := a.Client.CollectionGroup("activities").
		Where("parkrun_results_state", "==", int32(pb.ParkrunResultsState_PARKRUN_RESULTS_STATE_PENDING)).
		Documents(ctx)

	docs, err := iter.GetAll()
	if err != nil {
		return nil, nil, err
	}

	var activities []*pb.SynchronizedActivity
	var userIDs []string

	for _, d := range docs {
		// Extract user ID from path: users/{userId}/activities/{activityId}
		pathParts := d.Ref.Parent.Parent.ID // This gets us the userId
		userID := pathParts

		m := d.Data()
		activity := storage.FirestoreToSynchronizedActivity(m)
		if activity.ActivityId == "" {
			activity.ActivityId = d.Ref.ID
		}
		activities = append(activities, activity)
		userIDs = append(userIDs, userID)
	}

	return activities, userIDs, nil
}

// UpdateSynchronizedActivity updates specific fields on an activity
func (a *FirestoreAdapter) UpdateSynchronizedActivity(ctx context.Context, userId string, activityId string, data map[string]interface{}) error {
	return a.storage.Activities(userId).Doc(activityId).Update(ctx, data)
}

// GetSynchronizedActivity retrieves a single synchronized activity
func (a *FirestoreAdapter) GetSynchronizedActivity(ctx context.Context, userId string, activityId string) (*pb.SynchronizedActivity, error) {
	activity, err := a.storage.Activities(userId).Doc(activityId).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Ensure activity ID is set
	if activity != nil && activity.ActivityId == "" {
		activity.ActivityId = activityId
	}
	return activity, nil
}

func (a *FirestoreAdapter) ListPendingInputsByEnricher(ctx context.Context, enricherId string, status pb.PendingInput_Status) ([]*pb.PendingInput, error) {
	// Query across all pending inputs using collection group query
	iter := a.Client.CollectionGroup("pending_inputs").
		Where("enricher_provider_id", "==", enricherId).
		Where("status", "==", int32(status)).
		Documents(ctx)

	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	var inputs []*pb.PendingInput
	for _, d := range docs {
		m := d.Data()
		input := storage.FirestoreToPendingInput(m)
		if input.ActivityId == "" {
			input.ActivityId = d.Ref.ID
		}
		inputs = append(inputs, input)
	}

	return inputs, nil
}

// --- Showcased Activities (public shareable snapshots) ---

// ShowcaseActivityExists checks if a showcase ID already exists
func (a *FirestoreAdapter) ShowcaseActivityExists(ctx context.Context, showcaseId string) (bool, error) {
	_, err := a.storage.ShowcasedActivities().Doc(showcaseId).Ref.Get(ctx)
	if err != nil {
		// Check if it's a "not found" error
		if err.Error() == "rpc error: code = NotFound desc = Document not found" ||
			err.Error() == "document not found" ||
			isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// SetShowcasedActivity creates or updates a showcased activity
func (a *FirestoreAdapter) SetShowcasedActivity(ctx context.Context, activity *pb.ShowcasedActivity) error {
	return a.storage.ShowcasedActivities().Doc(activity.ShowcaseId).Set(ctx, activity)
}

// GetShowcasedActivity retrieves a showcased activity by ID
func (a *FirestoreAdapter) GetShowcasedActivity(ctx context.Context, showcaseId string) (*pb.ShowcasedActivity, error) {
	activity, err := a.storage.ShowcasedActivities().Doc(showcaseId).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Ensure showcase ID is set
	if activity != nil && activity.ShowcaseId == "" {
		activity.ShowcaseId = showcaseId
	}
	return activity, nil
}

// isNotFoundError checks if error is a Firestore not found error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "NotFound") || strings.Contains(errStr, "not found")
}

// --- Pipelines (Sub-collection) ---

// GetUserPipelines retrieves all pipelines for a user from the sub-collection
func (a *FirestoreAdapter) GetUserPipelines(ctx context.Context, userId string) ([]*pb.PipelineConfig, error) {
	iter := a.Client.Collection("users").Doc(userId).Collection("pipelines").Documents(ctx)
	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	pipelines := make([]*pb.PipelineConfig, len(docs))
	for i, doc := range docs {
		pipelines[i] = storage.FirestoreToPipeline(doc.Data())
		// Ensure ID is set from doc ID if missing
		if pipelines[i].Id == "" {
			pipelines[i].Id = doc.Ref.ID
		}
	}

	return pipelines, nil
}

// --- Uploaded Activities (for loop prevention) ---

// SetUploadedActivity records that an activity was uploaded to a destination.
// Used for loop prevention: when a webhook comes back, we check if we just uploaded it.
func (a *FirestoreAdapter) SetUploadedActivity(ctx context.Context, userId string, record *pb.UploadedActivityRecord) error {
	return a.storage.UploadedActivities(userId).Doc(record.Id).Set(ctx, record)
}

// GetUploadedActivity retrieves an uploaded activity record by destination and destination ID.
// This matches how webhooks arrive: when Hevy sends a webhook, we look up by HEVY:{hevy_workout_id}
func (a *FirestoreAdapter) GetUploadedActivity(ctx context.Context, userId string, destination pb.Destination, destinationId string) (*pb.UploadedActivityRecord, error) {
	// Query for the record with matching destination and destination_id
	iter := a.Client.Collection("users").Doc(userId).Collection("uploaded_activities").
		Where("destination", "==", int32(destination)).
		Where("destination_id", "==", destinationId).
		Limit(1).
		Documents(ctx)

	docs, err := iter.GetAll()
	if err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return nil, nil // Not found - not an error, just no match
	}

	m := docs[0].Data()
	record := storage.FirestoreToUploadedActivity(m)
	if record.Id == "" {
		record.Id = docs[0].Ref.ID
	}
	return record, nil
}

// --- Pipeline Runs (lifecycle tracking) ---

// CreatePipelineRun creates a new pipeline run document
func (a *FirestoreAdapter) CreatePipelineRun(ctx context.Context, userId string, run *pb.PipelineRun) error {
	return a.storage.PipelineRuns(userId).Doc(run.Id).Set(ctx, run)
}

// GetPipelineRun retrieves a pipeline run by ID
func (a *FirestoreAdapter) GetPipelineRun(ctx context.Context, userId string, id string) (*pb.PipelineRun, error) {
	run, err := a.storage.PipelineRuns(userId).Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	// Ensure ID is set
	if run != nil && run.Id == "" {
		run.Id = id
	}
	return run, nil
}

// UpdatePipelineRun updates specific fields on a pipeline run
func (a *FirestoreAdapter) UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	return a.storage.PipelineRuns(userId).Doc(id).Update(ctx, data)
}
