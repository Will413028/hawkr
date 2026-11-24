import type {
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementSlots,
	ProcurementSnapshot,
} from "@/features/procurement-assistant/types";
import type { ClarifyQuestion, ProcurementStage } from "./types";

export interface TurnSubmissionIdentity {
	clientRequestId: string;
	message: string;
}

/**
 * 產生（或重用）一輪自由輸入的冪等身分。
 *
 * 同一段 trim 後文字代表使用者在重試同一個 operation，必須沿用原 key；只要
 * 使用者改了文字，就必須建立新 key，避免把不同 payload 塞進舊 idempotency
 * record 後撞 409。`createId` 由呼叫端注入，讓這條交易規則可直接單元測試。
 */
export function prepareTurnSubmission(
	previous: TurnSubmissionIdentity | null,
	message: string,
	createId: () => string,
): TurnSubmissionIdentity {
	const normalizedMessage = message.trim();
	if (previous?.message === normalizedMessage) return previous;
	return {
		clientRequestId: createId(),
		message: normalizedMessage,
	};
}

/**
 * Handoff 草稿只在桌面寬度主動聚焦。手機開頁時叫出軟鍵盤會遮住內容並造成版面
 * 跳動；使用者點 composer 時再聚焦即可。matchMedia 由呼叫端傳入，SSR／測試沒有
 * browser API 時安全回 false。
 */
export function shouldAutofocusHandoff(
	matchMedia:
		| ((query: string) => Pick<MediaQueryList, "matches">)
		| null
		| undefined,
): boolean {
	return matchMedia?.("(min-width: 768px)").matches === true;
}

export interface AgentEntrySignals {
	/** `demo` 等具名相容路由；有值就不是正式新對話入口。 */
	conversationId?: string | null;
	initialRequestId?: string | null;
	initialConversationId?: string | null;
	sessionConversationId?: string | null;
	restoredSummary?: string | null;
	turnCount: number;
	stage: ProcurementStage;
}

/**
 * 只有正式 procurement 新對話、而且 session 完全沒有歷史時才顯示 Entry。
 * 抽成純函式，避免 demo/deep-link/reload 的路由條件藏在 JSX 裡而回歸。
 */
export function shouldShowAgentEntry({
	conversationId,
	initialRequestId,
	initialConversationId,
	sessionConversationId,
	restoredSummary,
	turnCount,
	stage,
}: AgentEntrySignals): boolean {
	return (
		!conversationId &&
		!initialRequestId &&
		!initialConversationId &&
		!sessionConversationId &&
		!restoredSummary &&
		turnCount === 0 &&
		stage === "clarifying"
	);
}

const SLOT_LABELS: Record<keyof ProcurementSlots, string> = {
	item: "品項",
	// 逐字對齊後端 domain.SlotLabels 的同名項——recordedThisTurn 的 chip label 由
	// 後端送，這裡是 missingRequired／rejected 走 slotLabel() 時的同一份文案，
	// 兩邊不一致的話同一個欄位在畫面上會有兩個名字。
	itemResolution: "品項分類",
	quantity: "數量",
	unit: "單位",
	packaging: "包裝",
	budgetPerUnit: "單價預算",
	deliveryCity: "配送城市",
	deliveryDate: "到貨日期",
	exactMatchRequired: "是否需完全相符",
};

/** missingRequired／rejected[].key 帶的是後端 domain.SlotKey（snake_case）；
 *  slots 物件本身（經 mapSlots 轉換後）用 camelCase。slotLabel 兩種拼法都要
 *  認得到，否則後端原始欄位識別字會直接洩漏到使用者看得到的文字。 */
const SNAKE_CASE_ALIASES: Record<string, keyof ProcurementSlots> = {
	budget_per_unit: "budgetPerUnit",
	delivery_city: "deliveryCity",
	delivery_date: "deliveryDate",
	exact_match_required: "exactMatchRequired",
};

export function slotLabel(key: string): string {
	const camelKey = (SNAKE_CASE_ALIASES[key] ?? key) as keyof ProcurementSlots;
	return SLOT_LABELS[camelKey] ?? key;
}

/**
 * 「這個 session 現在有沒有可看的成果」。⛔ 這是唯一判準：面板的掛載條件與
 * 「看得到／打得開」的條件必須是同一個函式。
 *
 * 兩個來源不對稱是刻意的：candidateSet 只在 session 內的候選輪詢期間存在
 * （stage 會是 "candidates"），而 publishedPublication 來自 GetSnapshot，重整後
 * 仍在——此時 deriveStageFromStatus 只會回 "clarifying"／"ready"（見其註解，那是
 * 刻意的），所以綁 stage 會讓已發布的成果在重整後永遠打不開。
 */
export function hasProcurementResults(state: {
	stage: ProcurementStage;
	candidateSet: unknown | null;
	publishedPublication: unknown | null;
}): boolean {
	return Boolean(
		(state.stage === "candidates" && state.candidateSet) ||
			state.publishedPublication,
	);
}

/** 需求狀態是後端權威：CLARIFYING 以外一律落到 "ready"，「開始搜尋候選」按鈕是 MATCHING 的復原路徑。 */
export function deriveStageFromStatus(
	status: ProcurementSnapshot["status"],
): ProcurementStage {
	return status === "CLARIFYING" ? "clarifying" : "ready";
}

/**
 * 候選搜尋的 run 是否已進終態。這是輪詢的唯一終止條件（每 1.5s 讀一次），
 * 漏判一個終態值就是無限輪詢、畫面永遠停在「正在搜尋候選」，而且沒有任何錯誤訊號。
 */
export function isTerminalCandidateStatus(
	status: ProcurementCandidateSearch["status"],
): boolean {
	return (
		status === "SUCCEEDED" || status === "FAILED" || status === "CANCELLED"
	);
}

/** 樂觀翻轉：回傳一份新陣列，只有目標 candidateId 的 selected 被改動，其餘
 * 物件參照不變。找不到目標 id 時整批原樣回傳。 */
export function applyOptimisticSelection(
	candidates: ProcurementCandidate[],
	candidateId: string,
	selected: boolean,
): ProcurementCandidate[] {
	return candidates.map((candidate) =>
		candidate.candidateId === candidateId
			? { ...candidate, selected }
			: candidate,
	);
}

/** 從候選清單（platform/external 兩個陣列）derive 出目前勾選的 candidateId 清單，
 * 跟 CandidateBrowser.tsx 既有的 selectedCount 用同一個 filter 邏輯，只是這裡回傳 id 陣列。 */
export function deriveSelectedCandidateIds(candidates: {
	platform: ProcurementCandidate[];
	external: ProcurementCandidate[];
}): string[] {
	return [...candidates.platform, ...candidates.external]
		.filter((c) => c.selected)
		.map((c) => c.candidateId);
}

/** quantity 複合問題卡／單位追問卡是否帶了單位選項（backend-payuni#2442 §4）。
 * ClarifyQuestionCard 的型態判斷與 AssistantWorkspace 的 composer 例外路由都要
 * 問同一件事，這裡是唯一權威來源——避免兩處各自複製一份判斷式而漂移。 */
export function questionHasUnitOptions(
	question: Pick<ClarifyQuestion, "unitOptions"> | null | undefined,
): boolean {
	return Boolean(question?.unitOptions?.length);
}
