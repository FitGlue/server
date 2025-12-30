package enricher_providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	fitbit "github.com/ripixel/fitglue-server/src/go/pkg/integrations/fitbit"

	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/oauth"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type FitBitHeartRate struct {
	Service *bootstrap.Service
}

func NewFitBitHeartRate() *FitBitHeartRate {
	return &FitBitHeartRate{}
}

func (p *FitBitHeartRate) SetService(svc *bootstrap.Service) {
	p.Service = svc
}

func (p *FitBitHeartRate) Name() string {
	return "fitbit-heart-rate"
}

func (p *FitBitHeartRate) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string) (*EnrichmentResult, error) {
	return p.EnrichWithClient(ctx, activity, user, inputs, nil)
}

// EnrichWithClient allows HTTP client injection for testing
func (p *FitBitHeartRate) EnrichWithClient(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, httpClient *http.Client) (*EnrichmentResult, error) {
	// 1. Check Credentials
	if user.Integrations == nil || user.Integrations.Fitbit == nil || !user.Integrations.Fitbit.Enabled {
		return nil, fmt.Errorf("fitbit integration not enabled")
	}

	// 2. Parse Activity Times
	startTime, err := time.Parse(time.RFC3339, activity.StartTime)
	if err != nil {
		return nil, fmt.Errorf("invalid start time: %w", err)
	}

	// Calculate end time
	durationSec := 3600 // Default
	if len(activity.Sessions) > 0 {
		durationSec = int(activity.Sessions[0].TotalElapsedTime)
	}
	endTime := startTime.Add(time.Duration(durationSec) * time.Second)

	// Format for Fitbit API
	startTimeStr := startTime.Format("15:04")
	endTimeStr := endTime.Format("15:04")

	// 3. Initialize OAuth HTTP Client if not provided (for testing)
	if httpClient == nil {
		tokenSource := oauth.NewFirestoreTokenSource(p.Service, user.UserId, "fitbit")
		httpClient = oauth.NewHTTPClient(tokenSource)
	}

	// 4. Create Fitbit Client with OAuth transport
	client, err := fitbit.NewClient("https://api.fitbit.com", fitbit.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create fitbit client: %w", err)
	}

	// 5. Request Data (Intraday HR)
	date := startTime.Format("2006-01-02")
	resp, err := client.GetHeartByDateTimestampIntraday(ctx, date, "1sec", startTimeStr, endTimeStr)
	if err != nil {
		return nil, fmt.Errorf("fitbit api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fitbit api error %d: %s", resp.StatusCode, string(body))
	}

	// 6. Parse Response
	var hrResponse struct {
		ActivitiesHeartIntraday struct {
			Dataset []struct {
				Time  string `json:"time"`
				Value int    `json:"value"`
			} `json:"dataset"`
		} `json:"activities-heart-intraday"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&hrResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 7. Build Stream
	stream := make([]int, durationSec)

	for _, dataPoint := range hrResponse.ActivitiesHeartIntraday.Dataset {
		ptTime, _ := time.Parse("15:04:05", dataPoint.Time)
		startDayTime, _ := time.Parse("15:04", startTimeStr)

		offset := int(ptTime.Sub(startDayTime).Seconds())

		if offset >= 0 && offset < durationSec {
			stream[offset] = dataPoint.Value
		}
	}

	// Fill gaps (Forward Fill)
	lastVal := 0
	for i := 0; i < len(stream); i++ {
		if stream[i] != 0 {
			lastVal = stream[i]
		} else {
			stream[i] = lastVal
		}
	}

	pointsFound := len(hrResponse.ActivitiesHeartIntraday.Dataset)
	if pointsFound > 0 {
		slog.Info("Retrieved Fitbit HR", "points", pointsFound, "duration", durationSec, "start_time", startTimeStr)
	} else {
		slog.Warn("No heart rate data points found in Fitbit response", "start_time", startTimeStr, "end_time", endTimeStr)
	}

	if len(stream) == 0 {
		slog.Warn("Heart rate stream is empty after processing")
	}

	// Status Message
	statusMsg := "Success"
	if pointsFound == 0 {
		statusMsg = "No heart rate data points found in Fitbit response"
	}

	return &EnrichmentResult{
		Metadata: map[string]string{
			"hr_source":     "fitbit",
			"hr_points":     strconv.Itoa(pointsFound),
			"query_start":   startTimeStr,
			"query_end":     endTimeStr,
			"status_detail": statusMsg,
		},
		HeartRateStream: stream,
	}, nil
}
