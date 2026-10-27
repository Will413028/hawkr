import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { footerPerspectiveForPathname } from "../lib/navigation/footer-perspective";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("seller workspace pages share the seller footer", () => {
	assert.equal(footerPerspectiveForPathname("/seller-center"), "seller");
	assert.equal(
		footerPerspectiveForPathname("/seller-center/dashboard"),
		"seller",
	);
	assert.equal(
		footerPerspectiveForPathname("/seller-center/orders/123"),
		"seller",
	);
});

test("buyer-facing pages share the buyer footer", () => {
	assert.equal(footerPerspectiveForPathname("/"), "buyer");
	assert.equal(footerPerspectiveForPathname("/market/products"), "buyer");
	assert.equal(footerPerspectiveForPathname("/account/orders"), "buyer");
	assert.equal(footerPerspectiveForPathname("/sellers/123"), "buyer");
});

test("public support pages retain the active seller workspace footer", () => {
	assert.equal(
		footerPerspectiveForPathname("/return-policy", "seller"),
		"seller",
	);
	assert.equal(footerPerspectiveForPathname("/faq", "seller"), "seller");
	assert.equal(footerPerspectiveForPathname("/privacy", "seller"), "seller");
	assert.equal(
		footerPerspectiveForPathname("/return-policy", "buyer"),
		"buyer",
	);
});

test("seller storefront preview uses the buyer footer", () => {
	assert.equal(footerPerspectiveForPathname("/seller-center/preview"), "buyer");
	assert.equal(
		footerPerspectiveForPathname("/seller-center/preview/123", "seller"),
		"buyer",
	);
	assert.equal(
		footerPerspectiveForPathname("/seller-center/preview/123/posts"),
		"buyer",
	);
});

test("similar route prefixes do not inherit seller chrome", () => {
	assert.equal(
		footerPerspectiveForPathname("/seller-center-preview/123"),
		"buyer",
	);
});

test("buyer perspective resolves to the canonical marketplace footer", () => {
	const roleAwareFooter = source("../components/shared/RoleAwareFooter.tsx");
	const buyerFooter = source("../components/shared/Footer.tsx");

	assert.match(roleAwareFooter, /import LegacyFooter from .*Footer/);
	// The 商品分類 / 熱門話題 columns are read on the server and handed in as an
	// element, because this component is client-only (see
	// tests/e2e/footer-columns-server-rendered.spec.ts).
	assert.match(
		roleAwareFooter,
		/return <LegacyFooter categoryColumns=\{categoryColumns\} \/>/,
	);
	assert.match(buyerFooter, /快速連結/);
	assert.match(buyerFooter, /客戶服務/);
	assert.match(buyerFooter, /聯絡資訊/);
	assert.match(buyerFooter, /supports@example\.com/);
	assert.match(buyerFooter, /<AppDownloadCTA variant="icon" \/>/);
	// 商品分類 / 熱門話題 ship as one client island between 快速連結 and 客戶服務.
	assert.match(buyerFooter, /\{categoryColumns\}/);
});

test("seller perspective resolves to one canonical seller footer", () => {
	const roleAwareFooter = source("../components/shared/RoleAwareFooter.tsx");
	const sellerFooter = source("../components/shared/SellerFooter.tsx");

	assert.match(roleAwareFooter, /import SellerFooter/);
	assert.match(roleAwareFooter, /useAuth\(\)/);
	assert.match(
		roleAwareFooter,
		/footerPerspectiveForPathname\(pathname, activeRole\)/,
	);
	assert.match(roleAwareFooter, /return <SellerFooter \/>/);
	assert.match(sellerFooter, /賣家中心/);
	assert.match(sellerFooter, /商品管理/);
	assert.match(sellerFooter, /訂單管理/);
});
