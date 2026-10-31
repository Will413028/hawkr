/**
 * The only place that reads the visitor's address from an inbound request.
 *
 * `x-real-ip` is written by the Vercel edge on every request and replaces any
 * value the client sent, so it cannot be forged. Other client-address headers
 * are not read: Vercel passes any header it does not own through untouched, so
 * reading one would let a caller pick its own rate-limit key.
 *
 * Returns `null` when the header is absent or blank (local dev, tests).
 */
export function getVisitorIp(request: { headers: Headers }): string | null {
	return request.headers.get("x-real-ip")?.trim() || null;
}
