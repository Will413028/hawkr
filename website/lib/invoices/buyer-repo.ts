/**
 * Buyer-side invoice API (58 號《電子發票服務》§6.1.1).
 *
 *   GET    /app/buyers/me/invoice_profiles                       — 端點 1 列表
 *   POST   /app/buyers/me/invoice_profiles                       — 端點 2 新增
 *   PUT    /app/buyers/me/invoice_profiles/{profileId}           — 端點 3 更新
 *   DELETE /app/buyers/me/invoice_profiles/{profileId}           — 端點 4 刪除
 *   POST   /app/buyers/me/invoice_profiles/{profileId}/default   — 端點 5 設為預設
 *
 *   GET    /app/orders/{orderNo}/invoice                          — 端點 8  訂單發票總覽（B04）
 *   GET    /app/invoices/documents                                — 端點 68 發票中心列表（B09）
 *   GET    /app/invoices/documents/{documentId}                   — 端點 69 文件詳情（B05）
 *   POST   /app/invoices/documents/{documentId}/notification_resends — 端點 71 重寄通知（B05）
 *
 *   POST   /app/orders/{orderNo}/invoice_complaints                            — 端點 13 提出申訴（B07）
 *   GET    /app/orders/{orderNo}/invoice_complaints                            — 端點 14 案件列表（B07，只在 9310 補救路徑用）
 *   GET    /app/orders/{orderNo}/invoice_complaints/{complaintId}              — 端點 15 單筆案件（B07 的唯一資料源）
 *   POST   /app/orders/{orderNo}/invoice_complaints/{id}/rejections            — 端點 63 不接受賣家回覆
 *   POST   /app/orders/{orderNo}/invoice_complaints/{id}/withdrawals           — 端點 64 撤回申訴
 *   POST   /app/orders/{orderNo}/invoice_complaints/{id}/reopenings            — 端點 65 重新提出
 *
 * ⚠️ B05 走的是 **document-scoped 的端點 69**，不是 order-scoped 的端點 9。理由是
 * 路由：`/account/invoices/[documentId]` 沒有 `orderNo`（發票通知的 `schema_url`
 * 也是 `/account/invoices/#{documentId}`），而端點 9 的路徑上必須填 `order_no`。
 *
 * ⚠️ **端點 10（PDF 下載資訊）沒有接**：它回的是 `{documentId, invoiceFileKey,
 * fileName, style}`，一個可以放進 `<a href>` 的網址都沒有 —— 真正的二進位在
 * `/file/content` 子路由（`invoiceFileUrl()`）。而它會 409 的兩個情況（9312 載具
 * 未中獎／9313 逾查詢期限）與 `summary.fileAvailable === false` 是同一個判定
 * （invoice-service `FileAvailability`），所以先打一次只是多燒一次限流額度。
 */

import { ApiClientError, apiClient } from "../api/client";
import type {
	BuyerInvoiceDocumentItem,
	BuyerInvoiceDocumentListResult,
	CreateInvoiceComplaintInput,
	CreateInvoiceProfileInput,
	InvoiceComplaint,
	InvoiceComplaintResponseItem,
	InvoiceCorrection,
	InvoiceCorrectionInput,
	InvoiceDocumentDetail,
	InvoiceDocumentSummary,
	InvoiceLine,
	InvoiceProfile,
	InvoiceProfileListResult,
	InvoiceTimelineItem,
	OrderInvoice,
	UpdateInvoiceProfileInput,
} from "./types";

const BASE = "/app/buyers/me/invoice_profiles";

/* ===== 業務碼 ===== */

/**
 * 58 §6.1.0 (13) 的碼表中，買家常用發票資料會遇到的五個。
 *
 * 🔴 一律讀業務碼，不讀 HTTP status（§0.4 ①）。發票的碼表有 7 條標
 * `preserveData` 的 HTTP 恆為 200，照 status 分流會永遠漏接。本檔目前用到的
 * 五個雖然都是 4xx，但分流的依據仍是碼——B07 的 9310 就是 200。
 */
export const INVOICE_BIZ_CODE = {
	/** Tier A：越權與不存在回同一個碼 ⇒ 前端文案也必須一致，不得寫「沒有權限」。 */
	DOCUMENT_NOT_FOUND: 9301,
	PROFILE_NOT_FOUND: 9302,
	PROFILE_LIMIT_EXCEEDED: 9303,
	PROFILE_DEFAULT_REQUIRED: 9304,
	CARRIER_INVALID: 9305,
	TARGET_CONFLICT: 9306,
	/** Tier A：越權與不存在回同一個碼 ⇒ 不得翻譯成「無權操作」（§B07 權限與邊界）。 */
	COMPLAINT_NOT_FOUND: 9309,
	/**
	 * 🔴 HTTP 是 **200**（`preserveData`）而不是 4xx，client 仍會把它 throw 成
	 * `ApiClientError`。**不是錯誤**：`payload.data.complaintId` 帶既有案件，
	 * 呼叫端要導向它而不是顯示紅字（§B-1.0.9 B05t）。
	 */
	COMPLAINT_DUPLICATE: 9310,
	COMPLAINT_NOT_OPEN_YET: 9311,
	/**
	 * 樂觀鎖衝突。碼名是稅務側的（`SELLER_TAX_PROFILE_VERSION_CONFLICT`，後端
	 * message 講的是「稅務資格資料已被更新」），63／64／65 借用它 ⇒ **不得直接
	 * 顯示後端 message**，要用申訴語境的自有文案（§B-1.0.6）。
	 */
	VERSION_CONFLICT: 9322,
	/**
	 * 案件當前狀態不允許此動作／重開次數用盡／冷卻未到。後端對三種守衛給三句
	 * 不同的話 ⇒ **直接顯示後端 message**，前端不自行推導是哪一種。
	 */
	COMPLAINT_ACTION_NOT_ALLOWED: 9336,
	IDEMPOTENCY_PAYLOAD_MISMATCH: 9350,
	/**
	 * 端點 71：重寄通知過於頻繁（同一份文件 10 分鐘一次的窗口）。
	 *
	 * 🔴 **它是 error 不是 `resent: false`**：後端在 claim 不到窗口時直接丟
	 * `9352`／HTTP 429，`data` 一定是空的 ⇒ 拿 `resent` 布林當節流分支寫出來的
	 * 是死碼，買家看到的會是通用的「寄送失敗」，而真正的原因是「剛剛才寄過」。
	 * （賣家端點 32 已為同一件事踩過一次，見 `seller-repo.ts` 的同名碼。）
	 */
	NOTIFICATION_RESEND_THROTTLED: 9352,
} as const;

/** Phase 1 唯一的申訴原因（端點 13 的 `reasonCode`）。 */
export const INVOICE_COMPLAINT_REASON_NOT_RECEIVED = "NOT_RECEIVED";

const BIZ_CODE_PREFIX = /^\[(\d+)\]/;

/**
 * 取出 Gateway 塞在 `message` 前綴的 `[NNNN] ` 業務碼；沒有前綴時退回 envelope
 * 的 `code`。形狀照 `lib/seller/shipment.ts` 的 `shipmentBusinessCode`，不做
 * 全站對照表。
 */
export function invoiceBizCode(err: unknown): number {
	if (!(err instanceof Error)) return 0;
	const matched = err.message.match(BIZ_CODE_PREFIX);
	if (matched) {
		const parsed = Number.parseInt(matched[1] ?? "", 10);
		if (Number.isFinite(parsed)) return parsed;
	}
	if (err instanceof ApiClientError && typeof err.code === "number") {
		return err.code;
	}
	return 0;
}

/** 去掉 `[NNNN] ` 前綴，讓使用者看到後端真正想說的那句話。 */
export function invoiceErrorMessage(err: unknown): string {
	if (!(err instanceof Error)) return "";
	return err.message.replace(BIZ_CODE_PREFIX, "").trim();
}

/* ===== 冪等鍵 ===== */

/**
 * 一次「使用者意圖」一把（§0.4 ②）。
 *
 * 🔴 呼叫端必須把回傳值存在該次操作的 state／closure 裡，重試沿用同一把；
 * **只有拿到 9350（payload mismatch）才換新的**。每次重試重生一把，等於告訴
 * 後端「這是第二次操作」，而發票的重複開立不可逆。
 */
export function newIdempotencyKey(): string {
	return crypto.randomUUID().replace(/-/g, "");
}

function idempotent(key: string): { headers: Record<string, string> } {
	return { headers: { "Idempotency-Key": key } };
}

/* ===== normalize ===== */

function str(value: unknown): string {
	return value == null ? "" : String(value);
}

function num(value: unknown): number {
	const parsed = Number(value);
	return Number.isFinite(parsed) ? parsed : 0;
}

/**
 * wire → 前端型別。`profileId` 是 64-bit TSID，一律保字串；`carrierType` 與
 * `status` 保 `string` 而不是收窄成 CodeMeta union，理由見 `types.ts` 檔頭。
 */
function normalizeProfile(raw: Record<string, unknown>): InvoiceProfile {
	return {
		profileId: str(raw.profileId),
		carrierType: str(raw.carrierType),
		displayValue: str(raw.displayValue),
		buyerTaxId: str(raw.buyerTaxId),
		buyerName: str(raw.buyerName),
		isDefault: raw.isDefault === true,
		label: str(raw.label),
		createdAt: str(raw.createdAt),
		updatedAt: str(raw.updatedAt),
	};
}

/* ===== 端點 ===== */

/**
 * 每個買家最多 20 筆（invoice-service `buyerProfileCap`），而端點 1 的預設
 * `page_size` 是 10 ⇒ **不指定就會靜默只拿到前 10 筆**：畫面照常渲染、沒有錯誤、
 * 沒有「下一頁」，第 11 筆之後的資料就這樣消失了。這一頁不做分頁，所以一次要滿。
 */
const PROFILE_PAGE_SIZE = "20";

/** 端點 1。買家一筆都沒建過時 `records` 是空陣列，沒有任何合成列。 */
export async function fetchInvoiceProfiles(): Promise<InvoiceProfileListResult> {
	const env = await apiClient.authed.get<{
		records?: Record<string, unknown>[];
		total?: unknown;
		page?: unknown;
		pageSize?: unknown;
	}>(BASE, { query: { page: "1", page_size: PROFILE_PAGE_SIZE } });
	const data = env.data ?? {};
	return {
		records: (data.records ?? []).map(normalizeProfile),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
	};
}

/** 端點 2。 */
export async function createInvoiceProfile(
	input: CreateInvoiceProfileInput,
	idempotencyKey: string,
): Promise<InvoiceProfile> {
	const env = await apiClient.authed.post<Record<string, unknown>>(BASE, {
		...idempotent(idempotencyKey),
		json: {
			carrierType: input.carrierType,
			value: input.value,
			buyerTaxId: input.buyerTaxId,
			buyerName: input.buyerName,
			label: input.label,
			setDefault: input.setDefault,
		},
	});
	return normalizeProfile(env.data ?? {});
}

/**
 * 端點 3。**沒有 `carrierType`**：類型不可變更；也**不帶回原值**——DB 存的是
 * 密文，Gateway 拿不到明碼，所以編輯等於重新輸入一次完整值。
 */
export async function updateInvoiceProfile(
	profileId: string,
	input: UpdateInvoiceProfileInput,
	idempotencyKey: string,
): Promise<InvoiceProfile> {
	const env = await apiClient.authed.put<Record<string, unknown>>(
		`${BASE}/${encodeURIComponent(profileId)}`,
		{
			...idempotent(idempotencyKey),
			json: {
				value: input.value,
				buyerTaxId: input.buyerTaxId,
				buyerName: input.buyerName,
				label: input.label,
			},
		},
	);
	return normalizeProfile(env.data ?? {});
}

/** 端點 4。刪預設卻沒有先指定新預設 → 9304（後端擋下來，不幫你改指）。 */
export async function deleteInvoiceProfile(
	profileId: string,
	idempotencyKey: string,
): Promise<boolean> {
	const env = await apiClient.authed.delete<{ deleted?: boolean }>(
		`${BASE}/${encodeURIComponent(profileId)}`,
		idempotent(idempotencyKey),
	);
	return env.data?.deleted === true;
}

/** 端點 5。原預設由後端自動取消，前端不模擬互斥（§B-1.0.9 B10t）。 */
export async function setDefaultInvoiceProfile(
	profileId: string,
	idempotencyKey: string,
): Promise<InvoiceProfile> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${BASE}/${encodeURIComponent(profileId)}/default`,
		idempotent(idempotencyKey),
	);
	return normalizeProfile(env.data ?? {});
}

/* ===== B04／B05：訂單發票與文件詳情 ===== */

function bool(value: unknown): boolean {
	return value === true;
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

/**
 * 端點 8。零元訂單與尚未成立發票義務的訂單回 404 ＋ 9301（Tier A），呼叫端據此
 * 整張卡不渲染——那不是錯誤態。
 */
export async function fetchOrderInvoice(
	orderNo: string,
): Promise<OrderInvoice> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/orders/${encodeURIComponent(orderNo)}/invoice`,
	);
	const data = env.data ?? {};
	const documents = Array.isArray(data.documents)
		? (data.documents as Record<string, unknown>[])
		: [];
	return {
		orderNo: str(data.orderNo) || orderNo,
		invoiceMode: str(data.invoiceMode),
		state: str(data.state),
		stateNotice: str(data.stateNotice),
		documents: documents.map(normalizeDocumentSummary),
		canRequestCorrection: bool(data.canRequestCorrection),
		canFileComplaint: bool(data.canFileComplaint),
		complaintOpensAt: str(data.complaintOpensAt),
		existingComplaintId: str(data.existingComplaintId),
		latestCorrectionId: str(data.latestCorrectionId),
		notifiedEmailMasked: str(data.notifiedEmailMasked),
		shippingFeeNotice: str(data.shippingFeeNotice),
		snapshotVersion: str(data.snapshotVersion),
	};
}

/** 端點 69。授權述詞是「這份文件的買受人是不是你」，不需要也不接受 `orderNo`。 */
export async function fetchInvoiceDocument(
	documentId: string,
): Promise<InvoiceDocumentDetail> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/invoices/documents/${encodeURIComponent(documentId)}`,
	);
	const data = env.data ?? {};
	const lines = Array.isArray(data.lines)
		? (data.lines as Record<string, unknown>[])
		: [];
	const timeline = Array.isArray(data.timeline)
		? (data.timeline as Record<string, unknown>[])
		: [];
	return {
		summary: normalizeDocumentSummary(
			(data.summary as Record<string, unknown>) ?? {},
		),
		issuerName: str(data.issuerName),
		issuerTaxId: str(data.issuerTaxId),
		recipientName: str(data.recipientName),
		recipientTaxId: str(data.recipientTaxId),
		salesAmount: str(data.salesAmount),
		taxAmount: str(data.taxAmount),
		totalAmount: str(data.totalAmount),
		taxCategory: str(data.taxCategory),
		lines: lines.map(normalizeLine),
		timeline: timeline.map(normalizeTimelineItem),
	};
}

/**
 * PDF 二進位的網址（`/file/content` 子路由，58 §6.1.4）。
 *
 * 三件事：
 *
 *   1. **走 `apiClient.buildUrl`**。本機的 API base 是相對路徑 `/api/v1`、部署版
 *      是絕對 URL，自己拼字串會在其中一邊壞掉；而 §B-1.0.9 B01t 禁的是「把後端
 *      給的、已含 `/api/v1` 的路徑再餵給 `buildUrl`」——這裡是從零組路徑，不是
 *      那個情況。
 *   2. **只能是 `<a href>`，不能是 `fetch` 之後 `window.open`**：`await` 之後才
 *      呼叫 `window.open` 會被 Safari 的彈窗攔截器擋掉。授權靠 cookie
 *      （`credentials: "include"` ＋ 同源反代），瀏覽器直接導覽帶得過去。
 *   3. **只在 `summary.fileAvailable === true` 時組**。為 false 時這條路由必定
 *      回 4xx，而使用者看到的會是瀏覽器自己的錯誤頁，我們攔不到。
 */
export function invoiceFileUrl(orderNo: string, documentId: string): string {
	return apiClient.buildUrl(
		"authed",
		`/app/orders/${encodeURIComponent(orderNo)}/invoice_documents/${encodeURIComponent(documentId)}/file/content`,
	);
}

/** 端點 71 的回應（gateway `ResendInvoiceNotificationData` 逐欄）。 */
export interface InvoiceNotificationResendResult {
	/** 成功時恆為 `true`；被節流時走的是 `9352` 例外，不會回 `false`。 */
	resent: boolean;
	sentAt: string;
	/** 遮罩過的收件信箱。載具發票／收據沒有寄件對象時是空字串（誠實的「沒有」）。 */
	toMasked: string;
}

/**
 * 端點 71｜買家自己重寄一次發票／收據通知。
 *
 * 🔴 **只在 `canResendInvoiceNotification(summary.status)` 為真時才給入口**：後端對
 * 「還沒開好的文件」回的是 `9301` ＋ 一句解釋，而 `9301` 同時也是「文件不存在／
 * 不是你的」（Tier A）⇒ 前端分不出來，只能靠狀態先擋住。
 *
 * 🔴 `Idempotency-Key` 是**必填 header**（gateway `@Param ... true`）。每一次按下
 * 都是一次新的「請再寄一次」意圖 ⇒ 每次換新的一把；沿用同一把會讓第二次按下被
 * 冪等層吃掉並回上一次的結果，而使用者看到「已寄出」卻什麼都沒發生。真正防重複
 * 寄送的是後端那個 10 分鐘窗口（`9352`），不是這把鑰匙。
 *
 * ⚠️ 買家側的回應**沒有 `nextResendAvailableAt`**（那一欄只在賣家的端點 31）⇒
 * 畫面做不出倒數，只能在拿到 `9352` 時說「剛剛才寄過」。
 */
export async function resendBuyerInvoiceNotification(
	documentId: string,
	idempotencyKey: string,
): Promise<InvoiceNotificationResendResult> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`/app/invoices/documents/${encodeURIComponent(documentId)}/notification_resends`,
		idempotent(idempotencyKey),
	);
	const data = env.data ?? {};
	return {
		resent: bool(data.resent),
		sentAt: str(data.sentAt),
		toMasked: str(data.toMasked),
	};
}

/* ===== B09：買家發票中心（端點 68） ===== */

/**
 * 端點 68 的 query。日期是 `YYYY-MM-DD`；`status` 與 `documentKind` 是 CodeMeta 值
 * （`InvoiceDocumentStatus`／`InvoiceDocumentKind`）。
 */
export interface BuyerInvoiceDocumentsQuery {
	dateFrom?: string;
	dateTo?: string;
	status?: string;
	documentKind?: string;
	page?: number;
	pageSize?: number;
}

function normalizeBuyerDocumentItem(
	raw: Record<string, unknown>,
): BuyerInvoiceDocumentItem {
	return {
		summary: normalizeDocumentSummary(
			(raw.summary as Record<string, unknown>) ?? {},
		),
		issuerName: str(raw.issuerName),
	};
}

/**
 * 端點 68｜跨訂單的發票／收據列表（B09）。
 *
 * 三件事：
 *
 *   1. **授權述詞是 `recipient_type='BUYER' AND recipient_ref_id=當前買家`** ⇒ 回來
 *      的每一筆都已經是自己是買受人的文件，**前端不需要也不得再過濾**。自行過濾
 *      等於在前端再造一份會漂移的授權判定，而它只會漏掉東西、不會多擋掉什麼。
 *   2. **`page_size` 值域 1–50，逾越一律 400、後端不夾住** ⇒ 頁大小由本檔決定，
 *      不讓呼叫端（更不讓使用者）自選。
 *   3. **`date_from`／`date_to` 格式錯回 400** ⇒ 呼叫端驗過 `YYYY-MM-DD` 才發請求；
 *      不要靠後端的 400 當驗證器，那會讓使用者每打一個字就吃一次紅字。
 *
 * ⚠️ 分頁形狀由 `response.Page` 定死：讀 `data.records` ＋ `data.total`，不是
 * `list`／`items`。
 */
export async function listBuyerInvoiceDocuments(
	query: BuyerInvoiceDocumentsQuery,
): Promise<BuyerInvoiceDocumentListResult> {
	const params: Record<string, string> = {};
	if (query.dateFrom) params.date_from = query.dateFrom;
	if (query.dateTo) params.date_to = query.dateTo;
	if (query.status) params.status = query.status;
	if (query.documentKind) params.document_kind = query.documentKind;
	params.page = String(query.page ?? 1);
	params.page_size = String(query.pageSize ?? BUYER_DOCUMENTS_PAGE_SIZE);

	const env = await apiClient.authed.get<{
		records?: Record<string, unknown>[];
		total?: unknown;
		page?: unknown;
		pageSize?: unknown;
	}>("/app/invoices/documents", { query: params });
	const data = env.data ?? {};
	return {
		records: (data.records ?? []).map(normalizeBuyerDocumentItem),
		total: num(data.total),
		page: num(data.page),
		pageSize: num(data.pageSize),
	};
}

/** 端點 68 的頁大小。值域 1–50（見 `listBuyerInvoiceDocuments` 第 2 點）。 */
export const BUYER_DOCUMENTS_PAGE_SIZE = 20;

/* ===== B07：未收到發票／收據申訴 ===== */

const COMPLAINT_BASE = (orderNo: string) =>
	`/app/orders/${encodeURIComponent(orderNo)}/invoice_complaints`;

function normalizeComplaintResponse(
	raw: Record<string, unknown>,
): InvoiceComplaintResponseItem {
	return {
		responseId: str(raw.responseId),
		actor: str(raw.actor),
		action: str(raw.action),
		note: str(raw.note),
		occurredAt: str(raw.occurredAt),
	};
}

/**
 * wire → 前端型別。三個數字欄位（`remainingBusinessDays`／`overdueBusinessDays`／
 * `reopenCount`）**保字串**：它們可以是負數與空字串，在這裡 `Number()` 就等於
 * 把「後端算不出來」（空字串）與「零」壓成同一個值，而前者要顯示「處理期限
 * 計算中」、後者不是（§0.3 SlaCountdown 第 4 條）。
 */
function normalizeComplaint(raw: Record<string, unknown>): InvoiceComplaint {
	const responses = Array.isArray(raw.responses)
		? (raw.responses as Record<string, unknown>[])
		: [];
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
		responses: responses.map(normalizeComplaintResponse),
		version: str(raw.version),
		createdAt: str(raw.createdAt),
		reopenCount: str(raw.reopenCount),
		buyerReviewDueAt: str(raw.buyerReviewDueAt),
		reopenAllowedAfter: str(raw.reopenAllowedAfter),
	};
}

/**
 * 從 9310 的錯誤物件裡取既有案件 id。
 *
 * `preserveData` 的碼 HTTP 恆為 200 且 `data` 原樣透傳，而 client 把整個 envelope
 * 放進 `ApiClientError.payload` ⇒ 這裡讀得到。拿不到就回空字串，呼叫端退到端點 14
 * （§B07「端點 14 只在 9310 的補救路徑用」）。
 */
export function complaintIdFromError(err: unknown): string {
	if (!(err instanceof ApiClientError)) return "";
	const payload = err.payload as { data?: { complaintId?: unknown } } | null;
	return str(payload?.data?.complaintId);
}

/**
 * 端點 13。`description` 選填。
 *
 * 🔴 9310（已有未結案）的 HTTP 是 **200**，仍然會 throw ⇒ 呼叫端必須用
 * `invoiceBizCode(e)` 分流，照 HTTP status 分流會永遠漏接（§B-1.0.9 B05t）。
 */
export async function createInvoiceComplaint(
	orderNo: string,
	input: CreateInvoiceComplaintInput,
	idempotencyKey: string,
): Promise<InvoiceComplaint> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		COMPLAINT_BASE(orderNo),
		{
			...idempotent(idempotencyKey),
			json: { reasonCode: input.reasonCode, description: input.description },
		},
	);
	return normalizeComplaint(env.data ?? {});
}

/**
 * 端點 14。**正常流程不呼叫**——案件 id 直接取端點 8 的 `existingComplaintId`。
 * 只有 9310 而 `data.complaintId` 因故缺席時，才用它把既有案件找回來。
 */
export async function listOrderInvoiceComplaints(
	orderNo: string,
): Promise<InvoiceComplaint[]> {
	const env = await apiClient.authed.get<{
		records?: Record<string, unknown>[];
	}>(COMPLAINT_BASE(orderNo), { query: { page: "1", page_size: "50" } });
	return (env.data?.records ?? []).map(normalizeComplaint);
}

/** 端點 15。`InvoiceComplaintSection` 的唯一資料源。 */
export async function fetchInvoiceComplaint(
	orderNo: string,
	complaintId: string,
): Promise<InvoiceComplaint> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${COMPLAINT_BASE(orderNo)}/${encodeURIComponent(complaintId)}`,
	);
	return normalizeComplaint(env.data ?? {});
}

/**
 * 端點 63｜不接受賣家回覆。`reason` **必填**（與建案相反，照抄建案的 Dialog 會
 * 讓送出鍵永遠可按而後端回 400）。
 *
 * 三支動作端點共通：`expectedVersion` 來源**只有** `InvoiceComplaint.version`
 * （不得拿 `responses.length` 之類的推導值頂替，那不是樂觀鎖）；回應是轉移後的
 * 完整單筆 ⇒ 呼叫端直接拿它覆蓋 cache，不再打一次端點 15。
 */
export async function rejectInvoiceComplaintResponse(
	orderNo: string,
	complaintId: string,
	reason: string,
	expectedVersion: string,
	idempotencyKey: string,
): Promise<InvoiceComplaint> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${COMPLAINT_BASE(orderNo)}/${encodeURIComponent(complaintId)}/rejections`,
		{ ...idempotent(idempotencyKey), json: { reason, expectedVersion } },
	);
	return normalizeComplaint(env.data ?? {});
}

/** 端點 64｜撤回申訴。**沒有輸入欄**（端點層與前端都不自行加嚴）。 */
export async function withdrawInvoiceComplaint(
	orderNo: string,
	complaintId: string,
	expectedVersion: string,
	idempotencyKey: string,
): Promise<InvoiceComplaint> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${COMPLAINT_BASE(orderNo)}/${encodeURIComponent(complaintId)}/withdrawals`,
		{ ...idempotent(idempotencyKey), json: { expectedVersion } },
	);
	return normalizeComplaint(env.data ?? {});
}

/**
 * 端點 65｜重新提出。`reason` 選填。
 *
 * ⚠️ wire 上的欄名是 **`reason`**，不是 `description`——畫面上的 label 是
 * 「補充說明（選填）」，兩者不同名。
 */
export async function reopenInvoiceComplaint(
	orderNo: string,
	complaintId: string,
	reason: string,
	expectedVersion: string,
	idempotencyKey: string,
): Promise<InvoiceComplaint> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`${COMPLAINT_BASE(orderNo)}/${encodeURIComponent(complaintId)}/reopenings`,
		{ ...idempotent(idempotencyKey), json: { reason, expectedVersion } },
	);
	return normalizeComplaint(env.data ?? {});
}

/* ===== 付款後修改發票資料（端點 11／12，B06）===== */

const CORRECTION_BASE = (orderNo: string) =>
	`/app/orders/${encodeURIComponent(orderNo)}/invoice_corrections`;

/**
 * 端點 11。買家申請修改發票資料。
 *
 * 🔴 `expectedSnapshotVersion` 取自端點 8 的 `snapshotVersion`（**內容軸**），
 * 不是 `issue_attempt_seq`（實體軸）。作廢重開會讓後者 +1 而前者不變 ⇒ 拿錯一個，
 * 買家在賣家剛作廢重開之後送出的修改會被誤判成衝突。
 *
 * 🔴 **`handlingMode` 由後端決定**（開票前直接改／開票後作廢重開）。前端不得依
 * 「有沒有發票號碼」自己推導——判準在 §7.3.6 的狀態機裡。
 *
 * ⚠️ 能不能申請看端點 8 的 `canRequestCorrection`，不要自己算：後端的條件包含
 * 開票狀態、法定作廢窗口、是否已有進行中的申請三件事。
 */
export async function createInvoiceCorrection(
	orderNo: string,
	input: InvoiceCorrectionInput,
	expectedSnapshotVersion: string,
	reason: string,
	idempotencyKey: string,
): Promise<InvoiceCorrection> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		CORRECTION_BASE(orderNo),
		{
			...idempotent(idempotencyKey),
			json: {
				expectedSnapshotVersion,
				newInvoice: {
					profileId: input.profileId ?? "",
					// 🔴 手動輸入統編時必須明說 carrierType（見 InvoiceCorrectionInput
					// 的註解）。在 2026-09-04 之前這一欄整個沒送 ⇒ 端點 11 回 9305，
					// 而買家填的統編完全合法。
					carrierType: input.carrierType ?? "",
					buyerTaxId: input.buyerTaxId ?? "",
					buyerName: input.buyerName ?? "",
					email: input.email ?? "",
				},
				reason,
			},
		},
	);
	return normalizeCorrection(env.data ?? {});
}

/** 端點 12。查一筆修改申請的進度。 */
export async function fetchInvoiceCorrection(
	orderNo: string,
	correctionId: string,
): Promise<InvoiceCorrection> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`${CORRECTION_BASE(orderNo)}/${encodeURIComponent(correctionId)}`,
	);
	return normalizeCorrection(env.data ?? {});
}

function normalizeCorrection(data: unknown): InvoiceCorrection {
	const raw = (data ?? {}) as Record<string, unknown>;
	const s = (v: unknown) => (v == null ? "" : String(v));
	return {
		correctionId: s(raw.correctionId),
		handlingMode: s(raw.handlingMode),
		status: s(raw.status),
		statusNotice: s(raw.statusNotice),
		createdAt: s(raw.createdAt),
		rejectReasonCode: s(raw.rejectReasonCode),
	};
}
