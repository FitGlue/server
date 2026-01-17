package user_input

import (
	"context"
	"fmt"
	"strings"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/pkg/enricher_providers"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type WaitForInputError struct {
	ActivityID     string
	RequiredFields []string
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
	enricher_providers.Register(&UserInputProvider{})
}

func (p *UserInputProvider) SetService(s *bootstrap.Service) {
	p.service = s
}
func (p *UserInputProvider) Name() string { return "user_input" }
func (p *UserInputProvider) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_USER_INPUT
}

func (p *UserInputProvider) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*enricher_providers.EnrichmentResult, error) {
	if p.service == nil {
		return nil, fmt.Errorf("service not initialized")
	}

	stableID := fmt.Sprintf("%s:%s", activity.Source, activity.ExternalId)

	// Check DB
	pending, err := p.service.DB.GetPendingInput(ctx, stableID)
	if err == nil && pending != nil {
		if pending.Status == pb.PendingInput_STATUS_COMPLETED {
			// CONSUME IT
			// Map input data to EnrichmentResult
			res := &enricher_providers.EnrichmentResult{
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
			return nil, &WaitForInputError{
				ActivityID:     stableID, // Pass stable ID to orchestrator (redundant if orchestration calculates it too)
				RequiredFields: parseFields(inputs["fields"]),
			}
		}
	}

	// No pending input doc exists -> Request it
	return nil, &WaitForInputError{
		ActivityID:     stableID,
		RequiredFields: parseFields(inputs["fields"]),
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
