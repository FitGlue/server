package firestore

import (
	"time"

	pb "github.com/ripixel/fitglue-server/src/go/pkg/types/pb"
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
		m["integrations"] = integrations
	}

	if len(u.Pipelines) > 0 {
		pipelines := make([]map[string]interface{}, len(u.Pipelines))
		for i, p := range u.Pipelines {
			enrichers := make([]map[string]interface{}, len(p.Enrichers))
			for j, e := range p.Enrichers {
				enrichers[j] = map[string]interface{}{
					"provider_type": int32(e.ProviderType),
					"inputs":        e.Inputs,
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
							// Inputs
							inputs := make(map[string]string)
							if iMap, ok := eMap["inputs"].(map[string]interface{}); ok {
								for k, v := range iMap {
									if s, ok := v.(string); ok {
										inputs[k] = s
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
								Inputs:       inputs,
							}
						}
					}
				}

				// Destinations
				var dests []string
				if dList, ok := pMap["destinations"].([]interface{}); ok {
					for _, d := range dList {
						if s, ok := d.(string); ok {
							dests = append(dests, s)
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
		"execution_id":        e.ExecutionId,
		"service":             e.Service,
		"status":              int32(e.Status), // Store enum as int or string? Protocol is int usually, logger used String()
		"timestamp":           e.Timestamp.AsTime(),
		"user_id":             e.UserId,
		"test_run_id":         e.TestRunId,
		"trigger_type":        e.TriggerType,
		"start_time":          e.StartTime.AsTime(),
		"end_time":            e.EndTime.AsTime(),
		"error_message":       e.ErrorMessage,
		"inputs_json":         e.InputsJson,
		"outputs_json":        e.OutputsJson,
		"parent_execution_id": e.ParentExecutionId,
	}
	return m
}

func FirestoreToExecution(m map[string]interface{}) *pb.ExecutionRecord {
	e := &pb.ExecutionRecord{
		ExecutionId:       getString(m, "execution_id"),
		Service:           getString(m, "service"),
		Timestamp:         getTime(m, "timestamp"),
		UserId:            getString(m, "user_id"),
		TestRunId:         getString(m, "test_run_id"),
		TriggerType:       getString(m, "trigger_type"),
		StartTime:         getTime(m, "start_time"),
		EndTime:           getTime(m, "end_time"),
		ErrorMessage:      getString(m, "error_message"),
		InputsJson:        getString(m, "inputs_json"),
		OutputsJson:       getString(m, "outputs_json"),
		ParentExecutionId: getString(m, "parent_execution_id"),
	}

	if v, ok := m["status"]; ok {
		// Handle int or string legacy
		switch val := v.(type) {
		case int64:
			e.Status = pb.ExecutionStatus(val)
		case string:
			// If legacy logger stored strings like "STATUS_STARTED"
			if val == "STATUS_STARTED" {
				e.Status = pb.ExecutionStatus_STATUS_STARTED
			}
			if val == "STATUS_SUCCESS" {
				e.Status = pb.ExecutionStatus_STATUS_SUCCESS
			}
			if val == "STATUS_FAILED" {
				e.Status = pb.ExecutionStatus_STATUS_FAILED
			}
		}
	}

	return e
}
