import { ApiClientError, type ApiEnvelope } from "../api/client";

export type TaggableEvent = { tags?: Record<string, unknown> };
export type SentryHint = { originalException?: unknown };

/**
 * 把後端 envelope 的 traceId 釘成 Sentry tag `backend_trace_id`，讓前端 error
 * 連到它在 Grafana Tempo 的 trace。設計成純函式，在各 Sentry.init 的 beforeSend
 * 內 compose 使用。非 ApiClientError 或無 traceId 時原樣返回。
 */
export function tagBackendTrace<T extends TaggableEvent>(
	event: T,
	hint?: SentryHint,
): T {
	const ex = hint?.originalException;
	if (ex instanceof ApiClientError) {
		const traceId = (ex.payload as ApiEnvelope<unknown> | undefined)?.traceId;
		if (
			(typeof traceId === "string" && traceId) ||
			ex.apiEndpoint ||
			ex.apiMethod ||
			ex.apiMode ||
			ex.apiRole ||
			ex.status
		) {
			if (event.tags === undefined) {
				event.tags = {};
			}
			if (typeof traceId === "string" && traceId) {
				event.tags.backend_trace_id = traceId;
			}
			if (ex.apiEndpoint) event.tags.api_endpoint = ex.apiEndpoint;
			if (ex.apiMethod) event.tags.api_method = ex.apiMethod;
			if (ex.apiMode) event.tags.api_mode = ex.apiMode;
			if (ex.apiRole) event.tags.api_role = ex.apiRole;
			if (ex.cloudflareRayId) {
				event.tags.cloudflare_ray_id = ex.cloudflareRayId;
			}
			event.tags.http_status = String(ex.status);
		}
	}
	return event;
}
