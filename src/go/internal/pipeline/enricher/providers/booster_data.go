package providers

// ToFloat64 safely converts a Firestore map value to float64.
// Firestore returns int64 for integer values and float64 for floats.
// This handles both cases to prevent silent type assertion failures.
func ToFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int64:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 0
	}
}
