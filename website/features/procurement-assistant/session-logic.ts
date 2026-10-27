import type {
	ProcurementCandidate,
	ProcurementCandidateSearch,
	ProcurementItemResolution,
	ProcurementSlots,
	ProcurementSnapshot,
} from "@/lib/api/openapi-types";
import type { PublicNarrativeBlock, PublicNarrativeKind } from "./narrative";
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

/** 後端 domain.Slots 的 wire 格式——見 lib/api/openapi-types.ts 的
 * ProcurementSlots 型別註解，這是這組端點唯一 snake_case 的地方。 */
export interface RawProcurementSlots {
	item?: string;
	quantity?: number;
	unit?: string;
	packaging?: string;
	budget_per_unit?: number;
	delivery_city?: string;
	delivery_date?: string;
	exact_match_required?: boolean;
	item_resolution?: RawProcurementItemResolution;
}

/** 後端 domain.ItemResolution 的 wire 格式（同樣 snake_case，見上方註解）。 */
export interface RawProcurementItemResolution {
	kind?: string;
	code?: string;
	label?: string;
	provenance?: string;
	price_per_kg?: number;
	display_value?: string;
}

const ITEM_RESOLUTION_KINDS = ["CROP", "GROUP", "SELF_PRICED"] as const;
const ITEM_RESOLUTION_PROVENANCES = ["SYSTEM", "BUYER_CHOICE"] as const;

/** 解析 item_resolution。回傳 undefined 有兩種情況，呼叫端不需要分辨：欄位不在，
 *  或內容不符合契約。
 *
 *  **不合契約時丟掉這個欄位、不讓整包 slots 失敗**——跟本檔其他欄位的嚴格處理
 *  刻意不同：它們是需求內容（數量錯了就不能拿去報價），這個是顯示用的解析結果，
 *  為了它讓整個 session 快照解析失敗，代價遠大於少顯示一行分類。
 *
 *  kind／provenance 用白名單而不是 `as` 斷言：後端日後新增第三種 provenance 時，
 *  這裡會安靜地退成 undefined（少一行提示），而不是讓一個 TypeScript 型別上不
 *  存在的值流進 switch 造成 UI 走到沒定義的分支。 */
function normalizeItemResolution(
	raw: unknown,
): ProcurementItemResolution | undefined {
	if (!isRecord(raw)) return undefined;
	const kind = ITEM_RESOLUTION_KINDS.find((k) => k === raw.kind);
	const provenance = ITEM_RESOLUTION_PROVENANCES.find(
		(p) => p === raw.provenance,
	);
	if (!kind || !provenance) return undefined;
	if (typeof raw.display_value !== "string" || raw.display_value === "") {
		// display_value 是這個型別存在的理由（顯示字串由伺服器組）。缺了它就沒有
		// 東西可顯示，其餘欄位單獨留著沒有意義。
		return undefined;
	}
	const optionalString = (v: unknown): string | undefined =>
		typeof v === "string" && v !== "" ? v : undefined;
	const pricePerKg =
		typeof raw.price_per_kg === "number" && Number.isFinite(raw.price_per_kg)
			? raw.price_per_kg
			: undefined;
	return {
		kind,
		provenance,
		displayValue: raw.display_value,
		code: optionalString(raw.code),
		label: optionalString(raw.label),
		pricePerKg,
	};
}

function isRecord(raw: unknown): raw is Record<string, unknown> {
	return Boolean(raw) && typeof raw === "object" && !Array.isArray(raw);
}

export function normalizeRawProcurementSlots(
	raw: unknown,
): RawProcurementSlots | null {
	if (!isRecord(raw)) return null;
	const stringKeys = [
		"item",
		"unit",
		"packaging",
		"delivery_city",
		"delivery_date",
	] as const;
	for (const key of stringKeys) {
		if (raw[key] !== undefined && typeof raw[key] !== "string") return null;
	}
	const numberKeys = ["quantity", "budget_per_unit"] as const;
	for (const key of numberKeys) {
		if (
			raw[key] !== undefined &&
			(typeof raw[key] !== "number" || !Number.isFinite(raw[key]))
		)
			return null;
	}
	if (
		raw.exact_match_required !== undefined &&
		typeof raw.exact_match_required !== "boolean"
	)
		return null;
	return {
		item: raw.item as string | undefined,
		quantity: raw.quantity as number | undefined,
		unit: raw.unit as string | undefined,
		packaging: raw.packaging as string | undefined,
		budget_per_unit: raw.budget_per_unit as number | undefined,
		delivery_city: raw.delivery_city as string | undefined,
		delivery_date: raw.delivery_date as string | undefined,
		exact_match_required: raw.exact_match_required as boolean | undefined,
		// 巢狀物件原樣帶過，型別轉換在 mapSlots 做（normalizeItemResolution）。
		// 刻意不在這裡驗：這支函式對扁平欄位是「型別不符就整包回 null」，而那個
		// 嚴格度對 item_resolution 不對——它是顯示用的解析結果，不該讓整個 session
		// 快照因為它而解析失敗。分層因此是：扁平欄位在這裡守，巢狀物件在
		// normalizeItemResolution 自己守（不合契約就丟掉那一個欄位）。
		item_resolution: raw.item_resolution as
			| RawProcurementItemResolution
			| undefined,
	};
}

export interface NormalizedProcurementTurnResult {
	requestId: string;
	revisionId: string;
	conversationId?: string;
	slots: RawProcurementSlots;
	rejected: Array<{ key: string; reason: string }>;
	missingRequired: string[];
	degraded: boolean;
	degradedReason?: string;
	reply: string;
	blocks?: PublicNarrativeBlock[]; // #2549 Phase 2 新增
	textFallback?: string; // #2549 Phase 2 新增
	question?: unknown;
	recordedThisTurn?: unknown;
	cropDisambiguation?: unknown;
	/** doc57 run ledger 的 interrupt 標記——只有這一輪落在 WAITING_FOR_INPUT
	 * 才會出現。unary SubmitTurn 與串流終態的 RUN_FINISHED.result 是同一份
	 * TurnResult（backend gateway/internal/proxy/ai_assistant_stream.go 的
	 * 註解：「interrupt_id=22, interrupt_kind=23 attached directly on
	 * TurnResult」），欄位形狀不因 transport 而異。兩者成對出現，缺一即視為
	 * 壞掉的回應（見 normalizeProcurementTurnResult 的驗證）。 */
	interruptId?: string;
	interruptKind?: string;
}

/** §F 的 7 個 block kind（6 個真實 kind + legacy fallback 專用的 SUMMARY）。
 * 與 narrative.ts 的 PublicNarrativeKind 是同一組值——這裡另立一份 allowlist
 * 陣列，是因為驗證時需要在執行期用 `includes` 逐一核對字串，型別本身在
 * 執行期並不存在。 */
const NARRATIVE_KINDS: readonly PublicNarrativeKind[] = [
	"UNDERSTOOD",
	"CONTEXT",
	"RATIONALE",
	"WARNING",
	"NEXT_STEP",
	"COMPLETE",
	"SUMMARY",
];

function isPublicNarrativeKind(kind: unknown): kind is PublicNarrativeKind {
	return (
		typeof kind === "string" &&
		(NARRATIVE_KINDS as readonly string[]).includes(kind)
	);
}

/** blocks[] 是唯一一個沒有逐元素驗證就被塞進 NormalizedProcurementTurnResult
 * 的欄位——而它正是 ProcurementClarifyChat 會直接 `.map()` 渲染的欄位。跟
 * `rejected` 同一個 flatMap-with-validation 慣例：形狀不對的元素直接丟棄，
 * 不 throw，也不讓整個 turn 因為一個壞元素報廢。
 *
 * kind 要核對 NARRATIVE_KINDS 這個真正的 allowlist，不能只查 `typeof ===
 * "string"`——NARRATIVE_ICON 是 `Object.freeze` 過的物件，但 freeze 不切斷
 * prototype chain，`kind: "constructor"` 這種字串在 `NARRATIVE_ICON[kind]`
 * 查表時會撞到 `Object.prototype.constructor`（truthy、但不是字串），沒有
 * allowlist 這道檢查的話會被當成合法 icon name 送進渲染層。
 *
 * 驗證後若整批都被丟棄，回傳 undefined 而非空陣列——讓呼叫端
 * `turn.blocks ?? legacyReplyToBlocks(turn.text)` 的 fallback 邏輯在前端自己
 * 就成立，不依賴後端 `omitempty` 剛好把空陣列擋在外面這個上游巧合。 */
export function normalizeBlocks(
	raw: unknown,
): PublicNarrativeBlock[] | undefined {
	if (!Array.isArray(raw)) return undefined;
	const blocks = raw.flatMap((item): PublicNarrativeBlock[] => {
		if (!isRecord(item)) return [];
		if (!isPublicNarrativeKind(item.kind)) return [];
		if (typeof item.text !== "string") return [];
		return [{ kind: item.kind, text: item.text }];
	});
	return blocks.length > 0 ? blocks : undefined;
}

/** Browser wire is untrusted even when apiClient has a generic type argument. */
export function normalizeProcurementTurnResult(
	raw: unknown,
): NormalizedProcurementTurnResult | null {
	if (!isRecord(raw)) return null;
	if (
		typeof raw.requestId !== "string" ||
		raw.requestId.length === 0 ||
		raw.requestId.trim() !== raw.requestId ||
		typeof raw.revisionId !== "string" ||
		raw.revisionId.length === 0 ||
		raw.revisionId.trim() !== raw.revisionId ||
		typeof raw.reply !== "string" ||
		typeof raw.degraded !== "boolean"
	)
		return null;
	if (
		(raw.conversationId !== undefined &&
			raw.conversationId !== null &&
			(typeof raw.conversationId !== "string" ||
				(raw.conversationId.length > 0 &&
					raw.conversationId.trim() !== raw.conversationId))) ||
		(raw.degradedReason !== undefined &&
			raw.degradedReason !== null &&
			typeof raw.degradedReason !== "string")
	)
		return null;
	if (
		(raw.interruptId !== undefined &&
			(typeof raw.interruptId !== "string" || raw.interruptId.length === 0)) ||
		(raw.interruptKind !== undefined &&
			(typeof raw.interruptKind !== "string" || raw.interruptKind.length === 0))
	)
		return null;
	// interruptId／interruptKind 在 wire 上成對出現（同一個 TurnResult 直接帶
	// 兩者）；只出現一個是不可能的中介狀態，視為壞掉的回應，不猜測另一半的值。
	if (Boolean(raw.interruptId) !== Boolean(raw.interruptKind)) return null;
	const slots = normalizeRawProcurementSlots(raw.slots);
	if (!slots) return null;
	if (
		!Array.isArray(raw.missingRequired) ||
		raw.missingRequired.some((key) => typeof key !== "string") ||
		!Array.isArray(raw.rejected) ||
		raw.rejected.some(
			(item) =>
				!isRecord(item) ||
				typeof item.key !== "string" ||
				typeof item.reason !== "string",
		)
	)
		return null;
	return {
		requestId: raw.requestId,
		revisionId: raw.revisionId,
		conversationId:
			typeof raw.conversationId === "string" && raw.conversationId.length > 0
				? raw.conversationId
				: undefined,
		slots,
		rejected: raw.rejected.map((item) => {
			const record = item as Record<string, unknown>;
			return { key: record.key as string, reason: record.reason as string };
		}),
		missingRequired: [...raw.missingRequired],
		degraded: raw.degraded,
		degradedReason:
			typeof raw.degradedReason === "string" ? raw.degradedReason : undefined,
		reply: raw.reply,
		blocks: normalizeBlocks(raw.blocks),
		textFallback:
			typeof raw.textFallback === "string" ? raw.textFallback : undefined,
		question: raw.question,
		recordedThisTurn: raw.recordedThisTurn,
		cropDisambiguation: raw.cropDisambiguation,
		interruptId:
			typeof raw.interruptId === "string" ? raw.interruptId : undefined,
		interruptKind:
			typeof raw.interruptKind === "string" ? raw.interruptKind : undefined,
	};
}

export interface NormalizedCandidateSelectionResult {
	candidateId: string;
	selected: boolean;
}

export function normalizeCandidateSelectionResult(
	raw: unknown,
	expectedCandidateId: string,
): NormalizedCandidateSelectionResult | null {
	if (
		!isRecord(raw) ||
		raw.candidateId !== expectedCandidateId ||
		typeof raw.selected !== "boolean"
	)
		return null;
	return { candidateId: expectedCandidateId, selected: raw.selected };
}

export function mapSlots(raw: unknown): ProcurementSlots {
	const normalized = normalizeRawProcurementSlots(raw) ?? {};
	return {
		item: normalized.item,
		quantity: normalized.quantity,
		unit: normalized.unit,
		packaging: normalized.packaging,
		budgetPerUnit: normalized.budget_per_unit,
		deliveryCity: normalized.delivery_city,
		deliveryDate: normalized.delivery_date,
		exactMatchRequired: normalized.exact_match_required,
		itemResolution: normalizeItemResolution(normalized.item_resolution),
	};
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
 * snapshot 的 status 是後端權威狀態。CLARIFYING 以外一律落到 "ready"——
 * MATCHING／CANDIDATES_READY 沒有可復原的 runId（GetSnapshot 不回傳它），
 * 但 POST /candidate-searches 對同一 revision 是冪等的（重複呼叫回傳既有
 * run，若已終態就直接帶終態候選回來），所以「ready」stage 的「開始搜尋
 * 候選」按鈕天然就是這兩個狀態的復原路徑，不需要後端另外加欄位。
 */
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

export function deriveStageFromStatus(
	status: ProcurementSnapshot["status"],
): ProcurementStage {
	return status === "CLARIFYING" ? "clarifying" : "ready";
}

/**
 * run 是否已進終態。這是輪詢的唯一終止條件（useProcurementSession 每 1.5s 打一次
 * GET candidate-searches），漏判一個終態值 = 無限輪詢、畫面永遠停在「正在搜尋候選」。
 *
 * 改名前的 `COMPLETED` / `PARTIAL` 已於本波移除：backend 已部署改名版本，DB 既有列也已由
 * migration 收斂。⚠️ 漏判**不會**有錯誤訊號——輪詢拿到的是 200 OK，只是永遠不終止。
 */
export function isTerminalCandidateStatus(
	status: ProcurementCandidateSearch["status"],
): boolean {
	return status === "SUCCEEDED" || status === "FAILED";
}

/**
 * run 是否成功結束（相對於 FAILED）。改名前這需要同時比對 COMPLETED 與 PARTIAL，
 * 因為「部分成功」曾是獨立終態；新值域裡部分成功就是 SUCCEEDED + 某條 lane FAILED，
 * 所以成功終態只剩一個。函式保留（而非讓呼叫端散寫 `=== "SUCCEEDED"`）是因為
 * hasPartialLaneFailure 的語意直接建立在它上面。
 */
export function isSucceededCandidateStatus(
	status: ProcurementCandidateSearch["status"],
): boolean {
	return status === "SUCCEEDED";
}

/**
 * 是否要提示「部分來源未完成」。契約凍結後 run 狀態不再帶這個資訊——判斷依據是
 * 「run 成功但有 lane 失敗」，這也是新值域對「部分成功」的定義本身。
 */
export function hasPartialLaneFailure(
	search: Pick<
		ProcurementCandidateSearch,
		"status" | "platformLaneStatus" | "externalLaneStatus"
	>,
): boolean {
	if (!isSucceededCandidateStatus(search.status)) return false;
	return (
		search.platformLaneStatus === "FAILED" ||
		search.externalLaneStatus === "FAILED"
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
