package spotify_tracks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/fitglue/server/src/go/pkg/bootstrap"
	"github.com/fitglue/server/src/go/functions/enricher/providers"
	"github.com/fitglue/server/src/go/pkg/infrastructure/oauth"
	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

type SpotifyTracks struct {
	Service *bootstrap.Service
}

func init() {
	providers.Register(NewSpotifyTracks())
}

func NewSpotifyTracks() *SpotifyTracks {
	return &SpotifyTracks{}
}

func (p *SpotifyTracks) SetService(service *bootstrap.Service) {
	p.Service = service
}

func (p *SpotifyTracks) Name() string {
	return "spotify-tracks"
}

func (p *SpotifyTracks) ProviderType() pb.EnricherProviderType {
	return pb.EnricherProviderType_ENRICHER_PROVIDER_SPOTIFY_TRACKS
}

func (p *SpotifyTracks) Enrich(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, doNotRetry bool) (*providers.EnrichmentResult, error) {
	return p.EnrichWithClient(ctx, activity, user, inputs, nil, doNotRetry)
}

// EnrichWithClient allows HTTP client injection for testing
func (p *SpotifyTracks) EnrichWithClient(ctx context.Context, activity *pb.StandardizedActivity, user *pb.UserRecord, inputs map[string]string, httpClient *http.Client, doNotRetry bool) (*providers.EnrichmentResult, error) {
	// 1. Check Credentials
	if user.Integrations == nil || user.Integrations.Spotify == nil || !user.Integrations.Spotify.Enabled {
		slog.Info("Spotify integration not enabled, skipping")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"spotify_status": "skipped",
				"status_detail":  "Spotify integration not enabled",
			},
		}, nil
	}

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

	// Convert to Unix milliseconds for Spotify API
	afterMs := startTime.UnixMilli()
	beforeMs := endTime.UnixMilli()

	// 3. Initialize OAuth HTTP Client if not provided (for testing)
	if httpClient == nil {
		tokenSource := oauth.NewFirestoreTokenSource(p.Service, user.UserId, "spotify")
		httpClient = oauth.NewClientWithUsageTracking(tokenSource, p.Service, user.UserId, "spotify")
	}

	// 4. Request Recently Played Tracks
	url := fmt.Sprintf("https://api.spotify.com/v1/me/player/recently-played?after=%d&before=%d&limit=50", afterMs, beforeMs)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("spotify api request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("spotify api error %d: %s", resp.StatusCode, string(body))
	}

	// 5. Parse Response
	var recentlyPlayed struct {
		Items []struct {
			Track struct {
				Name    string `json:"name"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"track"`
			PlayedAt string `json:"played_at"`
			Context  *struct {
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"context"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&recentlyPlayed); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 6. Process Tracks
	if len(recentlyPlayed.Items) == 0 {
		slog.Info("No tracks played during activity time window")
		return &providers.EnrichmentResult{
			Metadata: map[string]string{
				"spotify_status": "success",
				"track_count":    "0",
				"status_detail":  "No tracks played during activity",
			},
		}, nil
	}

	// Count track plays
	trackCounts := make(map[string]int)
	trackInfo := make(map[string]string) // track name -> artist
	var playlistName string

	for _, item := range recentlyPlayed.Items {
		trackName := item.Track.Name
		trackCounts[trackName]++

		// Store artist info
		if len(item.Track.Artists) > 0 {
			trackInfo[trackName] = item.Track.Artists[0].Name
		}

		// Extract playlist name from first item with playlist context
		if playlistName == "" && item.Context != nil && item.Context.Type == "playlist" {
			// We'll need to fetch playlist name via API, but for now use URI
			// Format: spotify:playlist:37i9dQZF1DXcBWIGoYBM5M
			playlistName = extractPlaylistID(item.Context.URI)
		}
	}

	// Find most played track
	var topTrack string
	var topCount int
	for track, count := range trackCounts {
		if count > topCount {
			topCount = count
			topTrack = track
		}
	}

	// If there's a tie, sort alphabetically for consistency
	if topCount > 0 {
		var tiedTracks []string
		for track, count := range trackCounts {
			if count == topCount {
				tiedTracks = append(tiedTracks, track)
			}
		}
		if len(tiedTracks) > 1 {
			sort.Strings(tiedTracks)
			topTrack = tiedTracks[0]
		}
	}

	// 7. Format Output
	// "🎵 Soundtrack: 12 tracks • Top played: Blinding Lights - The Weeknd • From playlist: Running Hits 2026"
	summaryText := fmt.Sprintf("\n\n🎵 Soundtrack: %d tracks", len(recentlyPlayed.Items))

	if topTrack != "" {
		artist := trackInfo[topTrack]
		if artist != "" {
			summaryText += fmt.Sprintf(" • Top played: %s - %s", topTrack, artist)
		} else {
			summaryText += fmt.Sprintf(" • Top played: %s", topTrack)
		}
	}

	if playlistName != "" {
		summaryText += fmt.Sprintf(" • From playlist: %s", playlistName)
	}

	// Append to existing description
	newDescription := activity.Description + summaryText

	slog.Info("Spotify tracks enrichment complete",
		"track_count", len(recentlyPlayed.Items),
		"top_track", topTrack,
		"playlist", playlistName,
	)

	return &providers.EnrichmentResult{
		Description: newDescription,
		Metadata: map[string]string{
			"spotify_status": "success",
			"track_count":    fmt.Sprintf("%d", len(recentlyPlayed.Items)),
			"top_track":      topTrack,
			"top_artist":     trackInfo[topTrack],
			"playlist":       playlistName,
			"status_detail":  "Successfully added soundtrack",
		},
	}, nil
}

// extractPlaylistID extracts playlist ID from Spotify URI
// Format: spotify:playlist:37i9dQZF1DXcBWIGoYBM5M
func extractPlaylistID(uri string) string {
	// For now, just return the URI as-is
	// In production, you'd want to fetch the playlist name via API
	// GET https://api.spotify.com/v1/playlists/{playlist_id}
	return uri
}
