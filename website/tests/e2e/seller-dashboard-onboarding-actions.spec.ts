import { expect, type Locator, type Page, test } from "@playwright/test";
import {
	fulfillConnect,
	type MockKycFlowStatus,
	type MockSellerRequirement,
	type MockStore,
	mockMyKyc,
	mockMyStore,
	seedStoredActiveRole,
	setupGatewayMocks,
} from "./helpers/mock-gateway";

const SELLER_ID = "90010001";
const LOADER_KEY = "hawkr:initial-loader-seen";
const DASHBOARD_UI_KEY = "hawkr:seller-dashboard:ui";

const BASE_STORE: MockStore = {
	id: SELLER_ID,
	storeCode: "2600000001",
	storeName: "任務測試商店",
	description: "用產地故事介紹每日上架的新鮮食材。",
	websiteUrl: "https://example.com/hawkr-task-fixture",
	contactPhone: "0912345678",
	address: { townCode: "66000290", addressDetail: "東關路一段 100 號" },
	logoUriTemplate: "https://example.com/{preset}/logo.png",
};

/** 認證通過（含 PayUni）：公開條件齊全；沒有食品業者登錄，展開錢包的食品列時直接顯示輸入框。 */
const APPROVED_STORE: MockStore = { ...BASE_STORE };

/** 還沒認證：伺服器的公開條件缺 KYC_APPROVED。 */
const NOT_STARTED_STORE: MockStore = {
	...BASE_STORE,
	missingRequirements: ["KYC_APPROVED"],
};

interface SellerFixture {
	store: MockStore;
	flow: MockKycFlowStatus;
}

/** 申請已核准、在 PayUni 階段的流程狀態：伺服器一定帶 `flow.payuni`。 */
const PAYUNI_STAGE: MockKycFlowStatus[] = [
	"BINDING",
	"BIND_FAILED",
	"FORWARDED",
	"UNDER_REVIEW",
	"PENDING_INFO",
	"PAYUNI_REJECTED",
];

const APPROVED: SellerFixture = { store: APPROVED_STORE, flow: "APPROVED" };
const NOT_STARTED: SellerFixture = {
	store: NOT_STARTED_STORE,
	flow: "NOT_STARTED",
};

interface ProductListRequest {
	page: string | null;
	pageSize: string | null;
	status: string | null;
}

async function prepareSellerPage(
	page: Page,
	baseURL: string,
	fixture: SellerFixture,
): Promise<{ productListRequests: ProductListRequest[] }> {
	// Lowest-priority fallback for secondary dashboard/destination queries. The
	// gateway helper and exact fixtures below are registered later, so they win.
	await page.route("**/api/v1/**", (route) => {
		void route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		});
	});

	// 其餘 Connect 方法（買家資料、通知…）：與上面的 REST 萬用規則同一個角色，回空訊息。
	await page.route("**/api/connect/**", (route) => {
		void fulfillConnect(route, {});
	});

	await setupGatewayMocks(page, {
		identity: {
			accountId: SELLER_ID,
			roles: ["seller"],
		},
		session: "identity",
	});

	await mockMyStore(page, fixture.store);
	await mockMyKyc(page, {
		status: fixture.flow,
		payuni: PAYUNI_STAGE.includes(fixture.flow)
			? {
					state: "REVIEWING",
					explanation: "伺服器獨特說明-PayUni 審核中",
				}
			: fixture.flow === "APPROVED"
				? { state: "APPROVED" }
				: undefined,
	});

	await page.route("**/api/v1/app/shipping_onboardings", (route) => {
		void route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: [] }),
		});
	});

	await page.route("**/api/v1/app/sellers/me/posts?**", (route) => {
		void route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: { records: [] } }),
		});
	});

	const productListRequests: ProductListRequest[] = [];
	await page.route("**/api/v1/app/products**", (route) => {
		const request = route.request();
		const url = new URL(request.url());
		if (request.method() !== "GET" || !url.pathname.endsWith("/app/products")) {
			void route.fallback();
			return;
		}

		productListRequests.push({
			page: url.searchParams.get("page"),
			pageSize: url.searchParams.get("page_size"),
			status: url.searchParams.get("status"),
		});
		void route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: {
					records: [],
					total: 0,
					page: Number(url.searchParams.get("page") ?? 1),
					pageSize: Number(url.searchParams.get("page_size") ?? 20),
				},
			}),
		});
	});

	await page
		.context()
		.addCookies([{ name: "hk_session", value: "opaque", url: baseURL }]);
	await seedStoredActiveRole(page, "seller");
	await page.addInitScript(
		([loaderKey, dashboardUiKey]) => {
			sessionStorage.setItem(loaderKey, "1");
			// A previous run may have persisted a collapsed card or another active
			// task. Each navigation in this matrix must start from product defaults.
			localStorage.removeItem(dashboardUiKey);
		},
		[LOADER_KEY, DASHBOARD_UI_KEY] as const,
	);

	return { productListRequests };
}

async function openDashboardTasks(page: Page): Promise<Locator> {
	await page.goto("/seller-center/dashboard");
	await expect(
		page.getByRole("heading", { name: "賣家總覽", exact: true }),
	).toBeVisible();

	const card = page.locator(".seller-onboarding-progress");
	await expect(card).toBeVisible();
	const body = card.locator(".seller-onboarding-progress__body");
	if (!(await body.isVisible())) {
		await card.getByRole("button", { name: "展開任務", exact: true }).click();
	}
	await expect(body).toBeVisible();
	return card;
}

async function selectTask(
	page: Page,
	taskTitle: string,
	panelTitle: string,
): Promise<Locator> {
	const card = await openDashboardTasks(page);
	const step = card.getByRole("button", {
		name: new RegExp(`^${taskTitle}，`),
	});
	await expect(step).toBeVisible();
	await step.click();
	await expect(step).toHaveAttribute("aria-current", "step");

	const detail = card.locator(".seller-onboarding-progress__detail");
	await expect(
		detail.getByRole("heading", { name: panelTitle, exact: true }),
	).toBeVisible();
	return detail;
}

async function clickTaskAction(
	page: Page,
	args: {
		taskTitle: string;
		panelTitle: string;
		cta: string;
	},
): Promise<void> {
	const detail = await selectTask(page, args.taskTitle, args.panelTitle);
	const action = detail.getByRole("link", { name: args.cta, exact: true });
	await expect(action).toBeVisible();
	await expect(action).not.toHaveAttribute("aria-disabled", "true");
	await action.click();
}

async function expectLocation(page: Page, expected: string): Promise<void> {
	await expect
		.poll(() => {
			const url = new URL(page.url());
			return `${url.pathname}${url.search}${url.hash}`;
		})
		.toBe(expected);
}

interface TaskActionCase {
	/** 併進 test 標題，讓失敗直接指名是哪一顆按鈕。 */
	name: string;
	taskTitle: string;
	panelTitle: string;
	cta: string;
	/** 點下去該落在哪裡（pathname + search + hash）。 */
	location: string;
	/** 落地後該頁必須成立的內容。 */
	verify: (page: Page) => Promise<void>;
}

/**
 * 「哪顆任務按鈕該落到哪一頁」的矩陣。每一列是一組獨立契約，所以是一列一個
 * test，不是一個 test 走完全部。
 *
 * 為什麼要拆：原本各列擠在同一個 test 裡循序跑，一輪要付 15 次導航。實測
 * （2026-08-27，production build ＋ CDP 6 倍 CPU 節流；該倍率下同檔的
 * not-started 那支耗時 3.5s，而 CI run 33010266441 實測 3.6s，可視為共用
 * runner 的等效速度）整支要 22.1s，而 test timeout 是 30s —— 只剩 8 秒餘裕，
 * runner 上任何額外爭用都會把它從中間砍斷。更麻煩的是**砍在哪一步只反映當下
 * 累積的耗時、不代表那一步壞掉**：CI 砍在 shipping action，本機 9 倍節流砍在
 * new-product action，兩者都不是真的故障點。拆開之後每一列各自擁有完整的 30
 * 秒預算，斷言一條沒改、沒少。
 */
const APPROVED_TASK_ACTIONS: TaskActionCase[] = [
	{
		name: "store profile action",
		taskTitle: "商店資料",
		panelTitle: "完成商店資料",
		cta: "去設定",
		location: "/seller-center/me",
		verify: async (page) => {
			await expect(
				page.getByRole("heading", { name: "商店設定", exact: true }),
			).toBeVisible();
			await expect(page.locator("form.seller-profile__form")).toBeVisible();
		},
	},
	{
		name: "store description action",
		taskTitle: "商店介紹",
		panelTitle: "完成商店介紹",
		cta: "去設定",
		location: "/seller-center/me",
		verify: async (page) => {
			await expect(
				page.getByRole("heading", { name: "商店設定", exact: true }),
			).toBeVisible();
			await expect(page.locator("form.seller-profile__form")).toBeVisible();
		},
	},
	{
		name: "shipping action",
		taskTitle: "物流設定",
		panelTitle: "設定物流與退貨",
		cta: "設定物流",
		location: "/seller-center/me/shipping",
		verify: async (page) => {
			await expect(
				page.getByRole("heading", { name: "物流與退貨", exact: true }),
			).toBeVisible();
			await expect(
				page.getByRole("heading", { name: "黑貓宅急便", exact: true }),
			).toBeVisible();
			await expect(
				page.getByRole("heading", { name: "7-11 店到店", exact: true }),
			).toBeVisible();
		},
	},
	{
		name: "new-product action",
		taskTitle: "上架商品",
		panelTitle: "上架第一個商品",
		cta: "新增商品",
		location: "/seller-center/products/new",
		verify: async (page) => {
			await expect(
				page.getByRole("heading", { name: "新增商品", exact: true }),
			).toBeVisible();
			await expect(page.locator("form.product-form-wizard")).toBeVisible();
		},
	},
	{
		name: "food-registration deep link expands its form",
		taskTitle: "食品加工登錄字號",
		panelTitle: "食品加工登錄字號",
		cta: "送出審核",
		location:
			"/seller-center/wallet?section=food-registration#food-registration",
		verify: async (page) => {
			await expect(
				page.getByRole("heading", { name: "錢包與認證", exact: true }),
			).toBeVisible();

			const row = page.locator("#food-registration");
			await expect(row).toHaveAttribute("aria-expanded", "true");
			await expect(row).toBeFocused();
			await expect(page.locator("#food-registration-form")).toBeVisible();
			await expect(
				page.getByRole("textbox", {
					name: /^食品業者登錄字號/,
				}),
			).toBeVisible();
		},
	},
];

test.describe("seller dashboard onboarding task actions", {
	tag: ["@ui"],
}, () => {
	test("dashboard asks for active products only", async ({ page, baseURL }) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		const { productListRequests } = await prepareSellerPage(
			page,
			baseURL as string,
			APPROVED,
		);

		await openDashboardTasks(page);
		await expect
			.poll(
				() =>
					productListRequests.some(
						(request) =>
							request.page === "1" &&
							request.pageSize === "1" &&
							request.status === "4004",
					),
				{
					message: "dashboard product count request should include status=4004",
				},
			)
			.toBe(true);
	});

	for (const taskAction of APPROVED_TASK_ACTIONS) {
		test(`approved seller: ${taskAction.name}`, async ({ page, baseURL }) => {
			test.skip(!baseURL, "Playwright baseURL is required");

			await prepareSellerPage(page, baseURL as string, APPROVED);

			await clickTaskAction(page, {
				taskTitle: taskAction.taskTitle,
				panelTitle: taskAction.panelTitle,
				cta: taskAction.cta,
			});
			await expectLocation(page, taskAction.location);
			await taskAction.verify(page);
		});
	}

	test("the card shows 公開前必須 and 建議完成 as two groups", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, APPROVED);
		const card = await openDashboardTasks(page);
		const aside = card.locator(".seller-onboarding-progress__aside");

		await expect(aside.getByText("公開前必須", { exact: true })).toBeVisible();
		await expect(aside.getByText("建議完成", { exact: true })).toBeVisible();
		const groups = aside.locator(".seller-onboarding-progress__steps");
		await expect(groups).toHaveCount(2);
		await expect(groups.nth(0).locator("li")).toHaveCount(2);
		await expect(
			groups.nth(0).getByRole("button", { name: /^商店資料，/ }),
		).toBeVisible();
		await expect(
			groups.nth(0).getByRole("button", { name: /^商家認證，/ }),
		).toBeVisible();
		await expect(groups.nth(1).locator("li")).toHaveCount(4);
		for (const title of [
			"物流設定",
			"上架商品",
			"食品加工登錄字號",
			"商店介紹",
		]) {
			await expect(
				groups.nth(1).getByRole("button", { name: new RegExp(`^${title}，`) }),
			).toBeVisible();
		}
		// 發布動態不是任何一組的任務。
		await expect(card.getByRole("button", { name: /^發布動態，/ })).toHaveCount(
			0,
		);
	});

	test("公開前必須 follows the server's missing requirements", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		const missing: MockSellerRequirement[] = ["LOGO", "KYC_APPROVED"];
		await prepareSellerPage(page, baseURL as string, {
			store: { ...BASE_STORE, missingRequirements: missing },
			flow: "UNDER_REVIEW",
		});
		const card = await openDashboardTasks(page);
		const required = card.locator(".seller-onboarding-progress__steps").first();

		await expect(
			required.getByRole("button", { name: /^商店資料，缺少：logo/ }),
		).toBeVisible();
		// KYC 那一列以流程狀態當狀態文字；PayUni 併在這一列，沒有獨立的收款列。
		await expect(
			required.getByRole("button", { name: /^商家認證，PayUni 審核中/ }),
		).toBeVisible();
		await expect(card.getByRole("button", { name: /^收款狀態，/ })).toHaveCount(
			0,
		);
	});

	test("shipping stays locked until the KYC flow is APPROVED", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, {
			store: NOT_STARTED_STORE,
			flow: "FORWARDED",
		});
		const card = await openDashboardTasks(page);
		const shipping = card.getByRole("button", { name: /^物流設定，/ });
		await expect(shipping).toHaveClass(/is-locked/);
		await expect(shipping).toHaveAccessibleName(/需先通過認證/);
	});

	test("a seller without certification can still open the new-product form", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, NOT_STARTED);
		await clickTaskAction(page, {
			taskTitle: "上架商品",
			panelTitle: "上架第一個商品",
			cta: "新增商品",
		});

		await expectLocation(page, "/seller-center/products/new");
		await expect(
			page.getByRole("heading", { name: "新增商品", exact: true }),
		).toBeVisible();
		await expect(page.locator("form.product-form-wizard")).toBeVisible();
	});

	test("not-started seller can use the certification task CTA", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, NOT_STARTED);
		await clickTaskAction(page, {
			taskTitle: "商家認證",
			panelTitle: "完成商家認證",
			cta: "開始認證",
		});

		await expectLocation(page, "/seller-center/me/certifications");
		await expect(
			page.getByRole("heading", {
				name: "選擇商家類型開始認證",
				exact: true,
			}),
		).toBeVisible();
	});
});
