package destination

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// --- Mock Database ---

type MockDatabase struct {
	Outcomes       []*pb.DestinationOutcome
	SetOutcomeFunc func(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error
	UpdateRunFunc  func(ctx context.Context, userId string, id string, data map[string]interface{}) error
	GetUserFunc    func(ctx context.Context, id string) (*pb.UserRecord, error)
}

func (m *MockDatabase) SetDestinationOutcome(ctx context.Context, userId string, pipelineRunId string, outcome *pb.DestinationOutcome) error {
	if m.SetOutcomeFunc != nil {
		return m.SetOutcomeFunc(ctx, userId, pipelineRunId, outcome)
	}
	m.Outcomes = append(m.Outcomes, outcome)
	return nil
}

func (m *MockDatabase) GetDestinationOutcomes(ctx context.Context, userId string, pipelineRunId string) ([]*pb.DestinationOutcome, error) {
	return m.Outcomes, nil
}

func (m *MockDatabase) UpdatePipelineRun(ctx context.Context, userId string, id string, data map[string]interface{}) error {
	if m.UpdateRunFunc != nil {
		return m.UpdateRunFunc(ctx, userId, id, data)
	}
	return nil
}

func (m *MockDatabase) GetUser(ctx context.Context, id string) (*pb.UserRecord, error) {
	if m.GetUserFunc != nil {
		return m.GetUserFunc(ctx, id)
	}
	return nil, fmt.Errorf("no user")
}

// --- Mock NotificationService ---

type MockNotifications struct {
	Sent []NotificationRecord
}

type NotificationRecord struct {
	UserID string
	Title  string
	Body   string
	Tokens []string
	Data   map[string]string
}

func (m *MockNotifications) SendPushNotification(ctx context.Context, userID string, title, body string, tokens []string, data map[string]string) error {
	m.Sent = append(m.Sent, NotificationRecord{
		UserID: userID,
		Title:  title,
		Body:   body,
		Tokens: tokens,
		Data:   data,
	})
	return nil
}

// --- Tests ---

func TestComputePipelineRunStatus_Empty(t *testing.T) {
	status := ComputePipelineRunStatus(nil)
	if status != pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING {
		t.Errorf("expected RUNNING for empty outcomes, got %v", status)
	}
}

func TestComputePipelineRunStatus_AllSuccess(t *testing.T) {
	outcomes := []*pb.DestinationOutcome{
		{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		{Destination: pb.Destination_DESTINATION_HEVY, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
	}
	status := ComputePipelineRunStatus(outcomes)
	if status != pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED {
		t.Errorf("expected SYNCED, got %v", status)
	}
}

func TestComputePipelineRunStatus_SomePending(t *testing.T) {
	outcomes := []*pb.DestinationOutcome{
		{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		{Destination: pb.Destination_DESTINATION_HEVY, Status: pb.DestinationStatus_DESTINATION_STATUS_PENDING},
	}
	status := ComputePipelineRunStatus(outcomes)
	if status != pb.PipelineRunStatus_PIPELINE_RUN_STATUS_RUNNING {
		t.Errorf("expected RUNNING, got %v", status)
	}
}

func TestComputePipelineRunStatus_SomeFailed(t *testing.T) {
	outcomes := []*pb.DestinationOutcome{
		{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		{Destination: pb.Destination_DESTINATION_HEVY, Status: pb.DestinationStatus_DESTINATION_STATUS_FAILED},
	}
	status := ComputePipelineRunStatus(outcomes)
	if status != pb.PipelineRunStatus_PIPELINE_RUN_STATUS_PARTIAL {
		t.Errorf("expected PARTIAL, got %v", status)
	}
}

func TestComputePipelineRunStatus_SuccessAndSkipped(t *testing.T) {
	outcomes := []*pb.DestinationOutcome{
		{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		{Destination: pb.Destination_DESTINATION_HEVY, Status: pb.DestinationStatus_DESTINATION_STATUS_SKIPPED},
	}
	status := ComputePipelineRunStatus(outcomes)
	if status != pb.PipelineRunStatus_PIPELINE_RUN_STATUS_SYNCED {
		t.Errorf("expected SYNCED (skipped doesn't count as failure), got %v", status)
	}
}

func TestUpdateStatus_SendsNotificationOnSynced(t *testing.T) {
	notifications := &MockNotifications{}
	db := &MockDatabase{
		// Pre-populate with one already-complete destination
		Outcomes: []*pb.DestinationOutcome{
			{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				FcmTokens: []string{"token1"},
			}, nil
		},
	}
	logger := slog.Default()

	// When Hevy also succeeds → all complete → SYNCED → notification should fire
	UpdateStatus(context.Background(), db, notifications, "user1", "run1",
		pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		"hevy-123", "", "Morning Run", logger)

	if len(notifications.Sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications.Sent))
	}
	n := notifications.Sent[0]
	if n.Title != "Activity Synced: Morning Run" {
		t.Errorf("unexpected title: %s", n.Title)
	}
	if !strings.Contains(n.Body, "Strava") || !strings.Contains(n.Body, "Hevy") {
		t.Errorf("expected body to mention both destinations, got: %s", n.Body)
	}
	if n.Data["type"] != "PIPELINE_SYNCED" {
		t.Errorf("expected PIPELINE_SYNCED type, got: %s", n.Data["type"])
	}
}

func TestUpdateStatus_SendsNotificationOnPartial(t *testing.T) {
	notifications := &MockNotifications{}
	db := &MockDatabase{
		Outcomes: []*pb.DestinationOutcome{
			{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_SUCCESS},
		},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				FcmTokens: []string{"token1"},
			}, nil
		},
	}
	logger := slog.Default()

	// When Hevy fails → PARTIAL → notification should fire with failure info
	UpdateStatus(context.Background(), db, notifications, "user1", "run1",
		pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_FAILED,
		"", "API error", "Morning Run", logger)

	if len(notifications.Sent) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notifications.Sent))
	}
	n := notifications.Sent[0]
	if n.Title != "Partial Sync: Morning Run" {
		t.Errorf("unexpected title: %s", n.Title)
	}
	if !strings.Contains(n.Body, "Hevy") || !strings.Contains(n.Body, "failed") {
		t.Errorf("expected body to mention Hevy failure, got: %s", n.Body)
	}
	if n.Data["type"] != "PIPELINE_PARTIAL" {
		t.Errorf("expected PIPELINE_PARTIAL type, got: %s", n.Data["type"])
	}
}

func TestUpdateStatus_NoNotificationWhileRunning(t *testing.T) {
	notifications := &MockNotifications{}
	db := &MockDatabase{
		// One destination still PENDING
		Outcomes: []*pb.DestinationOutcome{
			{Destination: pb.Destination_DESTINATION_STRAVA, Status: pb.DestinationStatus_DESTINATION_STATUS_PENDING},
		},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				FcmTokens: []string{"token1"},
			}, nil
		},
	}
	logger := slog.Default()

	// Only Hevy completes — Strava still pending → RUNNING → no notification
	UpdateStatus(context.Background(), db, notifications, "user1", "run1",
		pb.Destination_DESTINATION_HEVY, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		"hevy-123", "", "Morning Run", logger)

	if len(notifications.Sent) != 0 {
		t.Fatalf("expected 0 notifications while RUNNING, got %d", len(notifications.Sent))
	}
}

func TestUpdateStatus_NoNotificationWhenPrefsDisabled(t *testing.T) {
	notifications := &MockNotifications{}
	db := &MockDatabase{
		// Only one destination, so it goes to SYNCED immediately
		Outcomes: []*pb.DestinationOutcome{},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				FcmTokens: []string{"token1"},
				NotificationPreferences: &pb.NotificationPreferences{
					NotifyPipelineSuccess: false,
				},
			}, nil
		},
	}
	logger := slog.Default()

	UpdateStatus(context.Background(), db, notifications, "user1", "run1",
		pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		"strava-123", "", "Morning Run", logger)

	if len(notifications.Sent) != 0 {
		t.Fatalf("expected 0 notifications when prefs disabled, got %d", len(notifications.Sent))
	}
}

func TestUpdateStatus_NilNotificationService(t *testing.T) {
	db := &MockDatabase{
		Outcomes: []*pb.DestinationOutcome{},
		GetUserFunc: func(ctx context.Context, id string) (*pb.UserRecord, error) {
			return &pb.UserRecord{
				FcmTokens: []string{"token1"},
			}, nil
		},
	}
	logger := slog.Default()

	// Should not panic with nil notifications
	UpdateStatus(context.Background(), db, nil, "user1", "run1",
		pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		"strava-123", "", "Morning Run", logger)
}

func TestUpdateStatus_NoPipelineRunId(t *testing.T) {
	notifications := &MockNotifications{}
	db := &MockDatabase{}
	logger := slog.Default()

	// Empty pipelineRunId → early return (legacy flow)
	UpdateStatus(context.Background(), db, notifications, "user1", "",
		pb.Destination_DESTINATION_STRAVA, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		"strava-123", "", "Morning Run", logger)

	if len(notifications.Sent) != 0 {
		t.Fatalf("expected 0 notifications for empty pipelineRunId, got %d", len(notifications.Sent))
	}
}

func TestFormatDestinationName(t *testing.T) {
	tests := []struct {
		dest     pb.Destination
		expected string
	}{
		{pb.Destination_DESTINATION_STRAVA, "Strava"},
		{pb.Destination_DESTINATION_HEVY, "Hevy"},
		{pb.Destination_DESTINATION_SHOWCASE, "Showcase"},
		{pb.Destination_DESTINATION_TRAININGPEAKS, "TrainingPeaks"},
		{pb.Destination_DESTINATION_INTERVALS, "Intervals.icu"},
		{pb.Destination_DESTINATION_GOOGLESHEETS, "Google Sheets"},
		{pb.Destination_DESTINATION_MOCK, "Mock"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := FormatDestinationName(tt.dest)
			if got != tt.expected {
				t.Errorf("FormatDestinationName(%v) = %q, want %q", tt.dest, got, tt.expected)
			}
		})
	}
}
