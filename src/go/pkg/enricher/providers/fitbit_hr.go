package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/oapi-codegen/runtime/types"
	"github.com/ripixel/fitglue-server/src/go/pkg/enricher"
	fitbit "github.com/ripixel/fitglue-server/src/go/pkg/types/api/fitbit"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type FitBitHeartRate struct {
	Client *http.Client
}

func NewFitBitHeartRate() *FitBitHeartRate {
	return &FitBitHeartRate{
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *FitBitHeartRate) Name() string {
	return "fitbit-hr"
}

// SecurityProvider implements the security provider interface for the generated client
type SecurityProvider struct {
	Token string
}

func (s *SecurityProvider) Intercept(ctx context.Context, req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+s.Token)
	return nil
}

func (p *FitBitHeartRate) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string) (*enricher.EnrichmentResult, error) {
	// 1. Check Credentials
	if user.Integrations == nil || user.Integrations.Fitbit == nil || !user.Integrations.Fitbit.Enabled {
		return nil, fmt.Errorf("fitbit integration not enabled")
	}
	token := user.Integrations.Fitbit.AccessToken
	if token == "" {
		return nil, fmt.Errorf("missing fitbit access token")
	}

	// 2. Parse Actvity Times
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

	// 3. Initialize Client
	authProvider := &SecurityProvider{Token: token}
	// Use standard Client, not WithResponses, to handle body manually
	client, err := fitbit.NewClient("https://api.fitbit.com", fitbit.WithRequestEditorFn(authProvider.Intercept))
	if err != nil {
		return nil, fmt.Errorf("failed to create fitbit client: %w", err)
	}

	// 4. Request Data (Intraday HR)
	// endpoint: ranges from start to end time on the specific date
	// Method: GetHeartByDateTimestampIntraday(ctx, date, detailLevel, startTime, endTime)
	date := types.Date{Time: startTime}
	resp, err := client.GetHeartByDateTimestampIntraday(ctx, date, "1sec", startTimeStr, endTimeStr)
	if err != nil {
		return nil, fmt.Errorf("fitbit api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fitbit api error %d: %s", resp.StatusCode, string(body))
	}

	// 5. Parse Response
	// Note: We keep the response struct definition local as it matches the exact JSON shape we expect
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

	// 6. Build Stream
	// We need to map the dataset (time HH:MM:SS) to the activity stream (index 0 to duration).
	stream := make([]int, durationSec)

	for _, dataPoint := range hrResponse.ActivitiesHeartIntraday.Dataset {
		ptTime, _ := time.Parse("15:04:05", dataPoint.Time) // Parses as year 0000
		startDayTime, _ := time.Parse("15:04:05", startTimeStr)

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

	slog.Info("Retrieved Fitbit HR", "points", len(hrResponse.ActivitiesHeartIntraday.Dataset), "duration", durationSec)

	return &enricher.EnrichmentResult{
		Metadata: map[string]string{
			"hr_source": "fitbit",
			"hr_points": strconv.Itoa(len(hrResponse.ActivitiesHeartIntraday.Dataset)),
		},
		HeartRateStream: stream,
	}, nil
}
