package spotify_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/spotify"
)

func spotifyAllServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestAllSpotifyMethods calls ALL remaining spotify client methods not covered in client_test.go
func TestAllSpotifyMethods(t *testing.T) {
	srv := spotifyAllServer()
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
		{"GetMultipleArtists", func() (*http.Response, error) {
			p := &spotify.GetMultipleArtistsParams{Ids: "artist1,artist2"}
			return c.GetMultipleArtists(ctx, p)
		}},
		// Audiobooks
		{"GetMultipleAudiobooks", func() (*http.Response, error) {
			p := &spotify.GetMultipleAudiobooksParams{Ids: "book1,book2"}
			return c.GetMultipleAudiobooks(ctx, p)
		}},
		{"GetAnAudiobook", func() (*http.Response, error) {
			p := &spotify.GetAnAudiobookParams{}
			return c.GetAnAudiobook(ctx, "book1", p)
		}},
		{"GetAudiobookChapters", func() (*http.Response, error) {
			p := &spotify.GetAudiobookChaptersParams{}
			return c.GetAudiobookChapters(ctx, "book1", p)
		}},
		// Categories
		{"GetCategories", func() (*http.Response, error) {
			p := &spotify.GetCategoriesParams{}
			return c.GetCategories(ctx, p)
		}},
		{"GetACategory", func() (*http.Response, error) {
			p := &spotify.GetACategoryParams{}
			return c.GetACategory(ctx, "pop", p)
		}},
		{"GetACategoriesPlaylists", func() (*http.Response, error) {
			p := &spotify.GetACategoriesPlaylistsParams{}
			return c.GetACategoriesPlaylists(ctx, "pop", p)
		}},
		{"GetFeaturedPlaylists", func() (*http.Response, error) {
			p := &spotify.GetFeaturedPlaylistsParams{}
			return c.GetFeaturedPlaylists(ctx, p)
		}},
		{"GetNewReleases", func() (*http.Response, error) {
			p := &spotify.GetNewReleasesParams{}
			return c.GetNewReleases(ctx, p)
		}},
		// Chapters
		{"GetSeveralChapters", func() (*http.Response, error) {
			p := &spotify.GetSeveralChaptersParams{Ids: "ch1,ch2"}
			return c.GetSeveralChapters(ctx, p)
		}},
		{"GetAChapter", func() (*http.Response, error) {
			p := &spotify.GetAChapterParams{}
			return c.GetAChapter(ctx, "ch1", p)
		}},
		// Episodes
		{"GetMultipleEpisodes", func() (*http.Response, error) {
			p := &spotify.GetMultipleEpisodesParams{Ids: "ep1,ep2"}
			return c.GetMultipleEpisodes(ctx, p)
		}},
		{"GetAnEpisode", func() (*http.Response, error) {
			p := &spotify.GetAnEpisodeParams{}
			return c.GetAnEpisode(ctx, "ep1", p)
		}},
		// Markets
		{"GetAvailableMarkets", func() (*http.Response, error) {
			return c.GetAvailableMarkets(ctx)
		}},
		// Users
		{"GetCurrentUsersProfile", func() (*http.Response, error) {
			return c.GetCurrentUsersProfile(ctx)
		}},
		// Library: Albums
		{"GetUsersSavedAlbums", func() (*http.Response, error) {
			p := &spotify.GetUsersSavedAlbumsParams{}
			return c.GetUsersSavedAlbums(ctx, p)
		}},
		{"CheckUsersSavedAlbums", func() (*http.Response, error) {
			p := &spotify.CheckUsersSavedAlbumsParams{Ids: "album1,album2"}
			return c.CheckUsersSavedAlbums(ctx, p)
		}},
		// Library: Audiobooks
		{"GetUsersSavedAudiobooks", func() (*http.Response, error) {
			p := &spotify.GetUsersSavedAudiobooksParams{}
			return c.GetUsersSavedAudiobooks(ctx, p)
		}},
		{"CheckUsersSavedAudiobooks", func() (*http.Response, error) {
			p := &spotify.CheckUsersSavedAudiobooksParams{Ids: "book1,book2"}
			return c.CheckUsersSavedAudiobooks(ctx, p)
		}},
		// Library: Episodes
		{"GetUsersSavedEpisodes", func() (*http.Response, error) {
			p := &spotify.GetUsersSavedEpisodesParams{}
			return c.GetUsersSavedEpisodes(ctx, p)
		}},
		{"CheckUsersSavedEpisodes", func() (*http.Response, error) {
			p := &spotify.CheckUsersSavedEpisodesParams{Ids: "ep1,ep2"}
			return c.CheckUsersSavedEpisodes(ctx, p)
		}},
		// Personalization
		{"GetUsersTopArtistsAndTracks", func() (*http.Response, error) {
			p := &spotify.GetUsersTopArtistsAndTracksParams{}
			return c.GetUsersTopArtistsAndTracks(ctx, "artists", p)
		}},
		// Player
		{"GetInformationAboutTheUsersCurrentPlayback", func() (*http.Response, error) {
			p := &spotify.GetInformationAboutTheUsersCurrentPlaybackParams{}
			return c.GetInformationAboutTheUsersCurrentPlayback(ctx, p)
		}},
		{"GetTheUsersCurrentlyPlayingTrack", func() (*http.Response, error) {
			p := &spotify.GetTheUsersCurrentlyPlayingTrackParams{}
			return c.GetTheUsersCurrentlyPlayingTrack(ctx, p)
		}},
		{"GetAUsersAvailableDevices", func() (*http.Response, error) {
			return c.GetAUsersAvailableDevices(ctx)
		}},
		{"SkipUsersPlaybackToNextTrack", func() (*http.Response, error) {
			p := &spotify.SkipUsersPlaybackToNextTrackParams{}
			return c.SkipUsersPlaybackToNextTrack(ctx, p)
		}},
		{"PauseAUsersPlayback", func() (*http.Response, error) {
			p := &spotify.PauseAUsersPlaybackParams{}
			return c.PauseAUsersPlayback(ctx, p)
		}},
		{"SkipUsersPlaybackToPreviousTrack", func() (*http.Response, error) {
			p := &spotify.SkipUsersPlaybackToPreviousTrackParams{}
			return c.SkipUsersPlaybackToPreviousTrack(ctx, p)
		}},
		{"GetQueue", func() (*http.Response, error) {
			return c.GetQueue(ctx)
		}},
		{"GetRecentlyPlayed", func() (*http.Response, error) {
			p := &spotify.GetRecentlyPlayedParams{}
			return c.GetRecentlyPlayed(ctx, p)
		}},
		{"SetRepeatModeOnUsersPlayback", func() (*http.Response, error) {
			p := &spotify.SetRepeatModeOnUsersPlaybackParams{State: "off"}
			return c.SetRepeatModeOnUsersPlayback(ctx, p)
		}},
		{"SeekToPositionInCurrentlyPlayingTrack", func() (*http.Response, error) {
			p := &spotify.SeekToPositionInCurrentlyPlayingTrackParams{PositionMs: 0}
			return c.SeekToPositionInCurrentlyPlayingTrack(ctx, p)
		}},
		{"ToggleShuffleForUsersPlayback", func() (*http.Response, error) {
			p := &spotify.ToggleShuffleForUsersPlaybackParams{State: true}
			return c.ToggleShuffleForUsersPlayback(ctx, p)
		}},
		{"SetVolumeForUsersPlayback", func() (*http.Response, error) {
			p := &spotify.SetVolumeForUsersPlaybackParams{VolumePercent: 50}
			return c.SetVolumeForUsersPlayback(ctx, p)
		}},
		// Playlists
		{"GetAListOfCurrentUsersPlaylists", func() (*http.Response, error) {
			p := &spotify.GetAListOfCurrentUsersPlaylistsParams{}
			return c.GetAListOfCurrentUsersPlaylists(ctx, p)
		}},
		{"GetPlaylist", func() (*http.Response, error) {
			p := &spotify.GetPlaylistParams{}
			return c.GetPlaylist(ctx, "playlist1", p)
		}},
		{"UnfollowPlaylist", func() (*http.Response, error) {
			return c.UnfollowPlaylist(ctx, "playlist1")
		}},
		{"CheckIfUserFollowsPlaylist", func() (*http.Response, error) {
			ids := "user1,user2"
			p := &spotify.CheckIfUserFollowsPlaylistParams{Ids: &ids}
			return c.CheckIfUserFollowsPlaylist(ctx, "playlist1", p)
		}},
		{"GetPlaylistCover", func() (*http.Response, error) {
			return c.GetPlaylistCover(ctx, "playlist1")
		}},
		{"GetPlaylistsTracks", func() (*http.Response, error) {
			p := &spotify.GetPlaylistsTracksParams{}
			return c.GetPlaylistsTracks(ctx, "playlist1", p)
		}},
		// Recommendations
		{"GetRecommendations", func() (*http.Response, error) {
			p := &spotify.GetRecommendationsParams{}
			return c.GetRecommendations(ctx, p)
		}},
		{"GetRecommendationGenres", func() (*http.Response, error) {
			return c.GetRecommendationGenres(ctx)
		}},
		// Search
		{"Search", func() (*http.Response, error) {
			p := &spotify.SearchParams{
				Q:    "test query",
				Type: []spotify.SearchParamsType{spotify.SearchParamsTypeTrack},
			}
			return c.Search(ctx, p)
		}},
		// Shows
		{"GetMultipleShows", func() (*http.Response, error) {
			p := &spotify.GetMultipleShowsParams{Ids: "show1,show2"}
			return c.GetMultipleShows(ctx, p)
		}},
		{"GetAShow", func() (*http.Response, error) {
			p := &spotify.GetAShowParams{}
			return c.GetAShow(ctx, "show1", p)
		}},
		{"GetAShowsEpisodes", func() (*http.Response, error) {
			p := &spotify.GetAShowsEpisodesParams{}
			return c.GetAShowsEpisodes(ctx, "show1", p)
		}},
		{"GetUsersSavedShows", func() (*http.Response, error) {
			p := &spotify.GetUsersSavedShowsParams{}
			return c.GetUsersSavedShows(ctx, p)
		}},
		{"CheckUsersSavedShows", func() (*http.Response, error) {
			p := &spotify.CheckUsersSavedShowsParams{Ids: "show1"}
			return c.CheckUsersSavedShows(ctx, p)
		}},
		// Library: Tracks
		{"GetUsersSavedTracks", func() (*http.Response, error) {
			p := &spotify.GetUsersSavedTracksParams{}
			return c.GetUsersSavedTracks(ctx, p)
		}},
		{"CheckUsersSavedTracks", func() (*http.Response, error) {
			p := &spotify.CheckUsersSavedTracksParams{Ids: "track1,track2"}
			return c.CheckUsersSavedTracks(ctx, p)
		}},
		// Tracks
		{"GetSeveralTracks", func() (*http.Response, error) {
			p := &spotify.GetSeveralTracksParams{Ids: "track1,track2"}
			return c.GetSeveralTracks(ctx, p)
		}},
		{"GetTrack", func() (*http.Response, error) {
			p := &spotify.GetTrackParams{}
			return c.GetTrack(ctx, "track1", p)
		}},
		// Users
		{"GetUsersProfile", func() (*http.Response, error) {
			return c.GetUsersProfile(ctx, "user1")
		}},
		{"GetListUsersPlaylists", func() (*http.Response, error) {
			p := &spotify.GetListUsersPlaylistsParams{}
			return c.GetListUsersPlaylists(ctx, "user1", p)
		}},
		// Follow
		{"GetFollowed", func() (*http.Response, error) {
			p := &spotify.GetFollowedParams{Type: "artist"}
			return c.GetFollowed(ctx, p)
		}},
		{"CheckCurrentUserFollows", func() (*http.Response, error) {
			p := &spotify.CheckCurrentUserFollowsParams{
				Type: "artist",
				Ids:  "artist1,artist2",
			}
			return c.CheckCurrentUserFollows(ctx, p)
		}},
		// AddToQueue
		{"AddToQueue", func() (*http.Response, error) {
			p := &spotify.AddToQueueParams{Uri: "spotify:track:test"}
			return c.AddToQueue(ctx, p)
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

func TestSpotifyNewClientWithResponses(t *testing.T) {
	srv := spotifyAllServer()
	defer srv.Close()

	c, err := spotify.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}
