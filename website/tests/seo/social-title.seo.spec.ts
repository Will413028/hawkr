import { expect, test } from "@playwright/test";
import { SITE_NAME } from "@/lib/seo/metadata";
import { metaFromHtml, titleFromHtml } from "./_helpers";

/**
 * The share card must name the PAGE, not the site.
 *
 * Next resolves a social title by falling back twitter → openGraph →
 * `metadata.title`, and each link fires only when the previous one is empty.
 * Because segment merging hands a parent's `openGraph` to every descendant that
 * declares none, one title spelled into the ROOT openGraph short-circuits that
 * chain for the entire site — and does so invisibly, since each page's own
 * <title> keeps rendering perfectly in the tab. Measured before the fix: 84 of
 * 98 routes served the home page's title as their og:title AND twitter:title.
 *
 * tests/seo-metadata.test.ts guards the cause (root declares no social title)
 * in milliseconds on every push. This guards the EFFECT, in the served HTML,
 * which is the only place the fallback chain actually runs — a unit test on
 * buildRootMetadata() cannot tell whether Next still fills the way we think it
 * does after an upgrade, and this can.
 *
 * Read via request.get() rather than a browser: these are head-only assertions,
 * several routes are private (a synthetic session cookie is enough for
 * middleware to serve the real document), and raw HTML sidesteps the streaming
 * stalls the entity specs hit.
 */

/** What the root serves when a page supplies no title of its own. */
const ROOT_DEFAULT_TITLE = "Hawkr — 在地職人的市集";
const BRAND_SUFFIX = ` | ${SITE_NAME}`;

/**
 * Routes whose social title is ALLOWED to equal the root default — i.e. pages
 * that legitimately are the site. Named individually, never by prefix: a
 * directory-wide exemption is how the 84 hid in the first place.
 */
const ROOT_TITLE_IS_CORRECT = new Set<string>([
	// The home page's own title IS the root default (title.absolute), so its
	// card matching the root's is the correct answer, not a missing one.
	"/",
]);

const ROUTES = [
	// Public, indexable.
	"/",
	"/about",
	"/feed",
	"/market/products",
	"/faq",
	"/contact",
	"/seller",
	"/seller/pricing",
	"/seller-guide",
	// Public, indexable, and previously wrong: these four set their own
	// description but inherited the root's og:description as well.
	"/privacy",
	"/terms",
	"/return-policy",
	"/user-policy",
	"/seller-terms",
	// Private / noindex. Not indexed, but they ARE pasted into chats and tickets.
	"/search?q=test",
	"/login",
	"/cart",
	"/account",
	"/account/orders",
	"/seller-center/dashboard",
	"/seller-center/orders",
	// Redirect-only aliases. They answer 200 with a full document because the
	// response has already begun streaming when redirect() throws, so they need
	// (and now have) titles of their own — see the two page.tsx files.
	"/seller-center",
	"/seller-center/shipments/anomalies",
	// Miss branches. The route-level 404 and the app-wide one both used to serve
	// the home page's title while answering "this does not exist".
	"/products/99999999",
	"/this-route-does-not-exist",
] as const;

for (const route of ROUTES) {
	test(`${route} shares its own title, not the site's`, async ({ request }) => {
		const response = await request.get(route, {
			headers: { cookie: "hk_session=seo-route-contract" },
			maxRedirects: 0,
		});
		// 404 is a real document for the two miss branches; anything else must be
		// a 200, or we are asserting against the wrong page.
		expect(
			[200, 404],
			`${route} answered ${response.status()} — expected a rendered document`,
		).toContain(response.status());

		const html = await response.text();
		const title = titleFromHtml(html);
		const og = metaFromHtml(html, "og:title");
		const twitter = metaFromHtml(html, "twitter:title");

		expect(title, `${route} has no <title>`).toBeTruthy();
		expect(og, `${route} has no og:title`).toBeTruthy();
		expect(twitter, `${route} has no twitter:title`).toBeTruthy();

		// The twitter card is filled from openGraph. If these ever diverge, a
		// twitter title has been declared somewhere and the fill is suppressed.
		expect(twitter, `${route}: twitter:title no longer mirrors og:title`).toBe(
			og,
		);

		// og:title is emitted verbatim while <title> runs through the root
		// "%s | Hawkr" template, so the two agree in one of exactly two shapes:
		// the page handed Next a full title (they match), or it handed a bare one
		// and let the template add the brand (og is <title> minus the suffix).
		// Anything else means og:title came from somewhere other than this page.
		expect(
			og === title || title === `${og}${BRAND_SUFFIX}`,
			`${route}: og:title ${JSON.stringify(og)} does not correspond to ` +
				`<title> ${JSON.stringify(title)}`,
		).toBe(true);

		if (ROOT_TITLE_IS_CORRECT.has(route)) return;
		expect(
			og,
			`${route} shares the root default title — its page-specific og:title ` +
				`is missing and Next fell back to the site-wide one`,
		).not.toBe(ROOT_DEFAULT_TITLE);
	});
}

test("the route list still reaches the site it is meant to cover", () => {
	// Every assertion above is registered per route, so an emptied ROUTES leaves
	// this file reporting "passed" while checking nothing — measured: truncating
	// the list to zero still exits green, because the exception check below is
	// the only test registered unconditionally. A floor makes that loud.
	//
	// The 25 routes are hand-picked to span the four shapes the 84 hid across
	// (public indexable, private/noindex, redirect alias, miss branch), so
	// losing two is a deletion rather than an edit. Adding routes is always
	// fine; this only guards the other direction.
	expect(
		ROUTES.length,
		"ROUTES has shrunk — a social-title regression can now hide in the gap",
	).toBeGreaterThanOrEqual(24);
	expect(
		new Set(ROUTES).size,
		"ROUTES has duplicates, so its length overstates what is covered",
	).toBe(ROUTES.length);
});

test("the exception list has not quietly grown", () => {
	// A guard whose exception list can be appended to is a guard that will be
	// appended to. Exactly one route on this site is the site itself.
	expect([...ROOT_TITLE_IS_CORRECT]).toEqual(["/"]);
});
