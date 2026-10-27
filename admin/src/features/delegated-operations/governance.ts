import { hasAsciiControlCharacters, isSafeOpaqueId } from "./policy";

/**
 * 治理視角的三支查詢（撤銷／稽核紀錄／工作階段摘要）共用的 DTO 與 parser。
 *
 * 與 `model.ts` 分開的理由：那一份描述「建立工作階段」這條路徑上瀏覽器摸得到的
 * 東西（preflight、session summary），它的 fail-closed 是為了「不要在憑證不明時
 * 進工作區」；這一份描述**已經發生過的事**，它的 fail-closed 是為了「不要顯示一份
 * 不完整的稽核紀錄」——後者一旦少一列，畫面上看起來完全正常。
 */

/** `chk_aoal_result`。PENDING 是合法終態，不是過渡狀態。 */
export const DELEGATED_AUDIT_RESULTS = [
	"SUCCEEDED",
	"FAILED",
	"PENDING",
] as const;
export type DelegatedAuditResult = (typeof DELEGATED_AUDIT_RESULTS)[number];

/** `chk_aoal_resource_type`。 */
export const DELEGATED_AUDIT_RESOURCE_TYPES = [
	"storefront",
	"product",
	"post",
	"order",
] as const;
export type DelegatedAuditResourceType =
	(typeof DELEGATED_AUDIT_RESOURCE_TYPES)[number];

/** `chk_aoal_risk_level`。 */
export const DELEGATED_AUDIT_RISK_LEVELS = ["low", "medium", "high"] as const;
export type DelegatedAuditRiskLevel =
	(typeof DELEGATED_AUDIT_RISK_LEVELS)[number];

/**
 * `DelegationAuditAction` 的封閉十九值（56 §6.5.3 (3)）。
 *
 * 這裡只存**值域**，稽核列的中文由後端逐列下發的 `actionLabel` 決定（同
 * `changedFields[].label` 的理由）。值域留在前端是為了讓篩選送出前就擋掉不合法
 * 值：gateway 對未知篩選值回 9260，靜默送出只會換來一次 400。
 *
 * ⚠️ 篩選下拉是例外，因為它必須在**任何資料到達之前**就列滿這 19 個選項，此時
 * 權威中文還不存在。那一份引導用的中文在 `src/lib/codemeta/generated.ts` 的
 * `DELEGATION_AUDIT_ACTION_LABEL`，且被兩件事綁住：本常數與它的鍵集合必須相等
 * （`tests/delegated-operations-ui-contract.test.ts` 斷言），以及資料一到就以
 * `actionLabel` 就地覆蓋。
 *
 * ⚠️ **新增一個動作時，那一半不能在 admin 手改** —— `generated.ts` 是 backend
 * `generate.sh --admin` 的產物（DO NOT EDIT，CI 檢查 drift）。正解是改
 * `backend/docs/代碼管理（CodeMeta）/內部技術碼.md` → 重新生成；本檔這一半照常手動加。
 * 兩邊鍵集合不等時上面那支測試會紅。
 */
export const DELEGATED_AUDIT_ACTIONS = [
	"STOREFRONT_UPDATE",
	"STOREFRONT_LOGO_UPDATE",
	"STOREFRONT_COVER_UPDATE",
	"PRODUCT_CREATE",
	"PRODUCT_UPDATE",
	"PRODUCT_PUBLISH",
	"PRODUCT_UNPUBLISH",
	"PRODUCT_BATCH_UNPUBLISH",
	"POST_CREATE",
	"POST_UPDATE",
	"POST_PUBLISH",
	"POST_UNPUBLISH",
	"POST_SCHEDULE",
	"ORDER_PII_REVEAL",
	"STOREFRONT_READ",
	"PRODUCT_READ",
	"POST_READ",
	"ORDER_READ",
	"AUDIT_CORRECTION",
] as const;
export type DelegatedAuditAction = (typeof DELEGATED_AUDIT_ACTIONS)[number];

/** 工作階段終態三值（`delegated_operation_sessions.status`）。 */
export const DELEGATED_SESSION_STATUSES = [
	"active",
	"expired",
	"revoked",
] as const;
export type DelegatedSessionStatus =
	(typeof DELEGATED_SESSION_STATUSES)[number];

export const DELEGATED_AUDIT_MAX_PAGE_SIZE = 50;

export interface DelegatedAuditActor {
	adminId: string;
	displayName: string;
}

export interface DelegatedAuditTarget {
	storeId: string;
	sellerAccountId: string;
	storeName: string;
}

export interface DelegatedChangedField {
	field: string;
	label: string;
	before: string;
	after: string;
}

export interface DelegatedAuditLogEntry {
	auditLogId: string;
	createdAt: string;
	confirmedAt: string | null;
	actor: DelegatedAuditActor;
	target: DelegatedAuditTarget;
	action: string;
	actionLabel: string;
	riskLevel: DelegatedAuditRiskLevel;
	capability: string;
	resourceType: DelegatedAuditResourceType;
	resourceId: string;
	resourceName: string;
	resourceImageKey: string;
	operationReason: string;
	caseReference: string | null;
	result: DelegatedAuditResult;
	bizCode: number;
	/**
	 * `bizCode` 的中文說明，由後端查 CodeMeta 錯誤碼表下發。
	 *
	 * ⚠️ **不得在前端自建碼→文案對照表**：那是第二份 CodeMeta 鏡像，Markdown 改了
	 * 文案它不會轉紅，只是從那天起顯示舊字串。後端拿不出說明時是空字串，此時退回
	 * 只顯示 `bizCode`——顯示原碼還追得到，一句自己編的「未知錯誤」則會蓋掉線索。
	 */
	bizCodeLabel: string;
	resourceVersionBefore: string;
	resourceVersionAfter: string;
	changedFields: DelegatedChangedField[];
	correlationId: string;
	requestId: string;
	idempotencyKey: string;
	origin: string;
	/** `user_agent` 原文。解析成「Chrome 126 (macOS)」是呈現，不是後端欄位。 */
	userAgent: string;
	/** 語意是「代行完成事件已發布至 MQ」，不是「賣家已收到」。 */
	notifiedAt: string | null;
}

export interface DelegatedAuditLogPage {
	records: DelegatedAuditLogEntry[];
	total: number;
	page: number;
	pageSize: number;
}

export interface DelegatedSessionReport {
	sessionId: string;
	displayCode: string;
	target: DelegatedAuditTarget;
	actor: DelegatedAuditActor;
	issuedAt: string;
	endedAt: string | null;
	durationSeconds: number;
	status: DelegatedSessionStatus;
	operationCount: number;
	riskOperationCount: number;
	/**
	 * 只有 0 才可以顯示「所有結果均已確認」，而 0 還不夠 —— 一筆紀錄都沒有時那句
	 * 也不能說（對空集合的全稱宣稱），見 `SessionAuditConsole` 的 `hasAuditRecords`。
	 * ⚠️ 措辭刻意不用「寫入」：同頁「變更摘要」欄的「未寫入」講的是領域寫入，主語不同。
	 */
	pendingAuditCount: number;
	notifiedSeller: boolean;
	/**
	 * 結構恆為 0（代行 capability 封閉列舉裡沒有履約／財務能力），也正是
	 * 「未執行任何出貨或財務操作」那句宣稱的資料來源。⚠️ 非零＝三層鎖定被打穿。
	 * 它與 `pendingAuditCount` **不同源、不得連動**。
	 */
	fulfillmentActionCount: number;
	financeActionCount: number;
	/** 單位是天（設定值下發），前端不得把「2 年」寫死。 */
	auditRetentionDays: number;
	/**
	 * 撤銷四件套。未撤銷時後端回 JSON null（gateway 明言不得 omitempty）。
	 *
	 * 缺欄（undefined）與 null 一律當成「沒有」：ADM 與 gateway 各自 promote，要求它們
	 * 存在會讓兩者之間那段窗口整份摘要 fail closed —— 為了三個補充欄位把整張稽核頁擋掉，
	 * 代價不對等。型別錯仍舊整份失敗（同 `bizCodeLabel` 的理由）。
	 *
	 * ⚠️ `revokedByAdminDisplayName` 填的是 `admin_accounts.email` —— 該表沒有顯示名
	 * 欄位，與 `actor.displayName` 同一個來源。顯示時的標籤不得寫「姓名」。
	 */
	revokedAt: string | null;
	revokedByAdminId: string | null;
	revokedByAdminDisplayName: string | null;
	revokeReason: string | null;
}

export interface DelegatedSessionRevocation {
	sessionId: string;
	status: DelegatedSessionStatus;
	revision: number;
	/** 冪等重放時是第一次撤銷的時刻，不是 now()。 */
	revokedAt: string;
}

export interface DelegatedAuditLogQuery {
	page: number;
	pageSize: number;
	action: DelegatedAuditAction | "";
	resourceType: DelegatedAuditResourceType | "";
	result: DelegatedAuditResult | "";
	dateFrom: string;
	dateTo: string;
}

/**
 * 跨工作階段稽核查詢（`GET /api/v1/admin/delegated-operations/audit-logs`，56 §11.3 P1）。
 *
 * 比 per-session 版多兩個**精確比對**欄：`actorAdminId`（這位管理員做過什麼）與
 * `storeId`（這家店被動過什麼）。兩者都是可選、可同時給。
 *
 * 與 per-session 版**共用同一個回應形狀**（同一份 parseDelegatedAuditLogPage），差別
 * 只在「查哪些列」——這是後端刻意的設計，前端不得為它另建 parser。
 */
export interface DelegatedAllAuditLogQuery extends DelegatedAuditLogQuery {
	actorAdminId: string;
	storeId: string;
}

type UnknownRecord = Record<string, unknown>;

/**
 * ⚠️ 以下解析原語（`isRecord`／`parseTimestamp`／`parseNullableTimestamp`／
 * `parseCountString`／`parseText`／`parseNullableText`／`isOneOf`）對外 export，
 * 供同 feature 的 `authorization.ts`（代行**授權列**治理，backend#2444）使用。
 *
 * 抄一份過去會得到兩套「什麼叫合法的時間／計數／自由文字」，而漂移的症狀是同一份
 * 上游回應在一個面板上通過、在另一個面板上被判契約錯——兩邊各自單看都正確。
 * 授權面與稽核面同屬治理視角、對同一個 gateway 群組作答，共用一份原語是對的。
 */
export function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * gateway 的 `delegatedVersionRE`。工作階段 ID、操作者 admin id、店家 store_id 三者
 * 共用同一組值域（都是 TSID 數字字串），但**主語不同**，所以留兩個名字：混用時讀的人
 * 會以為某處把 admin id 當成了 session id 在用。
 */
export function isDelegatedNumericId(value: unknown): value is string {
	return typeof value === "string" && /^\d{1,19}$/u.test(value);
}

/** `sessionId` 是 TSID 數字字串（gateway 的 `^\d{1,19}$`）。 */
export function isDelegatedSessionId(value: unknown): value is string {
	return isDelegatedNumericId(value);
}

/**
 * 工作階段代號（`OPS-` ＋ 13 碼 Crockford base32）。
 *
 * 三處同一份形狀：DB 的 `chk_dos_display_code_format`、account-service 的
 * `domain.IsValidDisplayCode`、gateway 列表端點的 `delegatedDisplayCodeRE`。gateway
 * 對格式錯的代號回 9260 而不是放行 —— 未驗證的錯字會換來一個空白頁，讀起來像「沒有
 * 這個工作階段」而不是「你打錯了」，而前端這一份存在的理由完全相同。
 */
export function isDelegatedDisplayCode(value: unknown): value is string {
	return typeof value === "string" && /^OPS-[0-9A-Z]{13}$/u.test(value);
}

export type DelegatedLookupIntent =
	| { kind: "session"; sessionId: string }
	| { kind: "displayCode"; displayCode: string }
	| { kind: "invalid" };

/**
 * 入口頁那個查詢框收到一串字之後該做什麼。**兩種識別碼都要收**，因為兩者都是操作者
 * 手上真的會有的東西：數字 ID 出現在詳情頁網址與維運 SQL，`OPS-` 代號出現在工單、
 * 稽核截圖與治理台的摘要卡（那張卡顯示的就是代號，複製貼過來必須查得到）。
 *
 * 兩者的下一步不同，所以不能合成一種：數字 ID 唯一指向一個工作階段 ⇒ 直接跳詳情頁；
 * 代號則是列表的一個**篩選軸**（gateway 的 `display_code` 是精確比對，但仍走列表端點，
 * 回的是一頁而不是一筆）。
 *
 * 大小寫在這裡正規化而不是拒絕：代號的權威形態是全大寫，而從 log 或聊天視窗貼過來的
 * 常是小寫。拒絕它只會讓人以為代號打錯了。
 */
export function resolveDelegatedLookup(raw: string): DelegatedLookupIntent {
	const trimmed = raw.trim();
	// 兩個候選都先算出來：`isDelegatedSessionId` 是型別謂詞，在它之後 `trimmed` 會被
	// 窄化成 never，那時再取 `toUpperCase()` 是編譯錯。
	const upper = trimmed.toUpperCase();
	if (isDelegatedSessionId(trimmed)) {
		return { kind: "session", sessionId: trimmed };
	}
	if (isDelegatedDisplayCode(upper)) {
		return { kind: "displayCode", displayCode: upper };
	}
	return { kind: "invalid" };
}

/** 撤銷理由：trim 後 1–200 字、無控制字元（gateway 同一組值域）。 */
export function isDelegatedRevokeReason(value: unknown): value is string {
	return (
		typeof value === "string" &&
		value.trim().length >= 1 &&
		[...value.trim()].length <= 200 &&
		!hasAsciiControlCharacters(value)
	);
}

/** 只接受標準 UTC RFC 3339；帶偏移或缺 `Z` 的值一律拒絕。 */
export function parseTimestamp(value: unknown): string | null {
	if (
		typeof value !== "string" ||
		!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/u.test(value) ||
		!Number.isFinite(Date.parse(value))
	) {
		return null;
	}
	return value;
}

export function parseNullableTimestamp(value: unknown): {
	ok: boolean;
	value: string | null;
} {
	if (value === null || value === undefined) return { ok: true, value: null };
	const parsed = parseTimestamp(value);
	return parsed === null
		? { ok: false, value: null }
		: { ok: true, value: parsed };
}

/**
 * 「數字對外一律 string」的一般情形：`total`／各種計數過線時都是十進位字串。
 * 空字串與帶正負號的值一律拒絕 —— `Number("")` 是 0，靜默當成零筆是最糟的失敗。
 *
 * ⚠️ 也接受非負安全整數，因為**同一份資料會被解析兩次**：BFF 解析上游、瀏覽器再
 * 解析 BFF 的回應，而第二次看到的是第一次的產物（已經是 number）。parser 不冪等
 * 的症狀是治理頁永遠顯示「摘要格式不完整」，而兩邊各自單看都正確。
 */
export function parseCountString(value: unknown): number | null {
	if (typeof value === "number") {
		return Number.isSafeInteger(value) && value >= 0 ? value : null;
	}
	if (typeof value !== "string" || !/^\d{1,15}$/u.test(value)) return null;
	const parsed = Number(value);
	return Number.isSafeInteger(parsed) ? parsed : null;
}

/** 自由文字欄位（後端無值時是空字串，不是 null）。 */
export function parseText(value: unknown, maxLength: number): string | null {
	if (typeof value !== "string" || value.length > maxLength) return null;
	return hasAsciiControlCharacters(value) ? null : value;
}

export function parseNullableText(
	value: unknown,
	maxLength: number,
): { ok: boolean; value: string | null } {
	if (value === null || value === undefined) return { ok: true, value: null };
	const parsed = parseText(value, maxLength);
	return parsed === null
		? { ok: false, value: null }
		: { ok: true, value: parsed };
}

function parseAuditActor(value: unknown): DelegatedAuditActor | null {
	if (!isRecord(value)) return null;
	const displayName = parseText(value.displayName, 160);
	if (!isSafeOpaqueId(value.adminId) || displayName === null) return null;
	return { adminId: value.adminId, displayName };
}

function parseAuditTarget(value: unknown): DelegatedAuditTarget | null {
	if (!isRecord(value)) return null;
	const storeName = parseText(value.storeName, 160);
	if (
		!isSafeOpaqueId(value.storeId) ||
		!isSafeOpaqueId(value.sellerAccountId) ||
		storeName === null
	) {
		return null;
	}
	return {
		storeId: value.storeId,
		sellerAccountId: value.sellerAccountId,
		storeName,
	};
}

function parseChangedFields(value: unknown): DelegatedChangedField[] | null {
	if (value === null || value === undefined) return [];
	if (!Array.isArray(value)) return null;
	const parsed: DelegatedChangedField[] = [];
	for (const raw of value) {
		if (!isRecord(raw)) return null;
		const field = parseText(raw.field, 60);
		const label = parseText(raw.label, 60);
		const before = parseText(raw.before, 400);
		const after = parseText(raw.after, 400);
		if (
			field === null ||
			field.length === 0 ||
			label === null ||
			before === null ||
			after === null
		) {
			return null;
		}
		parsed.push({ field, label, before, after });
	}
	return parsed;
}

export function isOneOf<T extends string>(
	value: unknown,
	allowed: readonly T[],
): value is T {
	return (
		typeof value === "string" && (allowed as readonly string[]).includes(value)
	);
}

/**
 * ⚠️ 單列不合契約時整頁失敗，**不是**跳過那一列。
 *
 * 稽核紀錄是爭議時的舉證：靜默丟掉一列之後，畫面上剩下的每一列看起來都正常，
 * 而少掉的那一列正是最可能不正常的那一列。整頁報錯至少讓人知道要去查。
 */
export function parseDelegatedAuditLogEntry(
	value: unknown,
): DelegatedAuditLogEntry | null {
	if (!isRecord(value)) return null;
	const actor = parseAuditActor(value.actor);
	const target = parseAuditTarget(value.target);
	const createdAt = parseTimestamp(value.createdAt);
	const confirmedAt = parseNullableTimestamp(value.confirmedAt);
	const notifiedAt = parseNullableTimestamp(value.notifiedAt);
	const caseReference = parseNullableText(value.caseReference, 200);
	const changedFields = parseChangedFields(value.changedFields);
	const actionLabel = parseText(value.actionLabel, 60);
	// 缺欄視同「沒有說明」而不是契約違反：這一欄是後端 deploy 後才出現的，而 ADM 與
	// gateway 各自 promote。要求它存在，會讓兩者之間的那段窗口整頁報錯——為了一行
	// 補充說明把整份稽核紀錄擋掉，代價不對等。型別錯仍舊整頁失敗。
	// 長度上限取 400（同 changedFields 的前後值）：錯誤碼表最長的一條約 120 字。
	const bizCodeLabel = parseNullableText(value.bizCodeLabel, 400);
	const resourceName = parseText(value.resourceName, 160);
	const resourceImageKey = parseText(value.resourceImageKey, 255);
	const operationReason = parseText(value.operationReason, 100);
	const userAgent = parseText(value.userAgent, 512);
	const idempotencyKey = parseText(value.idempotencyKey, 160);
	const requestId = parseText(value.requestId, 64);
	const correlationId = parseText(value.correlationId, 64);
	const origin = parseText(value.origin, 20);
	const resourceId = parseText(value.resourceId, 64);
	const versionBefore = parseText(value.resourceVersionBefore, 32);
	const versionAfter = parseText(value.resourceVersionAfter, 32);
	if (
		!isSafeOpaqueId(value.auditLogId) ||
		!actor ||
		!target ||
		createdAt === null ||
		!confirmedAt.ok ||
		!notifiedAt.ok ||
		!caseReference.ok ||
		!bizCodeLabel.ok ||
		changedFields === null ||
		actionLabel === null ||
		resourceName === null ||
		resourceImageKey === null ||
		operationReason === null ||
		userAgent === null ||
		idempotencyKey === null ||
		requestId === null ||
		correlationId === null ||
		origin === null ||
		resourceId === null ||
		versionBefore === null ||
		versionAfter === null ||
		!isOneOf(value.action, DELEGATED_AUDIT_ACTIONS) ||
		!isOneOf(value.riskLevel, DELEGATED_AUDIT_RISK_LEVELS) ||
		!isOneOf(value.resourceType, DELEGATED_AUDIT_RESOURCE_TYPES) ||
		!isOneOf(value.result, DELEGATED_AUDIT_RESULTS) ||
		typeof value.capability !== "string" ||
		!Number.isSafeInteger(value.bizCode)
	) {
		return null;
	}
	return {
		auditLogId: value.auditLogId,
		createdAt,
		confirmedAt: confirmedAt.value,
		actor,
		target,
		action: value.action,
		actionLabel,
		riskLevel: value.riskLevel,
		capability: value.capability,
		resourceType: value.resourceType,
		resourceId,
		resourceName,
		resourceImageKey,
		operationReason,
		caseReference: caseReference.value,
		result: value.result,
		bizCode: value.bizCode as number,
		bizCodeLabel: bizCodeLabel.value ?? "",
		resourceVersionBefore: versionBefore,
		resourceVersionAfter: versionAfter,
		changedFields,
		correlationId,
		requestId,
		idempotencyKey,
		origin,
		userAgent,
		notifiedAt: notifiedAt.value,
	};
}

export function parseDelegatedAuditLogPage(
	value: unknown,
): DelegatedAuditLogPage | null {
	if (!isRecord(value) || !Array.isArray(value.records)) return null;
	const total = parseCountString(value.total);
	const page = parseCountString(value.page);
	const pageSize = parseCountString(value.pageSize);
	if (
		total === null ||
		page === null ||
		pageSize === null ||
		page < 1 ||
		pageSize < 1 ||
		pageSize > DELEGATED_AUDIT_MAX_PAGE_SIZE
	) {
		return null;
	}
	const records: DelegatedAuditLogEntry[] = [];
	for (const raw of value.records) {
		const entry = parseDelegatedAuditLogEntry(raw);
		if (!entry) return null;
		records.push(entry);
	}
	return { records, total, page, pageSize };
}

export function parseDelegatedSessionReport(
	value: unknown,
): DelegatedSessionReport | null {
	if (!isRecord(value)) return null;
	const target = parseAuditTarget(value.target);
	const actor = parseAuditActor(value.actor);
	const issuedAt = parseTimestamp(value.issuedAt);
	const endedAt = parseNullableTimestamp(value.endedAt);
	const displayCode = parseText(value.displayCode, 40);
	const durationSeconds = parseCountString(value.durationSeconds);
	const operationCount = parseCountString(value.operationCount);
	const riskOperationCount = parseCountString(value.riskOperationCount);
	const pendingAuditCount = parseCountString(value.pendingAuditCount);
	const fulfillmentActionCount = parseCountString(value.fulfillmentActionCount);
	const financeActionCount = parseCountString(value.financeActionCount);
	const auditRetentionDays = parseCountString(value.auditRetentionDays);
	const revokedAt = parseNullableTimestamp(value.revokedAt);
	const revokedByAdminId = parseNullableText(value.revokedByAdminId, 128);
	const revokedByAdminDisplayName = parseNullableText(
		value.revokedByAdminDisplayName,
		160,
	);
	const revokeReason = parseNullableText(value.revokeReason, 200);
	if (
		!isDelegatedSessionId(value.sessionId) ||
		!target ||
		!actor ||
		issuedAt === null ||
		!endedAt.ok ||
		displayCode === null ||
		displayCode.length === 0 ||
		durationSeconds === null ||
		operationCount === null ||
		riskOperationCount === null ||
		pendingAuditCount === null ||
		fulfillmentActionCount === null ||
		financeActionCount === null ||
		auditRetentionDays === null ||
		auditRetentionDays < 1 ||
		!revokedAt.ok ||
		!revokedByAdminId.ok ||
		!revokedByAdminDisplayName.ok ||
		!revokeReason.ok ||
		!isOneOf(value.status, DELEGATED_SESSION_STATUSES) ||
		typeof value.notifiedSeller !== "boolean"
	) {
		return null;
	}
	return {
		sessionId: value.sessionId,
		displayCode,
		target,
		actor,
		issuedAt,
		endedAt: endedAt.value,
		durationSeconds,
		status: value.status,
		operationCount,
		riskOperationCount,
		pendingAuditCount,
		notifiedSeller: value.notifiedSeller,
		fulfillmentActionCount,
		financeActionCount,
		auditRetentionDays,
		revokedAt: revokedAt.value,
		revokedByAdminId: revokedByAdminId.value,
		revokedByAdminDisplayName: revokedByAdminDisplayName.value,
		revokeReason: revokeReason.value,
	};
}

export function parseDelegatedSessionRevocation(
	value: unknown,
): DelegatedSessionRevocation | null {
	if (!isRecord(value)) return null;
	const revokedAt = parseTimestamp(value.revokedAt);
	if (
		!isDelegatedSessionId(value.sessionId) ||
		revokedAt === null ||
		!isOneOf(value.status, DELEGATED_SESSION_STATUSES) ||
		!Number.isSafeInteger(value.revision) ||
		(value.revision as number) < 0
	) {
		return null;
	}
	return {
		sessionId: value.sessionId,
		status: value.status,
		revision: value.revision as number,
		revokedAt,
	};
}

/**
 * 把使用者選的篩選轉成 gateway 的 query。空字串代表不篩選，因此**不送出**該參數：
 * 送一個空值過去會被值域檢查判為不合法而整支回 9260。
 */
export function buildDelegatedAuditLogSearch(
	query: Partial<DelegatedAuditLogQuery>,
): URLSearchParams {
	const search = new URLSearchParams();
	search.set("page", String(query.page && query.page > 0 ? query.page : 1));
	const pageSize =
		query.pageSize && query.pageSize > 0
			? Math.min(query.pageSize, DELEGATED_AUDIT_MAX_PAGE_SIZE)
			: 20;
	search.set("page_size", String(pageSize));
	if (isOneOf(query.action, DELEGATED_AUDIT_ACTIONS)) {
		search.set("action", query.action);
	}
	if (isOneOf(query.resourceType, DELEGATED_AUDIT_RESOURCE_TYPES)) {
		search.set("resource_type", query.resourceType);
	}
	if (isOneOf(query.result, DELEGATED_AUDIT_RESULTS)) {
		search.set("result", query.result);
	}
	if (query.dateFrom) search.set("date_from", query.dateFrom);
	if (query.dateTo) search.set("date_to", query.dateTo);
	return search;
}

// ==================== 工作階段列表（56 §11.3 P1） ====================

/**
 * `GET /api/v1/admin/delegated-operations` 的一列。
 *
 * 與 `DelegatedSessionReport`（單一工作階段的摘要）是兩種形狀，不可互相冒充：摘要回
 * 的是**算出來的統計**（操作數、風險數、未確認數、保留天數），列表回的是**這個工作
 * 階段是誰在什麼授權下開的**。把列表列餵給摘要 parser 會整頁 fail closed，反之則會在
 * 畫面上顯示一排 0。
 */
export interface DelegatedSessionReason {
	code: string;
	label: string;
}

export interface DelegatedOperationSessionRecord {
	sessionId: string;
	displayCode: string;
	target: DelegatedAuditTarget;
	actor: DelegatedAuditActor;
	reason: DelegatedSessionReason;
	caseReference: string | null;
	capabilities: string[];
	issuedAt: string;
	expiresAt: string;
	endedAt: string | null;
	/**
	 * ⚠️ `expired` 是 account-service 讀取當下以**單一** now() 對整頁逐列推導的，DB 只
	 * 存 active／revoked。前端不得自己拿 `expiresAt` 與瀏覽器時間再推一次：那會讓同一
	 * 列在列表與詳情頁上各有一個狀態，而讀的人正在比對時序。
	 */
	status: DelegatedSessionStatus;
	revokedAt: string | null;
	revokedByAdminId: string | null;
	revokedByAdminDisplayName: string | null;
	revokeReason: string | null;
}

export interface DelegatedOperationSessionPage {
	records: DelegatedOperationSessionRecord[];
	total: number;
	page: number;
	pageSize: number;
}

export interface DelegatedSessionListQuery {
	page: number;
	pageSize: number;
	actorAdminId: string;
	storeId: string;
	displayCode: string;
	dateFrom: string;
	dateTo: string;
}

function parseSessionReason(value: unknown): DelegatedSessionReason | null {
	if (!isRecord(value)) return null;
	const code = parseText(value.code, 40);
	const label = parseText(value.label, 60);
	if (code === null || code.length === 0 || label === null) return null;
	return { code, label };
}

/**
 * 能力清單只驗形狀，不驗值域。
 *
 * `policy.ts` 那支 `parseDelegatedOperationCapabilities` 會丟掉未知值、依賴不齊時整份
 * 作廢——那是**要進工作區之前**的 fail-closed。這裡是治理回看：一列歷史授權若含有
 * ADM 還不認得的能力（後端加了新能力、ADM 尚未跟上），把它丟掉等於在稽核畫面上少報
 * 代行者拿過的權限，而那正是回看的人最需要看到的東西。
 */
function parseSessionCapabilities(value: unknown): string[] | null {
	if (!Array.isArray(value)) return null;
	const parsed: string[] = [];
	for (const raw of value) {
		const capability = parseText(raw, 64);
		if (capability === null || capability.length === 0) return null;
		parsed.push(capability);
	}
	return parsed;
}

export function parseDelegatedOperationSessionRecord(
	value: unknown,
): DelegatedOperationSessionRecord | null {
	if (!isRecord(value)) return null;
	const target = parseAuditTarget(value.target);
	const actor = parseAuditActor(value.actor);
	const reason = parseSessionReason(value.reason);
	const capabilities = parseSessionCapabilities(value.capabilities);
	const displayCode = parseText(value.displayCode, 40);
	const issuedAt = parseTimestamp(value.issuedAt);
	const expiresAt = parseTimestamp(value.expiresAt);
	const endedAt = parseNullableTimestamp(value.endedAt);
	const revokedAt = parseNullableTimestamp(value.revokedAt);
	const caseReference = parseNullableText(value.caseReference, 200);
	const revokedByAdminId = parseNullableText(value.revokedByAdminId, 128);
	const revokedByAdminDisplayName = parseNullableText(
		value.revokedByAdminDisplayName,
		160,
	);
	const revokeReason = parseNullableText(value.revokeReason, 200);
	if (
		!isDelegatedSessionId(value.sessionId) ||
		!target ||
		!actor ||
		!reason ||
		capabilities === null ||
		displayCode === null ||
		!isDelegatedDisplayCode(displayCode) ||
		issuedAt === null ||
		expiresAt === null ||
		!endedAt.ok ||
		!revokedAt.ok ||
		!caseReference.ok ||
		!revokedByAdminId.ok ||
		!revokedByAdminDisplayName.ok ||
		!revokeReason.ok ||
		!isOneOf(value.status, DELEGATED_SESSION_STATUSES)
	) {
		return null;
	}
	return {
		sessionId: value.sessionId,
		displayCode,
		target,
		actor,
		reason,
		caseReference: caseReference.value,
		capabilities,
		issuedAt,
		expiresAt,
		endedAt: endedAt.value,
		status: value.status,
		revokedAt: revokedAt.value,
		revokedByAdminId: revokedByAdminId.value,
		revokedByAdminDisplayName: revokedByAdminDisplayName.value,
		revokeReason: revokeReason.value,
	};
}

/**
 * ⚠️ 同 `parseDelegatedAuditLogPage`：單列不合契約時整頁失敗，不是跳過那一列。
 *
 * 這一頁是「這段期間誰代行了哪些店」的全集。靜默丟掉一列之後，剩下的每一列看起來
 * 都正常，而少掉的那一列正是最可能不正常的那一列。
 */
export function parseDelegatedOperationSessionPage(
	value: unknown,
): DelegatedOperationSessionPage | null {
	if (!isRecord(value) || !Array.isArray(value.records)) return null;
	const total = parseCountString(value.total);
	const page = parseCountString(value.page);
	const pageSize = parseCountString(value.pageSize);
	if (
		total === null ||
		page === null ||
		pageSize === null ||
		page < 1 ||
		pageSize < 1 ||
		pageSize > DELEGATED_AUDIT_MAX_PAGE_SIZE
	) {
		return null;
	}
	const records: DelegatedOperationSessionRecord[] = [];
	for (const raw of value.records) {
		const record = parseDelegatedOperationSessionRecord(raw);
		if (!record) return null;
		records.push(record);
	}
	return { records, total, page, pageSize };
}

/**
 * 同 `buildDelegatedAuditLogSearch`：空字串代表不篩選，因此**不送出**該參數 ——
 * 送一個空值過去會被 gateway 的值域檢查判為 9260。
 *
 * 格式不合的識別碼也不送：gateway 對它回 9260（`delegationIDFilter` 明言「不得靜默
 * 丟棄」），而使用者一邊打字一邊查詢的中途狀態必然有半截的值，每一次按鍵都換來一次
 * 400 沒有意義。真正的守門在後端，這裡只是不要拿一個必被拒絕的值去問。
 */
export function buildDelegatedSessionSearch(
	query: Partial<DelegatedSessionListQuery>,
): URLSearchParams {
	const search = new URLSearchParams();
	search.set("page", String(query.page && query.page > 0 ? query.page : 1));
	const pageSize =
		query.pageSize && query.pageSize > 0
			? Math.min(query.pageSize, DELEGATED_AUDIT_MAX_PAGE_SIZE)
			: 20;
	search.set("page_size", String(pageSize));
	if (isDelegatedNumericId(query.actorAdminId)) {
		search.set("actor_admin_id", query.actorAdminId);
	}
	if (isDelegatedNumericId(query.storeId)) {
		search.set("store_id", query.storeId);
	}
	if (isDelegatedDisplayCode(query.displayCode)) {
		search.set("display_code", query.displayCode);
	}
	if (query.dateFrom) search.set("date_from", query.dateFrom);
	if (query.dateTo) search.set("date_to", query.dateTo);
	return search;
}

/**
 * 跨工作階段稽核查詢的 query string。
 *
 * 建在 buildDelegatedAuditLogSearch 之上而不是複製一份：兩者的 page／page_size／三個
 * 篩選／兩個時間欄必須逐字相同，否則同一組條件在兩個畫面上會查出不同的列，而那個
 * 差異不會有任何東西轉紅。
 *
 * ⚠️ 兩個 id 欄只在**非空**時送出。送空字串會讓後端把它當成一個要精確比對的值，
 * 於是「不篩選」變成「比對空字串」⇒ 永遠零筆，而畫面上與「這家店真的沒被動過」
 * 長得一模一樣。
 */
export function buildDelegatedAllAuditLogSearch(
	query: Partial<DelegatedAllAuditLogQuery>,
): URLSearchParams {
	const search = buildDelegatedAuditLogSearch(query);
	if (query.actorAdminId) search.set("actor_admin_id", query.actorAdminId);
	if (query.storeId) search.set("store_id", query.storeId);
	return search;
}

/** 跨工作階段稽核的 react-query key。理由同 delegatedSessionListQueryKey。 */
export function delegatedAllAuditLogQueryKey(
	query: Partial<DelegatedAllAuditLogQuery>,
): readonly [string, string, string] {
	return [
		"delegated-governance",
		"all-audit-logs",
		buildDelegatedAllAuditLogSearch(query).toString(),
	] as const;
}

/**
 * 列表查詢的 react-query key。
 *
 * 直接用送出去的 query string 當 key，而不是另外攤一個物件：兩者若各寫一份，篩選改了
 * 但 key 沒跟著改的那一刻，畫面會拿快取裡上一組條件的結果當新結果顯示 —— 而它看起來
 * 完全正常（有資料、有分頁、只是內容是舊的）。
 */
export function delegatedSessionListQueryKey(
	query: Partial<DelegatedSessionListQuery>,
): readonly [string, string, string] {
	return [
		"delegated-governance",
		"sessions",
		buildDelegatedSessionSearch(query).toString(),
	] as const;
}
