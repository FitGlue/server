package parkrun

import (
	"context"
	"testing"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
)

func TestParkrunProvider_Enrich(t *testing.T) {
	provider := NewParkrunProvider()

	// Helper to create activity with location
	createActivity := func(timeStr string, lat, long float64) *pb.StandardizedActivity {
		return &pb.StandardizedActivity{
			Type:      "Run",
			StartTime: timeStr,
			Sessions: []*pb.Session{
				{
					Laps: []*pb.Lap{
						{
							Records: []*pb.Record{
								{
									PositionLat:  lat,
									PositionLong: long,
								},
							},
						},
					},
				},
			},
		}
	}

	tests := []struct {
		name      string
		time      string // RFC3339
		lat, long float64
		inputs    map[string]string
		wantMatch bool
		wantName  string
		wantTags  []string
	}{
		{
			name:      "Saturday Morning at Bushy Park (Perfect Match)",
			time:      "2025-12-20T09:00:00Z", // UTC check (09:00 UTC is 09:00 GMT)
			lat:       51.4106,
			long:      -0.3421,
			wantMatch: true,
			wantName:  "Bushy Park Parkrun",
			wantTags:  []string{"Parkrun"},
		},
		{
			name:      "Saturday Morning at Bushy Park (Slightly Away - 100m)",
			time:      "2025-12-20T09:00:00Z",
			lat:       51.4115, // Approx 100m North
			long:      -0.3421,
			wantMatch: true,
			wantName:  "Bushy Park Parkrun",
			wantTags:  []string{"Parkrun"},
		},
		{
			name:      "Saturday Morning at Bushy Park (Too Far - 1km)",
			time:      "2025-12-20T09:00:00Z",
			lat:       51.4206,
			long:      -0.3421,
			wantMatch: false,
		},
		{
			name:      "Saturday Afternoon (Not Parkrun)",
			time:      "2025-12-20T14:00:00Z",
			lat:       51.4106,
			long:      -0.3421,
			wantMatch: false,
		},
		{
			name:      "Tuesday Morning (Not Parkrun)",
			time:      "2025-12-23T09:00:00Z",
			lat:       51.4106,
			long:      -0.3421,
			wantMatch: false,
		},
		{
			name:      "Christmas Day (Special Event check)",
			time:      "2025-12-25T09:00:00Z", // Thursday, but Xmas
			lat:       51.4106,
			long:      -0.3421,
			inputs:    map[string]string{"special_events": "12-25"},
			wantMatch: true,
			wantName:  "Bushy Park Parkrun",
		},
		{
			name: "Australian Parkrun (Timezone check - Albert Park)",
			// Albert Park: -37.8427, 144.9654
			// UTC+10 (Dec is Summer, so +11 actually)
			// 9am Melbourne = 10pm Previous Day (Friday) UTC
			time:      "2025-12-19T22:00:00Z", // Friday 10pm UTC
			lat:       -37.8427,
			long:      144.9654,
			wantMatch: true,
			wantName:  "Albert Parkrun, Melbourne",
		},
		{
			name: "Custom Tags",
			time: "2025-12-20T09:00:00Z",
			lat:  51.4106,
			long: -0.3421,
			inputs: map[string]string{
				"tags": "Parkrun,Race,5k",
			},
			wantMatch: true,
			wantName:  "Bushy Park Parkrun",
			wantTags:  []string{"Parkrun", "Race", "5k"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity := createActivity(tt.time, tt.lat, tt.long)

			inputs := tt.inputs
			if inputs == nil {
				inputs = make(map[string]string)
			}

			res, err := provider.Enrich(context.Background(), activity, nil, inputs, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !tt.wantMatch {
				if res != nil {
					t.Errorf("Expected nil result (no match), got %v", res)
				}
				return
			}

			if res == nil {
				t.Fatal("Expected matching result, got nil")
			}

			if res.Name != tt.wantName {
				t.Errorf("Expected Name %q, got %q", tt.wantName, res.Name)
			}

			if len(tt.wantTags) > 0 {
				if len(res.Tags) != len(tt.wantTags) {
					t.Errorf("Expected %d tags, got %v", len(tt.wantTags), res.Tags)
				}
			}
		})
	}
}
