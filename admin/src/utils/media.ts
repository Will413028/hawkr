import type { ImagePreset } from "./image-presets.generated";

/**
 * Image URL assembly — single entry point (design book 49 §6.5 / §8.6).
 *
 * Admin counterpart of the web app's `lib/media/cdn.ts`. The only media host
 * the app knows is the configured origin `NEXT_PUBLIC_MEDIA_BASE_URL`; there is
 * no built-in host to fall back to.
 *
 *   Origin configured — the value is treated as an objectKey and rewritten to
 *     `{mediaBaseUrl}/i/{preset}/{objectKey}`, which the Cloudflare Worker
 *     serves as the preset-transformed image (`original` = no transform). A
 *     value that is already a display URL on the configured host is peeled back
 *     to its objectKey first, so it re-renders under the requested preset;
 *     foreign URLs (Google avatars, etc.) pass through untouched.
 *
 *   No origin configured — a bare object key yields no URL (`""`), so the image
 *     is absent rather than pointing at a host nobody configured. Absolute /
 *     data / blob values still pass through.
 */

/**
 * Preset whitelist (design book 49 §4.4). The frontend only ever emits a preset
 * NAME; the Cloudflare Worker owns the transform parameters. 名稱由 presets.ts 生成
 * （ADR 2026-09-25-api-image-uri-template D4）。
 */
export type { ImagePreset };

/** Media-gateway base origin, e.g. `https://media.example.org`.
 *  Empty when NEXT_PUBLIC_MEDIA_BASE_URL is unset. Read lazily
 *  (Next inlines the literal at build time either way) so unit tests can toggle
 *  the gate via process.env. Trailing slash stripped so `${base}/i/...` never
 *  doubles up. */
function mediaBaseUrl(): string {
	return (process.env.NEXT_PUBLIC_MEDIA_BASE_URL ?? "")
		.trim()
		.replace(/\/$/, "");
}

/** True once the media origin is configured. */
export function isMediaGatewayEnabled(): boolean {
	return mediaBaseUrl().length > 0;
}

function isPassthrough(value: string): boolean {
	return (
		value.startsWith("http://") ||
		value.startsWith("https://") ||
		value.startsWith("data:") ||
		value.startsWith("blob:")
	);
}

/**
 * The configured media origin's host: URLs on it are OUR own image-gateway
 * output, not foreign avatars. During the migration a backend field may already
 * come back as a display URL (`{mediaBaseUrl}/i/original/{key}`) rather than a
 * bare objectKey; to re-render it under a different preset we must first peel
 * the origin + `/i/{preset}/` gateway segment back to the bare key. Every other
 * host is foreign and passes through untouched. Mirrors backend `pkg/mediaurl`
 * `stripImageGatewayPath` / `NormalizeToObjectKey`.
 */
function configuredMediaHost(): string | null {
	const base = mediaBaseUrl();
	if (!base) return null;
	try {
		return new URL(base).host;
	} catch {
		return null;
	}
}

/**
 * If `value` is an absolute URL on the configured media host, strip the
 * origin and any leading `/i/{preset}/` gateway segment and return the bare
 * objectKey. Returns null for non-URLs (bare keys, data:/blob:) and for absolute
 * URLs on any other host (foreign avatars) — callers then fall back to their
 * normal passthrough / key-prefix handling.
 *
 * objectKeys never begin with `i/` (they start with public/, secure/, tmp/,
 * defaults/ …), so peeling the `i/{preset}/` prefix is unambiguous.
 */
function restoreMediaObjectKey(value: string): string | null {
	let host: string;
	let pathname: string;
	try {
		const u = new URL(value);
		host = u.host;
		pathname = u.pathname;
	} catch {
		return null; // not an absolute URL
	}
	if (host !== configuredMediaHost()) return null; // foreign host — leave to caller
	let rest = pathname.replace(/^\/+/, ""); // drop leading slash(es)
	if (rest.startsWith("i/")) {
		const afterI = rest.slice(2); // "{preset}/{objectKey}"
		const idx = afterI.indexOf("/");
		if (idx >= 0) rest = afterI.slice(idx + 1);
	}
	return rest;
}

/**
 * Turn a backend image value (objectKey or, transitionally, a full URL) into a
 * renderable URL. See the file header.
 */
export function buildImageUrl(
	value: string | null | undefined,
	preset: ImagePreset = "original",
): string {
	if (!value) return "";
	const trimmed = value.trim();
	if (!trimmed) return "";

	const base = mediaBaseUrl();

	// Peel our own media-host display URLs back to the objectKey so they
	// re-render under the requested preset.
	const restored = restoreMediaObjectKey(trimmed);
	if (restored !== null) return `${base}/i/${preset}/${restored}`;
	// Foreign URLs / data / blob are not objectKeys → passthrough.
	if (isPassthrough(trimmed)) return trimmed;
	// No media origin configured: there is no host to build an object URL on.
	if (!base) return "";
	const key = trimmed.startsWith("/") ? trimmed.slice(1) : trimmed;
	return `${base}/i/${preset}/${key}`;
}
