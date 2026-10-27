import assert from "node:assert/strict";
import test from "node:test";
import {
	getPersistentShell,
	hasSamePersistentShell,
	isProductDetailPath,
	isSameTabHop,
	isTabToTabNav,
	shouldAnimateProgrammaticRoute,
	tabKeyFor,
} from "../lib/navigation/route-transition-policy";

test("route transition policy preserves app-tab and persistent-shell continuity", () => {
	assert.equal(tabKeyFor("/market/products"), "/market");
	assert.equal(isTabToTabNav("/", "/search"), true);
	assert.equal(isSameTabHop("/market", "/market/products"), true);
	assert.equal(getPersistentShell("/seller-center/orders"), "seller-center");
	assert.equal(
		getPersistentShell("/sellers/seller-1/reviews"),
		"seller-storefront:seller-1",
	);
	assert.equal(
		hasSamePersistentShell("/account", "/account/notifications"),
		true,
	);
	assert.equal(isProductDetailPath("/products/product-1"), true);
});

test("ordinary programmatic routes receive a commit-arrival transition", () => {
	assert.equal(
		shouldAnimateProgrammaticRoute(
			"/hawkr-agent-rfq-preview/rfq-compose",
			"/hawkr-agent-rfq-preview/seller-inbox",
		),
		true,
	);
	assert.equal(shouldAnimateProgrammaticRoute("/about", "/cart"), true);
	assert.equal(shouldAnimateProgrammaticRoute("/cart", "/checkout"), true);
});

test("routes with their own continuity stay instant", () => {
	const excludedRoutes: Array<[string, string]> = [
		["/search", "/search"],
		["/", "/feed"],
		["/market", "/market/products"],
		["/account", "/account/orders"],
		["/seller-center", "/seller-center/orders"],
		["/sellers/seller-1", "/sellers/seller-1/reviews"],
		["/market/products", "/products/product-1"],
		["/about", "/sellers/seller-1"],
		["/about", "/seller/onboarding/profile"],
		["/hawkr-agent-rfq-preview/external-vendor", "/about"],
	];

	for (const [source, target] of excludedRoutes) {
		assert.equal(
			shouldAnimateProgrammaticRoute(source, target),
			false,
			`${source} -> ${target}`,
		);
	}
});
