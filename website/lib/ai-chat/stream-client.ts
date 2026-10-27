/**
 * 通用 AI SSE 聊天傳輸層 — 從 lib/seller/ai-listing-chat.ts 抽出（見該檔案
 * 現況：feature-flag 關閉但是真的在跑的程式碼，抽出時維持其對外行為不變）。
 *
 * Wire 是 AG-UI 協定（backend-payuni 2026-08-17 全面改用，見 gateway
 * `ai_assistant_stream.go`）：SSE frame 只有 `data:` 行，沒有 `event:` 行，
 * 事件類型靠 JSON `type` 欄位分派。這層只處理 AG-UI 的 framing 與
 * TEXT_MESSAGE_CONTENT/RUN_FINISHED/RUN_ERROR 三種幾乎所有 AI 聊天 endpoint
 * 都會用到的通用事件，轉呼叫端熟悉的 onText/onDone/onError。CUSTOM 事件
 * （例如 procurement 的 step_detail）與 TOOL_CALL_RESULT（例如 seller-match
 * 的候選卡）轉呼叫端透過 onNamedEvent 自行解析 payload——不把單一 endpoint
 * 的 payload 形狀綁進這層共用邏輯。TOOL_CALL_RESULT 轉發時用的具名事件是
 * 對應 TOOL_CALL_START 帶的 `toolCallName`，一個串流呼叫一份對照表，不跨
 * 呼叫共用。
 */

import { AUTH_EXPIRED_EVENT, apiClient } from "@/lib/api/client";
import { parseJsonSafe } from "@/lib/api/json";
import { getStoredActiveRole, roleToHeader } from "@/lib/auth/active-role";
import { withClerkAuthorization } from "@/lib/auth/clerk-token";
import {
	type AgUiEventHandlers,
	createAgUiEventDispatcher,
	type RunOutcome,
} from "./agui-dispatch";

/** 遇 stall（讀不到下一個 chunk）時等待的上限，對齊 gateway proxy handler
 * 的 `aiStreamChunkTimeout`（backend-payuni `gateway/internal/handler/ai.go`）
 * ——後端本來就會在同樣的窗口內判定該次串流已死，前端跟著用同一個數字，
 * 逾時時機不會落後太多。 */
export const AI_STREAM_STALL_TIMEOUT_MS = 30_000;

/** stall timeout 專用的錯誤型別，讓呼叫端（例如 UI 的 catch 區塊）可以跟
 * 使用者主動 abort（`AbortError`）分開處理，而不是被當成一般網路錯誤。 */
export class AiStreamTimeoutError extends Error {
	constructor(message = "AI 沒有回應，請重新嘗試") {
		super(message);
		this.name = "AiStreamTimeoutError";
	}
}

/** non-2xx 開場失敗（連線還沒切成 SSE，所以還拿得到狀態碼）。呼叫端要
 * 靠狀態碼分流：404/405＝這個環境的 gateway 還沒部署串流端點、429＝併發
 * 上限已滿，兩者都該降級走同步端點；其餘才是真的錯誤。
 *
 * `reason` 是這個 codebase 通用的 `{code,reason,message}` 業務錯誤契約裡的
 * 機器碼（同 `lib/api/client.ts` 的 `ApiClientError.reason`），從錯誤回應的
 * JSON envelope 解析；沒有這個欄位（或 body 不是 JSON）一律預設空字串，不是
 * 猜測值。 */
export class AiStreamHttpError extends Error {
	constructor(
		readonly status: number,
		message: string,
		readonly reason: string = "",
	) {
		super(message);
		this.name = "AiStreamHttpError";
	}
}

export interface AiStreamRequest {
	/** Gateway path，例如 "/app/buyers/me/assistant/seller-match" */
	endpoint: string;
	/** 請求 body，呼叫端自行決定欄位命名——不同 endpoint 的後端 struct tag
	 * 可能不同調（見 ai-listing-chat.ts 的 session_id/sessionId 教訓），
	 * 這層不代為猜測。 */
	body: Record<string, unknown>;
	signal?: AbortSignal;
}

/** 這一層的 handler 形狀就是 AG-UI 的 handler 形狀（見 ./agui-dispatch）；
 * 保留這個名字是因為 turn-stream.ts 與 seller-match-chat.ts 都用它。 */
export type AiStreamHandlers = AgUiEventHandlers;
export type { RunOutcome };

/**
 * 開啟一個串流聊天請求，resolve 於串流結束（或被 abort）。
 * 只在網路失敗／non-2xx 時 reject；SSE 層級的 error event 走 handlers.onError。
 */
export async function streamAiChat(
	req: AiStreamRequest,
	handlers: AiStreamHandlers,
): Promise<void> {
	const url = apiClient.buildUrl("authed", req.endpoint);
	const headers = new Headers({
		"Content-Type": "application/json",
		Accept: "text/event-stream",
	});
	const activeRole = roleToHeader(getStoredActiveRole());
	if (activeRole) headers.set("X-Active-Role", activeRole);
	await withClerkAuthorization(headers);
	const init: RequestInit = {
		method: "POST",
		headers,
		body: JSON.stringify(req.body),
		credentials: "include",
		signal: req.signal,
		cache: "no-store",
	};
	let res = await fetch(url, init);
	if (res.status === 401 && headers.has("Authorization")) {
		await withClerkAuthorization(headers, { skipCache: true });
		res = await fetch(url, init);
	}

	if (!res.ok || !res.body) {
		const text = await res.text().catch(() => "");
		if (res.status === 401 && typeof window !== "undefined") {
			window.dispatchEvent(
				new CustomEvent(AUTH_EXPIRED_EVENT, {
					detail: {
						path: req.endpoint,
						message: extractErrorMessage(text) || "Unauthorized",
					},
				}),
			);
		}
		throw new AiStreamHttpError(
			res.status,
			extractErrorMessage(text) || text || `AI 對話失敗（HTTP ${res.status}）`,
			extractErrorReason(text),
		);
	}

	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	const dispatch = createDispatcher(handlers);
	let buffer = "";

	while (true) {
		let result: ReadableStreamReadResult<Uint8Array>;
		try {
			result = await readWithStallTimeout(reader, AI_STREAM_STALL_TIMEOUT_MS);
		} catch (e) {
			if (e instanceof AiStreamTimeoutError) {
				// 讀不到下一個 chunk——關掉這個 reader（連帶讓底層連線收掉），
				// 不留一個永遠 pending 的 fetch。
				await reader.cancel().catch(() => {});
			}
			throw e;
		}
		const { value, done } = result;
		if (done) break;
		buffer += decoder.decode(value, { stream: true });

		// Standard SSE: events separated by blank line.
		while (true) {
			const blankIdx = buffer.indexOf("\n\n");
			if (blankIdx === -1) break;
			const raw = buffer.slice(0, blankIdx);
			buffer = buffer.slice(blankIdx + 2);
			dispatch(raw);
		}
	}

	// Drain any final event without trailing blank line
	if (buffer.trim()) dispatch(buffer);
}

/** `reader.read()`，但讀不到下一個 chunk 超過 `timeoutMs` 就 reject —— 一次性
 * POST 聊天沒有像 `lib/sse/client.ts` 那樣的 heartbeat/reconnect 機制，這裡只
 * 補最小限度的「不要卡死」防線：每次呼叫都重新計時（stall 視窗，不是整條
 * 串流的總時長上限）。 */
function readWithStallTimeout(
	reader: ReadableStreamDefaultReader<Uint8Array>,
	timeoutMs: number,
): Promise<ReadableStreamReadResult<Uint8Array>> {
	let timer: ReturnType<typeof setTimeout>;
	const stall = new Promise<never>((_, reject) => {
		timer = setTimeout(() => reject(new AiStreamTimeoutError()), timeoutMs);
	});
	return Promise.race([reader.read(), stall]).finally(() => {
		clearTimeout(timer);
	});
}

/** 從 gateway 的 error envelope（`{"code":...,"message":...,"messageZh":...}`）
 * 取出人類看得懂的訊息；非 JSON 或沒有對應欄位一律回傳 null，交給呼叫端退回
 * raw text（同 lib/seller/ai-image.ts 的 messageZh/message/msg 優先序）。 */
function extractErrorMessage(text: string): string | null {
	if (!text.trim()) return null;
	let parsed: unknown;
	try {
		parsed = parseJsonSafe(text);
	} catch {
		return null;
	}
	if (typeof parsed !== "object" || parsed === null) return null;
	const envelope = parsed as {
		message?: string;
		messageZh?: string;
		msg?: string;
	};
	const message = envelope.messageZh || envelope.message || envelope.msg;
	return typeof message === "string" && message.trim() ? message : null;
}

/** 同 `extractErrorMessage`，但取 `reason`——procurement 等端點通用的
 * `{code,reason,message}` 業務錯誤契約裡的機器碼（key 名稱與大小寫對齊
 * `lib/api/client.ts` 的 `ApiClientError.reason`）。非 JSON、沒有這個欄位
 * 一律回傳空字串，跟 `ApiClientError` 的 `reason ?? ""` 預設值同一個道理，
 * 不是猜測值。 */
function extractErrorReason(text: string): string {
	if (!text.trim()) return "";
	let parsed: unknown;
	try {
		parsed = parseJsonSafe(text);
	} catch {
		return "";
	}
	if (typeof parsed !== "object" || parsed === null) return "";
	const envelope = parsed as { reason?: string };
	return typeof envelope.reason === "string" ? envelope.reason : "";
}

/** SSE framing 專屬：把一個 frame 的 `data:` 行併起來、解析成物件，其餘交給
 * 共用的 AG-UI dispatcher。這個端點的 frame 不帶 `id:` 行（那是 GET replay
 * 專屬的 ledger-seq cursor，doc57 §2.4），所以這裡也不讀。 */
function createDispatcher(handlers: AiStreamHandlers): (raw: string) => void {
	const dispatchEvent = createAgUiEventDispatcher(handlers);

	return (raw: string) => {
		const dataLines: string[] = [];
		for (const line of raw.split("\n")) {
			if (line.startsWith("data:")) {
				dataLines.push(line.slice(5).replace(/^\s/, ""));
			}
		}
		const dataRaw = dataLines.join("\n");
		if (!dataRaw) return;

		let event: unknown;
		try {
			event = JSON.parse(dataRaw);
		} catch {
			return;
		}
		dispatchEvent(event);
	};
}
