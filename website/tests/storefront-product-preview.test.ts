import assert from "node:assert/strict";
import test from "node:test";
import {
	productIdFromStorefrontPreviewHref,
	sellerIdFromPreviewProduct,
} from "../features/seller-center/preview/storefrontProductPreview";

test("storefront preview accepts only canonical numeric public product hrefs", () => {
	assert.equal(productIdFromStorefrontPreviewHref("/products/500"), "500");
	assert.equal(productIdFromStorefrontPreviewHref("/products/500/"), "500");
	assert.equal(
		productIdFromStorefrontPreviewHref("/products/%35%30%30"),
		"500",
	);

	for (const href of [
		null,
		"//example.com/products/500",
		"https://example.com/products/500",
		"/products/500?from=preview",
		"/products/500#detail",
		"/products/not-a-number",
		"/products/500%2Freviews",
		"/sellers/10/products/500",
	]) {
		assert.equal(
			productIdFromStorefrontPreviewHref(href),
			null,
			href ?? "null",
		);
	}
});

test("storefront preview resolves and validates seller ids from public product DTOs", () => {
	assert.equal(
		sellerIdFromPreviewProduct({ seller: { storeId: "2683638500" } }),
		"2683638500",
	);
	assert.equal(
		sellerIdFromPreviewProduct({ seller: { storeId: 2683638500 } }),
		"2683638500",
	);
	assert.equal(
		sellerIdFromPreviewProduct({ sellerAccountId: "2683638501" }),
		"2683638501",
	);
	assert.equal(sellerIdFromPreviewProduct({ storeId: "not-an-id" }), null);
	assert.equal(sellerIdFromPreviewProduct(null), null);
});
