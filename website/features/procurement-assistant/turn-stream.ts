/**
 * `POST /app/buyers/me/procurement/turns/stream` 的客戶端（後端
 * `PostTurnStream`）。跟同步版 `/turns` 同一個輸入契約，差別是邊執行邊推
 * 事件——所以思考步驟是**當下**發生的事實，不是事後回放。
 *
 * Wire 是 AG-UI 協定（`lib/ai-chat/stream-client.ts` 負責翻譯）：
 *   CUSTOM step_detail  {turnId, seq, code, status, label, text?, durationMs?}
 *   CUSTOM activity      {rowId, code, text} — graph 路徑專用，entry（text=""）
 *                        →completion（text 非空）依 rowId 收斂，跟 step_detail
 *                        互斥（一輪只會走其中一條路徑）；見 mergeLiveActivity。
 *   RUN_FINISHED.result 完整 TurnResult（與 /turns 的 data 同形狀）
 *   RUN_ERROR           {code, message, reason?}
 *
 * 每個階段前後各一個 step（RUNNING → SUCCEEDED），Compose 等待期間還會每隔一段
 * 時間重發同 code 的 RUNNING 當 heartbeat。所以消費端必須**按 code 收斂**：
 * 見 mergeLiveStep。
 */

import {
	AiStreamHttpError,
	AiStreamTimeoutError,
	type RunOutcome,
	streamAiChat,
} from "@/lib/ai-chat/stream-client";
import { parseJsonSafe } from "@/lib/api/json";
import { getEnv } from "@/lib/config/env";
import {
	type AgentActivityEvent,
	mergeLiveActivity,
	parseActivityEventJson,
} from "./activity";
import {
	type AgentStep,
	isStepSucceeded,
	normalizeSteps,
	stepLabel,
} from "./steps";

const STREAM_ENDPOINT = "/app/buyers/me/procurement/turns/stream";

/**
 * 降級是靜默的（使用者只會看到中性三點），所以非 prod 環境留一行紀錄——
 * 否則「思考過程沒出現」只能靠猜是前端沒播、還是串流根本沒建立起來。
 *
 * 特別值得記的一種：gateway 的 SSE proxy 無條件寫
 * `Content-Type: text/event-stream`，但把 ai-service 的原始 status 照抄回來
 * （backend `proxyAIAssistantSSE`）。所以 ai-service 那支 handler 不在或出錯時，
 * 回應「看起來像 SSE」其實是 404/500——這裡的狀態碼就是唯一的線索。
 */
function logFallback(reason: string, status?: number) {
	if (!getEnv().enableApiLogging) return;
	console.debug(
		"[procurement:stream] 降級走同步 /turns：",
		reason,
		status === undefined ? "" : `HTTP ${status}`,
	);
}

/**
 * 每個事件抵達的時刻（相對於送出）。`logFallback` 只回答「串流有沒有建立
 * 起來」，回答不了「事件是一路推過來的、還是整包擠在最後一刻才到」——而那
 * 個差別正好就是「跑條有沒有東西可播」的差別：整包才到的話，四個 step 會
 * 在 RUN_FINISHED 前幾毫秒一起進來，畫面上等同從頭到尾只有三點。
 *
 * 中間任何一層（gateway、反向代理、CDN、dev proxy）只要緩衝了 response
 * body，症狀就是這樣，而且不會有任何錯誤可以攔。非 prod 才記。
 */
function logStreamEvent(name: string, elapsedMs: number) {
	if (!getEnv().enableApiLogging) return;
	console.debug(`[procurement:stream] +${Math.round(elapsedMs)}ms ${name}`);
}

/** 這個環境的 gateway 有沒有串流端點。undefined＝還沒試過；false＝試過而
 * 端點不存在（404/405），之後直接走同步版，不每輪重試一次浪費一個 round
 * trip（與 conversations.ts 的 probe-once 慣例同源）。 */
let streamEndpointAvailable: boolean | undefined;

/** 測試用：重置 probe 狀態。 */
export function resetStreamProbe() {
	streamEndpointAvailable = undefined;
}

export function isStreamKnownUnavailable(): boolean {
	return streamEndpointAvailable === false;
}

export interface TurnStreamHandlers {
	/**
	 * 收到第一個通過 allowlist 驗證的 public step。這只能證明伺服器已開始輸出
	 * 這輪的公開進度，**不是** v6 durable `RUN_ACCEPTED`；目前 POST stream 契約
	 * 沒有 command ledger ACK，呼叫端不得把這個回呼寫成持久化承諾。
	 */
	onStartObserved?: (observedAt: number) => void;
	/** mergeLiveStep 判定不會改變 UI 時通知一次；呼叫端只累加，terminal 才送 aggregate。 */
	onDuplicateStepSuppressed?: () => void;
	/** 合法 RUN_FINISHED/result 或 terminal error 實際抵達的 monotonic timestamp。 */
	onTerminalObserved?: (observation: {
		observedAt: number;
		duplicateStepSuppressedCount: number;
	}) => void;
	/** 收到一個 step 事件（已按 code 收斂成「目前這一步」）。 */
	onStep: (step: AgentStep) => void;
	/** 收到一個 graph 路徑的 Activity 事件（entry 或 completion，依 rowId 決定
	 * 併到哪一列）；只有走 graph controller 的 turn 才會有這個事件，跟 onStep
	 * 互斥——一輪只會有其中一種事件。 */
	onActivity?: (event: AgentActivityEvent) => void;
}

/** 串流本身失敗（而不是這一輪失敗）時丟出，讓呼叫端決定要不要降級。 */
export class TurnStreamUnavailableError extends Error {
	constructor(
		readonly reason: "NOT_DEPLOYED" | "BUSY",
		message: string,
	) {
		super(message);
		this.name = "TurnStreamUnavailableError";
	}
}

/** AG-UI `RUN_ERROR`——這一輪真的失敗了，降級重送也不會變好。
 * message 沿用後端字串（已是給使用者看的中文，不含內部細節）。
 *
 * `reason` 是 procurement 的 `{code,reason,message}` 業務錯誤契約裡的機器碼
 * （用法對齊 `ApiClientError.reason`），預設空字串。兩個建構點現在都能給出
 * 真正的值：in-band AG-UI `RUN_ERROR`（`lib/ai-chat/stream-client.ts` 從還沒被
 * 官方 zod schema 裁剪過的原始 wire JSON 另外補讀出 `reason`，因為
 * `RunErrorEventSchema` 本身沒有這個欄位）；串流開場前的 HTTP 409
 * （`AiStreamHttpError`）現在也會解析 JSON body 的 `reason`。兩條路徑都缺值時
 * 才落到空字串，不是猜測值。這讓 `useProcurementSession.ts` 能跟
 * `ApiClientError` 分支一樣寫 `err.reason === "APPROVAL_PENDING"`，不必為兩種
 * transport 各寫一套判斷式。 */
export class TurnStreamEventError extends Error {
	constructor(
		readonly code: number,
		message: string,
		readonly reason: string = "",
	) {
		super(message);
		this.name = "TurnStreamEventError";
	}
}

/**
 * 把一個 step 事件併進目前的步驟串。
 *
 * 同一個 code 的 RUNNING → SUCCEEDED 都指向**同一步**，所以以 code 為 key
 * 覆寫既有步驟，而不是無腦 append（否則重試或重複事件會長成多行、畫面反覆
 * 跳動）。SUCCEEDED 帶著實測 durationMs／StepNarrator text，覆寫時要保留。
 *
 * 註：後端雖已移除心跳幀（composeWithHeartbeat），此處邏輯仍保留作為一般重複
 * step 事件的防禦性兜底，防止後續重複送達的 RUNNING 把已完成步驟打回進行中或清掉耗時。
 */
export function mergeLiveStep(
	steps: AgentStep[],
	next: AgentStep,
): AgentStep[] {
	const idx = steps.findIndex((s) => s.code === next.code);
	if (idx === -1) return [...steps, next];
	const prev = steps[idx];
	const staleRunning =
		isStepSucceeded(prev.status) && next.status === "RUNNING";
	const merged: AgentStep = {
		...prev,
		...next,
		// 重複或延遲送達的 RUNNING 不得把已完成的那步打回進行中，也不得清掉耗時。
		// 用 isStepSucceeded 而非 `=== `：改名窗口內同一條 stream 的前後兩個事件
		// 可能來自不同版本的 backend pod，新舊兩個成功值都要當成已完成。
		status: isStepSucceeded(prev.status) ? prev.status : next.status,
		durationMs: staleRunning
			? prev.durationMs
			: (next.durationMs ?? prev.durationMs),
		// 同 status 的後續事件仍可能補上 StepNarrator 文字／safeArgs，必須讓 rail
		// 更新；但已完成後遲到的 RUNNING heartbeat 不能覆蓋完成內容。
		text: staleRunning ? prev.text : (next.text ?? prev.text),
		safeArgs: staleRunning ? prev.safeArgs : (next.safeArgs ?? prev.safeArgs),
	};
	if (
		merged.status === prev.status &&
		merged.durationMs === prev.durationMs &&
		stepLabel(merged) === stepLabel(prev)
	)
		return steps;
	const out = [...steps];
	out[idx] = merged;
	return out;
}

function monotonicNow(): number {
	return typeof performance === "undefined" ? Date.now() : performance.now();
}

function parseStepEvent(raw: string): AgentStep | null {
	let parsed: unknown;
	try {
		parsed = parseJsonSafe(raw);
	} catch {
		return null;
	}
	// 單筆走 normalizeSteps 同一套 deny-by-default 收斂規則；未知 code、
	// wire label 與未知 status 都丟棄，不讓 private payload 穿進 Activity Rail。
	return normalizeSteps([parsed])[0] ?? null;
}

/**
 * 送出一輪並即時消費 SSE 進度事件，resolve 於 AG-UI
 * `RUN_FINISHED.result` 的 TurnResult。
 *
 * - 端點不存在（404/405）或併發已滿（429）→ 丟 TurnStreamUnavailableError，
 *   呼叫端用**同一個 clientRequestId** 改打同步版即可（後端冪等會 replay 出
 *   同一份結果，包含 steps，不會產生第二筆 revision）。
 * - AG-UI `RUN_ERROR` → 丟 TurnStreamEventError（這一輪失敗，別降級重送）。
 * - stall 逾時 → 透傳 AiStreamTimeoutError。
 */
export async function streamProcurementTurn(
	body: {
		requestId?: string;
		message: string;
		clientRequestId: string;
	},
	handlers: TurnStreamHandlers,
	signal?: AbortSignal,
): Promise<unknown> {
	if (streamEndpointAvailable === false) {
		logFallback("這個環境先前已探測到端點不存在");
		throw new TurnStreamUnavailableError(
			"NOT_DEPLOYED",
			"streaming endpoint unavailable",
		);
	}

	// RUN_FINISHED.result 在 onDone 回呼裡抵達，但呼叫端要的是「這一輪的
	// 結果」——用 holder 物件而不是裸 let，因為 TS 的控制流分析看不到回呼有沒有
	// 跑，裸 let 讀出來會被窄化成 null。
	//
	// `outcome` 必須一起保留：wire 上 interrupt **只存在於 RUN_FINISHED.outcome**
	// （`{type:"interrupt",interrupts:[{id,reason}]}`，AG-UI 官方 schema，
	// doc57 §9.3），`result` 裡沒有 interruptId／interruptKind。unary
	// SubmitTurn 才有那兩個欄位（gateway `procurement_response_mapping.go:206`）。
	// 兩條路徑形狀不同，統一成 unary 那一份是這一層轉接器的職責——見回傳前的
	// 正規化。
	const landed: { result: unknown; outcome: RunOutcome | undefined } = {
		result: undefined,
		outcome: undefined,
	};
	let eventError: TurnStreamEventError | null = null;
	let sawAnyEvent = false;
	let startObserved = false;
	let observedSteps: AgentStep[] = [];
	let duplicateStepSuppressedCount = 0;
	let doneObservedAt: number | null = null;
	let errorObservedAt: number | null = null;
	let terminalReported = false;
	const startedAt = monotonicNow();
	const elapsed = () => monotonicNow() - startedAt;
	const reportTerminal = (observedAt: number) => {
		if (terminalReported) return;
		terminalReported = true;
		handlers.onTerminalObserved?.({
			observedAt,
			duplicateStepSuppressedCount,
		});
	};

	try {
		await streamAiChat(
			{ endpoint: STREAM_ENDPOINT, body, signal },
			{
				// 這個端點不推 token 流（回覆在 RUN_FINISHED.result 一次
				// 性落地），收到
				// 就當沒事——不把 reply 拆兩條路徑進畫面。
				onText: () => {},
				onNamedEvent: (eventName, rawData) => {
					if (eventName === "activity") {
						const activity = parseActivityEventJson(rawData);
						if (!activity) return;
						const observedAt = monotonicNow();
						sawAnyEvent = true;
						streamEndpointAvailable = true;
						if (!startObserved) {
							startObserved = true;
							handlers.onStartObserved?.(observedAt);
						}
						logStreamEvent(
							`activity ${activity.code} ${activity.text ? "completed" : "entry"}`,
							elapsed(),
						);
						// 刻意不做 step_detail 分支那種「mergedSteps === observedSteps
						// 才算沒變化」的 dedup：這裡無條件呼叫 onActivity，重複的 rowId
						// replay 事件靠下游 mergeLiveActivity 的冪等性收斂，不在這一層擋。
						handlers.onActivity?.(activity);
						return;
					}
					if (eventName !== "step_detail") return;
					const step = parseStepEvent(rawData);
					if (!step) return;
					const observedAt = monotonicNow();
					sawAnyEvent = true;
					streamEndpointAvailable = true;
					if (!startObserved) {
						startObserved = true;
						handlers.onStartObserved?.(observedAt);
					}
					logStreamEvent(`step ${step.code} ${step.status}`, elapsed());
					const mergedSteps = mergeLiveStep(observedSteps, step);
					if (mergedSteps === observedSteps) {
						duplicateStepSuppressedCount += 1;
						handlers.onDuplicateStepSuppressed?.();
						return;
					}
					observedSteps = mergedSteps;
					handlers.onStep(step);
				},
				onDone: (rawData, outcome) => {
					doneObservedAt ??= monotonicNow();
					sawAnyEvent = true;
					streamEndpointAvailable = true;
					logStreamEvent("RUN_FINISHED", elapsed());
					try {
						landed.result = parseJsonSafe(rawData);
					} catch {
						// 壞掉的 RUN_FINISHED.result 當成沒收到結果——下面會
						// 走降級路徑，
						// 用同一個 clientRequestId 把結果從同步端點撈回來。
					}
					landed.outcome = outcome;
				},
				onError: (message, code, reason) => {
					errorObservedAt ??= monotonicNow();
					// AG-UI RUN_ERROR 若缺 code 就保守落到 500，不從 message 猜碼。
					eventError = new TurnStreamEventError(
						code ?? 500,
						message,
						reason ?? "",
					);
				},
			},
		);
	} catch (err) {
		if (err instanceof AiStreamHttpError) {
			// 409 是 operation identity 衝突，不是 transport unavailable。若把它
			// 當一般串流失敗降級，呼叫端雖然仍會沿用同一 key，卻會再送一次已知
			// 衝突的 command；更糟的舊呼叫端可能換 key。收斂成和 AG-UI
			// `RUN_ERROR` 相同的 terminal turn error，讓 UI fail closed 並保留原 prompt。
			if (err.status === 409) {
				reportTerminal(monotonicNow());
				// 用後端自己的 message／reason，不再硬編一句只對「身分衝突」這個
				// case 講得通的前端文案——這個 409 也可能是 APPROVAL_PENDING
				// （request 卡在等買家解掉某個 interrupt），用錯文案會誤導使用者。
				// useProcurementSession.ts 的 submitTurn 對非 APPROVAL_PENDING 的
				// 409 仍會覆寫成固定的 TURN_IDEMPOTENCY_CONFLICT 文案，所以這裡
				// 一律透傳真值是安全的。
				throw new TurnStreamEventError(409, err.message, err.reason);
			}
			if (err.status === 404 || err.status === 405) {
				streamEndpointAvailable = false;
				logFallback("gateway／ai-service 沒有這支端點", err.status);
				throw new TurnStreamUnavailableError("NOT_DEPLOYED", err.message);
			}
			if (err.status === 429) {
				logFallback("併發串流已達上限", err.status);
				throw new TurnStreamUnavailableError("BUSY", err.message);
			}
			// 其餘狀態碼（500／502／400…）不是「端點不存在」，不記入
			// probe，下一輪仍會再試一次串流。
			logFallback("串流開場失敗", err.status);
			throw err;
		}
		logFallback(err instanceof Error ? err.name : "未知錯誤");
		throw err;
	}

	if (eventError) {
		reportTerminal(errorObservedAt ?? monotonicNow());
		throw eventError;
	}
	// outcome.type==="interrupt" 是合法的終態（等待買家輸入），不是「沒收到
	// 結果」——但不再需要在這裡另外短路：interruptId／interruptKind 已經
	// 直接內嵌在 landed.result 這份 TurnResult 本身（unary SubmitTurn 與串流
	// 終態同形狀，session-logic.ts 的 normalizeProcurementTurnResult 負責
	// 解析），所以 interrupt 這個終態就跟成功終態走同一條回傳路徑，由呼叫端
	// （useProcurementSession.ts 的 applyTurnResult）從結果裡的欄位判斷要不要
	// 進 pendingInterrupt，不必在這一層另外丟一個 signal 把整份結果丟棄。
	// 串流開了、也沒有 RUN_ERROR，卻沒拿到 RUN_FINISHED.result
	// ——連線在中途斷了。
	// 這時無法從 browser 證明 operation 是否已落地；只能用**同一個**
	// clientRequestId 查回／重放結果，絕對不能換 key 猜一輪新的 operation。
	if (!sawAnyEvent || !landed.result || typeof landed.result !== "object") {
		logFallback(
			sawAnyEvent
				? "串流中途斷線，沒收到 RUN_FINISHED.result"
				: "串流開了但沒有任何事件",
		);
		throw new TurnStreamUnavailableError(
			"BUSY",
			"stream closed without a result",
		);
	}
	reportTerminal(doneObservedAt ?? monotonicNow());

	// 把串流的形狀正規化成 unary SubmitTurn 的形狀，讓上層
	// （useProcurementSession 的 applyTurnResult）只需要認得一種 TurnResult。
	//
	// ⚠️ session-logic.ts:249 有成對不變式——interruptId 與 interruptKind 必須
	// 同時出現，否則 normalizeProcurementTurnResult 會整份回 null。所以這裡兩個
	// 一起寫，不可只補其中一個。
	//
	// 2026-09-03 修：先前這裡直接 `return landed.result`，而 turn-stream 與其
	// 單元測試都寫著「interruptId 已經內嵌在 result 裡」——實測 dev 證明那是錯的
	// （result 的 17 個鍵沒有它）。造成 pendingInterrupt 永遠設不起來、前端退回
	// 打同步 /crop-disambiguation，被後端 GuardNoOpenInterrupt 擋成 409
	// APPROVAL_PENDING。T12 graph cutover 讓每條路徑都真的建立 interrupt row
	// 之後才引爆；當時的測試自己在 result 裡塞了那兩個欄位，所以是綠的。
	if (
		landed.outcome?.type === "interrupt" &&
		landed.result &&
		typeof landed.result === "object"
	) {
		return {
			...(landed.result as Record<string, unknown>),
			interruptId: landed.outcome.interruptId,
			interruptKind: landed.outcome.reason,
		};
	}
	return landed.result;
}

// 併列邏輯已移到 activity.ts（兩條 transport 共用）。這裡再匯出一次，讓
// useProcurementSession.ts 與 turn-stream.unit.test.ts 的既有 import 不動。
export { AiStreamTimeoutError, mergeLiveActivity };
