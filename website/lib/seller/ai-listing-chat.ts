/**
 * SSE client for the AI product-listing assistant. Mirrors the mobile
 * Flutter implementation at
 *   lib/features/product/presentation/product_listing_chat/
 *     product_listing_chat_repository.dart
 *
 * The endpoint isn't in OpenAPI but the mobile app has been talking to it
 * for months:
 *   POST /api/v1/app/sellers/me/assistant/product-listing/chat
 *   body: { message, sessionId?, imageUrl? }
 *   stream: text/event-stream
 *     event: text         data: "<chunk>"
 *     event: draft_patch  data: <Partial<DraftPatch>>
 *     event: done         data: { sessionId }
 *     event: error        data: "<msg>"
 *
 * Both request and response now use camelCase. ai-service struct tags and
 * Gateway proxy updated in coordinated rollout (backend-payuni PR).
 *
 * Auth: streamAiChat sends the same-origin session cookie (`__Host-hk_session`).
 */

import { streamAiChat } from "@/lib/ai-chat/stream-client";

export interface AiDraftPatch {
	name?: string;
	origin?: string;
	categoryId?: string | number;
	description?: string;
	tags?: string[];
	skus?: AiSkuDraft[];
	certifications?: AiCertDraft[];
}

export interface AiSkuDraft {
	skuName?: string;
	price?: string | number;
	unit?: string;
	weight?: string | number;
	stockQty?: string | number;
}

export interface AiCertDraft {
	name?: string;
	certNo?: string;
}

export interface AiChatHandlers {
	onText: (chunk: string) => void;
	onDraftPatch: (patch: AiDraftPatch) => void;
	onDone: (sessionId: string | null) => void;
	onError: (message: string) => void;
}

export interface AiChatRequest {
	message: string;
	sessionId?: string | null;
	imageUrl?: string;
	signal?: AbortSignal;
}

const ENDPOINT = "/app/sellers/me/assistant/product-listing/chat";

/**
 * Open a streaming chat session. Resolves when the stream ends (or is
 * aborted). Reject only on network / non-2xx; SSE-level errors come
 * through `handlers.onError`.
 */
export async function streamAiListingChat(
	req: AiChatRequest,
	handlers: AiChatHandlers,
): Promise<void> {
	return streamAiChat(
		{
			endpoint: ENDPOINT,
			body: {
				message: req.message,
				...(req.sessionId ? { sessionId: req.sessionId } : {}),
				...(req.imageUrl ? { imageUrl: req.imageUrl } : {}),
			},
			signal: req.signal,
		},
		{
			onText: handlers.onText,
			onNamedEvent: (eventName, data) => {
				if (eventName !== "draft_patch") return;
				try {
					const patch = JSON.parse(data) as AiDraftPatch;
					handlers.onDraftPatch(patch);
				} catch {
					/* swallow malformed patches — onText already received text */
				}
			},
			onDone: (data) => {
				let sessionId: string | null = null;
				try {
					const obj = JSON.parse(data) as { sessionId?: string };
					sessionId = obj?.sessionId ?? null;
				} catch {
					/* allow plain "done" with no payload */
				}
				handlers.onDone(sessionId);
			},
			onError: handlers.onError,
		},
	);
}
