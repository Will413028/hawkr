/**
 * Image URL assembly — single entry point (design book 49 §6.5 / §8.6).
 *
 * `buildImageUrl(keyOrUrl, preset)` is the only sanctioned way to turn a backend
 * image value into a renderable URL. The only media host the app knows is the
 * configured origin `env.mediaBaseUrl` (NEXT_PUBLIC_MEDIA_BASE_URL); there is no
 * built-in host to fall back to.
 *
 *   Origin configured — image-gateway behaviour. The value is treated as an
 *     objectKey and rewritten to `{mediaBaseUrl}/i/{preset}/{objectKey}`, which
 *     the Cloudflare Worker serves as the preset-transformed image (`original` =
 *     no transform). Already-absolute foreign URLs, data:/blob: URLs and local
 *     /public assets pass through untouched. A URL on the configured origin
 *     without a query / fragment is restored to its key so a new preset can be
 *     applied. A URL with either suffix is treated as opaque and preserved
 *     byte-for-byte: rewriting its path could invalidate an access signature,
 *     and dropping the suffix would silently change its resource identity.
 *
 *   No origin configured — a bare object key yields no URL (`""`), so the image
 *     is absent rather than pointing at a host nobody configured. Absolute /
 *     data / blob / local-public values still pass through untouched.
 *
 * `resolveCdnUrl` / `ossImageUrl` are thin wrappers so the ~60 existing call
 * sites keep working unchanged.
 */

import { getEnv } from "@/lib/config/env";
import { isGifMedia } from "@/lib/media/gif";
import type { ImagePreset } from "./image-presets.generated";

export { isGifMedia } from "@/lib/media/gif";

/**
 * Preset whitelist (design book 49 §4.4). The frontend only ever emits a preset
 * NAME; the Cloudflare Worker (`workers/image-gateway/presets.ts`) owns the
 * actual transform parameters, so adding/tuning a preset needs no web release.
 * `original` = serve the source object unmodified (full-screen view, quality-
 * sensitive description images, non-image files). 名稱由 presets.ts 生成
 * （ADR 2026-09-25-api-image-uri-template D4）。
 */
export type { ImagePreset };

/**
 * 貼文卡片對 GIF 送 `motion`：動圖會動，而且尺寸有界。
 *
 * `motion` 是 image-gateway 的動圖 preset（720 長邊、scale-down、q=70，`anim: true`），
 * backend-payuni#2831 新增，49 號設計書 §4.4 有表。對已部署的 dev worker 實測同一顆
 * 720×540 / 73 幀的貼文 GIF：
 *
 *   /i/original/  10,233,546 B  image/gif   73 幀   ← 不經轉換，使用者上傳的原檔
 *   /i/motion/     2,658,380 B  image/webp  73 幀   ← −74%，動畫完整保留
 *   /i/motion/     3,426,023 B  image/gif   73 幀   ← 不支援 webp 的客戶端
 *   /i/card/          32,815 B  image/jpeg   1 幀   ← 靜態，動畫沒了
 *
 * 為什麼不是 `card`：卡片動畫是刻意的產品行為（GIF 發文與逐幀取景是 2026-08-26
 * 才做的功能，賣家端文案就寫著「拖曳 GIF 調整每一幀取景」）。
 * 為什麼不是 `original`：那 10 MB 在首頁與其他 100+ 張圖搶同一個 media host，
 * 把頁面 settle 從 ~9 秒拖到 ~28 秒（實測見 website#528）。
 *
 * ⚠️ 2026-08-27 我曾把這裡改成一律回 `stillPreset`（卡片變靜態）好讓
 * `site-stability.spec.ts:666` 的 12 秒 no-pop-in 斷言通過。**那是錯的順序**：
 * 測試過不去時該先放寬門檻，產品行為的取捨由功能的擁有者決定，不由 CI 的
 * 時間預算決定。正解是補上缺的那個 derivative，也就是現在這個 `motion`。
 *
 * ⚠️ image-gateway 轉換失敗時會 degrade 直出原檔（49 號 §7.3），對 `motion` 而言
 * 就是**退回那 10 MB 而 HTTP 仍是 200**。所以那一側的保證不是靠狀態碼，是靠
 * `workers/image-gateway/scripts/verify-motion-preset.mjs` 實測位元組數與幀數。
 */
export function postCardPresetForMedia(
	value: string | null | undefined,
	stillPreset: ImagePreset = "card",
): ImagePreset {
	return isGifMedia(value) ? "motion" : stillPreset;
}

/**
 * Backend objectKey namespaces (mirrors `pkg/mediaurl`: an objectKey always
 * begins with `public/`, `secure/`, `tmp/` …, and an image-gateway path always
 * begins with `i/`). These are the only first path segments that stay a remote
 * object even when the value arrives with a stray leading slash — the backend
 * canonical form is slash-less, but `/public/x.jpg` must still resolve to the
 * CDN / gateway rather than to this app's origin.
 *
 * Local assets live under `public/placeholders/` and the other top-level `public/`
 * directories; the leading slash is what separates `/placeholders/post.svg`
 * from an objectKey such as `public/post/1.jpg`.
 */
const OBJECT_KEY_NAMESPACES: ReadonlySet<string> = new Set([
	"public",
	"secure",
	"tmp",
	"i",
]);

/**
 * True when `value` addresses a file shipped in this app's `public/` folder —
 * never on the CDN nor the media gateway, so it is returned as-is and <img>
 * resolves it against the page origin instead of 404ing remotely.
 *
 * The rule is structural rather than a per-directory allowlist: Next.js serves
 * `public/**` from the page origin under a LEADING SLASH, while backend
 * objectKeys are slash-less (`public/product/1/a.jpg`). So any rooted path is a
 * local asset unless its first segment is a reserved backend namespace. Adding
 * a new directory under `public/` therefore needs no edit here — the previous
 * allowlist silently CDN-prefixed (→ 404) every directory nobody remembered to
 * register.
 *
 * `//host/x` and `/\host/x` are protocol-relative: the browser would leave our
 * origin, so they are never local assets.
 */
function isLocalPublicAsset(value: string): boolean {
	if (!value.startsWith("/")) return false;
	if (value[1] === "/" || value[1] === "\\") return false;
	const firstSegment = value.slice(1).split(/[/?#]/, 1)[0];
	return !OBJECT_KEY_NAMESPACES.has(firstSegment);
}

/** True once the media origin is configured. Exported so
 *  paths that cannot be expressed as a preset (e.g. OSS `image/info` probing)
 *  can disable themselves cleanly rather than emit a broken URL. */
export function isMediaGatewayEnabled(): boolean {
	return getEnv().mediaBaseUrl.length > 0;
}

function isPassthrough(value: string): boolean {
	return (
		value.startsWith("http://") ||
		value.startsWith("https://") ||
		value.startsWith("data:") ||
		value.startsWith("blob:") ||
		isLocalPublicAsset(value)
	);
}

/**
 * True when `hostname` is the configured media origin's host: those URLs are
 * OUR own image-gateway output, not foreign avatars. During the migration a
 * backend field may already come back as a display URL
 * (`{mediaBaseUrl}/i/original/{key}`) rather than a bare objectKey; to re-render
 * it under a different preset we must first peel the origin + `/i/{preset}/`
 * gateway segment back to the bare key. Every other host is foreign and passes
 * through untouched. Mirrors backend `pkg/mediaurl` `stripImageGatewayPath` /
 * `NormalizeToObjectKey`.
 */
function isConfiguredMediaHost(hostname: string): boolean {
	const base = getEnv().mediaBaseUrl;
	if (!base) return false;
	return (
		normalizeMediaHostname(new URL(base).hostname) ===
		normalizeMediaHostname(hostname)
	);
}

/**
 * True when `url` is on the configured media origin (same scheme, host and port).
 * The allow-list for every server-side fetch of a client-supplied image URL:
 * with no media origin configured nothing is allowed.
 */
export function isConfiguredMediaUrl(url: URL): boolean {
	const base = getEnv().mediaBaseUrl;
	if (!base) return false;
	const origin = new URL(base);
	// `URL.port` is "" for a scheme's default port, so an explicit `:443` on an
	// https URL equals the configured origin without one.
	return (
		url.protocol === origin.protocol &&
		url.port === origin.port &&
		isConfiguredMediaHost(url.hostname)
	);
}

export function normalizeMediaHostname(hostname: string): string {
	// A fully-qualified DNS name may carry a trailing dot. Browsers resolve it
	// to the same host, so host-boundary checks must not let that spelling turn
	// the media host into a "foreign" passthrough URL.
	return hostname.toLowerCase().replace(/\.+$/, "");
}

interface RestoredMediaObject {
	objectKey: string;
	sourceUrl: URL;
	/** Query / fragments are opaque: they may be a signature or cache identity. */
	hasOpaqueSuffix: boolean;
}

/**
 * If `value` is an absolute URL on the configured media host, strip the
 * origin and any leading `/i/{preset}/` gateway segment and return the bare
 * objectKey together with the parsed source URL. Returns null for non-URLs
 * (bare keys, data:/blob:, local assets) and for absolute URLs on any other
 * host (foreign avatars) — callers then fall back to their normal passthrough /
 * key-prefix handling.
 *
 * objectKeys never begin with `i/` (they start with public/, secure/, tmp/,
 * defaults/ …), so peeling the `i/{preset}/` prefix is unambiguous.
 */
function restoreMediaObjectKey(value: string): RestoredMediaObject | null {
	let sourceUrl: URL;
	try {
		sourceUrl = new URL(value);
	} catch {
		return null; // not an absolute URL
	}
	if (!isConfiguredMediaHost(sourceUrl.hostname)) return null;
	let rest = sourceUrl.pathname.replace(/^\/+/, ""); // drop leading slash(es)
	if (rest.startsWith("i/")) {
		const afterI = rest.slice(2); // "{preset}/{objectKey}"
		const idx = afterI.indexOf("/");
		if (idx >= 0) rest = afterI.slice(idx + 1);
	}
	return {
		objectKey: rest,
		sourceUrl,
		// URL.search / URL.hash are empty for a bare `?` / `#`; inspect the raw
		// absolute URL as well so even those delimiters remain opaque.
		hasOpaqueSuffix: value.includes("?") || value.includes("#"),
	};
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

	const base = getEnv().mediaBaseUrl;

	// A value already on the configured media host is peeled back to its
	// objectKey so it re-renders under the requested preset. A query or fragment
	// makes the existing URL opaque: changing its path can invalidate a
	// signature, so the entire URL is preserved instead.
	const restored = restoreMediaObjectKey(trimmed);
	if (restored !== null) {
		if (restored.hasOpaqueSuffix) return trimmed;
		return `${base}/i/${preset}/${restored.objectKey}`;
	}
	// Foreign URLs / data / blob / local assets are not objectKeys → passthrough.
	if (isPassthrough(trimmed)) return trimmed;
	// No media origin configured: there is no host to build an object URL on.
	if (!base) return "";
	const key = trimmed.startsWith("/") ? trimmed.slice(1) : trimmed;
	return `${base}/i/${preset}/${key}`;
}

/**
 * WRITE-path counterpart of `buildImageUrl`: reduce a media value to the bare
 * objectKey the backend is allowed to store (49 §6.1 / 10 檔 §25 — 前端可控輸入
 * 只收 objectKey，完整 URL 一律不得落庫). Mirrors backend
 * `pkg/mediaurl.NormalizeToObjectKey` / `handler.resolveInputObjectKey`.
 *
 * Returns `""` for anything that is not (and cannot be reduced to) one of our
 * object keys — a foreign host, a `data:`/`blob:` URL, a local app asset, or a
 * media URL carrying an opaque query/fragment whose identity we must not
 * rewrite. Callers treat `""` as "no objectKey to send" rather than shipping a
 * value the backend would persist verbatim.
 */
export function toObjectKey(value: string | null | undefined): string {
	if (!value) return "";
	const trimmed = value.trim();
	if (!trimmed) return "";
	const restored = restoreMediaObjectKey(trimmed);
	if (restored) return restored.hasOpaqueSuffix ? "" : restored.objectKey;
	if (isPassthrough(trimmed)) return "";
	return trimmed.startsWith("/") ? trimmed.slice(1) : trimmed;
}

/**
 * Backward-compatible wrapper for the ~40 call sites that only need domain
 * resolution (raw display, no explicit preset). Equivalent to requesting the
 * `original` image: the Worker `/i/original/` URL.
 */
export function resolveCdnUrl(value: string | null | undefined): string {
	return buildImageUrl(value, "original");
}

/**
 * Legacy preset names (pre-R2) mapped onto the design-book 49 §4.4 whitelist.
 * Kept so the existing `ossImageUrl(value, "avatar")` call sites need no edit;
 * new code should call `buildImageUrl` with a §4.4 preset directly.
 */
export type LegacyOssPreset =
	| "avatar"
	| "productThumb"
	| "feedCard"
	| "cardThumb"
	| "productDetail"
	| "sellerCover";

const LEGACY_PRESET_MAP: Record<LegacyOssPreset, ImagePreset> = {
	avatar: "avatar",
	productThumb: "thumb",
	feedCard: "card",
	cardThumb: "card",
	productDetail: "detail",
	sellerCover: "banner",
};

/**
 * Backward-compatible wrapper for raw <img> call sites that requested a named
 * layout preset. Maps the legacy preset name onto the §4.4 whitelist and defers
 * to `buildImageUrl`.
 */
export function ossImageUrl(
	value: string | null | undefined,
	preset: LegacyOssPreset,
): string {
	return buildImageUrl(value, LEGACY_PRESET_MAP[preset]);
}
