package firestore

import (
	"time"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper to safely get string from map
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// Helper to convert string to pointer, returns nil for empty strings
func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Helper to safely get bool from map
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// Helper to safely get time from map (handles time.Time from Firestore)
func getTime(m map[string]interface{}, key string) *timestamppb.Timestamp {
	if v, ok := m[key]; ok {
		if t, ok := v.(time.Time); ok {
			return timestamppb.New(t)
		}
	}
	return nil
}

// --- UserRecord Converters ---

func UserToFirestore(u *pb.UserRecord) map[string]interface{} {
	m := map[string]interface{}{
		"user_id":    u.UserId,
		"created_at": u.CreatedAt.AsTime(),
	}

	if u.Integrations != nil {
		integrations := make(map[string]interface{})
		if u.Integrations.Hevy != nil {
			integrations["hevy"] = map[string]interface{}{
				"enabled": u.Integrations.Hevy.Enabled,
				"api_key": u.Integrations.Hevy.ApiKey,
				"user_id": u.Integrations.Hevy.UserId,
			}
		}
		if u.Integrations.Fitbit != nil {
			integrations["fitbit"] = map[string]interface{}{
				"enabled":        u.Integrations.Fitbit.Enabled,
				"access_token":   u.Integrations.Fitbit.AccessToken,
				"refresh_token":  u.Integrations.Fitbit.RefreshToken,
				"expires_at":     u.Integrations.Fitbit.ExpiresAt.AsTime(),
				"fitbit_user_id": u.Integrations.Fitbit.FitbitUserId,
			}
		}
		if u.Integrations.Strava != nil {
			integrations["strava"] = map[string]interface{}{
				"enabled":       u.Integrations.Strava.Enabled,
				"access_token":  u.Integrations.Strava.AccessToken,
				"refresh_token": u.Integrations.Strava.RefreshToken,
				"expires_at":    u.Integrations.Strava.ExpiresAt.AsTime(),
				"athlete_id":    u.Integrations.Strava.AthleteId,
			}
		}
		if u.Integrations.Parkrun != nil {
			integrations["parkrun"] = map[string]interface{}{
				"enabled":       u.Integrations.Parkrun.Enabled,
				"athlete_id":    u.Integrations.Parkrun.AthleteId,
				"country_url":   u.Integrations.Parkrun.CountryUrl,
				"consent_given": u.Integrations.Parkrun.ConsentGiven,
				"created_at":    u.Integrations.Parkrun.CreatedAt.AsTime(),
				"last_used_at":  u.Integrations.Parkrun.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Spotify != nil {
			integrations["spotify"] = map[string]interface{}{
				"enabled":         u.Integrations.Spotify.Enabled,
				"access_token":    u.Integrations.Spotify.AccessToken,
				"refresh_token":   u.Integrations.Spotify.RefreshToken,
				"expires_at":      u.Integrations.Spotify.ExpiresAt.AsTime(),
				"spotify_user_id": u.Integrations.Spotify.SpotifyUserId,
				"created_at":      u.Integrations.Spotify.CreatedAt.AsTime(),
				"last_used_at":    u.Integrations.Spotify.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Trainingpeaks != nil {
			integrations["trainingpeaks"] = map[string]interface{}{
				"enabled":       u.Integrations.Trainingpeaks.Enabled,
				"access_token":  u.Integrations.Trainingpeaks.AccessToken,
				"refresh_token": u.Integrations.Trainingpeaks.RefreshToken,
				"expires_at":    u.Integrations.Trainingpeaks.ExpiresAt.AsTime(),
				"athlete_id":    u.Integrations.Trainingpeaks.AthleteId,
				"created_at":    u.Integrations.Trainingpeaks.CreatedAt.AsTime(),
				"last_used_at":  u.Integrations.Trainingpeaks.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Intervals != nil {
			integrations["intervals"] = map[string]interface{}{
				"enabled":      u.Integrations.Intervals.Enabled,
				"api_key":      u.Integrations.Intervals.ApiKey,
				"athlete_id":   u.Integrations.Intervals.AthleteId,
				"created_at":   u.Integrations.Intervals.CreatedAt.AsTime(),
				"last_used_at": u.Integrations.Intervals.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Oura != nil {
			integrations["oura"] = map[string]interface{}{
				"enabled":       u.Integrations.Oura.Enabled,
				"access_token":  u.Integrations.Oura.AccessToken,
				"refresh_token": u.Integrations.Oura.RefreshToken,
				"expires_at":    u.Integrations.Oura.ExpiresAt.AsTime(),
				"oura_user_id":  u.Integrations.Oura.OuraUserId,
				"created_at":    u.Integrations.Oura.CreatedAt.AsTime(),
				"last_used_at":  u.Integrations.Oura.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Google != nil {
			integrations["google"] = map[string]interface{}{
				"enabled":        u.Integrations.Google.Enabled,
				"access_token":   u.Integrations.Google.AccessToken,
				"refresh_token":  u.Integrations.Google.RefreshToken,
				"expires_at":     u.Integrations.Google.ExpiresAt.AsTime(),
				"google_user_id": u.Integrations.Google.GoogleUserId,
				"created_at":     u.Integrations.Google.CreatedAt.AsTime(),
				"last_used_at":   u.Integrations.Google.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Polar != nil {
			integrations["polar"] = map[string]interface{}{
				"enabled":       u.Integrations.Polar.Enabled,
				"access_token":  u.Integrations.Polar.AccessToken,
				"refresh_token": u.Integrations.Polar.RefreshToken,
				"expires_at":    u.Integrations.Polar.ExpiresAt.AsTime(),
				"polar_user_id": u.Integrations.Polar.PolarUserId,
				"created_at":    u.Integrations.Polar.CreatedAt.AsTime(),
				"last_used_at":  u.Integrations.Polar.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Wahoo != nil {
			integrations["wahoo"] = map[string]interface{}{
				"enabled":       u.Integrations.Wahoo.Enabled,
				"access_token":  u.Integrations.Wahoo.AccessToken,
				"refresh_token": u.Integrations.Wahoo.RefreshToken,
				"expires_at":    u.Integrations.Wahoo.ExpiresAt.AsTime(),
				"wahoo_user_id": u.Integrations.Wahoo.WahooUserId,
				"created_at":    u.Integrations.Wahoo.CreatedAt.AsTime(),
				"last_used_at":  u.Integrations.Wahoo.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.Github != nil {
			integrations["github"] = map[string]interface{}{
				"enabled":         u.Integrations.Github.Enabled,
				"access_token":    u.Integrations.Github.AccessToken,
				"refresh_token":   u.Integrations.Github.RefreshToken,
				"expires_at":      u.Integrations.Github.ExpiresAt.AsTime(),
				"github_user_id":  u.Integrations.Github.GithubUserId,
				"github_username": u.Integrations.Github.GithubUsername,
				"scope":           u.Integrations.Github.Scope,
				"created_at":      u.Integrations.Github.CreatedAt.AsTime(),
				"last_used_at":    u.Integrations.Github.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.AppleHealth != nil {
			integrations["apple_health"] = map[string]interface{}{
				"enabled":      u.Integrations.AppleHealth.Enabled,
				"created_at":   u.Integrations.AppleHealth.CreatedAt.AsTime(),
				"last_used_at": u.Integrations.AppleHealth.LastUsedAt.AsTime(),
			}
		}
		if u.Integrations.HealthConnect != nil {
			integrations["health_connect"] = map[string]interface{}{
				"enabled":      u.Integrations.HealthConnect.Enabled,
				"created_at":   u.Integrations.HealthConnect.CreatedAt.AsTime(),
				"last_used_at": u.Integrations.HealthConnect.LastUsedAt.AsTime(),
			}
		}
		m["integrations"] = integrations
	}

	if len(u.FcmTokens) > 0 {
		m["fcm_tokens"] = u.FcmTokens
	}

	// Pipelines moved to sub-collection users/{userId}/pipelines

	// Tier management fields
	if u.Tier == pb.UserTier_USER_TIER_ATHLETE {
		m["tier"] = "athlete"
	} else {
		m["tier"] = "hobbyist"
	}
	if u.TrialEndsAt != nil {
		m["trial_ends_at"] = u.TrialEndsAt.AsTime()
	}
	m["is_admin"] = u.IsAdmin
	m["sync_count_this_month"] = u.SyncCountThisMonth
	if u.SyncCountResetAt != nil {
		m["sync_count_reset_at"] = u.SyncCountResetAt.AsTime()
	}
	m["stripe_customer_id"] = u.StripeCustomerId
	m["access_enabled"] = u.AccessEnabled
	m["prevented_sync_count"] = u.PreventedSyncCount

	return m
}

func FirestoreToUser(m map[string]interface{}) *pb.UserRecord {
	u := &pb.UserRecord{
		UserId:    getString(m, "user_id"),
		CreatedAt: getTime(m, "created_at"),
	}

	if iMap, ok := m["integrations"].(map[string]interface{}); ok {
		u.Integrations = &pb.UserIntegrations{}
		if hMap, ok := iMap["hevy"].(map[string]interface{}); ok {
			u.Integrations.Hevy = &pb.HevyIntegration{
				Enabled: getBool(hMap, "enabled"),
				ApiKey:  getString(hMap, "api_key"),
				UserId:  getString(hMap, "user_id"),
			}
		}
		if fMap, ok := iMap["fitbit"].(map[string]interface{}); ok {
			u.Integrations.Fitbit = &pb.FitbitIntegration{
				Enabled:      getBool(fMap, "enabled"),
				AccessToken:  getString(fMap, "access_token"),
				RefreshToken: getString(fMap, "refresh_token"),
				ExpiresAt:    getTime(fMap, "expires_at"),
				FitbitUserId: getString(fMap, "fitbit_user_id"),
			}
		}
		if sMap, ok := iMap["strava"].(map[string]interface{}); ok {
			u.Integrations.Strava = &pb.StravaIntegration{
				Enabled:      getBool(sMap, "enabled"),
				AccessToken:  getString(sMap, "access_token"),
				RefreshToken: getString(sMap, "refresh_token"),
				ExpiresAt:    getTime(sMap, "expires_at"),
			}
			// Safe int64 conversion
			if v, ok := sMap["athlete_id"]; ok {
				// Firestore stores numbers as int64, float64 or int
				switch n := v.(type) {
				case int64:
					u.Integrations.Strava.AthleteId = n
				case int:
					u.Integrations.Strava.AthleteId = int64(n)
				case float64:
					u.Integrations.Strava.AthleteId = int64(n)
				}
			}
		}
		if pMap, ok := iMap["parkrun"].(map[string]interface{}); ok {
			u.Integrations.Parkrun = &pb.ParkrunIntegration{
				Enabled:      getBool(pMap, "enabled"),
				AthleteId:    getString(pMap, "athlete_id"),
				CountryUrl:   getString(pMap, "country_url"),
				ConsentGiven: getBool(pMap, "consent_given"),
				CreatedAt:    getTime(pMap, "created_at"),
				LastUsedAt:   getTime(pMap, "last_used_at"),
			}
		}
		if spMap, ok := iMap["spotify"].(map[string]interface{}); ok {
			u.Integrations.Spotify = &pb.SpotifyIntegration{
				Enabled:       getBool(spMap, "enabled"),
				AccessToken:   getString(spMap, "access_token"),
				RefreshToken:  getString(spMap, "refresh_token"),
				ExpiresAt:     getTime(spMap, "expires_at"),
				SpotifyUserId: getString(spMap, "spotify_user_id"),
				CreatedAt:     getTime(spMap, "created_at"),
				LastUsedAt:    getTime(spMap, "last_used_at"),
			}
		}
		if tpMap, ok := iMap["trainingpeaks"].(map[string]interface{}); ok {
			u.Integrations.Trainingpeaks = &pb.TrainingPeaksIntegration{
				Enabled:      getBool(tpMap, "enabled"),
				AccessToken:  getString(tpMap, "access_token"),
				RefreshToken: getString(tpMap, "refresh_token"),
				ExpiresAt:    getTime(tpMap, "expires_at"),
				AthleteId:    getString(tpMap, "athlete_id"),
				CreatedAt:    getTime(tpMap, "created_at"),
				LastUsedAt:   getTime(tpMap, "last_used_at"),
			}
		}
		if ivMap, ok := iMap["intervals"].(map[string]interface{}); ok {
			u.Integrations.Intervals = &pb.IntervalsIntegration{
				Enabled:    getBool(ivMap, "enabled"),
				ApiKey:     getString(ivMap, "api_key"),
				AthleteId:  getString(ivMap, "athlete_id"),
				CreatedAt:  getTime(ivMap, "created_at"),
				LastUsedAt: getTime(ivMap, "last_used_at"),
			}
		}
		if oMap, ok := iMap["oura"].(map[string]interface{}); ok {
			u.Integrations.Oura = &pb.OuraIntegration{
				Enabled:      getBool(oMap, "enabled"),
				AccessToken:  getString(oMap, "access_token"),
				RefreshToken: getString(oMap, "refresh_token"),
				ExpiresAt:    getTime(oMap, "expires_at"),
				OuraUserId:   getString(oMap, "oura_user_id"),
				CreatedAt:    getTime(oMap, "created_at"),
				LastUsedAt:   getTime(oMap, "last_used_at"),
			}
		}
		if gMap, ok := iMap["google"].(map[string]interface{}); ok {
			u.Integrations.Google = &pb.GoogleIntegration{
				Enabled:      getBool(gMap, "enabled"),
				AccessToken:  getString(gMap, "access_token"),
				RefreshToken: getString(gMap, "refresh_token"),
				ExpiresAt:    getTime(gMap, "expires_at"),
				GoogleUserId: getString(gMap, "google_user_id"),
				CreatedAt:    getTime(gMap, "created_at"),
				LastUsedAt:   getTime(gMap, "last_used_at"),
			}
		}
		if polMap, ok := iMap["polar"].(map[string]interface{}); ok {
			u.Integrations.Polar = &pb.PolarIntegration{
				Enabled:      getBool(polMap, "enabled"),
				AccessToken:  getString(polMap, "access_token"),
				RefreshToken: getString(polMap, "refresh_token"),
				ExpiresAt:    getTime(polMap, "expires_at"),
				PolarUserId:  getString(polMap, "polar_user_id"),
				CreatedAt:    getTime(polMap, "created_at"),
				LastUsedAt:   getTime(polMap, "last_used_at"),
			}
		}
		if wMap, ok := iMap["wahoo"].(map[string]interface{}); ok {
			u.Integrations.Wahoo = &pb.WahooIntegration{
				Enabled:      getBool(wMap, "enabled"),
				AccessToken:  getString(wMap, "access_token"),
				RefreshToken: getString(wMap, "refresh_token"),
				ExpiresAt:    getTime(wMap, "expires_at"),
				WahooUserId:  getString(wMap, "wahoo_user_id"),
				CreatedAt:    getTime(wMap, "created_at"),
				LastUsedAt:   getTime(wMap, "last_used_at"),
			}
		}
		if ghMap, ok := iMap["github"].(map[string]interface{}); ok {
			u.Integrations.Github = &pb.GitHubIntegration{
				Enabled:        getBool(ghMap, "enabled"),
				AccessToken:    getString(ghMap, "access_token"),
				RefreshToken:   getString(ghMap, "refresh_token"),
				ExpiresAt:      getTime(ghMap, "expires_at"),
				GithubUserId:   getString(ghMap, "github_user_id"),
				GithubUsername: getString(ghMap, "github_username"),
				Scope:          getString(ghMap, "scope"),
				CreatedAt:      getTime(ghMap, "created_at"),
				LastUsedAt:     getTime(ghMap, "last_used_at"),
			}
		}
		if ahMap, ok := iMap["apple_health"].(map[string]interface{}); ok {
			u.Integrations.AppleHealth = &pb.AppleHealthIntegration{
				Enabled:    getBool(ahMap, "enabled"),
				CreatedAt:  getTime(ahMap, "created_at"),
				LastUsedAt: getTime(ahMap, "last_used_at"),
			}
		}
		if hcMap, ok := iMap["health_connect"].(map[string]interface{}); ok {
			u.Integrations.HealthConnect = &pb.HealthConnectIntegration{
				Enabled:    getBool(hcMap, "enabled"),
				CreatedAt:  getTime(hcMap, "created_at"),
				LastUsedAt: getTime(hcMap, "last_used_at"),
			}
		}
	}

	// Tier management fields
	if v, ok := m["tier"]; ok {
		switch val := v.(type) {
		case string:
			switch val {
			case "athlete", "pro":
				u.Tier = pb.UserTier_USER_TIER_ATHLETE
			default:
				u.Tier = pb.UserTier_USER_TIER_HOBBYIST
			}
		case int64:
			// Handle legacy numeric values (1=Hobbyist, 2=Athlete)
			if val == 2 {
				u.Tier = pb.UserTier_USER_TIER_ATHLETE
			} else {
				u.Tier = pb.UserTier_USER_TIER_HOBBYIST
			}
		case int:
			if val == 2 {
				u.Tier = pb.UserTier_USER_TIER_ATHLETE
			} else {
				u.Tier = pb.UserTier_USER_TIER_HOBBYIST
			}
		case float64:
			if val == 2 {
				u.Tier = pb.UserTier_USER_TIER_ATHLETE
			} else {
				u.Tier = pb.UserTier_USER_TIER_HOBBYIST
			}
		default:
			u.Tier = pb.UserTier_USER_TIER_HOBBYIST
		}
	} else {
		u.Tier = pb.UserTier_USER_TIER_HOBBYIST
	}
	u.IsAdmin = getBool(m, "is_admin")
	u.AccessEnabled = getBool(m, "access_enabled")
	u.StripeCustomerId = getString(m, "stripe_customer_id")
	u.TrialEndsAt = getTime(m, "trial_ends_at")
	u.SyncCountResetAt = getTime(m, "sync_count_reset_at")

	if v, ok := m["sync_count_this_month"]; ok {
		switch n := v.(type) {
		case int64:
			u.SyncCountThisMonth = int32(n)
		case int:
			u.SyncCountThisMonth = int32(n)
		case float64:
			u.SyncCountThisMonth = int32(n)
		}
	}

	if v, ok := m["prevented_sync_count"]; ok {
		switch n := v.(type) {
		case int64:
			u.PreventedSyncCount = int32(n)
		case int:
			u.PreventedSyncCount = int32(n)
		case float64:
			u.PreventedSyncCount = int32(n)
		}
	}

	if tokens, ok := m["fcm_tokens"].([]interface{}); ok {
		u.FcmTokens = make([]string, len(tokens))
		for i, v := range tokens {
			if s, ok := v.(string); ok {
				u.FcmTokens[i] = s
			}
		}
	} else if tokens, ok := m["fcm_tokens"].([]string); ok {
		u.FcmTokens = tokens
	}

	// Pipelines moved to sub-collection users/{userId}/pipelines

	return u
}

// --- PipelineConfig Converters ---

func PipelineToFirestore(p *pb.PipelineConfig) map[string]interface{} {
	enrichers := make([]map[string]interface{}, len(p.Enrichers))
	for i, e := range p.Enrichers {
		enrichers[i] = map[string]interface{}{
			"provider_type": int32(e.ProviderType),
			"typed_config":  e.TypedConfig,
		}
	}

	m := map[string]interface{}{
		"id":           p.Id,
		"name":         p.Name,
		"source":       p.Source,
		"destinations": p.Destinations,
		"enrichers":    enrichers,
		"disabled":     p.Disabled,
	}

	// Source config
	if len(p.SourceConfig) > 0 {
		m["source_config"] = p.SourceConfig
	}

	// Destination configs
	if len(p.DestinationConfigs) > 0 {
		destConfigs := make(map[string]interface{})
		for k, v := range p.DestinationConfigs {
			if v != nil {
				destConfigs[k] = map[string]interface{}{
					"config": v.Config,
				}
			}
		}
		m["destination_configs"] = destConfigs
	}

	return m
}

func FirestoreToPipeline(m map[string]interface{}) *pb.PipelineConfig {
	// Enrichers
	var enrichers []*pb.EnricherConfig
	if eList, ok := m["enrichers"].([]interface{}); ok {
		enrichers = make([]*pb.EnricherConfig, len(eList))
		for j, eRaw := range eList {
			if eMap, ok := eRaw.(map[string]interface{}); ok {
				// TypedConfig
				typedConfig := make(map[string]string)
				if cMap, ok := eMap["typed_config"].(map[string]interface{}); ok {
					for k, v := range cMap {
						if s, ok := v.(string); ok {
							typedConfig[k] = s
						}
					}
				}

				ptype := pb.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED
				if v, ok := eMap["provider_type"]; ok {
					switch n := v.(type) {
					case int64:
						ptype = pb.EnricherProviderType(n)
					case int:
						ptype = pb.EnricherProviderType(n)
					case float64:
						ptype = pb.EnricherProviderType(int32(n))
					}
				}

				enrichers[j] = &pb.EnricherConfig{
					ProviderType: ptype,
					TypedConfig:  typedConfig,
				}
			}
		}
	}

	// Destinations - handle both legacy strings and new enum ints
	var dests []pb.Destination
	if dList, ok := m["destinations"].([]interface{}); ok {
		for _, d := range dList {
			switch val := d.(type) {
			case int64:
				dests = append(dests, pb.Destination(val))
			case int:
				dests = append(dests, pb.Destination(val))
			case float64:
				dests = append(dests, pb.Destination(int32(val)))
			case string:
				// Legacy string support
				switch val {
				case "strava", "DESTINATION_STRAVA":
					dests = append(dests, pb.Destination_DESTINATION_STRAVA)
				case "mock", "DESTINATION_MOCK":
					dests = append(dests, pb.Destination_DESTINATION_MOCK)
				}
			}
		}
	}

	// Source config
	sourceConfig := make(map[string]string)
	if scMap, ok := m["source_config"].(map[string]interface{}); ok {
		for k, v := range scMap {
			if s, ok := v.(string); ok {
				sourceConfig[k] = s
			}
		}
	}

	// Destination configs
	destConfigs := make(map[string]*pb.DestinationConfig)
	if dcMap, ok := m["destination_configs"].(map[string]interface{}); ok {
		for destId, dcRaw := range dcMap {
			if dcObj, ok := dcRaw.(map[string]interface{}); ok {
				cfg := make(map[string]string)
				if cMap, ok := dcObj["config"].(map[string]interface{}); ok {
					for k, v := range cMap {
						if s, ok := v.(string); ok {
							cfg[k] = s
						}
					}
				}
				destConfigs[destId] = &pb.DestinationConfig{Config: cfg}
			}
		}
	}

	return &pb.PipelineConfig{
		Id:                 getString(m, "id"),
		Name:               getString(m, "name"),
		Source:             getString(m, "source"),
		Enrichers:          enrichers,
		Destinations:       dests,
		Disabled:           getBool(m, "disabled"),
		SourceConfig:       sourceConfig,
		DestinationConfigs: destConfigs,
	}
}

// --- Execution Record ---

func ExecutionToFirestore(e *pb.ExecutionRecord) map[string]interface{} {
	m := map[string]interface{}{
		"execution_id":          e.ExecutionId,
		"service":               e.Service,
		"status":                int32(e.Status), // Store enum as int or string? Protocol is int usually, logger used String()
		"timestamp":             e.Timestamp.AsTime(),
		"user_id":               e.UserId,
		"test_run_id":           e.TestRunId,
		"trigger_type":          e.TriggerType,
		"start_time":            e.StartTime.AsTime(),
		"end_time":              e.EndTime.AsTime(),
		"error_message":         e.ErrorMessage,
		"inputs_json":           e.InputsJson,
		"outputs_json":          e.OutputsJson,
		"pipeline_execution_id": e.PipelineExecutionId,
	}
	return m
}

func FirestoreToExecution(m map[string]interface{}) *pb.ExecutionRecord {
	e := &pb.ExecutionRecord{
		ExecutionId:         getString(m, "execution_id"),
		Service:             getString(m, "service"),
		Timestamp:           getTime(m, "timestamp"),
		TriggerType:         getString(m, "trigger_type"), // Required field, not a pointer
		UserId:              stringPtrOrNil(getString(m, "user_id")),
		TestRunId:           stringPtrOrNil(getString(m, "test_run_id")),
		StartTime:           getTime(m, "start_time"),
		EndTime:             getTime(m, "end_time"),
		ErrorMessage:        stringPtrOrNil(getString(m, "error_message")),
		InputsJson:          stringPtrOrNil(getString(m, "inputs_json")),
		OutputsJson:         stringPtrOrNil(getString(m, "outputs_json")),
		PipelineExecutionId: stringPtrOrNil(getString(m, "pipeline_execution_id")),
	}

	if v, ok := m["status"]; ok {
		// Handle int or string legacy
		switch val := v.(type) {
		case int64:
			e.Status = pb.ExecutionStatus(val)
		case int:
			e.Status = pb.ExecutionStatus(int32(val))
		case string:
			// Use proto-generated map for all status values
			if enumVal, ok := pb.ExecutionStatus_value[val]; ok {
				e.Status = pb.ExecutionStatus(enumVal)
			} else {
				e.Status = pb.ExecutionStatus_STATUS_UNKNOWN
			}
		}
	}

	return e
}

// --- Counter Converters ---

func CounterToFirestore(c *pb.Counter) map[string]interface{} {
	return map[string]interface{}{
		"id":           c.Id,
		"count":        c.Count,
		"last_updated": c.LastUpdated.AsTime(),
	}
}

func FirestoreToCounter(m map[string]interface{}) *pb.Counter {
	c := &pb.Counter{
		Id:          getString(m, "id"),
		LastUpdated: getTime(m, "last_updated"),
	}
	// Handle number types
	if v, ok := m["count"]; ok {
		switch n := v.(type) {
		case int64:
			c.Count = n
		case int:
			c.Count = int64(n)
		case float64:
			c.Count = int64(n)
		}
	}
	return c
}

// --- PersonalRecord Converters ---

func PersonalRecordToFirestore(r *pb.PersonalRecord) map[string]interface{} {
	m := map[string]interface{}{
		"record_type":   r.RecordType,
		"value":         r.Value,
		"unit":          r.Unit,
		"activity_id":   r.ActivityId,
		"achieved_at":   r.AchievedAt.AsTime(),
		"activity_type": int32(r.ActivityType),
	}
	if r.PreviousValue != nil {
		m["previous_value"] = *r.PreviousValue
	}
	if r.Improvement != nil {
		m["improvement"] = *r.Improvement
	}
	return m
}

func FirestoreToPersonalRecord(m map[string]interface{}) *pb.PersonalRecord {
	r := &pb.PersonalRecord{
		RecordType: getString(m, "record_type"),
		Unit:       getString(m, "unit"),
		ActivityId: getString(m, "activity_id"),
		AchievedAt: getTime(m, "achieved_at"),
	}

	// Value
	if v, ok := m["value"]; ok {
		switch n := v.(type) {
		case float64:
			r.Value = n
		case int64:
			r.Value = float64(n)
		case int:
			r.Value = float64(n)
		}
	}

	// Activity type
	if v, ok := m["activity_type"]; ok {
		switch val := v.(type) {
		case int64:
			r.ActivityType = pb.ActivityType(val)
		case int:
			r.ActivityType = pb.ActivityType(int32(val))
		case float64:
			r.ActivityType = pb.ActivityType(int32(val))
		}
	}

	// Optional previous_value
	if v, ok := m["previous_value"]; ok {
		switch n := v.(type) {
		case float64:
			r.PreviousValue = &n
		case int64:
			f := float64(n)
			r.PreviousValue = &f
		case int:
			f := float64(n)
			r.PreviousValue = &f
		}
	}

	// Optional improvement
	if v, ok := m["improvement"]; ok {
		switch n := v.(type) {
		case float64:
			r.Improvement = &n
		case int64:
			f := float64(n)
			r.Improvement = &f
		case int:
			f := float64(n)
			r.Improvement = &f
		}
	}

	return r
}

// --- PendingInput Converters ---

func PendingInputToFirestore(p *pb.PendingInput) map[string]interface{} {
	m := map[string]interface{}{
		"activity_id":                  p.ActivityId,
		"user_id":                      p.UserId,
		"status":                       int32(p.Status),
		"required_fields":              p.RequiredFields,
		"input_data":                   p.InputData,
		"created_at":                   p.CreatedAt.AsTime(),
		"updated_at":                   p.UpdatedAt.AsTime(),
		"completed_at":                 p.CompletedAt.AsTime(),
		"auto_populated":               p.AutoPopulated,
		"continued_without_resolution": p.ContinuedWithoutResolution,
		"enricher_provider_id":         p.EnricherProviderId,
		"linked_activity_id":           p.LinkedActivityId,
		"pipeline_id":                  p.PipelineId,
	}

	if p.AutoDeadline != nil {
		m["auto_deadline"] = p.AutoDeadline.AsTime()
	}

	if p.OriginalPayloadUri != "" {
		m["original_payload_uri"] = p.OriginalPayloadUri
	}

	// Provider metadata
	if len(p.ProviderMetadata) > 0 {
		m["provider_metadata"] = p.ProviderMetadata
	}
	return m
}

func FirestoreToPendingInput(m map[string]interface{}) *pb.PendingInput {
	p := &pb.PendingInput{
		ActivityId: getString(m, "activity_id"),
		UserId:     getString(m, "user_id"),
		Status:     pb.PendingInput_Status(m["status"].(int64)),
		RequiredFields: func() []string {
			if v, ok := m["required_fields"].([]string); ok {
				return v
			}
			// Handle []interface{} from Firestore
			if v, ok := m["required_fields"].([]interface{}); ok {
				strs := make([]string, len(v))
				for i, s := range v {
					if str, ok := s.(string); ok {
						strs[i] = str
					}
				}
				return strs
			}
			return nil
		}(),
		InputData: func() map[string]string {
			if v, ok := m["input_data"].(map[string]interface{}); ok {
				out := make(map[string]string)
				for k, val := range v {
					if str, ok := val.(string); ok {
						out[k] = str
					}
				}
				return out
			}
			return nil
		}(),
		CreatedAt:                  getTime(m, "created_at"),
		UpdatedAt:                  getTime(m, "updated_at"),
		CompletedAt:                getTime(m, "completed_at"),
		AutoPopulated:              getBool(m, "auto_populated"),
		ContinuedWithoutResolution: getBool(m, "continued_without_resolution"),
		EnricherProviderId:         getString(m, "enricher_provider_id"),
		AutoDeadline:               getTime(m, "auto_deadline"),
		LinkedActivityId:           getString(m, "linked_activity_id"),
		PipelineId:                 getString(m, "pipeline_id"),
		OriginalPayloadUri:         getString(m, "original_payload_uri"),
	}

	if v, ok := m["status"]; ok {
		switch n := v.(type) {
		case int64:
			p.Status = pb.PendingInput_Status(n)
		case int:
			p.Status = pb.PendingInput_Status(int32(n))
		}
	}

	// Note: original_payload is now stored in GCS via original_payload_uri

	// Provider metadata
	if v, ok := m["provider_metadata"].(map[string]interface{}); ok {
		p.ProviderMetadata = make(map[string]string)
		for k, val := range v {
			if str, ok := val.(string); ok {
				p.ProviderMetadata[k] = str
			}
		}
	}
	return p
}

// --- ShowcasedActivity Converters ---

func ShowcasedActivityToFirestore(s *pb.ShowcasedActivity) map[string]interface{} {
	m := map[string]interface{}{
		"showcase_id":         s.ShowcaseId,
		"activity_id":         s.ActivityId,
		"user_id":             s.UserId,
		"title":               s.Title,
		"description":         s.Description,
		"activity_type":       int32(s.ActivityType),
		"source":              int32(s.Source),
		"applied_enrichments": s.AppliedEnrichments,
		"enrichment_metadata": s.EnrichmentMetadata,
		"tags":                s.Tags,
		"fit_file_uri":        s.FitFileUri,
		"owner_display_name":  s.OwnerDisplayName,
	}

	if s.StartTime != nil {
		m["start_time"] = s.StartTime.AsTime()
	}
	if s.CreatedAt != nil {
		m["created_at"] = s.CreatedAt.AsTime()
	}
	if s.ExpiresAt != nil {
		m["expires_at"] = s.ExpiresAt.AsTime()
	}
	if s.PipelineExecutionId != nil {
		m["pipeline_execution_id"] = *s.PipelineExecutionId
	}

	// Activity data is stored in GCS, not inline (avoids Firestore 1MB field limit)
	// Only store the URI reference
	if s.ActivityDataUri != "" {
		m["activity_data_uri"] = s.ActivityDataUri
	}

	return m
}

func FirestoreToShowcasedActivity(m map[string]interface{}) *pb.ShowcasedActivity {
	s := &pb.ShowcasedActivity{
		ShowcaseId:          getString(m, "showcase_id"),
		ActivityId:          getString(m, "activity_id"),
		UserId:              getString(m, "user_id"),
		Title:               getString(m, "title"),
		Description:         getString(m, "description"),
		FitFileUri:          getString(m, "fit_file_uri"),
		ActivityDataUri:     getString(m, "activity_data_uri"), // GCS URI for activity JSON
		StartTime:           getTime(m, "start_time"),
		CreatedAt:           getTime(m, "created_at"),
		ExpiresAt:           getTime(m, "expires_at"),
		PipelineExecutionId: stringPtrOrNil(getString(m, "pipeline_execution_id")),
		OwnerDisplayName:    getString(m, "owner_display_name"),
	}

	// ActivityType
	if v, ok := m["activity_type"]; ok {
		switch val := v.(type) {
		case int64:
			s.ActivityType = pb.ActivityType(val)
		case int:
			s.ActivityType = pb.ActivityType(int32(val))
		case float64:
			s.ActivityType = pb.ActivityType(int32(val))
		}
	}

	// Source
	if v, ok := m["source"]; ok {
		switch val := v.(type) {
		case int64:
			s.Source = pb.ActivitySource(val)
		case int:
			s.Source = pb.ActivitySource(int32(val))
		case float64:
			s.Source = pb.ActivitySource(int32(val))
		}
	}

	// Applied enrichments
	if v, ok := m["applied_enrichments"].([]interface{}); ok {
		s.AppliedEnrichments = make([]string, len(v))
		for i, val := range v {
			if str, ok := val.(string); ok {
				s.AppliedEnrichments[i] = str
			}
		}
	}

	// Tags
	if v, ok := m["tags"].([]interface{}); ok {
		s.Tags = make([]string, len(v))
		for i, val := range v {
			if str, ok := val.(string); ok {
				s.Tags[i] = str
			}
		}
	}

	// Enrichment metadata
	if v, ok := m["enrichment_metadata"].(map[string]interface{}); ok {
		s.EnrichmentMetadata = make(map[string]string)
		for k, val := range v {
			if str, ok := val.(string); ok {
				s.EnrichmentMetadata[k] = str
			}
		}
	}

	// Activity data - deserialize from JSON
	if v, ok := m["activity_data"]; ok {
		var jsonStr string
		switch val := v.(type) {
		case string:
			jsonStr = val
		case []byte:
			jsonStr = string(val)
		}
		if jsonStr != "" {
			var data pb.StandardizedActivity
			if err := protojson.Unmarshal([]byte(jsonStr), &data); err == nil {
				s.ActivityData = &data
			}
		}
	}

	return s
}

// --- ShowcaseProfile Converters ---

func ShowcaseProfileEntryToFirestore(e *pb.ShowcaseProfileEntry) map[string]interface{} {
	m := map[string]interface{}{
		"showcase_id":   e.ShowcaseId,
		"title":         e.Title,
		"activity_type": int32(e.ActivityType),
		"source":        int32(e.Source),
	}
	if e.StartTime != nil {
		m["start_time"] = e.StartTime.AsTime()
	}
	if e.RouteThumbnailUrl != "" {
		m["route_thumbnail_url"] = e.RouteThumbnailUrl
	}
	if e.DistanceMeters != 0 {
		m["distance_meters"] = e.DistanceMeters
	}
	if e.DurationSeconds != 0 {
		m["duration_seconds"] = e.DurationSeconds
	}
	if e.TotalSets != 0 {
		m["total_sets"] = e.TotalSets
	}
	if e.TotalReps != 0 {
		m["total_reps"] = e.TotalReps
	}
	if e.TotalWeightKg != 0 {
		m["total_weight_kg"] = e.TotalWeightKg
	}
	return m
}

func FirestoreToShowcaseProfileEntry(m map[string]interface{}) *pb.ShowcaseProfileEntry {
	e := &pb.ShowcaseProfileEntry{
		ShowcaseId:        getString(m, "showcase_id"),
		Title:             getString(m, "title"),
		RouteThumbnailUrl: getString(m, "route_thumbnail_url"),
		StartTime:         getTime(m, "start_time"),
	}

	// ActivityType
	if v, ok := m["activity_type"]; ok {
		switch val := v.(type) {
		case int64:
			e.ActivityType = pb.ActivityType(val)
		case float64:
			e.ActivityType = pb.ActivityType(int32(val))
		}
	}

	// Source
	if v, ok := m["source"]; ok {
		switch val := v.(type) {
		case int64:
			e.Source = pb.ActivitySource(val)
		case float64:
			e.Source = pb.ActivitySource(int32(val))
		}
	}

	// DistanceMeters
	if v, ok := m["distance_meters"]; ok {
		switch n := v.(type) {
		case float64:
			e.DistanceMeters = n
		case int64:
			e.DistanceMeters = float64(n)
		}
	}

	// DurationSeconds
	if v, ok := m["duration_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			e.DurationSeconds = n
		case int64:
			e.DurationSeconds = float64(n)
		}
	}

	// TotalSets
	if v, ok := m["total_sets"]; ok {
		switch n := v.(type) {
		case int64:
			e.TotalSets = int32(n)
		case float64:
			e.TotalSets = int32(n)
		}
	}

	// TotalReps
	if v, ok := m["total_reps"]; ok {
		switch n := v.(type) {
		case int64:
			e.TotalReps = int32(n)
		case float64:
			e.TotalReps = int32(n)
		}
	}

	// TotalWeightKg
	if v, ok := m["total_weight_kg"]; ok {
		switch n := v.(type) {
		case float64:
			e.TotalWeightKg = n
		case int64:
			e.TotalWeightKg = float64(n)
		}
	}

	return e
}

func ShowcaseProfileToFirestore(p *pb.ShowcaseProfile) map[string]interface{} {
	entries := make([]map[string]interface{}, len(p.Entries))
	for i, e := range p.Entries {
		entries[i] = ShowcaseProfileEntryToFirestore(e)
	}

	m := map[string]interface{}{
		"slug":                   p.Slug,
		"user_id":                p.UserId,
		"display_name":           p.DisplayName,
		"entries":                entries,
		"total_activities":       p.TotalActivities,
		"total_distance_meters":  p.TotalDistanceMeters,
		"total_duration_seconds": p.TotalDurationSeconds,
		"total_sets":             p.TotalSets,
		"total_reps":             p.TotalReps,
		"total_weight_kg":        p.TotalWeightKg,
		"subtitle":               p.Subtitle,
		"bio":                    p.Bio,
		"profile_picture_url":    p.ProfilePictureUrl,
		"visible":                p.Visible,
	}

	if p.LatestActivityAt != nil {
		m["latest_activity_at"] = p.LatestActivityAt.AsTime()
	}
	if p.CreatedAt != nil {
		m["created_at"] = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		m["updated_at"] = p.UpdatedAt.AsTime()
	}

	return m
}

func FirestoreToShowcaseProfile(m map[string]interface{}) *pb.ShowcaseProfile {
	p := &pb.ShowcaseProfile{
		Slug:              getString(m, "slug"),
		UserId:            getString(m, "user_id"),
		DisplayName:       getString(m, "display_name"),
		LatestActivityAt:  getTime(m, "latest_activity_at"),
		CreatedAt:         getTime(m, "created_at"),
		UpdatedAt:         getTime(m, "updated_at"),
		Subtitle:          getString(m, "subtitle"),
		Bio:               getString(m, "bio"),
		ProfilePictureUrl: getString(m, "profile_picture_url"),
	}

	// Visible defaults to true for backward compat
	if v, ok := m["visible"]; ok {
		if b, ok := v.(bool); ok {
			p.Visible = b
		}
	} else {
		p.Visible = true
	}

	// TotalActivities
	if v, ok := m["total_activities"]; ok {
		switch n := v.(type) {
		case int64:
			p.TotalActivities = int32(n)
		case float64:
			p.TotalActivities = int32(n)
		}
	}

	// TotalDistanceMeters
	if v, ok := m["total_distance_meters"]; ok {
		switch n := v.(type) {
		case float64:
			p.TotalDistanceMeters = n
		case int64:
			p.TotalDistanceMeters = float64(n)
		}
	}

	// TotalDurationSeconds
	if v, ok := m["total_duration_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			p.TotalDurationSeconds = n
		case int64:
			p.TotalDurationSeconds = float64(n)
		}
	}

	// Entries
	if eList, ok := m["entries"].([]interface{}); ok {
		p.Entries = make([]*pb.ShowcaseProfileEntry, len(eList))
		for i, eRaw := range eList {
			if eMap, ok := eRaw.(map[string]interface{}); ok {
				p.Entries[i] = FirestoreToShowcaseProfileEntry(eMap)
			}
		}
	}

	return p
}

// --- PluginDefault Converters ---

func PluginDefaultToFirestore(p *pb.PluginDefault) map[string]interface{} {
	m := map[string]interface{}{
		"plugin_id": p.PluginId,
		"config":    p.Config,
	}
	if p.CreatedAt != nil {
		m["created_at"] = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		m["updated_at"] = p.UpdatedAt.AsTime()
	}
	return m
}

func FirestoreToPluginDefault(m map[string]interface{}) *pb.PluginDefault {
	p := &pb.PluginDefault{
		PluginId: getString(m, "plugin_id"),
	}
	if cfg, ok := m["config"].(map[string]interface{}); ok {
		p.Config = make(map[string]string, len(cfg))
		for k, v := range cfg {
			if s, ok := v.(string); ok {
				p.Config[k] = s
			}
		}
	}
	p.CreatedAt = getTime(m, "created_at")
	p.UpdatedAt = getTime(m, "updated_at")
	return p
}

// --- UploadedActivityRecord Converters (Loop Prevention) ---

func UploadedActivityToFirestore(r *pb.UploadedActivityRecord) map[string]interface{} {
	m := map[string]interface{}{
		"id":             r.Id,
		"user_id":        r.UserId,
		"source":         int32(r.Source),
		"external_id":    r.ExternalId,
		"destination":    int32(r.Destination),
		"destination_id": r.DestinationId,
	}

	if r.StartTime != nil {
		m["start_time"] = r.StartTime.AsTime()
	}
	if r.UploadedAt != nil {
		m["uploaded_at"] = r.UploadedAt.AsTime()
	}

	return m
}

func FirestoreToUploadedActivity(m map[string]interface{}) *pb.UploadedActivityRecord {
	r := &pb.UploadedActivityRecord{
		Id:            getString(m, "id"),
		UserId:        getString(m, "user_id"),
		ExternalId:    getString(m, "external_id"),
		DestinationId: getString(m, "destination_id"),
		StartTime:     getTime(m, "start_time"),
		UploadedAt:    getTime(m, "uploaded_at"),
	}

	// Source
	if v, ok := m["source"]; ok {
		switch val := v.(type) {
		case int64:
			r.Source = pb.ActivitySource(val)
		case int:
			r.Source = pb.ActivitySource(int32(val))
		case float64:
			r.Source = pb.ActivitySource(int32(val))
		}
	}

	// Destination
	if v, ok := m["destination"]; ok {
		switch val := v.(type) {
		case int64:
			r.Destination = pb.Destination(val)
		case int:
			r.Destination = pb.Destination(int32(val))
		case float64:
			r.Destination = pb.Destination(int32(val))
		}
	}

	return r
}

// --- PipelineRun Converters ---

func PipelineRunToFirestore(p *pb.PipelineRun) map[string]interface{} {
	m := map[string]interface{}{
		"id":                 p.Id,
		"pipeline_id":        p.PipelineId,
		"activity_id":        p.ActivityId,
		"source":             p.Source,
		"source_activity_id": p.SourceActivityId,
		"title":              p.Title,
		"description":        p.Description,
		"type":               int32(p.Type),
		"status":             int32(p.Status),
	}

	if p.StartTime != nil {
		m["start_time"] = p.StartTime.AsTime()
	}
	if p.CreatedAt != nil {
		m["created_at"] = p.CreatedAt.AsTime()
	}
	if p.UpdatedAt != nil {
		m["updated_at"] = p.UpdatedAt.AsTime()
	}
	if p.StatusMessage != nil {
		m["status_message"] = *p.StatusMessage
	}
	if p.PendingInputId != nil {
		m["pending_input_id"] = *p.PendingInputId
	}
	if p.OriginalPayloadUri != "" {
		m["original_payload_uri"] = p.OriginalPayloadUri
	}

	// Serialize boosters
	if len(p.Boosters) > 0 {
		boosters := make([]map[string]interface{}, len(p.Boosters))
		for i, b := range p.Boosters {
			booster := map[string]interface{}{
				"provider_name": b.ProviderName,
				"status":        b.Status,
				"duration_ms":   b.DurationMs,
				"metadata":      b.Metadata,
			}
			if b.Error != nil {
				booster["error"] = *b.Error
			}
			boosters[i] = booster
		}
		m["boosters"] = boosters
	}

	// Serialize destinations
	if len(p.Destinations) > 0 {
		dests := make([]map[string]interface{}, len(p.Destinations))
		for i, d := range p.Destinations {
			dest := map[string]interface{}{
				"destination": int32(d.Destination),
				"status":      int32(d.Status),
			}
			if d.ExternalId != nil {
				dest["external_id"] = *d.ExternalId
			}
			if d.Error != nil {
				dest["error"] = *d.Error
			}
			if d.CompletedAt != nil {
				dest["completed_at"] = d.CompletedAt.AsTime()
			}
			dests[i] = dest
		}
		m["destinations"] = dests
	}

	// Note: enriched_event is now stored in GCS via enriched_event_uri
	if p.EnrichedEventUri != "" {
		m["enriched_event_uri"] = p.EnrichedEventUri
	}
	// Note: original_payload is now stored in GCS via original_payload_uri

	return m
}

func FirestoreToPipelineRun(m map[string]interface{}) *pb.PipelineRun {
	p := &pb.PipelineRun{
		Id:                 getString(m, "id"),
		PipelineId:         getString(m, "pipeline_id"),
		ActivityId:         getString(m, "activity_id"),
		Source:             getString(m, "source"),
		SourceActivityId:   getString(m, "source_activity_id"),
		Title:              getString(m, "title"),
		Description:        getString(m, "description"),
		StartTime:          getTime(m, "start_time"),
		CreatedAt:          getTime(m, "created_at"),
		UpdatedAt:          getTime(m, "updated_at"),
		StatusMessage:      stringPtrOrNil(getString(m, "status_message")),
		PendingInputId:     stringPtrOrNil(getString(m, "pending_input_id")),
		OriginalPayloadUri: getString(m, "original_payload_uri"),
	}

	// Type
	if v, ok := m["type"]; ok {
		switch val := v.(type) {
		case int64:
			p.Type = pb.ActivityType(val)
		case int:
			p.Type = pb.ActivityType(int32(val))
		case float64:
			p.Type = pb.ActivityType(int32(val))
		}
	}

	// Status
	if v, ok := m["status"]; ok {
		switch val := v.(type) {
		case int64:
			p.Status = pb.PipelineRunStatus(val)
		case int:
			p.Status = pb.PipelineRunStatus(int32(val))
		case float64:
			p.Status = pb.PipelineRunStatus(int32(val))
		}
	}

	// Boosters
	if bList, ok := m["boosters"].([]interface{}); ok {
		p.Boosters = make([]*pb.BoosterExecution, len(bList))
		for i, bRaw := range bList {
			if bMap, ok := bRaw.(map[string]interface{}); ok {
				booster := &pb.BoosterExecution{
					ProviderName: getString(bMap, "provider_name"),
					Status:       getString(bMap, "status"),
					Error:        stringPtrOrNil(getString(bMap, "error")),
				}
				if v, ok := bMap["duration_ms"]; ok {
					switch n := v.(type) {
					case int64:
						booster.DurationMs = n
					case int:
						booster.DurationMs = int64(n)
					case float64:
						booster.DurationMs = int64(n)
					}
				}
				if meta, ok := bMap["metadata"].(map[string]interface{}); ok {
					booster.Metadata = make(map[string]string)
					for k, v := range meta {
						if s, ok := v.(string); ok {
							booster.Metadata[k] = s
						}
					}
				}
				p.Boosters[i] = booster
			}
		}
	}

	// Destinations
	if dList, ok := m["destinations"].([]interface{}); ok {
		p.Destinations = make([]*pb.DestinationOutcome, len(dList))
		for i, dRaw := range dList {
			if dMap, ok := dRaw.(map[string]interface{}); ok {
				dest := &pb.DestinationOutcome{
					ExternalId:  stringPtrOrNil(getString(dMap, "external_id")),
					Error:       stringPtrOrNil(getString(dMap, "error")),
					CompletedAt: getTime(dMap, "completed_at"),
				}
				if v, ok := dMap["destination"]; ok {
					switch val := v.(type) {
					case int64:
						dest.Destination = pb.Destination(val)
					case int:
						dest.Destination = pb.Destination(int32(val))
					case float64:
						dest.Destination = pb.Destination(int32(val))
					}
				}
				if v, ok := dMap["status"]; ok {
					switch val := v.(type) {
					case int64:
						dest.Status = pb.DestinationStatus(val)
					case int:
						dest.Status = pb.DestinationStatus(int32(val))
					case float64:
						dest.Status = pb.DestinationStatus(int32(val))
					}
				}
				p.Destinations[i] = dest
			}
		}
	}

	// Note: enriched_event is now stored in GCS via enriched_event_uri
	p.EnrichedEventUri = getString(m, "enriched_event_uri")

	// Note: original_payload is now stored in GCS via original_payload_uri

	return p
}
