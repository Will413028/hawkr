import { parseJsonSafe } from "@/lib/api/json";
import {
	ProcurementStepStatus,
	type ProcurementStepStatusValue,
} from "@/lib/codemeta/enums";

//
// Graph-path Activity events (T12 sub-project 3c) — a deliberately
// isolated wire/type/trust model from steps.ts's AgentStep. steps.ts's
// `label` is never trusted (only structured safeArgs cross the trust
// boundary); this `text` field is a backend-guarded model-generated
// sentence that IS trusted directly, by design. Merging the two would
// force `label` to become conditionally trusted depending on source —
// which is the single invariant that makes steps.ts safe today — or
// force the gateway to translate AGENT_ACTIVITY_SNAPSHOT into the
// step_detail proto shape, a coupling neither repo has.
// Do not merge this into AgentStep/steps.ts.

/** 值域 SSOT 是 CodeMeta 生成的 `ProcurementStepStatus`（7 個值），不是手寫
 * union——`lib/codemeta/generated.ts` 由 backend 的 generate.sh 覆蓋，後端增刪值
 * 時這裡是編譯期連動的。steps.ts 的 `AgentStepStatus` 是 2026-08 手寫的舊做法，
 * 它自己的註解就寫明「漏列一個線上真的會送的值...只會讓那些步驟被整個丟掉」；
 * 這條路徑不重蹈那個覆轍。 */
export type AgentActivityStatus = ProcurementStepStatusValue;

export interface AgentActivityEvent {
	rowId: string;
	/** 沿用 steps.ts 的 stepIcon 查表選圖示；查不到用預設圖示，不是自由文字風險
	 * （這裡的 code 只驅動圖示查表，不像 steps.ts 的 code 還決定要不要顯示固定
	 * 文案，未知 code 沒有安全疑慮）。 */
	code: string;
	/** 後端固定字串表（`domain.StepLabels`）產的短句，wire 上恆非空。跟 steps.ts
	 * 的 `label`（wire 值永不採用）**不是同一件事**：那條路徑的 label 有被模型
	 * 回填的可能，這條沒有——`ledgerEventTypeForActivity` 只從 StepLabels 查表，
	 * 結構上碰不到模型輸出。所以這裡直接信任顯示。
	 *
	 * 缺席時以 `""` 表示（見 normalizeActivityEvent）：T23 之前部署的 pod 不送這
	 * 個欄位，整筆丟棄會讓 rail 在部署窗口內整條消失，比少一行字糟得多。 */
	label: string;
	/** 這一列的執行狀態。失敗住這一軸，code 軸沒有 FAILED（T23 D3）。 */
	status: AgentActivityStatus;
	/** 父列的 rowId（十進位字串）。只有巢狀子列有——設計稿 01 的兩條 lane 掛在
	 * 「搜尋候選供應商」底下。rail 只支援一層，父列自己不得帶這個欄位。
	 * **缺席 ≠ 掛在 id 0 的列**，所以是 undefined 不是 ""。 */
	parentRowId?: string;
	/** 這一列的計數（十進位字串，例如 lane 找到幾家）。只有真的有計數概念的列
	 * 才有。**缺席 ≠ 0**：flow 53 實測 PLATFORM 查到 0 家時送 `count:"0"`，而父列
	 * 與 SUMMARIZE 整個欄位不存在，兩者不得互相代換。 */
	count?: string;
	/** "" = 這一列沒有 narrator 敘述——不代表未完成。候選搜尋 run 整條都沒有
	 * narrator（T23 ADR O1），四列的 text 恆為空。非空時是已通過 backend guard
	 * 的模型短句，直接信任顯示，不做內容白/黑名單。 */
	text: string;
}

/** Fail-closed 解析：非物件或任一必要欄位型別不符就整筆丟棄——同 steps.ts
 * normalizeSteps 的紀律。 */
export function normalizeActivityEvent(
	raw: unknown,
): AgentActivityEvent | null {
	if (!raw || typeof raw !== "object") return null;
	const record = raw as Record<string, unknown>;
	if (typeof record.rowId !== "string" || record.rowId === "") return null;
	if (typeof record.code !== "string" || record.code === "") return null;
	if (typeof record.text !== "string") return null;
	return {
		rowId: record.rowId,
		code: record.code,
		label: optionalString(record.label) ?? "",
		status: normalizeActivityStatus(record.status, record.text),
		parentRowId: optionalString(record.parentRowId),
		count: optionalString(record.count),
		text: record.text,
	};
}

/** 一個 top-level 列，以及掛在它底下的 lane 子列。 */
export interface ActivityRow extends AgentActivityEvent {
	children: AgentActivityEvent[];
}

/**
 * 把扁平的 activity 串收成設計稿 01 的一層巢狀（兩條 lane 掛在「搜尋候選供應商」
 * 底下）。
 *
 * **只做一層**，跟後端保證一致（`candidate_search_test.go` 釘住父列不得帶
 * `parentRowId`）。掛在子列底下的孫列會升級成 top-level 而不是做出第二層縮排——
 * 那是契約已經壞掉的情況，多縮一層只會讓畫面看起來像是刻意的。
 *
 * 認不出父列的子列一律**升級成 top-level，不丟掉**：靜默少一列比多一列平的糟
 * 得多（同 candidate-grouping.ts 的 `ungrouped` 判準）。這件事在部署窗口與亂序
 * 抵達時都會真的發生。
 *
 * top-level 維持抵達順序，不重排——後端送出的順序就是執行順序。
 */
export function groupActivityRows(
	events: readonly AgentActivityEvent[],
): ActivityRow[] {
	const topLevel: ActivityRow[] = [];
	const byRowId = new Map<string, ActivityRow>();

	// 單趟掃描，不是「先收父列再收子列」兩趟：兩趟會把升級成 top-level 的子列
	// 全部推到最後面，順序就不再是抵達順序了。後端保證父列先於子列送出
	// （父列 RUNNING → 兩條 lane 開跑），所以單趟就認得出父列。
	for (const event of events) {
		const parent =
			event.parentRowId === undefined
				? undefined
				: byRowId.get(event.parentRowId);
		if (parent) {
			parent.children.push(event);
			continue;
		}
		// 走到這裡有三種情況：本來就是 top-level；父列還沒抵達；父列自己是子列
		// （孫列，只會發生在契約壞掉時）。三種都攤平成 top-level。
		const row: ActivityRow = { ...event, children: [] };
		topLevel.push(row);
		byRowId.set(row.rowId, row);
	}

	return topLevel;
}

/** 每一列右側的狀態字（設計稿 01：已完成／進行中／等待中）。
 *
 * 刻意不用 CodeMeta 的 `PROCUREMENT_STEP_STATUS_LABEL`：那份是管理端口吻
 * （PENDING「待執行」、RUNNING「執行中」），稿上給買家看的是「等待中」「進行中」。
 * 同 `CandidateSearchProgress` 的 `LANE_STATUS_LABEL` 慣例——值域綁 CodeMeta、
 * 買家面文案留在前端。
 *
 * `Record<AgentActivityStatus, string>` 而不是 index signature：CodeMeta 增值時
 * 這裡少一個 key 就是編譯錯誤，不是執行期查表回 undefined、那一格空白。 */
const ACTIVITY_STATUS_LABEL: Record<AgentActivityStatus, string> = {
	PENDING: "等待中",
	RUNNING: "進行中",
	WAITING_FOR_INPUT: "等你回覆",
	SUCCEEDED: "已完成",
	FAILED: "沒過",
	SKIPPED: "略過",
	CANCELLED: "已取消",
};

export function activityStatusLabel(status: AgentActivityStatus): string {
	return ACTIVITY_STATUS_LABEL[status];
}

/** 三個 optional 欄位（label／parentRowId／count）共用的規則：**缺席或型別不符
 * 一律當缺席，整列照樣渲染**。
 *
 * 這跟 rowId／code／text 的 fail-closed 是刻意不同的紀律。那三個缺一個就沒有可
 * 渲染的列了；這三個各自缺席時，這一列只是退回 T23 之前的樣子（沒有巢狀、沒有
 * 家數、文字退回 text），那是一個已知good的渲染。丟掉整列則沒有退路也沒有訊號。
 *
 * 空字串一併當缺席：`parentRowId: ""` 不是「掛在 id 空字串的列」，`count: ""`
 * 不是「0 家」。後端本來就不會送空字串（有值才進 payload），這裡只是不讓一個
 * 壞掉的中間層把語意換掉。 */
function optionalString(raw: unknown): string | undefined {
	return typeof raw === "string" && raw !== "" ? raw : undefined;
}

/** status 缺席或不在值域內時，退回 T23 之前唯一可用的訊號：`text` 空 = 還在跑。
 *
 * 刻意**不** fail closed 丟掉整列。steps.ts 的 deny-by-default 守的是 privacy
 * boundary（未知 code／label 不得進 DOM），status 不是——它只決定這一列畫成什麼
 * 樣子。對它 fail closed 換不到任何安全，卻會讓「後端多加一個狀態值」表現成
 * rail 無聲變短，而那正是 steps.ts 註解警告過的那種無訊號故障。 */
function normalizeActivityStatus(
	raw: unknown,
	text: unknown,
): AgentActivityStatus {
	if (typeof raw === "string" && Object.hasOwn(ProcurementStepStatus, raw)) {
		return raw as AgentActivityStatus;
	}
	return text === "" ? "RUNNING" : "SUCCEEDED";
}

/** 展示層截斷，避免極端長字串撐壞 UI——不是內容白/黑名單，純粹是佈局保護。 */
const ACTIVITY_TEXT_MAX_DISPLAY_LEN = 80;

/** 顯示優先序：`text`（這一次實際發生了什麼）→ `label`（後端固定短句）→ 佔位。
 *
 * 順序不能反過來：proto 註解寫明「空的時候 consumer 顯示 label，不是反過來」。
 * 舊版把 `text === ""` 直接當成「還沒完成」而回佔位字串，於是沒有 narrator 的
 * run（T23 候選搜尋的四列）永遠停在「處理中…」。 */
export function activityLabel(event: AgentActivityEvent): string {
	const source = event.text !== "" ? event.text : event.label;
	if (source === "") return "處理中…";
	// 用 code point 而非 UTF-16 code unit 截斷——`.length`／`.slice` 是後者，
	// 剛好切在 surrogate pair（例如 emoji）中間會留下孤兒 surrogate，
	// 顯示成 U+FFFD 替換字元。
	const chars = [...source];
	return chars.length > ACTIVITY_TEXT_MAX_DISPLAY_LEN
		? `${chars.slice(0, ACTIVITY_TEXT_MAX_DISPLAY_LEN).join("")}…`
		: source;
}

/**
 * 把一個 Activity 事件併進目前的列表。同一個 rowId 的 entry（text=""）→
 * completion（text 非空）都指向同一列，以 rowId 覆寫既有列而不是無腦
 * append。防呆：空字串（entry）事件不得覆蓋已存在的非空字串（completion）
 * ——防的是 SSE reconnect/Last-Event-ID replay 把舊 entry 事件重送蓋掉已
 * 顯示的完成內容。
 *
 * 住在 activity.ts 而不是某一支 client：POST turn stream 與 GET run events
 * 兩條 transport 都要用它，而它的規則是 Activity 這個型別自己的規則。
 */
export function mergeLiveActivity(
	events: AgentActivityEvent[],
	next: AgentActivityEvent,
): AgentActivityEvent[] {
	const idx = events.findIndex((e) => e.rowId === next.rowId);
	if (idx === -1) return [...events, next];
	const prev = events[idx];
	if (next.text === "" && prev.text !== "") return events;
	const out = [...events];
	out[idx] = next;
	return out;
}

/** searching 期間「現在在做什麼」的那一列，給 sr-only 播報用。
 *
 * 刻意不是 `.at(-1)`：候選搜尋 run 開跑時會先把整份步驟清單以 PENDING 送出
 * （declarePlan），所以最後抵達的那一列是還沒跑到的「整理成給你的建議」。
 * 播報它會對螢幕閱讀器講一句當下為假的話。 */
export function lastRunningActivity(
	events: readonly AgentActivityEvent[],
): AgentActivityEvent | null {
	for (let i = events.length - 1; i >= 0; i -= 1) {
		if (events[i].status === "RUNNING") return events[i];
	}
	return null;
}

/** 一個 CUSTOM activity frame 的 `value`（JSON 字串）解析成列。壞掉的 JSON 與
 * 不合契約的 payload 都回 null，跟 normalizeActivityEvent 同一套 fail-closed
 * 紀律——不讓半個 payload 進 DOM。 */
export function parseActivityEventJson(raw: string): AgentActivityEvent | null {
	let parsed: unknown;
	try {
		parsed = parseJsonSafe(raw);
	} catch {
		return null;
	}
	return normalizeActivityEvent(parsed);
}
