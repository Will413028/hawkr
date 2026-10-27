import assert from "node:assert/strict";
import { test } from "node:test";
import {
	isAssistantAnalyticsRoute,
	privacySafePageReferrer,
	privacySafePageTitle,
	privacySafePageViewLocation,
	privacySafeRoutePathname,
	shouldTrackPageView,
} from "../lib/analytics/page-view-privacy";

const ORIGIN = "https://example.com";

test("procurement restore page views never expose requestId query values", () => {
	const result = privacySafePageViewLocation({
		pathname: "/account/messages/assistant/procurement",
		query: "requestId=req-secret-123&utm_source=internal",
		origin: ORIGIN,
	});

	assert.deepEqual(result, {
		pagePath: "/account/messages/assistant/procurement?utm_source=internal",
		pageLocation: `${ORIGIN}/account/messages/assistant/procurement?utm_source=internal`,
	});
	assert.doesNotMatch(JSON.stringify(result), /req-secret-123|requestId/);
});

test("hidden Agent redirects cannot leak requestId from the messages landing page", () => {
	const result = privacySafePageViewLocation({
		pathname: "/account/messages",
		query:
			"requestId=req-secret-123&utm_source=internal&requestId=req-secret-456",
		origin: ORIGIN,
	});

	assert.deepEqual(result, {
		pagePath: "/account/messages?utm_source=internal",
		pageLocation: `${ORIGIN}/account/messages?utm_source=internal`,
	});
	assert.doesNotMatch(
		JSON.stringify(result),
		/req-secret-123|req-secret-456|requestId/,
	);
});

test("assistant conversation page views use a route template instead of the id", () => {
	const result = privacySafePageViewLocation({
		pathname: "/account/messages/assistant/843856384063471760",
		query: "",
		origin: ORIGIN,
	});

	assert.deepEqual(result, {
		pagePath: "/account/messages/assistant/:conversationId",
		pageLocation: `${ORIGIN}/account/messages/assistant/:conversationId`,
	});
	assert.doesNotMatch(JSON.stringify(result), /843856384063471760/);
});

test("only exact static Agent paths are public; future nested values stay redacted", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/account/messages/assistant/procurement/future-secret",
			query: "",
			origin: ORIGIN,
		}),
		{
			pagePath: "/account/messages/assistant/:conversationId",
			pageLocation: `${ORIGIN}/account/messages/assistant/:conversationId`,
		},
	);
	assert.equal(
		privacySafeRoutePathname("/account/messages/assistant/new"),
		"/account/messages/assistant/new",
	);
});

test("non-attribution query values are removed on every route", () => {
	assert.equal(isAssistantAnalyticsRoute("/search"), false);
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/search",
			query:
				"q=apple&token=secret&buyerId=843856384063471760&buyerName=王小明&utm_source=google&gclid=Cj0KCQjw1234567890AB",
			origin: ORIGIN,
		}),
		{
			pagePath: "/search?utm_source=google&gclid=Cj0KCQjw1234567890AB",
			pageLocation: `${ORIGIN}/search?utm_source=google&gclid=Cj0KCQjw1234567890AB`,
		},
	);
});

test("known dynamic routes collapse identifiers to stable templates", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/seller-center/orders/ORD-2026-0001",
			query: "order=ORD-2026-0001&utm_campaign=seller_search",
			origin: ORIGIN,
		}),
		{
			pagePath: "/seller-center/orders/:orderNo?utm_campaign=seller_search",
			pageLocation: `${ORIGIN}/seller-center/orders/:orderNo?utm_campaign=seller_search`,
		},
	);
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/adm/2685914682/orders/HK260830001",
			query: "",
			origin: ORIGIN,
		}),
		{
			pagePath: "/adm/:storeId/orders/:orderNo",
			pageLocation: `${ORIGIN}/adm/:storeId/orders/:orderNo`,
		},
	);
	assert.equal(privacySafeRoutePathname("/adm/enter"), "/adm/enter");
});

test("unknown and 404 paths fail closed instead of exposing path input", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/private@example.com",
			query: "",
			origin: ORIGIN,
		}),
		{
			pagePath: "/:unknown",
			pageLocation: `${ORIGIN}/:unknown`,
		},
	);
	assert.equal(
		privacySafeRoutePathname("/products/123/0912345678"),
		"/products/:productId",
	);
});

test("delegated catch-all routes collapse the complete unknown area", () => {
	assert.equal(
		privacySafeRoutePathname("/adm/2685914682/private@example.com"),
		"/adm/:storeId/:area",
	);
	assert.equal(
		privacySafeRoutePathname(
			"/adm/2685914682/orders/HK260830001/private@example.com",
		),
		"/adm/:storeId/orders/:orderNo",
	);
});

test("campaign parameters with obvious contact data are discarded", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/seller/pricing",
			query:
				"utm_source=google&utm_campaign=private%40example.com&utm_term=0912_345_678&utm_content=John_Smith&utm_name=王小明&gclid=Cj0KCQjw1234567890AB",
			origin: ORIGIN,
		}),
		{
			pagePath: "/seller/pricing?utm_source=google&gclid=Cj0KCQjw1234567890AB",
			pageLocation: `${ORIGIN}/seller/pricing?utm_source=google&gclid=Cj0KCQjw1234567890AB`,
		},
	);
});

test("click-attribution fields accept only provider-shaped identifiers", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/seller/pricing",
			query:
				"gclid=0912345678&gbraid=%E7%8E%8B%E5%B0%8F%E6%98%8E&gad_source=1&gad_campaignid=1234567890&wbraid=Cj0KCQjw_ABC-123456789",
			origin: ORIGIN,
		}),
		{
			pagePath: "/seller/pricing?gad_source=1&wbraid=Cj0KCQjw_ABC-123456789",
			pageLocation: `${ORIGIN}/seller/pricing?gad_source=1&wbraid=Cj0KCQjw_ABC-123456789`,
		},
	);
});

test("manual UTM values are allowlisted rather than accepting free-form PII", () => {
	assert.deepEqual(
		privacySafePageViewLocation({
			pathname: "/seller/pricing",
			query:
				"utm_source=Google&utm_medium=cpc&utm_campaign=John_Smith&utm_term=0912_345_678&utm_content=private_name",
			origin: ORIGIN,
		}),
		{
			pagePath: "/seller/pricing?utm_source=google&utm_medium=cpc",
			pageLocation: `${ORIGIN}/seller/pricing?utm_source=google&utm_medium=cpc`,
		},
	);
});

test("dynamic page titles collapse to their safe route template", () => {
	assert.equal(
		privacySafePageTitle(
			"訂單 HK260830001｜Hawkr",
			"/adm/2685914682/orders/HK260830001",
		),
		"/adm/:storeId/orders/:orderNo",
	);
});

test("same-origin referrers are scrubbed and external referrers keep only origin", () => {
	assert.equal(
		privacySafePageReferrer(
			`${ORIGIN}/checkout/success?order=ORD-SECRET&utm_source=google`,
			ORIGIN,
		),
		`${ORIGIN}/checkout/success?utm_source=google`,
	);
	assert.equal(
		privacySafePageReferrer(
			"https://partner.example/private/customer/123?email=a%40b.example",
			ORIGIN,
		),
		"https://partner.example/",
	);
});

test("the same normalized location is counted only once", () => {
	assert.equal(shouldTrackPageView(null, `${ORIGIN}/seller/pricing`), true);
	assert.equal(
		shouldTrackPageView(`${ORIGIN}/seller/pricing`, `${ORIGIN}/seller/pricing`),
		false,
	);
	assert.equal(
		shouldTrackPageView(
			`${ORIGIN}/seller/pricing`,
			`${ORIGIN}/seller/onboarding`,
		),
		true,
	);
});
