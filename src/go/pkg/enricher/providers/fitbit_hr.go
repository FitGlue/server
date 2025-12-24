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

	"github.com/ripixel/fitglue-server/src/go/pkg/enricher"
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

	// Format for Fitbit API: HH:MM
	startTimeStr := startTime.Format("15:04")
	endTimeStr := endTime.Format("15:04")
	dateStr := startTime.Format("2006-01-02")

	// 3. Request Data (Intraday HR)
	// endpoint: https://api.fitbit.com/1/user/-/activities/heart/date/[date]/1d/1sec/time/[startTime]/[endTime].json
	url := fmt.Sprintf("https://api.fitbit.com/1/user/-/activities/heart/date/%s/1d/1sec/time/%s/%s.json", dateStr, startTimeStr, endTimeStr)

	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fitbit api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fitbit api error %d: %s", resp.StatusCode, string(body))
	}

	// 4. Parse Response
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

	// 5. Build Stream
	// We need to map the dataset (time HH:MM:SS) to the activity stream (index 0 to duration).
	// This simple implementation assumes synchronized clocks and maps relatively.
	// For production, we'd sync strict timestamps.

	stream := make([]int, durationSec)

	for _, dataPoint := range hrResponse.ActivitiesHeartIntraday.Dataset {
		// dataPoint.Time is "HH:MM:SS"
		// We need to find the offset from startTime
		// This is tricky if it crosses midnight or if timezones differ (Fitbit returns user local time usually)

		// MVP: Ignore exact time mapping, just fill stream sequentially? NO, that's bad.
		// MVP: Parse HH:MM:SS relative to expected start.

		// Let's assume the response is ordered and within the window we requested.
		// We'll simplisticly map based on duration if possible, or just append?
		// "1sec" resolution.

		ptTime, _ := time.Parse("15:04:05", dataPoint.Time) // Parses as year 0000
		startDayTime, _ := time.Parse("15:04:05", startTimeStr)

		offset := int(ptTime.Sub(startDayTime).Seconds())

		if offset >= 0 && offset < durationSec {
			stream[offset] = dataPoint.Value
		}
	}

	// Fill gaps? (Zero-Order Hold or Linear Interp?)
	// For MVP: Leave 0s (Fit file generator handles 0s usually or we interp later)
	// Let's do simple forward fill
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
