import { expect, type Page, test } from "@playwright/test";
import { requiredAccessForPath } from "@/lib/auth/redirect";
import { discoverAppPageRoutes } from "./helpers/app-route-inventory";
import {
	mockMyKyc,
	mockMyStore,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { assertNoResidualLoading } from "./helpers/page-stability";
import { isKnownNonEnforcingConsoleError } from "./helpers/runtime-console";

const ACTIVE_ROLE_KEY = "hkui:auth:active-role";
const LOADER_KEY = "hawkr:initial-loader-seen";
const SELLER_FIXTURE_ID = "2683638500";
type UserRole = "buyer" | "seller";

const PROTECTED_STATIC_ROUTES = discoverAppPageRoutes({
	includeDynamic: false,
}).flatMap((route) => {
	const access = requiredAccessForPath(route);
	return access ? [{ access, route }] : [];
});

const routesOf = (access: "buyer" | "seller" | "account") =>
	PROTECTED_STATIC_ROUTES.filter((entry) => entry.access === access).map(
		({ route }) => route,
	);

const ROUTES_BY_ROLE: Record<UserRole, string[]> = {
	buyer: routesOf("buyer"),
	seller: routesOf("seller"),
};

/** 登入即可、不分工作區的頁面；訪客導 /login 且不帶 role。 */
const ACCOUNT_ROUTES = routesOf("account");

/** Callback/result screens need representative query state to exercise their
 * real rendered branch instead of an intentionally sparse no-order shell. */
const ROUTE_QUERY: Partial<Record<string, string>> = {
	"/checkout/fail": "?order=HK-ROUTE-SMOKE&reason=route-smoke",
	"/checkout/submitted": "?order=HK-ROUTE-SMOKE",
	"/checkout/success": "?order=HK-ROUTE-SMOKE",
};

const EXPECTED_FINAL_ROUTE: Partial<
	Record<string, { pathname: string; search?: string }>
> = {
	"/seller-center": {
		pathname: "/seller-center/dashboard",
	},
	"/seller-center/preview": {
		pathname: `/seller-center/preview/${SELLER_FIXTURE_ID}`,
	},
	"/seller-center/shipments/anomalies": {
		pathname: "/seller-center/orders",
		search: "?tab=in_transit",
	},
};

const ERROR_SHELL =
	/Application error|Internal Server Error|NEXT_NOT_FOUND|This page could not be found|糟糕，火箭偏離了航道|暫時迷航|這頁不在貨架上/i;
const UNRESOLVED_HREF = /(?:^|[/=?#])(?:undefined|null)(?:$|[/&#])|\[[^\]]+\]/i;

function hasControlCharacter(value: string): boolean {
	for (let index = 0; index < value.length; index++) {
		const code = value.charCodeAt(index);
		if (code <= 0x1f || code === 0x7f) return true;
	}
	return false;
}

async function prepareAuthenticatedPage(
	page: Page,
	baseURL: string,
	role: UserRole,
) {
	// Register the generic fallback first. setupGatewayMocks installs its
	// session/app handlers afterwards, and Playwright gives the most recently
	// registered matching route priority.
	await page.route("**/api/v1/**", (route) => {
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, data: {} }),
		});
	});
	await setupGatewayMocks(page, {
		identity: {
			accountId: "1",
			roles: role === "buyer" ? ["buyer"] : ["seller"],
		},
		session: "identity",
	});
	// The seller shell and preview resolver both require a real store record.
	// Register this after the generic gateway mocks so the exact GET wins.
	// 一個完成新手引導、但還沒認證的店家：舊形狀會讓賣家看起來是半註冊帳號，路由守衛的判定
	// 就測不到了。
	await mockMyStore(page, {
		id: SELLER_FIXTURE_ID,
		storeName: "路由測試商店",
		missingRequirements: ["KYC_APPROVED"],
	});
	await mockMyKyc(page, { status: "NOT_STARTED" });
	await page.route("**/api/v1/app/product_import_jobs**", (route) => {
		const request = route.request();
		const pathname = new URL(request.url()).pathname;
		if (
			request.method() !== "GET" ||
			!pathname.endsWith("/app/product_import_jobs")
		) {
			void route.fallback();
			return;
		}
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({
				code: 0,
				data: { records: [], total: 0, page: 1, pageSize: 20 },
			}),
		});
	});
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
	page.on("pageerror", (error) => {
		issues.push(`pageerror: ${error.stack || error.message}`);
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
		const requestUrl = new URL(request.url());
		// Chromium aborts superseded fetches when a guard redirects or a query
		// observer is disposed during navigation. The destination document is
		// still verified below; only completed HTTP failures are actionable.
		if (/net::ERR_ABORTED|cancelled/i.test(failure)) {
			return;
		}
		if (requestUrl.origin === appOrigin) {
			issues.push(
				`requestfailed: ${request.method()} ${request.url()} (${failure})`,
			);
		}
	});
	page.on("response", (response) => {
		const status = response.status();
		if (status < 400) return;
		try {
			if (new URL(response.url()).origin !== appOrigin) return;
		} catch {
			return;
		}
		issues.push(
			`HTTP ${status} ${response.request().method()} ${response.url()}`,
		);
	});
	return issues;
}

async function assertRenderedLinksSafe(page: Page, route: string) {
	const hrefs = await page.locator("a[href]").evaluateAll((anchors) =>
		anchors.flatMap((anchor) => {
			const href = anchor.getAttribute("href");
			return href ? [href] : [];
		}),
	);
	const current = new URL(page.url());
	const issues: string[] = [];

	for (const href of hrefs) {
		if (
			href.startsWith("//") ||
			href.includes("\\") ||
			hasControlCharacter(href)
		) {
			issues.push(`unsafe href: ${href}`);
			continue;
		}
		if (href.startsWith("#")) {
			const id = decodeURIComponent(href.slice(1));
			const hasTarget =
				!id ||
				(await page
					.locator("[id]")
					.evaluateAll(
						(nodes, expectedId) =>
							nodes.some((node) => (node as HTMLElement).id === expectedId),
						id,
					));
			if (!hasTarget) {
				issues.push(`missing hash target: ${href}`);
			}
			continue;
		}

		let target: URL;
		try {
			target = new URL(href, current);
		} catch {
			issues.push(`invalid href: ${href}`);
			continue;
		}
		if (target.origin === current.origin) {
			if (!["http:", "https:"].includes(target.protocol)) {
				issues.push(`unsafe internal protocol: ${href}`);
			} else if (UNRESOLVED_HREF.test(`${target.pathname}${target.search}`)) {
				issues.push(`unresolved internal href: ${href}`);
			}
		} else if (!["https:", "mailto:", "tel:"].includes(target.protocol)) {
			issues.push(`unsafe external protocol: ${href}`);
		}
	}

	expect(issues, `${route} rendered unsafe or unresolved links`).toEqual([]);
}

async function assertProtectedRoute(
	page: Page,
	route: string,
	baseURL: string,
) {
	const runtimeIssues = attachRuntimeMonitors(page, baseURL);
	const target = `${route}${ROUTE_QUERY[route] ?? ""}`;
	const response = await page.goto(target, {
		waitUntil: "domcontentloaded",
		timeout: 25_000,
	});
	expect(response, `${route} returned no document response`).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${route} document response must be successful`,
	).toBeLessThan(400);

	await expect(
		page.locator('[data-auth-hydrated="true"]'),
		`${route} never hydrated its authenticated shell`,
	).toBeAttached({ timeout: 15_000 });

	const expected = EXPECTED_FINAL_ROUTE[route] ?? { pathname: route };
	await expect
		.poll(() => {
			const current = new URL(page.url());
			return `${current.pathname}${expected.search ? current.search : ""}`;
		}, `${route} ended on the wrong page`)
		.toBe(`${expected.pathname}${expected.search ?? ""}`);

	const pageContent = page.locator("main.page-content");
	await expect(
		pageContent,
		`${route} has no visible page-content region`,
	).toBeVisible({ timeout: 15_000 });
	await expect
		.poll(
			async () => (await pageContent.innerText()).trim().length,
			`${route} rendered only shared chrome or an empty main region`,
		)
		.toBeGreaterThan(10);

	// Give client queries, lazy chunks, and redirect guards one turn to surface
	// errors that occur after the first hydrated commit.
	await page.waitForTimeout(500);
	const text = await pageContent.innerText();
	expect(runtimeIssues, `${route} emitted runtime/network failures`).toEqual(
		[],
	);
	expect(text, `${route} rendered an error/not-found shell`).not.toMatch(
		ERROR_SHELL,
	);
	if (route === "/seller-center/preview") {
		expect(text).not.toContain("商店缺少公開識別碼");
	}
	await assertNoResidualLoading(page, route);
	await assertRenderedLinksSafe(page, route);
}

test.describe("authenticated protected static route smoke", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the authenticated route inventory only needs one desktop browser",
	);

	test("protected static inventory stays complete", () => {
		// `/account/find-sellers` is now a redirect into the canonical
		// procurement assistant, not a protected page of its own.
		// 58 號電子發票買家面：B09 買家發票中心 ＋1 —— /account/invoices。
		// B05（/account/invoices/[documentId]）是動態路由，登記在 dynamic-route-catalog。
		expect(ROUTES_BY_ROLE.buyer).toHaveLength(23);
		// 58 號電子發票賣家面：
		//   第一波（S01／S02／S06）＋3 —— /seller-center/invoices、.../tax-status、
		//     .../documents
		//   第二波（S08–S12）＋1 —— .../invoices/appeals（S09）
		//   第三波（S03／S04／S05）＋3 —— .../invoices/amego、
		//     .../invoices/self-issue-terms、.../invoices/receipt-authorization
		// S07 掛進既有的訂單詳情頁、S12 是 .../documents 的 `?tab=received` 分頁，
		// 兩者都不是新路由；S08／S10／S11 是動態路由，登記在 dynamic-route-catalog。
		// ⚠️ 賣家面已無未實作的 RPC（2026-09-01 從 gateway handler 的呼叫點反推查證，
		// 而不是憑記憶猜 RPC 名——先前用自己編的名字查，把兩個做好的端點誤判成缺）。
		//
		// ⚠️ 下面的數字是**手寫 pin**，而路由本身是 `discoverAppPageRoutes` 自動掃出來的
		// ⇒ 新增一個 page.tsx 就會讓它們對不上。改的時候先對帳增減量，
		// 不要為了讓測試變綠而直接抄失敗訊息的數字。
		// 賣家工作區 31 條，不含措施頁；措施與申覆列表 /seller/measures 是帳號層級
		// （登入即可、不分工作區），單獨一條（ACCOUNT_ROUTES）。買家 23 ＋ 賣家 31 ＋ 帳號 1，
		// 三類合計 55。單筆、申覆與解除請求頁 /seller/measures/[measureId](/appeal|/remediation)
		// 是動態路由，登記在 dynamic-route-catalog。
		expect(ROUTES_BY_ROLE.seller).toHaveLength(31);
		expect(ROUTES_BY_ROLE.seller).not.toContain("/seller-center/measures");
		expect(ACCOUNT_ROUTES).toEqual(["/seller/measures"]);
		expect(PROTECTED_STATIC_ROUTES).toHaveLength(55);
		expect(ROUTES_BY_ROLE.buyer).not.toContain("/account/find-sellers");
		expect(ROUTES_BY_ROLE.buyer).toContain(
			"/account/messages/assistant/procurement",
		);
		expect(ROUTES_BY_ROLE.buyer).toContain("/account/invoice-profiles");
		expect(ROUTES_BY_ROLE.buyer).toContain("/account/invoices");
	});

	for (const role of ["buyer", "seller"] as const) {
		for (const route of ROUTES_BY_ROLE[role]) {
			test(`${role} ${route} renders for the matching single-role account`, async ({
				page,
				baseURL,
				request,
			}) => {
				const appBaseURL = baseURL ?? "http://localhost:3000";
				await prepareAuthenticatedPage(page, appBaseURL, role);
				// /seller-center/preview redirects into /seller-center/preview/[id],
				// whose server layout SSR-fetches the *live* public seller record for
				// SELLER_FIXTURE_ID — a request Playwright cannot intercept. On a
				// pre-launch prod backend that seller does not exist, so the layout
				// legitimately notFound()s. Probe the same public endpoint and skip
				// when the fixture seller is absent; the full render assertion still
				// runs whenever the record exists (dev).
				if (route === "/seller-center/preview") {
					const probe = await request.get(
						`/api/v1/public/sellers/${SELLER_FIXTURE_ID}`,
						{ failOnStatusCode: false, timeout: 15_000 },
					);
					let sellerPresent = false;
					if (probe.status() === 200) {
						const body = (await probe.json().catch(() => null)) as {
							data?: unknown;
						} | null;
						sellerPresent = Boolean(body?.data);
					}
					test.skip(
						!sellerPresent,
						"public seller storefront fixture absent (pre-launch prod)",
					);
				}
				await assertProtectedRoute(page, route, appBaseURL);
			});
		}
	}
});

test.describe("guest redirects by access class", { tag: ["@ui"] }, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the redirect contract is the same in every browser; run it once",
	);

	for (const [route, role] of [
		["/account/orders", "buyer"],
		["/seller-center/orders", "seller"],
	] as const) {
		test(`guest on ${route} is sent to login with role=${role}`, async ({
			request,
		}) => {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			expect(response.status()).toBe(307);
			const target = new URL(
				response.headers().location ?? "",
				"http://route-contract.local",
			);
			expect(target.pathname).toBe("/login");
			expect(target.searchParams.get("role")).toBe(role);
			expect(target.searchParams.get("redirect")).toBe(route);
		});
	}

	for (const route of ACCOUNT_ROUTES) {
		test(`guest on account page ${route} is sent to login without a role and returns to it`, async ({
			request,
		}) => {
			const response = await request.get(`${route}?from=notice`, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			expect(response.status()).toBe(307);
			const target = new URL(
				response.headers().location ?? "",
				"http://route-contract.local",
			);
			expect(target.pathname).toBe("/login");
			expect(target.searchParams.has("role")).toBe(false);
			expect(target.searchParams.get("redirect")).toBe(`${route}?from=notice`);
		});
	}
});
