package usecase

// stringOrEmpty keeps generated nullable text at the application boundary.
// The API/domain contract for these fields uses an empty string for SQL NULL.
func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
