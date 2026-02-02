package pending_input

import (
	"fmt"
	"strings"
)

// GenerateID creates a pending input document ID with the standardized format.
// Format: {source}:{external_id}:{enricher_provider_id}
func GenerateID(source, externalID, enricherProviderID string) string {
	return fmt.Sprintf("%s:%s:%s", source, externalID, enricherProviderID)
}

// ParseID extracts components from a pending input document ID.
// Returns source, externalID, enricherProviderID, and error if invalid.
func ParseID(id string) (source, externalID, enricherProviderID string, err error) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid pending input ID format: %s", id)
	}
	return parts[0], parts[1], parts[2], nil
}

// GetActivityKey returns the activity portion without the enricher suffix.
// Useful for grouping pending inputs by activity.
func GetActivityKey(source, externalID string) string {
	return fmt.Sprintf("%s:%s", source, externalID)
}
