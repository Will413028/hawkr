import { expect, test } from "@playwright/test";
import { defaultLocale } from "@/lib/intl/locales";
import { productUrl, sellerUrl } from "@/lib/seo/urls";
import {
	canonicalFromHtml,
	discoverLiveEntity,
	imgTagsMissingAlt,
	jsonLdTypes,
	parseJsonLdFromHtml,
	resolveSellerCanonicalId,
} from "./_helpers";

/**
 * Entity-template SEO gate. Asserts on the SERVED HTML (request.get) — canonical,
 * JSON-LD and img alt are crawler-visible in the raw HTML and unaffected by
 * streaming.
 *
 * <h1> uniqueness is verified separately, against the final DOM, in
 * tests/seo/h1-uniqueness.seo.spec.ts. Every entity is on-demand ISR
 * (generateStaticParams returns [], so the build never prerenders real
 * entities); with cacheComponents/PPR off there is no static-shell + resume
 * dual-<h1> artifact in the raw HTML.
 *
 * The pages throw 500 in production on a backend error, so we discover a live
 * entity from the dev gateway and test.skip (with a logged reason — NOT a silent
 * pass) if none is reachable.
 */

test("PDP template: self-canonical, Product + BreadcrumbList JSON-LD, img alt", async ({
	request,
}) => {
	const id = await discoverLiveEntity(request, "product");
	test.skip(
		id === null,
		"no live product reachable from the dev gateway — entity gate skipped (NOT a pass)",
	);
	if (id === null) return;

	const res = await request.get(`/products/${id}`, { maxRedirects: 0 });
	expect(res.status(), "PDP should serve 200").toBe(200);
	const html = await res.text();

	expect(canonicalFromHtml(html)).toBe(productUrl(id, defaultLocale));

	const types = jsonLdTypes(parseJsonLdFromHtml(html));
	expect(types.has("Product"), "PDP must emit Product JSON-LD").toBeTruthy();
	expect(types.has("BreadcrumbList")).toBeTruthy();

	expect(
		imgTagsMissingAlt(html),
		"PDP <img> tag indices missing an alt attribute",
	).toEqual([]);
});

test("seller template: self-canonical, ProfilePage + BreadcrumbList JSON-LD", async ({
	request,
}) => {
	const id = await discoverLiveEntity(request, "seller");
	test.skip(
		id === null,
		"no live seller reachable from the dev gateway — entity gate skipped (NOT a pass)",
	);
	if (id === null) return;

	// Enter by the CANONICAL segment. `discoverLiveEntity` surfaces the store_id
	// (the "recommended" listing carries no store_code), and a store_id URL is
	// not where the storefront lives: middleware answers it with a real 308 to
	// the store_code (#365). This gate is about the SEO template — canonical tag
	// and JSON-LD — not about the redirect, so it must be pointed at the URL that
	// actually renders the template. Asserting 200 on a store_id URL only passed
	// while the first recommended seller happened to be a grandfathered one whose
	// store_code equals its store_id; that was luck, not a contract.
	const canonicalId = await resolveSellerCanonicalId(request, id);

	const res = await request.get(`/sellers/${canonicalId}`, {
		maxRedirects: 0,
	});
	expect(res.status(), "seller storefront should serve 200").toBe(200);
	const html = await res.text();

	expect(canonicalFromHtml(html)).toBe(sellerUrl(canonicalId, defaultLocale));

	const types = jsonLdTypes(parseJsonLdFromHtml(html));
	expect(types.has("ProfilePage")).toBeTruthy();
	expect(types.has("BreadcrumbList")).toBeTruthy();
});
