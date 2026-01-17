package plugin

import (
	"testing"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

func TestValidateConfig_RequiredField(t *testing.T) {
	schema := []*pb.ConfigFieldSchema{
		{Key: "name", Label: "Name", FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING, Required: true},
	}

	tests := []struct {
		name    string
		config  map[string]string
		wantErr bool
	}{
		{"present", map[string]string{"name": "test"}, false},
		{"missing", map[string]string{}, true},
		{"empty", map[string]string{"name": ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(tt.config, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_NumberField(t *testing.T) {
	minVal := 1.0
	maxVal := 10.0
	schema := []*pb.ConfigFieldSchema{
		{
			Key:       "count",
			FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_NUMBER,
			Validation: &pb.ConfigFieldValidation{
				MinValue: &minVal,
				MaxValue: &maxVal,
			},
		},
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "5", false},
		{"min edge", "1", false},
		{"max edge", "10", false},
		{"below min", "0", true},
		{"above max", "11", true},
		{"not a number", "abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(map[string]string{"count": tt.value}, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_SelectField(t *testing.T) {
	schema := []*pb.ConfigFieldSchema{
		{
			Key:       "format",
			FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT,
			Options: []*pb.ConfigFieldOption{
				{Value: "compact", Label: "Compact"},
				{Value: "detailed", Label: "Detailed"},
			},
		},
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid compact", "compact", false},
		{"valid detailed", "detailed", false},
		{"invalid", "verbose", true},
		{"empty optional", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(map[string]string{"format": tt.value}, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_BooleanField(t *testing.T) {
	schema := []*pb.ConfigFieldSchema{
		{Key: "enabled", FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_BOOLEAN},
	}

	tests := []struct {
		value   string
		wantErr bool
	}{
		{"true", false},
		{"false", false},
		{"yes", true},
		{"1", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(map[string]string{"enabled": tt.value}, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_StringValidation(t *testing.T) {
	minLen := int32(3)
	maxLen := int32(10)
	pattern := "^[a-z]+$"
	schema := []*pb.ConfigFieldSchema{
		{
			Key:       "code",
			FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING,
			Validation: &pb.ConfigFieldValidation{
				MinLength: &minLen,
				MaxLength: &maxLen,
				Pattern:   &pattern,
			},
		},
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"valid", "hello", false},
		{"too short", "ab", true},
		{"too long", "abcdefghijk", true},
		{"bad pattern uppercase", "HELLO", true},
		{"bad pattern numbers", "hello123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(map[string]string{"code": tt.value}, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig_MultiSelectField(t *testing.T) {
	schema := []*pb.ConfigFieldSchema{
		{
			Key:       "types",
			FieldType: pb.ConfigFieldType_CONFIG_FIELD_TYPE_MULTI_SELECT,
			Options: []*pb.ConfigFieldOption{
				{Value: "running", Label: "Running"},
				{Value: "cycling", Label: "Cycling"},
				{Value: "swimming", Label: "Swimming"},
			},
		},
	}

	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"single valid", "running", false},
		{"multiple valid", "running,cycling", false},
		{"all valid", "running,cycling,swimming", false},
		{"one invalid", "running,walking", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfigAgainstSchema(map[string]string{"types": tt.value}, schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
