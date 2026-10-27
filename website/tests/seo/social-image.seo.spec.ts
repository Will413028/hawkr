import { expect, test } from "@playwright/test";
import { metaFromHtml } from "./_helpers";

/**
 * Every page ships a share picture, including the ones that declare no
 * `openGraph` of their own.
 *
 * `app/opengraph-image.tsx` is a file convention: Next folds it into the metadata
 * of the segment whose directory holds it (`app/`) and nowhere else. The root
 * layouts are one segment below (`app/[locale]/`, `app/(delegated)/`) and their
 * `openGraph` replaces the parent's resolved one, so a page that inherits it —
 * the account, checkout, search, login and seller-center pages and the
 * route-level 404s — has og:image, `og:image:width/height/type/alt` and
 * twitter:image only because `buildRootMetadata` names the card.
 * tests/seo-social-image.test.ts guards the cause in
 * milliseconds; this guards the EFFECT in the served HTML, which is the only
 * place Next's merge and its twitter fallback actually run.
 *
 * Read with request.get() like the other head assertions (social-title.seo.spec.ts),
 * without its synthetic session cookie: every route here is open to a guest.
 */

const SHARE_IMAGE_PATH = "/opengraph-image";

/**
 * Pages whose metadata declares no `openGraph`: they inherit the root layout's.
 * Guest-reachable ones only — the account and seller-center pages sit behind the
 * login guard, and the same layouts serve them.
 */
const INHERITING = [
	"/search?q=test",
	"/login",
	"/login/sso-callback",
	"/cart",
	"/payment/return",
	"/seller/onboarding",
	// Route-level 404 (notFound() inside a [locale] route) and the app-wide one.
	"/products/99999999",
	"/this-route-does-not-exist",
] as const;

/** Pages that declare an `openGraph` and name the card through socialImages(). */
const DECLARING = ["/", "/about", "/faq", "/market/products"] as const;

for (const route of [...INHERITING, ...DECLARING]) {
	test(`${route} has a share picture and a twitter card that mirrors it`, async ({
		request,
	}) => {
		const response = await request.get(route, { maxRedirects: 0 });
		expect(
			[200, 404],
			`${route} answered ${response.status()} — expected a rendered document`,
		).toContain(response.status());

		const html = await response.text();
		const image = metaFromHtml(html, "og:image");
		expect(image, `${route} has no og:image`).toBeTruthy();
		expect(
			new URL(image ?? "").pathname,
			`${route}: og:image is not the site card`,
		).toBe(SHARE_IMAGE_PATH);
		// Next fills the twitter card from openGraph; a divergence means a twitter
		// image is declared somewhere and pins the card.
		expect(metaFromHtml(html, "twitter:image"), `${route}: twitter:image`).toBe(
			image,
		);
	});
}

for (const route of INHERITING) {
	test(`${route} describes its share picture`, async ({ request }) => {
		const response = await request.get(route, { maxRedirects: 0 });
		const html = await response.text();
		// What the file convention publishes for the picture; the inheriting pages
		// carry the same description.
		expect(metaFromHtml(html, "og:image:width"), `${route}`).toBe("1200");
		expect(metaFromHtml(html, "og:image:height"), `${route}`).toBe("630");
		expect(metaFromHtml(html, "og:image:type"), `${route}`).toBe("image/png");
		expect(metaFromHtml(html, "og:image:alt"), `${route}`).toBeTruthy();
		expect(metaFromHtml(html, "twitter:image:width"), `${route}`).toBe("1200");
		expect(metaFromHtml(html, "twitter:image:alt"), `${route}`).toBeTruthy();
	});
}

test("the route list still reaches both shapes it is meant to cover", () => {
	expect(INHERITING.length).toBeGreaterThanOrEqual(7);
	expect(DECLARING.length).toBeGreaterThanOrEqual(3);
	expect(
		new Set([...INHERITING, ...DECLARING]).size,
		"duplicate routes overstate what is covered",
	).toBe(INHERITING.length + DECLARING.length);
});
