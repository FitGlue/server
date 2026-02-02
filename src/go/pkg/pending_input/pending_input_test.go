package pending_input

import (
	"testing"
)

func TestGenerateID(t *testing.T) {
	tests := []struct {
		name               string
		source             string
		externalID         string
		enricherProviderID string
		expected           string
	}{
		{
			name:               "Standard format",
			source:             "SOURCE_STRAVA",
			externalID:         "12345",
			enricherProviderID: "parkrun",
			expected:           "SOURCE_STRAVA:12345:parkrun",
		},
		{
			name:               "File upload with hybrid race tagger",
			source:             "SOURCE_FILE_UPLOAD",
			externalID:         "upload_abc123",
			enricherProviderID: "hybrid_race_tagger",
			expected:           "SOURCE_FILE_UPLOAD:upload_abc123:hybrid_race_tagger",
		},
		{
			name:               "User input enricher",
			source:             "SOURCE_HEVY",
			externalID:         "workout_456",
			enricherProviderID: "user_input",
			expected:           "SOURCE_HEVY:workout_456:user_input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateID(tt.source, tt.externalID, tt.enricherProviderID)
			if result != tt.expected {
				t.Errorf("GenerateID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseID(t *testing.T) {
	tests := []struct {
		name               string
		id                 string
		expectedSource     string
		expectedExternalID string
		expectedEnricherID string
		expectError        bool
	}{
		{
			name:               "Valid 3-part ID",
			id:                 "SOURCE_STRAVA:12345:parkrun",
			expectedSource:     "SOURCE_STRAVA",
			expectedExternalID: "12345",
			expectedEnricherID: "parkrun",
			expectError:        false,
		},
		{
			name:               "Valid ID with underscores",
			id:                 "SOURCE_FILE_UPLOAD:upload_abc123:hybrid_race_tagger",
			expectedSource:     "SOURCE_FILE_UPLOAD",
			expectedExternalID: "upload_abc123",
			expectedEnricherID: "hybrid_race_tagger",
			expectError:        false,
		},
		{
			name:        "Invalid 2-part ID",
			id:          "SOURCE_STRAVA:12345",
			expectError: true,
		},
		{
			name:        "Invalid 1-part ID",
			id:          "SOURCE_STRAVA",
			expectError: true,
		},
		{
			name:        "Empty string",
			id:          "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, externalID, enricherID, err := ParseID(tt.id)

			if tt.expectError {
				if err == nil {
					t.Errorf("ParseID() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseID() unexpected error: %v", err)
			}

			if source != tt.expectedSource {
				t.Errorf("ParseID() source = %q, want %q", source, tt.expectedSource)
			}
			if externalID != tt.expectedExternalID {
				t.Errorf("ParseID() externalID = %q, want %q", externalID, tt.expectedExternalID)
			}
			if enricherID != tt.expectedEnricherID {
				t.Errorf("ParseID() enricherID = %q, want %q", enricherID, tt.expectedEnricherID)
			}
		})
	}
}

func TestGetActivityKey(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		externalID string
		expected   string
	}{
		{
			name:       "Standard format",
			source:     "SOURCE_STRAVA",
			externalID: "12345",
			expected:   "SOURCE_STRAVA:12345",
		},
		{
			name:       "With underscores",
			source:     "SOURCE_FILE_UPLOAD",
			externalID: "upload_abc123",
			expected:   "SOURCE_FILE_UPLOAD:upload_abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetActivityKey(tt.source, tt.externalID)
			if result != tt.expected {
				t.Errorf("GetActivityKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}
