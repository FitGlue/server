package spotify_tracks

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSpotifyTracks_ProviderType(t *testing.T) {
	provider := NewSpotifyTracks()
	if provider.ProviderType() != pb.EnricherProviderType_ENRICHER_PROVIDER_SPOTIFY_TRACKS {
		t.Errorf("Expected ENRICHER_PROVIDER_SPOTIFY_TRACKS, got %v", provider.ProviderType())
	}
}

func TestSpotifyTracks_Name(t *testing.T) {
	provider := NewSpotifyTracks()
	if provider.Name() != "spotify-tracks" {
		t.Errorf("Expected 'spotify-tracks', got %s", provider.Name())
	}
}

func TestSpotifyTracks_IntegrationDisabled(t *testing.T) {
	provider := NewSpotifyTracks()
	provider.SetService(&bootstrap.Service{})

	activity := &pb.StandardizedActivity{
		StartTime: timestamppb.New(time.Now()),
		Sessions: []*pb.Session{
			{TotalElapsedTime: 3600},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Spotify: &pb.SpotifyIntegration{
				Enabled: false,
			},
		},
	}

	result, err := provider.Enrich(context.Background(), activity, user, map[string]string{}, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Metadata["spotify_status"] != "skipped" {
		t.Errorf("Expected status 'skipped', got %s", result.Metadata["spotify_status"])
	}
}

func TestSpotifyTracks_NoTracksPlayed(t *testing.T) {
	// Mock Spotify API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items": []}`))
	}))
	defer server.Close()

	// Create mock HTTP client that redirects to test server
	mockClient := &http.Client{
		Transport: &mockTransport{
			testServer: server.URL,
		},
	}

	provider := NewSpotifyTracks()
	provider.SetService(&bootstrap.Service{})

	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now().Add(-1 * time.Hour)),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{TotalElapsedTime: 3600},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Spotify: &pb.SpotifyIntegration{
				Enabled:      true,
				AccessToken:  "test-token",
				RefreshToken: "test-refresh",
			},
		},
	}

	result, err := provider.EnrichWithClient(context.Background(), activity, user, map[string]string{}, mockClient, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Metadata["track_count"] != "0" {
		t.Errorf("Expected track_count '0', got %s", result.Metadata["track_count"])
	}

	if result.Description != "" {
		t.Errorf("Expected empty description, got %s", result.Description)
	}
}

func TestSpotifyTracks_SuccessfulEnrichment(t *testing.T) {
	// Mock Spotify API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"items": [
				{
					"track": {
						"name": "Blinding Lights",
						"artists": [{"name": "The Weeknd"}]
					},
					"played_at": "2026-01-21T10:00:00Z",
					"context": {
						"type": "playlist",
						"uri": "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M"
					}
				},
				{
					"track": {
						"name": "Blinding Lights",
						"artists": [{"name": "The Weeknd"}]
					},
					"played_at": "2026-01-21T10:03:30Z",
					"context": {
						"type": "playlist",
						"uri": "spotify:playlist:37i9dQZF1DXcBWIGoYBM5M"
					}
				},
				{
					"track": {
						"name": "Levitating",
						"artists": [{"name": "Dua Lipa"}]
					},
					"played_at": "2026-01-21T10:07:00Z",
					"context": null
				}
			]
		}`))
	}))
	defer server.Close()

	// Create mock HTTP client
	mockClient := &http.Client{
		Transport: &mockTransport{
			testServer: server.URL,
		},
	}

	provider := NewSpotifyTracks()
	provider.SetService(&bootstrap.Service{})

	activity := &pb.StandardizedActivity{
		StartTime:   timestamppb.New(time.Now().Add(-1 * time.Hour)),
		Description: "Morning Run",
		Sessions: []*pb.Session{
			{TotalElapsedTime: 3600},
		},
	}

	user := &pb.UserRecord{
		UserId: "test-user",
		Integrations: &pb.UserIntegrations{
			Spotify: &pb.SpotifyIntegration{
				Enabled:      true,
				AccessToken:  "test-token",
				RefreshToken: "test-refresh",
			},
		},
	}

	result, err := provider.EnrichWithClient(context.Background(), activity, user, map[string]string{}, mockClient, false)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Metadata["track_count"] != "3" {
		t.Errorf("Expected track_count '3', got %s", result.Metadata["track_count"])
	}

	if result.Metadata["top_track"] != "Blinding Lights" {
		t.Errorf("Expected top_track 'Blinding Lights', got %s", result.Metadata["top_track"])
	}

	if result.Metadata["top_artist"] != "The Weeknd" {
		t.Errorf("Expected top_artist 'The Weeknd', got %s", result.Metadata["top_artist"])
	}

	expectedDesc := "🎵 Soundtrack: 3 tracks • Top played: Blinding Lights - The Weeknd • From playlist: spotify:playlist:37i9dQZF1DXcBWIGoYBM5M"
	if result.Description != expectedDesc {
		t.Errorf("Expected description:\n%s\nGot:\n%s", expectedDesc, result.Description)
	}
}

// mockTransport redirects all requests to the test server
type mockTransport struct {
	testServer string
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	req.URL.Scheme = "http"
	req.URL.Host = m.testServer[7:] // Remove "http://"
	return http.DefaultTransport.RoundTrip(req)
}
