const RESERVED_EXACT_HEADERS = new Set([
	"x-active-store",
	"x-target-seller-id",
	"x-admin-actor-id",
	"x-operation-session",
	"x-operation-session-id",
	"x-delegated-store-id",
	"x-impersonated-user-id",
	// Browser-supplied rate-limit identity assertions are never accepted.
	"x-hk-origin-auth",
]);

const RESERVED_HEADER_PREFIXES = [
	"x-hk-operation-",
	"x-delegated-operation-",
] as const;

const MAX_DELEGATED_PATH_DECODE_PASSES = 16;

const GENERIC_BFF_FORWARDED_HEADERS = new Set([
	"accept",
	"accept-language",
	"baggage",
	// 對外 API（Connect，/{service}/{method}）的協定 header：版本與 client 的 deadline。
	"connect-protocol-version",
	"connect-timeout-ms",
	"content-type",
	"idempotency-key",
	"if-match",
	"if-none-match",
	"range",
	"sentry-trace",
	"traceparent",
	"tracestate",
]);

/** Generic browser requests may forward representation/trace metadata only. */
export function isAllowedGenericBffRequestHeader(name: string): boolean {
	return GENERIC_BFF_FORWARDED_HEADERS.has(name.toLowerCase());
}

export function encodeGenericBffPath(path: string[]): string {
	return path.map(encodeURIComponent).join("/");
}

function hasUnsafeProxyPathCharacter(segment: string): boolean {
	for (const character of segment) {
		const code = character.charCodeAt(0);
		if (code <= 32 || code === 127 || "\\/?#".includes(character)) return true;
	}
	return false;
}

export function isReservedDelegatedOperationHeader(name: string): boolean {
	const lower = name.toLowerCase();
	return (
		RESERVED_EXACT_HEADERS.has(lower) ||
		RESERVED_HEADER_PREFIXES.some((prefix) => lower.startsWith(prefix))
	);
}

/** The catch-all BFF must never proxy the dedicated operation namespace. */
export function isDelegatedOperationNamespace(path: string[]): boolean {
	return path.some((rawSegment) => {
		let segment = rawSegment;
		let stable = false;
		for (let pass = 0; pass < MAX_DELEGATED_PATH_DECODE_PASSES; pass += 1) {
			try {
				const decoded = decodeURIComponent(segment);
				if (decoded === segment) {
					stable = true;
					break;
				}
				segment = decoded;
			} catch {
				return true;
			}
		}
		if (!stable) return true;
		if (
			segment === "." ||
			segment === ".." ||
			hasUnsafeProxyPathCharacter(segment)
		) {
			return true;
		}
		const lower = segment.toLowerCase();
		return lower === "delegated-operation" || lower === "delegated-operations";
	});
}
