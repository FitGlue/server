package spotify_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/spotify"
)

func spotifyFakeServer(status int, body interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestSpotifyNewClient(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := spotify.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestSpotifyGetMultipleAlbums(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"albums": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	ids := "album1,album2"
	params := &spotify.GetMultipleAlbumsParams{Ids: ids}
	resp, err := c.GetMultipleAlbums(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnAlbum(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"id": "album1", "name": "Test Album"})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	params := &spotify.GetAnAlbumParams{}
	resp, err := c.GetAnAlbum(context.Background(), "album1", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnAlbumsTracks(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"items": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	params := &spotify.GetAnAlbumsTracksParams{}
	resp, err := c.GetAnAlbumsTracks(context.Background(), "album1", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnArtist(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"id": "artist1", "name": "Test Artist"})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	resp, err := c.GetAnArtist(context.Background(), "artist1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnArtistsAlbums(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"items": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	params := &spotify.GetAnArtistsAlbumsParams{}
	resp, err := c.GetAnArtistsAlbums(context.Background(), "artist1", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnArtistsRelatedArtists(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"artists": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	resp, err := c.GetAnArtistsRelatedArtists(context.Background(), "artist1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAnArtistsTopTracks(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"tracks": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	params := &spotify.GetAnArtistsTopTracksParams{}
	resp, err := c.GetAnArtistsTopTracks(context.Background(), "artist1", params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetAudioAnalysis(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"bars": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	resp, err := c.GetAudioAnalysis(context.Background(), "track1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyGetSeveralAudioFeatures(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, map[string]interface{}{"audio_features": []interface{}{}})
	defer srv.Close()
	c, _ := spotify.NewClient(srv.URL)
	ids := "track1,track2"
	params := &spotify.GetSeveralAudioFeaturesParams{Ids: ids}
	resp, err := c.GetSeveralAudioFeatures(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestSpotifyWithHTTPClient(t *testing.T) {
	srv := spotifyFakeServer(http.StatusOK, nil)
	defer srv.Close()
	c, err := spotify.NewClient(srv.URL, spotify.WithHTTPClient(http.DefaultClient))
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
