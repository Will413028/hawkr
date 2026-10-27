import { expect, test } from "@playwright/test";

/**
 * The footer's data columns (商品分類 / 熱門話題) must be read on the server.
 *
 * They used to load through an IntersectionObserver, so the two public reads
 * fired only once the visitor scrolled near the bottom. That is what
 * site-stability's "started a content request only because the user scrolled"
 * contract forbids, and it blocked a production deploy on every long public
 * route at once. Scroll-triggered loading also hides the footer's internal
 * links from crawlers that never scroll.
 *
 * This is the hermetic half of that contract: site-stability owns the @live
 * version, which only runs on a main push and needs the shared dev gateway, so
 * a dev push could not see the regression until it had already reached main.
 *
 * Scope: this asserts the browser never issues the reads, not that the markup
 * is in the initial HTML. RoleAwareFooter picks a footer from pathname + auth
 * on the client, so no footer is in the document response today; the columns
 * are rendered by a server component passed through that client boundary and
 * arrive in the RSC payload. Making the footer itself crawlable is a separate
 * change to that gating.
 */

const FOOTER_ENDPOINTS = ["/categories/tree", "/tags/trending"];

test.describe("footer data columns", { tag: ["@ui"] }, () => {
	test("are read on the server and never fetched from the browser", async ({
		page,
	}) => {
		await page.addInitScript(() => {
			try {
				sessionStorage.setItem("hawkr:initial-loader-seen", "1");
			} catch {
				// ignore strict-mode browsers
			}
		});

		const browserReads: string[] = [];
		page.on("request", (request) => {
			const { pathname } = new URL(request.url());
			if (FOOTER_ENDPOINTS.some((endpoint) => pathname.endsWith(endpoint))) {
				browserReads.push(pathname);
			}
		});

		await page.goto("/", { waitUntil: "load" });

		const categoryColumn = page
			.locator("footer")
			.locator("details", { hasText: "商品分類" })
			.first();
		await expect(categoryColumn).toBeVisible();

		// Scrolling is what used to trigger the reads, so the assertion is only
		// meaningful once the footer has actually been reached.
		await page.evaluate(() =>
			window.scrollTo(0, document.documentElement.scrollHeight),
		);
		await expect(categoryColumn.locator("a").first()).toBeVisible();
		await page.waitForTimeout(1_000);

		expect(
			Array.from(new Set(browserReads)),
			"footer columns fetched their content from the browser instead of being read on the server",
		).toEqual([]);
	});
});
