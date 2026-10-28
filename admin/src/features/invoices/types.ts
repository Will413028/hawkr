/**
 * ADM 發票模組（A01–A13）共用的 wire 型別。
 *
 * ## 三條規則
 *
 * 1. **欄名逐字照 Gateway DTO**（`gateway/internal/handler/invoice_dto.go` 的 json tag），
 *    不憑語意造名。TS 對 `any` 回應不擋、runtime 也不擋 ⇒ 名字打錯的症狀是畫面上
 *    渲染出 `undefined` 或一片空白，沒有任何東西會轉紅。
 * 2. **一律 `string`**。ID（`jobId` / `storeId` / `documentId`）是 TSID，超過
 *    `Number.MAX_SAFE_INTEGER`；金額與計數也一律以字串過線。要運算才 `Number(...)`，
 *    而 ID 永遠不需要運算。（`merchants/page.tsx:20-26` 已記過這個事故。）
 * 3. **狀態值不在這裡列舉**。值域的 SSOT 是 CodeMeta 生成物
 *    （`src/lib/codemeta/generated.ts`），這裡宣告成 `string` ⇒ 後端新增一個值時
 *    前端不會編譯錯，而顯示層的 default 分支會接住它。
 */

/** 分頁列表的統一 envelope（58 §6.1.0 (9)：四個欄位全部是字串）。 */
export interface InvoicePagedData<T> {
	records: T[];
	total: string;
	page: string;
	pageSize: string;
}

export interface InvoiceEnvelope<T> {
	code: number;
	message: string;
	data: T;
}

/**
 * A01 的一格 KPI（端點 37）。
 *
 * ⚠️ `value` **可能是空字串**，而且那不是 bug：`suspended_sellers` 的權威在
 * seller 模組，發票服務取不到時回空字串而不是 "0"（後端逐字：「取不到就明說
 * 取不到，不得回 0 充數」）。前端必須把空字串顯示成「取不到」，直接印會變成一張
 * 沒有數字的卡片，而印成 0 等於幫後端說了一個它拒絕說的謊。
 *
 * `label` 不在這個 DTO 裡：後端只下發 `key` / `value` / `windowLabel` / `definition`，
 * 卡片標題由前端的封閉列舉提供（見 A01 的 KPI_META）。
 */
export interface InvoiceKpiItem {
	key: string;
	value: string;
	windowLabel: string;
	definition: string;
}

export interface InvoiceRetryPolicy {
	initialIntervalSeconds: string;
	backoffMultiplier: string;
	maxIntervalSeconds: string;
	maxAttempts: string;
}

/**
 * 一筆發票工作（A01 優先清單 / A05 佇列 / A06 詳情共用同一個 DTO）。
 *
 * ⚠️ `canRetry` / `canCancel` 是**後端判定的布林**，不得從 `status` 推導：
 * 「可安全重試」的定義包含一個 provider 端事實（該冪等鍵在 AMEGO 已確認未產生
 * 文件），前端看不到那個事實，推錯的後果是開出第二張發票。
 */
export interface InvoiceJobInfo {
	jobId: string;
	orderNo: string;
	storeId: string;
	operationType: string;
	status: string;
	summary: string;
	attemptCount: string;
	maxAttempts: string;
	lastAttemptAt: string;
	nextRetryAt: string;
	retryPolicy: InvoiceRetryPolicy;
	shipmentUnaffectedNotice: string;
	providerRequestId: string;
	version: string;
	canRetry: boolean;
	canCancel: boolean;
	manualReasonCode: string;
	needsFundingDecision: boolean;
	externalInvoiceNo: string;
	externalInvoiceDate: string;
	reportedAt: string;
	canReportExternal: boolean;
}

export interface InvoiceOverviewData {
	kpis: InvoiceKpiItem[];
	priorityJobs: InvoiceJobInfo[];
}

/**
 * A02 佇列的一列（端點 38）。
 *
 * ⚠️ `claimedStatus` 是賣家**自述**的稅務身分（SellerTaxObligation），不是平台的
 * 判定結果；判定結果是 `reviewStatus`（SellerTaxReviewStatus）。兩者在同一列上，
 * 混用會讓管理員以為某個賣家已經核定過了。
 *
 * ⚠️ `remainingDays` 是後端算好的字串、**可為負**（逾期）。前端不得自己拿
 * `deadlineAt` 減 now：那會只扣週末而不扣國定假日，與後端的行事曆表得到不同答案。
 */
export interface TaxReviewItem {
	storeId: string;
	storeName: string;
	entityType: string;
	claimedStatus: string;
	reviewStatus: string;
	submittedAt: string;
	deadlineAt: string;
	remainingDays: string;
	extensionCount: string;
	selfIssueAccepted: boolean;
	/**
	 * 收據代製授權（S05，端點 29）已簽。
	 *
	 * 🔴 它是 T08「核定免用統一發票」的**前置條件**（未授權按下去回 9354）。
	 * 佇列上看得見，ADM 才不必逐一點進 A03 才知道哪一列現在核定得了 ——
	 * 端點 38 的 DTO 在 2026-09-05 之前沒有這一欄，那正是 A02 的用途被抵銷的地方。
	 *
	 * ⚠️ 它與 `selfIssueAccepted` 是**兩份不同的簽署**：後者是自行開票責任條款
	 * （賣家自己開票），本欄是授權平台代製收據。同一位賣家可以只簽其中一份。
	 */
	receiptAuthorized: boolean;
	riskFlags: string[];
}

// ---------------------------------------------------------------------------
// A03　稅務身分審核詳情（端點 39 `GET /admin/invoices/tax_reviews/{store_id}`）
// ---------------------------------------------------------------------------

/**
 * 一份稅務證明文件。
 *
 * `fileDownloadPath`：ADM 端點 39 的回應由 gateway 填好完整相對路徑（invoice 持有授權的
 * 稅籍佐證端點，路徑由擁有者回傳、前端不組）。空字串／缺漏 ⇒ 不畫連結。
 * 後台沒有通用的機敏檔下載；`fileAssetId` 只表示 media 查得到這份資產，不是下載的輸入。
 *
 * ⚠️ `kind` 的值域是 `TAX_REGISTRATION` / `EXEMPTION_CERT` / `ID_DOCUMENT` / `OTHER`
 * （proto 註解），而它**不是 CodeMeta enum** ⇒ 沒有中文可查。前端原樣顯示碼值，
 * 不自己寫一張四筆的對照表——那會是一份沒有 SSOT 的第二碼表（§0.4 ⑤）。
 */
export interface TaxDocumentInfo {
	documentId: string;
	kind: string;
	fileName: string;
	fileSize: string;
	fileKey: string;
	uploadedAt: string;
	fileAssetId: string;
	fileDownloadPath?: string;
}

/**
 * 賣家申報的稅務主體資料。
 *
 * ⚠️ 地址是**三個欄位**（city / district / detail），DTO 沒有單一地址欄，前端串接。
 * ⚠️ `claimedStatus` 是賣家**自述**（`SellerTaxObligation`），不是判定結果；判定結果
 * 是外層的 `reviewStatus`。畫面上必須標「賣家自述，非判定依據」。
 */
export interface SellerTaxIdentity {
	entityType: string;
	taxId: string;
	legalName: string;
	responsibleName: string;
	addressCity: string;
	addressDistrict: string;
	addressDetail: string;
	claimedStatus: string;
}

/**
 * 自行開票責任條款的接受紀錄。
 *
 * ⚠️ proto 是 message 不是 optional ⇒ **JSON 上永遠是物件，不會是 `null`**。
 * 「未接受」的判準是 `termsVersion` 為空字串，不是 `=== null`（後者永遠不成立，
 * 於是「未接受」那一列永遠不會出現）。
 *
 * ⚠️ 沒有 `termsUrl`：58A 寫「條款全文走 `selfIssueAttestation.termsUrl`（公開 CDN）」，
 * 而 DTO 給的是 `termsKey`（media objectKey）。ADM 不組 media URL（那需要
 * `MEDIA_BASE_URL`，ADM 側沒有這個設定），所以本波只顯示版本與接受時間，不畫連結。
 */
export interface SelfIssueAttestationInfo {
	termsVersion: string;
	termsKey: string;
	acceptedAt: string;
	acceptedByAccountId: string;
	effective: boolean;
}

/**
 * 待審期間的接單限制現況。
 *
 * ⚠️ 同上，JSON 上永遠是物件。「非待審期間」的判準是 `orderCountLimit` 為空字串
 * ⇒ 整塊不渲染。判成 `null` 會讓它在每一頁都顯示「0 / 0」，而 0 與「沒有限制」
 * 是兩件完全不同的事。
 *
 * ⚠️ 上限數字由後端下發（env 決定 DB 預設值，依環境不同），前端不寫死。
 */
export interface SellerIntakeLimit {
	orderCountUsed: string;
	orderCountLimit: string;
	amountUsed: string;
	amountLimit: string;
	blocked: boolean;
}

/** ADM 在審核過程留下的備註。 */
export interface TaxReviewNote {
	noteId: string;
	content: string;
	createdAt: string;
}

/**
 * 端點 39 的回應（Gateway `SellerTaxProfileData`）。
 *
 * 🔴 **與 58A §A03 的欄位對照有四處落差，全部是「規格有、DTO 沒有」**，逐一列出是
 * 為了讓下一個人不用再查一次（欄名打錯的症狀是渲染 `undefined`，沒有東西會轉紅）：
 *   - `storeName`　　　　→ 沒有。頁首只能顯示 `storeId`（A02 列表那一支才有店名）
 *   - `kycTaxId`　　　　→ 沒有。「兩個統編分兩欄」只畫得出申請統編那一欄
 *   - `riskFlags[]`　　 → 沒有。風險標記只在 A02 的列表 DTO 上
 *   - `decisionHistory[]`→ 沒有。最接近的是 `admNotes[]`（只有 noteId/content/createdAt，
 *     不帶 decision／actor／revokedEffectiveAt）⇒ 決策歷程只能畫成「審核備註」時間軸
 */
export interface SellerTaxProfileData {
	storeId: string;
	reviewStatus: string;
	version: string;
	submittedAt: string;
	deadlineAt: string;
	extensionCount: string;
	canRequestExtension: boolean;
	reapplyCooldownUntil: string;
	taxIdentity: SellerTaxIdentity;
	documents: TaxDocumentInfo[];
	admNotes: TaxReviewNote[];
	selfIssueAttestation: SelfIssueAttestationInfo;
	intakeLimit: SellerIntakeLimit;
	/**
	 * 賣家已完成收據代製授權（S05，端點 29）。
	 *
	 * 🔴 **它是「核定免用統一發票」的前置條件**（2026-09-05 定版，`DEC-20260905-015836`）。
	 * 原本的死結（核定要先授權、授權要先核定）解除的方式是**放寬端點 29**：allowed-from
	 * 從只有 `APPROVED_INVOICE_EXEMPT` 改成 `SUBMITTED`／`UNDER_REVIEW`／
	 * `SUPPLEMENT_REQUIRED`／`SELF_ISSUE_PENDING`／`APPROVED_INVOICE_EXEMPT`
	 * ⇒ 賣家在核定之前就簽得了 ⇒ **核定當下這一欄不再必然是 false**。
	 * 端點 40 因此新增 T08 守衛：`APPROVE_INVOICE_EXEMPT` ＋ 這一欄為 false ⇒ 回
	 * 9354 `INVOICE_TAX_APPROVAL_RECEIPT_AUTH_MISSING`（HTTP 409）。
	 *
	 * **什麼時候該擋**：只有 `APPROVE_INVOICE_EXEMPT` 那一顆按鈕。
	 * **什麼時候不該擋**：`APPROVE_INVOICE_REQUIRED` 與其餘五個決策都不受 T08 拘束——
	 * 拿這一欄去 disable 它們，會擋掉一整批與收據代製無關的裁決。
	 *
	 * ⚠️ **前端擋住不代表 9354 不會回來**：賣家可以在這一頁載入之後才取消授權 ⇒
	 * 業務碼分流不得省略（`admin-actions.ts` 的 `TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE`）。
	 *
	 * 核定之後它一樣要顯示：未授權的 EXEMPT 賣家，每一筆已付訂單都會落
	 * `AWAITING_MANUAL` / `RECEIPT_AUTH_MISSING`，買家端永遠停在「處理中」。
	 */
	receiptAuthorized: boolean;
	/** ISO 8601 UTC；未授權為空字串。 */
	receiptAuthorizedAt: string;
	/**
	 * 尚未裁決的展延申請（端點 20 → 41）；沒有時為 `null`。
	 *
	 * 🔴 **它是「這顆核准展延按鈕現在該不該按」的唯一依據。** 端點 20 只寫一列
	 * `invoice_status_history`、不碰 `seller_tax_profiles` 的任何一欄 ⇒ 在這一欄
	 * 出現之前（2026-09-05），申請過與沒申請過的賣家在 ADM 這側**完全無法分辨**：
	 * A02 的 `filter=UNDER_REVIEW` 把兩者混在同一頁（設計書逐字說「都是待 ADM
	 * 決策，A02 的手勢不需要分開」），A03 只有 `extensionCount`（准了幾次）與
	 * `canRequestExtension`（賣家還能不能按）。dev 實測有一筆掛了兩天沒人看見。
	 *
	 * ⚠️ 它為 `null` **不代表**不能核准展延：`taxExtensionApprovable` 的判準沒變
	 * （SELF_ISSUE_PENDING ＋ 還沒延長過）。這一欄只回答「有沒有人在等」。
	 */
	pendingExtensionRequest: TaxExtensionRequestInfo | null;
}

/** 一筆尚未被端點 41 裁決的展延申請。 */
export interface TaxExtensionRequestInfo {
	/** ISO 8601 UTC。 */
	requestedAt: string;
	/** 賣家送出時填的理由（必填）。ADM 決定准不准的唯一依據。 */
	reason: string;
}

// ---------------------------------------------------------------------------
// A04　AMEGO 串接監控（端點 42 列表 / 43 詳情）
// ---------------------------------------------------------------------------

/**
 * 端點 42 的一列。
 *
 * ⚠️ `sellerTaxId` 是**完整 8 碼**，不是遮罩值（58 號 §8.1.3(b) 把賣家統編判為 O 類、
 * 三端皆完整——它是公開營業資訊，而 ADM 核對統編正是 A04 的用途之一）。
 * 欄名 2026-09-02 由 `sellerTaxIdMasked` 改名（#3017）：舊名帶著 masked 卻回完整值，
 * 會讓下游誤以為已經遮過而可以安全轉發。前端一樣不遮也不還原，照吐。
 * ⚠️ 沒有「最近一次檢查結果」——列表只有 `lastVerifiedAt`（時間），結果在端點 43。
 */
export interface ProviderAccountItem {
	storeId: string;
	storeName: string;
	sellerTaxId: string;
	status: string;
	credentialLast4: string;
	lastVerifiedAt: string;
	automationCutoverAt: string;
}

/**
 * 端點 43 的回應（Gateway `SellerProviderAccountData`）。
 *
 * 🔴 **58A §A04 指定的三塊資料在這個 DTO 上不存在**：
 *   - `storeName` / `taxReviewStatus`　→ 沒有（頁首只印 storeId）
 *   - `recentHealthChecks[≤10]`　　　　→ 沒有 ⇒ 「最近檢查」Section 本波不畫
 *   - `trackBalance`　　　　　　　　　 → 沒有 ⇒ 「字軌餘量」Section 本波不畫
 * 這三塊不是「查不到所以不畫」，是**契約上沒有這個欄位**；畫一張永遠空的卡等於
 * 告訴管理員「這個賣家沒有檢查紀錄」，而那是一句我們無從得知的話。
 *
 * ⚠️ `credentialFingerprint` 也不在 DTO 上——58A 已定版「本波一律不顯示」，兩者一致。
 * ⚠️ `status` 三態（NOT_STARTED / NOT_CONFIGURED / CONFIG_ERROR）**不得做 fallback**
 * （陷阱 T16）：`status || "NOT_STARTED"` 會把後端刻意區分的三態壓回兩態。
 */
export interface SellerProviderAccountData {
	provider: string;
	status: string;
	statusNotice: string;
	version: string;
	sellerTaxId: string;
	credentialLast4: string;
	credentialUpdatedAt: string;
	lastVerifiedAt: string;
	automationCutoverAt: string;
	consentVersion: string;
	lastRequestId: string;
	pendingOrderCount: string;
}

/**
 * 端點 45（停用連線）的回應（Gateway `DeactivateProviderAccountForAdminData`）。
 *
 * ⚠️ `pendingOrderCount` **只拿得到事後值**：這是停用**成功之後**的計數，而端點 43
 * （dialog 開啟時已載入的那一支）也有一個同名欄位、語意是「當下待處理」。
 * ⇒ dialog 內不顯示任何受影響訂單數（前端估算會與後端判定不同，中間還有 TOCTOU
 * 窗口），只有成功畫面顯示這一份。
 *
 * ⚠️ `affectedScopeNotice` 是後端下發的說明句（「只影響停用時點之後付款的訂單」），
 * 前端不自己造這句話——它的口徑要跟 `automation_cutover_at` 的判定一致。
 */
export interface DeactivateProviderAccountResultData {
	storeId: string;
	status: string;
	deactivatedAt: string;
	affectedScopeNotice: string;
	pendingOrderCount: string;
	auditLogId: string;
}

/**
 * 端點 67（撤銷連線）的回應（Gateway `RevokeProviderAccountForAdminData`）。
 *
 * 與端點 45 的差別只有時間欄名（`revokedAt` vs `deactivatedAt`）——**不要合成一個
 * 共用型別再加 optional**：兩支端點的欄名是各自的契約，合起來之後「哪一支會回哪一個」
 * 就只剩註解在講，而註解不會轉紅。
 */
export interface RevokeProviderAccountResultData {
	storeId: string;
	status: string;
	revokedAt: string;
	affectedScopeNotice: string;
	pendingOrderCount: string;
	auditLogId: string;
}

// ---------------------------------------------------------------------------
// A06　發票異常詳情（端點 51 `GET /admin/invoices/jobs/{job_id}`）
// ---------------------------------------------------------------------------

/** 與結帳 preview 同一個金額 DTO。金額一律字串。 */
export interface InvoiceAmountBreakdown {
	productAmount: string;
	shippingAmount: string;
	discountAmount: string;
	invoiceAmount: string;
	taxCategory: string;
}

/**
 * 一次與 AMEGO 的往返。
 *
 * ⚠️ `providerCode` 與 `providerMessage` **必須成對顯示**：AMEGO 的 `53` 在官方錯誤表
 * 出現兩次且語意不同，只看 code 會導向錯誤的下一步。
 * ⚠️ `masked*Body` 是 invoice-service **寫入時**就遮好的，前端不做任何遮罩邏輯，
 * 也不提供「顯示完整」切換——沒有完整值可顯示（§0.4 ③）。
 */
export interface ProviderExchangeItem {
	exchangeId: string;
	occurredAt: string;
	endpoint: string;
	providerCode: string;
	providerMessage: string;
	requestId: string;
	maskedRequestBody: string;
	maskedResponseBody: string;
}

/** 一筆發票稽核紀錄（A06 的操作稽核時間軸）。 */
export interface InvoiceAuditLogItem {
	auditLogId: string;
	occurredAt: string;
	actorAccountId: string;
	actorDisplayName: string;
	operationType: string;
	targetKind: string;
	targetId: string;
	beforeJson: string;
	afterJson: string;
	reason: string;
	requestId: string;
	termsVersion: string;
	result: string;
}

/**
 * 雙人人工解鎖的現況。
 *
 * ⚠️ 同樣是 message 不是 optional ⇒ JSON 上永遠是物件；「沒有解鎖請求」的判準是
 * `unlockId` 為空字串。
 *
 * 產生這個物件的兩支端點（54 發起／55 覆核）**都已實作**
 * （`services/invoice-service/internal/grpc/manual_unlock.go` 的
 * `RequestManualUnlockForAdmin`／`ApproveManualUnlockForAdmin`），A06 已接上兩顆按鈕。
 * 舊註解寫「backend #2899：schema 沒有任何 manual_unlock 承載」——那份 schema 現在有
 * `invoice_manual_unlocks` 表（`db/queries/invoice_manual_unlocks.sql`），該敘述已過期。
 *
 * ⚠️ `status` 的四個值（`AWAITING_SECOND_APPROVAL`／`APPROVED`／`REJECTED`／`EXPIRED`）
 * **不是 CodeMeta enum**，它們是 `internal/domain/manual_unlock.go` 的 Go 常數 ⇒ 中文
 * 走 `features/invoices/admin-actions.ts` 的 `MANUAL_UNLOCK_STATUS_LABEL`。
 */
export interface ManualUnlockInfo {
	unlockId: string;
	status: string;
	requestedByAccountId: string;
	approvedByAccountId: string;
	providerCheckEvidence: string;
	providerCheckedAt: string;
	requestedAt: string;
	expiresAt: string;
	/**
	 * J28 的取消證據（`InvoiceCancelEvidenceKind` 的**兩個需人判讀的值**）。
	 * 空字串＝這一列走的是 `CANCELLED_BY_ADMIN` 那條路徑，證據已在 operation 上。
	 * 覆核者要看得到它——沒有它，第二個人是在對看不見證據的請求按同意。
	 */
	cancelEvidenceKind: string;
	cancelEvidenceObjectKey: string;
}

/**
 * 端點 51 的回應（Gateway `AdminInvoiceJobDetail`）。
 *
 * ⚠️ `orderSnapshotJson` **是字串不是物件**：它是**當時**那份快照的原文。解析成 DTO
 * 等於讓前端契約綁死一個歷史結構，那個結構本來就會隨時間改變。原樣 `<pre>` 呈現。
 */
export interface AdminInvoiceJobDetail {
	job: InvoiceJobInfo;
	orderSnapshotJson: string;
	amountBreakdown: InvoiceAmountBreakdown;
	providerExchanges: ProviderExchangeItem[];
	idempotencyKey: string;
	shipmentStatus: string;
	auditLogs: InvoiceAuditLogItem[];
	manualUnlock: ManualUnlockInfo;
}

// ---------------------------------------------------------------------------
// A07／A08　未收到發票申訴（端點 58 列表 / 59 詳情，同一個 DTO）
// ---------------------------------------------------------------------------

/**
 * 一筆往來紀錄（買家申訴 → 賣家回覆 → ADM 動作）。
 *
 * ⚠️ 沒有 `eventLabel`：58A §B-3.1.3 說「A07／A08 時間軸的中文由後端逐列下發
 * `eventLabel`，所以 `InvoiceComplaintEventType`／`InvoiceComplaintAction` 故意不進
 * `ADMIN_ENUMS`」——但 Gateway 的 `ComplaintResponseItem` 只有下面五欄。
 * ⇒ 兩條路都沒有中文，前端原樣顯示 `actor` / `action` 碼值，**不自己補一張碼表**。
 */
export interface ComplaintResponseItem {
	responseId: string;
	actor: string;
	action: string;
	note: string;
	occurredAt: string;
}

/**
 * 端點 58 的一列，也是端點 59 的完整回應（Gateway `InvoiceComplaintInfo`，同一型別）。
 *
 * 🔴 **與 58A §A07／§A08 的落差（規格有、DTO 沒有）**：
 *   - `storeName`　　　　　　　→ 沒有。列表的「賣家」欄只印 `storeId`
 *   - `violationCountIn90Days`→ 沒有 ⇒ 「90 天同類申訴」與「是否達門檻」兩欄畫不出來。
 *     它只出現在端點 60 的回應（`SuspensionRecommendation`）⇒ 送出「判定賣家違規」或
 *     「建議暫停接單」之後才拿得到，詳情這一支查不到
 *   - `invoiceSnapshot`　　　 → 沒有 ⇒ A08 的「發票現況」Section 畫不出來
 *   - `notificationHistory[]` → 沒有 ⇒ A08 的「通知寄送歷程」Section 畫不出來。
 *     58A 稱這一區是「A08 能不能裁決的關鍵」，缺它等於裁決依據不在畫面上
 *   - 期限欄名是 `dueAt` 不是 `slaDueAt`
 *
 * ⚠️ `remainingBusinessDays` / `overdueBusinessDays` 是**後端算好的工作天字串、可為負**。
 * 前端零計算（§0.6 C10）：只扣週末不扣國定假日會與後端得到不同答案，而逾期會計違規、
 * 90 天達 3 次時建議停權（由管理員建立處分）。
 */
export interface InvoiceComplaintInfo {
	complaintId: string;
	caseNo: string;
	orderNo: string;
	storeId: string;
	status: string;
	expectedDocKind: string;
	dueAt: string;
	remainingBusinessDays: string;
	overdueBusinessDays: string;
	buyerMaskedName: string;
	description: string;
	responses: ComplaintResponseItem[];
	version: string;
	createdAt: string;
	reopenCount: string;
	buyerReviewDueAt: string;
	reopenAllowedAfter: string;
}

// ---------------------------------------------------------------------------
// A12／A13　發票文件（端點 47 列表 / 48 詳情）
// ---------------------------------------------------------------------------

/**
 * 文件摘要（Gateway `InvoiceDocumentSummary`，列表與詳情共用）。
 *
 * 🔴 **欄名是 `invoiceNumber` 不是 `invoiceNoMasked`，而且它是完整值。**
 * 58A §A12／§A13 的欄位對照兩處都寫 `invoiceNoMasked`（「DTO 就是遮罩值，例
 * `AB1000****`」），實際契約沒有那個欄位。這不是後端漏遮：`api/proto/invoice/invoice.proto`
 * 的 `invoice_number` 逐字寫著「完整值，**不遮罩**。§8.1.3 (b) 把它判為 O 類」，
 * invoice-service 的 `toDocumentSummary` 更寫「invoice_number 三端皆完整（O 類，
 * §8.1.3(c) 裁定 1 **逐字推翻** img-05 的遮罩畫法）」——它是對獎／報帳／申訴的唯一
 * 憑據，同時印在發票上、寄在通知信裡、存在載具裡。
 * ⇒ 前端原樣顯示。**不得自己補遮罩**（§0.4 ③：遮罩一律後端做；自己遮會蓋掉唯一憑據）。
 *
 * ⚠️ 沒有 `downloadPath`：58A §A13 說「下載 PDF 走 `downloadPath`」，而端點 48 的
 * 回應**沒有這個欄位**（`InvoiceDocumentDetail` 只有 `invoiceFileKey`，那是 objectKey，
 * 58A 自己也明令不顯示）。自行拼接下載路徑是陷阱 T22 ⇒ A12／A13 都不畫下載鍵。
 */
export interface InvoiceDocumentSummary {
	documentId: string;
	documentKind: string;
	status: string;
	invoiceNumber: string;
	issuedAt: string;
	totalAmount: string;
	carrierSummary: string;
	relatedDocumentId: string;
	fileAvailable: boolean;
	fileUnavailableReason: string;
}

/**
 * 端點 47 的一列（Gateway `SellerInvoiceDocumentItem`，ADM 與賣家共用同一型別）。
 *
 * 🔴 **沒有 `storeId` / `storeName`**：58A §A12 寫「records ＝
 * `AdminInvoiceDocumentItemResponse`（＝ `SellerInvoiceDocumentItemResponse` ＋
 * `storeId`／`storeName`）」，而 Gateway 的 handler
 * （`invoice_admin.go` 的 `ListInvoiceDocumentsForAdmin`）回的就是
 * `[]SellerInvoiceDocumentItem` 本身，沒有那個 admin 專屬型別。
 * ⇒ 「訂單／賣家」那一欄畫不出賣家；賣家維度只剩 `store_id` 篩選框。
 *
 * ✅ **`paidAt` 與 `shipmentStatus` 已由後端填值**（backend#3019，2026-09-02）：
 * `paidAt` 取 `invoice_documents.paid_at`（開票當下落下的收款時點）；
 * `shipmentStatus` 由 invoice-service 逐頁向 shipping-service 批次查。
 *
 * 🔴 **`shipmentStatus` 的空字串只有在 `shipmentStatusDegraded === false` 時
 * 才等於「還沒出貨」。** 兩者塌在一起正是本欄原本不敢畫的理由——降級時每一列都是
 * 空字串，而空白會被讀成「還沒出貨」，那是一句我們無從得知的話。
 * ⇒ 讀這一欄之前**必須**先看 `InvoiceDocumentsPagedData.shipmentStatusDegraded`。
 *
 * 🔴 值域是 `ShippingStatus` 的 value（數字字串，例 `"210"`），不是人類可讀文案 ⇒
 * 一律走 `codeLabel(SHIPPING_STATUS_LABEL, …)`，前端不得手寫中文。
 *
 * ⚠️ 2026-09-02 之前建立的文件 `paid_at` 是 NULL ⇒ 那些列的 `paidAt` 仍是空字串。
 * 那是誠實的「不知道」，不是本頁的 bug。
 */
/**
 * 端點 30／47 的分頁 data：標準四欄 ＋ 一個布林。
 *
 * `shipmentStatusDegraded === true` ＝ 這一頁的 `shipmentStatus` 沒有取到
 * （shipping-service 不可用／逾時），**不是**「這些訂單都還沒出貨」。
 * 降級是**整批**的性質（後端一次 RPC 涵蓋整頁），所以它在 data 層而不是逐列。
 */
export interface InvoiceDocumentsPagedData
	extends InvoicePagedData<AdminInvoiceDocumentItem> {
	shipmentStatusDegraded: boolean;
}

export interface AdminInvoiceDocumentItem {
	summary: InvoiceDocumentSummary;
	orderNo: string;
	paidAt: string;
	buyerKind: string;
	invoiceMode: string;
	shipmentStatus: string;
	updatedAt: string;
	latestJobId: string;
}

/** 一行發票品項。⚠️ `DISCOUNT` 行的 `netAmount` 是**負數**，顯示為 `-NT$ 120`，不取絕對值。 */
export interface InvoiceLine {
	lineType: string;
	name: string;
	quantity: string;
	unitPrice: string;
	netAmount: string;
	taxCategory: string;
}

/**
 * 文件的一筆歷程。
 *
 * 🔴 **`event` 的值域是文件狀態（`InvoiceDocumentStatus`），不是
 * `InvoiceDocumentTimelineEvent`。** 58A §B-3.1.3 與 §A13 都寫「`timeline[].event` 的中文
 * 走 `codeLabel(INVOICE_DOCUMENT_TIMELINE_EVENT_LABEL, …)`」，但 invoice-service 的
 * `toDocumentDetail` 填進去的是 `StatusHistoryEntry.ToStatus`
 * （`internal/domain/repository.go` 的 `StatusHistoryEntry`）——那是狀態歷程表的
 * 「轉到哪個狀態」，值域是 11 個文件狀態。
 * ⇒ 中文查表見 `features/invoices/documents.ts` 的 `documentTimelineLabel()`：
 * 先查文件狀態表、再退回 timeline event 表、最後原樣顯示碼值。
 * 兩張表都是 CodeMeta 生成物，前端一個中文字都沒有手寫。
 */
export interface InvoiceTimelineItem {
	occurredAt: string;
	event: string;
	note: string;
}

/** 文件完整明細（Gateway `InvoiceDocumentDetail`）。 */
export interface InvoiceDocumentDetail {
	summary: InvoiceDocumentSummary;
	issuerName: string;
	issuerTaxId: string;
	recipientName: string;
	recipientTaxId: string;
	salesAmount: string;
	taxAmount: string;
	totalAmount: string;
	taxCategory: string;
	lines: InvoiceLine[];
	timeline: InvoiceTimelineItem[];
	invoiceFileKey: string;
}

/**
 * 賣家視角的文件詳情（Gateway `SellerInvoiceDocumentDetail`），ADM 詳情把它整包內嵌。
 *
 * 🔴 **沒有 `fulfillmentMode`**：58A §A13 的 Section 1 欄位表列了它，DTO 上不存在。
 * 最接近的是列表 DTO 的 `invoiceMode`（同一個 `InvoiceFulfillmentMode` 值域），
 * 但詳情這一支沒有帶 ⇒ 「開票模式」只在 A12 的列表看得到。
 *
 * 🔴 **`fundedBy` 恆為 `UNDETERMINED`**：invoice-service 的 `toSellerDocumentDetail`
 * 直接寫死該常數（它沒有 coupon client，fail-closed）。而 `UNDETERMINED` 不在任何
 * CodeMeta 值域裡 ⇒ 沒有中文可查。本波不渲染這一欄：印一個沒有中文、恆為同值的碼
 * 只會讓人以為那是這張發票的性質。
 *
 * 🔴 **`createdBy` 與 `nextResendAvailableAt` 同樣從不被填**（同一個 mapper 沒有設）。
 * 兩者都不渲染。⚠️ 「重新寄送通知」（端點 49）**已實作**且 A13 已接上，但它的冷卻
 * 判斷不能靠 `nextResendAvailableAt`（恆為空字串，讀它等於「隨時都可以寄」）——
 * 唯一可靠的訊號是後端回的 9352（`notification_resend.go` 的 10 分鐘窗口，守衛在 SQL
 * 的 WHERE，三個入口共用同一個計數器）。
 */
export interface SellerInvoiceDocumentDetail {
	detail: InvoiceDocumentDetail;
	fundedBy: string;
	createdBy: string;
	cutoverVersion: string;
	providerRequestId: string;
	manualReasonCode: string;
	nextResendAvailableAt: string;
	latestJobId: string;
}

/**
 * 端點 48 的回應（Gateway `AdminInvoiceDocumentDetail`）。
 *
 * ⚠️ 打這一支**會落一列 `INVOICE_PII_REVEAL` 稽核**，而且是 fail-closed 的：
 * invoice-service 的 `GetInvoiceDocumentForAdmin` 註解逐字寫「usecase 在回資料之前先寫
 * 一列 INVOICE_PII_REVEAL 稽核，寫不進去就不回」。⇒ A13 的副標必須寫「開啟本頁會記錄
 * 一筆存取紀錄」，而 A06 不得寫（陷阱 T30：那一頁的主資料是端點 51）。
 *
 * 🔴 **`idempotencyKey` 從不被填**：`toAdminDocumentDetail` 只設
 * `SellerView`／`ProviderExchanges`／`RelatedJobs` 三個欄位 ⇒ 恆為空字串。
 * 58A §A13 的 Section 5 要顯示它並附複製鍵 ⇒ 本波不畫那一欄（畫出「冪等鍵：—」
 * 等於宣稱這份文件沒有冪等鍵，而那是假的）；`relatedJobs` 照畫。
 */
export interface AdminInvoiceDocumentDetail {
	sellerView: SellerInvoiceDocumentDetail;
	providerExchanges: ProviderExchangeItem[];
	idempotencyKey: string;
	relatedJobs: InvoiceJobInfo[];
	/** `invoice_documents.version`。端點 73 的 `expectedVersion` 取它（不是 job 的版本）。 */
	documentVersion: string;
	/**
	 * 可以按「送回自動線」（端點 73）。
	 *
	 * 🔴 **不得自行從 `status` 推導**。後端的述詞是
	 * `usecase.CanResumeDocumentFromManual`（status=AWAITING_MANUAL **且** 零 operation），
	 * 讀端與寫端共用同一支——前端自己判會讓按鈕亮在一個 CAS 必定 rows=0 的狀態上，
	 * 而 rows=0 是靜默的（#3031／#3089 第 8 項兩次都是讀寫端分家）。
	 *
	 * 已經有開票作業的文件走端點 52（入口在 A06），不是這一顆。
	 */
	canResumeFromManual: boolean;
}

// ---------------------------------------------------------------------------
// A09　AMEGO 啟用切點與版本歷程（端點 46
// `GET /admin/invoices/provider_accounts/{store_id}/cutovers`）
// ---------------------------------------------------------------------------

/**
 * 一筆自動開立切點（Gateway `ProviderCutoverItem`）。
 *
 * ⚠️ `deactivatedAt` 為空字串 ＝ **這一版仍生效中**，不是「沒有資料」。
 *
 * 🔴 **後端的排序鍵是 `created_at DESC, id DESC`，不是 `cutover_at`**
 * （`services/invoice-service/db/queries/invoice_provider_cutovers.sql` 的
 * `ListProviderCutoversForAdmin`；第二鍵是為了讓「同一交易內關舊列＋插新列」
 * 那組相同時間戳在分頁時穩定）。58A §A09 寫的是「依 `cutoverAt` 遞減」。
 * ⇒ 前端**不重排**：端點是分頁的，前端排序只會排到當前頁，結果是看起來像全域
 * 排序其實不是（A04 列表頁的「不做表頭排序」記的是同一件事）。畫面兩個時間欄
 * 都印出來，讀者自己看得到差異。
 */
export interface ProviderCutoverItem {
	version: string;
	cutoverAt: string;
	deactivatedAt: string;
	mode: string;
	createdByAccountId: string;
	createdAt: string;
	reason: string;
}

// ---------------------------------------------------------------------------
// A11　手動開立發票（端點 57 `POST /admin/invoices/issuances`）
// ---------------------------------------------------------------------------

/** 開票人。`kind === "SELLER"` 時 `storeId` 必填。 */
export interface IssuanceIssuer {
	kind: string;
	storeId: string;
}

/**
 * 受票人。欄位依 `kind` 取用：`BUYER` 用 `buyerId`、`SELLER` 用 `storeId`、
 * `EXTERNAL` 用其餘手填欄。
 *
 * ⚠️ `carrierValue` 是 P 類個資 ⇒ 送出後不回顯（成功畫面不印它）。
 */
export interface IssuanceRecipient {
	kind: string;
	buyerId: string;
	storeId: string;
	name: string;
	taxId: string;
	email: string;
	carrierType: string;
	carrierValue: string;
}

/** 品項來源。`ORDER_SNAPSHOT` 只帶 `orderNo`；`MANUAL_LINES` 只帶 `lines`。 */
export interface IssuanceSource {
	kind: string;
	orderNo: string;
	lines: InvoiceLine[];
}

/**
 * 端點 57 的 request body（Gateway `CreateManualIssuanceForAdminRequest`）。
 *
 * `Idempotency-Key` 是 **header** 不是 body 欄位，格式受 gateway
 * `middleware.RequireIdempotencyKey` 強制（`^[A-Za-z0-9._:-]{8,128}$`，不符即 400）。
 */
export interface ManualIssuanceRequest {
	issuer: IssuanceIssuer;
	recipient: IssuanceRecipient;
	source: IssuanceSource;
	documentKind: string;
	reason: string;
	confirmed: boolean;
}

/**
 * 端點 57 的回應 data（Gateway `CreateManualIssuanceForAdminData`）。
 *
 * 🔴 **`status` 的值域隨 `code` 改變**（invoice-service `admin_write.go` 的
 * `CreateManualIssuanceForAdmin` ＋ `usecase/admin_issuance.go` 的 `Create`）：
 *   - `code === 0`　　 → `status` ＝ 剛建立的 **operation** 狀態（`READY`），
 *     值域是 `InvoiceOperationStatus`
 *   - `code === 9347` → `status` ＝ 既有 **document** 的狀態，值域是
 *     `InvoiceDocumentStatus`
 * 兩個值域沒有交集，同一個欄位查錯表會靜默落回原碼值。9347 的處置是導向 A13
 * ⇒ 本頁只在 `code === 0` 時渲染它，並且查 operation 表。
 */
export interface ManualIssuanceResultData {
	jobId: string;
	documentId: string;
	status: string;
	auditLogId: string;
}

// ---------------------------------------------------------------------------
// ADM 寫入端點的回應（40／41／52／53／54／55／56／60／49）
// ---------------------------------------------------------------------------
//
// 這一組在 2026-09-02 補上。它們原本不在本檔，因為那時的判斷是「後端沒實作」——
// 而那個判斷來自用**猜的路徑名**去探測（`complaints/{id}/decisions` 回 404，真名是
// `.../actions`）。九支 RPC 實際上全在，見 `features/invoices/admin-actions.ts` 的檔頭。

/**
 * 端點 40／41 的回應（Gateway `DecideTaxReviewForAdminData`）。
 *
 * ⚠️ `version` 是**裁決之後**的新版本。頁面拿它回填沒有意義（下一個動作要用當下
 * 讀到的那筆），所以成功後一律 refetch，不拿這個值當樂觀鎖的下一發。
 * ⚠️ `revokedEffectiveAt` 只有 `REVOKE` 會帶：它是「已代製收據不追溯」的分界時點。
 */
export interface DecideTaxReviewForAdminData {
	storeId: string;
	reviewStatus: string;
	version: string;
	auditLogId: string;
	revokedEffectiveAt: string;
}

/**
 * 端點 52／53／56 的共用回應（Gateway `InvoiceJobMutationData`）。
 *
 * ⚠️ `nextStep` 是後端下發的一句話（「接下來會發生什麼」），前端**原樣顯示不改寫**：
 * 同一個 status 在不同 operationType 下的下一步不同，前端推不出來。
 */
export interface InvoiceJobMutationData {
	jobId: string;
	status: string;
	version: string;
	auditLogId: string;
	lastProviderCheckAt: string;
	providerRequestId: string;
	nextStep: string;
}

/**
 * 端點 60 回應裡的停權建議（Gateway `SuspensionRecommendation`）。
 *
 * 🔴 **它只是建議**：發票模組不寫任何停權狀態、不發停權事件、不排恢復 job
 * （`admin_complaint.go` 的 ③「沒有第三件」）。畫面上必須讓人看得出「按下去沒有人被
 * 停權」，否則 ADM 會以為處分已經生效而不去治理頁。
 *
 * 措施由賣家治理模組的 `CreateMeasure` 建立：建議卡的「依建議建立措施」在同一頁開啟建立措施
 * 對話框，賣家是這個物件的 `storeId`（沒有時用申訴案件的 `storeId`）。
 */
export interface SuspensionRecommendation {
	shouldSuspend: boolean;
	storeId: string;
	violationCountIn90Days: string;
	thresholdReached: boolean;
	suggestedSuspendDays: string;
	suggestedReason: string;
}

/**
 * 端點 60 的回應（Gateway `ActOnInvoiceComplaintForAdminData`）。
 *
 * ⚠️ `recommendation` 在 proto 上是 message 不是 optional ⇒ JSON 上永遠是物件。
 * 「這次沒有建議」的判準是 `storeId` 為空字串（只有 `DECIDE_VIOLATION` 與
 * `RECOMMEND_SUSPENSION` 兩個 action 會真的算它），不是 `=== null`。
 */
export interface ActOnInvoiceComplaintForAdminData {
	complaintId: string;
	status: string;
	version: string;
	recommendation: SuspensionRecommendation;
	auditLogId: string;
}

/**
 * 端點 49 的回應（Gateway `ResendInvoiceNotificationData`）。
 *
 * ⚠️ `toMasked` 是後端遮好的收件人，前端不還原也不補遮。
 */
export interface ResendInvoiceNotificationData {
	resent: boolean;
	sentAt: string;
	toMasked: string;
}
