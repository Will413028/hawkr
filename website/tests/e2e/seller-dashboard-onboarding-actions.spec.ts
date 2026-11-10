import { expect, type Locator, type Page, test } from "@playwright/test";
import {
	fulfillConnect,
	type MockKycFlowStatus,
	type MockOnboardingTask,
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

/** 平台審核通過且收款帳戶啟用：公開條件齊全。 */
const APPROVED_STORE: MockStore = { ...BASE_STORE };

/** 還沒認證：伺服器的公開條件缺 ONBOARDING_COMPLETE。 */
const NOT_STARTED_STORE: MockStore = {
	...BASE_STORE,
	missingRequirements: ["ONBOARDING_COMPLETE"],
};

interface SellerFixture {
	store: MockStore;
	flow: MockKycFlowStatus;
	/** GetMyOnboarding 的回應；沒給時由申請狀態產生（見 mock-gateway 的 `onboardingOf`）。 */
	onboarding?: MockOnboardingTask[] | "error";
}

const APPROVED: SellerFixture = { store: APPROVED_STORE, flow: "APPROVED" };
/** 商店資料還缺項：伺服器的任務是 ACTION_REQUIRED、下一步 EDIT_PROFILE。 */
const PROFILE_MISSING: SellerFixture = {
	store: { ...BASE_STORE, missingRequirements: ["LOGO"] },
	flow: "APPROVED",
	onboarding: [
		{
			id: "PROFILE",
			state: "ACTION_REQUIRED",
			action: "EDIT_PROFILE",
			title: "商店資料",
			status: "還缺少：logo。",
		},
		{
			id: "PLATFORM_REVIEW",
			state: "COMPLETED",
			action: "NONE",
			title: "平台審核",
			status: "商家認證已通過。",
		},
		{
			id: "PAYMENT_ACCOUNT",
			state: "COMPLETED",
			action: "NONE",
			title: "收款帳戶",
			status: "收款帳戶已啟用。",
		},
	],
};
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
		onboarding: fixture.onboarding,
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
	// 卡片在 seller-self、KYC 等資料回來前就渲染，且先是展開的；「公開前必須」全部滿足的帳號在資料
	// 回來後才預設收合。所以只點一次會被這個轉變抵消：重試到展開狀態穩定為止。
	const toggle = card.locator(".seller-onboarding-progress__toggle");
	await expect(async () => {
		if ((await toggle.getAttribute("aria-expanded")) !== "true") {
			await toggle.click();
		}
		await expect(body).toBeVisible({ timeout: 1_000 });
	}).toPass({ timeout: 15_000 });
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
	/** 預設是 APPROVED（必要任務都完成）。 */
	fixture?: SellerFixture;
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
		fixture: PROFILE_MISSING,
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

			await prepareSellerPage(
				page,
				baseURL as string,
				taskAction.fixture ?? APPROVED,
			);

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
		await expect(groups.nth(0).locator("li")).toHaveCount(3);
		await expect(
			groups.nth(0).getByRole("button", { name: /^商店資料，/ }),
		).toBeVisible();
		await expect(
			groups.nth(0).getByRole("button", { name: /^平台審核，/ }),
		).toBeVisible();
		await expect(
			groups.nth(0).getByRole("button", { name: /^收款帳戶，/ }),
		).toBeVisible();
		await expect(groups.nth(1).locator("li")).toHaveCount(3);
		for (const title of ["物流設定", "上架商品", "商店介紹"]) {
			await expect(
				groups.nth(1).getByRole("button", { name: new RegExp(`^${title}，`) }),
			).toBeVisible();
		}
		// 發布動態不是任何一組的任務。
		await expect(card.getByRole("button", { name: /^發布動態，/ })).toHaveCount(
			0,
		);
	});

	test("公開前必須 shows the server's task sentences", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, {
			store: { ...BASE_STORE, missingRequirements: ["ONBOARDING_COMPLETE"] },
			flow: "SUBMITTED",
			onboarding: [
				{
					id: "PROFILE",
					state: "ACTION_REQUIRED",
					action: "EDIT_PROFILE",
					title: "伺服器標題一",
					status: "伺服器句子一：缺 logo",
				},
				{
					id: "PLATFORM_REVIEW",
					state: "IN_PROGRESS",
					action: "NONE",
					title: "伺服器標題二",
					status: "伺服器句子二：審核中",
				},
				{
					id: "PAYMENT_ACCOUNT",
					state: "LOCKED",
					action: "NONE",
					title: "伺服器標題三",
					status: "伺服器句子三：尚未開放",
				},
			],
		});
		const card = await openDashboardTasks(page);
		const required = card.locator(".seller-onboarding-progress__steps").first();

		await expect(
			required.getByRole("button", {
				name: "伺服器標題一，伺服器句子一：缺 logo",
			}),
		).toBeVisible();
		await expect(
			required.getByRole("button", {
				name: "伺服器標題二，伺服器句子二：審核中",
			}),
		).toBeVisible();
		await expect(
			required.getByRole("button", {
				name: "伺服器標題三，伺服器句子三：尚未開放",
			}),
		).toBeVisible();
	});

	test("公開前必須 follows the summary, not the KYC application", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		// GetMyKYC 說已核准，摘要說平台審核還要送件：畫面跟摘要。
		await prepareSellerPage(page, baseURL as string, {
			store: NOT_STARTED_STORE,
			flow: "APPROVED",
			onboarding: [
				{
					id: "PROFILE",
					state: "COMPLETED",
					action: "NONE",
					title: "商店資料",
					status: "店名、logo 與所在地都已填寫。",
				},
				{
					id: "PLATFORM_REVIEW",
					state: "ACTION_REQUIRED",
					action: "SUBMIT_CERTIFICATION",
					title: "平台審核",
					status: "伺服器說：還沒送件",
				},
				{
					id: "PAYMENT_ACCOUNT",
					state: "LOCKED",
					action: "NONE",
					title: "收款帳戶",
					status: "平台審核通過後才能設定收款帳戶。",
				},
			],
		});
		const card = await openDashboardTasks(page);
		await expect(
			card.getByRole("button", { name: "平台審核，伺服器說：還沒送件" }),
		).toBeVisible();
		await expect(card.getByText("1/3")).toBeVisible();
	});

	test("GetMyOnboarding failing leaves one load-failure state and keeps the recommended tasks", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, {
			store: NOT_STARTED_STORE,
			flow: "SUBMITTED",
			onboarding: "error",
		});
		const card = await openDashboardTasks(page);
		const aside = card.locator(".seller-onboarding-progress__aside");
		await expect(aside.getByRole("alert")).toHaveText("系統錯誤");
		await expect(
			aside.locator(".seller-onboarding-progress__steps"),
		).toHaveCount(1);
		for (const title of ["物流設定", "上架商品", "商店介紹"]) {
			await expect(
				aside.getByRole("button", { name: new RegExp(`^${title}，`) }),
			).toBeVisible();
		}
	});

	test("shipping stays locked until the onboarding is complete", async ({
		page,
		baseURL,
	}) => {
		test.skip(!baseURL, "Playwright baseURL is required");

		await prepareSellerPage(page, baseURL as string, {
			store: NOT_STARTED_STORE,
			flow: "SUBMITTED",
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
			taskTitle: "平台審核",
			panelTitle: "完成平台審核",
			cta: "前往商家認證",
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
