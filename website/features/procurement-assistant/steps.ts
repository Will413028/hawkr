import type { AppIconName } from "@/components/shared/AppIcon";

/** 後端 B-244 Ask 2 的白名單步驟碼。前端再次以固定 code／label 表驗證；
 * 未知 code 或 wire label 一律不進 Activity Rail。公開進度的 privacy boundary
 * 不能用 forward-compat fail-open，否則 private tool code／label 會直接進 DOM。 */
/**
 * 值域 SSOT 是 backend CodeMeta 的 ProcurementStepStatus（issue #2446 契約凍結）。
 * 這個 union 是手寫的：proto 的 `StepStatus` 另有 PENDING／WAITING_FOR_INPUT／CANCELLED，
 * `proto-map.ts` 的 `stepFromProto` 把 WAITING_FOR_INPUT 當 RUNNING，其餘沒有對應的值不顯示。
 */
export type AgentStepStatus = "RUNNING" | "SUCCEEDED" | "SKIPPED" | "FAILED";

/** step 已完成。Rail 的收尾判斷一律走這裡，不要散寫 `=== `。 */
export function isStepSucceeded(status: AgentStepStatus): boolean {
	return status === "SUCCEEDED";
}

export interface AgentStep {
	code: string;
	status: AgentStepStatus;
	/** Wire label 永不採用；固定短句一律由前端 allowlist 決定（見
	 * STEP_FALLBACK_LABELS）。這條沒有改變——改變的是下面的 `text`。 */
	label?: string;
	/** 後端 StepNarrator（LLM）就這一步的結構化結果產生的一句話，例如
	 * 「甘藍條件有效，無拒絕欄位。」。有值時只在 pending 的 Live Activity Rail
	 * 顯示，取代前端固定短句；不投影進終態 turn 或歷史紀錄。空字串／缺席代表
	 * narrator 逾時或被 guard 擋下，此時退回既有固定短句。 */
	text?: string;
	durationMs?: number;
	/** 少數 code 的動態文案模板要用的一小撮 slots 值，來源見後端
	 * domain.Step.SafeArgs——鍵值皆為結構化資料，不是自由文字。 */
	safeArgs?: Record<string, string>;
}

/**
 * `CAPTURE_*` 是抽取階段的逐欄位子步驟。命名是封閉約定：一個 slot key
 * 一碼，所以 mergeLiveStep 按 code 收斂時每個欄位都會是自己一行。
 * `CAPTURE_SPEC` 已隨 current origin/dev 的 spec slot 移除，不再接受。
 */

/** 步驟碼 → AppIcon 名稱。標了「暫代」的項目等 B-244 icon 採購清單
 * 到貨後替換（比對天平／靜態地球／排序圖示目前不存在於 registry）。 */
export const STEP_ICON: Record<string, AppIconName> = {
	UNDERSTAND_INPUT: "message",
	EXTRACT_SLOTS: "document",
	// 逐欄位抽取：每個欄位給自己的 glyph，否則 8 行全落到 clock 降級、
	// 長得一模一樣，就失去「一行字＋icon」的意義。
	CAPTURE_ITEM: "leaf",
	CAPTURE_QUANTITY: "cubes",
	CAPTURE_UNIT: "category",
	CAPTURE_PACKAGING: "box",
	CAPTURE_BUDGET: "wallet",
	CAPTURE_CITY: "pin",
	CAPTURE_DATE: "calendar",
	CAPTURE_EXACT_MATCH: "scan",
	VALIDATE_SLOTS: "filter",
	// 這一步跟 VALIDATE_SLOTS 同輪相鄰出現，glyph 必須明顯不同才看得出是兩件事。
	// folder＝分類，語意直指「把買家講的品項對到哪一個作物分類」。
	CROP_GATE: "folder",
	SLOTS_REJECTED: "info",
	PICK_NEXT_QUESTION: "info",
	REQUIREMENT_READY: "check",
	// 候選搜尋階段的父列（T23）。兩條 lane 子列各自是 search／discovery，父列
	// 必須明顯不同——三個 glyph 疊在同一組巢狀列裡，長得像就等於沒有分層。
	SEARCH_CANDIDATES: "people",
	SEARCH_PLATFORM: "search",
	// 暫代：discovery 實際是指北針；待靜態地球 glyph 到貨後替換。
	SEARCH_EXTERNAL: "discovery",
	READ_SELLER: "user",
	// 暫代：待天平／比較 glyph。
	COMPARE: "reorder",
	VERIFY_ORIGIN: "location",
	CHECK_STOCK: "stock",
	// 暫代：待排序／排名 glyph。
	RANK_CANDIDATES: "graph",
	SUMMARIZE: "clipboard",
	DEGRADED_FALLBACK: "info",
};

/** Public Activity Rail 的**退路**文案。後端 `label` 仍然一律丟棄。
 *
 * `FAILED` 已隨 T23 移除：失敗住 status 軸，`domain.StepCode` 不得再出現這個碼
 * （T23 ADR D3 的 Invariant）。留著會讓同一件事橫跨兩個 enum 各自演化。
 *
 * 但「使用者看見的文字只能由這張表決定」已經不成立——2026-09-03 起
 * AgentStep.text（後端 StepNarrator 產的自由文字）優先於這張表，見 stepLabel。
 * 這張表現在的角色是 narrator 缺席時的退路，不是唯一來源。
 *
 * 文案守 brand voice 三條規則：動詞開頭（不是「正在X」的狀態描述）、
 * 具體勝過漂亮（講抓什麼欄位，不講「整理條件」）、第二人稱。 */
export const STEP_FALLBACK_LABELS: Record<string, string> = {
	UNDERSTAND_INPUT: "讀你寫的需求",
	EXTRACT_SLOTS: "抓出品項、數量和交期",
	CAPTURE_ITEM: "抓到你要的品項",
	CAPTURE_QUANTITY: "抓到數量",
	CAPTURE_UNIT: "抓到單位",
	CAPTURE_PACKAGING: "抓到包裝方式",
	CAPTURE_BUDGET: "抓到你的預算",
	CAPTURE_CITY: "抓到送達縣市",
	CAPTURE_DATE: "抓到到貨日",
	CAPTURE_EXACT_MATCH: "抓到要不要完全相符",
	VALIDATE_SLOTS: "核對條件合不合理",
	CROP_GATE: "核對品項分類",
	SLOTS_REJECTED: "有幾個條件先擱著",
	PICK_NEXT_QUESTION: "看還缺什麼關鍵資訊",
	REQUIREMENT_READY: "需求齊了",
	SEARCH_CANDIDATES: "搜尋候選供應商",
	SEARCH_PLATFORM: "找平台內的候選資料",
	SEARCH_EXTERNAL: "找平台外的公開候選資料",
	READ_SELLER: "讀這筆候選的公開資料",
	COMPARE: "把公開條件放在一起比",
	VERIFY_ORIGIN: "整理產地欄位",
	CHECK_STOCK: "標出仍需商家確認的供貨資訊",
	RANK_CANDIDATES: "整理候選資料順序",
	SUMMARIZE: "整理這一輪已提供的事實",
	DEGRADED_FALLBACK: "解析不穩，改用簡化方式",
};

export function stepIcon(code: string): AppIconName {
	return STEP_ICON[code] ?? "clock";
}

/** 只有這三個 code 有具體 slots 欄位可講：EXTRACT_SLOTS／PICK_NEXT_QUESTION
 * 剛抓到什麼，REQUIREMENT_READY 收斂成最終確認句。缺 item（每個模板的必要
 * 欄位）時回傳 undefined，讓呼叫端退回既有固定文案，不拼半殘句。 */
const DYNAMIC_LABEL_TEMPLATES: Record<
	string,
	(args: Record<string, string>) => string | undefined
> = {
	EXTRACT_SLOTS: (args) =>
		args.item && args.city
			? `抓到你要送到【${args.city}】的【${args.item}】`
			: undefined,
	PICK_NEXT_QUESTION: (args) =>
		args.item ? `已經抓到【${args.item}】，看還缺什麼` : undefined,
	REQUIREMENT_READY: (args) =>
		args.item && args.city
			? `條件都齊了：【${args.item}】送到【${args.city}】`
			: undefined,
};

export function stepLabel(
	step: Pick<AgentStep, "code" | "label" | "safeArgs" | "text">,
): string {
	// 後端敘述優先——那是「這一步實際發生了什麼」，比任何前端模板都具體。
	// 沒有敘述才退回既有的動態模板／固定短句（narrator 逾時或被 guard 擋下
	// 時就會走到這裡，所以這條退路必須留著）。
	if (step.text) return step.text;
	const dynamic =
		step.safeArgs && DYNAMIC_LABEL_TEMPLATES[step.code]?.(step.safeArgs);
	return dynamic ?? STEP_FALLBACK_LABELS[step.code] ?? "處理中";
}

export function isPublicAgentStepCode(code: string): boolean {
	return Object.hasOwn(STEP_FALLBACK_LABELS, code);
}

/**
 * 把一個 step 事件併進目前的步驟串。
 *
 * 同一個 code 的 RUNNING → SUCCEEDED 都指向同一步，所以以 code 為 key 覆寫既有步驟，而不是 append。
 * SUCCEEDED 帶著耗時與 narrator 文字，覆寫時要保留；已完成後遲到的 RUNNING 不得把它打回進行中。
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
		status: isStepSucceeded(prev.status) ? prev.status : next.status,
		durationMs: staleRunning
			? prev.durationMs
			: (next.durationMs ?? prev.durationMs),
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
