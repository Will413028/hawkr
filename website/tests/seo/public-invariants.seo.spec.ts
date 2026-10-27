import { expect, test } from "@playwright/test";
import { defaultLocale } from "@/lib/intl/locales";
import { absoluteUrl } from "@/lib/seo/urls";
import {
	getCanonical,
	getMetaDescription,
	getRobotsMeta,
	h1Count,
	jsonLdTypes,
	parseJsonLd,
} from "./_helpers";

/** Public, indexable, backend-independent routes (render 200 even if API is down). */
const PUBLIC_ROUTES = [
	"/",
	"/market/products",
	"/feed",
	"/faq",
	"/seller",
	"/seller/pricing",
	"/about",
	"/contact",
	"/privacy",
	"/terms",
	"/return-policy",
	"/user-policy",
	"/seller-terms",
] as const;

for (const route of PUBLIC_ROUTES) {
	test.describe(`public invariants: ${route}`, () => {
		test.beforeEach(async ({ page }) => {
			await page.goto(route, { waitUntil: "domcontentloaded" });
		});

		test("renders exactly one <h1>", async ({ page }) => {
			expect(await h1Count(page)).toBe(1);
		});

		test("self-canonical equals the clean route URL", async ({ page }) => {
			expect(await getCanonical(page)).toBe(absoluteUrl(route, defaultLocale));
		});

		test("robots meta is indexable (gate open)", async ({ page }) => {
			const robots = await getRobotsMeta(page);
			expect(robots ?? "index, follow").not.toContain("noindex");
		});

		test("has a 120–160 char meta description", async ({ page }) => {
			const desc = await getMetaDescription(page);
			expect(desc, `${route} missing description`).not.toBeNull();
			expect(
				(desc as string).length,
				`${route} description length ${(desc as string).length}`,
			).toBeGreaterThanOrEqual(120);
			expect((desc as string).length).toBeLessThanOrEqual(160);
		});

		test("every <img> carries an alt attribute", async ({ page }) => {
			// Snapshot all <img> alts atomically in the browser. A count()+nth(i)
			// loop races page hydration (the live img set mutates between awaits, so
			// a now-detached nth(i) times out); evaluateAll reads them in one pass.
			// alt="" is valid for decorative imgs — a MISSING alt attr (null) fails.
			const missing = await page
				.locator("img")
				.evaluateAll((els) =>
					els
						.map((el, i) => (el.getAttribute("alt") === null ? i : -1))
						.filter((i) => i >= 0),
				);
			expect(
				missing,
				`img indices without an alt attribute on ${route}: ${missing}`,
			).toEqual([]);
		});

		test("emits root Organization + WebSite JSON-LD that parses", async ({
			page,
		}) => {
			const types = jsonLdTypes(await parseJsonLd(page));
			expect(types.has("Organization")).toBeTruthy();
			expect(types.has("WebSite")).toBeTruthy();
		});
	});
}

test("public route meta descriptions are unique", async ({ page }) => {
	const seen = new Map<string, string>();
	for (const route of PUBLIC_ROUTES) {
		await page.goto(route, { waitUntil: "domcontentloaded" });
		const desc = (await getMetaDescription(page)) ?? "";
		const dup = seen.get(desc);
		expect(dup, `${route} shares its description with ${dup}`).toBeUndefined();
		seen.set(desc, route);
	}
});
