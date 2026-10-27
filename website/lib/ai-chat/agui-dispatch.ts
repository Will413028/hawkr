/**
 * AG-UI 事件的解碼與分派——從 lib/ai-chat/stream-client.ts 抽出，因為
 * 那支只服務 POST 一次性串流，而 GET replay 端點
 * （features/procurement-assistant/run-events-stream.ts）走的是
 * lib/sse/client.ts：SSE framing 由後者做完，兩條路真正共用的只有「拿到一個
 * 已解析的事件物件之後怎麼驗、怎麼分派」。
 *
 * 抽出的重點是 EventSchemas.safeParse 這個 conformance anchor 只有一份：
 * gateway 的手刻 encoder 若偏離 AG-UI spec，兩條 transport 會在同一個地方
 * 明確失敗，而不是其中一條靜默少畫一列。
 */

import { EventSchemas, type RunFinishedOutcome } from "@ag-ui/core";

/** AG-UI `RUN_FINISHED.outcome` 收斂成呼叫端要的扁平形狀。官方 schema 的
 * `interrupts` 是陣列（保留擴充成多個 interrupt 的彈性），但 v1 一次 run
 * 最多一個未結案 interrupt（doc57 ledger 設計的前提），這裡只取第一筆；
 * 跟 backend Task 15 REST snapshot 的 `openConfirmation` 一樣，對外只曝露單一
 * interruptId／reason，不管走的是哪種 transport，這層給到的形狀要一致。 */
export type RunOutcome =
	| { type: "success" }
	| { type: "interrupt"; interruptId: string; reason: string };

export interface AgUiEventHandlers {
	onText: (chunk: string) => void;
	/** text/done/error 以外的具名 event，呼叫端自行 JSON.parse 並窄化型別。 */
	onNamedEvent: (eventName: string, rawData: string) => void;
	/** done event 的 raw data字串，呼叫端自行解析（payload 形狀因 endpoint 而異）。
	 * `outcome` 是 `RUN_FINISHED.outcome` 收斂後的結果——沒有這個欄位的舊
	 * frame，向後相容視為 legacy success；不符已知形狀的 outcome（理論上不會
	 * 發生，schema 早一步擋下）同樣收斂成 `{type:"success"}`，不臆測一個不
	 * 存在的 interrupt。 */
	onDone: (rawData: string, outcome: RunOutcome) => void;
	/** `code` 是 AG-UI `RUN_ERROR.code` 轉成數字。缺值或不合法時省略，
	 * 不從 message 猜出一個看似合理的業務碼。
	 * `reason` 是這個 codebase 通用的 `{code,reason,message}` 業務錯誤契約裡的
	 * 機器碼（同 `ApiClientError.reason`）——官方 AG-UI `RunErrorEventSchema`
	 * 沒有這個欄位，所以這裡是從未經 schema 裁剪的原始 wire JSON 另外補讀
	 * 出來的，不合法或缺值時省略。 */
	onError: (message: string, code?: number, reason?: string) => void;
	/** 沒通過 `EventSchemas.safeParse` 的 frame。**省略時退回 `onError`**，也就
	 * 是 POST turn stream 的既有行為：那條路的每個 frame 都是這一輪的內容，
	 * 契約不符等於這一輪的輸出已經不可信，整輪失敗是對的。
	 *
	 * GET replay 那條端點不一樣：它是一整個 run 的 ledger 重播，合法地帶著這個
	 * client 不消費的 frame 型別，而且 conformance anchor 擋下一個 frame 不代表
	 * 後面的 frame 也壞了。所以那條路提供這個 handler——大聲記一筆、只丟掉這個
	 * frame、訂閱繼續；真正該停的只有 in-band `RUN_ERROR`。 */
	onInvalidEvent?: (message: string) => void;
}

/** `record.outcome` 在這裡已經是 `EventSchemas.safeParse` 驗證過的
 * `RunFinishedOutcome`，不用再對 `unknown` 防禦性解析。`interrupts` 陣列在
 * schema 上有 `min(1)`，不合這個形狀的 outcome 早在 safeParse 就整個 frame
 * 一起被拒絕（走 onError），走不到這裡；`raw.interrupts[0]` 的存在性判斷
 * 因此只是型別層面的保守寫法，不是真的預期會落空。 */
function toRunOutcome(raw: RunFinishedOutcome | null | undefined): RunOutcome {
	if (raw?.type === "interrupt" && raw.interrupts[0]) {
		return {
			type: "interrupt",
			interruptId: raw.interrupts[0].id,
			reason: raw.interrupts[0].reason,
		};
	}
	return { type: "success" }; // 向後相容預設 success（異常形狀已在 schema 層擋下，走不到這裡）
}

/** 一個串流呼叫一份 dispatcher；toolCallId 的命名空間不可跨請求共用。 */
export function createAgUiEventDispatcher(
	handlers: AgUiEventHandlers,
): (event: unknown) => void {
	const toolCallNames = new Map<string, string>();

	return (event: unknown) => {
		// 用官方 zod schema 驗證，而不是自己檢查欄位——這一步是本層的
		// conformance anchor：gateway 的手刻 encoder 若偏離 AG-UI spec，
		// 會在這裡明確失敗，而不是靜默少畫一個氣泡。
		const parsed = EventSchemas.safeParse(event);
		if (!parsed.success) {
			const message = `AI 回應格式不符 AG-UI 契約：${parsed.error.issues[0]?.message ?? "unknown"}`;
			// 有 onInvalidEvent 就只丟這一個 frame，沒有就照舊整輪失敗（見
			// AgUiEventHandlers.onInvalidEvent 的說明）。
			if (handlers.onInvalidEvent) handlers.onInvalidEvent(message);
			else handlers.onError(message);
			return;
		}
		const record = parsed.data;

		switch (record.type) {
			case "TEXT_MESSAGE_CONTENT":
				if (typeof record.delta === "string") handlers.onText(record.delta);
				break;
			case "RUN_FINISHED":
				handlers.onDone(
					JSON.stringify(record.result ?? {}),
					toRunOutcome(record.outcome),
				);
				break;
			case "RUN_ERROR": {
				const codeRaw = record.code;
				const code =
					typeof codeRaw === "string" && /^-?\d+$/.test(codeRaw)
						? Number(codeRaw)
						: undefined;
				// 官方 `RunErrorEventSchema`（@ag-ui/core@0.0.58）只有
				// type/message/code/usage，沒有 `reason`——zod 的預設 `.object()`
				// 行為會把 `record`（=`parsed.data`）上任何 schema 沒宣告的欄位
				// 剝掉。但後端確實會在 wire JSON 裡多送一個 `reason`（procurement
				// 通用的 `{code,reason,message}` 業務錯誤契約），所以改從還沒被
				// schema 裁剪過的原始 `event` 讀，而不是從 `record` 讀。
				const reasonRaw =
					typeof event === "object" && event !== null
						? (event as Record<string, unknown>).reason
						: undefined;
				const reason = typeof reasonRaw === "string" ? reasonRaw : undefined;
				handlers.onError(
					typeof record.message === "string" && record.message
						? record.message
						: "AI 助手出錯",
					code,
					reason,
				);
				break;
			}
			case "TOOL_CALL_START":
				if (
					typeof record.toolCallId === "string" &&
					typeof record.toolCallName === "string"
				) {
					toolCallNames.set(record.toolCallId, record.toolCallName);
				}
				break;
			case "TOOL_CALL_RESULT": {
				const name =
					(typeof record.toolCallId === "string" &&
						toolCallNames.get(record.toolCallId)) ||
					"tool_result";
				handlers.onNamedEvent(
					name,
					typeof record.content === "string" ? record.content : "",
				);
				break;
			}
			case "CUSTOM":
				if (typeof record.name === "string") {
					handlers.onNamedEvent(
						record.name,
						JSON.stringify(record.value ?? null),
					);
				}
				break;
			default:
				break;
		}
	};
}
