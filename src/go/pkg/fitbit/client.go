package fitbit

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	UserID       string
	ClientID     string
	ClientSecret string
	Client       *http.Client // Authenticated client
}

type HeartRateIntraday struct {
	ActivitiesHeart []struct {
		DateTime string `json:"dateTime"`
	} `json:"activities-heart"`
	ActivitiesHeartIntraday struct {
		Dataset []struct {
			Time  string `json:"time"`
			Value int    `json:"value"`
		} `json:"dataset"`
		DatasetInterval int    `json:"datasetInterval"`
		DatasetType     string `json:"datasetType"`
	} `json:"activities-heart-intraday"`
}

// HeartRateIntraday struct...

func NewClient(userID, clientID, clientSecret string) *Client {
	// In real impl: fetch token from Firestore using these creds to refresh if needed.
	return &Client{
		UserID:       userID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Client:       http.DefaultClient,
	}
}

// GetHeartRateSeries fetches 1sec resolution HR data for the window
func (c *Client) GetHeartRateSeries(date string, startTime, endTime string) ([]struct {
	Time  string
	Value int
}, error) {
	// Endpoint: /1/user/[user-id]/activities/heart/date/[date]/1d/1sec/time/[start-time]/[end-time].json
	// startTime format: HH:MM
	url := fmt.Sprintf("https://api.fitbit.com/1/user/%s/activities/heart/date/%s/1d/1sec/time/%s/%s.json",
		c.UserID, date, startTime, endTime)

	// Mock response logic if client is not truly auth'd
	// if os.Getenv("MOCK_FITBIT") == "true" ...

	req, _ := http.NewRequest("GET", url, nil)
	// req.Header.Set("Authorization", "Bearer " + token)

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fitbit api error: %d", resp.StatusCode)
	}

	var data HeartRateIntraday
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	// Map to return type
	result := make([]struct {
		Time  string
		Value int
	}, len(data.ActivitiesHeartIntraday.Dataset))

	for i, d := range data.ActivitiesHeartIntraday.Dataset {
		result[i] = struct {
			Time  string
			Value int
		}{
			Time:  d.Time,
			Value: d.Value,
		}
	}

	return result, nil
}
