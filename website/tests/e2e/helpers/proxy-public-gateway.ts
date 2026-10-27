import type { Page } from "@playwright/test";

// Loopback by default: a spec run without env must not leave the machine.
const TEST_GATEWAY_ORIGIN = new URL(
	process.env.API_BASE_URL ?? "http://127.0.0.1:9/api/v1",
).origin;

/**
 * CI bakes a same-origin localhost API base into the production-mode E2E build
 * so cookie-session tests can intercept gateway traffic deterministically.
 * The real local proxy intentionally returns 404 under `next start`, though,
 * because deployed builds must call the gateway directly. Tests that exercise
 * live public homepage content therefore need to forward those anonymous GETs
 * to the test gateway explicitly.
 */
export async function proxyPublicGatewayReads(
	page: Page,
): Promise<() => Promise<void>> {
	await page.route("**/api/v1/public/**", async (route) => {
		if (route.request().method() !== "GET") {
			await route.continue();
			return;
		}
		const requestUrl = new URL(route.request().url());
		const upstreamUrl = new URL(
			`${requestUrl.pathname}${requestUrl.search}`,
			TEST_GATEWAY_ORIGIN,
		);
		const response = await route.fetch({ url: upstreamUrl.href });
		await route.fulfill({ response });
	});

	// A detail view can start another public read (for example comments) just as
	// the assertion completes. Drain route handlers before Playwright tears down
	// the page; otherwise route.fetch rejects against the closed context and turns
	// a successful interaction into a false-negative E2E failure. But if the page
	// itself already closed (test navigated away / context torn down first),
	// unrouteAll throws "Target page, context or browser has been closed" instead
	// of the fetch rejections `ignoreErrors` is meant to swallow — skip it then.
	return async () => {
		if (page.isClosed()) return;
		await page.unrouteAll({ behavior: "ignoreErrors" });
	};
}
