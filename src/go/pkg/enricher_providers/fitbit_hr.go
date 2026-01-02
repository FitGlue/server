package enricher_providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/ripixel/fitglue-server/src/go/pkg/bootstrap"
	"github.com/ripixel/fitglue-server/src/go/pkg/infrastructure/oauth"
	fitbit "github.com/ripixel/fitglue-server/src/go/pkg/integrations/fitbit"
	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

type FitBitHeartRate struct {
	Service *bootstrap.Service
}

func init() {
	Register(NewFitBitHeartRate())
}

func NewFitBitHeartRate() *FitBitHeartRate {
	return &FitBitHeartRate{}
}

func (p *FitBitHeartRate) Name() string {
	return "fitbit-heart-rate"
}

func (p *FitBitHeartRate) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_FITBIT_HEART_RATE
}

func (p *FitBitHeartRate) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*EnrichmentResult, error) {
	return p.EnrichWithClient(ctx, activity, user, inputs, nil, doNotRetry)
}

// EnrichWithClient allows HTTP client injection for testing
func (p *FitBitHeartRate) EnrichWithClient(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, httpClient *http.Client, doNotRetry bool) (*EnrichmentResult, error) {
	// 1. Check Credentials
	if user.Integrations == nil || user.Integrations.Fitbit == nil || !user.Integrations.Fitbit.Enabled {
		return nil, fmt.Errorf("fitbit integration not enabled")
	}

	// 2. Parse Activity Times
	// 2. Parse Activity Times
	startTime := activity.StartTime.AsTime()
	if startTime.IsZero() {
		return nil, fmt.Errorf("invalid start time: zero")
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
	slog.Info(fmt.Sprintf("Retrieved Fitbit HR points=%d duration=%d start_time=%s", pointsFound, durationSec, startTimeStr))

	// Lag Detection (Start/End Coverage)
	hasStart := false
	hasEnd := false
	startThreshold := 120 // 2 minutes (or 10% logic)
	endThreshold := durationSec - 120

	if pointsFound > 0 {
		// Calculate coverage
		// Sort just in case? API returns sorted usually.
		firstPt := hrResponse.ActivitiesHeartIntraday.Dataset[0]
		lastPt := hrResponse.ActivitiesHeartIntraday.Dataset[pointsFound-1]

		t1, _ := time.Parse("15:04:05", firstPt.Time)
		t2, _ := time.Parse("15:04:05", lastPt.Time)
		startBase, _ := time.Parse("15:04", startTimeStr)

		offset1 := int(t1.Sub(startBase).Seconds())
		offset2 := int(t2.Sub(startBase).Seconds())

		if offset1 <= startThreshold {
			hasStart = true
		}
		if offset2 >= endThreshold {
			hasEnd = true
		}
	}
	slog.Info(fmt.Sprintf("Retrieved Fitbit HR points=%d duration=%d start_time=%s has_start=%v has_end=%v", pointsFound, durationSec, startTimeStr, hasStart, hasEnd))

	// Decision logic
	timeSinceEnd := time.Since(endTime)
	isRecent := timeSinceEnd < 30*time.Minute

	var lagErr error
	if (!hasStart || !hasEnd) && isRecent {
		reason := fmt.Sprintf("incomplete data (start:%v end:%v) for recent activity (%v ago)", hasStart, hasEnd, timeSinceEnd.Round(time.Second))

		// Check if we exhausted retries
		if doNotRetry {
			slog.Warn("Incomplete data detected but forced to continue: " + reason)
			// DO NOT return error, accept whatever data we have
		} else {
			slog.Warn("Incomplete data detected: " + reason)
			// Return RetryableError to trigger lag mechanism
			lagErr = NewRetryableError(fmt.Errorf("incomplete data"), 1*time.Minute, reason)
			// Logic: If it's a RetryableError, the system will discard this result anyway.
			return nil, lagErr
		}
	} else if pointsFound == 0 && !isRecent {
		// If old and empty, likely no data ever. Just return empty.
		slog.Warn(fmt.Sprintf("No heart rate data points found in Fitbit response start_time=%s end_time=%s", startTimeStr, endTimeStr))
	}

	return &EnrichmentResult{
		Name:            "", // Don't wipe name
		HeartRateStream: stream,
		Metadata: map[string]string{
			"hr_source":     "fitbit",
			"query_date":    date,
			"query_start":   startTimeStr,
			"query_end":     endTimeStr,
			"points_found":  fmt.Sprintf("%d", pointsFound),
			"status_detail": "Success",
			"do_not_retry":  fmt.Sprintf("%v", doNotRetry),
		},
	}, nil
}
