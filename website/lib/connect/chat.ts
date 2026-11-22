import { createClient } from "@connectrpc/connect";
import { ChatAppService } from "@hawkr/api/gen/hawkr/chat/v1alpha1/chat_pb";
import { authedTransport, streamTransport } from "./transport";

/** ChatAppService 的一元呼叫（列表、送出、已讀、未讀、在線）。 */
export function chatAppClient() {
	return createClient(ChatAppService, authedTransport());
}

/**
 * ChatAppService 的串流呼叫（`StreamEvents`、`StreamStoreEvents`）：用沒有逾時、沒有重試的 `streamTransport`，
 * 串流的壽命由伺服器的 `max_lifetime` 決定，重連由 `lib/chat/stream.ts` 負責。
 */
export function chatStreamClient() {
	return createClient(ChatAppService, streamTransport());
}
