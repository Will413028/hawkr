/**
 * 商品匯入精靈文案 i18n map（繁中）。
 *
 * 對應 backend 40 號設計書 §11 錯誤碼 / §A 狀態碼，加上 UI 文案。
 * Repo 本身沒 next-intl，全繁中寫死；本檔集中管理避免文案散落。
 */

import {
	IMPORT_ITEM_STATUS,
	IMPORT_JOB_STATUS,
	type ImportItemStatusCode,
	type ImportJobStatusCode,
	TARGET_STATUS,
	type TargetStatusValue,
} from "./types";

/* ===== Job 狀態文字 + 視覺 tone ===== */
export type StatusTone =
	| "primary"
	| "success"
	| "warning"
	| "danger"
	| "default";

export const JOB_STATUS_LABEL: Record<ImportJobStatusCode, string> = {
	[IMPORT_JOB_STATUS.CREATED]: "建立中",
	[IMPORT_JOB_STATUS.PARSING]: "解析中",
	[IMPORT_JOB_STATUS.PARSE_FAILED]: "解析失敗",
	[IMPORT_JOB_STATUS.READY]: "可預覽",
	[IMPORT_JOB_STATUS.COMMITTING]: "上架中",
	[IMPORT_JOB_STATUS.COMMITTED]: "已上架",
	[IMPORT_JOB_STATUS.EXPIRED]: "已過期",
	[IMPORT_JOB_STATUS.CANCELLED]: "已取消",
	[IMPORT_JOB_STATUS.CSV_INVALID]: "CSV 內容有誤",
};

export const JOB_STATUS_TONE: Record<ImportJobStatusCode, StatusTone> = {
	[IMPORT_JOB_STATUS.CREATED]: "primary",
	[IMPORT_JOB_STATUS.PARSING]: "primary",
	[IMPORT_JOB_STATUS.PARSE_FAILED]: "danger",
	[IMPORT_JOB_STATUS.READY]: "success",
	[IMPORT_JOB_STATUS.COMMITTING]: "primary",
	[IMPORT_JOB_STATUS.COMMITTED]: "default",
	[IMPORT_JOB_STATUS.EXPIRED]: "default",
	[IMPORT_JOB_STATUS.CANCELLED]: "default",
	[IMPORT_JOB_STATUS.CSV_INVALID]: "warning",
};

/* ===== Item 狀態文字 + tone ===== */
export const ITEM_STATUS_LABEL: Record<ImportItemStatusCode, string> = {
	[IMPORT_ITEM_STATUS.PARSED]: "解析完成",
	[IMPORT_ITEM_STATUS.ASSET_DOWNLOADING]: "圖片下載中",
	[IMPORT_ITEM_STATUS.READY]: "可上架",
	[IMPORT_ITEM_STATUS.REMOVED]: "已移除",
	[IMPORT_ITEM_STATUS.COMMITTING]: "上架中",
	[IMPORT_ITEM_STATUS.COMMITTED]: "已上架",
	[IMPORT_ITEM_STATUS.COMMIT_FAILED]: "上架失敗",
};

export const ITEM_STATUS_TONE: Record<ImportItemStatusCode, StatusTone> = {
	[IMPORT_ITEM_STATUS.PARSED]: "primary",
	[IMPORT_ITEM_STATUS.ASSET_DOWNLOADING]: "primary",
	[IMPORT_ITEM_STATUS.READY]: "success",
	[IMPORT_ITEM_STATUS.REMOVED]: "default",
	[IMPORT_ITEM_STATUS.COMMITTING]: "primary",
	[IMPORT_ITEM_STATUS.COMMITTED]: "default",
	[IMPORT_ITEM_STATUS.COMMIT_FAILED]: "danger",
};

/* ===== Target status 文字 ===== */
export const TARGET_STATUS_LABEL: Record<TargetStatusValue, string> = {
	[TARGET_STATUS.ACTIVE]: "立即上架",
	[TARGET_STATUS.DRAFT]: "不上架",
	[TARGET_STATUS.PENDING_REVIEW]: "送審",
};

/* ===== 配送類型 ===== */
export const SHIPPING_TYPE_LABEL: Record<"TAQBIN" | "SEVEN", string> = {
	TAQBIN: "黑貓宅配",
	SEVEN: "7-11 店到店",
};

/* ===== ValidationError code map（28+ 項，前端 fallback 用） ===== */
/**
 * 對應 backend `ImportItemErrorType` 完整 41 條（services/java/shared/common-core/
 * src/main/java/io/github/will413028/hawkr/common/utils/code/ImportItemErrorType.java），
 * 以及 gateway 業務錯誤碼（PRODUCT_IMPORT_*）與 import-service 端產出（GALLERY_TRUNCATED_EXCEEDS_LIMIT
 * / VALIDATION_DRIFT / IMPORT_ITEM_ALREADY_COMMITTED）。
 *
 * 翻譯文案對齊 backend `ImportErrorMessageCatalog`（已於 PR #1099 補齊全 41 條 code-only fallback）。
 *
 * 維護規則：新增 ImportItemErrorType 時必須在此 map 補對應翻譯；
 * 後端已有 `ImportErrorMessageCatalogTest.everyImportItemErrorTypeHasCodeOnlyCatalogEntry` 強制
 * 後端 catalog 對應，前端目前以 type test 約束（見檔尾 `assertValidationLabelCoverage`）。
 */
export const VALIDATION_ERROR_LABEL: Record<string, string> = {
	// === Gateway 業務錯誤碼（檔案級 L1 與操作級，由 gRPC 拋回） ===
	PRODUCT_IMPORT_FILE_TOO_LARGE: "檔案過大（請小於 2 MB）",
	PRODUCT_IMPORT_FILENAME_CONFLICT:
		"已有同名檔案正在處理中，請改檔名或等前一個任務完成",
	PRODUCT_IMPORT_RATE_LIMITED:
		"已達匯入任務上限（同時進行中最多 5 個 / 當日累計最多 10 個），請等任務完成或明日再試",
	PRODUCT_IMPORT_CONFIRMATION_TOKEN_INVALID:
		"分類二次確認 token 無效或已逾期，請重新選擇分類",
	PRODUCT_IMPORT_CONFIRMATION_TOKEN_USED: "此分類確認碼已使用過，請重新取得",
	PRODUCT_IMPORT_CONFIRMATION_TOKEN_MISMATCH: "確認碼與所選分類不符",
	PRODUCT_IMPORT_JOB_NOT_COMMITTABLE: "目前狀態無法上架",
	PRODUCT_IMPORT_HAS_BLOCKING_ITEMS: "尚有商品需修正才能上架",
	PRODUCT_IMPORT_JOB_NOT_CANCELLABLE: "目前狀態無法取消",
	PRODUCT_IMPORT_ITEM_ALREADY_COMMITTED:
		"該商品已併入正式商品，無法再編輯或移除",

	// === backend ImportItemErrorType 41 條（依 enum 順序）===
	REQUIRED_MISSING: "必填欄位缺漏，請至編輯頁補齊",
	ENUM_INVALID: "欄位值不在可選清單內",
	NUMERIC_INVALID: "數字欄位格式不正確",
	SALE_PRICE_INVALID: "售價必須大於 0 且小於原價",
	PRODUCT_FIELD_INCONSISTENT: "同一商品多筆規格的商品欄位不一致（已取第一筆）",
	CATEGORY_INVALID: "商品分類不合法或非葉節點，請至編輯頁重新選擇",
	URL_INVALID: "網址格式不正確（必須為 https）",
	IMAGE_DOWNLOAD_FAILED: "圖片下載失敗，請確認圖片網址可正常開啟",
	HTML_PARSE_WARNING: "商品描述 HTML 解析有警告，已盡量保留樣式",
	ENCODING_NOT_UTF8:
		"CSV 編碼非 UTF-8，請另存為 UTF-8（建議使用 Excel 另存新檔選 CSV UTF-8）",
	COVER_IMAGE_MISSING: "封面圖（第 1 張商品圖）缺漏",
	SHIPPING_AT_LEAST_ONE_REQUIRED: "至少需勾選一種配送方式",
	SHIPPING_DIMENSION_MISSING: "缺長寬高，無法判斷配送方式可用範圍",
	SHIPPING_WEIGHT_EXCEEDED: "重量超出該配送方式可承載範圍，已自動停用",
	SHIPPING_DIMENSION_EXCEEDED: "尺寸超出該配送方式可承載範圍，已自動停用",
	SHIPPING_TEMP_INCOMPATIBLE: "該配送方式不支援目前溫層，已自動停用",
	SHIPPING_NOT_ENABLED: "您尚未開通該配送方式",
	SHIPPING_ALL_DOWNGRADED:
		"所有配送方式都不可用，請調整尺寸 / 重量或啟用其他配送方式",
	SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED:
		"尺寸 / 重量超出該配送方式上限，已自動停用",
	DUPLICATE_NAME_IN_JOB: "此批次內有重複的商品名稱，請改名",
	DUPLICATE_NAME_VS_EXISTING:
		"與既有商品同名（如為相同商品請勾選「允許重名」一併匯入）",
	// 19 號設計書 §6.5.3／§6.5.4：這一碼自本期起是 warning 而不是 error —— 政府資料庫
	// 查無不代表證書是假的（可能還沒登記上去），該筆會轉人工審核而不是被丟掉。
	// 沿用舊文案「認證資訊驗證失敗」會讓賣家以為那一筆沒進去，實際上它已在 ADM 佇列。
	// Java 側 `ImportErrorMessageCatalog` 的同一碼必須同步（兩處是同一份文案的兩個拷貝）。
	CERT_VERIFY_FAILED:
		"認證編號查驗未通過，已轉送人工審核；商品可正常上架，標章待審核通過後顯示",
	// §6.5.3：移除同步查驗後本碼在匯入路徑不再可達，但**碼值保留**——既有
	// `product_import_item_error` 列還存著它，刪掉文案會讓歷史列變成無法翻譯的孤兒值。
	CERT_VERIFY_PENDING: "認證資訊驗證逾時，將於背景重試",
	CERT_FORMAT_INVALID: "認證資訊欄位格式不正確",
	CERT_ENUM_INVALID: "認證代碼不在可選清單內，已自動略過該筆認證",
	CERT_NO_MISSING: "認證編號缺漏，已自動略過該筆認證",
	HASHTAG_LENGTH_EXCEEDED: "標籤超過 40 字，已自動略過",
	HASHTAG_COUNT_EXCEEDED: "標籤數量超過 10 個，已自動截至前 10 個",
	HASHTAG_INVALID_CHAR: "標籤包含不允許的特殊字元，已自動略過",
	HASHTAG_EMPTY_AFTER_NORMALIZE: "標籤經整理後為空，已自動略過",
	GALLERY_TRUNCATED_EXCEEDS_LIMIT: "商品圖超過上限已截前 9 張，請至編輯頁確認",
	VALIDATION_DRIFT: "上架規則已變更，請重新檢視商品內容後再 commit",
	GALLERY_REQUIRED: "商品圖至少需 1 張",
	GALLERY_EXCEEDS_LIMIT: "商品圖超過 9 張上限",
	NAME_REQUIRED: "商品名稱必填",
	NAME_LENGTH_EXCEEDS: "商品名稱超過 80 字",
	CATEGORY_NOT_LEAF: "請選擇葉節點分類",
	CATEGORY_NOT_FOUND: "找不到指定分類",
	SKUS_REQUIRED: "至少需要一組商品規格",
	SKUS_EXCEEDS_LIMIT: "商品規格數量超過上限",
	PRICE_INVALID: "規格原價或售價不合法（原價必須 > 0，售價必須 > 0 且 < 原價）",
	DESC_BLOCK_INVALID: "商品描述格式不合法",
	ASSET_NOT_READY: "圖片尚未上傳完成，請稍候",
};

/* ===== Commit 批次結果 — skip reason 對應文字（v3）===== */
export const COMMIT_SKIP_REASON_LABEL: Record<string, string> = {
	BLOCKING_ERROR: "尚有阻擋錯誤未修正",
	ALREADY_COMMITTED: "已上架，無需重複 Commit",
	REMOVED: "已移除，不會送出",
	NOT_READY: "尚未進入可上架狀態",
};

export function commitSkipReasonLabel(reasonCode: string): string {
	return COMMIT_SKIP_REASON_LABEL[reasonCode] ?? reasonCode;
}

/** 友善文字：先查 map，沒查到就回原始 server message。 */
export function validationErrorLabel(
	errorType: string,
	fallbackMessage?: string,
): string {
	return VALIDATION_ERROR_LABEL[errorType] ?? fallbackMessage ?? errorType;
}

/**
 * Internal field name → 中文 label。對齊 backend `ImportErrorMessageCatalog.fieldLabel()`
 * 翻譯表（PR #1099）。所有顯示給賣家的錯誤 / 警告欄位都必須走此函式，
 * 禁止直接 print 原始 field code（`category_id` / `skus` / `skus[0].price` 等）。
 *
 * 巢狀欄位（如 `skus[0].price` / `description_json.blocks[0].type`）依根欄位翻譯，
 * 並保留索引以利賣家定位「第幾組規格」。
 */
const FIELD_LABEL_MAP: Record<string, string> = {
	name: "商品名稱",
	product_name: "商品名稱",
	category_id: "商品分類",
	gallery_images: "商品圖",
	description: "商品描述",
	description_json: "商品描述",
	origin: "產地",
	target_status: "上架狀態",
	certifications: "認證",
	hashtags: "標籤",
	skus: "商品規格",
	sku_name: "規格名稱",
	sku_shipping: "配送方式",
	shipping_methods: "運送方式",
	sku_price: "規格原價",
	sku_sale_price: "規格售價",
	sku_stock_qty: "規格庫存",
	sku_unit: "規格單位",
	sku_weight: "規格重量",
	sku_dimension: "規格長寬高",
	price: "原價",
	sale_price: "售價",
	stock_qty: "庫存",
	unit: "單位",
	weight: "重量",
	length_cm: "長",
	width_cm: "寬",
	height_cm: "高",
};

export function fieldLabel(field: string | null | undefined): string {
	if (!field) return "";
	if (field.startsWith("description_json")) {
		return FIELD_LABEL_MAP.description_json;
	}
	const exact = FIELD_LABEL_MAP[field];
	if (exact) return exact;
	// 處理 skus[0].price 之類巢狀：抽 root + 留索引尾巴
	const nestedMatch = field.match(/^([a-z_]+)(\[(\d+)\])?\.?([a-z_]+)?/i);
	if (nestedMatch) {
		const [, root, , idx, sub] = nestedMatch;
		const rootLabel = FIELD_LABEL_MAP[root];
		const subLabel = sub ? FIELD_LABEL_MAP[sub] : undefined;
		if (rootLabel && subLabel) {
			return idx !== undefined
				? `${rootLabel} 第 ${Number(idx) + 1} 組 · ${subLabel}`
				: `${rootLabel} · ${subLabel}`;
		}
		if (rootLabel) {
			return idx !== undefined
				? `${rootLabel} 第 ${Number(idx) + 1} 組`
				: rootLabel;
		}
	}
	return field;
}

/* ===== 常用 UI 文字 ===== */
export const UI_TEXT = {
	pageTitle: "批次匯入商品",
	downloadTemplate: "下載 CSV 範本",
	newJob: "新增匯入任務",
	jobsListEmpty: "尚無匯入任務",
	rateLimited: "已達匯入任務上限（同時進行 5 個 / 當日累計 10 個）",
	expireSoon: "即將過期",
	tableCols: {
		createdAt: "建立時間",
		filename: "檔名",
		status: "狀態",
		progress: "進度",
		totalItems: "總商品數",
		committedItems: "已上架",
		expiresIn: "剩餘到期天數",
		actions: "操作",
	},
	actions: {
		view: "查看詳情",
		cancel: "取消任務",
		commit: "全部 Commit",
		commitSelected: "Commit 已勾選",
		preview: "預覽此次匯入",
		retryAssets: "重試所有失敗下載",
		edit: "編輯",
		remove: "移除",
		restore: "恢復",
		retryItemAssets: "重試失敗圖片",
		save: "儲存",
		saveAndNext: "儲存並下一筆",
		cancelEdit: "取消",
		removeItem: "移除此商品",
		previewItem: "預覽",
		openFormalProduct: "前往正式商品",
		formalProductRemoved: "已移除",
		formalProductStatusUnavailable: "狀態暫時無法取得",
	},
	bulk: {
		setShipping: "批次設定運送",
		setCategory: "批次套用商品類別",
		setTargetStatus: "批次套用目標上架狀態",
		removeBatch: "批次移除",
		restoreBatch: "批次恢復",
		commitSelected: "Commit 已勾選",
		selected: (n: number) => `已選 ${n} 筆`,
	},
	uploadModal: {
		title: "上傳 CSV 檔案",
		retitle: "重傳 CSV 檔案",
		dragHint: "拖曳 .csv 檔案到這裡，或點擊選擇檔案",
		accepted: ".csv（UTF-8，2 MB 以內）",
		upload: "上傳",
		uploading: "上傳中…",
		errors: "檔案驗證失敗，請修正後重傳：",
		templateHint: "首次匯入？先下載 26 欄 CSV 範本",
		openGuide: "查看 CSV 編輯指南",
		openGuideOnError: "遇到格式錯誤？查看 CSV 編輯指南",
	},
	csvGuideModal: {
		title: "CSV 編輯指南",
		close: "關閉",
		section1: {
			heading: "推薦做法",
			body: "建議使用 Excel、Google Sheets 或 Numbers 編輯，匯出時選擇 CSV (UTF-8) 格式。",
			note: "這些工具會自動處理欄位中的逗號、換行、雙引號，不必手動跳脫。",
		},
		section2: {
			heading: "手動編輯時必須遵守",
			bodyPrefix: "若欄位內容含",
			bodyEmphasis: "逗號、換行或雙引號",
			bodyMiddle: "，該欄位必須用半形雙引號",
			bodyQuoteChar: '"',
			bodyAfterQuote: "整段包起來；欄位中原本的雙引號要寫成連續兩個",
			bodyDoubleQuote: '""',
			bodyEnd: "。",
			caseLabelBad: "錯誤寫法",
			caseLabelGood: "正確寫法",
			case1Bad: "SHOPEE1,蘋果,紅富士,1995,...",
			case1BadReason: "名稱含「,」卻沒用雙引號包，parser 會把它切成兩欄",
			case1Good: 'SHOPEE1,"蘋果,紅富士",1995,...',
			case2Bad: '...,<img src="cat.jpg" />,...',
			case2BadReason: "欄位內含雙引號卻未跳脫，parser 會誤判",
			case2Good: '...,"<img src=""cat.jpg"" />",...',
		},
		section3: {
			heading: "商品描述欄位特別提醒",
			bodyPrefix:
				"商品描述常含 HTML 標籤、emoji、逗號、雙引號等特殊字元，手動編輯極易出錯。",
			bodyEmphasis: "強烈建議用 Excel / Google Sheets 編輯描述欄位",
			bodySuffix: "，工具會自動處理跳脫規則。",
		},
		section4: {
			heading: "參考範本",
			bodyPrefix: "下載範本內已示範實際情境：商品 SHOPEE1 含內嵌 ",
			bodyCode: "<img>",
			bodyMiddle: " 跳脫、SHOPEE5 描述含全形「，」與換行。",
		},
	},
	csvInvalidBanner: {
		title: "CSV 內容有誤，請修正後重傳",
		retry: "重傳 CSV",
		toggleErrors: (n: number) => `${n} 筆錯誤明細`,
	},
	progress: {
		parsing: "解析中…",
		downloading: "圖片下載中",
		ready: "可預覽 / 上架",
		eligible: (n: number) => `${n} 筆已就緒`,
		needsFix: (n: number) => `${n} 筆需修正`,
		downloadingCount: (n: number) => `${n} 筆下載中`,
	},
	duplicate: {
		title: "商品名稱與既有商品重複",
		/** name / id / status 任一缺值時，落到 fallback 摘要句。 */
		description: (name: string, id: string, status: string) => {
			if (!name && !id) return "此名稱與您已建立的某個商品重複。";
			const statusPart = status ? `，狀態：${status}` : "";
			const idPart = id ? `（ID ${id}${statusPart}）` : "";
			return `此名稱與您既有商品「${name || "(未命名)"}」${idPart}重複。`;
		},
		checkboxLabel: "我確認允許與既有商品同名（買家可能會混淆）",
		viewExistingLink: "查看既有商品",
	},
	preview: {
		bannerListing: (filename: string) =>
			`以下為匯入預覽，需 Commit 後才會真正上架 · 本次匯入：${filename}`,
		bannerDetail: "預覽模式：以下為匯入商品的商城呈現，尚未上架",
		eligibleCount: (n: number) => `共 ${n} 筆可上架`,
		needsFixCount: (n: number) => `另有 ${n} 筆需修正後才會顯示在預覽`,
		downloadingCount: (n: number) => `${n} 筆圖片下載中`,
		searchDisabled: "預覽模式無法搜尋",
		backToManage: "返回管理頁",
		backToPreview: "返回預覽列表",
		previewBadge: "預覽",
		cartLockedHint: "預覽商品無法加入購物車或購買",
	},
	commit: {
		confirmTitle: "確定 Commit？",
		confirmDesc: (n: number) =>
			`Commit 是最終守門，將把 ${n} 筆商品建立為正式商品；目標上架狀態（直接上架 / 草稿）依各商品設定執行。`,
		confirmSelectedTitle: "確定 Commit 已勾選商品？",
		confirmSelectedDesc: (n: number) =>
			`將對 ${n} 筆已勾選商品執行 Commit（其餘商品不受影響）；不可上架者會自動跳過並回報原因。`,
		cancelConfirmTitle: "確定取消整個匯入任務？",
		cancelConfirmDesc:
			"取消後本次匯入的所有暫存編輯與圖片下載結果將永久遺失（不可復原）。儲存中的草稿不會影響既有正式商品。",
		success: (n: number) => `已送出 ${n} 筆商品 Commit（背景處理中）`,
		successWithSkipped: (committed: number, skipped: number) =>
			`已送出 ${committed} 筆 Commit，另跳過 ${skipped} 筆（詳見下方原因）`,
		skipReasonHeading: "跳過原因",
		alreadyDone: "任務已完成或正在處理",
	},
	shippingModal: {
		title: "批次設定運送",
		warning: "將完全覆寫所選商品全部 SKU 的原運送設定",
		freeThresholdLabel: "免運門檻（元）",
		freeThresholdHint:
			"留空 = 買家自付運費；填 0 或正整數 = 賣家補貼，達門檻免運",
		notOpened: "未開通，請先到「物流設定」開通此配送方式",
		atLeastOne: "至少需勾選一個配送方式",
	},
	categoryModal: {
		title: "批次套用商品類別",
		pathLabel: "選定分類路徑：",
		confirmLabel: (n: number) => `我已確認分類路徑正確，將套用至 ${n} 個商品`,
		tokenExpired: "分類確認碼已逾期，請重新選擇",
	},
	targetStatusModal: {
		title: "批次套用上架狀態",
		hint: "套用後本任務內所選商品的目標上架狀態會被覆寫。",
	},
	csvInvalidExpireHint: "提醒：CSV_INVALID 狀態的任務僅保留 3 天，請儘快重傳。",
	leaveEditConfirm: "未儲存的變更將遺失，確定離開？",
} as const;
