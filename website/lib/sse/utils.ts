/**
 * Small typed coercion helpers shared by the SSE stream hooks
 * (`useCommentStream`, `useNotificationStream`).
 *
 * Why these instead of `as string` casts: SSE payloads cross the JSON wire
 * with no compile-time guarantees, so we normalise once at the boundary
 * and then let the rest of the hook trust the typed shape. Keeps the
 * dispatch table readable and the type signature honest (no
 * "string vs number drift" hiding in the consumer reducer).
 */

/** Coerce any JSON value to a string. `null` / `undefined` → fallback. */
export function asStr(v: unknown, fallback = ""): string {
	if (v == null) return fallback;
	return typeof v === "string" ? v : String(v);
}

/** Coerce any JSON value to a finite number. NaN / non-numeric → fallback. */
export function asNum(v: unknown, fallback = 0): number {
	const n = Number(v);
	return Number.isFinite(n) ? n : fallback;
}

/** Coerce any JSON value to a boolean. Accepts `true` / `"true"` only — strict
 *  to avoid silent truthy traps (any non-empty string was being read as true
 *  by previous `Boolean(v)` casts in the wire layer). */
export function asBool(v: unknown): boolean {
	return v === true || v === "true";
}

/** Common envelope of a batched SSE event:
 *  `{ events: Array<{ event, data, id? }> }`. Used to unpack both
 *  `comment.batch` and `notification.batch` for the same dispatch loop. */
export interface BatchEnvelope {
	events?: Array<{ event: string; data: unknown; id?: string }>;
}

/** Extract the inner event array from a batched SSE payload defensively. */
export function unpackBatch(data: unknown): Array<{
	event: string;
	data: unknown;
	id: string | null;
}> {
	const events = (data as BatchEnvelope)?.events ?? [];
	return events.map((e) => ({
		event: e.event,
		data: e.data,
		id: e.id ?? null,
	}));
}
