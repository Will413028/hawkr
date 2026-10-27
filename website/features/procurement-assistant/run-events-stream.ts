/**
 * `GET /app/buyers/me/procurement/runs/{runId}/events` 的客戶端（gateway
 * `ProxyProcurementRunEvents` → ai-service `WatchRunEvents`）。
 *
 * 這是候選搜尋那四列**唯一**的路：候選搜尋是脫離 graph 的背景 goroutine，買家
 * 端沒有任何開著的連線可以夾帶 frame，所以 T13 子專案 1 那招（同一份 payload
 * 也往 POST turn stream 送一份）在這裡不成立。
 *
 * 建在 lib/sse/client.ts 而不是 lib/ai-chat/stream-client.ts：這支端點是 GET，
 * 且以 `Last-Event-ID`（十進位 ledger seq）當 replay cursor，而 stream-client
 * 是寫死 POST、沒有 cursor 也沒有重連的一次性傳輸。SseClient 另外帶來三件這裡
 * 真的需要的事：重連時自動帶上最新 cursor、heartbeat watchdog（gateway 每 20s
 * 送 `: ping`，client 門檻 35s），以及 **429/503 直接停止重試**——prod 的
 * `PROCUREMENT_REPLAY_SSE_ENABLED` 還關著，固定回 503，重試只是白打。
 *
 * Wire 是 AG-UI，frame 沒有 `event:` 行、只有 `id:` 與 `data:`（doc57 §2.4：
 * `id:` 是 replay 專屬的 cursor，POST 路徑沒有）。所以 SseClient 會把事件名
 * 報成 "message"，分派一律看 AG-UI 的 `type` 欄位。
 *
 * ⚠️ **伺服器不會在 run 終態時關閉連線**：ai-service 的 WatchRunEvents 會停在
 * 等待 wake 的迴圈裡，唯一會收掉這條連線的是 gateway 的 120 秒
 * `streamTurnDeadline`。呼叫端必須自己 close()。
 */

import { createAgUiEventDispatcher } from "@/lib/ai-chat/agui-dispatch";
import { parseJsonSafe } from "@/lib/api/json";
import { getEnv } from "@/lib/config/env";
import { SseClient } from "@/lib/sse/client";
import { type AgentActivityEvent, parseActivityEventJson } from "./activity";

/** 降級是靜默的（買家只會看到退回兩列 lane 的舊畫面），所以非 prod 留一行
 * 紀錄——否則「四列沒出現」只能靠猜是 flag 沒開、cursor 壞了、還是連線根本
 * 沒建立起來。同 turn-stream.ts 的 logFallback。 */
function logUnavailable(reason: string) {
	if (!getEnv().enableApiLogging) return;
	console.debug("[procurement:run-events] 訂閱結束，退回輪詢畫面：", reason);
}

/** conformance anchor 擋下來的 frame。**大聲記一筆但只丟掉這一個 frame**：
 * 這支端點重播的是整個 run 的 ledger，本來就會帶著本 client 不消費的型別
 * （`run_status`、`step_detail`、`artifact`…），而且一個 frame 不合契約不代表
 * 後面的 frame 也壞了——把任何 schema 拒絕都當成訂閱終結，等於讓後端多送一種
 * frame 就殺掉整條搜尋進度。真正該停止訂閱的只有 in-band `RUN_ERROR`。 */
function logInvalidFrame(message: string) {
	if (!getEnv().enableApiLogging) return;
	console.warn("[procurement:run-events] 丟棄不合契約的 frame：", message);
}

/** 一筆 typed artifact 的參照。**這是通知不是內容**——後端 ledger 事件刻意只帶
 * 三個參照（`repository/run.go` 的 `artifactUpsertedPayload`，doc 57 §4.2 event
 * data 欄明文：內容由 artifact 讀取端負責，兩邊各存一份會立刻分裂）。 */
export interface RunArtifactRef {
	artifactId: string;
	type: string;
	version: number;
}

/** fail-closed 同 `parseActivityEventJson`：少了 `artifactId` 就不是一個指得到
 * 東西的參照，寧可整筆不交出，也不要讓消費端拿著一個空 id 去讀。 */
function parseArtifactRefJson(raw: string): RunArtifactRef | null {
	let parsed: unknown;
	try {
		parsed = parseJsonSafe(raw);
	} catch {
		return null;
	}
	if (typeof parsed !== "object" || parsed === null) return null;
	const { artifactId, type, version } = parsed as Record<string, unknown>;
	if (typeof artifactId !== "string" || artifactId === "") return null;
	if (typeof type !== "string" || type === "") return null;
	if (typeof version !== "number" || !Number.isFinite(version)) return null;
	return { artifactId, type, version };
}

export interface RunActivityStreamHandlers {
	/** 收到一列 Activity（已通過 fail-closed 解析）。合併由呼叫端做。 */
	onActivity: (event: AgentActivityEvent) => void;
	/** 收到一筆 `ARTIFACT_UPSERTED`（gateway 投影成 CUSTOM `name="artifact"`）。
	 *
	 * ⚠️ 選填：這裡拿到的只是**參照**（artifactId／type／version），把它換成內容
	 * 要走 artifact 讀取端。
	 *
	 * ⚠️ **這段原本寫著「doc57 §5.6 的 runtime-snapshot 至今不存在」——那句已經
	 * 過期。** 端點已經上 dev（`GET .../conversations/{id}/runtime-snapshot`，
	 * gateway `ProxyProcurementConversationRuntimeSnapshot`），本 repo 的消費端是
	 * `useRfqRuntimeSnapshot`／`runtime-snapshot-contract.ts`，呈現在
	 * `QuoteBatchSummary` 的「結果版本」那一列。註解寫下當時是真的，之後沒人回頭
	 * 改；它跨 repo，backend 那次上線不會經過本 repo 的 review。
	 *
	 * 這裡仍先把 envelope 消費起來並釘住形狀，讓串流契約有一個真實讀過它的消費
	 * 端；golden fixture 只證明序列化，不證明 wire 可用。 */
	onArtifact?: (ref: RunArtifactRef) => void;
	/** 收到 AG-UI `RUN_FINISHED`。候選搜尋成功時後端寫的就是這個終態
	 * （`candidate_search.go` 的 `terminalEventType`）。
	 *
	 * ⚠️ **這段原本寫著「今天在 dev／prod 上這條路實際上走不到」——那句已經過期。**
	 * 缺 `threadId` 的缺陷已由後端修好（gateway 改以 `RunEvent.conversation_id`
	 * 投影成 `threadId`），dev 實測 frame 通過官方 schema。註解寫下當時是真的，
	 * 之後沒人回頭改；它跨 repo，backend 那次修正不會經過本 repo 的 review。
	 *
	 * 失敗終態不走這裡：後端寫的是 `RUN_FAILED`，gateway 投影成**原生
	 * `RUN_ERROR`**（不是 `CUSTOM run_status`——那也是一句已經過期的敘述），
	 * 會落到 `onUnavailable`。 */
	onTerminal: () => void;
	/** 這個 run 的訂閱結束且不會自行恢復：in-band `RUN_ERROR`（410 游標過期／
	 * 409 游標超前，兩者重連也只會拿到同一個拒絕）、401、503，或重連預算用盡。 */
	onUnavailable: (reason: string) => void;
	/** 這個 **run 自己失敗了**（後端 `RUN_FAILED` → 原生 AG-UI `RUN_ERROR`）。
	 *
	 * 與 `onUnavailable` 的差別是成因不是嚴重度：run 失敗代表它已經終態，重連只會
	 * replay 到同一個終態，所以一樣要收線——但呈現的是「搜尋失敗（原因）」而不是
	 * 「連線中斷」。`reason` 是 CodeMeta 的 `ProcurementRunFailureReason`，
	 * `message` 是後端給的可讀文案。 */
	onRunFailed?: (failure: RunFailure) => void;
}

/** 一次業務層的 run 失敗。 */
export interface RunFailure {
	reason: string;
	message: string;
}

export function openRunActivityStream(
	runId: string,
	handlers: RunActivityStreamHandlers,
): { close: () => void } {
	let finished = false;

	const dispatch = createAgUiEventDispatcher({
		// 這個端點不推 token 流。
		onText: () => {},
		onNamedEvent: (name, rawData) => {
			if (finished) return;
			if (name === "activity") {
				const event = parseActivityEventJson(rawData);
				if (event) handlers.onActivity(event);
				return;
			}
			if (name === "artifact") {
				const ref = parseArtifactRefJson(rawData);
				if (ref) handlers.onArtifact?.(ref);
				return;
			}
			// 其餘 CUSTOM name（`run_status`、`step_detail`、`confirmation`…）目前
			// 沒有消費端。⚠️ `run_status` **不是忘了接**：gateway 組 frame 時只帶
			// slug 與 producer 的 data_json（`ai_run_events_stream.go` 的
			// writeRunEvent），ledger event type 不在 frame 裡，所以 RUN_FAILED／
			// RUN_CANCELLED／RUN_ACCEPTED 等七種事件在這裡完全不可分辨——而候選
			// 搜尋的 payload 是字面 `{}`。要接失敗終態得先讓後端把事件型別放進
			// frame，不能在這裡猜。
		},
		onDone: () => {
			if (finished) return;
			handlers.onTerminal();
		},
		onError: (message, code, reason) => {
			// 判別靠**有沒有 `code`**，不是靠 `reason`：兩種 RUN_ERROR 都可能帶
			// `reason`（傳輸失敗那條的 reason 來自 gateway 的 BusinessError），但只有
			// 傳輸失敗會帶 `code`——gateway 對 410／409／503 一類送的是 HTTP 字串碼
			// （見 procurementErrorFrameCode 的呼叫點），而 RUN_FAILED 投影出來的
			// RUN_ERROR 只帶 message ＋ reason。
			//
			// 這條界線是後端刻意留的：機器碼一律走 `reason`（`code` 被本 repo 以
			// /^-?\d+$/ 過濾），於是 `code` 就只剩傳輸層在用，正好成為判別欄位。
			if (code === undefined && reason !== undefined && reason !== "") {
				if (!finished) {
					finished = true;
					client.close();
					handlers.onRunFailed?.({ reason, message });
				}
				return;
			}
			const parts = [
				code === undefined ? "" : String(code),
				reason ?? "",
				message,
			].filter((part) => part !== "");
			fail(parts.join(" "));
		},
		// schema 拒絕不終止訂閱，只丟這個 frame（見 logInvalidFrame）。
		onInvalidEvent: (message) => {
			if (finished) return;
			logInvalidFrame(message);
		},
	});

	const client = new SseClient({
		path: `/buyers/me/procurement/runs/${runId}/events`,
		mode: "authed",
		handlers: {
			// AG-UI frame 沒有 `event:` 行，SseClient 一律報成 "message"；
			// 分派看的是 AG-UI 的 `type`，所以名字這裡用不到。
			onEvent: (_eventName, data) => {
				if (!finished) dispatch(data);
			},
			// 這個端點沒有 `ready` 信封（那是留言／通知那條 topic 串流才有的
			// 東西）。傳一個 no-op 而不是把共用型別改成選填——一個呼叫端不夠
			// 構成放寬共用契約的理由。
			onReady: () => {},
			onStatusChange: (status) => {
				if (status === "unavailable") fail("stream unavailable");
			},
		},
	});

	function fail(reason: string) {
		if (finished) return;
		finished = true;
		logUnavailable(reason);
		client.close();
		handlers.onUnavailable(reason);
	}

	client.open();

	return {
		close: () => {
			finished = true;
			client.close();
		},
	};
}
