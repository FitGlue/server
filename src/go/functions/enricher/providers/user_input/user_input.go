package user_input

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pendinginput "github.com/fitglue/server/src/go/pkg/pending_input"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type WaitForInputError struct {
	ActivityID         string
	RequiredFields     []string
	Metadata           map[string]string // Optional metadata to store with pending input (e.g., lap info)
	EnricherProviderID string            // The enricher that created this pending input
}

func (e *WaitForInputError) Error() string {
	return fmt.Sprintf("wait for input: %s (enricher: %s)", e.ActivityID, e.EnricherProviderID)
}

type UserInputProvider struct {
	service     *bootstrap.Service
	activityBag *pb.ActivityPayload // Hack? No, Enrich passes ActivityPayload? No.
	// Provider signature: Enrich(ctx, activity *pb.StandardizedActivity ...)
	// But we need the FULL Payload to save it for re-publishing!
	// The interface doesn't pass the full payload.
	// We need to change the interface? Or Orchestrator needs to handle the payload saving?
	// The Implementation Plan said: "UserInputProvider checks PendingInput... If WAITING -> Returns WaitForInputError."
	// Providing the payload is the Orchestrator's job when it catches the error?
	// YES. The provider error just signals "I need input".
}

func init() {
	providers.Register(&UserInputProvider{})
}

func (p *UserInputProvider) SetService(s *bootstrap.Service) {
	p.service = s
}
func (p *UserInputProvider) Name() string { return "user_input" }
func (p *UserInputProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_USER_INPUT
}

func (p *UserInputProvider) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	stableID := pendinginput.GenerateID(activity.Source, activity.ExternalId, p.Name())

	logger.Debug("user_input: starting",
		"stable_id", stableID,
		"requested_fields", inputs["fields"],
	)

	if p.service == nil {
		logger.Debug("user_input: error - service not initialized")
		return nil, fmt.Errorf("service not initialized")
	}

	// Check DB
	pending, err := p.service.DB.GetPendingInput(ctx, user.UserId, stableID)
	if err == nil && pending != nil {
		logger.Debug("user_input: found pending input",
			"status", pending.Status.String(),
		)

		if pending.Status == pb.PendingInput_STATUS_COMPLETED {
			// CONSUME IT
			logger.Debug("user_input: applying completed input",
				"has_title", pending.InputData["title"] != "",
				"has_description", pending.InputData["description"] != "",
			)
			// Map input data to EnrichmentResult
			res := &providers.EnrichmentResult{
				Name:        pending.InputData["title"],
				Description: pending.InputData["description"],
				Metadata: map[string]string{
					"user_input_applied": "true",
				},
			}
			return res, nil
		}
		if pending.Status == pb.PendingInput_STATUS_WAITING {
			// Still waiting
			logger.Debug("user_input: still waiting for user input")
			fields := parseFields(inputs["fields"])
			return nil, &WaitForInputError{
				ActivityID:         stableID, // Pass stable ID to orchestrator (redundant if orchestration calculates it too)
				RequiredFields:     fields,
				EnricherProviderID: p.Name(),
				Metadata:           buildDisplayConfig(fields),
			}
		}
	}

	// No pending input doc exists -> Request it
	requiredFields := parseFields(inputs["fields"])
	logger.Debug("user_input: no pending input exists - requesting",
		"required_fields", requiredFields,
	)
	return nil, &WaitForInputError{
		ActivityID:         stableID,
		RequiredFields:     requiredFields,
		EnricherProviderID: p.Name(),
		Metadata:           buildDisplayConfig(requiredFields),
	}
}

func parseFields(s string) []string {
	if s == "" {
		return []string{"description"} // Default
	}
	// e.g. "title,description"
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return []string{"description"}
	}
	return out
}

// buildDisplayConfig generates display metadata for the generic user input fields
func buildDisplayConfig(fields []string) map[string]string {
	labels := make(map[string]string)
	types := make(map[string]string)
	for _, f := range fields {
		labels[f] = humanize(f)
		switch f {
		case "description":
			types[f] = "textarea:rows=3"
		default:
			types[f] = "text"
		}
	}
	labelsJSON, _ := json.Marshal(labels)
	typesJSON, _ := json.Marshal(types)
	return map[string]string{
		"display.field_labels": string(labelsJSON),
		"display.field_types":  string(typesJSON),
		"display.summary":      "Provide additional details for this activity",
	}
}

// humanize converts a snake_case field name to Title Case
func humanize(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
