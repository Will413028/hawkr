/**
 * 商品匯入精靈 — 後端 17 endpoint 契約的 TS 型別。
 *
 * 對應 backend-payuni 40 號設計書 §4 + gateway handler
 * `gateway/internal/handler/seller_product_import.go` json tag 為唯一真實來源。
 *
 * 命名規則：
 *   - JSON body / response field：一律 camelCase（backend/docs/開發規範/API 設計規範.md「JSON body camelCase」）
 *   - URL query params：一律 snake_case（API 設計規範「URL/Query snake_case」），
 *     在 repo.ts 內保留 snake_case 不動
 *   - 數字欄位（id / 計數 / 金額 / 時間戳）：一律 string（API 設計規範）
 */

import type { InlineRun } from "@/lib/products/seller-repo";

/* ===== ImportJob 9 狀態（對應 40 §11 / §A）===== */
export const IMPORT_JOB_STATUS = {
	CREATED: "9101",
	PARSING: "9102",
	PARSE_FAILED: "9103",
	READY: "9104",
	COMMITTING: "9105",
	COMMITTED: "9106",
	EXPIRED: "9107",
	CANCELLED: "9108",
	CSV_INVALID: "9111",
} as const;

export type ImportJobStatusCode =
	(typeof IMPORT_JOB_STATUS)[keyof typeof IMPORT_JOB_STATUS];

/** 終態（不可再操作）。 */
export const TERMINAL_JOB_STATUSES: ImportJobStatusCode[] = [
	"9103",
	"9106",
	"9107",
	"9108",
];

/** active 集合（計入 5 jobs 上限的狀態）。 */
export const ACTIVE_JOB_STATUSES: ImportJobStatusCode[] = [
	"9101",
	"9102",
	"9104",
	"9105",
	"9111",
];

/* ===== ImportItem 7 狀態 ===== */
export const IMPORT_ITEM_STATUS = {
	PARSED: "9201",
	ASSET_DOWNLOADING: "9202",
	READY: "9203",
	REMOVED: "9205",
	COMMITTING: "9206",
	COMMITTED: "9207",
	COMMIT_FAILED: "9208",
} as const;

export type ImportItemStatusCode =
	(typeof IMPORT_ITEM_STATUS)[keyof typeof IMPORT_ITEM_STATUS];

/* ===== target_status（CSV 第 13 欄 + 批次套用）===== */
export const TARGET_STATUS = {
	ACTIVE: "ACTIVE",
	DRAFT: "DRAFT",
	PENDING_REVIEW: "PENDING_REVIEW",
} as const;

export type TargetStatusValue =
	(typeof TARGET_STATUS)[keyof typeof TARGET_STATUS];

/* ===== Validation Error（L1 CSV header / row 級錯誤） ===== */
export interface ValidationError {
	/** CSV 行號（header=1）；header 級錯誤回 "0"。 */
	csvLine: string;
	/** 欄位名（snake_case CSV 欄名）；header 級錯誤回 "header"。 */
	field: string;
	errorType: string;
	message: string;
	rawValue?: string;
}

/* ===== Item Error / Warning（L2 item 級錯誤） ===== */
export interface ItemIssue {
	errorType: string;
	severity: "ERROR" | "WARNING" | string;
	field?: string;
	message: string;
	csvLine?: string;
	/* ===== v3 新增（backend PR #1059）— gateway 上線後才會回填 ===== */
	/** 錯誤來源（CSV / VALIDATION / ASSET_DOWNLOAD / COMMIT 等），便於前端分組。 */
	source?: string;
	/** 重名同類錯誤指向的既有商品 id（DUPLICATE_NAME_VS_EXISTING 才有）。 */
	existingProductId?: string;
	existingProductName?: string;
	existingProductStatusLabel?: string;
}

/* ===== Progress（僅 detail 才有，brief 沒有） ===== */
export interface ImportJobProgress {
	overallPercentage: string;
	parsePercentage: string;
	assetDownloadPercentage: string;
	commitPercentage: string;
	parseCompleted: boolean;
	assetDownloadCompleted: boolean;
	commitCompleted: boolean;
}

/* ===== Job counters（detail 才有） ===== */
export interface ImportJobCounters {
	itemsTotal: string;
	itemsParsed: string;
	itemsAssetDownloading: string;
	itemsReady: string;
	itemsNeedsFix: string;
	itemsRemoved: string;
	itemsCommitting: string;
	itemsCommitted: string;
	itemsCommitFailed: string;
	itemsWithWarnings: string;
	itemsDuplicateExisting: string;
	assetsTotal: string;
	assetsPending: string;
	assetsDownloading: string;
	assetsUploaded: string;
	assetsFailed: string;
	assetsLinked: string;
}

/* ===== Job brief（list endpoint 用） ===== */
export interface ImportJobBrief {
	importJobId: string;
	status: ImportJobStatusCode;
	sourceFilename: string;
	totalItems: string;
	committedCount: string;
	failedCount: string;
	pendingCount: string;
	createdAt: string;
	expiresAt: string;
}

/* ===== Job detail ===== */
export interface ImportJobDetail {
	importJobId: string;
	status: ImportJobStatusCode;
	sourceFilename: string;
	totalRows: string;
	totalItems: string;
	counters: ImportJobCounters | null;
	targetStatus: TargetStatusValue;
	parseError?: string;
	createdAt: string;
	updatedAt: string;
	expiresAt: string;
	committedAt?: string;
	cancelledAt?: string;
	progress: ImportJobProgress | null;
	validationErrors?: ValidationError[];
}

/* ===== Item brief (list) ===== */
export interface ImportItemBrief {
	importItemId: string;
	productKey: string;
	status: ImportItemStatusCode;
	name: string;
	/** 葉節點 id（數字字串）；編輯頁 form load 唯一來源；尚未通過 ValidateLeafCategoryInternal 時為空字串（backend issue #1043）。 */
	categoryId: string;
	catL3Name: string;
	skuCount: string;
	priceMin: string;
	priceMax: string;
	/** presigned previewUrl 或原站 sourceUrl —— **不是**平台媒體 URL，
	 *  也沒有對應的 objectKey（圖片還在 tmp/product-import/ 前綴，image Worker
	 *  只服務 public/）。禁止丟進 buildImageUrl / resolveCdnUrl。 */
	firstImagePreviewUrl: string;
	blockingErrorCount: string;
	warningCount: string;
	commitFailedReason?: string;
	formalProductId?: string;
	needsFix: boolean;
	hasDuplicateExisting: boolean;
	commitRevision: string;
	/** 樂觀鎖 entity version；UI 編輯時必須帶回 expectedVersion；新建初始 "0"。**禁止**用 commitRevision（語意不同會撞 409）。 */
	version: string;
	/* ===== v3 新增（backend PR #1059）：以下欄位 gateway 上線後才會回填，
	 *  目前皆為 optional，前端 fallback 不渲染。 ===== */
	/** COMMITTED 後正式商品的 status code（如 4001=草稿 / 4004=上架）。 */
	formalProductStatus?: string;
	/** COMMITTED 後正式商品 status 的中文標籤（如「上架」、「草稿」）。 */
	formalProductStatusLabel?: string;
	/**
	 * 正式商品的發布生命週期（ProductPublicationState，4011~4014）。
	 * `"4014"` = 已移除（商品被軟刪）——此時 `/products/{id}` 會回 404，
	 * 不可再顯示「前往正式商品」連結。與 formalProductStatus 正交：
	 * 那個答「能不能賣」，本欄答「還在不在」。
	 */
	formalProductPublicationState?: string;
	/**
	 * `true` = 後端**查不到**正式商品狀態（對 product-service 的 RPC 失敗或無此列）。
	 *
	 * 🔴 與 `formalProductPublicationState === "4014"` 是兩件事：
	 * 4014 是「確定已移除」，本欄是「不知道」。修復前兩者都表現為欄位為空，
	 * 賣家無從分辨（backend #2819）。
	 */
	formalProductStatusUnavailable?: boolean;
	/** 同名既有商品 id（hasDuplicateExisting=true 時有值）。 */
	existingProductId?: string;
	/** 同名既有商品名稱。 */
	existingProductName?: string;
	/** 同名既有商品 status 的中文標籤。 */
	existingProductStatusLabel?: string;
}

/* ===== Shipping method 於 item SKU（response 形狀） ===== */
export interface ImportShippingMethodResponse {
	method: string; // ShippingMethod value（如 "3001" / "3002" / "3003"）
}

/* ===== SKU on item detail ===== */
export interface ImportItemSku {
	skuId: string;
	csvLine?: string;
	skuName: string;
	price: string;
	salePrice: string;
	stockQty: string;
	weight: string;
	unit: string;
	lengthCm: string;
	widthCm: string;
	heightCm: string;
	shippingMethods: ImportShippingMethodResponse[];
}

/* ===== Asset（圖片下載狀態；backend ImportItemAssetResponse） ===== */
export interface ImportItemAsset {
	assetId: string;
	usage: "GALLERY" | "DESCRIPTION" | string;
	sortOrder: string;
	blockIndex?: string;
	sourceUrl: string;
	status: string; // ImportAssetLocalStatus value
	previewUrl?: string;
	failureReason?: string;
	failureDetail?: string;
	retryCount: string;
}

/* ===== Description block（與正式 Product 同形狀，version 1.0）===== */
export interface ImportDescriptionBlock {
	type: "text" | "table" | "list" | "image";
	/** text block：inline runs（40 §12 inline-run 模型；與 legacy content 並存相容） */
	runs?: InlineRun[];
	content?: string;
	/** table block：`{ withHeadings, rows:[[{runs}]] }` JSON 字串（與正式 Product 同形） */
	tableJson?: string;
	/** list block：`{ ordered, items:[{runs}] }` JSON 字串（與正式 Product 同形） */
	listJson?: string;
	url?: string;
	assetStatus?: "downloading" | "ready" | "failed";
	/**
	 * text block 帶的 inline 樣式 JSON 字串（後端 HTML→JSON 轉換時，h1-h6 會帶
	 * `{"fontSize": 14-32, "fontWeight": "700"}` — 對應 40 號設計書 §12.2）。
	 * 渲染層若不解析此欄位，會 fallback 為一般 text，不影響顯示。
	 */
	styleJson?: string;
}

export interface ImportDescription {
	version: "1.0";
	sourceKind?: "url" | "html" | "text";
	blocks: ImportDescriptionBlock[];
}

/* ===== Item detail（backend ImportItemDetailResponse 結構：brief wrapper） ===== */
export interface ImportItemDetail {
	brief: ImportItemBrief;
	/** Backend 用 JSON 字串攜帶 ImportDescription；frontend 在 adapter 內 parse。 */
	descriptionJson: string;
	skus: ImportItemSku[];
	galleryAssets: ImportItemAsset[];
	descriptionAssets: ImportItemAsset[];
	errors: ItemIssue[];
	warnings: ItemIssue[];
	commitRevision: string;
	origin: string;
	hashtags: string[];
	targetStatus: TargetStatusValue;
}

/* ===== Gallery item（preview 用） ===== */
/**
 * 匯入預覽的圖片條目。欄名刻意**不叫** `imageKey`/`imageUrl`（正式商品的那組）：
 * 這裡的值是 presigned previewUrl 或原站 sourceUrl，不是平台媒體 URL，
 * 也沒有對應的 objectKey。名字不同，就是為了讓「不要拿去組 preset URL」
 * 這件事在呼叫點看得見（backend#2235）。
 */
export interface GalleryItem {
	previewUrl: string;
	sortOrder: string;
}

/* ===== Preview detail data（與正式 ProductDetail 相容） ===== */
export interface ProductDetailPresentation {
	id: string;
	name: string;
	rating: string;
	status: string;
	categoryId: string;
	origin: string;
	/** Backend 是 JSON 字串（ImportDescription 序列化）— 在 page 內 JSON.parse + 防呆。 */
	description: string;
	gallery: GalleryItem[];
	priceMin: string;
	priceMax: string;
	skus: ImportItemSku[];
	catL1Id: string;
	catL1Name: string;
	catL2Id: string;
	catL2Name: string;
	catL3Id: string;
	catL3Name: string;
}

/* ===== Preview list card（買家視角商品卡） ===== */
export interface ProductListPresentation {
	id: string;
	name: string;
	rating: string;
	categoryId: string;
	categoryName: string;
	origin: string;
	gallery: GalleryItem[];
	priceMin: string;
	priceMax: string;
	reviewCount: string;
	skuCount: string;
	/** 同 GalleryItem.previewUrl：presigned／原站 URL，不是平台媒體 URL。 */
	imagePreviewUrl: string;
}

/* ===== Item preview ===== */
export interface ImportItemPreview {
	importJobId: string;
	importItemId: string;
	importStatus: ImportItemStatusCode;
	validationErrors: ItemIssue[];
	warnings: ItemIssue[];
	formalProductId?: string;
	previewData: ProductDetailPresentation;
}

/* ===== Preview listing card ===== */
export interface ImportItemPreviewListingRecord {
	importItemId: string;
	importStatus: ImportItemStatusCode;
	previewData: ProductListPresentation;
}

export interface ImportPreviewListing {
	importJobId: string;
	totalEligible: string;
	totalNeedsFix: string;
	totalPending: string;
	totalRemoved: string;
	records: ImportItemPreviewListingRecord[];
	total: string;
	page: string;
	pageSize: string;
}

/* ===== Gallery image input（PUT items/{id} 用）=====
 *
 * 每筆 `sourceUrl` XOR `stagedToken` 二擇一：
 *   - 既有圖（從 CSV 解析來的 sourceUrl）原樣保留 → 送 `sourceUrl`
 *   - 編輯時本地上傳 / 重排後的新圖 → 送 `stagedToken`
 *
 * 後端 stagedToken 支援尚在 backend issue #1014 追蹤中；前端先把上傳完拿到的
 * stagedToken 帶上，後端就緒前 fallback 走 sourceUrl（uploadImageFile 完成後
 * `previewUrl` 即可作為 sourceUrl）。
 */
export interface ImportItemImageInput {
	sourceUrl?: string;
	stagedToken?: string;
}

/* ===== SKU input（PUT items/{id} 用） ===== */
export interface ShippingMethodInput {
	method: string; // ShippingMethod value
	temperature: string; // 固定 AMBIENT_TEMPERATURE_CODE（B4 刪）
}

export interface UpdateImportSkuInput {
	skuId?: string; // 空 = 新增；非空 = 更新
	skuName?: string;
	price?: string;
	salePrice?: string;
	stockQty?: string;
	weight?: string;
	unit?: string;
	lengthCm?: string;
	widthCm?: string;
	heightCm?: string;
	shippingMethods?: ShippingMethodInput[];
	delete?: boolean;
}

/* ===== Update item input（PUT /items/{id} 用，set-pattern）===== */
export interface UpdateImportItemInput {
	expectedVersion: string;
	nameSet?: boolean;
	name?: string;
	categoryIdSet?: boolean;
	categoryId?: string;
	originSet?: boolean;
	origin?: string;
	targetStatusSet?: boolean;
	targetStatus?: TargetStatusValue;
	descriptionSet?: boolean;
	/** Backend 用 JSON 字串攜帶；adapter 序列化 ImportDescription 後寫入。 */
	description?: string;
	hashtagsSet?: boolean;
	hashtags?: string[];
	galleryImagesSet?: boolean;
	galleryImages?: ImportItemImageInput[];
	skusSet?: boolean;
	skus?: UpdateImportSkuInput[];
	allowDuplicateNameSet?: boolean;
	allowDuplicateName?: boolean;
}

/* ===== Batch update shipping input ===== */
export interface BatchShippingMethod {
	type: string; // ShippingMethod value
	temperature: string;
	freeThreshold?: string;
}

export interface BatchUpdateShippingInput {
	importItemIds?: string[];
	applyAll?: boolean;
	shippingMethods: BatchShippingMethod[];
}

/* ===== Batch update category input ===== */
export interface BatchUpdateCategoryInput {
	importItemIds?: string[];
	applyAll?: boolean;
	categoryId: string;
	confirmationToken: string;
}

export interface CategoryConfirmationToken {
	confirmationToken: string;
	expiresAt: string;
	catL1Id: string;
	catL1Name: string;
	catL2Id: string;
	catL2Name: string;
	catL3Id: string;
	catL3Name: string;
}

/* ===== Batch update target_status input ===== */
export interface BatchUpdateTargetStatusInput {
	importItemIds?: string[];
	applyAll?: boolean;
	targetStatus: TargetStatusValue;
}

/* ===== Commit input ===== */
export interface CommitImportJobInput {
	retryFailedOnly?: boolean;
	includeItemIds?: string[];
}

/* ===== List query params（注意：URL query 仍走 snake_case，repo.ts 內處理） ===== */
export interface ListImportJobsParams {
	status?: ImportJobStatusCode[];
	page?: number;
	pageSize?: number;
}

export interface ListImportItemsParams {
	status?: ImportItemStatusCode[];
	onlyWithErrors?: boolean;
	includeRemoved?: boolean;
	page?: number;
	pageSize?: number;
}

export interface ListPreviewParams {
	includePending?: boolean;
	page?: number;
	pageSize?: number;
}

/* ===== L1 fail（HTTP 400）payload ===== */
export interface L1FailPayload {
	code: number;
	message: string;
	validationErrors?: ValidationError[];
}

/* ===== Create job response（含 L2 fail；status=CSV_INVALID 時 validationErrors 填入）===== */
export interface CreateImportJobResponse {
	importJobId: string;
	status: ImportJobStatusCode;
	dedupHit?: boolean;
	validationErrors?: ValidationError[];
}

/* ===== Commit skip reason 代碼（backend PR #1059 v3）===== */
export const COMMIT_SKIP_REASON = {
	BLOCKING_ERROR: "BLOCKING_ERROR",
	ALREADY_COMMITTED: "ALREADY_COMMITTED",
	REMOVED: "REMOVED",
	NOT_READY: "NOT_READY",
} as const;

export type CommitSkipReasonCode =
	(typeof COMMIT_SKIP_REASON)[keyof typeof COMMIT_SKIP_REASON];

export interface CommitSkipReason {
	importItemId: string;
	reasonCode: CommitSkipReasonCode | string;
}

/* ===== Commit job response ===== */
export interface CommitImportJobResponse {
	status: string;
	queuedCount: string;
	/* ===== v3 新增（gateway 上線後才會回填，optional fallback）===== */
	/** 因故未排入 commit 的 item ids（與 skipReasons 1:1 對應）。 */
	skippedItemIds?: string[];
	/** 對應 skippedItemIds 的原因清單。 */
	skipReasons?: CommitSkipReason[];
}

/* ===== List page result（backend response.PageData 是 camelCase pageSize） ===== */
export interface PageResult<T> {
	records: T[];
	total: string | number;
	page: string | number;
	pageSize: string | number;
}
