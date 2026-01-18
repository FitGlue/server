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
		m["integrations"] = integrations
	}

	if len(u.FcmTokens) > 0 {
		m["fcm_tokens"] = u.FcmTokens
	}

	if len(u.Pipelines) > 0 {
		pipelines := make([]map[string]interface{}, len(u.Pipelines))
		for i, p := range u.Pipelines {
			enrichers := make([]map[string]interface{}, len(p.Enrichers))
			for j, e := range p.Enrichers {
				enrichers[j] = map[string]interface{}{
					"provider_type": int32(e.ProviderType),
					"typed_config":  e.TypedConfig,
				}
			}
			pipelines[i] = map[string]interface{}{
				"id":           p.Id,
				"source":       p.Source,
				"destinations": p.Destinations,
				"enrichers":    enrichers,
			}
		}
		m["pipelines"] = pipelines
	}

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

	if pList, ok := m["pipelines"].([]interface{}); ok {
		u.Pipelines = make([]*pb.PipelineConfig, len(pList))
		for i, pRaw := range pList {
			if pMap, ok := pRaw.(map[string]interface{}); ok {
				// Enrichers
				var enrichers []*pb.EnricherConfig
				if eList, ok := pMap["enrichers"].([]interface{}); ok {
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
								// int conversion
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
				if dList, ok := pMap["destinations"].([]interface{}); ok {
					for _, d := range dList {
						switch val := d.(type) {
						case int64:
							dests = append(dests, pb.Destination(val))
						case int:
							dests = append(dests, pb.Destination(val))
						case float64:
							dests = append(dests, pb.Destination(int32(val)))
						case string:
							// Legacy string support - map known strings to enums
							switch val {
							case "strava", "DESTINATION_STRAVA":
								dests = append(dests, pb.Destination_DESTINATION_STRAVA)
							case "mock", "DESTINATION_MOCK":
								dests = append(dests, pb.Destination_DESTINATION_MOCK)
							}
						}
					}
				}

				u.Pipelines[i] = &pb.PipelineConfig{
					Id:           getString(pMap, "id"),
					Source:       getString(pMap, "source"),
					Enrichers:    enrichers,
					Destinations: dests,
				}
			}
		}
	}

	return u
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

// --- PendingInput Converters ---

func PendingInputToFirestore(p *pb.PendingInput) map[string]interface{} {
	m := map[string]interface{}{
		"activity_id":     p.ActivityId,
		"user_id":         p.UserId,
		"status":          int32(p.Status),
		"required_fields": p.RequiredFields,
		"input_data":      p.InputData,
		"created_at":      p.CreatedAt.AsTime(),
		"updated_at":      p.UpdatedAt.AsTime(),
		"completed_at":    p.CompletedAt.AsTime(),
	}

	// Serialize original_payload to JSON string (not binary proto) so TypeScript can republish it
	if p.OriginalPayload != nil {
		jsonBytes, err := protojson.Marshal(p.OriginalPayload)
		if err == nil {
			m["original_payload"] = string(jsonBytes)
		}
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
		CreatedAt:   getTime(m, "created_at"),
		UpdatedAt:   getTime(m, "updated_at"),
		CompletedAt: getTime(m, "completed_at"),
	}

	if v, ok := m["status"]; ok {
		switch n := v.(type) {
		case int64:
			p.Status = pb.PendingInput_Status(n)
		case int:
			p.Status = pb.PendingInput_Status(int32(n))
		}
	}

	if v, ok := m["original_payload"]; ok {
		var jsonStr string
		switch val := v.(type) {
		case string:
			jsonStr = val
		case []byte:
			jsonStr = string(val)
		}
		if jsonStr != "" {
			var payload pb.ActivityPayload
			if err := protojson.Unmarshal([]byte(jsonStr), &payload); err == nil {
				p.OriginalPayload = &payload
			}
		}
	}
	return p
}

// --- SynchronizedActivity Converters ---

func SynchronizedActivityToFirestore(s *pb.SynchronizedActivity) map[string]interface{} {
	m := map[string]interface{}{
		"activity_id":           s.ActivityId,
		"title":                 s.Title,
		"description":           s.Description,
		"type":                  int32(s.Type),
		"source":                s.Source,
		"start_time":            s.StartTime.AsTime(),
		"synced_at":             s.SyncedAt.AsTime(),
		"pipeline_id":           s.PipelineId,
		"pipeline_execution_id": s.PipelineExecutionId,
		// Parkrun tracking fields
		"parkrun_results_state": int32(s.ParkrunResultsState),
		"parkrun_event_name":    s.ParkrunEventName,
		"parkrun_event_slug":    s.ParkrunEventSlug,
	}

	if s.ParkrunPollingDeadline != nil {
		m["parkrun_polling_deadline"] = s.ParkrunPollingDeadline.AsTime()
	}

	if s.Destinations != nil {
		m["destinations"] = s.Destinations
	}

	return m
}

func FirestoreToSynchronizedActivity(m map[string]interface{}) *pb.SynchronizedActivity {
	s := &pb.SynchronizedActivity{
		ActivityId:             getString(m, "activity_id"),
		Title:                  getString(m, "title"),
		Description:            getString(m, "description"),
		Source:                 getString(m, "source"),
		StartTime:              getTime(m, "start_time"),
		SyncedAt:               getTime(m, "synced_at"),
		PipelineId:             getString(m, "pipeline_id"),
		PipelineExecutionId:    getString(m, "pipeline_execution_id"),
		ParkrunEventName:       getString(m, "parkrun_event_name"),
		ParkrunEventSlug:       getString(m, "parkrun_event_slug"),
		ParkrunPollingDeadline: getTime(m, "parkrun_polling_deadline"),
	}

	if v, ok := m["type"]; ok {
		// Handle int or string legacy
		switch val := v.(type) {
		case int64:
			s.Type = pb.ActivityType(val)
		case int:
			s.Type = pb.ActivityType(int32(val))
		case string:
			if enumVal, ok := pb.ActivityType_value[val]; ok {
				s.Type = pb.ActivityType(enumVal)
			}
		}
	}

	if v, ok := m["parkrun_results_state"]; ok {
		switch val := v.(type) {
		case int64:
			s.ParkrunResultsState = pb.ParkrunResultsState(val)
		case int:
			s.ParkrunResultsState = pb.ParkrunResultsState(int32(val))
		case float64:
			s.ParkrunResultsState = pb.ParkrunResultsState(int32(val))
		}
	}

	if v, ok := m["destinations"].(map[string]interface{}); ok {
		dests := make(map[string]string)
		for k, val := range v {
			if str, ok := val.(string); ok {
				dests[k] = str
			}
		}
		s.Destinations = dests
	}

	return s
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

	// Serialize StandardizedActivity to JSON for easy TypeScript consumption
	if s.ActivityData != nil {
		jsonBytes, err := protojson.Marshal(s.ActivityData)
		if err == nil {
			m["activity_data"] = string(jsonBytes)
		}
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
