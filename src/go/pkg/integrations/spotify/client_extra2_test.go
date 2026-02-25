package spotify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/spotify"
)

func spotifyExtra2Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestSpotifyIntegrationsExtra2 covers ALL remaining uncovered method calls in pkg/integrations/spotify.
func TestSpotifyIntegrationsExtra2(t *testing.T) {
	srv := spotifyExtra2Server()
	defer srv.Close()

	c, err := spotify.NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		// Albums
		{"GetMultipleAlbums", func() (*http.Response, error) {
			p := &spotify.GetMultipleAlbumsParams{}
			return c.GetMultipleAlbums(ctx, p)
		}},
		{"GetAnAlbum", func() (*http.Response, error) {
			p := &spotify.GetAnAlbumParams{}
			return c.GetAnAlbum(ctx, "album-id-1", p)
		}},
		{"GetAnAlbumsTracks", func() (*http.Response, error) {
			p := &spotify.GetAnAlbumsTracksParams{}
			return c.GetAnAlbumsTracks(ctx, "album-id-1", p)
		}},
		// Artists
		{"GetAnArtist", func() (*http.Response, error) {
			return c.GetAnArtist(ctx, "artist-id-1")
		}},
		{"GetAnArtistsAlbums", func() (*http.Response, error) {
			p := &spotify.GetAnArtistsAlbumsParams{}
			return c.GetAnArtistsAlbums(ctx, "artist-id-1", p)
		}},
		{"GetAnArtistsRelatedArtists", func() (*http.Response, error) {
			return c.GetAnArtistsRelatedArtists(ctx, "artist-id-1")
		}},
		{"GetAnArtistsTopTracks", func() (*http.Response, error) {
			p := &spotify.GetAnArtistsTopTracksParams{}
			return c.GetAnArtistsTopTracks(ctx, "artist-id-1", p)
		}},
		// Audio Features
		{"GetAudioAnalysis", func() (*http.Response, error) {
			return c.GetAudioAnalysis(ctx, "track-id-1")
		}},
		{"GetAudioFeatures", func() (*http.Response, error) {
			return c.GetAudioFeatures(ctx, "track-id-1")
		}},
		{"GetSeveralAudioFeatures", func() (*http.Response, error) {
			p := &spotify.GetSeveralAudioFeaturesParams{Ids: "track-id-1,track-id-2"}
			return c.GetSeveralAudioFeatures(ctx, p)
		}},
		// User Library: Albums
		{"SaveAlbumsUser", func() (*http.Response, error) {
			p := &spotify.SaveAlbumsUserParams{}
			return c.SaveAlbumsUser(ctx, p, spotify.SaveAlbumsUserJSONRequestBody{})
		}},
		{"RemoveAlbumsUser", func() (*http.Response, error) {
			p := &spotify.RemoveAlbumsUserParams{}
			return c.RemoveAlbumsUser(ctx, p, spotify.RemoveAlbumsUserJSONRequestBody{})
		}},
		// User Library: Audiobooks
		{"SaveAudiobooksUser", func() (*http.Response, error) {
			p := &spotify.SaveAudiobooksUserParams{}
			return c.SaveAudiobooksUser(ctx, p)
		}},
		{"RemoveAudiobooksUser", func() (*http.Response, error) {
			p := &spotify.RemoveAudiobooksUserParams{}
			return c.RemoveAudiobooksUser(ctx, p)
		}},
		// User Library: Episodes
		{"SaveEpisodesUser", func() (*http.Response, error) {
			p := &spotify.SaveEpisodesUserParams{}
			return c.SaveEpisodesUser(ctx, p, spotify.SaveEpisodesUserJSONRequestBody{})
		}},
		{"RemoveEpisodesUser", func() (*http.Response, error) {
			p := &spotify.RemoveEpisodesUserParams{}
			return c.RemoveEpisodesUser(ctx, p, spotify.RemoveEpisodesUserJSONRequestBody{})
		}},
		// User Library: Shows
		{"SaveShowsUser", func() (*http.Response, error) {
			p := &spotify.SaveShowsUserParams{}
			return c.SaveShowsUser(ctx, p)
		}},
		{"RemoveShowsUser", func() (*http.Response, error) {
			p := &spotify.RemoveShowsUserParams{}
			return c.RemoveShowsUser(ctx, p)
		}},
		// User Library: Tracks
		{"SaveTracksUser", func() (*http.Response, error) {
			return c.SaveTracksUser(ctx, spotify.SaveTracksUserJSONRequestBody{})
		}},
		{"RemoveTracksUser", func() (*http.Response, error) {
			p := &spotify.RemoveTracksUserParams{}
			return c.RemoveTracksUser(ctx, p, spotify.RemoveTracksUserJSONRequestBody{})
		}},
		// Artist Following
		{"FollowArtistsUsers", func() (*http.Response, error) {
			p := &spotify.FollowArtistsUsersParams{}
			return c.FollowArtistsUsers(ctx, p, spotify.FollowArtistsUsersJSONRequestBody{})
		}},
		{"UnfollowArtistsUsers", func() (*http.Response, error) {
			p := &spotify.UnfollowArtistsUsersParams{}
			return c.UnfollowArtistsUsers(ctx, p, spotify.UnfollowArtistsUsersJSONRequestBody{})
		}},
		// Playback
		{"StartAUsersPlayback", func() (*http.Response, error) {
			p := &spotify.StartAUsersPlaybackParams{}
			return c.StartAUsersPlayback(ctx, p, spotify.StartAUsersPlaybackJSONRequestBody{})
		}},
		{"TransferAUsersPlayback", func() (*http.Response, error) {
			return c.TransferAUsersPlayback(ctx, spotify.TransferAUsersPlaybackJSONRequestBody{})
		}},
		// Playlists
		{"ChangePlaylistDetails", func() (*http.Response, error) {
			return c.ChangePlaylistDetails(ctx, "playlist-id-1", spotify.ChangePlaylistDetailsJSONRequestBody{})
		}},
		{"FollowPlaylist", func() (*http.Response, error) {
			return c.FollowPlaylist(ctx, "playlist-id-1", spotify.FollowPlaylistJSONRequestBody{})
		}},
		{"AddTracksToPlaylist", func() (*http.Response, error) {
			p := &spotify.AddTracksToPlaylistParams{}
			return c.AddTracksToPlaylist(ctx, "playlist-id-1", p, spotify.AddTracksToPlaylistJSONRequestBody{})
		}},
		{"RemoveTracksPlaylist", func() (*http.Response, error) {
			return c.RemoveTracksPlaylist(ctx, "playlist-id-1", spotify.RemoveTracksPlaylistJSONRequestBody{})
		}},
		{"ReorderOrReplacePlaylistsTracks", func() (*http.Response, error) {
			p := &spotify.ReorderOrReplacePlaylistsTracksParams{}
			return c.ReorderOrReplacePlaylistsTracks(ctx, "playlist-id-1", p, spotify.ReorderOrReplacePlaylistsTracksJSONRequestBody{})
		}},
		{"CreatePlaylist", func() (*http.Response, error) {
			return c.CreatePlaylist(ctx, "user-id-1", spotify.CreatePlaylistJSONRequestBody{})
		}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := tc.fn()
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.name, err)
				return
			}
			if resp == nil {
				t.Errorf("%s: expected non-nil response", tc.name)
			}
		})
	}
}
