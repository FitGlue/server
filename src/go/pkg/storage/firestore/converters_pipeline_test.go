package firestore

import (
	"testing"

	pbpipeline "github.com/fitglue/server/src/go/pkg/types/pb/models/pipeline"
	pbplugin "github.com/fitglue/server/src/go/pkg/types/pb/models/plugin"
)

func TestFirestoreToPipeline_ProviderTypeNumeric(t *testing.T) {
	m := map[string]interface{}{
		"id":     "p1",
		"source": "SOURCE_HEVY",
		"enrichers": []interface{}{
			map[string]interface{}{
				"provider_type": int64(2), // ENRICHER_PROVIDER_WORKOUT_SUMMARY
				"typed_config":  map[string]interface{}{},
			},
		},
		"destinations": []interface{}{},
	}

	pipeline := FirestoreToPipeline(m)

	if len(pipeline.Enrichers) != 1 {
		t.Fatalf("Expected 1 enricher, got %d", len(pipeline.Enrichers))
	}
	if pipeline.Enrichers[0].ProviderType != pbplugin.EnricherProviderType_ENRICHER_PROVIDER_WORKOUT_SUMMARY {
		t.Errorf("Expected ENRICHER_PROVIDER_WORKOUT_SUMMARY, got %v", pipeline.Enrichers[0].ProviderType)
	}
}

func TestFirestoreToPipeline_ProviderTypeFloat(t *testing.T) {
	m := map[string]interface{}{
		"id":     "p1",
		"source": "SOURCE_HEVY",
		"enrichers": []interface{}{
			map[string]interface{}{
				"provider_type": float64(15), // ENRICHER_PROVIDER_AI_COMPANION
				"typed_config":  map[string]interface{}{},
			},
		},
		"destinations": []interface{}{},
	}

	pipeline := FirestoreToPipeline(m)

	if len(pipeline.Enrichers) != 1 {
		t.Fatalf("Expected 1 enricher, got %d", len(pipeline.Enrichers))
	}
	if pipeline.Enrichers[0].ProviderType != pbplugin.EnricherProviderType_ENRICHER_PROVIDER_AI_COMPANION {
		t.Errorf("Expected ENRICHER_PROVIDER_AI_COMPANION, got %v", pipeline.Enrichers[0].ProviderType)
	}
}

func TestFirestoreToPipeline_ProviderTypeString(t *testing.T) {
	m := map[string]interface{}{
		"id":     "p1",
		"source": "SOURCE_HEVY",
		"enrichers": []interface{}{
			map[string]interface{}{
				"provider_type": "ENRICHER_PROVIDER_WORKOUT_SUMMARY",
				"typed_config":  map[string]interface{}{},
			},
		},
		"destinations": []interface{}{},
	}

	pipeline := FirestoreToPipeline(m)

	if len(pipeline.Enrichers) != 1 {
		t.Fatalf("Expected 1 enricher, got %d", len(pipeline.Enrichers))
	}
	if pipeline.Enrichers[0].ProviderType != pbplugin.EnricherProviderType_ENRICHER_PROVIDER_WORKOUT_SUMMARY {
		t.Errorf("Expected ENRICHER_PROVIDER_WORKOUT_SUMMARY, got %v", pipeline.Enrichers[0].ProviderType)
	}
}

func TestFirestoreToPipeline_ProviderTypeUnknownString(t *testing.T) {
	m := map[string]interface{}{
		"id":     "p1",
		"source": "SOURCE_HEVY",
		"enrichers": []interface{}{
			map[string]interface{}{
				"provider_type": "NOT_A_REAL_PROVIDER",
				"typed_config":  map[string]interface{}{},
			},
		},
		"destinations": []interface{}{},
	}

	pipeline := FirestoreToPipeline(m)

	if len(pipeline.Enrichers) != 1 {
		t.Fatalf("Expected 1 enricher, got %d", len(pipeline.Enrichers))
	}
	if pipeline.Enrichers[0].ProviderType != pbplugin.EnricherProviderType_ENRICHER_PROVIDER_UNSPECIFIED {
		t.Errorf("Expected ENRICHER_PROVIDER_UNSPECIFIED for unknown string, got %v", pipeline.Enrichers[0].ProviderType)
	}
}

func TestFirestoreToPipeline_DestinationStringAllTypes(t *testing.T) {
	tests := []struct {
		input    string
		expected pbplugin.DestinationType
	}{
		{"DESTINATION_STRAVA", pbplugin.DestinationType_DESTINATION_STRAVA},
		{"DESTINATION_SHOWCASE", pbplugin.DestinationType_DESTINATION_SHOWCASE},
		{"DESTINATION_HEVY", pbplugin.DestinationType_DESTINATION_HEVY},
		{"DESTINATION_TRAININGPEAKS", pbplugin.DestinationType_DESTINATION_TRAININGPEAKS},
		{"DESTINATION_INTERVALS", pbplugin.DestinationType_DESTINATION_INTERVALS},
		{"DESTINATION_GOOGLESHEETS", pbplugin.DestinationType_DESTINATION_GOOGLESHEETS},
		{"DESTINATION_GITHUB", pbplugin.DestinationType_DESTINATION_GITHUB},
		{"DESTINATION_MOCK", pbplugin.DestinationType_DESTINATION_MOCK},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := map[string]interface{}{
				"id":           "p1",
				"source":       "SOURCE_HEVY",
				"enrichers":    []interface{}{},
				"destinations": []interface{}{tt.input},
			}

			pipeline := FirestoreToPipeline(m)

			if len(pipeline.Destinations) != 1 {
				t.Fatalf("Expected 1 destination, got %d", len(pipeline.Destinations))
			}
			if pipeline.Destinations[0] != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, pipeline.Destinations[0])
			}
		})
	}
}

func TestFirestoreToPipeline_DestinationShortForm(t *testing.T) {
	tests := []struct {
		input    string
		expected pbplugin.DestinationType
	}{
		{"strava", pbplugin.DestinationType_DESTINATION_STRAVA},
		{"showcase", pbplugin.DestinationType_DESTINATION_SHOWCASE},
		{"hevy", pbplugin.DestinationType_DESTINATION_HEVY},
		{"mock", pbplugin.DestinationType_DESTINATION_MOCK},
		{"STRAVA", pbplugin.DestinationType_DESTINATION_STRAVA},
		{"Showcase", pbplugin.DestinationType_DESTINATION_SHOWCASE},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := map[string]interface{}{
				"id":           "p1",
				"source":       "SOURCE_HEVY",
				"enrichers":    []interface{}{},
				"destinations": []interface{}{tt.input},
			}

			pipeline := FirestoreToPipeline(m)

			if len(pipeline.Destinations) != 1 {
				t.Fatalf("Expected 1 destination for input %q, got %d", tt.input, len(pipeline.Destinations))
			}
			if pipeline.Destinations[0] != tt.expected {
				t.Errorf("For input %q: expected %v, got %v", tt.input, tt.expected, pipeline.Destinations[0])
			}
		})
	}
}

func TestFirestoreToPipeline_RoundTrip(t *testing.T) {
	// Test that PipelineToFirestore -> FirestoreToPipeline preserves data.
	// Note: PipelineToFirestore stores provider_type as int32, but Firestore
	// always returns integers as int64. We simulate this by converting the
	// enricher maps to use int64 values, as Firestore would.
	original := &pbpipeline.PipelineConfig{
		Id:     "test-pipe",
		Name:   "Test Pipeline",
		Source: "SOURCE_HEVY",
		Enrichers: []*pbpipeline.EnricherConfig{
			{
				ProviderType: pbplugin.EnricherProviderType_ENRICHER_PROVIDER_WORKOUT_SUMMARY,
				TypedConfig:  map[string]string{"key": "val"},
			},
			{
				ProviderType: pbplugin.EnricherProviderType_ENRICHER_PROVIDER_AI_COMPANION,
				TypedConfig:  map[string]string{},
			},
		},
		Destinations: []pbplugin.DestinationType{
			pbplugin.DestinationType_DESTINATION_STRAVA,
			pbplugin.DestinationType_DESTINATION_SHOWCASE,
		},
	}

	firestoreMap := PipelineToFirestore(original)

	// Simulate Firestore's behavior: convert int32 -> int64 in enricher maps
	if eList, ok := firestoreMap["enrichers"].([]map[string]interface{}); ok {
		for _, eMap := range eList {
			if pt, ok := eMap["provider_type"].(int32); ok {
				eMap["provider_type"] = int64(pt)
			}
		}
		// Convert []map[string]interface{} to []interface{} as Firestore returns
		enricherSlice := make([]interface{}, len(eList))
		for i, e := range eList {
			enricherSlice[i] = e
		}
		firestoreMap["enrichers"] = enricherSlice
	}
	// Simulate Firestore's behavior for destinations: convert int32 -> int64
	if dList, ok := firestoreMap["destinations"].([]pbplugin.DestinationType); ok {
		destSlice := make([]interface{}, len(dList))
		for i, d := range dList {
			destSlice[i] = int64(d)
		}
		firestoreMap["destinations"] = destSlice
	}

	roundTripped := FirestoreToPipeline(firestoreMap)

	if roundTripped.Id != original.Id {
		t.Errorf("ID mismatch: %s vs %s", roundTripped.Id, original.Id)
	}
	if len(roundTripped.Enrichers) != len(original.Enrichers) {
		t.Fatalf("Enricher count mismatch: %d vs %d", len(roundTripped.Enrichers), len(original.Enrichers))
	}
	for i, e := range roundTripped.Enrichers {
		if e.ProviderType != original.Enrichers[i].ProviderType {
			t.Errorf("Enricher[%d] ProviderType mismatch: %v vs %v", i, e.ProviderType, original.Enrichers[i].ProviderType)
		}
	}
	if len(roundTripped.Destinations) != len(original.Destinations) {
		t.Fatalf("Destination count mismatch: %d vs %d", len(roundTripped.Destinations), len(original.Destinations))
	}
	for i, d := range roundTripped.Destinations {
		if d != original.Destinations[i] {
			t.Errorf("Destination[%d] mismatch: %v vs %v", i, d, original.Destinations[i])
		}
	}
}
