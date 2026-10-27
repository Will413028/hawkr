/**
 * Seller-side invoice API (58 號《電子發票服務》§6.1.7–§6.1.11)。
 *
 *   GET  /app/seller/invoices/overview                                — 端點 16  總覽（S01）
 *   GET  /app/seller/invoices/tax_profile                             — 端點 17  稅務身分（S02）
 *   PUT  /app/seller/invoices/tax_profile/draft                       — 端點 18  存草稿（S02）
 *   POST /app/seller/invoices/tax_profile/submissions                 — 端點 19  送出審核（S02）
 *   POST /app/seller/invoices/tax_profile/extension_requests          — 端點 20  申請延長（S02）
 *   GET  /app/seller/invoices/receipt_authorization                   — 端點 28  收據授權（S05）
 *   POST /app/seller/invoices/receipt_authorization/confirmations     — 端點 29  確認授權（S05）
 *   GET  /app/seller/invoices/self_issue_attestations                 — 端點 21  自行開票條款（S04）
 *   POST /app/seller/invoices/self_issue_attestations                 — 端點 22  補簽條款（S04）
 *   GET  /app/seller/invoices/provider_account                        — 端點 23  AMEGO 帳號（S03）
 *   PUT  /app/seller/invoices/provider_account/credentials            — 端點 24  設定 AppKey（S03）
 *   POST /app/seller/invoices/provider_account/verifications          — 端點 25  連線測試（S03）
 *   POST /app/seller/invoices/provider_account/activations            — 端點 26  設定啟用切點（S03）
 *   POST /app/seller/invoices/provider_account/rotations              — 端點 27  輪替 AppKey（S03）
 *   GET  /app/seller/invoices/documents                               — 端點 30  發票紀錄（S06／S07 段 1）
 *   GET  /app/seller/invoices/documents/{document_id}                 — 端點 31  發票明細（S07 段 2／S11）
 *   POST /app/seller/invoices/documents/{id}/notification_resends      — 端點 32  重寄通知（S11）
 *   GET  /app/seller/invoices/received_documents                      — 端點 72  進項發票（S12 進項分頁）
 *   GET  /app/seller/invoices/jobs/{job_id}                           — 端點 33  發票異常詳情（S08）
 *   GET  /app/seller/invoices/complaints                              — 端點 34  買家申訴列表（S09）
 *   GET  /app/seller/invoices/complaints/{complaint_id}               — 端點 35  申訴詳情（S10）
 *   POST /app/seller/invoices/complaints/{id}/responses                — 端點 36  回覆申訴（S10 ⑥）
 *   POST /app/seller/invoices/jobs/{job_id}/external_issuances        — 端點 62  回報自行開立（S07 ⑦／S08 ⑤）
 *   GET  /app/seller/invoices/orders/{order_no}/external_issuances    — 端點 62R 本訂單的自行開立案件（S07 ⑦）
 *
 * 🔴 **只有本檔 import 傳輸層**（§B-2.0 T01）。`features/seller-center/**` 被
 * `biome.json` 的 `noRestrictedImports` 列為禁區，三層是 repo → `lib/hooks/useSellerInvoice*`
 * → feature，形狀照 `lib/seller/measures.ts`。
 *
 * ─────────────────────────────────────────────────────────────────────────
 * ⚠️ 賣家面**寫入**端點的現況（2026-08-31 重查，取代前一版「一支都沒有」的敘述）：
 *
 *   端點 18 `SaveSellerTaxProfileDraft`      端點 19 `SubmitSellerTaxProfile`
 *   端點 20 `RequestSellerTaxExtension`      端點 22 `AcceptSelfIssueAttestation`
 *     ↳ 四支都已實作（`internal/grpc/tax_profile.go`）**且四支的 UI 都已接上**：
 *       18／19／20 在 `tax-status/`（`TaxIdentityForm` ＋ `TaxSubmitSection` ＋
 *       `TaxExtensionSection`）、22 在 `self-issue-terms/SelfIssueTermsPage`。
 *       稅籍證明走 secure 兩段式上傳（`lib/media/secure-upload.ts`），**不得**改用
 *       `uploadImageFile`——那條走公開通道，會讓稅籍文件變成任何人拿到 key 就下得到。
 *   端點 24–27 provider credentials／驗證／啟用／輪替
 *     ↳ **2026-09-01 重查已全部實作**（`internal/grpc/provider_account.go` ＋
 *       `query.go` 的 `GetSellerProviderAccount`）⇒ S03 已接上，見本檔尾段。
 *   端點 28／29 收據代製授權（`GetSellerReceiptAuthorization`／
 *     `ConfirmSellerReceiptAuthorization`，`internal/grpc/receipt_authorization.go`）
 *   端點 32 重寄通知（`ResendInvoiceNotification`，`internal/grpc/notification_resend.go`）
 *   端點 36／62（`RespondSellerInvoiceComplaint`／`ReportExternalIssuance`，
 *     `internal/grpc/seller_write.go`）
 *     ↳ **三組已全數接上**：端點 28／29（S05）、端點 32（S06／S11 的重寄鍵）、
 *       端點 36（S10 ⑥）、端點 62（S07 ⑦／S08 ⑤，2026-09-02）。
 *
 * 🔴 **2026-09-01：賣家面已無任何未實作的 RPC。** 判準改成從呼叫端反推，不要憑
 * 記憶猜 RPC 名——我曾用 `AuthorizeSellerReceipt`／`ResendSellerInvoiceNotification`
 * 兩個**自己編的名字**去查，兩次都查不到，於是把兩個做得好好的端點寫成「仍缺」。
 * 正確做法（可直接跑）：
 *     grep -ohE 'h\.Invoice\.[A-Za-z]+\(' gateway/internal/handler/invoice_seller.go \
 *       | sed 's/h.Invoice.//;s/(//' | sort -u
 *   把那份清單逐支拿去 `git grep 'func (s \*InvoiceServer) <名字>(' origin/dev`。
 *   **否定結論比肯定結論更需要證明「查詢打到了對的東西」**，而「名字是我編的」
 *   正是查詢打不到的最常見原因。
 *
 * gRPC `Unimplemented` 經 `response.GrpcErr` 對外是 **503**。這條原則仍然成立
 * （真的缺的端點不寫函式、也不渲染會打它的按鈕），但**目前沒有任何一支適用**。
 *
 * 🔴 **解除條件是可機械檢查的**，不要憑印象判斷「後端好了沒」，也不要憑印象判斷
 * 「前端接了沒」——這兩個問題被混為一談過不只一次。查法：
 *     後端有沒有：git cat-file -e origin/dev:services/invoice-service/internal/grpc/<檔名>.go
 *     前端接沒接：rg -n '<repo 函式名>' --glob '!node_modules'（**要有呼叫端，不是只有定義**）
 * ─────────────────────────────────────────────────────────────────────────
 */

import { apiClient } from "../api/client";
import type {
	InvoiceJobInfo,
	SelfIssueAttestation,
	SellerIntakeLimit,
	SellerInvoiceComplaint,
	SellerInvoiceComplaintResponse,
	SellerInvoiceDocumentDetail,
	SellerInvoiceDocumentItem,
	SellerInvoiceDocumentsResult,
	SellerInvoiceOverview,
	SellerPagedResult,
	SellerProviderAccount,
	SellerReceiptAuthorization,
	SellerReceivedInvoiceItem,
	SellerTaxDocument,
	SellerTaxExtensionRequest,
	SellerTaxIdentity,
	SellerTaxIdentityInput,
	SellerTaxProfile,
	SellerTaxReviewNote,
} from "./seller-types";
import type {
	InvoiceDocumentSummary,
	InvoiceLine,
	InvoiceTimelineItem,
} from "./types";

const BASE = "/app/seller/invoices";

/* ===== 業務碼 ===== */

/**
 * 58 §6.1.0 (13) 的碼表中，本波四個畫面會遇到的那些。文案在 `./errorCopy.ts`。
 *
 * 🔴 一律讀業務碼不讀 HTTP status（§0.4 ①）：`9324`／`9331` 標 `preserveData`，
 * HTTP **恆為 200** 卻仍會被 client 丟成 `ApiClientError`。
 */
export const SELLER_INVOICE_BIZ_CODE = {
	/** 停權賣家打 BLOCK 端點。`lib/api/client.ts` 會同時廣播全域停權事件。 */
	SUSPENDED: 9240,
	/** Tier A：越權與不存在同碼 ⇒ 文案不得寫「沒有權限」。 */
	DOCUMENT_NOT_FOUND: 9301,
	CARRIER_NOT_WON: 9312,
	QUERY_WINDOW_EXPIRED: 9313,
	/** 尚未建立稅務身分資料——**不是錯誤態**，渲染空表單／未開始狀態。 */
	TAX_PROFILE_NOT_FOUND: 9320,
	TAX_STATE_NOT_ALLOWED: 9321,
	TAX_VERSION_CONFLICT: 9322,
	/**
	 * 重新申請冷卻中（`REJECTED`／`EXPIRED` 後 7 天）。
	 *
	 * 🔴 **preserveData：HTTP 恆 200**，但 `apiClient` 仍會丟成 `ApiClientError`。
	 * 倒數時刻在 `data.reapplyCooldownUntil`，少讀它賣家只看到一句紅字。
	 */
	TAX_REAPPLY_COOLDOWN: 9324,
	/** 送審時未勾選自行開票責任條款（端點 19，裁決 C）。 */
	TAX_SELF_ISSUE_TERMS_NOT_ACCEPTED: 9325,
	/** 端點 23 尚無 provider 帳號——**不是錯誤態**，等同 `NOT_STARTED`。 */
	PROVIDER_ACCOUNT_NOT_FOUND: 9330,
	/**
	 * 端點 25 連線測試失敗。
	 *
	 * 🔴 **preserveData：HTTP 恆 200**，仍會被丟成 `ApiClientError`。
	 * `error.data.lastRequestId` 是給客服追查用的，少讀它賣家只看到一句紅字。
	 */
	PROVIDER_VERIFICATION_FAILED: 9331,
	PROVIDER_UNAVAILABLE: 9332,
	/** 端點 26：尚未通過連線測試就按啟用。 */
	PROVIDER_NOT_VERIFIED: 9333,
	/** 端點 24：送出統編與已核定稅籍不符。下一步是回 S02 更正，不是改 AppKey。 */
	PROVIDER_TAXID_MISMATCH: 9334,
	/** 端點 24：這組憑證已被別的賣場綁走 ⇒ 回光貿後台換一組。 */
	PROVIDER_CREDENTIAL_TAKEN: 9337,
	/**
	 * 端點 32：重寄通知過於頻繁（節流）。
	 *
	 * 🔴 **它是 error 不是 `resent:false`**（2026-09-01 dev 實測）。原本我照
	 * `ResendInvoiceNotificationData.resent` 這個布林寫了一個「被節流」分支，
	 * 而後端根本不走那條——第二次呼叫直接回 9352。⇒ 那個分支是死碼，
	 * 賣家看到的會是通用的「寄送失敗」，而真正的原因是「剛剛才寄過」。
	 */
	NOTIFICATION_RESEND_THROTTLED: 9352,
	JOB_NOT_FOUND: 9340,
	/** 端點 62：`expectedVersion` 與這筆作業當前的版本不符 ⇒ 重新整理後再送。 */
	JOB_VERSION_CONFLICT: 9343,
	/**
	 * 端點 62：本作業尚未開放自行開立回報。
	 *
	 * 🔴 只有 `MANUAL_PROCESS_ALLOWED` 這一個狀態接受回報（invoice-service
	 * `SellerJob.ReportExternalIssuance` 逐字），而那要由 ADM 雙人解鎖打開。
	 * ⇒ 入口一律看後端的 `canReportExternal`，**不得自己從 status 推導**：
	 * 今天它只依賴 status，明天多一條「解鎖已逾期」的條件時，那份推導會靜默過期
	 * ——按鈕還在、按下去 9349。
	 */
	EXTERNAL_ISSUANCE_NOT_ALLOWED: 9349,
	/** 相同冪等鍵帶了不同內容 ⇒ 換一把新的重送（呼叫端不要沿用上一把）。 */
	IDEMPOTENCY_PAYLOAD_MISMATCH: 9350,
	/** 這張發票有處理中的作業。**不是失敗**：顯示進度並鎖住重複送出。 */
	OPERATION_IN_PROGRESS: 9351,
	/** Tier A：越權與不存在同碼 ⇒ 文案不得寫「沒有權限」（S10）。 */
	COMPLAINT_NOT_FOUND: 9309,
} as const;

/* ===== normalize ===== */

function str(value: unknown): string {
	return value == null ? "" : String(value);
}

function bool(value: unknown): boolean {
	return value === true;
}

function num(value: unknown): number {
	const parsed = Number(value);
	return Number.isFinite(parsed) ? parsed : 0;
}

function obj(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function arr(value: unknown): Record<string, unknown>[] {
	return Array.isArray(value) ? (value as Record<string, unknown>[]) : [];
}

function normalizeDocumentSummary(
	raw: Record<string, unknown>,
): InvoiceDocumentSummary {
	return {
		documentId: str(raw.documentId),
		documentKind: str(raw.documentKind),
		status: str(raw.status),
		invoiceNumber: str(raw.invoiceNumber),
		issuedAt: str(raw.issuedAt),
		totalAmount: str(raw.totalAmount),
		carrierSummary: str(raw.carrierSummary),
		carrierType: str(raw.carrierType),
		relatedDocumentId: str(raw.relatedDocumentId),
		fileAvailable: bool(raw.fileAvailable),
		fileUnavailableReason: str(raw.fileUnavailableReason),
	};
}

function normalizeLine(raw: Record<string, unknown>): InvoiceLine {
	return {
		lineType: str(raw.lineType),
		name: str(raw.name),
		quantity: str(raw.quantity),
		unitPrice: str(raw.unitPrice),
		netAmount: str(raw.netAmount),
		taxCategory: str(raw.taxCategory),
	};
}

function normalizeTimelineItem(
	raw: Record<string, unknown>,
): InvoiceTimelineItem {
	return {
		occurredAt: str(raw.occurredAt),
		event: str(raw.event),
		note: str(raw.note),
	};
}

function normalizeIntakeLimit(raw: unknown): SellerIntakeLimit {
	const it = obj(raw);
	return {
		orderCountUsed: str(it.orderCountUsed),
		orderCountLimit: str(it.orderCountLimit),
		amountUsed: str(it.amountUsed),
		amountLimit: str(it.amountLimit),
		blocked: bool(it.blocked),
	};
}

/* ===== 端點 16 ===== */

/** 端點 16。唯讀聚合，`recentDocuments` 固定 5 筆、不接受 query 調整。 */
export async function fetchSellerInvoiceOverview(): Promise<SellerInvoiceOverview> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/overview`,
	);
	const raw = obj(env.data);
	return {
		taxReviewStatus: str(raw.taxReviewStatus),
		invoiceMode: str(raw.invoiceMode),
		providerStatus: str(raw.providerStatus),
		pendingDocumentCount: str(raw.pendingDocumentCount),
		openComplaintCount: str(raw.openComplaintCount),
		overdueComplaintCount: str(raw.overdueComplaintCount),
		deadlineAt: str(raw.deadlineAt),
		extensionCount: str(raw.extensionCount),
		canRequestExtension: bool(raw.canRequestExtension),
		selfIssueEffective: bool(raw.selfIssueEffective),
		intakeLimit: normalizeIntakeLimit(raw.intakeLimit),
		recentDocuments: arr(raw.recentDocuments).map(normalizeDocumentSummary),
	};
}

/* ===== 端點 17 ===== */

function normalizeTaxIdentity(raw: unknown): SellerTaxIdentity {
	const it = obj(raw);
	return {
		entityType: str(it.entityType),
		taxId: str(it.taxId),
		legalName: str(it.legalName),
		responsibleName: str(it.responsibleName),
		addressCity: str(it.addressCity),
		addressDistrict: str(it.addressDistrict),
		addressDetail: str(it.addressDetail),
		claimedStatus: str(it.claimedStatus),
	};
}

function normalizeTaxDocument(raw: Record<string, unknown>): SellerTaxDocument {
	return {
		documentId: str(raw.documentId),
		kind: str(raw.kind),
		fileName: str(raw.fileName),
		fileSize: str(raw.fileSize),
		fileKey: str(raw.fileKey),
		uploadedAt: str(raw.uploadedAt),
		fileAssetId: str(raw.fileAssetId),
	};
}

function normalizeReviewNote(
	raw: Record<string, unknown>,
): SellerTaxReviewNote {
	return {
		noteId: str(raw.noteId),
		content: str(raw.content),
		createdAt: str(raw.createdAt),
	};
}

function normalizeAttestation(raw: unknown): SelfIssueAttestation {
	const it = obj(raw);
	return {
		termsVersion: str(it.termsVersion),
		termsKey: str(it.termsKey),
		acceptedAt: str(it.acceptedAt),
		acceptedByAccountId: str(it.acceptedByAccountId),
		effective: bool(it.effective),
		currentTermsVersion: str(it.currentTermsVersion),
	};
}

/**
 * 端點 19：送出稅務資格審核（S02）。
 *
 * 🔴 **`acceptedTerms` 必須為 `true`。** 送審與「同意自行開票責任條款」是同一個
 * 動作（裁決 C，2026-08-31）——後端在同一句 UPDATE 裡寫下 `SUBMITTED` ＋ 60 天
 * 時鐘 ＋ 條款簽署記錄。傳 `false` ⇒ `9325`，且後端**一筆都不寫**。
 *
 * 之所以不做成「先打端點 22 再打 19」：端點 22（T04）的來源狀態是 `SUBMITTED`，
 * 而送審才會產生 `SUBMITTED` ⇒ 新賣家（`DRAFT`）走不進那個環。端點 22 保留給
 * 補簽（S04）。
 *
 * `expectedVersion` 只能來自端點 17 的 `version`；不符回 `9322`，此時**不要清空
 * 使用者輸入**，refetch 之後讓他再按一次。
 */
export async function submitSellerTaxProfile(args: {
	expectedVersion: string;
	acceptedTerms: boolean;
}): Promise<SellerTaxProfile> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/tax_profile/submissions`,
		{
			json: {
				expectedVersion: args.expectedVersion,
				acceptedTerms: args.acceptedTerms,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("taxsubmit") },
		},
	);
	return normalizeTaxProfile(env.data);
}

/**
 * `Idempotency-Key`（§6.1.0）：8–128 字、`^[A-Za-z0-9._:-]+$`。
 *
 * 58A §S02 指定送審用 `inv-taxsubmit-{uuid}`。連字號合法，但 UUID 的連字號一律
 * 去掉以免長度與字元集在不同 runtime 上飄。
 */
function newInvoiceIdempotencyKey(scope: string): string {
	const random =
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID().replace(/-/g, "")
			: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`;
	return `inv-${scope}-${random}`;
}

/**
 * 端點 17。尚未建立時後端回 404 ＋ `9320`——**那不是錯誤態**，呼叫端的 hook 把它
 * 折成 `NOT_STARTED`（見 `lib/hooks/useSellerInvoiceRemedy.ts`）。
 */
export async function fetchSellerTaxProfile(): Promise<SellerTaxProfile> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/tax_profile`,
	);
	return normalizeTaxProfile(env.data);
}

/**
 * 端點 17／19 共用的 `SellerTaxProfileData` 折解。
 *
 * 🔴 兩支端點回**同一個** DTO，所以只能有一份折解：分成兩份時，其中一份漏掉新欄位
 * 是完全靜默的（型別相同、值變成空字串），畫面只會顯示「沒有」。
 */
function normalizeTaxProfile(data: unknown): SellerTaxProfile {
	const raw = obj(data);
	return {
		storeId: str(raw.storeId),
		reviewStatus: str(raw.reviewStatus),
		version: str(raw.version),
		submittedAt: str(raw.submittedAt),
		deadlineAt: str(raw.deadlineAt),
		extensionCount: str(raw.extensionCount),
		canRequestExtension: bool(raw.canRequestExtension),
		reapplyCooldownUntil: str(raw.reapplyCooldownUntil),
		taxIdentity: normalizeTaxIdentity(raw.taxIdentity),
		documents: arr(raw.documents).map(normalizeTaxDocument),
		admNotes: arr(raw.admNotes).map(normalizeReviewNote),
		selfIssueAttestation: normalizeAttestation(raw.selfIssueAttestation),
		intakeLimit: normalizeIntakeLimit(raw.intakeLimit),
		pendingExtensionRequest: normalizeExtensionRequest(
			raw.pendingExtensionRequest,
		),
	};
}

/**
 * 待裁決的展延申請。**沒有申請時後端回 `null`，這裡就必須回 `null`**——
 * 折成一個每欄皆空的物件會讓 S02 對每個賣家都渲染「已申請、等待審核」。
 */
function normalizeExtensionRequest(
	data: unknown,
): SellerTaxExtensionRequest | null {
	if (!data || typeof data !== "object") return null;
	const raw = obj(data);
	const requestedAt = str(raw.requestedAt);
	// 時間是「這筆申請存在」的證據；沒有它就當作沒有申請，不要畫一塊沒有時間的提示。
	if (!requestedAt) return null;
	return { requestedAt, reason: str(raw.reason) };
}

/**
 * 稅籍證明文件的下載連結。
 *
 * ⚠️ 58A §S02 的欄位表寫的是 DTO 上的 `fileDownloadPath`，但 gateway 的
 * `TaxDocumentInfo` 給的是 `fileAssetId`——路徑由前端組，形狀與
 * `gateway/internal/handler/certification.go` 的 `secure_download` 完全一致
 * （帶既有 cookie 直開）。已回報衝突，未自行改規格。
 *
 * `fileAssetId` 為空（草稿剛上傳、後端尚未換發）時回空字串 ⇒ 那一列不渲染連結，
 * 點下去必定 404（S02 的「已上傳未存草稿」態）。
 */
export function sellerTaxDocumentHref(document: SellerTaxDocument): string {
	if (!document.fileAssetId) return "";
	return `/api/v1/app/media/secure_download/${encodeURIComponent(document.fileAssetId)}`;
}

/* ===== 端點 30 ===== */

/** 端點 30 的 query。全部走 URL query，值域見 §S06 的表。 */
export interface SellerInvoiceDocumentsQuery {
	dateFrom?: string;
	dateTo?: string;
	mode?: string;
	status?: string;
	abnormalOnly?: boolean;
	orderNo?: string;
	page?: number;
	pageSize?: number;
}

function normalizeDocumentItem(
	raw: Record<string, unknown>,
): SellerInvoiceDocumentItem {
	return {
		summary: normalizeDocumentSummary(obj(raw.summary)),
		orderNo: str(raw.orderNo),
		paidAt: str(raw.paidAt),
		buyerKind: str(raw.buyerKind),
		invoiceMode: str(raw.invoiceMode),
		shipmentStatus: str(raw.shipmentStatus),
		updatedAt: str(raw.updatedAt),
		latestJobId: str(raw.latestJobId),
	};
}

/**
 * 端點 30。`page_size` 上限 50，**逾越一律 400、後端不夾住** ⇒ 不讓使用者自選。
 */
export async function listSellerInvoiceDocuments(
	query: SellerInvoiceDocumentsQuery,
): Promise<SellerInvoiceDocumentsResult> {
	const params: Record<string, string> = {};
	if (query.dateFrom) params.date_from = query.dateFrom;
	if (query.dateTo) params.date_to = query.dateTo;
	if (query.mode) params.mode = query.mode;
	if (query.status) params.status = query.status;
	if (query.abnormalOnly) params.abnormal_only = "true";
	if (query.orderNo) params.order_no = query.orderNo;
	params.page = String(query.page ?? 1);
	params.page_size = String(query.pageSize ?? 20);

	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/documents`,
		{ query: params },
	);
	const data = obj(env.data);
	return {
		records: arr(data.records).map(normalizeDocumentItem),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
		// 🔴 `=== true` 而不是 `Boolean(...)`：舊版後端不回這個 key ⇒ undefined ⇒ false，
		// 也就是這一波之前的行為（空字串就是沒出貨）。若預設成 true，每一頁都會掛降級提示。
		shipmentStatusDegraded: data.shipmentStatusDegraded === true,
	};
}

/* ===== 端點 31 ===== */

/**
 * 端點 31。回應把明細包在 `detail` 之下、賣家專屬欄位平放在外層，這裡攤平成一層
 * ——`detail` 那一層在畫面上沒有任何語意，留著只會讓每個消費點多寫一次 `.detail`。
 */
export async function fetchSellerInvoiceDocument(
	documentId: string,
): Promise<SellerInvoiceDocumentDetail> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/documents/${encodeURIComponent(documentId)}`,
	);
	const raw = obj(env.data);
	const detail = obj(raw.detail);
	return {
		summary: normalizeDocumentSummary(obj(detail.summary)),
		issuerName: str(detail.issuerName),
		issuerTaxId: str(detail.issuerTaxId),
		recipientName: str(detail.recipientName),
		recipientTaxId: str(detail.recipientTaxId),
		salesAmount: str(detail.salesAmount),
		taxAmount: str(detail.taxAmount),
		totalAmount: str(detail.totalAmount),
		taxCategory: str(detail.taxCategory),
		lines: arr(detail.lines).map(normalizeLine),
		timeline: arr(detail.timeline).map(normalizeTimelineItem),
		createdBy: str(raw.createdBy),
		fundedBy: str(raw.fundedBy),
		cutoverVersion: str(raw.cutoverVersion),
		providerRequestId: str(raw.providerRequestId),
		manualReasonCode: str(raw.manualReasonCode),
		nextResendAvailableAt: str(raw.nextResendAvailableAt),
		latestJobId: str(raw.latestJobId),
	};
}

/* ===== 端點 62R ===== */

function normalizeRetryPolicy(raw: unknown): InvoiceJobInfo["retryPolicy"] {
	const it = obj(raw);
	return {
		initialIntervalSeconds: str(it.initialIntervalSeconds),
		backoffMultiplier: str(it.backoffMultiplier),
		maxIntervalSeconds: str(it.maxIntervalSeconds),
		maxAttempts: str(it.maxAttempts),
	};
}

function normalizeJobInfo(raw: Record<string, unknown>): InvoiceJobInfo {
	return {
		jobId: str(raw.jobId),
		orderNo: str(raw.orderNo),
		storeId: str(raw.storeId),
		operationType: str(raw.operationType),
		status: str(raw.status),
		summary: str(raw.summary),
		attemptCount: str(raw.attemptCount),
		maxAttempts: str(raw.maxAttempts),
		lastAttemptAt: str(raw.lastAttemptAt),
		nextRetryAt: str(raw.nextRetryAt),
		retryPolicy: normalizeRetryPolicy(raw.retryPolicy),
		shipmentUnaffectedNotice: str(raw.shipmentUnaffectedNotice),
		providerRequestId: str(raw.providerRequestId),
		version: str(raw.version),
		manualReasonCode: str(raw.manualReasonCode),
		externalInvoiceNo: str(raw.externalInvoiceNo),
		externalInvoiceDate: str(raw.externalInvoiceDate),
		reportedAt: str(raw.reportedAt),
		canReportExternal: bool(raw.canReportExternal),
		documentKind: str(raw.documentKind),
	};
}

/**
 * 端點 62R。`CapInvoiceTaxRemedy`，**停權賣家放行**——它是停權期間唯一還回得來的
 * 發票查詢，S07 ⑦ 的存在理由（§B-2.0 T05）。
 */
export async function listOrderExternalIssuances(
	orderNo: string,
): Promise<SellerPagedResult<InvoiceJobInfo>> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/orders/${encodeURIComponent(orderNo)}/external_issuances`,
		{ query: { page: "1", page_size: "20" } },
	);
	const data = obj(env.data);
	return {
		records: arr(data.records).map(normalizeJobInfo),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
	};
}

/**
 * 端點 62｜賣家回報自己開立的發票號碼。
 *
 * ── 輸入的邊界全部由 invoice-service 定死，前端照抄不自創 ──────────────────
 * （`internal/usecase/seller_job.go` 的 `ReportExternalIssuance` ＋
 * `validateEvidenceAssetKeys`，`internal/usecase/admin_job.go` 的 pattern）：
 *
 *   `invoiceNumber`  `^[A-Z]{2}[0-9]{8}$`（兩碼英文字軌 ＋ 八碼數字）
 *   `invoiceDate`    `YYYY-MM-DD`，且**不得早於付款日**（後端用 UTC+8 曆日比較）
 *   `note`           ≤ 500 字（rune 數）
 *   `evidenceAssetKeys` ≤ 5 筆，且只收 media **objectKey**、不收 URL
 *
 * 🔴 `expectedVersion` 取自**當前畫面讀到的那筆作業**的 `version`
 * （S08 走端點 33、S07 走端點 62R），不得寫死、不得傳 `0`、不得自己編一個。
 * 對不上時後端回 `9343`，而那正是「有人在你填表時動過這筆」的訊號。
 *
 * ⚠️ **request 沒有金額欄**（proto 逐字）：金額取該 document 的既有總額，不是賣家
 * 打進來的數字。端點 56（ADM 代錄）才收金額。
 *
 * ⚠️ 佐證截圖本波不接：secure 兩段式上傳（`lib/media/secure-upload.ts`）產出的是
 * staged token 而不是 objectKey，而本端點只收 objectKey ⇒ 送 token 進去會被
 * `ErrExternalEvidenceInvalid` 擋成 400。空陣列是合法的，所以恆送空陣列，
 * 不做一個傳完才發現送不出去的上傳元件。
 */
export async function reportExternalIssuance(args: {
	jobId: string;
	invoiceNumber: string;
	invoiceDate: string;
	note: string;
	expectedVersion: string;
}): Promise<InvoiceJobInfo> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/jobs/${encodeURIComponent(args.jobId)}/external_issuances`,
		{
			json: {
				invoiceNumber: args.invoiceNumber,
				invoiceDate: args.invoiceDate,
				evidenceAssetKeys: [],
				note: args.note,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("extissue") },
		},
	);
	return normalizeJobInfo(obj(env.data));
}

/**
 * 端點 62 的發票號碼格式（後端 `externalInvoiceNoPattern` 逐字）。
 *
 * 🔴 前端先驗一次不是為了取代後端，而是因為送錯只會拿到一句 400 的英文
 * （`invoice: invoice_no must match ...`）——那句話會被直接顯示給賣家看。
 */
export const EXTERNAL_INVOICE_NO_PATTERN = /^[A-Z]{2}[0-9]{8}$/;

/** 端點 62 的 `note` 長度上限（後端 `adminNoteMaxRunes`）。 */
export const EXTERNAL_ISSUANCE_NOTE_MAX = 500;

/* ===== 端點 33 ===== */

/**
 * 端點 33（S08 唯一資料源）。
 *
 * 越權與不存在都回 404 ＋ `9340`（Tier A）⇒ 呼叫端的文案必須完全相同，不得寫
 * 「你沒有權限查看」——那等於把這一頁變成可枚舉的 oracle。
 */
export async function fetchSellerInvoiceJob(
	jobId: string,
): Promise<InvoiceJobInfo> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/jobs/${encodeURIComponent(jobId)}`,
	);
	return normalizeJobInfo(obj(env.data));
}

/* ===== 端點 34／35 ===== */

/**
 * 端點 34 的 query。
 *
 * 🔴 `status` 是**視圖名不是 DB 狀態值**（`AWAITING_SELLER`／`SELLER_RESPONDED`
 * ／`CLOSED`），兩套字串不可互相代入：後端 `sellerComplaintViews` 把 7 個 DB 值
 * 展開成三個互斥且窮盡的視圖。送 DB 值進去會拿到 400。
 *
 * 🔴 `sort` 固定 `due_at_asc`（最接近逾期者在最上面），**畫面不提供排序 UI**：
 * 這一頁的唯一目的就是「先處理快逾期的」，給一個能關掉它的開關等於讓賣家自己
 * 製造違規。後端的實作是「只有明確要求 `created_at_desc` 才切換」，所以這一欄
 * 送什麼都不會壞，但送出去才有機械證據說明前端要的是哪一種。
 */
export interface SellerInvoiceComplaintsQuery {
	view: string;
	page?: number;
	pageSize?: number;
}

/** 端點 34 的 `sort`。畫面上沒有開關，值寫死在這裡。 */
export const SELLER_COMPLAINT_SORT_DUE_ASC = "due_at_asc";

function normalizeComplaintResponse(
	raw: Record<string, unknown>,
): SellerInvoiceComplaintResponse {
	return {
		responseId: str(raw.responseId),
		actor: str(raw.actor),
		action: str(raw.action),
		note: str(raw.note),
		occurredAt: str(raw.occurredAt),
	};
}

function normalizeComplaint(
	raw: Record<string, unknown>,
): SellerInvoiceComplaint {
	return {
		complaintId: str(raw.complaintId),
		caseNo: str(raw.caseNo),
		orderNo: str(raw.orderNo),
		storeId: str(raw.storeId),
		status: str(raw.status),
		expectedDocKind: str(raw.expectedDocKind),
		dueAt: str(raw.dueAt),
		remainingBusinessDays: str(raw.remainingBusinessDays),
		overdueBusinessDays: str(raw.overdueBusinessDays),
		buyerMaskedName: str(raw.buyerMaskedName),
		description: str(raw.description),
		responses: arr(raw.responses).map(normalizeComplaintResponse),
		version: str(raw.version),
		createdAt: str(raw.createdAt),
		reopenCount: str(raw.reopenCount),
		buyerReviewDueAt: str(raw.buyerReviewDueAt),
		reopenAllowedAfter: str(raw.reopenAllowedAfter),
	};
}

/** 端點 34（S09 唯一資料源）。`CapOrderFulfill`／**停權放行**。 */
export async function listSellerInvoiceComplaints(
	query: SellerInvoiceComplaintsQuery,
): Promise<SellerPagedResult<SellerInvoiceComplaint>> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/complaints`,
		{
			query: {
				status: query.view,
				sort: SELLER_COMPLAINT_SORT_DUE_ASC,
				page: String(query.page ?? 1),
				page_size: String(query.pageSize ?? 20),
			},
		},
	);
	const data = obj(env.data);
	return {
		records: arr(data.records).map(normalizeComplaint),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
	};
}

/**
 * 端點 35（S10 唯一資料源）。同樣 Tier A：越權與不存在都是 404 ＋ `9309`。
 *
 * ⚠️ 回應**沒有內嵌任何發票文件摘要**（實查 gateway `InvoiceComplaintInfo` 的 17 欄
 * 與 invoice-service 的 `toComplaintInfo`）⇒ 58A §S10 ④「目前發票狀態卡」在本波
 * 做不出來。這是 §B-2.4 A4 明列「開工時實打確認、形狀不同時改的是 S10 第 5 項」
 * 的那一項，已回報、未自行改規格。
 */
export async function fetchSellerInvoiceComplaint(
	complaintId: string,
): Promise<SellerInvoiceComplaint> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/complaints/${encodeURIComponent(complaintId)}`,
	);
	return normalizeComplaint(obj(env.data));
}

/* ===== 端點 72 ===== */

/** 端點 72 的 query。與端點 30 不同：**沒有** `mode`／`abnormal_only`／`order_no`。 */
export interface SellerReceivedInvoicesQuery {
	dateFrom?: string;
	dateTo?: string;
	status?: string;
	page?: number;
	pageSize?: number;
}

function normalizeReceivedItem(
	raw: Record<string, unknown>,
): SellerReceivedInvoiceItem {
	return {
		summary: normalizeDocumentSummary(obj(raw.summary)),
		issuerName: str(raw.issuerName),
		recipientNote: str(raw.recipientNote),
	};
}

/**
 * 端點 72（S12 進項分頁唯一資料源）。recipient-scoped：回的是**平台開給這個賣家**
 * 的發票，與端點 30（issuer-scoped，我開給買家的）是不同的資料集與不同的法律角色。
 */
export async function listSellerReceivedInvoiceDocuments(
	query: SellerReceivedInvoicesQuery,
): Promise<SellerPagedResult<SellerReceivedInvoiceItem>> {
	const params: Record<string, string> = {};
	if (query.dateFrom) params.date_from = query.dateFrom;
	if (query.dateTo) params.date_to = query.dateTo;
	if (query.status) params.status = query.status;
	params.page = String(query.page ?? 1);
	params.page_size = String(query.pageSize ?? 20);

	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/received_documents`,
		{ query: params },
	);
	const data = obj(env.data);
	return {
		records: arr(data.records).map(normalizeReceivedItem),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
	};
}

/* ===== PDF 子路由 ===== */

/**
 * 賣家面 PDF 的絕對網址（`GET /documents/{document_id}/file/content`，
 * `CapInvoiceManage` 與端點 31 同 cap）。
 *
 * 回的是二進位不是 JSON ⇒ 不經 `apiClient.authed.get`，用 `buildUrl` 組出網址交給
 * `window.open`（形狀照買家面的 `invoiceFileUrl`）。是否渲染這條連結一律看
 * `summary.fileAvailable`——它與 PDF 端點會回的 `9312`／`9313` 是同一個判定，
 * 先打一次探測只是多燒一次限流額度。
 */
export function sellerInvoiceFileUrl(documentId: string): string {
	return apiClient.buildUrl(
		"authed",
		`${BASE}/documents/${encodeURIComponent(documentId)}/file/content`,
	);
}

/* ===== AMEGO 憑證帳號（端點 23–27，S03）===== */

/**
 * `PUT /credentials` 的 `consentVersion`（後端 `requireConsentVersion` 只驗非空 ＋ 長度）。
 *
 * 🔴 **這個值由前端擁有，因為後端沒有「當期版本」可讀**：`SellerProviderAccountData`
 * 回的 `consentVersion` 是**上次同意的版本**，首次串接時必然是空的；`tax_profile` 的
 * `currentTermsVersion` 是自行開票責任條款（S04）的版本，語意不同不可挪用。
 * ⇒ 改版時改這裡一個地方，並在 S03 畫面上同步顯示。
 */
export const AMEGO_CONSENT_VERSION = "amego-connect-v1";

/** AMEGO 是本波唯一的服務商；端點 24 的 `provider` 由後端 `requireProviderAMEGO` 正面列舉。 */
export const AMEGO_PROVIDER = "AMEGO";

/**
 * 端點 23。**404 ＋ `9330` 不是錯誤態**：代表「還沒開始串接」，
 * 呼叫端把它折成 `NOT_STARTED` 的空白精靈（58A §S03 empty 態）。
 *
 * 🔴 回應裡沒有 AppKey，也沒有憑證指紋——見 `SellerProviderAccount` 的型別註解。
 */
export async function fetchSellerProviderAccount(): Promise<SellerProviderAccount> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/provider_account`,
	);
	return normalizeProviderAccount(env.data);
}

/**
 * 端點 24。寫入（或覆寫）AppKey。落地即 `CREDENTIAL_VERIFYING`——
 * 後端刻意不對外呈現 `CREDENTIAL_SUBMITTED` 這個中間態。
 *
 * ⚠️ `sellerTaxId` 必須等於平台核定的統編，否則 `9334`。畫面上那格是唯讀的，
 * 值從端點 17 的 `taxIdentity.taxId` 來——那是核定值的唯一來源。
 *
 * ⚠️ 2026-09-02 更正（#3017）：這裡原本寫「不要拿端點 23 的 `sellerTaxIdMasked`，
 * 它是遮罩過的顯示值，送出去必定不符」——**那個理由是錯的**。端點 23 的那一欄
 * 從來沒有被遮罩過（58 號 §8.1.3(b) 把賣家統編判為 O 類、三端皆完整），
 * 說謊的是舊欄名。欄位已改名為 `sellerTaxId`，值與端點 17 相同。
 * 仍然建議取端點 17：核定值只該有一個來源，這是 SSOT 的理由，不是遮罩的理由。
 */
export async function putSellerProviderCredentials(args: {
	sellerTaxId: string;
	appKey: string;
	expectedVersion: string;
}): Promise<SellerProviderAccount> {
	const env = await apiClient.authed.put<Record<string, unknown>>(
		`${BASE}/provider_account/credentials`,
		{
			json: {
				provider: AMEGO_PROVIDER,
				sellerTaxId: args.sellerTaxId,
				appKey: args.appKey,
				consentVersion: AMEGO_CONSENT_VERSION,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("cred") },
		},
	);
	return normalizeProviderAccount(env.data);
}

/**
 * 端點 25。連線測試。
 *
 * 🔴 驗證失敗是 **HTTP 200 ＋ `code 9331`**（`preserveData`）：`apiClient` 仍會丟成
 * `ApiClientError`，所以呼叫端一律讀 `code` 不讀 status，並從 `error.data` 取
 * `lastRequestId` 給客服。只看 HTTP status 會把失敗當成功。
 *
 * ⚠️ 重試要沿用同一把 idempotency key，所以由呼叫端傳入而非在此生成。
 */
export async function verifySellerProviderAccount(args: {
	expectedVersion: string;
	idempotencyKey: string;
}): Promise<SellerProviderAccount> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/provider_account/verifications`,
		{
			json: { expectedVersion: args.expectedVersion },
			headers: { "Idempotency-Key": args.idempotencyKey },
		},
	);
	return normalizeProviderAccount(env.data);
}

/**
 * 端點 26。設定自動開票切點（`cutoverAt`，ISO 8601 UTC）。
 *
 * 🔴 切點只能是未來：**較早付款的訂單不會回頭改用新設定**，所以送出前要再驗一次
 * （前端擋 400，不是為了省一次往返，是為了讓錯誤訊息落在欄位上而不是紅色橫幅）。
 */
export async function activateSellerProviderAccount(args: {
	cutoverAt: string;
	expectedVersion: string;
}): Promise<SellerProviderAccount> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/provider_account/activations`,
		{
			json: {
				cutoverAt: args.cutoverAt,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("activate") },
		},
	);
	return normalizeProviderAccount(env.data);
}

/**
 * 端點 27。已 `ACTIVE`／`DEGRADED` 時換 AppKey 走這支（端點 24 的 allowed-from
 * 不含那兩個狀態）。
 *
 * ⚠️ 失敗時**原有憑證仍然有效**，開票不受影響——文案必須講這件事，否則賣家會以為
 * 自己把線上的開票弄壞了。
 */
export async function rotateSellerProviderCredentials(args: {
	appKey: string;
	expectedVersion: string;
}): Promise<SellerProviderAccount> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/provider_account/rotations`,
		{
			json: { appKey: args.appKey, expectedVersion: args.expectedVersion },
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("rotate") },
		},
	);
	return normalizeProviderAccount(env.data);
}

function normalizeProviderAccount(data: unknown): SellerProviderAccount {
	const raw = obj(data);
	return {
		provider: str(raw.provider),
		status: str(raw.status),
		statusNotice: str(raw.statusNotice),
		version: str(raw.version),
		sellerTaxId: str(raw.sellerTaxId),
		credentialLast4: str(raw.credentialLast4),
		credentialUpdatedAt: str(raw.credentialUpdatedAt),
		lastVerifiedAt: str(raw.lastVerifiedAt),
		automationCutoverAt: str(raw.automationCutoverAt),
		consentVersion: str(raw.consentVersion),
		lastRequestId: str(raw.lastRequestId),
		pendingOrderCount: str(raw.pendingOrderCount),
	};
}

/* ===== 自行開票責任條款（端點 21／22，S04）===== */

/**
 * 端點 21。**恆有回應**（未簽過的賣家拿到 `termsVersion: ""` ＋ 非空的
 * `currentTermsVersion`），所以沒有 empty 態。
 *
 * ⚠️ `termsKey` 與 `acceptedByAccountId` **恆為空字串**：`seller_tax_profiles` 只有
 * `attestation_version` ＋ `attested_at` 兩欄，沒有存條款檔 key 與接受人
 * （`grpc/query.go` 的 `toSelfIssueAttestation` 已註明）。
 * ⇒ 畫面不得渲染「接受帳號」，也不得拿 `termsKey` 組下載連結——一個永遠空白的欄位
 * 比不顯示更糟。要查是誰簽的走 ADM 稽核主控台的狀態歷程。
 */
export async function fetchSelfIssueAttestation(): Promise<SelfIssueAttestation> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/self_issue_attestations`,
	);
	return normalizeAttestation(env.data);
}

/**
 * 端點 22。補簽條款並把賣家推進 `SELF_ISSUE_PENDING`。
 *
 * 🔴 `termsVersion` 送的是 **`currentTermsVersion`（當期版本）**，不是
 * `termsVersion`（賣家已簽的版本）。後者未簽時是空字串，拿它回送必被 400 拒 ⇒
 * 新賣家永遠簽不成功。呼叫端已把參數命名成 `currentTermsVersion` 讓這件事難以寫錯。
 */
export async function acceptSelfIssueAttestation(args: {
	currentTermsVersion: string;
}): Promise<SelfIssueAttestation> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/self_issue_attestations`,
		{
			json: { termsVersion: args.currentTermsVersion, accepted: true },
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("attest") },
		},
	);
	return normalizeAttestation(env.data);
}

/* ===== 收據代製授權（端點 28／29，S05）===== */

/**
 * 端點 29 的 `consentVersion`。與 S03 的 `AMEGO_CONSENT_VERSION` 同理：後端只驗
 * 非空，沒有「當期版本」可讀（端點 28 回的是**上次授權**的版本，首次授權時為空）。
 */
export const RECEIPT_AUTH_CONSENT_VERSION = "receipt-auth-v1";

/** 端點 28。恆有回應；`eligible === false` 是正常狀態不是錯誤。 */
export async function fetchSellerReceiptAuthorization(): Promise<SellerReceiptAuthorization> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${BASE}/receipt_authorization`,
	);
	return normalizeReceiptAuthorization(env.data);
}

/** 端點 29。授權平台依賣家名義製作交易收據。 */
export async function confirmSellerReceiptAuthorization(args: {
	expectedVersion: string;
}): Promise<SellerReceiptAuthorization> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/receipt_authorization/confirmations`,
		{
			json: {
				consentVersion: RECEIPT_AUTH_CONSENT_VERSION,
				authorized: true,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("receiptauth") },
		},
	);
	return normalizeReceiptAuthorization(env.data);
}

/**
 * 核定依據文件的下載路徑（我方**授權下載通道**，瀏覽器帶既有 cookie）。
 *
 * 🔴 **不是公開 CDN URL**：核定文件是該賣家的國稅局文件，屬機敏檔。
 * 形狀與 `sellerTaxDocumentHref` 一致，兩者是同一條通道。
 *
 * ⚠️ 呼叫端必須先看 `approvalBasisAvailable`：這支函式只負責組字串，它組得出
 * 一個賣家其實無權下載的路徑。
 */
export function receiptApprovalBasisHref(assetId: string): string {
	if (!assetId) return "";
	return `/api/v1/app/media/secure_download/${encodeURIComponent(assetId)}`;
}

function normalizeReceiptAuthorization(
	data: unknown,
): SellerReceiptAuthorization {
	const raw = obj(data);
	return {
		eligible: bool(raw.eligible),
		ineligibleReason: str(raw.ineligibleReason),
		approvalBasisKey: str(raw.approvalBasisKey),
		approvedAt: str(raw.approvedAt),
		approvalValidUntil: str(raw.approvalValidUntil),
		authorizedAt: str(raw.authorizedAt),
		consentVersion: str(raw.consentVersion),
		receiptTemplateKey: str(raw.receiptTemplateKey),
		version: str(raw.version),
		approvalBasisAssetId: str(raw.approvalBasisAssetId),
		approvalBasisAvailable: bool(raw.approvalBasisAvailable),
		approvalBasisUnavailableReason: str(raw.approvalBasisUnavailableReason),
	};
}

/* ===== 賣家回覆申訴（端點 36，S10 ⑥）===== */

/**
 * 賣家可選的三個 action。
 *
 * 🔴 **正面列舉三值**，不是「`InvoiceComplaintAction` 減掉 ADM 的三個」：那份碼表是
 * 6 值共用表，日後長胖時減法寫法會把新的 ADM 動作靜默收進賣家的選單。
 * 後端 `normalizeSellerComplaintAction` 用的是同一種寫法，兩邊對稱。
 */
export const SELLER_COMPLAINT_ACTIONS = [
	"RESENT",
	"REISSUED",
	"NEED_PLATFORM_HELP",
] as const;

export type SellerComplaintAction = (typeof SELLER_COMPLAINT_ACTIONS)[number];

/**
 * 端點 36。賣家回覆申訴（追加語意，同一件案可回覆多次）。
 *
 * 🔴 `expectedVersion` 取自 `complaint.version`，**不得用 `responses.length` 頂替**
 * ——那兩個數字在平台側也追加事件時就會分岔。
 *
 * ⚠️ 端點 36 的 request **沒有佐證檔欄位**，那是刻意的不是遺漏（58 §6.1 端點 36 定版③）。
 */
export async function respondSellerInvoiceComplaint(args: {
	complaintId: string;
	action: SellerComplaintAction;
	note: string;
	expectedVersion: string;
}): Promise<SellerInvoiceComplaint> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/complaints/${encodeURIComponent(args.complaintId)}/responses`,
		{
			json: {
				action: args.action,
				note: args.note,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("cmplresp") },
		},
	);
	return normalizeComplaint(obj(env.data));
}

/* ===== 稅務身分草稿（端點 18，S02 表單）===== */

/**
 * 端點 18。存草稿。**這是整條發票鏈的第一步**：只有這支會寫
 * `evidence_object_keys`，而端點 19（送審）的守衛讀的正是那一欄
 * ⇒ 沒有它，賣家從零走不到可送審的 `DRAFT`。
 *
 * 🔴 佐證檔走 **secure 兩段式上傳**（`lib/media/secure-upload.ts` 的
 * `uploadSecureDocument`），送進來的是 `evidenceStagedTokens`——**不是** objectKey。
 * 用公開通道的 `uploadImageFile` 會讓稅籍證明變成任何人拿到 key 就下得到的檔案。
 *
 * ⚠️ `evidenceObjectKeys` 是**已落庫**的既有佐證（回存時原樣帶回，避免被清空）；
 * `evidenceStagedTokens` 是**這次新上傳**的。兩者不是同一件事，也不可互相頂替：
 * staged token 是短命的，落庫後才變成 objectKey。
 */
export async function saveSellerTaxProfileDraft(args: {
	taxIdentity: SellerTaxIdentityInput;
	evidenceObjectKeys: string[];
	evidenceStagedTokens: string[];
	expectedVersion: string;
}): Promise<SellerTaxProfile> {
	// 端點 18 是 PUT 不是 POST（58 號設計書 :4780 與 :8737 兩處一致，gateway 也註冊 PUT）。
	// 這裡曾經是 post，實測回 404 page not found ⇒「儲存草稿」按鈕 100% 失敗、
	// 稅籍生命週期第一步就斷，連帶讓已上傳的稅籍證明停在 upload_session 成為孤兒檔案。
	const env = await apiClient.authed.put<Record<string, unknown>>(
		`${BASE}/tax_profile/draft`,
		{
			json: {
				taxIdentity: args.taxIdentity,
				evidenceObjectKeys: args.evidenceObjectKeys,
				evidenceStagedTokens: args.evidenceStagedTokens,
				expectedVersion: args.expectedVersion,
			},
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("taxdraft") },
		},
	);
	return normalizeTaxProfile(env.data);
}

/**
 * 端點 20。申請延長審核期限（60 天期限，符合條件可延長一次）。
 *
 * ⚠️ 能不能申請看的是端點 17 的 `canRequestExtension`，不是自己算 `extensionCount`
 * ——後端的條件不只次數（還有期限剩餘天數）。
 */
export async function requestSellerTaxExtension(args: {
	reason: string;
	expectedVersion: string;
}): Promise<SellerTaxProfile> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/tax_profile/extension_requests`,
		{
			json: { reason: args.reason, expectedVersion: args.expectedVersion },
			headers: { "Idempotency-Key": newInvoiceIdempotencyKey("taxext") },
		},
	);
	return normalizeTaxProfile(env.data);
}

/* ===== 重寄發票通知（端點 32，S06／S11）===== */

/**
 * 端點 32。重寄發票通知信給買家。
 *
 * ⚠️ **`resent: false` 不是錯誤**：後端有節流（同一份文件短時間內只寄一次），
 * 此時 HTTP 是 200、`resent` 為 `false`。呼叫端要看這個布林而不是只看有沒有丟例外
 * ——只判例外會讓賣家以為寄出去了，然後買家還是沒收到。
 *
 * `toMasked` 是遮罩過的收件信箱，用來讓賣家確認寄到哪裡；不是完整地址。
 */
export async function resendSellerInvoiceNotification(
	documentId: string,
): Promise<{ resent: boolean; sentAt: string; toMasked: string }> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/documents/${encodeURIComponent(documentId)}/notification_resends`,
		{ headers: { "Idempotency-Key": newInvoiceIdempotencyKey("resend") } },
	);
	const raw = obj(env.data);
	return {
		resent: bool(raw.resent),
		sentAt: str(raw.sentAt),
		toMasked: str(raw.toMasked),
	};
}
