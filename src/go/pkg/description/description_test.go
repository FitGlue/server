package description

import (
	"testing"
)

func TestHasSection(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		headerPrefix string
		expected     bool
	}{
		{
			name:         "Section found at start",
			description:  "🏃 Parkrun Results:\nWaiting for results...",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     true,
		},
		{
			name:         "Section found in middle",
			description:  "Original\n\n🏃 Parkrun Results:\nWaiting...\n\n❤️ Heart Rate:",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     true,
		},
		{
			name:         "Section not found",
			description:  "Some description without the section",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     false,
		},
		{
			name:         "Empty description",
			description:  "",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasSection(tt.description, tt.headerPrefix)
			if result != tt.expected {
				t.Errorf("HasSection() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestReplaceSection(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		headerPrefix string
		newContent   string
		expected     string
	}{
		{
			name:         "Replace section at start",
			description:  "🏃 Parkrun Results:\nWaiting for results...",
			headerPrefix: "🏃 Parkrun Results:",
			newContent:   "🏃 Parkrun Results:\n42nd place, 23:45",
			expected:     "🏃 Parkrun Results:\n42nd place, 23:45",
		},
		{
			name:         "Replace section with content before",
			description:  "Original description\n\n🏃 Parkrun Results:\nWaiting for results...",
			headerPrefix: "🏃 Parkrun Results:",
			newContent:   "🏃 Parkrun Results:\n42nd place, 23:45",
			expected:     "Original description\n\n🏃 Parkrun Results:\n42nd place, 23:45",
		},
		{
			name:         "Replace section with content after",
			description:  "🏃 Parkrun Results:\nWaiting...\n\n❤️ Heart Rate:\n150 bpm avg",
			headerPrefix: "🏃 Parkrun Results:",
			newContent:   "🏃 Parkrun Results:\n42nd place",
			expected:     "🏃 Parkrun Results:\n42nd place\n\n❤️ Heart Rate:\n150 bpm avg",
		},
		{
			name:         "Section not found - append",
			description:  "Some description",
			headerPrefix: "🏃 Parkrun Results:",
			newContent:   "🏃 Parkrun Results:\n42nd place",
			expected:     "Some description\n\n🏃 Parkrun Results:\n42nd place",
		},
		{
			name:         "Empty description - set",
			description:  "",
			headerPrefix: "🏃 Parkrun Results:",
			newContent:   "🏃 Parkrun Results:\n42nd place",
			expected:     "🏃 Parkrun Results:\n42nd place",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReplaceSection(tt.description, tt.headerPrefix, tt.newContent)
			if result != tt.expected {
				t.Errorf("ReplaceSection() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRemoveSection(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		headerPrefix string
		expected     string
	}{
		{
			name:         "Remove only section",
			description:  "🏃 Parkrun Results:\nWaiting for results...",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     "",
		},
		{
			name:         "Remove section with content before",
			description:  "Original description\n\n🏃 Parkrun Results:\nWaiting...",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     "Original description",
		},
		{
			name:         "Remove section with content after",
			description:  "🏃 Parkrun Results:\nWaiting...\n\n❤️ Heart Rate:\n150 bpm",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     "❤️ Heart Rate:\n150 bpm",
		},
		{
			name:         "Section not found - no change",
			description:  "Some description",
			headerPrefix: "🏃 Parkrun Results:",
			expected:     "Some description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveSection(tt.description, tt.headerPrefix)
			if result != tt.expected {
				t.Errorf("RemoveSection() = %q, want %q", result, tt.expected)
			}
		})
	}
}
