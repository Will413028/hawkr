import { isVercelDeploymentHost } from "@/lib/seo/canonical-host";
import { type SentryLikeEvent, scrubSentryEvent } from "./scrub";
import {
	type SentryHint,
	type TaggableEvent,
	tagBackendTrace,
} from "./trace-tag";

/**
 * 這個瀏覽端 origin 是不是 Vercel 的部署網域（`*.vercel.app`，每次部署與 preview
 * 都有一個）。以 hostname 後綴比對，`vercel.app.example.com` 不算。
 *
 * 為何要判斷：這些網域上的事件不是正式使用者流量，不應污染正式 Sentry project，
 * 因此一律丟棄，且刻意做成網域級而非只擋 404；正式網域的同類錯誤仍照常上報。
 */
export function isPreviewDeploymentHost(
	hostname: string | null | undefined,
): boolean {
	if (typeof hostname !== "string") return false;
	return isVercelDeploymentHost(hostname.toLowerCase());
}

/**
 * 瀏覽端 Sentry 的 `beforeSend` pipeline：先擋預覽網域事件，再對要上報的事件補後端
 * trace tag 與去識別化。`hostname` 參數僅供測試注入；正式路徑讀當前 origin
 * （讀不到 → 視為非預覽網域，寧可多報也不要讓真使用者的錯誤靜默消失）。
 */
export function clientBeforeSend<T extends SentryLikeEvent & TaggableEvent>(
	event: T,
	hint?: SentryHint,
	hostname: string | null | undefined = globalThis.location?.hostname,
): T | null {
	if (isPreviewDeploymentHost(hostname)) return null;
	return scrubSentryEvent(tagBackendTrace(event, hint));
}
