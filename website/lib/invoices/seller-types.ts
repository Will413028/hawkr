/**
 * 賣家面電子發票的 wire 型別鏡像（58 號《電子發票服務》§6.1.7–§6.1.11，
 * gateway `handler/invoice_dto.go` 的 `Seller*` 系列）。
 *
 * 沿用 `./types.ts` 檔頭那兩條規約，不重述：
 *   1. 從 wire 讀進來的 enum 欄位一律 `string`（收窄成 CodeMeta union 會讓
 *      display map 的 default 分支在型別上不可達，而那個分支是白畫面的唯一防線）。
 *   2. id、金額、計數一律字串（TSID 與金額都不能進 `Number`）。
 *
 * 🔴 第三條是賣家面獨有的：**`intakeLimit` 與 `selfIssueAttestation` 在 wire 上
 * 永遠存在，不會是 `null`**。Gateway 兩處都是**值型別的 struct**（不是指標、
 * 沒有 `omitempty`），所以「沒有接單上限」在 wire 上長成「每一欄都是空字串」。
 * 用 `x == null` 判有無會恆為 false ⇒ 一律走本檔的 `hasIntakeLimit()` /
 * `isSelfIssueAccepted()`，不要在畫面上自己判。
 */

import type {
	InvoiceDocumentSummary,
	InvoiceLine,
	InvoiceTimelineItem,
} from "./types";

/** `response.Page` 的三個欄位在 wire 上是數字字串（見 §B-2.0 T25）。 */
export interface SellerPagedResult<T> {
	records: T[];
	/** 總筆數，**不是頁數**。換算成頁數要 `Math.ceil(total / pageSize)`。 */
	total: number;
	page: number;
	pageSize: number;
}

/**
 * 端點 30 的分頁結果：標準四欄 ＋ 一個布林。
 *
 * `shipmentStatusDegraded === true` ＝ 這一頁的 `shipmentStatus` 沒有取到
 * （shipping-service 不可用／逾時），**不是**「這些訂單都還沒出貨」。
 * 降級是整批的性質（後端一次 RPC 涵蓋整頁），所以它在分頁層而不是逐列。
 */
export interface SellerInvoiceDocumentsResult
	extends SellerPagedResult<SellerInvoiceDocumentItem> {
	shipmentStatusDegraded: boolean;
}

/* ===== 端點 16｜總覽 ===== */

/**
 * 稅務審核未完成期間的接單上限（58 §7.3.1）。
 *
 * 見檔頭第 3 條：這是值型別 struct，「沒有上限」＝五欄全空 ＋ `blocked === false`。
 */
export interface SellerIntakeLimit {
	orderCountUsed: string;
	orderCountLimit: string;
	amountUsed: string;
	amountLimit: string;
	blocked: boolean;
}

/** 有沒有接單上限這回事——判準是「有沒有給出上限值」，不是有沒有這個物件。 */
export function hasIntakeLimit(limit: SellerIntakeLimit): boolean {
	return limit.orderCountLimit !== "" || limit.amountLimit !== "";
}

/** 端點 16 `GET /app/seller/invoices/overview`（`SellerInvoiceOverviewData`）。 */
export interface SellerInvoiceOverview {
	/** `SellerTaxReviewStatus`（10 DB 值 ＋ 合成的 `NOT_STARTED`）。 */
	taxReviewStatus: string;
	/** `InvoiceFulfillmentMode`。 */
	invoiceMode: string;
	/** `InvoiceProviderAccountStatus`（11 DB 值 ＋ 2 投影值）。 */
	providerStatus: string;
	pendingDocumentCount: string;
	openComplaintCount: string;
	overdueComplaintCount: string;
	/** 稅務審核期限（ISO UTC）；非待審類狀態時為空字串。 */
	deadlineAt: string;
	extensionCount: string;
	canRequestExtension: boolean;
	selfIssueEffective: boolean;
	intakeLimit: SellerIntakeLimit;
	/**
	 * 固定 5 筆、不接受 query 調整。用的是 `InvoiceDocumentSummary`，
	 * **沒有 `orderNo`** ⇒ 這裡的列不得反推訂單連結（S01 ⑤）。
	 */
	recentDocuments: InvoiceDocumentSummary[];
}

/* ===== 端點 17｜稅務身分 ===== */

/** 稅籍資料（`SellerTaxIdentity`）。 */
export interface SellerTaxIdentity {
	/** `SellerTaxEntityType`。 */
	entityType: string;
	taxId: string;
	legalName: string;
	responsibleName: string;
	addressCity: string;
	addressDistrict: string;
	addressDetail: string;
	/**
	 * `SellerTaxObligation`（`TAXABLE`／`EXEMPT`）——賣家**自述**的類型。
	 * 🔴 與 `reviewStatus` 是兩個維度，畫面必須分開顯示：自述免用 ≠ 已核定免用，
	 * 後者是 ADM 的裁決（S02 ⑤）。
	 */
	claimedStatus: string;
}

/**
 * 一份稅籍證明文件（`TaxDocumentInfo`）。
 *
 * ⚠️ **沒有 `fileDownloadPath` 這一欄**——58A §S02 的欄位表寫的是它，實際 DTO 給的
 * 是 `fileAssetId`，下載路徑要由前端用 `sellerTaxDocumentHref()` 組（形狀與
 * `certification.go` 的 `secure_download` 一致）。已回報，未自行改規格。
 *
 * `fileKey` 是 media objectKey：**不顯示、不組 URL**，但必須逐筆回填端點 18 的
 * `evidenceObjectKeys[]` 保留清單（§B-2.0 T07）。
 */
export interface SellerTaxDocument {
	documentId: string;
	kind: string;
	fileName: string;
	/** bytes，數字字串。畫面自行換算 KB／MB。 */
	fileSize: string;
	fileKey: string;
	uploadedAt: string;
	fileAssetId: string;
}

/** ADM 補件備註（`TaxReviewNote`）。 */
export interface SellerTaxReviewNote {
	noteId: string;
	content: string;
	createdAt: string;
}

/**
 * 自行開票責任條款的具結狀態（`SelfIssueAttestationInfo`）。
 *
 * 見檔頭第 3 條：值型別 struct，未具結 ＝ `acceptedAt` 空字串 ＋ `effective false`。
 */
export interface SelfIssueAttestation {
	/** 這個賣家**已簽**的版本；未簽為空字串。 */
	termsVersion: string;
	termsKey: string;
	acceptedAt: string;
	acceptedByAccountId: string;
	effective: boolean;
	/**
	 * **當期生效**的條款版本，恆非空。
	 *
	 * 🔴 與 `termsVersion` 是兩件事，不可互相取代：一個是「簽了哪一版」（未簽為空），
	 * 一個是「現在要簽的是哪一版」。端點 22 要求原樣回送本欄（不符即 400）⇒
	 * 拿 `termsVersion` 去送，未簽過的賣家送出的是空字串，必被拒。
	 */
	currentTermsVersion: string;
}

/** 賣家是否已同意過自行開票責任條款（端點 19 的前置條件）。 */
export function isSelfIssueAccepted(
	attestation: SelfIssueAttestation,
): boolean {
	return attestation.acceptedAt !== "";
}

/** 端點 17 `GET /app/seller/invoices/tax_profile`（`SellerTaxProfileData`）。 */
export interface SellerTaxProfile {
	storeId: string;
	/** `SellerTaxReviewStatus`。 */
	reviewStatus: string;
	/** 樂觀鎖版本；端點 18／19／20 的 `expectedVersion` 只能來自這一欄。 */
	version: string;
	submittedAt: string;
	deadlineAt: string;
	extensionCount: string;
	canRequestExtension: boolean;
	/** `REJECTED`／`EXPIRED` 的重新申請冷卻終點；空字串代表沒有冷卻。 */
	reapplyCooldownUntil: string;
	taxIdentity: SellerTaxIdentity;
	documents: SellerTaxDocument[];
	admNotes: SellerTaxReviewNote[];
	selfIssueAttestation: SelfIssueAttestation;
	intakeLimit: SellerIntakeLimit;
	/**
	 * 尚未被平台裁決的展延申請（端點 20 → 41）；沒有時為 `null`。
	 *
	 * 🔴 **它是賣家判斷「我剛才那一按有沒有送出去」的唯一依據。** 端點 20 只寫一列
	 * 稽核歷程、不碰 `seller_tax_profiles` 的任何一欄 ⇒ 在這一欄出現之前
	 * （2026-09-05），按下「申請延長期限」之後 `extensionCount` 與 `deadlineAt`
	 * 都不變、`canRequestExtension` 仍是 `true`，畫面與按之前**一模一樣**。
	 */
	pendingExtensionRequest: SellerTaxExtensionRequest | null;
}

/** 一筆尚未裁決的展延申請。 */
export interface SellerTaxExtensionRequest {
	/** ISO 8601 UTC。 */
	requestedAt: string;
	/** 送出時填的原因。 */
	reason: string;
}

/* ===== 端點 30／31｜發票與收據紀錄 ===== */

/** 端點 30 的一列（`SellerInvoiceDocumentItem`）。 */
export interface SellerInvoiceDocumentItem {
	summary: InvoiceDocumentSummary;
	orderNo: string;
	paidAt: string;
	/** `InvoiceRecipientType`。 */
	buyerKind: string;
	/** `InvoiceFulfillmentMode`。 */
	invoiceMode: string;
	/**
	 * 出貨狀態。🔴 與 `summary.status`（發票狀態）是**兩個彼此獨立的維度**，
	 * 畫面必須是兩顆 Chip，不得合成「綜合狀態」（S06 表格第 6／7 欄）。
	 */
	shipmentStatus: string;
	updatedAt: string;
	/**
	 * 最近一筆 operation 的 id；**空字串代表這份文件沒有任何 operation**
	 * ⇒「查看異常」整顆不渲染，不要渲染 disabled 鍵，也不要拿 `documentId`
	 * 去組 `/jobs/{jobId}`（會 404＋9340，與「不是你的」長得一模一樣，§B-2.0 T06）。
	 */
	latestJobId: string;
}

/**
 * 端點 31 `GET /documents/{document_id}`（`SellerInvoiceDocumentDetail`）。
 *
 * ⚠️ **沒有 `invoiceMode`**：那一欄只在端點 30 的列上。S07 的兩段查詢因此不是
 * 為了省時間而是必要的——`<dl>` 的「開票方式」只能從第 1 段拿。
 *
 * ⚠️ **沒有 `notifiedEmailMasked`**（那一欄在買家端點 8）⇒ 該列整列不渲染，
 * 不得用賣家自己的 email 頂替（§B-2.0 T21）。
 */
export interface SellerInvoiceDocumentDetail {
	summary: InvoiceDocumentSummary;
	issuerName: string;
	issuerTaxId: string;
	recipientName: string;
	recipientTaxId: string;
	salesAmount: string;
	taxAmount: string;
	totalAmount: string;
	/** `InvoiceTaxCategory`。 */
	taxCategory: string;
	/** `InvoiceLineType` 三值；`DISCOUNT` 的 `netAmount` 是負數。 */
	lines: InvoiceLine[];
	timeline: InvoiceTimelineItem[];
	/** `SELLER`／`PLATFORM`。 */
	createdBy: string;
	/**
	 * `SELLER`／`PLATFORM`／`UNDETERMINED` **三值不是一個常數**：`UNDETERMINED`
	 * 是偵測到 ADMIN 券之後 fail-closed 轉人工，畫面要顯示「待平台確認」而不是
	 * 空白或「賣家負擔」。
	 */
	fundedBy: string;
	cutoverVersion: string;
	providerRequestId: string;
	/** `InvoiceManualReason`；`AWAITING_MANUAL` 時的 banner 說明。 */
	manualReasonCode: string;
	/**
	 * 下次可重寄通知的時間。⚠️ **後端目前恆為空字串**（實測 2026-08-30），而空字串
	 * 在前端語意是「可以重寄」⇒ **不得拿它做倒數或 disable**，那會是一顆永遠可按的鍵。
	 *
	 * 端點 32 本身已實作、S11 的重寄鍵也已接上（`SellerInvoiceDetailPage`）：節流由
	 * 後端的 `9352` 表達，前端在拿到它時才說「剛剛才寄過」。這一欄留著只為型別完整，
	 * 有值之前不要有任何呼叫端。
	 */
	nextResendAvailableAt: string;
	latestJobId: string;
}

/* ===== 端點 33／62R｜開票作業（job） ===== */

/**
 * 系統自動重試的政策（`RetryPolicy`）。四欄都是**數字字串**。
 *
 * 這四欄是特意寫進 DTO 的：S08 ④ 必須把它們**真的顯示出來**（「首次間隔幾秒、
 * 每次加倍、最長間隔、最多幾次」），不是塞進 `<details>` —— 賣家看到「已嘗試
 * 3/5 次」而不知道下一次什麼時候、還會不會再試，就只能去按一顆不存在的重試鍵。
 */
export interface InvoiceRetryPolicy {
	initialIntervalSeconds: string;
	backoffMultiplier: string;
	maxIntervalSeconds: string;
	maxAttempts: string;
}

/* ===== 端點 62R｜本訂單的自行開立案件 ===== */

/**
 * 端點 62R `GET /orders/{order_no}/external_issuances` 的一列（`InvoiceJobInfo`）。
 *
 * 這是 S07 ⑦ 的唯一資料源，而且是**停權賣家唯一拿得到的那一支**——所以它的 hook
 * 不得掛 `!isSuspended`，`enabled` 只能看 `orderNo`（§B-2.0 T05）。
 */
export interface InvoiceJobInfo {
	jobId: string;
	orderNo: string;
	storeId: string;
	/** `InvoiceOperationType`。 */
	operationType: string;
	/** `InvoiceOperationStatus`。 */
	status: string;
	summary: string;
	attemptCount: string;
	maxAttempts: string;
	lastAttemptAt: string;
	nextRetryAt: string;
	/**
	 * ⚠️ 逐列的 `maxAttempts` 取自那一列（建列當下寫入），**與 `retryPolicy.maxAttempts`
	 * 不必相等**：舊列可能是舊政策值。④ 的「已嘗試 N／M 次」用的是 `maxAttempts`
	 * 那一欄，政策說明句用的是 `retryPolicy`——兩者混用會讓「還剩幾次」算錯。
	 */
	retryPolicy: InvoiceRetryPolicy;
	shipmentUnaffectedNotice: string;
	providerRequestId: string;
	version: string;
	/** `InvoiceManualReason`。 */
	manualReasonCode: string;
	externalInvoiceNo: string;
	externalInvoiceDate: string;
	reportedAt: string;
	/**
	 * 🔴 直接讀這個布林，**不得從 `status` 自己推導**：今天它只依賴 status，
	 * 明天加上「解鎖已逾期失效」之類條件時，前端那份推導會靜默過期
	 * ——按鈕還在、按下去 409（§B-2.0 T23）。
	 */
	canReportExternal: boolean;
	/**
	 * `InvoiceDocumentKind`：`B2B` / `B2C` / `RECEIPT`。
	 *
	 * 🔴 收據與發票在賣家中心是**兩個並列的東西**（backend#3066 裁決 B）。免用統一
	 * 發票的賣家永遠不會有發票 ⇒ 對他顯示任何帶「發票」字樣的框架都是錯的。
	 * **不得從 `operationType` 推導** —— 那個 enum 沒有 RECEIPT（收據是 documentKind）。
	 */
	documentKind: string;
}

/* ===== 端點 34／35｜買家發票申訴 ===== */

/**
 * 申訴的一則處理紀錄（`ComplaintResponseItem`）。**追加語意**：端點是
 * `POST .../responses`（建立子資源）不是 `PUT .../response` ⇒ 既有紀錄不可編輯、
 * 不可刪除，畫面上也不提供這兩個入口。
 */
export interface SellerInvoiceComplaintResponse {
	responseId: string;
	/** `SELLER`／`ADMIN`／`BUYER`。 */
	actor: string;
	/** `InvoiceComplaintAction`（6 值）。 */
	action: string;
	note: string;
	occurredAt: string;
}

/**
 * 端點 34（列一列）與端點 35（整份）共用的 `InvoiceComplaintInfo`。
 *
 * 🔴 **四個欄位後端目前恆為空字串**（2026-08-30 逐欄實查 invoice-service 的
 * `toComplaintInfo`，`services/invoice-service/internal/grpc/query.go`）：
 *
 *   `caseNo`          —— `invoice_complaints` 沒有對外案號欄，後端自陳「與
 *                        complaint_id 同源時前端顯示 id 即可」
 *   `storeId`         —— 該 view 未帶 issuer
 *   `expectedDocKind` —— `toComplaintInfo` 從未寫入這一欄
 *   `buyerMaskedName` —— 同上
 *
 * ⇒ 依 §B-2.0 T21 的同一條規則：**該列整列不渲染**，不得用別的值頂替（頂一個假的
 * 買家名字，賣家會照著它去回覆一個不存在的人）。這四欄留在型別上是為了後端補上
 * 之後版面不必再動。58A §S09／§S10 的欄位表把它們畫進版面，已回報衝突、未自行改規格。
 */
export interface SellerInvoiceComplaint {
	complaintId: string;
	/** 後端恆空——見介面檔頭。頁首退回顯示 `complaintId`。 */
	caseNo: string;
	orderNo: string;
	/** 後端恆空——見介面檔頭。 */
	storeId: string;
	/** `InvoiceComplaintStatus`（7 值）。 */
	status: string;
	/** 後端恆空——見介面檔頭。 */
	expectedDocKind: string;
	dueAt: string;
	/**
	 * 🔴 **數字字串、可為負、可為空**，前端零計算（§0.6 C10）。折疊一律走
	 * `lib/invoices/slaCountdown.ts`，不要就地 `Number()` 比大小。
	 */
	remainingBusinessDays: string;
	overdueBusinessDays: string;
	/** 後端恆空——見介面檔頭。 */
	buyerMaskedName: string;
	/** 買家提出申訴時填的說明（可為空，端點 13 的 `description` 是選填）。 */
	description: string;
	responses: SellerInvoiceComplaintResponse[];
	/**
	 * 樂觀鎖版本；端點 36 的 `expectedVersion` 只能來自這一欄。
	 * **不得用 `responses.length` 頂替**——那不是樂觀鎖，兩個分頁同時回覆會互相
	 * 覆蓋而沒有任何人發現。
	 */
	version: string;
	createdAt: string;
	/** `"0"`–`"2"`。**這是屬性不是狀態**：不要為它做第四個分頁，也不要拿它當 status 判斷。 */
	reopenCount: string;
	/** 只有 `SELLER_RESPONDED` 時後端才填。 */
	buyerReviewDueAt: string;
	/** 只有 `RESOLVED` 時後端才填。 */
	reopenAllowedAfter: string;
}

/* ===== 端點 72｜進項發票（平台開給賣家的） ===== */

/**
 * 端點 72 `GET /app/seller/invoices/received_documents` 的一列
 * （`SellerReceivedInvoiceItem`）。
 *
 * 🔴 與端點 30（銷項）是**不同的法律角色**，不可混進同一個列表：銷項是我開給買家的
 * （可作廢、可折讓），進項是平台開給我的報稅憑證（**唯讀**，我沒有權限也沒有法律
 * 地位去作廢它）。S12 因此是兩個 tab 而不是一個列表加篩選。
 *
 * `summary.invoiceNumber` 對進項是**完整不遮**（58 §8.1.3(c) 裁定 1：發票號碼是對獎／
 * 報帳／申訴的唯一憑據，同時印在發票上、寄在信裡、存在載具裡，遮它保護不到任何人）。
 */
export interface SellerReceivedInvoiceItem {
	summary: InvoiceDocumentSummary;
	/** 開立人（平台）的抬頭快照。 */
	issuerName: string;
	/** 用途說明，由明細行推導（例：2026 年 8 月平台服務費）。 */
	recipientNote: string;
}

/**
 * AMEGO 憑證帳號（58 §6.1 端點 23，S03）。
 *
 * 欄位逐一抄自 `gateway/internal/handler/invoice_dto.go` 的
 * `SellerProviderAccountData`（§0.3：跨邊界的型別不得手寫想像）。
 *
 * 🔴 **這裡沒有、也不會有 AppKey 或憑證指紋。**
 * AppKey 以 AES-GCM 加密存於後端，解密只發生在 AMEGO client 的呼叫路徑上；
 * `credential_fingerprint_prefix` 在 `api/proto/invoice/invoice.proto` 是
 * `reserved`，註解逐字寫明依 §8.1.1(f) U2「不要加」。
 * ⚠️ 58A §S03 的畫面規格仍寫著「指紋前 8 碼」——**那份是舊的**，契約 SSOT 是 proto，
 * 前端不得為了對齊畫面稿去顯示一個拿不到的值。
 */
export interface SellerProviderAccount {
	provider: string;
	/** 11 個 DB 值 ＋ 2 個投影值（`NOT_STARTED`／`CONFIG_ERROR`），對照表見 S03。 */
	status: string;
	/** 後端給的人話 ＋ 下一步。**前端不重寫**（58A §S03）。 */
	statusNotice: string;
	/** 樂觀鎖；四支 mutation 的 `expectedVersion` 都取這個值。 */
	version: string;
	/**
	 * 核定統編的顯示值，不可編輯。**完整 8 碼，不是遮罩值**——
	 * 58 號 §8.1.3(b) 把賣家統編判為 O 類、三端皆完整（公開營業資訊）。
	 * 欄名 2026-09-02 由 `sellerTaxIdMasked` 改名（#3017）：舊名帶著 masked
	 * 卻回完整值，會讓下游誤以為已經遮過而可以安全轉發。
	 */
	sellerTaxId: string;
	credentialLast4: string;
	credentialUpdatedAt: string;
	lastVerifiedAt: string;
	/** 自動開票切點。**步驟 4 與 5 的分界看它有沒有值，不看 `status`。** */
	automationCutoverAt: string;
	consentVersion: string;
	/** 供客服追查的服務商 request id；驗證失敗時展開 `<details>` 顯示。 */
	lastRequestId: string;
	pendingOrderCount: string;
}

/**
 * 免用統一發票與收據代製授權（58 §6.1 端點 28，S05）。
 *
 * 欄位逐一抄自 gateway 的 `SellerReceiptAuthorizationData`。
 *
 * 🔴 **本型別沒有、也不該有「切換開票方式」的欄位。** 免用資格是國稅局核定 ＋
 * ADM 審核的結果，API 層根本沒有讓賣家寫入的端點 ⇒ 畫面上不得出現任何 toggle、
 * radio 或「改用收據」入口。
 *
 * ⚠️ 58A §S05 的 DTO 欄位表寫了 `approvalBasisDownloadPath` 與 `receiptTemplateUrl`
 * 兩個欄位，**契約上都不存在**：後端給的是 `approvalBasisAssetId`（要自己組授權下載
 * 路徑，形狀照 `sellerTaxDocumentHref`）與 `receiptTemplateKey`（objectKey，走
 * `resolveCdnUrl`）。以契約為準。
 */
export interface SellerReceiptAuthorization {
	/** 決定整頁是 gate 還是內容。 */
	eligible: boolean;
	/** `NOT_APPROVED`／`APPROVAL_REVOKED`／`APPROVAL_EXPIRED`。 */
	ineligibleReason: string;
	/** objectKey。**不直接顯示**——下載走 `approvalBasisAssetId`。 */
	approvalBasisKey: string;
	approvedAt: string;
	approvalValidUntil: string;
	/** 非空 ＝ 已授權 ⇒ ⑥ 換成唯讀。 */
	authorizedAt: string;
	consentVersion: string;
	/** 平台自己的收據範本 objectKey（無 PII，公開 CDN 是刻意例外）。 */
	receiptTemplateKey: string;
	/** 樂觀鎖；端點 29 的 `expectedVersion`。 */
	version: string;
	approvalBasisAssetId: string;
	/**
	 * 🔴 **「查看」渲不渲染看這個布林，不看 `approvalBasisAssetId` 非空**：
	 * asset 可能存在但這個賣家無權下載（`UPLOADER_NOT_SELLER`），此時組得出路徑
	 * 卻點下去必定失敗——做出一顆註定 404 的按鈕比不給按鈕更糟。
	 */
	approvalBasisAvailable: boolean;
	/** `NOT_DESIGNATED`／`UPLOADER_NOT_SELLER`。兩者對賣家的文案相同。 */
	approvalBasisUnavailableReason: string;
}

/**
 * 端點 18 的 `taxIdentity`（欄位逐一抄自 gateway 的 `SellerTaxIdentity`）。
 *
 * 與讀取用的 `SellerTaxIdentity` 是**同一組欄位**，但刻意分開命名：讀回來的那份
 * 是平台核定後的結果，這份是賣家自述的輸入。兩者在 S02 上會同時出現。
 */
export interface SellerTaxIdentityInput {
	/** `FARMER`／`SOLE_PROPRIETOR`／`PARTNERSHIP`／`COMPANY`／`OTHER`。 */
	entityType: string;
	/** 統一編號 8 碼；免用統一發票的自然人賣家可留空。 */
	taxId: string;
	legalName: string;
	responsibleName: string;
	addressCity: string;
	addressDistrict: string;
	addressDetail: string;
	/** 賣家**自述**的開票義務：`REQUIRED`／`EXEMPT`／`UNKNOWN`。 */
	claimedStatus: string;
}
