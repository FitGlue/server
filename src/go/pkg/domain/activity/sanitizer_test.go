package activity

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSanitizeActivityPayloadJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean payload stringified",
			input:    `{"userId":"123","originalPayloadJson":"{\"a\":\"b\"}"}`,
			expected: `{"originalPayloadJson":"{\"a\":\"b\"}","userId":"123"}`,
		},
		{
			name:     "legacy unescaped object camelCase",
			input:    `{"userId":"123","originalPayloadJson":{"a":"b"}}`,
			expected: `{"originalPayloadJson":"{\"a\":\"b\"}","userId":"123"}`, // Note the escaped output
		},
		{
			name:     "legacy unescaped object snake_case",
			input:    `{"user_id":"123","original_payload_json":{"a":1}}`,
			expected: `{"original_payload_json":"{\"a\":1}","user_id":"123"}`,
		},
		{
			name:     "null value should be ignored",
			input:    `{"userId":"123","originalPayloadJson":null}`,
			expected: `{"originalPayloadJson":null,"userId":"123"}`,
		},
		{
			name:     "invalid json",
			input:    `{"userId":"123",}`,
			expected: `{"userId":"123",}`, // untouched
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			output := SanitizeActivityPayloadJSON([]byte(tc.input))

			// If it's invalid JSON, we expect exact equality
			if tc.name == "invalid json" {
				if string(output) != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, string(output))
				}
				return
			}

			// For valid JSON, we parse both expected and actual to map for comparison
			// because JSON key ordering is not guaranteed during json.Marshal
			var expectedMap, outputMap map[string]interface{}
			if err := json.Unmarshal([]byte(tc.expected), &expectedMap); err != nil {
				t.Fatalf("Failed to parse expected JSON: %v", err)
			}
			if err := json.Unmarshal(output, &outputMap); err != nil {
				t.Fatalf("Failed to parse output JSON: %v. Output was: %s", err, string(output))
			}

			if !reflect.DeepEqual(expectedMap, outputMap) {
				t.Errorf("Mismatch.\nExpected: %v\nGot: %v", expectedMap, outputMap)
			}
		})
	}
}
