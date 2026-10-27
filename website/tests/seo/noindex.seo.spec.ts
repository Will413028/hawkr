import { expect, test } from "@playwright/test";
import { canonicalFromHtml } from "./_helpers";

/**
 * Private / thin / transactional routes. Each must (1) render a noindex robots
 * meta and (2) carry NO canonical (a canonical on a noindex page is a mixed
 * signal). A synthetic session cookie lets middleware serve the actual private
 * document. Reading raw HTML (without following redirects) prevents this test
 * from accidentally validating `/login` metadata instead of the requested
 * route's metadata.
 */
const PRIVATE_ROUTES = [
	"/search?q=test",
	"/login",
	"/cart",
	"/account",
	"/seller-center/dashboard",
	"/seller-product-import-preview/test-job",
] as const;

for (const route of PRIVATE_ROUTES) {
	test(`${route} is noindex and has no canonical`, async ({ request }) => {
		const response = await request.get(route, {
			headers: { cookie: "hk_session=seo-route-contract" },
			maxRedirects: 0,
		});
		expect(response.status(), `${route} must render its own document`).toBe(
			200,
		);
		const html = await response.text();
		const robots =
			html
				.match(/<meta\b[^>]*name="robots"[^>]*>/i)?.[0]
				?.match(/content="([^"]*)"/i)?.[1] ?? null;
		expect(robots, `${route} missing robots meta`).not.toBeNull();
		expect((robots as string).toLowerCase()).toContain("noindex");
		expect(
			canonicalFromHtml(html),
			`${route} must not emit canonical`,
		).toBeNull();
	});
}
