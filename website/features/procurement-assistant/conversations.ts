import {
	HAWKR_AGENT_CATEGORY_IDS,
	type HawkrAgentCategoryId,
} from "@/lib/animation/hawkr-agent-season";
import { ApiClientError, apiClient } from "@/lib/api/client";
import type { RecordedField } from "./types";

/** B-244 Ask 1 的對話列表／transcript 契約（backend-payuni#2343）。
 * 端點尚未上線時（404/405）一律回空集合安靜降級——側欄不長出 agent
 * 區段、深連結走 RESTORE_FAILED，行為與今天一致；後端照 issue 規格
 * 補上後這裡零改動點亮。 */

/** 列表那一列。`primaryCategoryId` 是可空的後端分類真值；legacy、混合品項、
 * 尚未完成分類或未知 wire 值一律是 null，前端不從標題／預覽猜。 */
export interface AgentConversationSummary {
	conversationId: string;
	title: string;
	lastMessagePreview: string;
	updatedAt: string;
	primaryCategoryId: HawkrAgentCategoryId | null;
}

export interface AgentConversationPage {
	records: AgentConversationSummary[];
	hasMore: boolean;
	/** 不透明 cursor token（後端依 (updated_at, id) 排序，游標是複合值），
	 * 原樣回傳給下一次的 `cursor_id`——前端不解讀內容。 */
	nextCursorId: string | null;
	/** false = 端點尚未部署（404/405），呼叫端不應把它當錯誤呈現。 */
	available: boolean;
}

export interface AgentTranscriptMessage {
	/** 後端欄位名是 `id`（形如 `{revisionId}-buyer`／`-assistant`）。 */
	id: string;
	role: "buyer" | "assistant";
	text: string;
	/** assistant 訊息才有；後端 transcript 直接重播當輪已記下的欄位。 */
	recordedThisTurn?: RecordedField[];
	createdAt: string;
}

export interface AgentTranscript {
	conversationId: string;
	requestId: string;
	status: string;
	title: string;
	records: AgentTranscriptMessage[];
}

const CONVERSATIONS_PATH = "/buyers/me/procurement/conversations";
const RUNS_PATH = "/buyers/me/procurement/runs";

/** 後端 `maxConversationPageSize`；要更多靠 nextCursorId 續抓。 */
const MAX_PAGE_SIZE = 50;

const UNAVAILABLE_PAGE: AgentConversationPage = {
	records: [],
	hasMore: false,
	nextCursorId: null,
	available: false,
};

/**
 * 端點是否存在——一次頁面載入只探一次。後端 B-244 Ask 1 上線前每次
 * 掛載側欄都會打出一個 404，那是無意義的 console 噪音（404 不進 Sentry，
 * 見 lib/api/client.ts 的 `status < 500` 閘門，但仍會污染 devtools）。
 * 存 module scope 而非 localStorage：這是「端點在不在」不是帳號資料，
 * 且部署上線後只要重新載入頁面就會重新探測。
 */
let endpointAvailable: boolean | null = null;

function isEndpointMissing(err: unknown): boolean {
	return (
		err instanceof ApiClientError && (err.status === 404 || err.status === 405)
	);
}

function str(value: unknown): string {
	return typeof value === "string" ? value : "";
}

const HAWKR_AGENT_CATEGORY_ID_SET = new Set<string>(HAWKR_AGENT_CATEGORY_IDS);

export function normalizeHawkrAgentCategoryId(
	value: unknown,
): HawkrAgentCategoryId | null {
	if (typeof value !== "string") return null;
	const normalized = value.trim();
	return HAWKR_AGENT_CATEGORY_ID_SET.has(normalized)
		? (normalized as HawkrAgentCategoryId)
		: null;
}

function normalizeRecordedFields(raw: unknown): RecordedField[] | undefined {
	if (!Array.isArray(raw)) return undefined;
	const fields = raw.flatMap((item) => {
		if (!item || typeof item !== "object") return [];
		const r = item as Record<string, unknown>;
		const key = str(r.key);
		const value = str(r.value);
		if (!key || !value) return [];
		return [{ key, label: str(r.label) || key, value }];
	});
	return fields.length > 0 ? fields : undefined;
}

function normalizeSummary(raw: unknown): AgentConversationSummary | null {
	if (!raw || typeof raw !== "object") return null;
	const r = raw as Record<string, unknown>;
	const conversationId = str(r.conversationId);
	// 只有 conversationId 是必要的——它是那一列的身分與連結目標。列表刻意
	// 不回 requestId（續聊時才由 messages 端點給），所以不能拿它當必填，
	// 否則整份列表會被丟掉、側欄空白。
	if (!conversationId) return null;
	return {
		conversationId,
		title: str(r.title),
		lastMessagePreview: str(r.lastMessagePreview),
		updatedAt: str(r.updatedAt),
		primaryCategoryId: normalizeHawkrAgentCategoryId(r.primaryCategoryId),
	};
}

export async function fetchAgentConversations(params?: {
	cursorId?: string;
	pageSize?: number;
}): Promise<AgentConversationPage> {
	// 已知端點不存在就不再打——避免每次進訊息頁都留一個 404 在 console。
	if (endpointAvailable === false) return UNAVAILABLE_PAGE;
	// 房規：query 參數 snake_case（10 檔 §2）。`cursor_id` 這個名字沿用，但
	// 值已是不透明 token（後端 GetConversations 的註解說明語意變更）。
	// page_size 後端上限 50，要多的靠 cursor 續抓。
	const search = new URLSearchParams();
	if (params?.cursorId) search.set("cursor_id", params.cursorId);
	search.set("page_size", String(params?.pageSize ?? MAX_PAGE_SIZE));
	try {
		const env = await apiClient.authed.get<{
			records?: unknown[];
			hasMore?: boolean;
			nextCursorId?: string | null;
		}>(`${CONVERSATIONS_PATH}?${search.toString()}`);
		const records = (env.data.records ?? [])
			.map(normalizeSummary)
			.filter((item): item is AgentConversationSummary => item !== null);
		endpointAvailable = true;
		return {
			records,
			hasMore: Boolean(env.data.hasMore),
			nextCursorId: env.data.nextCursorId ?? null,
			available: true,
		};
	} catch (err) {
		if (isEndpointMissing(err)) {
			endpointAvailable = false;
			return UNAVAILABLE_PAGE;
		}
		// 其他錯誤（5xx／網路）不代表端點不存在，不記入 endpointAvailable，
		// 下次仍會重試。
		throw err;
	}
}

/** 測試用：重置端點探測結果。 */
export function resetAgentConversationsProbe(): void {
	endpointAvailable = null;
}

/**
 * 深連結續聊用的完整對話紀錄。
 *
 * 這支端點不分頁（後端 `GetConversationMessages` 不吃 cursor／page_size，
 * 一次回整份 revision 鏈），所以一個請求就是完整的，不需要追游標。
 */
export async function fetchAgentTranscript(
	conversationId: string,
): Promise<AgentTranscript> {
	const env = await apiClient.authed.get<{
		conversationId?: string;
		requestId?: string;
		status?: string;
		title?: string;
		records?: unknown[];
	}>(`${CONVERSATIONS_PATH}/${encodeURIComponent(conversationId)}/messages`);

	const records: AgentTranscriptMessage[] = [];
	for (const raw of env.data.records ?? []) {
		if (!raw || typeof raw !== "object") continue;
		const r = raw as Record<string, unknown>;
		records.push({
			// 後端每筆 revision 展成兩列，id 是 `{revisionId}-buyer|-assistant`。
			id: str(r.id) || crypto.randomUUID(),
			// role 是小寫（後端 MessageRecord 直接寫 "buyer"／"assistant"）；
			// 大小寫不敏感比對，免得任一邊改了大小寫就整份錯判成 assistant。
			role: str(r.role).toLowerCase() === "buyer" ? "buyer" : "assistant",
			text: str(r.text),
			recordedThisTurn: normalizeRecordedFields(r.recordedThisTurn),
			// records 也帶 steps 與推理過程，但思考過程不重現在歷史紀錄裡，
			// 所以兩者都不解析。
			createdAt: str(r.createdAt),
		});
	}
	return {
		conversationId: str(env.data.conversationId) || conversationId,
		requestId: str(env.data.requestId),
		status: str(env.data.status) || "CLARIFYING",
		title: str(env.data.title),
		records,
	};
}

export async function renameAgentConversation(
	conversationId: string,
	title: string,
): Promise<void> {
	await apiClient.authed.patch(
		`${CONVERSATIONS_PATH}/${encodeURIComponent(conversationId)}`,
		{ json: { title, clientRequestId: crypto.randomUUID() } },
	);
}

/** 軟刪。後端 DELETE 也吃 body 且 `clientRequestId` 必填（缺了會 400
 * "invalid request body"），跟 PATCH 同一套冪等鍵規則。 */
export async function deleteAgentConversation(
	conversationId: string,
): Promise<void> {
	await apiClient.authed.delete(
		`${CONVERSATIONS_PATH}/${encodeURIComponent(conversationId)}`,
		{ json: { clientRequestId: crypto.randomUUID() } },
	);
}

/** 取消命令的 request body（gateway `POST /runs/{runId}/cancel`）。 */
export interface ProcurementRunCancelCommand {
	runId: string;
	/**
	 * CAS token，必填。唯一的來源是 runtime-snapshot 的
	 * `runs[].runVersion`（`selectCancellableRun`）——這個模組不生、不猜、也不
	 * 給預設值，缺了就不該呼叫到這裡。
	 */
	expectedRunVersion: string;
	/** 買家沒有輸入理由的欄位，目前一律空字串；契約允許空值。 */
	reason?: string;
}

/**
 * 對一個執行中的 run 下取消命令。
 *
 * ⚠️ **回 200 不等於「已取消」。** `data.cancelDisposition` 有四個值，四個都是
 * 成功回應，其中三個代表這一輪早就到終點了（見
 * `runtime-snapshot-contract.ts` 的 `RUN_CANCEL_DISPOSITIONS`）。所以這裡只負責
 * 把 `data` 原樣交出去，由狀態擁有者用 `runCancelResultSchema` 解析並決定畫面
 * 要講哪一句——在這一層先翻成 boolean 會把那個分辨丟掉。
 *
 * CAS 沒過時後端回錯誤（409），那**不是**「取消失敗」，而是手上的版本過期；
 * 呼叫端該做的是重讀快照拿新版本再送一次，不是把它端到買家面前。
 */
export async function cancelProcurementRun(
	command: ProcurementRunCancelCommand,
): Promise<unknown> {
	const env = await apiClient.authed.post<unknown>(
		`${RUNS_PATH}/${encodeURIComponent(command.runId)}/cancel`,
		{
			json: {
				expectedRunVersion: command.expectedRunVersion,
				reason: command.reason ?? "",
			},
		},
	);
	return env.data;
}
