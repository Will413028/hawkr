import { type SentryLikeEvent, scrubSentryEvent } from "./scrub";
import {
	type SentryHint,
	type TaggableEvent,
	tagBackendTrace,
} from "./trace-tag";

/**
 * Cloudflare 為每個 Worker 附掛的預覽網域後綴（`<script>.<account>.workers.dev`）。
 */
const PREVIEW_DEPLOYMENT_HOST_SUFFIX = ".workers.dev";

/**
 * Playwright post-deploy smoke injects this value before any application script
 * runs. It is deliberately an in-memory marker rather than a request header,
 * cookie, or query string: the marker must identify browser-side Sentry events
 * without changing cache behaviour or travelling to media / analytics origins.
 */
export const SYNTHETIC_SMOKE_GLOBAL = "__HAWKR_SYNTHETIC_SMOKE__";
export const SYNTHETIC_SMOKE_SOURCE = "cloudflare-post-deploy";

const SYNTHETIC_SMOKE_ID_PREFIX = `${SYNTHETIC_SMOKE_SOURCE}/`;
const SYNTHETIC_SMOKE_ID_MAX_LENGTH = 160;
const SYNTHETIC_SMOKE_ID_PATTERN =
	/^cloudflare-post-deploy\/[A-Za-z0-9][A-Za-z0-9._:-]*$/;

export type SyntheticSmokeMarker = {
	source: typeof SYNTHETIC_SMOKE_SOURCE;
	id: string;
};

type SyntheticTaggableEvent = TaggableEvent & {
	contexts?: Record<string, Record<string, unknown>>;
};

/**
 * The marker lives in a browser global and is therefore untrusted input. Keep
 * it observational only and accept exactly the CI-owned namespace and a small,
 * non-secret identifier alphabet. Invalid values are ignored fail-open: they
 * never suppress, downgrade, or otherwise change an event.
 */
export function parseSyntheticSmokeMarker(
	value: unknown,
): SyntheticSmokeMarker | null {
	if (!value || typeof value !== "object") return null;
	const candidate = value as { source?: unknown; id?: unknown };
	if (candidate.source !== SYNTHETIC_SMOKE_SOURCE) return null;
	if (typeof candidate.id !== "string") return null;
	if (
		candidate.id.length > SYNTHETIC_SMOKE_ID_MAX_LENGTH ||
		!SYNTHETIC_SMOKE_ID_PATTERN.test(candidate.id)
	) {
		return null;
	}
	return { source: SYNTHETIC_SMOKE_SOURCE, id: candidate.id };
}

function browserSyntheticSmokeMarker(): unknown {
	return (
		globalThis as typeof globalThis & {
			__HAWKR_SYNTHETIC_SMOKE__?: unknown;
		}
	)[SYNTHETIC_SMOKE_GLOBAL];
}

/**
 * Add only low-cardinality values as Sentry tags so Issues/Discover can split
 * synthetic traffic from real users without creating one tag value per deploy.
 * The complete run/deployment identifier remains available in event context for
 * exact forensic correlation.
 */
export function tagSyntheticSmoke<T extends SyntheticTaggableEvent>(
	event: T,
	markerValue: unknown = browserSyntheticSmokeMarker(),
): T {
	const marker = parseSyntheticSmokeMarker(markerValue);
	if (!marker) return event;

	if (event.tags === undefined) event.tags = {};
	event.tags.traffic_type = "synthetic";
	event.tags.synthetic_source = SYNTHETIC_SMOKE_SOURCE;

	if (event.contexts === undefined) event.contexts = {};
	event.contexts.synthetic_smoke = {
		...(event.contexts.synthetic_smoke ?? {}),
		source: marker.source,
		id: marker.id,
		deployment_id: marker.id.slice(SYNTHETIC_SMOKE_ID_PREFIX.length),
	};
	return event;
}

/**
 * 這個瀏覽端 origin 是不是 Cloudflare 的預覽網域。
 *
 * 為何要判斷：主要 post-deploy cf-smoke 現在一律走 custom domain，讓頁面與
 * `/api/v1/*` 都經過正式 first-party proxy 拓撲；`workers.dev` 只保留一支獨立的
 * noindex 過渡期檢查，直到入口正式關閉。這個 legacy host 不具備 custom-domain
 * API route，瀏覽器事件也不是使用者流量，因此不應污染正式 Sentry project。
 *
 * 因此預覽網域上的事件一律丟棄，且刻意做成**網域級**而非只擋 404。正式網域
 * （example.com / www / web-dev）的同類錯誤仍照常上報——那裡的 404 才是真
 * 缺陷，不可一起靜音。
 */
export function isPreviewDeploymentHost(
	hostname: string | null | undefined,
): boolean {
	if (typeof hostname !== "string") return false;
	return hostname.toLowerCase().endsWith(PREVIEW_DEPLOYMENT_HOST_SUFFIX);
}

/**
 * 瀏覽端 Sentry 的 `beforeSend` pipeline：先擋預覽網域噪音，再對真正要上報的事件
 * 補後端 trace tag 與去識別化。`hostname` 參數僅供測試注入；正式路徑讀當前 origin
 * （讀不到 → 視為非預覽網域，寧可多報也不要讓真使用者的錯誤靜默消失）。
 */
export function clientBeforeSend<T extends SentryLikeEvent & TaggableEvent>(
	event: T,
	hint?: SentryHint,
	hostname: string | null | undefined = globalThis.location?.hostname,
	syntheticMarker: unknown = browserSyntheticSmokeMarker(),
): T | null {
	if (isPreviewDeploymentHost(hostname)) return null;
	return scrubSentryEvent(
		tagSyntheticSmoke(tagBackendTrace(event, hint), syntheticMarker),
	);
}
