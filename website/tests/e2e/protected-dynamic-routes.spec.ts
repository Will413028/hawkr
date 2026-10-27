import {
	expect,
	type Page,
	type Request,
	type Route,
	test,
} from "@playwright/test";
import {
	DYNAMIC_ROUTE_CATALOG,
	type DynamicRouteAccess,
} from "./helpers/dynamic-route-catalog";
import {
	fulfillConnect,
	type MockStore,
	mockBuyerAddresses,
	mockMyKyc,
	mockMyStore,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { assertNoResidualLoading } from "./helpers/page-stability";
import { isKnownNonEnforcingConsoleError } from "./helpers/runtime-console";

const ACTIVE_ROLE_KEY = "hkui:auth:active-role";
const LOADER_KEY = "hawkr:initial-loader-seen";

const BUYER_ACCOUNT_ID = 300;
const SELLER_ID = "200";
const BUYER_ID = "300";
const POST_ID = "400";
const PRODUCT_ID = "500";
const JOB_ID = "600";
const ITEM_ID = "601";
const ORDER_NO = "HK-ROUTE-SMOKE";
const CASE_NO = "AS-ROUTE-SMOKE";
const COUPON_ID = "900";
// 58 號電子發票的四條動態路由（買家 B05 ＋ 賣家 S08／S10／S11）。
// `/seller-center/invoices/[documentId]` 與同層的 `documents`／`tax-status`／
// `appeals`／`jobs` 四個靜態段並存 ⇒ 這個 id 刻意不長得像那四個字串。
const INVOICE_DOCUMENT_ID = "INVDOC-ROUTE-SMOKE";
const SELLER_INVOICE_DOCUMENT_ID = "SELLERINVDOC-ROUTE-SMOKE";
const INVOICE_COMPLAINT_ID = "INVCMP-ROUTE-SMOKE";
const INVOICE_JOB_ID = "INVJOB-ROUTE-SMOKE";
const PRODUCT_DESCRIPTION_TEXT = {
	first: "第一段",
	second:
		"第二段這是一段刻意超過手機單行寬度的商品故事，用來確認區塊操作圖示永遠對齊目前段落的第一行，不會掉到整段文字下方。",
	third: "第三段",
} as const;

type UserRole = Exclude<DynamicRouteAccess, "public">;
type ApiMethod = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

interface ApiMock {
	method?: ApiMethod;
	path: string;
	data: unknown;
}

interface RequiredRequest {
	method: ApiMethod;
	path: string;
	query?: Record<string, string>;
}

interface SeenRequest {
	method: string;
	path: string;
	url: string;
}

interface DynamicRouteCase {
	id: string;
	pattern: string;
	role: UserRole;
	url: string;
	pathname: string;
	mocks: readonly ApiMock[];
	requiredRequests: readonly RequiredRequest[];
	/** Connect 的 mock（`/api/connect/*`）；REST 的 `mocks` 打 `/api/v1`。 */
	setup?(page: Page): Promise<void>;
	assertVisible(page: Page): Promise<void>;
}

// GetMyStore 的店家：沒有完成新手引導的店家會被當成半註冊帳號導去 onboarding，
// 這些路由守衛的斷言就整批測不到真正想測的東西。
const SELLER_STORE: MockStore = {
	id: SELLER_ID,
	storeName: "路由測試商店",
};

const PUBLIC_SELLER = {
	storeId: SELLER_ID,
	name: "路由測試商店",
	storeName: "路由測試商店",
	sellerName: "路由測試賣家",
	description: "路由測試商店介紹",
	coverKey: "",
	coverUrl: "",
};

const ORDER_FIXTURE = {
	orderNo: ORDER_NO,
	storeId: SELLER_ID,
	sellerName: "路由測試商店",
	buyerId: BUYER_ID,
	buyerName: "路由測試買家",
	status: "delivered",
	totalActualAmount: "120",
	totalOriginalAmount: "120",
	createdAt: 1767225600000,
	canReview: true,
	canRefund: true,
	items: [
		{
			orderItemId: "701",
			productId: PRODUCT_ID,
			skuId: "sku-1",
			productName: "路由測試青菜",
			unitPrice: "120",
			quantity: "1",
			reviewed: false,
		},
	],
	shipping: {
		receiverName: "路由測試收件人",
		receiverPhone: "0912345678",
		address: "臺北市中正區測試路1號",
		method: "3001",
		trackingNo: "TRACK-ROUTE-SMOKE",
	},
};

const AFTER_SALE_FIXTURE = {
	caseNo: CASE_NO,
	orderNo: ORDER_NO,
	buyerId: String(BUYER_ACCOUNT_ID),
	buyerName: "路由測試買家",
	sellerName: "路由測試商店",
	status: "0",
	reason: "5001",
	description: "路由測試售後說明",
	refundableAmount: "120",
	returnShipmentNo: "",
	returnShippingMethod: "",
	createdAt: "2026-01-01T00:00:00Z",
	items: [
		{
			orderItemId: "701",
			productName: "路由測試青菜",
			skuName: "一份",
			quantity: 1,
			unitPrice: "120",
		},
	],
	evidence: [],
	resolutions: [],
};

/**
 * 端點 69 `GET /app/invoices/documents/{document_id}`（B05 唯一資料源）。
 * 形狀照 `lib/invoices/buyer-repo.ts` 的 `fetchInvoiceDocument` 逐欄推導。
 */
const INVOICE_DOCUMENT_FIXTURE = {
	summary: {
		documentId: INVOICE_DOCUMENT_ID,
		documentKind: "B2C",
		status: "ISSUED",
		invoiceNumber: "AB-87654321",
		issuedAt: "2026-01-02T03:04:05Z",
		totalAmount: "120",
		carrierSummary: "手機條碼 /T****+P",
		relatedDocumentId: "",
		fileAvailable: false,
		fileUnavailableReason: "",
	},
	issuerName: "路由測試商店",
	issuerTaxId: "22223223",
	recipientName: "路由測試買家",
	recipientTaxId: "",
	salesAmount: "115",
	taxAmount: "5",
	totalAmount: "120",
	taxCategory: "1",
	lines: [
		{
			lineType: "PRODUCT",
			name: "路由測試青菜",
			quantity: "1",
			unitPrice: "120",
			netAmount: "120",
			taxCategory: "1",
		},
	],
	// `event` 的值域是 `InvoiceDocumentTimelineEvent`（5 值），不是 `InvoiceEventType`
	// ——查錯表只會整欄空白而不報錯，所以這裡刻意用會查到 label 的 `ISSUED`。
	timeline: [{ occurredAt: "2026-01-02T03:04:05Z", event: "ISSUED", note: "" }],
};

/**
 * 端點 31 `GET /app/seller/invoices/documents/{document_id}`（S11 唯一資料源）。
 *
 * ⚠️ 賣家側的 wire 把明細包在 `detail` 之下、賣家專屬欄位平放在外層
 * （`fetchSellerInvoiceDocument` 才攤平成一層）⇒ 這裡必須照 wire 的兩層形狀給，
 * 直接照買家那份平放會讓整個 `summary` 讀成空字串而畫面停在「找不到這份發票文件」。
 */
const SELLER_INVOICE_DOCUMENT_FIXTURE = {
	detail: {
		summary: {
			documentId: SELLER_INVOICE_DOCUMENT_ID,
			documentKind: "B2B",
			status: "ISSUED",
			invoiceNumber: "CD-13572468",
			issuedAt: "2026-01-03T05:06:07Z",
			totalAmount: "120",
			carrierSummary: "",
			relatedDocumentId: "",
			fileAvailable: false,
			fileUnavailableReason: "",
		},
		issuerName: "路由測試商店",
		issuerTaxId: "22223223",
		recipientName: "路由測試買家",
		recipientTaxId: "87654321",
		salesAmount: "115",
		taxAmount: "5",
		totalAmount: "120",
		taxCategory: "1",
		lines: [
			{
				lineType: "PRODUCT",
				name: "路由測試青菜",
				quantity: "1",
				unitPrice: "120",
				netAmount: "120",
				taxCategory: "1",
			},
		],
		timeline: [
			{
				occurredAt: "2026-01-03T05:06:07Z",
				event: "ISSUED",
				note: "已開立並寄送給買家",
			},
		],
	},
	createdBy: "SELLER",
	fundedBy: "SELLER",
	cutoverVersion: "",
	providerRequestId: "",
	manualReasonCode: "",
	nextResendAvailableAt: "",
	latestJobId: "",
};

/**
 * 端點 35 `GET /app/seller/invoices/complaints/{complaint_id}`（S10 唯一資料源）。
 *
 * `remainingBusinessDays` 是**後端給的數字字串**（前端零計算）⇒ 給 `"3"` 時倒數
 * 恆為「剩餘 3 個工作天」，斷言不會隨測試執行當下的時間漂移。
 */
const SELLER_INVOICE_COMPLAINT_FIXTURE = {
	complaintId: INVOICE_COMPLAINT_ID,
	// 後端恆為空字串 ⇒ 頁面退回顯示 complaintId，這裡照現況給空的。
	caseNo: "",
	orderNo: ORDER_NO,
	storeId: SELLER_ID,
	status: "OPEN",
	expectedDocKind: "",
	dueAt: "",
	remainingBusinessDays: "3",
	overdueBusinessDays: "",
	buyerMaskedName: "",
	description: "付款後三天還沒收到發票，麻煩協助確認。",
	responses: [
		{
			responseId: "inv-cmp-resp-1",
			actor: "ADMIN",
			action: "REMINDED",
			note: "平台已通知賣家處理。",
			occurredAt: "2026-01-04T01:02:03Z",
		},
	],
	version: "1",
	createdAt: "2026-01-03T01:02:03Z",
	reopenCount: "0",
	buyerReviewDueAt: "",
	reopenAllowedAfter: "",
};

/**
 * 端點 33 `GET /app/seller/invoices/jobs/{job_id}`（S08 唯一資料源）。
 *
 * `nextRetryAt` 刻意留空：`DeadlineCountdown` 讀不出期限時整個不渲染，避免這一頁
 * 出現一個隨執行日期改變的「剩餘 N 天」而讓斷言變成時間相依。
 */
const SELLER_INVOICE_JOB_FIXTURE = {
	jobId: INVOICE_JOB_ID,
	orderNo: ORDER_NO,
	storeId: SELLER_ID,
	operationType: "ISSUE",
	status: "SELLER_ACTION_REQUIRED",
	summary: "發票開立失敗：買受人統編格式不正確。",
	attemptCount: "3",
	maxAttempts: "5",
	lastAttemptAt: "2026-01-05T02:03:04Z",
	nextRetryAt: "",
	retryPolicy: {
		initialIntervalSeconds: "60",
		backoffMultiplier: "2",
		maxIntervalSeconds: "3600",
		maxAttempts: "5",
	},
	shipmentUnaffectedNotice: "發票異常不影響此訂單的出貨流程。",
	providerRequestId: "REQ-ROUTE-SMOKE",
	version: "1",
	manualReasonCode: "",
	externalInvoiceNo: "",
	externalInvoiceDate: "",
	reportedAt: "",
	canReportExternal: false,
};

const IMPORT_ITEM_BRIEF = {
	importItemId: ITEM_ID,
	productKey: "row-2",
	status: "9203",
	name: "路由測試青菜",
	categoryId: "301",
	catL3Name: "葉菜類",
	skuCount: "1",
	priceMin: "120",
	priceMax: "120",
	firstImageUrl: "",
	blockingErrorCount: "0",
	warningCount: "0",
	needsFix: false,
	hasDuplicateExisting: false,
	commitRevision: "0",
	version: "0",
};

const IMPORT_ITEM_DETAIL = {
	brief: IMPORT_ITEM_BRIEF,
	descriptionJson: '{"version":"1.0","blocks":[]}',
	skus: [
		{
			skuId: "sku-import-1",
			csvLine: "2",
			skuName: "一份",
			price: "120",
			salePrice: "",
			stockQty: "10",
			weight: "1",
			unit: "3301",
			lengthCm: "",
			widthCm: "",
			heightCm: "",
			shippingMethods: [],
		},
	],
	galleryAssets: [],
	descriptionAssets: [],
	errors: [],
	warnings: [],
	commitRevision: "0",
	origin: "台灣",
	hashtags: ["路由測試"],
	certifications: [],
	targetStatus: "DRAFT",
};

const IMPORT_JOB_DETAIL = {
	importJobId: JOB_ID,
	status: "9104",
	sourceFilename: "route-smoke.csv",
	totalRows: "1",
	totalItems: "1",
	targetStatus: "DRAFT",
	createdAt: "2026-01-01T00:00:00Z",
	updatedAt: "2026-01-01T00:00:00Z",
	expiresAt: "2027-01-01T00:00:00Z",
	validationErrors: [],
	counters: {
		itemsTotal: "1",
		itemsParsed: "1",
		itemsAssetDownloading: "0",
		itemsReady: "1",
		itemsNeedsFix: "0",
		itemsRemoved: "0",
		itemsCommitting: "0",
		itemsCommitted: "0",
		itemsCommitFailed: "0",
		itemsWithWarnings: "0",
		itemsDuplicateExisting: "0",
		assetsTotal: "0",
		assetsPending: "0",
		assetsDownloading: "0",
		assetsUploaded: "0",
		assetsFailed: "0",
		assetsLinked: "0",
	},
	progress: {
		overallPercentage: "100",
		parsePercentage: "100",
		assetDownloadPercentage: "100",
		commitPercentage: "0",
		parseCompleted: true,
		assetDownloadCompleted: true,
		commitCompleted: false,
	},
};

const IMPORT_PREVIEW_LISTING = {
	importJobId: JOB_ID,
	totalEligible: "1",
	totalNeedsFix: "0",
	totalPending: "0",
	totalRemoved: "0",
	total: "1",
	page: "1",
	pageSize: "24",
	records: [
		{
			importItemId: ITEM_ID,
			importStatus: "9203",
			previewData: {
				id: ITEM_ID,
				name: "路由測試青菜",
				rating: "0",
				categoryId: "301",
				categoryName: "葉菜類",
				origin: "台灣",
				gallery: [],
				priceMin: "120",
				priceMax: "120",
				reviewCount: "0",
				skuCount: "1",
				imageUrl: "",
			},
		},
	],
};

const IMPORT_PREVIEW_DETAIL = {
	importJobId: JOB_ID,
	importItemId: ITEM_ID,
	importStatus: "9203",
	validationErrors: [],
	warnings: [],
	previewData: {
		id: ITEM_ID,
		name: "路由測試青菜",
		rating: "0",
		status: "0",
		categoryId: "301",
		origin: "台灣",
		description: '{"version":"1.0","blocks":[]}',
		gallery: [],
		certifications: [],
		priceMin: "120",
		priceMax: "120",
		skus: [
			{
				skuId: "sku-import-1",
				skuName: "一份",
				price: "120",
				salePrice: "",
				stockQty: "10",
				weight: "1",
				unit: "3301",
				lengthCm: "",
				widthCm: "",
				heightCm: "",
				shippingMethods: [],
			},
		],
		catL1Id: "3",
		catL1Name: "生鮮",
		catL2Id: "30",
		catL2Name: "蔬菜",
		catL3Id: "301",
		catL3Name: "葉菜類",
	},
};

const CHAT_BACKGROUND_MOCKS: readonly ApiMock[] = [
	{ path: "/app/chat/unread", data: { unread: 0 } },
	{ method: "POST", path: "/app/chat/ws-ticket", data: {} },
];

const EMPTY_BUYER_AFTER_SALES: ApiMock = {
	path: "/app/after_sales",
	data: { records: [], total: 0 },
};

function chatMocks(targetId: string, targetNickname: string): ApiMock[] {
	return [
		{
			path: "/app/chat/conversations",
			data: {
				records: [
					{
						targetUserId: targetId,
						targetNickname,
						lastMessage: "",
						lastMessageTime: 0,
						unreadCount: 0,
					},
				],
			},
		},
		{
			path: `/app/chat/history/${targetId}`,
			data: { messages: [] },
		},
		{
			method: "PATCH",
			path: `/app/chat/conversations/${targetId}`,
			data: {},
		},
		...CHAT_BACKGROUND_MOCKS,
	];
}

async function assertHeading(page: Page, name: string | RegExp) {
	await expect(page.getByRole("heading", { name }).first()).toBeVisible();
}

async function assertText(page: Page, text: string | RegExp) {
	await expect(
		page.getByText(text, { exact: typeof text === "string" }).first(),
	).toBeVisible();
}

const ROUTE_CASES: readonly DynamicRouteCase[] = [
	{
		id: "buyer-address-edit",
		pattern: "/account/addresses/[id]/edit",
		role: "buyer",
		url: "/account/addresses/101/edit",
		pathname: "/account/addresses/101/edit",
		mocks: [],
		async setup(page) {
			await mockBuyerAddresses(page, [
				{
					id: "101",
					receiverName: "路由測試收件人",
					receiverPhone: "0912345678",
					townCode: "63000050",
					addressDetail: "測試路1號",
					defaultAddress: true,
				},
			]);
		},
		requiredRequests: [],
		async assertVisible(page) {
			await assertHeading(page, "編輯地址");
			await expect(page.getByLabel("收件人姓名")).toHaveValue("路由測試收件人");
		},
	},
	{
		id: "buyer-after-sale-detail",
		pattern: "/account/after-sales/[caseNo]",
		role: "buyer",
		url: `/account/after-sales/${CASE_NO}`,
		pathname: `/account/after-sales/${CASE_NO}`,
		mocks: [
			{
				path: `/app/after_sales/${CASE_NO}`,
				data: AFTER_SALE_FIXTURE,
			},
		],
		requiredRequests: [{ method: "GET", path: `/app/after_sales/${CASE_NO}` }],
		async assertVisible(page) {
			await assertHeading(page, "售後案件");
			await assertText(page, new RegExp(`#${CASE_NO}`));
			await assertHeading(page, "申請內容");
			await assertText(page, "路由測試青菜");
		},
	},
	{
		id: "buyer-invoice-document",
		pattern: "/account/invoices/[documentId]",
		role: "buyer",
		url: `/account/invoices/${INVOICE_DOCUMENT_ID}`,
		pathname: `/account/invoices/${INVOICE_DOCUMENT_ID}`,
		mocks: [
			{
				path: `/app/invoices/documents/${INVOICE_DOCUMENT_ID}`,
				data: INVOICE_DOCUMENT_FIXTURE,
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/invoices/documents/${INVOICE_DOCUMENT_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "發票與收據");
			// 發票號碼是這一頁的識別欄，也是買家對獎／報帳的唯一憑據：斷言它才分得出
			// 「渲染了正確那份文件」與「渲染了殼」。
			await assertText(page, "AB-87654321");
			await assertHeading(page, "文件資訊");
			await assertText(page, "二聯式（買受人為個人）");
			await assertHeading(page, "金額明細");
			await assertText(page, "路由測試青菜");
			await assertHeading(page, "文件歷程");
		},
	},
	{
		id: "buyer-message-thread",
		pattern: "/account/messages/[targetId]",
		role: "buyer",
		url: `/account/messages/${SELLER_ID}`,
		pathname: `/account/messages/${SELLER_ID}`,
		mocks: [
			...chatMocks(SELLER_ID, "路由測試賣家"),
			{ path: `/public/sellers/${SELLER_ID}`, data: PUBLIC_SELLER },
		],
		requiredRequests: [
			{ method: "GET", path: "/app/chat/conversations" },
			{ method: "GET", path: `/app/chat/history/${SELLER_ID}` },
			{
				method: "PATCH",
				path: `/app/chat/conversations/${SELLER_ID}`,
			},
		],
		async assertVisible(page) {
			const thread = page.getByRole("region", { name: "對話內容" });
			await expect(thread).toBeVisible();
			await expect(
				thread.getByText("路由測試賣家", { exact: true }).first(),
			).toBeVisible();
			await expect(
				thread.getByText("打個招呼，開啟與路由測試賣家的對話吧。", {
					exact: true,
				}),
			).toBeVisible();
		},
	},
	{
		id: "buyer-procurement-assistant-demo",
		pattern: "/account/messages/assistant/[conversationId]",
		role: "buyer",
		url: "/account/messages/assistant/demo",
		pathname: "/account/messages/assistant/demo",
		mocks: [
			{
				path: "/app/chat/conversations",
				data: { conversations: [] },
			},
			...CHAT_BACKGROUND_MOCKS,
		],
		requiredRequests: [{ method: "GET", path: "/app/chat/conversations" }],
		async assertVisible(page) {
			// initialRequestId is null for this route (page.tsx doesn't pass one
			// for conversationId="demo"), so session.restoring starts false and
			// the workspace renders straight into the "clarifying" stage — no
			// procurement backend mocks needed for this smoke check.
			const workspace = page.locator('[data-assistant-mode="live"]');
			await expect(workspace).toBeVisible();
			await expect(
				workspace.getByText("Hawkr Agent", { exact: true }).first(),
			).toBeVisible();
			await expect(
				workspace.locator(":scope > .thread-page__head"),
			).toBeVisible();
			await expect(
				workspace.locator(":scope > .thread-page__body"),
			).toBeVisible();
			await assertText(
				page,
				"告訴我你想採購什麼——品項、數量、預算、配送城市與到貨日，想到什麼先說什麼。",
			);

			const composeInput = page.getByLabel("採購需求輸入");
			await expect(composeInput).toBeVisible();
			await expect(composeInput).toBeEnabled();
		},
	},
	{
		id: "buyer-order-detail",
		pattern: "/account/orders/[orderNo]",
		role: "buyer",
		url: `/account/orders/${ORDER_NO}`,
		pathname: `/account/orders/${ORDER_NO}`,
		mocks: [
			{ path: `/app/orders/${ORDER_NO}`, data: ORDER_FIXTURE },
			EMPTY_BUYER_AFTER_SALES,
		],
		requiredRequests: [
			{ method: "GET", path: `/app/orders/${ORDER_NO}` },
			{
				method: "GET",
				path: "/app/after_sales",
				query: { page: "1", page_size: "100" },
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "訂單詳情");
			await assertText(page, new RegExp(`#${ORDER_NO}`));
			await assertHeading(page, "商品（1）");
			await assertText(page, "路由測試青菜");
		},
	},
	{
		id: "buyer-order-return",
		pattern: "/account/orders/[orderNo]/return",
		role: "buyer",
		url: `/account/orders/${ORDER_NO}/return`,
		pathname: `/account/orders/${ORDER_NO}/return`,
		mocks: [{ path: `/app/orders/${ORDER_NO}`, data: ORDER_FIXTURE }],
		requiredRequests: [{ method: "GET", path: `/app/orders/${ORDER_NO}` }],
		async assertVisible(page) {
			await assertHeading(page, "申請退換貨");
			await assertHeading(page, "訂單摘要");
			await assertText(page, "路由測試青菜");
		},
	},
	{
		id: "buyer-order-review",
		pattern: "/account/orders/[orderNo]/review",
		role: "buyer",
		url: `/account/orders/${ORDER_NO}/review`,
		pathname: `/account/orders/${ORDER_NO}/review`,
		mocks: [{ path: `/app/orders/${ORDER_NO}`, data: ORDER_FIXTURE }],
		requiredRequests: [{ method: "GET", path: `/app/orders/${ORDER_NO}` }],
		async assertVisible(page) {
			await assertHeading(page, "評價商品");
			await assertText(page, "路由測試青菜");
			await expect(
				page.getByRole("radiogroup", { name: "評分" }),
			).toBeVisible();
		},
	},
	{
		id: "buyer-order-tracking",
		pattern: "/account/orders/[orderNo]/tracking",
		role: "buyer",
		url: `/account/orders/${ORDER_NO}/tracking`,
		pathname: `/account/orders/${ORDER_NO}/tracking`,
		mocks: [
			{ path: `/app/orders/${ORDER_NO}`, data: ORDER_FIXTURE },
			{
				path: `/app/orders/${ORDER_NO}/logistics`,
				data: {
					orderNo: ORDER_NO,
					shipTradeNo: "TRACK-ROUTE-SMOKE",
					shippingMethod: "3001",
					status: "IN_TRANSIT",
					statusLabel: "配送中",
					statusTime: "2026-01-02T03:04:05Z",
					recipientName: "路由測試收件人",
				},
			},
		],
		requiredRequests: [
			{ method: "GET", path: `/app/orders/${ORDER_NO}` },
			{
				method: "GET",
				path: `/app/orders/${ORDER_NO}/logistics`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "物流追蹤");
			await assertText(page, /託運單號 TRACK-ROUTE-SMOKE/);
			await assertText(page, "配送中");
		},
	},
	{
		id: "buyer-wallet-detail",
		pattern: "/account/wallet/[sellerId]",
		role: "buyer",
		url: `/account/wallet/${SELLER_ID}?name=${encodeURIComponent("路由測試商店")}`,
		pathname: `/account/wallet/${SELLER_ID}`,
		mocks: [],
		async setup(page) {
			await page.route(
				/\/api\/connect\/hawkr\.buyer\.v1alpha1\.BuyerAppService\/ListSpendingRecords(?:\?|$)/,
				(route) =>
					fulfillConnect(route, {
						records: [
							{
								orderNumber: ORDER_NO,
								amount: "120",
								refundedAmount: "0",
								couponApplied: false,
								orderTime: "2026-01-01T00:00:00Z",
							},
						],
						nextPageToken: "",
						totalSpent: "120",
						totalRefunded: "0",
					}),
			);
		},
		requiredRequests: [],
		async assertVisible(page) {
			await assertHeading(page, "路由測試商店 · 會員卡");
			await assertText(page, `訂單 ${ORDER_NO}`);
			await assertText(page, "NT$ 120");
		},
	},
	{
		id: "seller-coupon-edit",
		pattern: "/seller-center/coupons/[id]/edit",
		role: "seller",
		url: `/seller-center/coupons/${COUPON_ID}/edit`,
		pathname: `/seller-center/coupons/${COUPON_ID}/edit`,
		mocks: [
			{
				path: `/app/coupon_templates/${COUPON_ID}`,
				data: {
					template: {
						id: COUPON_ID,
						templateId: COUPON_ID,
						title: "路由測試優惠券",
						type: "1",
						discountAmount: 50,
						thresholdAmount: 500,
						targetType: "1",
						totalIssueLimit: 100,
						issuedCount: 0,
						startAt: "2026-01-01T00:00:00Z",
						endAt: "2027-01-01T00:00:00Z",
					},
				},
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/coupon_templates/${COUPON_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "檢視優惠券");
			const nameInput = page.getByLabel("優惠券名稱");
			await expect(nameInput).toHaveValue("路由測試優惠券");
			await expect(nameInput).toBeDisabled();
		},
	},
	{
		id: "seller-customer-detail",
		pattern: "/seller-center/customers/[buyerId]",
		role: "seller",
		url: `/seller-center/customers/${BUYER_ID}`,
		pathname: `/seller-center/customers/${BUYER_ID}`,
		mocks: [
			{
				path: `/app/sellers/me/customers/${BUYER_ID}`,
				data: {
					buyerId: BUYER_ID,
					nickname: "路由測試買家",
					phone: "0912345678",
					email: "buyer@example.test",
					address: "臺北市中正區測試路1號",
					orderCount: 0,
					totalSpent: "0",
					couponCount: 0,
					customerType: "normal",
					tags: [],
				},
			},
			{
				path: "/app/orders",
				data: { records: [], total: 0, page: 1, pageSize: 100 },
			},
			{
				path: `/app/sellers/me/customers/${BUYER_ID}/tags`,
				data: [],
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/sellers/me/customers/${BUYER_ID}`,
			},
			{
				method: "GET",
				path: "/app/orders",
				query: { view: "seller", buyer_id: BUYER_ID },
			},
			{
				method: "GET",
				path: `/app/sellers/me/customers/${BUYER_ID}/tags`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "客戶詳情");
			await assertText(page, "路由測試買家");
			await assertHeading(page, "購買紀錄");
			await assertText(page, "尚無購買紀錄");
		},
	},
	{
		id: "seller-invoice-document",
		pattern: "/seller-center/invoices/[documentId]",
		role: "seller",
		url: `/seller-center/invoices/${SELLER_INVOICE_DOCUMENT_ID}`,
		pathname: `/seller-center/invoices/${SELLER_INVOICE_DOCUMENT_ID}`,
		mocks: [
			{
				path: `/app/seller/invoices/documents/${SELLER_INVOICE_DOCUMENT_ID}`,
				data: SELLER_INVOICE_DOCUMENT_FIXTURE,
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/seller/invoices/documents/${SELLER_INVOICE_DOCUMENT_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "發票詳情");
			await assertHeading(page, "文件資訊");
			await assertText(page, "CD-13572468");
			// 沒有 `?kind=received` ⇒ 這一頁必須標成「銷項」。標錯方向會讓賣家以為
			// 自己可以作廢平台開給他的發票，所以這一格是有鑑別力的斷言而非裝飾。
			await assertText(page, "銷項");
			await assertHeading(page, "金額與稅額明細");
			await assertText(page, "87654321");
			await assertHeading(page, "處理歷程");
		},
	},
	{
		id: "seller-invoice-appeal-detail",
		pattern: "/seller-center/invoices/appeals/[complaintId]",
		role: "seller",
		url: `/seller-center/invoices/appeals/${INVOICE_COMPLAINT_ID}`,
		pathname: `/seller-center/invoices/appeals/${INVOICE_COMPLAINT_ID}`,
		mocks: [
			{
				path: `/app/seller/invoices/complaints/${INVOICE_COMPLAINT_ID}`,
				data: SELLER_INVOICE_COMPLAINT_FIXTURE,
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/seller/invoices/complaints/${INVOICE_COMPLAINT_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "申訴詳情");
			// 案件狀態（CodeMeta 的 7 個原值之一）＋ 工作天倒數：這兩格是賣家判斷
			// 「該不該現在處理」的依據，也是這一頁唯一會逾期計入違規的資訊。
			await assertText(page, "待賣家處理");
			await assertText(page, "剩餘 3 個工作天");
			await assertHeading(page, "申訴內容");
			await assertText(page, "付款後三天還沒收到發票，麻煩協助確認。");
			await assertHeading(page, "處理紀錄");
			await assertText(page, "平台已提醒賣家");
		},
	},
	{
		id: "seller-invoice-job",
		pattern: "/seller-center/invoices/jobs/[jobId]",
		role: "seller",
		url: `/seller-center/invoices/jobs/${INVOICE_JOB_ID}`,
		pathname: `/seller-center/invoices/jobs/${INVOICE_JOB_ID}`,
		mocks: [
			{
				path: `/app/seller/invoices/jobs/${INVOICE_JOB_ID}`,
				data: SELLER_INVOICE_JOB_FIXTURE,
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/seller/invoices/jobs/${INVOICE_JOB_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "發票異常詳情");
			// 摘要句由後端保證「可理解、不含 provider code」⇒ 它就是這一頁的標題行。
			await assertHeading(page, "發票開立失敗：買受人統編格式不正確。");
			await assertText(page, "待賣家處理");
			await assertHeading(page, "系統重試進度");
			// 「已嘗試 N／M 次」的 M 取那一列建立當下的 `maxAttempts`，不是政策現值；
			// 混用會讓「還剩幾次」算錯，所以連字串一起釘死。
			await assertText(page, "3 / 5 次");
			await assertText(page, "開立");
		},
	},
	{
		id: "seller-message-thread",
		pattern: "/seller-center/messages/[targetId]",
		role: "seller",
		url: `/seller-center/messages/${BUYER_ID}`,
		pathname: `/seller-center/messages/${BUYER_ID}`,
		mocks: chatMocks(BUYER_ID, "路由測試買家"),
		requiredRequests: [
			{ method: "GET", path: "/app/chat/conversations" },
			{ method: "GET", path: `/app/chat/history/${BUYER_ID}` },
			{
				method: "PATCH",
				path: `/app/chat/conversations/${BUYER_ID}`,
			},
		],
		async assertVisible(page) {
			const thread = page.getByRole("region", { name: "對話內容" });
			await expect(thread).toBeVisible();
			await expect(
				thread.getByText("路由測試買家", { exact: true }).first(),
			).toBeVisible();
			await expect(
				thread.getByText("這裡是與路由測試買家對話的起點。", {
					exact: true,
				}),
			).toBeVisible();
		},
	},
	{
		id: "seller-order-detail",
		pattern: "/seller-center/orders/[orderNo]",
		role: "seller",
		url: `/seller-center/orders/${ORDER_NO}`,
		pathname: `/seller-center/orders/${ORDER_NO}`,
		mocks: [
			{ path: `/app/orders/${ORDER_NO}`, data: ORDER_FIXTURE },
			{
				path: "/app/seller/after_sales",
				data: { records: [], total: 0 },
			},
			{
				path: "/app/shipments/anomalies",
				data: { records: [], total: 0, page: 1, pageSize: 100 },
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/orders/${ORDER_NO}`,
				query: { view: "seller" },
			},
			{ method: "GET", path: "/app/seller/after_sales" },
			{ method: "GET", path: "/app/shipments/anomalies" },
		],
		async assertVisible(page) {
			await assertHeading(page, `訂單 #${ORDER_NO}`);
			await assertHeading(page, "商品（1 件）");
			await assertText(page, "路由測試青菜");
			await assertText(page, "路由測試收件人");
		},
	},
	{
		id: "seller-post-edit",
		pattern: "/seller-center/posts/[id]/edit",
		role: "seller",
		url: `/seller-center/posts/${POST_ID}/edit`,
		pathname: `/seller-center/posts/${POST_ID}/edit`,
		mocks: [
			{
				// 編輯頁讀賣家自己的單篇（任何發布狀態），不是公開的 /public/posts。
				path: `/app/sellers/posts/${POST_ID}`,
				data: {
					id: POST_ID,
					content: "路由測試貼文內容",
					images: [],
					products: [],
					tags: [],
				},
			},
			{
				path: `/public/sellers/${SELLER_ID}/products`,
				data: { records: [], total: 0, totalPages: 0 },
			},
		],
		requiredRequests: [
			{ method: "GET", path: `/app/sellers/posts/${POST_ID}` },
			{
				method: "GET",
				path: `/public/sellers/${SELLER_ID}/products`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "編輯貼文");
			await expect(page.getByLabel("貼文內容")).toHaveValue("路由測試貼文內容");
		},
	},
	{
		id: "seller-product-edit",
		pattern: "/seller-center/products/[id]/edit",
		role: "seller",
		url: `/seller-center/products/${PRODUCT_ID}/edit`,
		pathname: `/seller-center/products/${PRODUCT_ID}/edit`,
		mocks: [
			{
				path: `/app/products/${PRODUCT_ID}`,
				data: {
					id: PRODUCT_ID,
					name: "路由測試青菜",
					description: {
						version: "1.0",
						blocks: [
							{
								id: "description-first",
								type: "text",
								runs: [{ text: PRODUCT_DESCRIPTION_TEXT.first }],
							},
							{
								id: "description-second",
								type: "text",
								runs: [{ text: PRODUCT_DESCRIPTION_TEXT.second }],
							},
							{
								id: "description-third",
								type: "text",
								runs: [{ text: PRODUCT_DESCRIPTION_TEXT.third }],
							},
						],
					},
					images: [],
					categoryId: 301,
					categoryName: "葉菜類",
					tags: ["路由測試"],
					deliveryTemp: [],
					origin: "台灣",
					certifications: [],
					status: 0,
					skus: [
						{
							id: "sku-1",
							skuName: "一份",
							price: "120",
							salePrice: "",
							stockQty: 10,
							unit: "3301",
							shippingMethods: [],
						},
					],
				},
			},
		],
		requiredRequests: [{ method: "GET", path: `/app/products/${PRODUCT_ID}` }],
		async assertVisible(page) {
			await assertHeading(page, "編輯商品");
			await expect(page.locator("#product-name")).toHaveValue("路由測試青菜");
		},
	},
	{
		id: "seller-import-job",
		pattern: "/seller-center/products/import/[job_id]",
		role: "seller",
		url: `/seller-center/products/import/${JOB_ID}`,
		pathname: `/seller-center/products/import/${JOB_ID}`,
		mocks: [
			{
				path: `/app/product_import_jobs/${JOB_ID}`,
				data: IMPORT_JOB_DETAIL,
			},
			{
				path: `/app/product_import_jobs/${JOB_ID}/items`,
				data: {
					records: [IMPORT_ITEM_BRIEF],
					total: "1",
					page: "1",
					pageSize: "50",
				},
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}`,
			},
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}/items`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, /route-smoke\.csv/);
			await assertText(page, "路由測試青菜");
			await expect(
				page.getByRole("checkbox", { name: "選取 路由測試青菜" }),
			).toBeVisible();
		},
	},
	{
		id: "seller-import-item-edit",
		pattern: "/seller-center/products/import/[job_id]/items/[item_id]/edit",
		role: "seller",
		url: `/seller-center/products/import/${JOB_ID}/items/${ITEM_ID}/edit`,
		pathname: `/seller-center/products/import/${JOB_ID}/items/${ITEM_ID}/edit`,
		mocks: [
			{
				path: `/app/product_import_jobs/${JOB_ID}/items/${ITEM_ID}`,
				data: IMPORT_ITEM_DETAIL,
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}/items/${ITEM_ID}`,
			},
		],
		async assertVisible(page) {
			await assertHeading(page, "編輯匯入商品");
			await expect(page.locator("#import-product-name")).toHaveValue(
				"路由測試青菜",
			);
			await assertHeading(page, "目標上架狀態");
		},
	},
	{
		id: "seller-import-preview-list",
		pattern: "/seller-product-import-preview/[job_id]",
		role: "seller",
		url: `/seller-product-import-preview/${JOB_ID}`,
		pathname: `/seller-product-import-preview/${JOB_ID}`,
		mocks: [
			{
				path: `/app/product_import_jobs/${JOB_ID}`,
				data: IMPORT_JOB_DETAIL,
			},
			{
				path: `/app/product_import_jobs/${JOB_ID}/preview_listing`,
				data: IMPORT_PREVIEW_LISTING,
			},
			{ path: `/public/sellers/${SELLER_ID}`, data: PUBLIC_SELLER },
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}`,
			},
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}/preview_listing`,
			},
		],
		async assertVisible(page) {
			await assertText(page, "列表預覽（完整商城 layout）");
			await assertText(page, "路由測試青菜");
			// MarketProductCard 改成單一 full-card overlay link（見
			// bf573afa），accessible name 變成 `查看商品：${name}`，不再是
			// 裸商品名 — 改用該次改動就加的穩定 hook 屬性選取。
			const previewLink = page.locator("[data-market-product-primary-link]");
			await expect(previewLink).toBeVisible();
			await expect(previewLink).toHaveAttribute(
				"href",
				`/seller-product-import-preview/${JOB_ID}/items/${ITEM_ID}`,
			);
		},
	},
	{
		id: "seller-import-preview-detail",
		pattern: "/seller-product-import-preview/[job_id]/items/[item_id]",
		role: "seller",
		url: `/seller-product-import-preview/${JOB_ID}/items/${ITEM_ID}`,
		pathname: `/seller-product-import-preview/${JOB_ID}/items/${ITEM_ID}`,
		mocks: [
			{
				path: `/app/product_import_jobs/${JOB_ID}/items/${ITEM_ID}/preview`,
				data: IMPORT_PREVIEW_DETAIL,
			},
			{ path: `/public/sellers/${SELLER_ID}`, data: PUBLIC_SELLER },
			{
				path: `/public/sellers/${SELLER_ID}/reviews/stats`,
				data: { averageRating: "0", reviewCount: "0" },
			},
		],
		requiredRequests: [
			{
				method: "GET",
				path: `/app/product_import_jobs/${JOB_ID}/items/${ITEM_ID}/preview`,
			},
		],
		async assertVisible(page) {
			await assertText(page, "商品詳情預覽（完整 PDP layout）");
			await assertHeading(page, "路由測試青菜");
			await expect(
				page
					.locator(`a[href="/seller-product-import-preview/${JOB_ID}"]`)
					.filter({ hasText: "返回列表預覽" }),
			).toBeVisible();
		},
	},
];

const SSR_PREVIEW_PATTERNS = new Set([
	"/seller-center/preview/[id]",
	"/seller-center/preview/[id]/messages",
	"/seller-center/preview/[id]/posts",
	"/seller-center/preview/[id]/reviews",
]);

const ERROR_SHELL =
	/Application error|Internal Server Error|NEXT_NOT_FOUND|This page could not be found|糟糕，火箭偏離了航道|暫時迷航|這頁不在貨架上|找不到訂單|找不到售後案件|找不到任務|找不到預覽商品|載入動態失敗/i;

function apiPath(url: string): string | null {
	try {
		const pathname = new URL(url).pathname;
		const marker = "/api/v1";
		const index = pathname.indexOf(marker);
		return index >= 0 ? pathname.slice(index + marker.length) || "/" : null;
	} catch {
		return null;
	}
}

function queryMatches(url: string, expected?: Record<string, string>): boolean {
	if (!expected) return true;
	const params = new URL(url).searchParams;
	return Object.entries(expected).every(
		([key, value]) => params.get(key) === value,
	);
}

function requestMatches(seen: SeenRequest, expected: RequiredRequest): boolean {
	return (
		seen.method === expected.method &&
		seen.path === expected.path &&
		queryMatches(seen.url, expected.query)
	);
}

function recordApiRequests(page: Page): SeenRequest[] {
	const seen: SeenRequest[] = [];
	page.on("request", (request: Request) => {
		const path = apiPath(request.url());
		if (!path) return;
		seen.push({ method: request.method(), path, url: request.url() });
	});
	return seen;
}

async function installApiMocks(page: Page, mocks: readonly ApiMock[]) {
	await page.route("**/api/v1/**", async (route: Route) => {
		const request = route.request();
		const path = apiPath(request.url());
		const match = mocks.find(
			(candidate) =>
				candidate.path === path &&
				(candidate.method ?? "GET") === request.method(),
		);
		if (!match) {
			await route.fallback();
			return;
		}
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: match.data }),
		});
	});
}

async function prepareAuthenticatedPage(
	page: Page,
	baseURL: string,
	role: UserRole,
	mocks: readonly ApiMock[],
) {
	// Lowest-priority safety net for unrelated chrome/background queries.
	await page.route("**/api/v1/**", (route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		}),
	);
	await setupGatewayMocks(page, {
		identity: {
			accountId: role === "buyer" ? String(BUYER_ACCOUNT_ID) : SELLER_ID,
			roles: role === "buyer" ? ["buyer"] : ["seller"],
		},
		session: "identity",
	});
	await mockMyStore(page, SELLER_STORE);
	await mockMyKyc(page, { status: "APPROVED" });
	// Route-specific dispatcher is installed last, so it wins over the generic
	// /app/** gateway handler while still falling through for unmatched paths.
	await installApiMocks(page, mocks);

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await page.addInitScript(
		([roleKey, loaderKey, activeRole]) => {
			localStorage.setItem(roleKey, activeRole);
			sessionStorage.setItem(loaderKey, "1");
		},
		[ACTIVE_ROLE_KEY, LOADER_KEY, role] as const,
	);
}

function attachRuntimeMonitors(page: Page, baseURL: string): string[] {
	const issues: string[] = [];
	const appOrigin = new URL(baseURL).origin;
	const isOwnedRequest = (url: string) => {
		try {
			const parsed = new URL(url);
			return parsed.origin === appOrigin || apiPath(url) !== null;
		} catch {
			return false;
		}
	};

	page.on("pageerror", (error) => {
		issues.push(`pageerror: ${error.message}`);
	});
	page.on("console", (message) => {
		// Google Pay reports this browser diagnostic without enforcing it; keep
		// every other console error visible to the smoke test.
		if (
			message.type() === "error" &&
			!isKnownNonEnforcingConsoleError(message.text())
		) {
			issues.push(`console.error: ${message.text()}`);
		}
	});
	page.on("requestfailed", (request) => {
		const failure = request.failure()?.errorText ?? "unknown failure";
		if (/net::ERR_ABORTED|cancelled/i.test(failure)) {
			return;
		}
		if (isOwnedRequest(request.url())) {
			issues.push(
				`requestfailed: ${request.method()} ${request.url()} (${failure})`,
			);
		}
	});
	page.on("response", (response) => {
		if (response.status() < 400 || !isOwnedRequest(response.url())) return;
		issues.push(
			`HTTP ${response.status()} ${response.request().method()} ${response.url()}`,
		);
	});
	return issues;
}

async function assertNoInvalidInternalLinks(page: Page, route: string) {
	const invalid = await page.locator("a[href]").evaluateAll((anchors) => {
		const failures: string[] = [];
		for (const anchor of anchors) {
			const href = anchor.getAttribute("href")?.trim() ?? "";
			if (/^(?:javascript|data|vbscript):/i.test(href)) {
				failures.push(href);
				continue;
			}
			// External, hash-only, mail and phone links are outside this route
			// contract. Internal links must be concrete, safe app paths.
			if (!href.startsWith("/") || href.startsWith("//")) continue;
			if (
				/\[[^\]]+\]/.test(href) ||
				/(^|[/=?&])(undefined|null)(?=$|[/#?&=])/i.test(href)
			) {
				failures.push(href);
			}
		}
		return [...new Set(failures)].sort();
	});
	expect(invalid, `${route} rendered invalid internal links`).toEqual([]);
}

async function assertRequiredRequests(
	seen: SeenRequest[],
	routeCase: DynamicRouteCase,
) {
	await expect
		.poll(
			() =>
				routeCase.requiredRequests.filter(
					(required) =>
						!seen.some((request) => requestMatches(request, required)),
				),
			{
				message: `${routeCase.url} did not consume every route-specific API fixture`,
				timeout: 10_000,
			},
		)
		.toEqual([]);
}

async function runDynamicRouteSmoke(
	page: Page,
	baseURL: string,
	routeCase: DynamicRouteCase,
) {
	const seen = recordApiRequests(page);
	const runtimeIssues = attachRuntimeMonitors(page, baseURL);
	await prepareAuthenticatedPage(
		page,
		baseURL,
		routeCase.role,
		routeCase.mocks,
	);
	await routeCase.setup?.(page);

	const response = await page.goto(routeCase.url, {
		waitUntil: "domcontentloaded",
		timeout: 25_000,
	});
	expect(
		response,
		`${routeCase.url} returned no document response`,
	).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${routeCase.url} document response must be successful`,
	).toBeLessThan(400);

	await expect(
		page.locator('[data-auth-hydrated="true"]'),
		`${routeCase.url} never hydrated its authenticated shell`,
	).toBeAttached({ timeout: 15_000 });
	await expect
		.poll(() => new URL(page.url()).pathname, {
			message: `${routeCase.url} redirected away from its dynamic route`,
		})
		.toBe(routeCase.pathname);

	await routeCase.assertVisible(page);
	await assertRequiredRequests(seen, routeCase);
	// CategorySelector replaces its initial aria-busy tree in the same React
	// commit that reveals the populated form. Observe the settled state through
	// the shared polling gate instead of taking a one-frame DOM snapshot that can
	// catch React between those mutations and report a false flaky failure.
	await assertNoResidualLoading(page, routeCase.url);
	await assertNoInvalidInternalLinks(page, routeCase.url);

	// Let lazy chunks and background client queries surface post-hydration
	// failures before the runtime ledger is evaluated.
	await page.waitForTimeout(350);
	const mainText = await page.locator("main").allInnerTexts();
	expect(
		mainText.join("\n"),
		`${routeCase.url} rendered an error shell`,
	).not.toMatch(ERROR_SHELL);
	expect(
		runtimeIssues,
		`${routeCase.url} emitted runtime/network failures`,
	).toEqual([]);
}

test.describe("authenticated protected dynamic route smoke", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) =>
			browserName !== "chromium" || Boolean(isMobile),
		"the complete dynamic inventory runs once in desktop Chromium",
	);

	test("browser-mockable protected dynamic inventory stays complete", () => {
		const expected = DYNAMIC_ROUTE_CATALOG.filter(
			(entry) =>
				entry.access !== "public" &&
				entry.access !== "delegated" &&
				// 帳號層級的頁面（`account`）不在這份清單：e2e 的 mock 無法建立 Clerk 工作階段
				// （見 helpers/mock-gateway.ts），登入後的畫面由 SellerMeasureRequestPage 的元件測試承接，
				// 訪客導頁由 site-links 斷言。
				entry.access !== "account" &&
				!SSR_PREVIEW_PATTERNS.has(entry.pattern),
		).map(({ access, pattern }) => ({ role: access, pattern }));
		const covered = ROUTE_CASES.map(({ role, pattern }) => ({ role, pattern }));
		expect(covered).toHaveLength(23);
		expect(covered).toEqual(expected);
	});

	for (const routeCase of ROUTE_CASES) {
		test(`${routeCase.role} ${routeCase.pathname} renders with deterministic data`, async ({
			page,
			baseURL,
		}) => {
			test.setTimeout(45_000);
			await runDynamicRouteSmoke(
				page,
				baseURL ?? "http://localhost:3000",
				routeCase,
			);
		});
	}
});

const MOBILE_CANARY_IDS = new Set([
	"buyer-procurement-assistant-demo",
	"buyer-order-detail",
	"seller-product-edit",
	"seller-import-preview-list",
]);

test.describe("protected dynamic mobile canaries", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !isMobile,
		"mobile canaries only run in the mobile Chromium project",
	);

	for (const routeCase of ROUTE_CASES.filter(({ id }) =>
		MOBILE_CANARY_IDS.has(id),
	)) {
		test(`${routeCase.role} ${routeCase.pathname} renders on mobile`, async ({
			page,
			baseURL,
		}) => {
			test.setTimeout(45_000);
			await runDynamicRouteSmoke(
				page,
				baseURL ?? "http://localhost:3000",
				routeCase,
			);
		});
	}

	test("seller description editor keeps mobile block actions aligned", async ({
		page,
		baseURL,
	}) => {
		test.setTimeout(45_000);
		const routeCase = ROUTE_CASES.find(
			(candidate) => candidate.id === "seller-product-edit",
		);
		expect(routeCase).toBeTruthy();
		if (!routeCase) return;

		await runDynamicRouteSmoke(
			page,
			baseURL ?? "http://localhost:3000",
			routeCase,
		);

		const editor = page.locator('.desc-editorjs[data-ready="true"]');
		await expect(editor).toBeVisible();
		const blocks = editor.locator(".codex-editor__redactor > .ce-block");
		await expect(blocks).toHaveCount(3);

		const secondEditable = blocks
			.nth(1)
			.locator('[contenteditable="true"]')
			.first();
		await secondEditable.tap();

		const toolbar = editor.locator(".ce-toolbar");
		const actions = toolbar.locator(".ce-toolbar__actions");
		await expect(actions).toBeVisible();
		await expect(toolbar).toHaveAttribute("data-mobile-block-index", "1");

		const geometry = await Promise.all([
			actions.evaluate((element) => {
				const rect = element.getBoundingClientRect();
				return {
					top: rect.top,
					left: rect.left,
					right: rect.right,
					centerY: rect.top + rect.height / 2,
				};
			}),
			secondEditable.evaluate((element) => {
				const rect = element.getBoundingClientRect();
				const style = getComputedStyle(element);
				const paddingTop = Number.parseFloat(style.paddingTop) || 0;
				const lineHeight = Number.parseFloat(style.lineHeight);
				return {
					firstLineCenterY: rect.top + paddingTop + lineHeight / 2,
					viewportWidth: window.innerWidth,
				};
			}),
		]);
		const [actionGeometry, textGeometry] = geometry;
		expect(
			Math.abs(actionGeometry.centerY - textGeometry.firstLineCenterY),
		).toBeLessThanOrEqual(2);
		expect(actionGeometry.left).toBeGreaterThanOrEqual(0);
		expect(actionGeometry.top).toBeGreaterThanOrEqual(0);
		expect(actionGeometry.right).toBeLessThanOrEqual(
			textGeometry.viewportWidth,
		);

		// Reproduce the old touch bug without letting the browser synthesize a
		// pointerover: mouseover used to overwrite the insertion target even though
		// the second block remained the current mobile block.
		await blocks.nth(2).dispatchEvent("mouseover", { bubbles: false });
		await actions.locator(".ce-toolbar__plus").dispatchEvent("click");
		await expect(blocks).toHaveCount(4);
		const blockText = await blocks.evaluateAll((elements) =>
			elements.map((element) =>
				(element.textContent ?? "").replaceAll("\uFEFF", "").trim(),
			),
		);
		expect(blockText).toEqual([
			PRODUCT_DESCRIPTION_TEXT.first,
			PRODUCT_DESCRIPTION_TEXT.second,
			"",
			PRODUCT_DESCRIPTION_TEXT.third,
		]);
	});
});
