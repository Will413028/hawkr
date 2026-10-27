import {
	type CertificationApplicationWire,
	type CertificationAuditLogWire,
	type Paged,
	type PagedWire,
	parsePaged,
} from "@/lib/product-certifications";
import { api } from "./api";

/**
 * 商品農產品認證審核的 ADM API 封裝（backend-payuni 19 號設計書 §9.2 / §9.3）。
 *
 * 🔴 路徑是 `/admin/product_certification_applications`，**不是** `/admin/certifications`
 * ——後者是商家認證（KYC，18 號設計書），兩者完全不同的東西，打錯會拿到別人的資料。
 *
 * 型別與純函式在 `@/lib/product-certifications`；本檔只負責打網路。
 */

const BASE = "/api/v1/admin/product_certification_applications";

interface Envelope<T> {
	code: number;
	message: string;
	data: T;
}

/**
 * 冪等鍵。
 *
 * ⚠️ 後端 `IdempotencyService` 會比對 request hash：**同一把鑰匙配不同 payload 會被
 * 拒絕**（`IdempotencyConflictException`），不是靜默覆蓋。所以送出內容一改就要換鑰匙，
 * 呼叫端負責這件事（裁決對話框在任一欄位變動時重發一把）。
 *
 * `crypto.randomUUID` 需要 secure context；ADM 走 HTTPS（localhost 也算），但仍留
 * fallback——這把鑰匙拿不到會讓整個裁決送不出去，代價遠大於一個較弱的亂數。
 */
export function newIdempotencyKey(prefix: string): string {
	const uuid =
		typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
			? crypto.randomUUID()
			: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
	return `${prefix}-${uuid}`;
}

export interface CertificationQueueQuery {
	/** ProductCertificationStatus；空字串＝不篩 */
	status?: string;
	/** CertificationType value（"3701"–"3706"）；空字串＝不篩 */
	certificationType?: string;
	storeId?: string;
	/** 商品名或證號 */
	keyword?: string;
	/** ISO 8601 UTC，由 `toUtcDayStart` 產生 */
	submittedFrom?: string;
	/** ISO 8601 UTC，由 `toUtcDayEnd` 產生 */
	submittedTo?: string;
	page: number;
	pageSize: number;
}

/**
 * 查詢參數用 snake_case（URL/query 一律 snake_case，回應 JSON 才是 camelCase），
 * 且**空值一律不帶**——帶了空字串等於送出一個「篩選為空字串」的條件。
 */
function buildQueueParams(q: CertificationQueueQuery): URLSearchParams {
	const params = new URLSearchParams({
		page: String(q.page),
		page_size: String(q.pageSize),
	});
	const optional: Array<[string, string | undefined]> = [
		["status", q.status],
		["certification_type", q.certificationType],
		["store_id", q.storeId],
		["keyword", q.keyword],
		["submitted_from", q.submittedFrom],
		["submitted_to", q.submittedTo],
	];
	for (const [key, value] of optional) {
		const trimmed = value?.trim();
		if (trimmed) params.set(key, trimmed);
	}
	return params;
}

export async function listCertificationApplications(
	q: CertificationQueueQuery,
): Promise<Paged<CertificationApplicationWire>> {
	const res = await api.get<Envelope<PagedWire<CertificationApplicationWire>>>(
		`${BASE}?${buildQueueParams(q)}`,
	);
	return parsePaged(res?.data, q.page, q.pageSize);
}

export async function getCertificationApplication(
	id: string,
): Promise<CertificationApplicationWire | null> {
	const res = await api.get<Envelope<CertificationApplicationWire>>(
		`${BASE}/${encodeURIComponent(id)}`,
	);
	return res?.data ?? null;
}

export async function listCertificationAuditLogs(
	id: string,
	page = 1,
	pageSize = 50,
): Promise<Paged<CertificationAuditLogWire>> {
	const params = new URLSearchParams({
		page: String(page),
		page_size: String(pageSize),
	});
	const res = await api.get<Envelope<PagedWire<CertificationAuditLogWire>>>(
		`${BASE}/${encodeURIComponent(id)}/audit_logs?${params}`,
	);
	return parsePaged(res?.data, page, pageSize);
}

/** approve / reject / revoke 三支共用同一份請求體，動作由路由區分。 */
export type CertificationReviewAction = "approve" | "reject" | "revoke";

export interface CertificationReviewPayload {
	idempotencyKey: string;
	/** 樂觀鎖：原樣回填詳情回應的 version */
	expectedVersion: string;
	/** reject / revoke 必填（缺少後端回業務碼 3028） */
	reasonCode?: string;
	/** 會展示給賣家 */
	publicReason?: string;
	/** Admin-only，賣家看不到 */
	internalNote?: string;
}

export async function reviewCertificationApplication(
	id: string,
	action: CertificationReviewAction,
	payload: CertificationReviewPayload,
): Promise<CertificationApplicationWire | null> {
	const res = await api.post<Envelope<CertificationApplicationWire>>(
		`${BASE}/${encodeURIComponent(id)}/${action}`,
		payload,
	);
	return res?.data ?? null;
}

/**
 * 重新查驗。
 *
 * 🔴 必須是 POST：它會打政府開放資料平台（消耗每日配額）並寫 DB，GET 會被瀏覽器、
 * 代理與 prefetch 重放（19 §9.3）。
 *
 * ⚠️ **只重新查驗、不改 status**——即使結果轉成 VERIFIED 也不會自動核准，要核准仍走
 * approve。這是刻意的：ADM 已經在看這一筆了，讓系統同時裁決會讓兩個決定在同一秒打架。
 *
 * ⚠️ 冪等鍵**每次點擊都要換新的**：它擋的是「連點兩次打兩次政府 API」，不是「這輩子
 * 只能查一次」。證書可能在申請之後才登記上政府資料庫，同一筆申請本來就會被查很多次。
 */
export async function reverifyCertificationApplication(
	id: string,
	idempotencyKey: string,
): Promise<CertificationApplicationWire | null> {
	const res = await api.post<Envelope<CertificationApplicationWire>>(
		`${BASE}/${encodeURIComponent(id)}/reverify`,
		{ idempotencyKey },
	);
	return res?.data ?? null;
}
