package running_dynamics

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type RunningDynamics struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewRunningDynamics())
}

func NewRunningDynamics() *RunningDynamics {
	return &RunningDynamics{}
}

func (p *RunningDynamics) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *RunningDynamics) Name() string {
	return "running-dynamics"
}

func (p *RunningDynamics) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_RUNNING_DYNAMICS
}

func (p *RunningDynamics) Enrich(ctx context.Context, logger *slog.Logger, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	logger.Debug("running_dynamics: starting",
		"activity_name", activity.Name,
	)

	var gcts []int32
	var vos []int32
	var vrs []int32
	var sls []float64

	for _, session := range activity.Sessions {
		for _, lap := range session.Laps {
			for _, record := range lap.Records {
				if record.GroundContactTime != nil && *record.GroundContactTime > 0 {
					gcts = append(gcts, *record.GroundContactTime)
				}
				if record.VerticalOscillation != nil && *record.VerticalOscillation > 0 {
					vos = append(vos, *record.VerticalOscillation)
				}
				if record.VerticalRatio != nil && *record.VerticalRatio > 0 {
					vrs = append(vrs, *record.VerticalRatio)
				}
				if record.StepLength != nil && *record.StepLength > 0 {
					sls = append(sls, *record.StepLength)
				}
			}
		}
	}

	if len(gcts) == 0 && len(vos) == 0 && len(sls) == 0 {
		logger.Debug("running_dynamics: skipping - no running dynamics data found")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"running_dynamics_status": "skipped",
			},
		}, nil
	}

	// Calculate averages
	var summaryParts []string

	if len(gcts) > 0 {
		var sum int64
		for _, v := range gcts {
			sum += int64(v)
		}
		avg := float64(sum) / float64(len(gcts))
		summaryParts = append(summaryParts, fmt.Sprintf("⏱️ GCT: %.0f ms", avg))
	}

	if len(sls) > 0 {
		var sum float64
		for _, v := range sls {
			sum += v
		}
		avg := sum / float64(len(sls))
		summaryParts = append(summaryParts, fmt.Sprintf("📏 Stride: %.2f m", avg))
	}

	if len(vos) > 0 {
		var sum int64
		for _, v := range vos {
			sum += int64(v)
		}
		avg := float64(sum) / float64(len(vos))
		summaryParts = append(summaryParts, fmt.Sprintf("↕️ Vert: %.1f cm", avg/10.0)) // mm to cm
	}

	if len(summaryParts) == 0 {
		return &providers.EnrichmentResult{}, nil
	}

	// Build summary line
	summaryText := "🏃 Running Dynamics: "
	for i, part := range summaryParts {
		if i > 0 {
			summaryText += " • "
		}
		summaryText += part
	}

	return &providers.EnrichmentResult{
		Description: summaryText,
		Metadata: map[string]string{
			"running_dynamics_status": "success",
		},
	}, nil
}
