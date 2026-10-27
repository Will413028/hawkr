/**
 * The backend hands out 64-bit snowflake IDs (post / comment / order) that
 * exceed Number.MAX_SAFE_INTEGER, so a naive `res.json()` rounds the ID to
 * the nearest safe integer and every subsequent fetch keyed on that ID
 * misses (404, empty list).
 *
 * This pre-processor scans the raw response body for unquoted integer
 * literals 16+ digits long and wraps them as strings before parsing. Token
 * keys ending in `id`, timestamps stored in seconds (10 digits) and
 * milliseconds (13 digits) are untouched, so this is safe for the rest of
 * the API surface.
 */
const BIGINT_VALUE_RE = /([:[,]\s*)(-?\d{16,})(\s*[,}\]])/g;

export function preserveBigInts(text: string): string {
	return text.replace(BIGINT_VALUE_RE, '$1"$2"$3');
}

export function parseJsonSafe<T = unknown>(text: string): T {
	return JSON.parse(preserveBigInts(text)) as T;
}

/** Read a fetch Response, preserving 64-bit IDs in the parsed body. */
export async function readJsonSafe<T = unknown>(res: Response): Promise<T> {
	const text = await res.text();
	if (!text.trim()) return {} as T;
	return parseJsonSafe<T>(text);
}
