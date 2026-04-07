package activity

import (
	"encoding/json"
)

// SanitizeActivityPayloadJSON preprocesses raw JSON data representing an ActivityPayload
// or EnrichedActivityEvent before it is unmarshaled by protojson.
// Specifically, it ensures that the `original_payload_json` field is correctly
// encoded as a string, preventing parsing failures if legacy payloads contain
// unescaped objects or arrays.
func SanitizeActivityPayloadJSON(data []byte) []byte {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		// If it's not valid JSON at all, return it unmodified and let
		// protojson.Unmarshal return the exact structural error.
		return data
	}

	mutated := false

	// The payload might use either camelCase or snake_case depending on source serialization
	for _, key := range []string{"originalPayloadJson", "original_payload_json"} {
		if val, exists := m[key]; exists {
			// val will be a string if correctly formatted.
			// If it's another type (map or slice), we serialize it into a string.
			if _, isString := val.(string); !isString && val != nil {
				if b, err := json.Marshal(val); err == nil {
					m[key] = string(b)
					mutated = true
				}
			}
		}
	}

	if mutated {
		if fixedData, err := json.Marshal(m); err == nil {
			return fixedData
		}
	}

	return data
}
