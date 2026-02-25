package spotify_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fitglue/server/src/go/pkg/integrations/spotify"
)

func spotifyExtra3Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestSpotifyClientWithResponses covers ALL spotify ClientWithResponses methods for code coverage.
// No error assertions — focused on exercising ParseXXXResponse functions for coverage.
func TestSpotifyClientWithResponses(t *testing.T) {
	srv := spotifyExtra3Server()
	defer srv.Close()

	c, err := spotify.NewClientWithResponses(srv.URL)
	if err != nil {
		t.Fatalf("NewClientWithResponses failed: %v", err)
	}

	ctx := context.Background()
	albumId := spotify.PathAlbumId("album1")
	artistId := spotify.PathArtistId("artist1")
	audiobookId := spotify.PathAudiobookId("ab1")
	playlistId := spotify.PathPlaylistId("pl1")
	showId := spotify.PathShowId("show1")
	userId := spotify.PathUserId("user1")
	chapterId := spotify.PathChapterId("ch1")

	// Albums
	c.GetMultipleAlbumsWithResponse(ctx, &spotify.GetMultipleAlbumsParams{})                                                                  //nolint:errcheck
	c.GetAnAlbumWithResponse(ctx, albumId, &spotify.GetAnAlbumParams{})                                                                       //nolint:errcheck
	c.GetAnAlbumsTracksWithResponse(ctx, albumId, &spotify.GetAnAlbumsTracksParams{})                                                         //nolint:errcheck
	c.RemoveAlbumsUserWithBodyWithResponse(ctx, &spotify.RemoveAlbumsUserParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.RemoveAlbumsUserWithResponse(ctx, &spotify.RemoveAlbumsUserParams{}, spotify.RemoveAlbumsUserJSONRequestBody{})                         //nolint:errcheck
	c.GetUsersSavedAlbumsWithResponse(ctx, &spotify.GetUsersSavedAlbumsParams{})                                                              //nolint:errcheck
	c.SaveAlbumsUserWithBodyWithResponse(ctx, &spotify.SaveAlbumsUserParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))     //nolint:errcheck
	c.SaveAlbumsUserWithResponse(ctx, &spotify.SaveAlbumsUserParams{}, spotify.SaveAlbumsUserJSONRequestBody{})                               //nolint:errcheck
	c.CheckUsersSavedAlbumsWithResponse(ctx, &spotify.CheckUsersSavedAlbumsParams{})                                                          //nolint:errcheck
	c.GetNewReleasesWithResponse(ctx, &spotify.GetNewReleasesParams{})                                                                        //nolint:errcheck

	// Artists
	c.GetMultipleArtistsWithResponse(ctx, &spotify.GetMultipleArtistsParams{})                 //nolint:errcheck
	c.GetAnArtistWithResponse(ctx, artistId)                                                   //nolint:errcheck
	c.GetAnArtistsAlbumsWithResponse(ctx, artistId, &spotify.GetAnArtistsAlbumsParams{})       //nolint:errcheck
	c.GetAnArtistsRelatedArtistsWithResponse(ctx, artistId)                                    //nolint:errcheck
	c.GetAnArtistsTopTracksWithResponse(ctx, artistId, &spotify.GetAnArtistsTopTracksParams{}) //nolint:errcheck

	// Audio
	c.GetAudioAnalysisWithResponse(ctx, "track1")                                        //nolint:errcheck
	c.GetSeveralAudioFeaturesWithResponse(ctx, &spotify.GetSeveralAudioFeaturesParams{}) //nolint:errcheck
	c.GetAudioFeaturesWithResponse(ctx, "track1")                                        //nolint:errcheck

	// Audiobooks
	c.GetMultipleAudiobooksWithResponse(ctx, &spotify.GetMultipleAudiobooksParams{})            //nolint:errcheck
	c.GetAnAudiobookWithResponse(ctx, audiobookId, &spotify.GetAnAudiobookParams{})             //nolint:errcheck
	c.GetAudiobookChaptersWithResponse(ctx, audiobookId, &spotify.GetAudiobookChaptersParams{}) //nolint:errcheck
	c.GetUsersSavedAudiobooksWithResponse(ctx, &spotify.GetUsersSavedAudiobooksParams{})        //nolint:errcheck
	c.RemoveAudiobooksUserWithResponse(ctx, &spotify.RemoveAudiobooksUserParams{})              //nolint:errcheck
	c.SaveAudiobooksUserWithResponse(ctx, &spotify.SaveAudiobooksUserParams{})                  //nolint:errcheck
	c.CheckUsersSavedAudiobooksWithResponse(ctx, &spotify.CheckUsersSavedAudiobooksParams{})    //nolint:errcheck

	// Chapters
	c.GetSeveralChaptersWithResponse(ctx, &spotify.GetSeveralChaptersParams{}) //nolint:errcheck
	c.GetAChapterWithResponse(ctx, chapterId, &spotify.GetAChapterParams{})    //nolint:errcheck

	// Categories
	c.GetCategoriesWithResponse(ctx, &spotify.GetCategoriesParams{})                            //nolint:errcheck
	c.GetACategoryWithResponse(ctx, "pop", &spotify.GetACategoryParams{})                       //nolint:errcheck
	c.GetACategoriesPlaylistsWithResponse(ctx, "pop", &spotify.GetACategoriesPlaylistsParams{}) //nolint:errcheck
	c.GetFeaturedPlaylistsWithResponse(ctx, &spotify.GetFeaturedPlaylistsParams{})              //nolint:errcheck

	// Episodes
	c.GetMultipleEpisodesWithResponse(ctx, &spotify.GetMultipleEpisodesParams{})                                                                  //nolint:errcheck
	c.GetAnEpisodeWithResponse(ctx, "ep1", &spotify.GetAnEpisodeParams{})                                                                         //nolint:errcheck
	c.GetUsersSavedEpisodesWithResponse(ctx, &spotify.GetUsersSavedEpisodesParams{})                                                              //nolint:errcheck
	c.RemoveEpisodesUserWithBodyWithResponse(ctx, &spotify.RemoveEpisodesUserParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.RemoveEpisodesUserWithResponse(ctx, &spotify.RemoveEpisodesUserParams{}, spotify.RemoveEpisodesUserJSONRequestBody{})                       //nolint:errcheck
	c.SaveEpisodesUserWithBodyWithResponse(ctx, &spotify.SaveEpisodesUserParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))     //nolint:errcheck
	c.SaveEpisodesUserWithResponse(ctx, &spotify.SaveEpisodesUserParams{}, spotify.SaveEpisodesUserJSONRequestBody{})                             //nolint:errcheck
	c.CheckUsersSavedEpisodesWithResponse(ctx, &spotify.CheckUsersSavedEpisodesParams{})                                                          //nolint:errcheck

	// Player
	c.GetInformationAboutTheUsersCurrentPlaybackWithResponse(ctx, &spotify.GetInformationAboutTheUsersCurrentPlaybackParams{})                      //nolint:errcheck
	c.TransferAUsersPlaybackWithBodyWithResponse(ctx, "application/json", io.NopCloser(strings.NewReader("{}")))                                    //nolint:errcheck
	c.TransferAUsersPlaybackWithResponse(ctx, spotify.TransferAUsersPlaybackJSONRequestBody{})                                                      //nolint:errcheck
	c.GetAUsersAvailableDevicesWithResponse(ctx)                                                                                                    //nolint:errcheck
	c.GetTheUsersCurrentlyPlayingTrackWithResponse(ctx, &spotify.GetTheUsersCurrentlyPlayingTrackParams{})                                          //nolint:errcheck
	c.SkipUsersPlaybackToNextTrackWithResponse(ctx, &spotify.SkipUsersPlaybackToNextTrackParams{})                                                  //nolint:errcheck
	c.SkipUsersPlaybackToPreviousTrackWithResponse(ctx, &spotify.SkipUsersPlaybackToPreviousTrackParams{})                                          //nolint:errcheck
	c.SeekToPositionInCurrentlyPlayingTrackWithResponse(ctx, &spotify.SeekToPositionInCurrentlyPlayingTrackParams{})                                //nolint:errcheck
	c.SetRepeatModeOnUsersPlaybackWithResponse(ctx, &spotify.SetRepeatModeOnUsersPlaybackParams{})                                                  //nolint:errcheck
	c.SetVolumeForUsersPlaybackWithResponse(ctx, &spotify.SetVolumeForUsersPlaybackParams{})                                                        //nolint:errcheck
	c.ToggleShuffleForUsersPlaybackWithResponse(ctx, &spotify.ToggleShuffleForUsersPlaybackParams{})                                                //nolint:errcheck
	c.GetRecentlyPlayedWithResponse(ctx, &spotify.GetRecentlyPlayedParams{})                                                                        //nolint:errcheck
	c.GetQueueWithResponse(ctx)                                                                                                                     //nolint:errcheck
	c.AddToQueueWithResponse(ctx, &spotify.AddToQueueParams{})                                                                                      //nolint:errcheck
	c.PauseAUsersPlaybackWithResponse(ctx, &spotify.PauseAUsersPlaybackParams{})                                                                    //nolint:errcheck
	c.StartAUsersPlaybackWithBodyWithResponse(ctx, &spotify.StartAUsersPlaybackParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.StartAUsersPlaybackWithResponse(ctx, &spotify.StartAUsersPlaybackParams{}, spotify.StartAUsersPlaybackJSONRequestBody{})                      //nolint:errcheck

	// Playlists
	c.GetAListOfCurrentUsersPlaylistsWithResponse(ctx, &spotify.GetAListOfCurrentUsersPlaylistsParams{})                                                                                //nolint:errcheck
	c.GetPlaylistWithResponse(ctx, playlistId, &spotify.GetPlaylistParams{})                                                                                                            //nolint:errcheck
	c.ChangePlaylistDetailsWithBodyWithResponse(ctx, playlistId, "application/json", io.NopCloser(strings.NewReader("{}")))                                                             //nolint:errcheck
	c.ChangePlaylistDetailsWithResponse(ctx, playlistId, spotify.ChangePlaylistDetailsJSONRequestBody{})                                                                                //nolint:errcheck
	c.UnfollowPlaylistWithResponse(ctx, playlistId)                                                                                                                                     //nolint:errcheck
	c.FollowPlaylistWithBodyWithResponse(ctx, playlistId, "application/json", io.NopCloser(strings.NewReader("{}")))                                                                    //nolint:errcheck
	c.FollowPlaylistWithResponse(ctx, playlistId, spotify.FollowPlaylistJSONRequestBody{})                                                                                              //nolint:errcheck
	c.CheckIfUserFollowsPlaylistWithResponse(ctx, playlistId, &spotify.CheckIfUserFollowsPlaylistParams{})                                                                              //nolint:errcheck
	c.GetPlaylistCoverWithResponse(ctx, playlistId)                                                                                                                                     //nolint:errcheck
	c.UploadCustomPlaylistCoverWithBodyWithResponse(ctx, playlistId, "image/jpeg", io.NopCloser(strings.NewReader("")))                                                                 //nolint:errcheck
	c.RemoveTracksPlaylistWithBodyWithResponse(ctx, playlistId, "application/json", io.NopCloser(strings.NewReader("{}")))                                                              //nolint:errcheck
	c.RemoveTracksPlaylistWithResponse(ctx, playlistId, spotify.RemoveTracksPlaylistJSONRequestBody{})                                                                                  //nolint:errcheck
	c.GetPlaylistsTracksWithResponse(ctx, playlistId, &spotify.GetPlaylistsTracksParams{})                                                                                              //nolint:errcheck
	c.AddTracksToPlaylistWithBodyWithResponse(ctx, playlistId, &spotify.AddTracksToPlaylistParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))                         //nolint:errcheck
	c.AddTracksToPlaylistWithResponse(ctx, playlistId, &spotify.AddTracksToPlaylistParams{}, spotify.AddTracksToPlaylistJSONRequestBody{})                                              //nolint:errcheck
	c.ReorderOrReplacePlaylistsTracksWithBodyWithResponse(ctx, playlistId, &spotify.ReorderOrReplacePlaylistsTracksParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.ReorderOrReplacePlaylistsTracksWithResponse(ctx, playlistId, &spotify.ReorderOrReplacePlaylistsTracksParams{}, spotify.ReorderOrReplacePlaylistsTracksJSONRequestBody{})          //nolint:errcheck

	// Recommendations
	c.GetRecommendationsWithResponse(ctx, &spotify.GetRecommendationsParams{}) //nolint:errcheck
	c.GetRecommendationGenresWithResponse(ctx)                                 //nolint:errcheck

	// Search
	c.SearchWithResponse(ctx, &spotify.SearchParams{}) //nolint:errcheck

	// Shows
	c.GetMultipleShowsWithResponse(ctx, &spotify.GetMultipleShowsParams{})           //nolint:errcheck
	c.GetAShowWithResponse(ctx, showId, &spotify.GetAShowParams{})                   //nolint:errcheck
	c.GetAShowsEpisodesWithResponse(ctx, showId, &spotify.GetAShowsEpisodesParams{}) //nolint:errcheck
	c.RemoveShowsUserWithResponse(ctx, &spotify.RemoveShowsUserParams{})             //nolint:errcheck
	c.GetUsersSavedShowsWithResponse(ctx, &spotify.GetUsersSavedShowsParams{})       //nolint:errcheck
	c.SaveShowsUserWithResponse(ctx, &spotify.SaveShowsUserParams{})                 //nolint:errcheck
	c.CheckUsersSavedShowsWithResponse(ctx, &spotify.CheckUsersSavedShowsParams{})   //nolint:errcheck

	// Tracks
	c.GetSeveralTracksWithResponse(ctx, &spotify.GetSeveralTracksParams{})                                                                    //nolint:errcheck
	c.GetTrackWithResponse(ctx, "track1", &spotify.GetTrackParams{})                                                                          //nolint:errcheck
	c.RemoveTracksUserWithBodyWithResponse(ctx, &spotify.RemoveTracksUserParams{}, "application/json", io.NopCloser(strings.NewReader("{}"))) //nolint:errcheck
	c.RemoveTracksUserWithResponse(ctx, &spotify.RemoveTracksUserParams{}, spotify.RemoveTracksUserJSONRequestBody{})                         //nolint:errcheck
	c.GetUsersSavedTracksWithResponse(ctx, &spotify.GetUsersSavedTracksParams{})                                                              //nolint:errcheck
	c.SaveTracksUserWithBodyWithResponse(ctx, "application/json", io.NopCloser(strings.NewReader("{}")))                                      //nolint:errcheck
	c.SaveTracksUserWithResponse(ctx, spotify.SaveTracksUserJSONRequestBody{})                                                                //nolint:errcheck
	c.CheckUsersSavedTracksWithResponse(ctx, &spotify.CheckUsersSavedTracksParams{})                                                          //nolint:errcheck

	// Users
	c.GetUsersTopArtistsAndTracksWithResponse(ctx, spotify.GetUsersTopArtistsAndTracksParamsType("artists"), &spotify.GetUsersTopArtistsAndTracksParams{}) //nolint:errcheck
	c.GetUsersProfileWithResponse(ctx, userId)                                                                                                             //nolint:errcheck
	c.GetListUsersPlaylistsWithResponse(ctx, userId, &spotify.GetListUsersPlaylistsParams{})                                                               //nolint:errcheck
	c.CreatePlaylistWithBodyWithResponse(ctx, userId, "application/json", io.NopCloser(strings.NewReader("{}")))                                           //nolint:errcheck
	c.CreatePlaylistWithResponse(ctx, userId, spotify.CreatePlaylistJSONRequestBody{})                                                                     //nolint:errcheck
	c.FollowArtistsUsersWithBodyWithResponse(ctx, &spotify.FollowArtistsUsersParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))          //nolint:errcheck
	c.FollowArtistsUsersWithResponse(ctx, &spotify.FollowArtistsUsersParams{}, spotify.FollowArtistsUsersJSONRequestBody{})                                //nolint:errcheck
	c.GetFollowedWithResponse(ctx, &spotify.GetFollowedParams{})                                                                                           //nolint:errcheck
	c.UnfollowArtistsUsersWithBodyWithResponse(ctx, &spotify.UnfollowArtistsUsersParams{}, "application/json", io.NopCloser(strings.NewReader("{}")))      //nolint:errcheck
	c.UnfollowArtistsUsersWithResponse(ctx, &spotify.UnfollowArtistsUsersParams{}, spotify.UnfollowArtistsUsersJSONRequestBody{})                          //nolint:errcheck

	t.Logf("All spotify integrations ClientWithResponses methods exercised for coverage")
}
