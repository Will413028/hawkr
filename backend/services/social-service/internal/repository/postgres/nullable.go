package postgres

// nullableText expresses the repository policy used by optional text columns:
// an empty input is stored as SQL NULL.
func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// textPointer preserves an empty string as a valid, non-NULL value. Use it
// only for columns whose existing write contract deliberately accepted empty
// text as a value.
func textPointer(value string) *string {
	return &value
}

// nullableInt64 expresses the repository policy used by optional ID columns:
// zero means that no ID was resolved and is stored as SQL NULL.
func nullableInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
