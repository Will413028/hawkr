package postgres

func nullableText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
