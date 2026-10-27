package postgres

// nullableText preserves the repository policy that an empty optional text
// value is stored as SQL NULL.
func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
