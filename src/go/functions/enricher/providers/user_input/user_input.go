package user_input

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type WaitForInputError struct {
	ActivityID     string
	RequiredFields []string
	Metadata       map[string]string // Optional metadata to store with pending input (e.g., lap info)
}

func (e *WaitForInputError) Error() string {
	return fmt.Sprintf("wait for input: %s", e.ActivityID)
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
	stableID := fmt.Sprintf("%s:%s", activity.Source, activity.ExternalId)

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
			return nil, &WaitForInputError{
				ActivityID:     stableID, // Pass stable ID to orchestrator (redundant if orchestration calculates it too)
				RequiredFields: parseFields(inputs["fields"]),
			}
		}
	}

	// No pending input doc exists -> Request it
	requiredFields := parseFields(inputs["fields"])
	logger.Debug("user_input: no pending input exists - requesting",
		"required_fields", requiredFields,
	)
	return nil, &WaitForInputError{
		ActivityID:     stableID,
		RequiredFields: requiredFields,
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
