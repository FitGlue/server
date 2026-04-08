package server

import "os"

type OAuthProviderConfig struct {
	AuthURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

func GetOAuthConfig(provider string) *OAuthProviderConfig {
	switch provider {
	case "strava":
		return &OAuthProviderConfig{
			AuthURL:      "https://www.strava.com/oauth/authorize",
			TokenURL:     "https://www.strava.com/oauth/token",
			ClientID:     os.Getenv("STRAVA_CLIENT_ID"),
			ClientSecret: os.Getenv("STRAVA_CLIENT_SECRET"),
			Scopes:       []string{"read_all", "activity:read_all", "activity:write"},
		}
	case "fitbit":
		return &OAuthProviderConfig{
			AuthURL:      "https://www.fitbit.com/oauth2/authorize",
			TokenURL:     "https://api.fitbit.com/oauth2/token",
			ClientID:     os.Getenv("FITBIT_CLIENT_ID"),
			ClientSecret: os.Getenv("FITBIT_CLIENT_SECRET"),
			Scopes:       []string{"activity", "profile", "heartrate"},
		}
	case "oura":
		return &OAuthProviderConfig{
			AuthURL:      "https://cloud.ouraring.com/oauth/authorize",
			TokenURL:     "https://api.ouraring.com/oauth/token",
			ClientID:     os.Getenv("OURA_CLIENT_ID"),
			ClientSecret: os.Getenv("OURA_CLIENT_SECRET"),
			Scopes:       []string{"daily", "heartrate", "personal", "session", "workout"},
		}
	case "polar":
		return &OAuthProviderConfig{
			AuthURL:      "https://flow.polar.com/oauth2/authorization",
			TokenURL:     "https://polarremote.com/v2/oauth2/token",
			ClientID:     os.Getenv("POLAR_CLIENT_ID"),
			ClientSecret: os.Getenv("POLAR_CLIENT_SECRET"),
			Scopes:       []string{"access_link"},
		}
	case "wahoo":
		return &OAuthProviderConfig{
			AuthURL:      "https://api.wahooligan.com/oauth/authorize",
			TokenURL:     "https://api.wahooligan.com/oauth/token",
			ClientID:     os.Getenv("WAHOO_CLIENT_ID"),
			ClientSecret: os.Getenv("WAHOO_CLIENT_SECRET"),
			Scopes:       []string{"workouts_read"},
		}
	case "spotify":
		return &OAuthProviderConfig{
			AuthURL:      "https://accounts.spotify.com/authorize",
			TokenURL:     "https://accounts.spotify.com/api/token",
			ClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
			ClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
			Scopes:       []string{"user-read-recently-played", "user-read-playback-state"},
		}
	case "github":
		return &OAuthProviderConfig{
			AuthURL:      "https://github.com/login/oauth/authorize",
			TokenURL:     "https://github.com/login/oauth/access_token",
			ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
			ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
			Scopes:       []string{"read:user"},
		}
	}
	return nil
}
