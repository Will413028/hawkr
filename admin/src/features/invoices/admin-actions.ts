/**
 * ADM 發票模組 A03／A06／A08／A13 四頁**寫入動作**的共用常數與判定。
 *
 * ## 為什麼這一批在 2026-09-02 才接上
 *
 * 這四頁原本掛著一則「這一頁目前只能檢視」的告示，理由寫的是「invoice-service
 * 那一側整批沒有實作」。**那個判斷是錯的，而且錯在方法**：當時的查法是拿自己想
 * 當然的路徑名去打 dev，`complaints/{id}/decisions`、`jobs/{id}/notification_resends`、
 * `jobs/{id}/external_issuances` 三條都回 404，於是被讀成「後端沒做」。
 * 真名分別是 `complaints/{id}/actions`、`documents/{id}/notification_resends`、
 * `jobs/{id}/manual_results`（`gateway/internal/router/constants.go` 的
 * `AdminInvoiceComplaintActions`／`AdminInvoiceDocumentResend`／`AdminInvoiceJobManualResults`）。
 *
 * ⇒ **識別字一律取自路由註冊，不得取自記憶**。本檔每一條路徑下面都標了它在
 * `constants.go` 的常數名，改名時 grep 得到。
 *
 * 實作面同樣坐實：`services/invoice-service/internal/grpc/` 的 receiver 清單裡
 * `DecideTaxReviewForAdmin`／`DecideTaxExtensionForAdmin`／`RetryInvoiceJobForAdmin`／
 * `CancelInvoiceJobForAdmin`／`RequestManualUnlockForAdmin`／`ApproveManualUnlockForAdmin`／
 * `ReportManualResultForAdmin`／`ActOnInvoiceComplaintForAdmin`／
 * `ResendInvoiceNotificationForAdmin` 九支全在。
 *
 * ## 這裡放「判定」不放「送出」
 *
 * 送出要帶 `Idempotency-Key` header（gateway 的 `RequireIdempotencyKey` 掛在整個
 * `/admin/invoices` 群組上），而 `api.post` 不收自訂 header ⇒ 呼叫點直接用
 * `services/api.ts` 的 `request()`。那幾行留在頁面上，是為了讓「哪一顆鍵打哪一支端點」
 * 在頁面本身讀得到（沿用 A04 `provider-actions.ts` 的既有分工）。
 *
 * ## 這裡的碼值哪些是 CodeMeta、哪些不是
 *
 * - `SellerTaxReviewDecision`（端點 40）**是** CodeMeta，一律從 `@/lib/codemeta` 取。
 * - 端點 41 的 `APPROVE`／`REJECT`、端點 52 的 `fundingDecision`、端點 60 的五個
 *   `action`：三組都**不是** CodeMeta，它們是各端點自己的 API 值域，定義在
 *   invoice-service 的 Go 常數（見各段註解）。在這裡寫成字串常數不是「手抄碼表」，
 *   是照抄一份沒有碼表的契約——所以每一組都標了它在後端的檔案與常數名。
 */

import {
	SELLER_TAX_REVIEW_DECISION_LABEL,
	SellerTaxReviewDecision,
	SellerTaxReviewStatus,
} from "@/lib/codemeta";

// ---------------------------------------------------------------------------
// 端點路徑（逐一對應 gateway/internal/router/constants.go）
// ---------------------------------------------------------------------------

/** `AdminInvoiceTaxReviewID` 的前綴（端點 39／40／41）。 */
export const TAX_REVIEW_ENDPOINT = "/api/v1/admin/invoices/tax_reviews";
/** `AdminInvoiceJobID` 的前綴（端點 51／52／53／54／55／56）。 */
export const JOBS_ENDPOINT = "/api/v1/admin/invoices/jobs";
/** `AdminInvoiceComplaintID` 的前綴（端點 59／60）。 */
export const COMPLAINTS_ENDPOINT = "/api/v1/admin/invoices/complaints";
/** `AdminInvoiceDocumentID` 的前綴（端點 48／49）。 */
export const DOCUMENTS_ENDPOINT = "/api/v1/admin/invoices/documents";

/**
 * 這一次操作的冪等鍵。
 *
 * 格式要通過 gateway 的 `^[A-Za-z0-9._:-]{8,128}$`，而且會落進
 * `invoice_admin_audit_logs.request_id`（varchar(64)）。
 *
 * 🔴 **送出失敗時不換鍵**：操作可能已經生效而回應沒送到，換一把新的會被後端當成
 * 第二次操作。反過來，**收到回應之後**（不論 code 是 0 還是業務碼）下一次按鍵是新的
 * 操作，必須換一把——`uq_invadmaudit_client_request` 是
 * `(actor_account_id, action, request_id)` 的唯一索引，插入走 `ON CONFLICT DO NOTHING`
 * ⇒ 沿用同一把鍵的第二次操作**不會報錯，只會靜靜地不寫稽核列**。
 * （與 A04 的 `createProviderIdempotencyKey` 同一份規則，前綴不同。）
 */
export function createInvoiceAdminIdempotencyKey(): string {
	const uuid =
		typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
			? crypto.randomUUID()
			: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
	return `inv-adm-${uuid}`;
}

/** ADM 理由欄的上限（`admin_job.go` 的 `adminReasonMaxRunes`／稅務側同值）。 */
export const MAX_ADMIN_REASON_RUNES = 500;
/** 端點 56 備註欄的上限（`admin_job.go` 的 `adminNoteMaxRunes`）。 */
export const MAX_ADMIN_NOTE_RUNES = 500;
/** 端點 54 查證說明的上限（`admin_manual_unlock.go` 的 `unlockEvidenceMaxRunes`）。 */
export const MAX_UNLOCK_EVIDENCE_RUNES = 1000;

// ---------------------------------------------------------------------------
// A03　端點 40（審核裁決）
// ---------------------------------------------------------------------------

/**
 * 「決策 × 允許的來源狀態」，逐列等於 `tax_profile.go` 的 `taxDecisionAllowedFrom`。
 *
 * 🔴 **多一個值 ⇒ 一顆按下去必死的鍵**（該狀態下 CAS rows=0，後端回 9348）；
 * 少一個值 ⇒ 合法決策的按鈕不見了。兩種都不會有任何東西轉紅，所以有一支 gate
 * 逐列比對這張表（`tests/invoice-adm-contract.test.ts`）。
 *
 * ⚠️ `SUPPLEMENT_REQUIRED` 不在任何一列：賣家補齊後由 T06 先推回 `UNDER_REVIEW`。
 * ⚠️ `EXTEND` 是 `SellerTaxReviewDecision` 的第 8 值但**不是端點 40 的決策**——它只
 * 出現在端點 41 的稽核 `reason_code`。放進這張表就會多長出一顆打錯端點的按鈕。
 */
export const TAX_DECISION_ALLOWED_FROM: Record<string, readonly string[]> = {
	[SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED]: [
		SellerTaxReviewStatus.UNDER_REVIEW,
		SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	],
	[SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT]: [
		SellerTaxReviewStatus.UNDER_REVIEW,
		SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	],
	[SellerTaxReviewDecision.REJECT]: [
		SellerTaxReviewStatus.UNDER_REVIEW,
		SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	],
	[SellerTaxReviewDecision.REQUEST_SUPPLEMENT]: [
		SellerTaxReviewStatus.UNDER_REVIEW,
	],
	[SellerTaxReviewDecision.KEEP_UNDER_REVIEW]: [
		SellerTaxReviewStatus.UNDER_REVIEW,
	],
	[SellerTaxReviewDecision.REVOKE]: [
		SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED,
		SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
	],
	[SellerTaxReviewDecision.REOPEN_REVIEW]: [
		SellerTaxReviewStatus.APPROVED_INVOICE_REQUIRED,
		SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
		SellerTaxReviewStatus.REVOKED,
		SellerTaxReviewStatus.EXPIRED,
	],
};

/** 按鈕排列順序（決策卡的視覺順序，與值域無關）。 */
export const TAX_DECISION_ORDER: readonly string[] = [
	SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED,
	SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
	SellerTaxReviewDecision.REQUEST_SUPPLEMENT,
	SellerTaxReviewDecision.KEEP_UNDER_REVIEW,
	SellerTaxReviewDecision.REJECT,
	SellerTaxReviewDecision.REVOKE,
	SellerTaxReviewDecision.REOPEN_REVIEW,
];

/**
 * 需要二次確認的兩個決策（`tax_profile.go` 的 `taxDecisionNeedsConfirm`）。
 *
 * 🔴 缺 `confirmed` ⇒ 後端回 **400**（不是 9348）。二次確認是授權設計的一部分
 * （以「強制填理由 ＋ 二次確認 ＋ 完整稽核」取代細粒度 capability），不是 UX 裝飾。
 */
export const TAX_DECISION_NEEDS_CONFIRM: ReadonlySet<string> = new Set([
	SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
	SellerTaxReviewDecision.REVOKE,
]);

/** 會走 `Approve` 分支的兩個決策（可帶 `approvalValidUntil`）。 */
export const TAX_DECISION_APPROVALS: ReadonlySet<string> = new Set([
	SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED,
	SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
]);

/** 這個決策在目前狀態下成不成立。條件不成立時**不渲染**（不是 disabled）。 */
export function taxDecisionAvailable(
	decision: string,
	reviewStatus: string,
): boolean {
	return (TAX_DECISION_ALLOWED_FROM[decision] ?? []).includes(reviewStatus);
}

/**
 * 端點 29（賣家簽署收據代製授權）的 allowed-from，逐列等於 invoice-service 的
 * `receiptAuthAllowedFrom`。
 *
 * 🔴 **這張表 2026-09-05 才放寬**（`DEC-20260905-015836`）：原本只有
 * `APPROVED_INVOICE_EXEMPT`，於是「核定要先授權、授權要先核定」互相卡死。放寬之後
 * 賣家在審核期間就簽得了，端點 40 才擋得起 T08（未授權不得核定免用統一發票）。
 *
 * A03 用它決定「收據代製授權」那一欄**畫不畫**：不在這張表裡的狀態下，賣家根本簽不了
 * ⇒ 畫出「尚未授權」會被讀成缺件，而那是一句不成立的話。
 */
export const RECEIPT_AUTH_SIGNABLE_STATUSES: readonly string[] = [
	SellerTaxReviewStatus.SUBMITTED,
	SellerTaxReviewStatus.UNDER_REVIEW,
	SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
	SellerTaxReviewStatus.SELF_ISSUE_PENDING,
	SellerTaxReviewStatus.APPROVED_INVOICE_EXEMPT,
];

/** 這個狀態下賣家簽得了收據代製授權 ⇒ 該欄位有意義，要顯示。 */
export function receiptAuthRelevant(reviewStatus: string): boolean {
	return RECEIPT_AUTH_SIGNABLE_STATUSES.includes(reviewStatus);
}

/**
 * 這個決策會不會被端點 40 的 T08 守衛擋下（9354）。
 *
 * 🔴 **只有 `APPROVE_INVOICE_EXEMPT` 一個決策受拘束。** 寫成函式而不是在頁面上寫
 * `!receiptAuthorized` 的用意就在這裡：那樣寫很容易一路套到整排按鈕上，而
 * `APPROVE_INVOICE_REQUIRED`（以及駁回／補件／撤銷／重開）與收據代製授權無關——
 * 擋住它們不會有任何東西轉紅，只是那些裁決在畫面上永遠按不下去。
 *
 * ⚠️ 這是**預防**不是授權：後端才是守門的那一道。前端擋住之後 9354 仍然回得來
 * （賣家可以在這一頁載入之後才取消授權）⇒ 分流照樣要留。
 */
export function taxDecisionBlockedByReceiptAuth(
	decision: string,
	receiptAuthorized: boolean,
): boolean {
	return (
		decision === SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT &&
		!receiptAuthorized
	);
}

/** 決策的中文。查 CodeMeta 生成物，本檔一個中文碼值都不寫。 */
export function taxDecisionLabel(decision: string): string {
	return SELLER_TAX_REVIEW_DECISION_LABEL[decision] ?? decision;
}

/**
 * 每個決策「按下去會發生什麼」的一句話。
 *
 * 這不是碼表鏡像（CodeMeta 的 label 只有名稱），是這一頁自己的後果說明——ADM 在按下
 * 之前要看得到不可逆的部分（撤銷會讓已核定資格立刻失效、駁回會鎖 7 天重申冷卻）。
 */
export const TAX_DECISION_CONSEQUENCE: Record<string, string> = {
	[SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED]:
		"核定之後這個賣家的訂單會開立統一發票，核定效期預設一年。",
	[SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT]:
		"核定免用統一發票之後，這個賣家的訂單改由平台代製收據，買家在結帳頁看到的法定告知也會跟著改變。",
	[SellerTaxReviewDecision.REQUEST_SUPPLEMENT]:
		"退回給賣家補件，理由會出現在賣家端；補齊後會自動回到審核中。",
	[SellerTaxReviewDecision.KEEP_UNDER_REVIEW]:
		"狀態不變、期限不動，只留下一筆審核備註。",
	[SellerTaxReviewDecision.REJECT]: "駁回之後這個賣家 7 天內不能重新申請。",
	[SellerTaxReviewDecision.REVOKE]:
		"撤銷已核定的資格。已經代製的收據不追溯，但會留下分界時點；之後的訂單依撤銷後的狀態處理。",
	[SellerTaxReviewDecision.REOPEN_REVIEW]:
		"重新把這筆申請放回審核中，可用於受理逾期補件或發起複審。",
};

// ---------------------------------------------------------------------------
// A03　端點 41（延長裁決）
// ---------------------------------------------------------------------------

/**
 * 端點 41 的兩個 decision 值。
 *
 * ⚠️ **不是 `SellerTaxReviewDecision` 的成員**：`tax_profile.go` 的
 * `taxExtensionApprove`／`taxExtensionReject` 是端點 41 自己的值域（58 §6.1.12 逐字
 * 「端點 41 自己的 Decision 欄（APPROVE／REJECT）是另一組值域」）。送 `EXTEND` 過去
 * 會拿到 `decision is outside the defined value set`。
 */
export const TAX_EXTENSION_APPROVE = "APPROVE";
export const TAX_EXTENSION_REJECT = "REJECT";

/** 未指定 `extendDays` 時後端用的天數（`taxExtensionDefaultDays`）。 */
export const TAX_EXTENSION_DEFAULT_DAYS = 30;
/** `extendDays` 的上界（`taxExtensionMaxDays`）。APPROVE 時 1–365 之外一律 400。 */
export const TAX_EXTENSION_MAX_DAYS = 365;

/**
 * 延長裁決在目前狀態下成不成立。
 *
 * 逐字取自 `DecideExtension` 的三道守衛：APPROVE 需要 `SELF_ISSUE_PENDING`
 * （否則 9321）且 `extensionCount === 0`（否則 9323）。
 * REJECT 沒有狀態守衛——「駁回一個延長申請」在任何狀態下都是一次要留痕的 ADM 行為。
 */
export function taxExtensionApprovable(
	reviewStatus: string,
	extensionCount: string,
): boolean {
	return (
		reviewStatus === SellerTaxReviewStatus.SELF_ISSUE_PENDING &&
		(extensionCount === "" || extensionCount === "0")
	);
}

// ---------------------------------------------------------------------------
// A06　端點 52／53／54／55／56
// ---------------------------------------------------------------------------

/**
 * 端點 52 的 `fundingDecision` 值域（`admin_job.go` 的兩個常數，兩值不多不少）。
 *
 * ⚠️ **不是 CodeMeta 碼值**：`funded_by` 的三值（SELLER／PLATFORM／UNDETERMINED）是
 * coupon 側的事實，而這裡是 ADM 對「這筆平台出資折扣怎麼算」的裁決。名字相近、
 * 語意不同，不得互相代入。
 *
 * 🔴 `needsFundingDecision === true` 時**必填**，而且送 `PLATFORM_FUNDED` 與
 * `SELLER_FUNDED` 的結果不同（前者改開立金額）——不是一顆「隨便選一個就好」的選項。
 */
export const FUNDING_DECISION_SELLER = "SELLER_FUNDED";
export const FUNDING_DECISION_PLATFORM = "PLATFORM_FUNDED";

/** 兩個裁決的 UI 文案（沒有碼表，這是本頁自己的說明）。 */
export const FUNDING_DECISION_OPTIONS: ReadonlyArray<{
	value: string;
	label: string;
	hint: string;
}> = [
	{
		value: FUNDING_DECISION_SELLER,
		label: "由賣家吸收",
		hint: "折扣視為賣家給的折讓，開立金額為折扣後金額。",
	},
	{
		value: FUNDING_DECISION_PLATFORM,
		label: "由平台吸收",
		hint: "折扣視為平台補貼，開立金額為折扣前金額。",
	},
];

/**
 * 端點 56 的發票號碼格式（`admin_job.go` 的 `externalInvoiceNoPattern`）。
 *
 * DB 端另有 `chk_invdoc_external_no_shape` 守同一件事；那一道回的是 23514（對 ADM 是
 * 一句 500），所以前端這一道存在的價值是「回得出一句看得懂的話」。
 */
export const EXTERNAL_INVOICE_NO_PATTERN = /^[A-Z]{2}[0-9]{8}$/u;

/** 人工解鎖請求的狀態值（`domain/manual_unlock.go` 的四個常數，非 CodeMeta）。 */
export const MANUAL_UNLOCK_AWAITING = "AWAITING_SECOND_APPROVAL";
export const MANUAL_UNLOCK_APPROVED = "APPROVED";
export const MANUAL_UNLOCK_REJECTED = "REJECTED";
export const MANUAL_UNLOCK_EXPIRED = "EXPIRED";

/** 解鎖狀態的中文（沒有 CodeMeta 碼表，這是本頁自己的顯示文案）。 */
export const MANUAL_UNLOCK_STATUS_LABEL: Record<string, string> = {
	[MANUAL_UNLOCK_AWAITING]: "等待第二位管理員覆核",
	[MANUAL_UNLOCK_APPROVED]: "已覆核放行",
	[MANUAL_UNLOCK_REJECTED]: "已駁回",
	[MANUAL_UNLOCK_EXPIRED]: "已逾期失效（發起後 24 小時）",
};

// ---------------------------------------------------------------------------
// A08　端點 60
// ---------------------------------------------------------------------------

/**
 * 端點 60 的五個 `action`（`admin_complaint.go` 的五個 `complaintAction*` 常數）。
 *
 * ⚠️ **它們不是 CodeMeta `InvoiceComplaintAction` 的值**：那張碼表只有 6 值、記的是
 * 「時間軸上留了哪一筆」，而且刻意不含 `REQUEST_REWORK`／`RECOMMEND_SUSPENSION`
 * （那兩個不寫時間軸）。兩者交集非空但不相等，不得互相代入——這也是那張碼表沒有
 * 進 `ADMIN_ENUMS` 的原因。
 */
export const COMPLAINT_ACTION_REMIND_SELLER = "REMIND_SELLER";
export const COMPLAINT_ACTION_REQUEST_REWORK = "REQUEST_REWORK";
export const COMPLAINT_ACTION_DECIDE_RESOLVED = "DECIDE_RESOLVED";
export const COMPLAINT_ACTION_DECIDE_VIOLATION = "DECIDE_VIOLATION";
export const COMPLAINT_ACTION_RECOMMEND_SUSPENSION = "RECOMMEND_SUSPENSION";

/**
 * 「動作 × 允許的來源狀態」。
 *
 * 三個會轉狀態的動作逐字等於 SQL 的 `AND status = 'ESCALATED_TO_ADM'`
 * （`invoice_complaints.sql` 的 `DecideEscalatedInvoiceComplaint` 與
 * `ReturnInvoiceComplaintToSeller`）；rows=0 一律回 9336。
 *
 * 另兩個（提醒賣家／建議暫停接單）在後端**沒有狀態守衛**——它們不 CAS、只寫稽核與
 * 建議。所以這兩列是**前端自訂的**渲染條件，不是契約鏡像：
 *   - 提醒賣家：案件還沒結案才有意義（四個未結狀態）
 *   - 建議暫停接單：判定違規之後才有意義（`VIOLATION_CONFIRMED`）
 * 放寬它們不會產生「按下去必死」的鍵（後端會收），但會產生沒有意義的稽核列。
 */
export const COMPLAINT_ACTION_ALLOWED_FROM: Record<string, readonly string[]> =
	{
		[COMPLAINT_ACTION_REMIND_SELLER]: [
			"OPEN",
			"OVERDUE",
			"SELLER_RESPONDED",
			"ESCALATED_TO_ADM",
		],
		[COMPLAINT_ACTION_REQUEST_REWORK]: ["ESCALATED_TO_ADM"],
		[COMPLAINT_ACTION_DECIDE_RESOLVED]: ["ESCALATED_TO_ADM"],
		[COMPLAINT_ACTION_DECIDE_VIOLATION]: ["ESCALATED_TO_ADM"],
		[COMPLAINT_ACTION_RECOMMEND_SUSPENSION]: ["VIOLATION_CONFIRMED"],
	};

/** 五個動作的顯示順序。 */
export const COMPLAINT_ACTION_ORDER: readonly string[] = [
	COMPLAINT_ACTION_REMIND_SELLER,
	COMPLAINT_ACTION_REQUEST_REWORK,
	COMPLAINT_ACTION_DECIDE_RESOLVED,
	COMPLAINT_ACTION_DECIDE_VIOLATION,
	COMPLAINT_ACTION_RECOMMEND_SUSPENSION,
];

/** 動作的中文與後果（本頁自己的文案，沒有碼表可查——見上方說明）。 */
export const COMPLAINT_ACTION_LABEL: Record<string, string> = {
	[COMPLAINT_ACTION_REMIND_SELLER]: "提醒賣家",
	[COMPLAINT_ACTION_REQUEST_REWORK]: "要求再次處理",
	[COMPLAINT_ACTION_DECIDE_RESOLVED]: "判定已解決",
	[COMPLAINT_ACTION_DECIDE_VIOLATION]: "判定賣家違規",
	[COMPLAINT_ACTION_RECOMMEND_SUSPENSION]: "建議暫停接單",
};

export const COMPLAINT_ACTION_CONSEQUENCE: Record<string, string> = {
	[COMPLAINT_ACTION_REMIND_SELLER]: "案件狀態不變，只留下一筆往來紀錄與稽核。",
	[COMPLAINT_ACTION_REQUEST_REWORK]:
		"案件退回賣家並重新計算處理期限，狀態回到「處理中」。",
	[COMPLAINT_ACTION_DECIDE_RESOLVED]:
		"判定沒有違規並結案。買家在 24 小時後仍可重開這件申訴。",
	[COMPLAINT_ACTION_DECIDE_VIOLATION]:
		"判定賣家違規並結案（終態，不可重開）。回應會帶回 90 天內的違規件數與是否達到停權門檻——但發票模組不執行處分，處分要另外建立措施。",
	[COMPLAINT_ACTION_RECOMMEND_SUSPENSION]:
		"只產生一份停權建議與稽核紀錄，案件狀態不變。**它不會停權任何人**：實際處分要另外建立措施。",
};

/** 只有「建議暫停接單」需要 `confirmed`（`admin_complaint.go` 的那一道守衛）。 */
export const COMPLAINT_ACTION_NEEDS_CONFIRM: ReadonlySet<string> = new Set([
	COMPLAINT_ACTION_RECOMMEND_SUSPENSION,
]);

/** 會把案件推進終態、按下去收不回來的兩個動作（UI 用 danger）。 */
export const COMPLAINT_ACTION_DANGER: ReadonlySet<string> = new Set([
	COMPLAINT_ACTION_DECIDE_VIOLATION,
	COMPLAINT_ACTION_RECOMMEND_SUSPENSION,
]);

export function complaintActionAvailable(
	action: string,
	status: string,
): boolean {
	return (COMPLAINT_ACTION_ALLOWED_FROM[action] ?? []).includes(status);
}

// ---------------------------------------------------------------------------
// 業務碼分流
// ---------------------------------------------------------------------------

/**
 * 「這個動作在目前狀態下不成立」或「資料已經被別人改過」那一類。
 *
 * 共同處置：**關 dialog ＋ refetch ＋ 在頁面上說一句自己的話**。留在 dialog 裡沒有
 * 意義——使用者能做的只有看新的狀態再決定按哪一顆。
 *
 * 逐碼出處：
 *   - 9322 `SELLER_TAX_PROFILE_VERSION_CONFLICT`（端點 40／41）
 *   - 9321 `SELLER_TAX_PROFILE_STATE_INVALID`（端點 41 的 APPROVE 狀態守衛）
 *   - 9323 `SELLER_TAX_EXTENSION_ALREADY_USED`（端點 41，一次用完）
 *   - 9348 `INVOICE_TAX_DECISION_INVALID`（端點 40 的 allowed-from CAS）
 *   - 9336 `INVOICE_COMPLAINT_ACTION_NOT_ALLOWED`（端點 60 的 CAS）
 *   - 9341 `INVOICE_JOB_RETRY_NOT_ALLOWED`（端點 52）
 *   - 9343 `INVOICE_JOB_VERSION_CONFLICT`（端點 52–56 共用）
 *   - 9345 `INVOICE_MANUAL_UNLOCK_SAME_ACTOR`（端點 55 的雙人守衛）
 *   - 9349 `INVOICE_EXTERNAL_ISSUANCE_NOT_ALLOWED`（端點 56）
 */
export const ACTION_CONFLICT_CODES: ReadonlySet<number> = new Set([
	9321, 9322, 9323, 9336, 9341, 9343, 9345, 9348, 9349,
]);

/**
 * 端點 40 核定「應開統一發票」時賣家還沒填統編／法定名稱（backend#3049）。
 *
 * 🔴 **刻意不放進 `ACTION_CONFLICT_CODES`**：那一類的下一步是「重新整理再看」，
 * 而這一個的下一步是**要求賣家補件**——資料沒有變舊，是本來就缺。混進去會給出
 * 一句叫人重整的話，重整幾次都不會變。
 *
 * 這條路徑原本由 DB 的 `chk_stp_required_needs_taxid` 擋，以 500 ＋ Postgres 原文呈現。
 */
export const TAX_APPROVAL_IDENTITY_INCOMPLETE_CODE = 9353;

/**
 * 端點 40 核定「免用統一發票」時賣家還沒簽收據代製授權（T08 守衛，2026-09-05）。
 *
 * 🔴 **同樣不放進 `ACTION_CONFLICT_CODES`**：那一類的文案是「重新整理再看」，而這一格
 * ADM 自己解決不了——下一步是**請賣家去簽**（端點 29，ADM 無法代簽）。
 *
 * ⚠️ 但它與 9353 有一處相反：9353 是「本來就缺」所以不 refetch，這一格**要 refetch**。
 * 前端已經照 `receiptAuthorized` 擋過一次，會走到這裡代表畫面上那一筆是舊的
 * （賣家在載入之後取消了授權）⇒ 重新載入才會讓那顆按鈕跟著變成不可按。
 */
export const TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE = 9354;

/**
 * 端點 73（A13「送回自動線」）被回絕。
 *
 * 🔴 **與端點 52 共用 9341，但下一步不同** ⇒ 不得套 `actionConflictText(9341)` 的文案
 * （那一句講的是「無法確認服務商端沒有產生文件」，與這裡無關）。這一格的兩種成因是
 * 「文件不在待人工處理狀態」與「已經有開票作業，請改走 A06」，後端的 message 分得出來
 * ⇒ **優先顯示後端 message**，前端不自造。
 */
export const DOCUMENT_RESUME_NOT_ALLOWED_CODE = 9341;

/** 案件／申請／工作查無（各頁的 404 業務碼）。 */
export const TAX_PROFILE_NOT_FOUND_CODE = 9320;
export const COMPLAINT_NOT_FOUND_CODE = 9309;
export const JOB_NOT_FOUND_CODE = 9340;
export const DOCUMENT_NOT_FOUND_CODE = 9301;

/**
 * 端點 54 撞到「已經有一個等待覆核的請求」。
 *
 * 🔴 **這不是錯誤**：9344 是 preserveData 碼，HTTP 恆 200 且 data 帶著既有那一列的
 * `unlockId` ⇒ interceptor 不轉 error、前端 `code` 是 0 的路徑也走不到。呼叫端要自己
 * 看 `code`，把畫面切到「覆核」而不是顯示紅字。
 */
export const MANUAL_UNLOCK_NEEDS_SECOND_APPROVAL_CODE = 9344;

/**
 * 重寄通知的冷卻（端點 49／`notification_resend.go` 的 `resendWindow` ＝ 10 分鐘）。
 *
 * 🔴 **三個入口共用同一個計數器**（key ＝ `document_id`，不是 `document_id × actor`）：
 * ADM 幫忙點一次、賣家再點一次，買家會在十分鐘內收到兩封 ⇒ 撞到窗口回 9352（429），
 * 那是「剛剛才寄過」不是「寄失敗」，文案不得寫成失敗。
 */
export const RESEND_TOO_FREQUENT_CODE = 9352;
export const RESEND_WINDOW_MINUTES = 10;

/**
 * 版本衝突與狀態不合法的頁面級文案。
 *
 * ⚠️ **一律不透傳後端 message**：那些定版文案是寫給各自語境的（9322 說「稅務資格
 * 資料已被更新」、9336 說「這件申訴目前的狀態…」），跨頁借用會答非所問。
 */
export function actionConflictText(code: number): string {
	if (code === 9345) {
		return "覆核的人不能是發起的人，也不能是當初提出取消的人。請由另一位管理員覆核。";
	}
	if (code === 9323) {
		return "這筆申請的延長次數已經用完（只能延長一次），畫面已更新。";
	}
	if (code === 9341) {
		return "這件工作目前不能安全重試——後端無法確認服務商端沒有產生文件。畫面已更新。";
	}
	if (code === 9349) {
		return "這件工作尚未開放回報賣家自行開立的結果（需要先完成人工出口解鎖）。畫面已更新。";
	}
	return "這筆資料剛剛有新的變更，或目前的狀態無法進行這個動作，畫面已更新。";
}
