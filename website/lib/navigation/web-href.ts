export interface NormalizeWebHrefOptions {
	/** Permit a fully-qualified URL to leave the Hawkr origin. */
	allowExternal?: boolean;
	/** Permit plain HTTP for integrations that still explicitly require it. */
	allowInsecureHttp?: boolean;
}

const INTERNAL_BASE = "https://hawkr.invalid";

/**
 * Normalise an untrusted navigation value before rendering it as an href.
 *
 * Root-relative paths stay root-relative. Protocol-relative/backslash/control-
 * character variants are rejected because browsers can reinterpret them as an
 * off-origin URL. Absolute destinations are opt-in and HTTPS-only by default.
 */
export function normalizeWebHref(
	value: unknown,
	options: NormalizeWebHrefOptions = {},
): string | undefined {
	const candidate = typeof value === "string" ? value.trim() : "";
	if (!candidate) return undefined;
	for (let index = 0; index < candidate.length; index++) {
		const code = candidate.charCodeAt(index);
		if (code <= 0x1f || code === 0x7f) return undefined;
	}

	if (candidate.startsWith("/")) {
		if (/^\/[\\/]/.test(candidate) || candidate.includes("\\")) {
			return undefined;
		}
		try {
			const url = new URL(candidate, INTERNAL_BASE);
			if (url.origin !== INTERNAL_BASE) return undefined;
			return `${url.pathname}${url.search}${url.hash}`;
		} catch {
			return undefined;
		}
	}

	if (!options.allowExternal) return undefined;
	try {
		const url = new URL(candidate);
		if (url.protocol === "https:") return url.toString();
		if (options.allowInsecureHttp && url.protocol === "http:") {
			return url.toString();
		}
	} catch {
		// Invalid backend/user-provided href: render the surface as non-clickable.
	}
	return undefined;
}
