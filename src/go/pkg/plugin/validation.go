package plugin

import (
	"fmt"
	"regexp"
	"strconv"

	pb "github.com/fitglue/server/src/go/pkg/types/pb"
)

// ValidateConfigAgainstSchema validates config against a schema

// ValidateConfigAgainstSchema validates config against a schema
func ValidateConfigAgainstSchema(config map[string]string, schema []*pb.ConfigFieldSchema) error {
	for _, field := range schema {
		value, exists := config[field.Key]

		// Check required
		if field.Required && (!exists || value == "") {
			return fmt.Errorf("required field %q is missing", field.Key)
		}

		if !exists || value == "" {
			continue
		}

		// Validate based on field type
		switch field.FieldType {
		case pb.ConfigFieldType_CONFIG_FIELD_TYPE_NUMBER:
			num, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return fmt.Errorf("field %q must be a number", field.Key)
			}
			// Check min/max if validation exists
			if field.Validation != nil {
				if field.Validation.MinValue != nil && num < *field.Validation.MinValue {
					return fmt.Errorf("field %q must be >= %v", field.Key, *field.Validation.MinValue)
				}
				if field.Validation.MaxValue != nil && num > *field.Validation.MaxValue {
					return fmt.Errorf("field %q must be <= %v", field.Key, *field.Validation.MaxValue)
				}
			}

		case pb.ConfigFieldType_CONFIG_FIELD_TYPE_BOOLEAN:
			if value != "true" && value != "false" {
				return fmt.Errorf("field %q must be 'true' or 'false'", field.Key)
			}

		case pb.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT:
			valid := false
			for _, opt := range field.Options {
				if opt.Value == value {
					valid = true
					break
				}
			}
			if !valid {
				return fmt.Errorf("field %q has invalid value %q", field.Key, value)
			}

		case pb.ConfigFieldType_CONFIG_FIELD_TYPE_MULTI_SELECT:
			// Multi-select values are comma-separated
			if value != "" {
				values := splitMultiSelect(value)
				for _, v := range values {
					valid := false
					for _, opt := range field.Options {
						if opt.Value == v {
							valid = true
							break
						}
					}
					if !valid {
						return fmt.Errorf("field %q has invalid value %q", field.Key, v)
					}
				}
			}

		case pb.ConfigFieldType_CONFIG_FIELD_TYPE_STRING:
			if field.Validation != nil {
				if field.Validation.MinLength != nil && len(value) < int(*field.Validation.MinLength) {
					return fmt.Errorf("field %q must be at least %d characters", field.Key, *field.Validation.MinLength)
				}
				if field.Validation.MaxLength != nil && len(value) > int(*field.Validation.MaxLength) {
					return fmt.Errorf("field %q must be at most %d characters", field.Key, *field.Validation.MaxLength)
				}
				if field.Validation.Pattern != nil && *field.Validation.Pattern != "" {
					matched, err := regexp.MatchString(*field.Validation.Pattern, value)
					if err != nil || !matched {
						return fmt.Errorf("field %q does not match required pattern", field.Key)
					}
				}
			}
		}
	}

	return nil
}

// splitMultiSelect splits a comma-separated multi-select value
func splitMultiSelect(value string) []string {
	if value == "" {
		return nil
	}
	var result []string
	for _, v := range regexp.MustCompile(`,\s*`).Split(value, -1) {
		if v != "" {
			result = append(result, v)
		}
	}
	return result
}
