import assert from "node:assert/strict";
import { registerHooks } from "node:module";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

// This Node-only render test has no CSS loader. Intercept only the two inert
// seller chrome stylesheets reached by SellerProfileV2; all business and render
// modules remain real, so the markup assertions below keep exercising the
// production SellerProfileMeta implementation.
const sellerChromeStyles = new Set([
	new URL("../styles/features/seller-profile.css", import.meta.url).href,
	new URL("../styles/features/seller-share-card.css", import.meta.url).href,
]);
const stylesheetHooks = registerHooks({
	load(url, context, nextLoad) {
		if (sellerChromeStyles.has(url)) {
			return { format: "module", source: "export {};", shortCircuit: true };
		}
		return nextLoad(url, context);
	},
});
const { SellerProfileMeta, sellerProfileLocation } = await import(
	"../features/seller/components/SellerProfileV2"
);
stylesheetHooks.deregister();

test("seller profile location is derived from the wire city and district fields", () => {
	assert.equal(
		sellerProfileLocation({ city: "台北市", district: "信義區" }),
		"台北市 · 信義區",
	);
	assert.equal(sellerProfileLocation({ city: "台北市" }), "台北市");
	assert.equal(sellerProfileLocation({ district: "信義區" }), "信義區");
	assert.equal(sellerProfileLocation({}), "");
});

test("seller profile meta never starts with an orphan separator", () => {
	const markup = renderToStaticMarkup(
		createElement(SellerProfileMeta, {
			rating: "0.0",
			totalReviews: 0,
			location: "",
			subscribers: 0,
			websiteUrl: "",
		}),
	);
	assert.match(
		markup,
		/^<div class="hkui-seller-profile__meta"><span class="hkui-seller-profile__meta-item/,
	);
	assert.doesNotMatch(
		markup,
		/__meta"><span class="hkui-seller-profile__meta-dot"/,
	);
	assert.equal(
		(markup.match(/hkui-seller-profile__meta-dot/g) ?? []).length,
		0,
	);
});

test("seller profile inserts separators only between visible meta items", () => {
	const markup = renderToStaticMarkup(
		createElement(SellerProfileMeta, {
			rating: "4.8",
			totalReviews: 12,
			location: "台北市 · 信義區",
			subscribers: 9,
			websiteUrl: "",
		}),
	);
	assert.equal(
		(markup.match(/hkui-seller-profile__meta-dot/g) ?? []).length,
		2,
	);
	assert.match(markup, /台北市 · 信義區/);
});
