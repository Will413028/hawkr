import { ApiClientError } from "@/services/api-error";

type TaggableEvent = { tags?: Record<string, unknown> };
type SentryHint = { originalException?: unknown };

/**
 * 把後端 envelope 的 traceId 釘成 Sentry tag `backend_trace_id`，讓前端 error
 * 連到它在 Grafana Tempo 的 trace。純函式，於各 Sentry.init 的 beforeSend 內
 * compose。非 ApiClientError 或無 traceId 時原樣返回。
 *
 * 回傳 T & TaggableEvent（而非純 T）讓呼叫端保留 tags 的 index signature；admin 的
 * tsc 含 tests/，故需此標註。
 */
export function tagBackendTrace<T extends TaggableEvent>(
	event: T,
	hint?: SentryHint,
): T & TaggableEvent {
	const ex = hint?.originalException;
	if (ex instanceof ApiClientError && ex.traceId) {
		if (event.tags === undefined) {
			event.tags = {};
		}
		event.tags.backend_trace_id = ex.traceId;
	}
	return event;
}
