package githubuploader

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/description"
	"github.com/fitglue/server/src/go/pkg/destination"
	"github.com/fitglue/server/src/go/pkg/domain/activity"
	"github.com/fitglue/server/src/go/pkg/framework"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	ghclient "github.com/fitglue/server/src/go/pkg/integrations/github"
	"github.com/fitglue/server/src/go/pkg/loopprevention"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

var (
	svc     *bootstrap.Service
	svcOnce sync.Once
	svcErr  error
)

func init() {
	functions.CloudEvent("UploadToGitHub", UploadToGitHub)
}

func initService(ctx context.Context) (*bootstrap.Service, error) {
	if svc != nil {
		return svc, nil
	}
	svcOnce.Do(func() {
		baseSvc, err := bootstrap.NewService(ctx)
		if err != nil {
			svcErr = err
			return
		}
		svc = baseSvc
	})
	return svc, svcErr
}

// UploadToGitHub is the entry point
func UploadToGitHub(ctx context.Context, e event.Event) error {
	svc, err := initService(ctx)
	if err != nil {
		return fmt.Errorf("service init failed: %v", err)
	}
	return framework.WrapCloudEvent("github-uploader", svc, uploadHandler(nil))(ctx, e)
}

// uploadHandler contains the business logic
// httpClient can be injected for testing; if nil, creates OAuth client
func uploadHandler(httpClient *http.Client) framework.HandlerFunc {
	return func(ctx context.Context, e event.Event, fwCtx *framework.FrameworkContext) (interface{}, error) {
		var eventPayload pb.EnrichedActivityEvent

		unmarshaler := protojson.UnmarshalOptions{
			DiscardUnknown: true,
			AllowPartial:   true,
		}
		if err := unmarshaler.Unmarshal(e.Data(), &eventPayload); err != nil {
			return nil, fmt.Errorf("protojson.Unmarshal: %w", err)
		}

		// Resolve activity data from GCS if needed
		if err := activity.ResolveEnrichedEvent(ctx, &eventPayload, fwCtx.Service.Store); err != nil {
			fwCtx.Logger.Warn("Failed to resolve activity data from GCS", "error", err)
		}

		fwCtx.Logger.Info("Starting GitHub upload",
			"activity_id", eventPayload.ActivityId,
			"pipeline_id", eventPayload.PipelineId)

		// Initialize OAuth HTTP Client if not provided (for testing)
		if httpClient == nil {
			tokenSource := oauth.NewFirestoreTokenSource(fwCtx.Service, eventPayload.UserId, "github")
			httpClient = oauth.NewClientWithUsageTracking(tokenSource, fwCtx.Service, eventPayload.UserId, "github")
		}

		// Create typed GitHub API client
		ghClient, err := ghclient.NewClientWithResponses("https://api.github.com",
			ghclient.WithHTTPClient(httpClient),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub client: %w", err)
		}

		// Load GitHub-specific config from enrichmentMetadata
		// These fields are injected by the pipeline splitter from the user's destination config
		config, err := loadGitHubConfig(&eventPayload)
		if err != nil {
			destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
				pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_FAILED,
				"", fmt.Sprintf("config error: %s", err), eventPayload.Name, fwCtx.Logger)
			return nil, fmt.Errorf("failed to load GitHub config: %w", err)
		}

		// Check if this is an UPDATE operation
		if useUpdate, ok := eventPayload.EnrichmentMetadata["use_update_method"]; ok && useUpdate == "true" {
			return handleGithubUpdate(ctx, ghClient, &eventPayload, config, fwCtx)
		}

		// --- CREATE MODE ---
		return handleGithubCreate(ctx, ghClient, &eventPayload, config, fwCtx)
	}
}

type gitHubConfig struct {
	Repo   string // "owner/repo"
	Folder string // e.g. "workouts/"
	Owner  string // Parsed from Repo
	Name   string // Parsed from Repo
}

func loadGitHubConfig(eventPayload *pb.EnrichedActivityEvent) (*gitHubConfig, error) {
	repo := ""
	folder := "workouts/"

	if r, ok := eventPayload.EnrichmentMetadata["github_repo"]; ok {
		repo = r
	}
	if f, ok := eventPayload.EnrichmentMetadata["github_folder"]; ok && f != "" {
		folder = f
	}
	if repo == "" {
		return nil, fmt.Errorf("github_repo not configured in metadata")
	}
	if !strings.HasSuffix(folder, "/") {
		folder += "/"
	}

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format: %s (expected owner/repo)", repo)
	}

	return &gitHubConfig{
		Repo:   repo,
		Folder: folder,
		Owner:  parts[0],
		Name:   parts[1],
	}, nil
}

// handleGithubCreate commits a new activity file to GitHub
func handleGithubCreate(ctx context.Context, ghClient *ghclient.ClientWithResponses, eventPayload *pb.EnrichedActivityEvent, config *gitHubConfig, fwCtx *framework.FrameworkContext) (interface{}, error) {
	// Build the Markdown content
	markdownContent := buildMarkdownContent(eventPayload)

	// Build the file path
	activityDate := time.Now()
	if eventPayload.StartTime != nil {
		activityDate = eventPayload.StartTime.AsTime()
	}
	filePath := buildFilePath(config.Folder, eventPayload, activityDate)

	fwCtx.Logger.Info("Creating file in GitHub",
		"repo", config.Repo,
		"path", filePath,
		"content_length", len(markdownContent),
	)

	// Create the file via GitHub Contents API
	commitMessage := fmt.Sprintf("Add %s — %s", eventPayload.Name, activityDate.Format("2006-01-02"))
	externalID, err := createOrUpdateFile(ctx, ghClient, config, filePath, markdownContent, commitMessage, nil)
	if err != nil {
		fwCtx.Logger.Error("GitHub file creation failed", "error", err)
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
			pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_FAILED,
			"", fmt.Sprintf("API error: %s", err), eventPayload.Name, fwCtx.Logger)
		return nil, fmt.Errorf("GitHub create failed: %w", err)
	}

	// Record upload for loop prevention
	uploadRecord := &pb.UploadedActivityRecord{
		Id:            loopprevention.BuildUploadedActivityID(pb.Destination_DESTINATION_GITHUB, externalID),
		UserId:        eventPayload.UserId,
		Source:        eventPayload.Source,
		ExternalId:    eventPayload.ActivityData.GetExternalId(),
		StartTime:     eventPayload.StartTime,
		Destination:   pb.Destination_DESTINATION_GITHUB,
		DestinationId: externalID,
		UploadedAt:    timestamppb.Now(),
	}
	if err := svc.DB.SetUploadedActivity(ctx, eventPayload.UserId, uploadRecord); err != nil {
		fwCtx.Logger.Warn("Failed to record uploaded activity for loop prevention", "error", err)
	}

	// Increment sync count for billing
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err, "userId", eventPayload.UserId)
	}

	// Update PipelineRun destination as synced
	destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
		pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		externalID, "", eventPayload.Name, fwCtx.Logger)

	return map[string]interface{}{
		"status":        "SUCCESS",
		"github_commit": externalID,
		"file_path":     filePath,
		"activity_id":   eventPayload.ActivityId,
		"pipeline_id":   eventPayload.PipelineId,
		"activity_name": eventPayload.Name,
		"activity_type": eventPayload.ActivityType.String(),
		"description":   eventPayload.Description,
		"mode":          "CREATE",
	}, nil
}

// handleGithubUpdate updates an existing activity file in GitHub
func handleGithubUpdate(ctx context.Context, ghClient *ghclient.ClientWithResponses, eventPayload *pb.EnrichedActivityEvent, config *gitHubConfig, fwCtx *framework.FrameworkContext) (interface{}, error) {
	fwCtx.Logger.Info("Starting GitHub UPDATE",
		"activity_id", eventPayload.ActivityId,
		"user_id", eventPayload.UserId)

	// Lookup PipelineRun to get the GitHub external ID (commit SHA / file path)
	pipelineRun, err := svc.DB.GetPipelineRunByActivityId(ctx, eventPayload.UserId, eventPayload.ActivityId)
	if err != nil || pipelineRun == nil {
		fwCtx.Logger.Info("Pipeline run not found for UPDATE - skipping",
			"activity_id", eventPayload.ActivityId, "error", err)
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
			pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED,
			"", "activity_not_found", eventPayload.Name, fwCtx.Logger)
		return map[string]interface{}{
			"status":      "SKIPPED",
			"skip_reason": "activity_not_found",
			"activity_id": eventPayload.ActivityId,
			"mode":        "UPDATE",
		}, nil
	}

	// Find the GitHub destination external ID (file path)
	var existingFilePath string
	for _, dest := range pipelineRun.Destinations {
		if dest.Destination == pb.Destination_DESTINATION_GITHUB && dest.ExternalId != nil && *dest.ExternalId != "" {
			existingFilePath = *dest.ExternalId
			break
		}
	}

	if existingFilePath == "" {
		fwCtx.Logger.Info("No GitHub destination in pipeline run for UPDATE - skipping")
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
			pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_SKIPPED,
			"", "no_github_destination", eventPayload.Name, fwCtx.Logger)
		return map[string]interface{}{
			"status":      "SKIPPED",
			"skip_reason": "no_github_destination",
			"mode":        "UPDATE",
		}, nil
	}

	// Fetch existing file to get its SHA (required for update)
	existingSHA, existingContent, err := getFileContent(ctx, ghClient, config, existingFilePath)
	if err != nil {
		fwCtx.Logger.Warn("Failed to fetch existing file for UPDATE", "error", err, "path", existingFilePath)
		// Fall back to create mode
		existingSHA = nil
		existingContent = ""
	}

	// Merge description sections if updating (same pattern as strava-uploader)
	if existingContent != "" && eventPayload.Description != "" {
		// Check for section header in metadata (signals replaceable section)
		sectionHeader := ""
		for key, val := range eventPayload.EnrichmentMetadata {
			if strings.HasPrefix(key, "section_header_") {
				sectionHeader = val
				break
			}
		}

		if sectionHeader != "" && description.HasSection(eventPayload.Description, sectionHeader) {
			newSectionContent := description.ExtractSection(eventPayload.Description, sectionHeader)
			if newSectionContent != "" {
				eventPayload.Description = description.ReplaceSection(existingContent, sectionHeader, newSectionContent)
			}
		}
	}

	// Build new content, preserving user content below <!-- fitglue:end -->
	markdownContent := buildMarkdownContent(eventPayload)
	if existingContent != "" {
		markdownContent = mergeWithUserContent(markdownContent, existingContent)
	}

	commitMessage := fmt.Sprintf("Update %s", eventPayload.Name)
	externalID, err := createOrUpdateFile(ctx, ghClient, config, existingFilePath, markdownContent, commitMessage, existingSHA)
	if err != nil {
		destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
			pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_FAILED,
			"", fmt.Sprintf("update error: %s", err), eventPayload.Name, fwCtx.Logger)
		return nil, fmt.Errorf("GitHub update failed: %w", err)
	}

	// Increment sync count
	if err := svc.DB.IncrementSyncCount(ctx, eventPayload.UserId); err != nil {
		fwCtx.Logger.Warn("Failed to increment sync count", "error", err)
	}

	destination.UpdateStatus(ctx, svc.DB, svc.Notifications, eventPayload.UserId, fwCtx.PipelineExecutionId,
		pb.Destination_DESTINATION_GITHUB, pb.DestinationStatus_DESTINATION_STATUS_SUCCESS,
		externalID, "", eventPayload.Name, fwCtx.Logger)

	return map[string]interface{}{
		"status":        "SUCCESS",
		"github_commit": externalID,
		"file_path":     existingFilePath,
		"activity_id":   eventPayload.ActivityId,
		"activity_name": eventPayload.Name,
		"activity_type": eventPayload.ActivityType.String(),
		"description":   eventPayload.Description,
		"mode":          "UPDATE",
	}, nil
}

// buildMarkdownContent generates the Markdown file content from the enriched activity
func buildMarkdownContent(event *pb.EnrichedActivityEvent) string {
	var sb strings.Builder

	// YAML frontmatter
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("title: %q\n", event.Name))
	sb.WriteString(fmt.Sprintf("type: %s\n", event.ActivityType.String()))
	if event.StartTime != nil {
		sb.WriteString(fmt.Sprintf("date: %s\n", event.StartTime.AsTime().Format("2006-01-02T15:04:05Z07:00")))
	}
	sb.WriteString(fmt.Sprintf("source: %s\n", event.Source.String()))
	sb.WriteString(fmt.Sprintf("activity_id: %s\n", event.ActivityId))
	sb.WriteString(fmt.Sprintf("pipeline_id: %s\n", event.PipelineId))
	if len(event.AppliedEnrichments) > 0 {
		sb.WriteString(fmt.Sprintf("enrichments: [%s]\n", strings.Join(event.AppliedEnrichments, ", ")))
	}
	if len(event.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("tags: [%s]\n", strings.Join(event.Tags, ", ")))
	}
	sb.WriteString("---\n\n")

	// Title
	sb.WriteString(fmt.Sprintf("# %s\n\n", event.Name))

	// Description (contains all booster output sections)
	if event.Description != "" {
		sb.WriteString(event.Description)
		sb.WriteString("\n")
	}

	// FitGlue end marker — user content below this line is preserved on updates
	sb.WriteString("\n<!-- fitglue:end -->\n")

	return sb.String()
}

// buildFilePath constructs the file path for the activity
func buildFilePath(folder string, event *pb.EnrichedActivityEvent, activityDate time.Time) string {
	// Format: workouts/2026/02/2026-02-08-morning-run/activity.md
	dateStr := activityDate.Format("2006-01-02")
	safeName := sanitizeFileName(event.Name)
	return fmt.Sprintf("%s%s/%s/%s-%s/activity.md",
		folder,
		activityDate.Format("2006"),
		activityDate.Format("01"),
		dateStr,
		safeName,
	)
}

// sanitizeFileName converts a title to a safe filename slug
func sanitizeFileName(name string) string {
	lower := strings.ToLower(name)
	// Replace spaces and special chars with dashes
	replacer := strings.NewReplacer(
		" ", "-", "/", "-", "\\", "-", ":", "-",
		"'", "", "\"", "", "(", "", ")", "",
		".", "-", ",", "",
	)
	result := replacer.Replace(lower)
	// Remove consecutive dashes
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// mergeWithUserContent preserves user-written content below <!-- fitglue:end -->
func mergeWithUserContent(newContent, existingContent string) string {
	// Find user content in existing file (everything after <!-- fitglue:end -->)
	marker := "<!-- fitglue:end -->"
	idx := strings.Index(existingContent, marker)
	if idx == -1 {
		return newContent // No marker found, just use new content
	}

	userContent := existingContent[idx+len(marker):]
	if strings.TrimSpace(userContent) == "" {
		return newContent // No user content to preserve
	}

	// Replace the marker and everything after it in new content with preserved user content
	newIdx := strings.Index(newContent, marker)
	if newIdx == -1 {
		return newContent + "\n" + marker + userContent
	}

	return newContent[:newIdx+len(marker)] + userContent
}

// ─── GitHub API Helpers (using generated client) ────────────────

// gitHubHeaders adds required GitHub API headers to requests
func gitHubHeaders(ctx context.Context, req *http.Request) error {
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "FitGlue/1.0")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	return nil
}

// createOrUpdateFile puts a file into the repository using the generated GitHub client
func createOrUpdateFile(ctx context.Context, ghClient *ghclient.ClientWithResponses, config *gitHubConfig, filePath, content, message string, existingSHA *string) (string, error) {
	committerName := "FitGlue Bot"
	committerEmail := "bot@fitglue.com"

	reqBody := ghclient.CreateOrUpdateFileRequest{
		Message: message,
		Content: base64.StdEncoding.EncodeToString([]byte(content)),
		Sha:     existingSHA,
		Committer: &ghclient.CommitAuthor{
			Name:  committerName,
			Email: committerEmail,
		},
	}

	resp, err := ghClient.ReposcreateOrUpdateFileContentsWithResponse(ctx,
		config.Owner, config.Name, filePath, reqBody, gitHubHeaders)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}

	if resp.StatusCode() >= 400 {
		return "", fmt.Errorf("GitHub API error: status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	// Return the file path as the external ID (used for updates)
	return filePath, nil
}

// getFileContent fetches an existing file's content and SHA from GitHub
func getFileContent(ctx context.Context, ghClient *ghclient.ClientWithResponses, config *gitHubConfig, filePath string) (*string, string, error) {
	resp, err := ghClient.ReposgetContentWithResponse(ctx,
		config.Owner, config.Name, filePath, nil, gitHubHeaders)
	if err != nil {
		return nil, "", fmt.Errorf("GitHub API request failed: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, "", nil // File doesn't exist
	}
	if resp.StatusCode() >= 400 {
		return nil, "", fmt.Errorf("GitHub API error: status %d, body: %s", resp.StatusCode(), string(resp.Body))
	}

	if resp.JSON200 == nil {
		return nil, "", fmt.Errorf("unexpected nil response body")
	}

	file := resp.JSON200
	if file.Content == nil {
		return &file.Sha, "", nil
	}

	// Decode base64 content
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(*file.Content, "\n", ""))
	if err != nil {
		return &file.Sha, "", fmt.Errorf("failed to decode file content: %w", err)
	}

	return &file.Sha, string(decoded), nil
}
