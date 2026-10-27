import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import {
	PRODUCT_CONTENT_READY_SELECTOR,
	ROUTE_TERMINAL_READY_SELECTOR,
	routeContentReadySelectors,
	SEARCH_CONTENT_READY_SELECTOR,
	SELLER_PROFILE_READY_SELECTOR,
	SELLER_STOREFRONT_CONTENT_READY_SELECTOR,
} from "../lib/loading/initial-route-ready";

test("request-time public routes own an explicit initial content handoff", () => {
	assert.deepEqual(routeContentReadySelectors("/search"), [
		SEARCH_CONTENT_READY_SELECTOR,
	]);
	assert.deepEqual(routeContentReadySelectors("/search/"), [
		SEARCH_CONTENT_READY_SELECTOR,
	]);
	assert.deepEqual(routeContentReadySelectors("/products/123"), [
		PRODUCT_CONTENT_READY_SELECTOR,
	]);
	assert.deepEqual(routeContentReadySelectors("/products/not-a-valid-id"), [
		PRODUCT_CONTENT_READY_SELECTOR,
	]);
	assert.deepEqual(routeContentReadySelectors("/sellers/456/products/123"), [
		PRODUCT_CONTENT_READY_SELECTOR,
	]);

	for (const pathname of [
		"/sellers/456",
		"/sellers/456/messages",
		"/sellers/456/posts",
		"/sellers/456/reviews",
		"/sellers/456/reviews/",
	]) {
		assert.deepEqual(
			routeContentReadySelectors(pathname),
			[SELLER_PROFILE_READY_SELECTOR, SELLER_STOREFRONT_CONTENT_READY_SELECTOR],
			pathname,
		);
	}

	assert.deepEqual(routeContentReadySelectors("/about"), []);
});

test("protected route guards remain part of the same handoff policy", () => {
	assert.deepEqual(routeContentReadySelectors("/account/notifications"), [
		'[data-hk-page-content-ready="notifications"]',
		'[data-hk-role-zone-ready="buyer"]',
	]);
	assert.deepEqual(routeContentReadySelectors("/seller-center/wallet"), [
		'[data-hk-page-content-ready="seller-wallet"]',
		'[data-hk-role-zone-ready="seller"]',
		"[data-hk-seller-workspace-ready]",
	]);
});

test("role-zone markers come from the access table, account pages included", () => {
	assert.deepEqual(routeContentReadySelectors("/seller/measures"), [
		'[data-hk-role-zone-ready="account"]',
	]);
	for (const path of [
		"/seller/measures/m1",
		"/seller/measures/m1/appeal",
		"/seller/measures/m1/remediation",
	]) {
		assert.deepEqual(routeContentReadySelectors(path), [
			'[data-hk-role-zone-ready="account"]',
		]);
	}
	assert.deepEqual(routeContentReadySelectors("/checkout/pay"), [
		'[data-hk-role-zone-ready="buyer"]',
	]);
	assert.deepEqual(
		routeContentReadySelectors("/seller-product-import-preview/job-1"),
		['[data-hk-role-zone-ready="seller"]'],
	);
	// Public and onboarding pages under /seller wait for no role zone.
	assert.deepEqual(routeContentReadySelectors("/seller/pricing"), []);
	assert.deepEqual(routeContentReadySelectors("/seller/onboarding"), []);
});

test("every public handoff selector is emitted by complete or terminal content", async () => {
	const sources = await Promise.all(
		[
			// /search is ISR: the ready marker lives in the client shell that renders
			// the landing/results, not the server page.
			"../features/search/SearchClient.tsx",
			"../features/product/ProductDetailLayout.tsx",
			"../features/product/ProductUnavailableView.tsx",
			"../features/sellers/storefront/SellerProductsClient.tsx",
			"../features/seller/components/SellerProfileV2.tsx",
			"../features/sellers/posts/SellerPostsPage.tsx",
			"../features/sellers/messages/SellerMessagesPage.tsx",
			"../features/sellers/reviews/ReviewsClient.tsx",
			"../components/shared/ErrorState.tsx",
			// The 404 markup, split out of app/[locale]/not-found.tsx so that route file
			// can be a server component and carry its own metadata title.
			"../components/shared/NotFoundView.tsx",
		].map((path) => readFile(new URL(path, import.meta.url), "utf8")),
	);
	const [
		search,
		product,
		productUnavailable,
		sellerProducts,
		sellerProfile,
		sellerPosts,
		sellerMessages,
		sellerReviews,
		errorState,
		notFound,
	] = sources;

	assert.match(search, /data-hk-page-content-ready="search"/);
	assert.match(product, /"product-detail"/);
	assert.match(product, /data-hk-product-detail-id=/);
	assert.match(
		productUnavailable,
		/data-hk-page-content-ready="product-detail"/,
	);
	assert.match(productUnavailable, /data-hk-product-detail-id=\{productId\}/);
	assert.match(sellerProfile, /data-hk-seller-profile-ready/);
	for (const source of [
		sellerProducts,
		sellerPosts,
		sellerMessages,
		sellerReviews,
	]) {
		assert.match(source, /data-hk-page-content-ready="seller-storefront"/);
	}
	assert.match(errorState, /data-hk-route-terminal-ready="error"/);
	assert.match(notFound, /data-hk-route-terminal-ready="not-found"/);
	assert.equal(ROUTE_TERMINAL_READY_SELECTOR, "[data-hk-route-terminal-ready]");
});
