import { expect, type Page, test } from "@playwright/test";
import { requiredAccessForPath } from "@/lib/auth/redirect";
import { discoverAppPageRoutes } from "./helpers/app-route-inventory";
import { expectNoAxeViolations } from "./helpers/axe";
import { captureHydrationDiagnostics } from "./helpers/hydration-diagnostics";
import {
	mockMyKyc,
	mockMyStore,
	setupGatewayMocks,
} from "./helpers/mock-gateway";
import { assertNoResidualLoading } from "./helpers/page-stability";
import { isKnownNonEnforcingConsoleError } from "./helpers/runtime-console";
import { gotoStreamed } from "./helpers/streamed-navigation";

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
	"/checkout/success": "?order=HK-ROUTE-SMOKE",
};

const EXPECTED_FINAL_ROUTE: Partial<
	Record<string, { pathname: string; search?: string }>
> = {
	"/seller-center": {
		pathname: "/seller-center/dashboard",
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
	await mockMyStore(page, {
		id: SELLER_FIXTURE_ID,
		storeName: "路由測試商店",
		missingRequirements: ["ONBOARDING_COMPLETE"],
	});
	await mockMyKyc(page, { status: "NOT_STARTED" });
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
	captureHydrationDiagnostics(page);
	page.on("pageerror", (error) => {
		issues.push(`pageerror: ${error.stack || error.message}`);
	});
	page.on("console", (message) => {
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

/**
 * 還沒申請 KYC 的店家，GetMyKYC 依契約回 not_found（HTTP 404），瀏覽器為每一筆再記一則
 * 「Failed to load resource ... 404」的 console.error。這是契約規定的回應，不是故障；
 * 只扣掉 GetMyKYC 的 404 與同數量的那則 console 訊息，其他 404 與任何失敗照樣算。
 */
function withoutContractKycNotFound(issues: readonly string[]): string[] {
	const kycNotFound =
		/^HTTP 404 GET \S+\/hawkr\.seller\.v1alpha1\.SellerAppService\/GetMyKYC\?/;
	const consoleNotFound =
		/^console\.error: Failed to load resource: the server responded with a status of 404/;
	let consoleBudget = issues.filter((issue) => kycNotFound.test(issue)).length;
	return issues.filter((issue) => {
		if (kycNotFound.test(issue)) return false;
		if (consoleBudget > 0 && consoleNotFound.test(issue)) {
			consoleBudget--;
			return false;
		}
		return true;
	});
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
	const response = await gotoStreamed(page, target, {
		timeout: 25_000,
	});
	expect(response, `${route} returned no document response`).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${route} document response must be successful`,
	).toBeLessThan(400);

	await expect(
		page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
		`${route} never resolved its authenticated session status`,
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

	await page.waitForTimeout(500);
	const text = await pageContent.innerText();
	expect(
		withoutContractKycNotFound(runtimeIssues),
		`${route} emitted runtime/network failures`,
	).toEqual([]);
	expect(text, `${route} rendered an error/not-found shell`).not.toMatch(
		ERROR_SHELL,
	);
	if (route.startsWith("/seller-center/preview")) {
		expect(text).not.toContain("店面預覽暫時載入不到");
	}
	await assertNoResidualLoading(page, route);
	await assertRenderedLinksSafe(page, route);
	await expectNoAxeViolations(page, route);
}

test.describe("authenticated protected static route smoke", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the authenticated route inventory only needs one desktop browser",
	);

	test("protected static inventory stays complete", () => {
		expect(ROUTES_BY_ROLE.buyer).toHaveLength(23);
		expect(ROUTES_BY_ROLE.seller).toHaveLength(32);
		expect(ROUTES_BY_ROLE.seller).not.toContain("/seller-center/measures");
		expect(ACCOUNT_ROUTES).toEqual(["/seller/inquiries", "/seller/measures"]);
		expect(PROTECTED_STATIC_ROUTES).toHaveLength(57);
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
			}) => {
				const appBaseURL = baseURL ?? "http://localhost:3000";
				await prepareAuthenticatedPage(page, appBaseURL, role);
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
