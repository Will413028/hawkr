import { type SentryLikeEvent, scrubSentryEvent } from "./scrub";
import {
	type SentryHint,
	type TaggableEvent,
	tagBackendTrace,
} from "./trace-tag";

/**
 * 瀏覽端 Sentry 的 `beforeSend` pipeline：對要上報的事件補後端 trace tag 與去識別化。
 */
export function clientBeforeSend<T extends SentryLikeEvent & TaggableEvent>(
	event: T,
	hint?: SentryHint,
): T {
	return scrubSentryEvent(tagBackendTrace(event, hint));
}
