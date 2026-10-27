/**
 * `GET /api/v1/admin/after_sales/{case_no}` 的 wire 契約，與爭議詳情頁對它做的取值運算。
 *
 * 形狀權威 = backend-payuni `gateway/internal/handler/aftersales.go`：
 * `AdminAppealDetailResponse`（:995-1000）／`AfterSaleCaseDetailResponse`（:85-131）／
 * `EvidenceResponse`（:146-152）／`AfterSaleCaseItemResponse`（:134-143）／
 * `AppealEventResponse`（:958-963）／`AdminContextResponse`（:1012-1024）。
 *
 * 從頁面拆出來是為了讓形狀能被實打回應釘住（tests/dispute-detail-media-contract.test.ts）。
 * 頁面原本宣告 `evidence: { url, thumbnailUrl }`、`items: { productName, unitPrice }`、
 * `caseDetail.appealSubmittedAt`、`timeline: { desc }` —— 這四組欄位 gateway 從來沒送過，
 * 於是佐證照片全渲染成空框、金額印出字串 "undefined"、申訴時間恆為 "-"、時間軸說明全空
 * （hawkr-website#430 A2）。TypeScript 攔不到這種錯：介面是我們自己宣告的，
 * 宣告錯了就一路錯到畫面。
 *
 * 媒體欄位一律讀 `*Key`（objectKey ＝ SSOT，設計書 49 §6.1），顯示 URL 交給
 * `buildImageUrl(key, preset)` 組；不讀 gateway 併送的 `*Url`（那是固定 `/i/original/`
 * 的原圖，縮圖位置用它等於拉原圖）。
 */

/** AfterSaleEvidenceType value（CodeMeta `內部技術碼.md`）。 */
export const EVIDENCE_TYPE_BUYER = "8601";
export const EVIDENCE_TYPE_SELLER_INSPECTION = "8602";

/** AppealEventResponse.eventType 的線上實值（gateway 送字面名稱，不是數字碼）。 */
export const APPEAL_EVENT_SUBMITTED = "APPEAL_SUBMITTED";

export interface EvidencePhoto {
	assetId: string;
	/** 佐證圖 objectKey — SSOT，顯示走 buildImageUrl。 */
	imageKey: string;
	/** gateway 併送的 `/i/original/` 顯示 URL（本頁不讀，留著標示契約全貌）。 */
	imageUrl: string;
	evidenceType: string;
	createdAt: string;
}

export interface CaseItem {
	orderItemId: string;
	skuId: string;
	/** 售後案件明細沒有商品名，只有規格名（proto `AfterSaleCaseItemView` 無 product_name）。 */
	skuName: string;
	quantity: string;
	originalPrice: string;
	actualPayAmount: string;
	imageKey: string;
	imageUrl: string;
}

export interface CaseDetail {
	caseNo: string;
	orderNo: string;
	/** 買家營運鍵（buyers PK surrogate，非 account_id；設計書 44）。 */
	buyerId: string;
	storeId: string;
	status: string;
	appealState: string;
	refundableAmount: string;
	items: CaseItem[];
	evidence: EvidencePhoto[];
	/** 出貨照片 objectKey — SSOT；gateway 另送 shipmentPhotoUrls（原圖 URL），本頁不讀。 */
	shipmentPhotoKeys: string[] | null;
}

export interface TimelineEntry {
	eventType: string;
	label: string;
	timestamp: string;
	description: string;
}

export interface Decision {
	outcome: string;
	refundAmount: string;
	reason: string;
	decidedAt: string;
}

export interface Arbitration {
	state: string;
	timeline: TimelineEntry[];
	decision?: Decision;
}

export interface AdminContext {
	decidedByAdminId?: string;
	buyerAppealReason: string;
	buyerAppealDescription: string;
	sellerRejectReason: string;
	sellerRejectMessage: string;
	buyerName: string;
	/** 買家頭像 objectKey — SSOT。扁平成對，不帶主體前綴（49 §6.6）。 */
	avatarKey: string;
	avatarUrl: string;
	sellerName: string;
	/** 商家 logo objectKey — SSOT（欄位名就叫 logoKey，不是 sellerAvatarKey）。 */
	logoKey: string;
	logoUrl: string;
}

export interface DisputeDetail {
	caseDetail: CaseDetail;
	arbitration: Arbitration;
	adminContext: AdminContext;
}

/**
 * 申訴提交時間。`caseDetail` 沒有這個欄位（列表的 AppealSummaryResponse 才有
 * `appealSubmittedAt`），詳情這邊唯一的來源是仲裁時間軸上的 APPEAL_SUBMITTED 事件。
 * 沒申訴過的案件時間軸為空 → 回空字串，呼叫端顯示 "-"。
 */
export function selectAppealSubmittedAt(
	arbitration: Arbitration | undefined,
): string {
	return (
		arbitration?.timeline?.find((e) => e.eventType === APPEAL_EVENT_SUBMITTED)
			?.timestamp ?? ""
	);
}

export interface DisputePhotoKeys {
	buyerOriginal: string[];
	buyerAppeal: string[];
	sellerInspection: string[];
	sellerShipment: string[];
}

/**
 * 把佐證照片分進畫面四個區塊，回傳 objectKey。
 *
 * 買家舉證（8601）在原始申請與申訴追加兩個時點都會上傳，DB 沒有欄位區分，只能以
 * 申訴提交時間切分；申訴時間取不到時全部歸「原始申請」（不會憑空生出追加區塊）。
 * 沒有 objectKey 的列直接略過 —— 渲染成空 `<img>` 正是本次要修掉的症狀，缺值該讓區塊
 * 顯示「無照片」，而不是掛一個破圖框。
 */
export function selectDisputePhotoKeys(
	caseDetail: CaseDetail | undefined,
	appealSubmittedAt: string,
): DisputePhotoKeys {
	const evidence = caseDetail?.evidence ?? [];
	const keysOf = (list: EvidencePhoto[]) =>
		list.map((e) => e.imageKey).filter((key): key is string => Boolean(key));
	const buyerEvidence = evidence.filter(
		(e) => e.evidenceType === EVIDENCE_TYPE_BUYER,
	);

	return {
		buyerOriginal: keysOf(
			buyerEvidence.filter(
				(e) => !appealSubmittedAt || e.createdAt < appealSubmittedAt,
			),
		),
		buyerAppeal: appealSubmittedAt
			? keysOf(buyerEvidence.filter((e) => e.createdAt >= appealSubmittedAt))
			: [],
		sellerInspection: keysOf(
			evidence.filter(
				(e) => e.evidenceType === EVIDENCE_TYPE_SELLER_INSPECTION,
			),
		),
		sellerShipment: (caseDetail?.shipmentPhotoKeys ?? []).filter(Boolean),
	};
}
