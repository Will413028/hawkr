export function errorMessage(error: unknown, fallback: string): string | null {
	// Only surface an error when there genuinely is one. A disabled query (e.g.
	// logged-out) has `error === null`; returning the fallback there would show a
	// false "載入失敗" when the real state is just "not signed in / no data".
	if (!error) return null;
	return error instanceof Error ? error.message : fallback;
}
