import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const productCard = new URL(
	"../features/market/components/MarketProductCard.tsx",
	import.meta.url,
);
const pageTransition = new URL(
	"../components/shared/PageTransition.tsx",
	import.meta.url,
);
const quickViewOrigin = new URL(
	"../lib/product/quickViewOrigin.ts",
	import.meta.url,
);
const marketStyles = new URL("../styles/features/market.css", import.meta.url);

test("the card exposes one full-card product link without nesting controls", async () => {
	const source = await readFile(productCard, "utf8");

	assert.equal(
		source.match(/href=\{product\.href\}/g)?.length,
		1,
		"each card should have exactly one primary product anchor",
	);
	assert.match(source, /data-market-product-primary-link=""/);
	assert.match(source, /className="absolute inset-0 z-10/);
	assert.match(source, /aria-label=\{`查看商品：\$\{product\.name\}`\}/);
	assert.match(source, /className="absolute[^"]*z-20[^"]*"/);
	// The seller anchor sits above the full-card link and points at the
	// CANONICAL storefront segment — see tests/seller-link-canonical.test.ts.
	assert.match(
		source,
		/href=\{sellerPath\(product\.sellerStoreCode, product\.sellerId\)\}/,
	);
	assert.match(source, /className="relative z-20 flex min-w-0/);
	assert.match(source, /market-product-card__qty relative z-20/);
	assert.match(source, /market-product-card__cart-btn relative z-20/);
});

test("full-card clicks still animate from the product media", async () => {
	const [transitionSource, originSource] = await Promise.all([
		readFile(pageTransition, "utf8"),
		readFile(quickViewOrigin, "utf8"),
	]);

	assert.match(
		transitionSource,
		/productCard\?\.querySelector<HTMLElement>\("\[data-zoom-card\]"\)/,
	);
	assert.match(originSource, /f&&f\.querySelector\("\[data-zoom-card\]"\)/);
});

test("seller preview cards expose dialog semantics without entering route quick view", async () => {
	const [cardSource, transitionSource, originSource, styleSource] =
		await Promise.all([
			readFile(productCard, "utf8"),
			readFile(pageTransition, "utf8"),
			readFile(quickViewOrigin, "utf8"),
			readFile(marketStyles, "utf8"),
		]);

	assert.match(cardSource, /storefrontPreviewMode\?: boolean/);
	assert.match(cardSource, /aria-haspopup=\{storefrontPreviewMode \? "dialog"/);
	assert.match(cardSource, /data-storefront-product-preview-trigger=/);
	assert.match(
		transitionSource,
		/a\[data-storefront-product-preview-trigger\]/,
	);
	assert.match(
		originSource,
		/hasAttribute\("data-storefront-product-preview-trigger"\)/,
	);
	assert.match(
		styleSource,
		/\[data-market-product-primary-link\]:focus-visible[\s\S]*outline: none;[\s\S]*border-radius: inherit;/,
	);
});
