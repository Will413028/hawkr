import { expect, test } from "@playwright/test";
import { installPublicChromeMocks } from "./helpers/mock-public-chrome";
import {
	assertPublicStaticRoute,
	EXPECTED_PUBLIC_STATIC_ROUTES,
	PUBLIC_STATIC_ROUTES,
	prepareGuestPage,
} from "./helpers/public-static-routes";

test.describe("canonical public static route browser health", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the public static inventory runs once in desktop Chromium",
	);

	test("public static inventory stays explicit and complete", () => {
		expect(PUBLIC_STATIC_ROUTES).toEqual([...EXPECTED_PUBLIC_STATIC_ROUTES]);
		expect(PUBLIC_STATIC_ROUTES).toHaveLength(23);
	});

	// EVERY route here is `@live`, including the ones that render nothing but
	// static copy (`/terms`, `/privacy`, `/faq`…). Measured, not assumed: with
	// API_BASE_URL pointed at a closed port, all 21 fail — the shared guest
	// chrome client-fetches `/public/categories/tree`, `/public/hashtags/popular`,
	// `/public/tags/trending` and `/public/posts` on every guest-facing document,
	// and this suite counts any same-origin >=400 as a page-health failure. So a
	// legal page's "does it load cleanly" check is coupled to the shared dev
	// backend just as tightly as the storefront's. That coupling — UI intent,
	// live dependency — is the thing recorded fixtures are meant to cut
	// (hawkr-website#369), and it is four endpoints wide, not three specs wide.
	for (const route of PUBLIC_STATIC_ROUTES) {
		test(`${route} hard-loads, hydrates, and renders safe links`, {
			tag: ["@live"],
		}, async ({ page, baseURL }) => {
			test.setTimeout(45_000);
			const appBaseURL = baseURL ?? "http://localhost:3000";
			const guestState = await prepareGuestPage(page);
			await assertPublicStaticRoute(page, route, appBaseURL, guestState);
		});
	}
});

// The same inventory as above, but the guest chrome's four client reads are
// answered by `page.route` fixtures, so CI (no backend) runs these and every
// page gets the axe scan. Any other `/api/**` request is recorded and fails.
test.describe("public static routes with mocked guest chrome", {
	tag: ["@ui"],
}, () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"the public static inventory runs once in desktop Chromium",
	);

	for (const route of PUBLIC_STATIC_ROUTES) {
		test(`${route} passes axe with mocked chrome data`, async ({
			page,
			baseURL,
		}) => {
			test.setTimeout(45_000);
			const unmocked = await installPublicChromeMocks(page);
			const guestState = await prepareGuestPage(page);
			await assertPublicStaticRoute(
				page,
				route,
				baseURL ?? "http://localhost:3000",
				guestState,
			);
			expect(unmocked, `${route} made unmocked /api requests`).toEqual([]);
		});
	}
});
