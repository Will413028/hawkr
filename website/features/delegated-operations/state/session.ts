/**
 * 代行工作階段的狀態推導（56 號 §4.5 L1033）。
 *
 * 由 `hawkr-admin` 的 `src/features/delegated-operations/model.ts` 搬移
 * 「狀態推導」那一半而來，**parser 與封閉列舉留在 admin**：那裡才有 create 流程
 * （preflight → create → current 的九欄對帳），需要嚴格的 fail-closed 解析；
 * 這裡只消費 `GET current` 下發的 summary，對未知 capability 一律忽略而非整包
 * 拒絕（與 admin `policy.ts` 的同一裁決同源）。
 *
 * ⚠️ 這份檔案不得引入任何 React／DOM 依賴：`/adm/enter` 這支 route handler 也讀它。
 */

/**
 * capability 在 website 側刻意是開放字串而非封閉聯集：後端新增第 10 個
 * capability 時，工作區只會少畫一顆 chip，不會整個 502。
 */
export type DelegatedCapability = string;

export interface DelegatedSessionTarget {
	storeId: string;
	/** 該店家的登入 principal（accounts PK），不是營運鍵。 */
	sellerAccountId: string;
	storeName: string;
}

export interface DelegatedSessionActor {
	adminId: string;
	displayName: string;
}

export interface DelegatedSessionReason {
	code: string;
	label: string;
	caseReferenceRequired?: boolean;
}

export type DelegatedSessionStatus = "active" | "expired" | "revoked";

/**
 * `GET /adm/api/session/current` → gateway `GET /admin/delegated-operations/current`
 * 的對外投影。這裡永遠不會有 operation credential（它只存在於 httpOnly cookie）。
 */
export interface DelegatedSessionSummary {
	sessionId: string;
	/** `OPS-` ＋ 13 碼大寫 base36；人類可讀的對帳鍵，不是排序鍵。 */
	displayCode: string;
	target: DelegatedSessionTarget;
	actor: DelegatedSessionActor;
	reason: DelegatedSessionReason;
	caseReference: string | null;
	capabilities: DelegatedCapability[];
	issuedAt: string;
	expiresAt: string;
	status: DelegatedSessionStatus;
	/** JSON number（值域安全例外）。存續期間恆為 0，不得拿它做 refetch 判斷。 */
	revision: number;
}

export type DelegatedSessionState =
	| "active"
	| "expiring"
	| "expired"
	| "revoked"
	| "end_unknown"
	| "target_mismatch"
	| "invalid";

export const DELEGATED_OPERATION_MAX_TTL_SECONDS = 30 * 60;
export const DELEGATED_OPERATION_EXPIRING_SECONDS = 5 * 60;
export const DELEGATED_OPERATION_CLOCK_SKEW_SECONDS = 60;

/**
 * 工作階段終態的業務碼（§6.1.4 終態表）。工作區內任一代行請求收到這三個碼即
 * 立刻切阻擋畫面並清掉 website 的 credential cookie。
 */
export const DELEGATED_TERMINAL_BIZ_CODES: Record<
	number,
	DelegatedSessionState
> = {
	9270: "expired",
	9271: "revoked",
	9272: "revoked",
};

/** 交接碼兌換失敗；四種成因共用同一個碼是刻意的（§4.5 S6）。 */
export const DELEGATION_HANDOFF_INVALID_CODE = 9282;

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function nonEmptyString(value: unknown): string | null {
	return typeof value === "string" && value.length > 0 ? value : null;
}

/**
 * 形狀檢查（不是 admin 那份 fail-closed parser）。
 *
 * 它只回答「這份 JSON 撐得起工作區的 chrome 嗎」：少了倒數要的時間戳、少了
 * 標題要的店名，畫面就會出現 `undefined`，那比切阻擋畫面更難除錯。值域與
 * 封閉列舉的權威在後端，前端多做一份只會在後端擴充時整包壞掉。
 */
export function readDelegatedSessionSummary(
	value: unknown,
): DelegatedSessionSummary | null {
	if (!isRecord(value)) return null;
	const target = isRecord(value.target) ? value.target : null;
	const actor = isRecord(value.actor) ? value.actor : null;
	const reason = isRecord(value.reason) ? value.reason : null;
	const storeId = nonEmptyString(target?.storeId);
	const storeName = nonEmptyString(target?.storeName);
	const adminId = nonEmptyString(actor?.adminId);
	const issuedAt = nonEmptyString(value.issuedAt);
	const expiresAt = nonEmptyString(value.expiresAt);
	const status = value.status;
	if (
		!target ||
		!actor ||
		!reason ||
		!storeId ||
		!storeName ||
		!adminId ||
		!issuedAt ||
		!expiresAt ||
		!Array.isArray(value.capabilities) ||
		(status !== "active" && status !== "expired" && status !== "revoked") ||
		!Number.isSafeInteger(value.revision) ||
		(value.revision as number) < 0
	) {
		return null;
	}
	return {
		sessionId: nonEmptyString(value.sessionId) ?? "",
		displayCode: nonEmptyString(value.displayCode) ?? "",
		target: {
			storeId,
			sellerAccountId: nonEmptyString(target.sellerAccountId) ?? "",
			storeName,
		},
		actor: {
			adminId,
			displayName: nonEmptyString(actor.displayName) ?? "",
		},
		reason: {
			code: nonEmptyString(reason.code) ?? "",
			label: nonEmptyString(reason.label) ?? "",
			...(typeof reason.caseReferenceRequired === "boolean"
				? { caseReferenceRequired: reason.caseReferenceRequired }
				: {}),
		},
		caseReference: nonEmptyString(value.caseReference),
		capabilities: value.capabilities.filter(
			(capability): capability is string => typeof capability === "string",
		),
		issuedAt,
		expiresAt,
		status,
		revision: value.revision as number,
	};
}

/**
 * 工作階段身分：兩次工作階段之間必須完全隔離快取（§4.5 定案第 4 條）。
 *
 * `revision` 對隔離的實際貢獻是零（存續期間恆為 0），保留是因為它讓「工作階段
 * 身分」是完整表達，日後 bump 規則放寬時 key 不必回頭改。
 */
export interface DelegatedIdentity {
	adminId: string;
	storeId: string;
	issuedAt: string;
	revision: number;
}

/**
 * summary 還沒回來時的佔位身分。
 *
 * `enabled: false` 的查詢仍然要一把鍵，而那把鍵**不能與任何真實工作階段相撞**
 * （真實 `adminId`／`storeId` 都是非空 TSID、`revision` 非負）—— 撞上就代表
 * 「還沒拿到 summary」的那一瞬間與某一家店共用了槽位。
 */
export const DELEGATED_PENDING_IDENTITY: DelegatedIdentity = {
	adminId: "",
	storeId: "",
	issuedAt: "",
	revision: -1,
};

export function delegatedIdentityOf(
	summary: DelegatedSessionSummary,
): DelegatedIdentity {
	return {
		adminId: summary.actor.adminId,
		storeId: summary.target.storeId,
		issuedAt: summary.issuedAt,
		revision: summary.revision,
	};
}

export function delegatedIdentityKey(identity: DelegatedIdentity): string {
	return [
		identity.adminId,
		identity.storeId,
		identity.issuedAt,
		String(identity.revision),
	].join(":");
}

export function deriveDelegatedSessionState(
	summary: DelegatedSessionSummary,
	expectedStoreId: string,
	now: number | Date = Date.now(),
): DelegatedSessionState {
	if (!expectedStoreId || !summary.target.storeId) return "invalid";
	if (summary.target.storeId !== expectedStoreId) return "target_mismatch";

	const expiresAt = Date.parse(summary.expiresAt);
	const issuedAt = Date.parse(summary.issuedAt);
	const nowEpoch = now instanceof Date ? now.getTime() : now;
	const lifetimeMs = expiresAt - issuedAt;
	if (
		!Number.isFinite(expiresAt) ||
		!Number.isFinite(issuedAt) ||
		!Number.isFinite(nowEpoch) ||
		lifetimeMs <= 0 ||
		lifetimeMs > DELEGATED_OPERATION_MAX_TTL_SECONDS * 1000 ||
		issuedAt > nowEpoch + DELEGATED_OPERATION_CLOCK_SKEW_SECONDS * 1000
	) {
		return "invalid";
	}
	if (summary.status === "revoked") return "revoked";
	if (summary.status === "expired" || expiresAt <= nowEpoch) return "expired";
	if (summary.status !== "active") return "invalid";
	if (expiresAt - nowEpoch <= DELEGATED_OPERATION_EXPIRING_SECONDS * 1000) {
		return "expiring";
	}
	return "active";
}

export function canRenderDelegatedWorkspace(
	state: DelegatedSessionState,
): boolean {
	return state === "active" || state === "expiring";
}

/**
 * 已經由**手上的 summary** 定案、不可能再變回別的東西的狀態。
 *
 * 一個到期的工作階段不會變回有效，一個目標不符的路由也不會自己對上。所以之後任何一次
 * 輪詢失敗都不得把它們降級成 `invalid`。
 */
const DELEGATED_SETTLED_STATES: ReadonlySet<DelegatedSessionState> = new Set([
	"expired",
	"revoked",
	"target_mismatch",
]);

/**
 * 工作階段狀態的**唯一**判定入口：把「輪詢失敗」與「summary 自己推導出來的狀態」
 * 依優先序合成一個結論。
 *
 * 為什麼不能直接「錯了就 invalid」（那是原本的寫法）：credential cookie 一旦不在，
 * BFF 回的是 `401 CREDENTIAL_MISSING`（字串碼，對不上終態數字表）⇒ 一個**已經到期**
 * 的工作階段會顯示成「目前無法確認代行工作階段」。到期是這個系統最尋常的結束方式，
 * 而它唯一該說的那句話反而永遠說不出口。
 *
 * 順序：
 * 1. 上游明講的終態業務碼（9270／9271／9272）—— 伺服器的判斷永遠優先。
 * 2. 手上的 summary 已經定案（到期／撤銷／目標不符）—— **傳輸失敗不會讓它變回未知**。
 * 3. 其餘失敗 → `invalid`（真的不知道）。
 * 4. 沒有失敗 → 照 summary 推導；連 summary 都沒有才是 `invalid`。
 */
export function resolveDelegatedSessionState(input: {
	summary: DelegatedSessionSummary | null;
	/** 最近一次 `GET current` 是否失敗。 */
	failed: boolean;
	/** 失敗時的業務碼（BFF 自己的碼是字串，Gateway 的是數字）。 */
	failureCode: number | string | null;
	expectedStoreId: string;
	now: number | Date;
}): DelegatedSessionState {
	const { summary, failed, failureCode, expectedStoreId, now } = input;
	const derived = summary
		? deriveDelegatedSessionState(summary, expectedStoreId, now)
		: null;
	if (failed) {
		const terminal =
			typeof failureCode === "number"
				? DELEGATED_TERMINAL_BIZ_CODES[failureCode]
				: undefined;
		if (terminal) return terminal;
		if (derived && DELEGATED_SETTLED_STATES.has(derived)) return derived;
		return "invalid";
	}
	return derived ?? "invalid";
}

/**
 * 左欄「操作紀錄」的目的地（§3871）。
 *
 * ⚠️ **必須是一個真的到得了的頁面。** 這個入口存在的理由就是「讓操作者查得到自己
 * 剛留下的稽核」，而稽核查詢掛 `requireAdmin()`，只能在 ADM host 上開。它曾經指向
 * 商家詳情頁上的 `#delegated-operation-audit` 錨點 —— admin repo 裡沒有任何元素帶
 * 這個 id，於是連結每次都只是把人丟回商家頁，且**畫面上完全看不出來**：瀏覽器對
 * 不存在的 fragment 不報錯，人只會以為稽核還沒寫進去。
 *
 * `sessionId` 還沒到手時退到查詢頁而不是照樣組一個 `/undefined`：少一步比死路好。
 */
export function delegatedAuditUrl(
	admOrigin: string,
	sessionId: string | null | undefined,
): string {
	const base = `${admOrigin}/admin/delegated-operations`;
	return sessionId ? `${base}/${encodeURIComponent(sessionId)}` : base;
}

/**
 * 訂單抽屜「平台異常處置」的目的地（產品需求 §6.7：「平台異常處置會**離開代行工作區
 * 並重新驗治理權限**」；56 號 §2.2 Out-of-scope #5 把「異常訂單處置」明確歸給 29 號的
 * 既有 admin 入口）。⇒ 這裡只組一條跨 host 連結，**不新增任何後端端點**。
 *
 * ⚠️ query 參數必須與 admin repo `/admin/anomalies` 讀的鍵**逐字相同**，否則就是
 * `delegatedAuditUrl` 上面那段修掉的壞法的翻版：頁面照樣 200、照樣渲染，只是帶過去
 * 的訂單完全沒有落點，而**畫面上看不出來**。對應實作在 hawkr-admin 的
 * `src/features/delegated-operations/anomaly-handoff.ts`，兩邊各有一支測試釘住鍵名。
 *
 * ⚠️ `store_id` 是這條連結有沒有用的關鍵。`GET /api/v1/admin/orders/abnormal`
 * **沒有 `order_no` 參數**（實查 gateway `admin_order.go` 的 `@Param` 清單只有
 * page／page_size／provider_status／status／buyer_id／store_id／payment_method／
 * shipping_method／created_from／created_to／sort）⇒ 伺服器端能收斂的最細粒度就是
 * 「這一家店的異常訂單」，訂單編號只能由 admin 端在當頁標出來。少帶 `store_id`
 * 會讓操作者落在全平台的異常清單上，那跟沒帶一樣。
 *
 * `status` 一併帶過去是因為該端點**未帶 status 時預設只回 35（物流異常）**：從一筆
 * 付款異常的訂單點過去，不帶 status 會落在一份查無此單的清單上。
 */
export function delegatedAnomalyUrl(
	admOrigin: string,
	handoff: {
		storeId: string | null | undefined;
		orderNo: string | null | undefined;
		status?: string | null;
	},
): string {
	const query = new URLSearchParams({ tab: "orders" });
	if (handoff.storeId) query.set("store_id", handoff.storeId);
	if (handoff.orderNo) query.set("order_no", handoff.orderNo);
	if (handoff.status) query.set("status", handoff.status);
	return `${admOrigin}/admin/anomalies?${query.toString()}`;
}

/** 倒數是本地推算；真正的終態偵測靠每 30 秒一次的 `GET current`（§3.2）。 */
export function formatRemainingDuration(
	expiresAt: string,
	now: number | Date = Date.now(),
): string {
	const expiryEpoch = Date.parse(expiresAt);
	const nowEpoch = now instanceof Date ? now.getTime() : now;
	if (!Number.isFinite(expiryEpoch) || !Number.isFinite(nowEpoch)) {
		return "--:--";
	}
	const remainingSeconds = Math.max(
		0,
		Math.ceil((expiryEpoch - nowEpoch) / 1000),
	);
	const minutes = Math.floor(remainingSeconds / 60);
	const seconds = remainingSeconds % 60;
	return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}
