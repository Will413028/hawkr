import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import { resolveSellerCardProductState } from "../features/sellers/card/product-state";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("public seller card owns a self-canonical social profile URL", () => {
	const page = source("../app/[locale]/(public)/sellers/[id]/card/page.tsx");

	assert.match(
		page,
		/sellerCanonicalUrl\(\s*seller,\s*id,\s*[\w()]+,\s*"\/card",?\s*\)/,
	);
	assert.match(page, /type: "profile"/);
	assert.match(page, /images: socialImages\(sellerSocialImage\(seller\)\)/);
	assert.match(page, /sellerSchema\(/);
});

test("seller card renders only authoritative seller products with an honest empty state", () => {
	const page = source("../features/sellers/card/SellerCardPage.tsx");
	const rail = source("../features/sellers/card/SellerCardProductRail.tsx");
	const styles = source("../styles/features/seller-card-page.css");

	assert.match(page, /getCachedSellerStorefrontProducts\(storeId, 1, 8\)/);
	assert.match(page, /getCachedSellerProducts\(storeId, 1, 8\)/);
	assert.match(page, /productState\.records\.slice\(0, 8\)\.map\(mapProduct\)/);
	assert.doesNotMatch(page, /mock|fixture|placeholderProduct/i);
	assert.match(rail, /這間店目前還沒有上架商品/);
	assert.match(rail, /商品暫時無法載入/);
	assert.match(rail, /data-seller-card-product-rail/);
	assert.match(
		styles,
		/\.seller-public-card__cover\s*\{[\s\S]*?aspect-ratio: 4 \/ 1;/,
	);
});

test("seller card keeps public URL identity separate from authoritative store API identity", () => {
	const page = source("../features/sellers/card/SellerCardPage.tsx");
	const rail = source("../features/sellers/card/SellerCardProductRail.tsx");

	assert.match(page, /const rawStoreId = String\(seller\.storeId/);
	assert.match(page, /getCachedSellerReviewStats\(storeId\)/);
	assert.match(page, /getCachedSellerStats\(storeId\)/);
	assert.match(page, /getCachedSellerStorefrontProducts\(storeId, 1, 8\)/);
	assert.doesNotMatch(page, /getCachedSeller\w*\(sellerId/);
	assert.match(page, /followers !== null/);
	assert.match(page, /status=\{productState\.status\}/);
	assert.doesNotMatch(rail, /aria-label=\{`查看商品/);
});

test("seller card distinguishes loaded, empty, and unavailable product states", () => {
	assert.deepEqual(
		resolveSellerCardProductState({ records: ["rich"] }, undefined),
		{ records: ["rich"], status: "loaded" },
	);
	assert.deepEqual(
		resolveSellerCardProductState({ records: [] }, { records: ["lean"] }),
		{ records: ["lean"], status: "loaded" },
	);
	assert.deepEqual(
		resolveSellerCardProductState({ records: [] }, { records: [] }),
		{ records: [], status: "empty" },
	);
	assert.deepEqual(resolveSellerCardProductState({ records: [] }, null), {
		records: [],
		status: "unavailable",
	});
	assert.deepEqual(resolveSellerCardProductState(null, null), {
		records: [],
		status: "unavailable",
	});
});

test("seller card is read-only and exposes one storefront call to action", () => {
	const page = source("../features/sellers/card/SellerCardPage.tsx");
	const rail = source("../features/sellers/card/SellerCardProductRail.tsx");

	assert.doesNotMatch(
		page,
		/Seller(?:Chat|Share|Subscribe)Button|buildSellerShareCardData/,
	);
	assert.equal(page.match(/href=\{storefrontHref\}/g)?.length, 1);
	assert.match(page, /data-hkui-button=""/);
	assert.match(page, /className="seller-public-card__storefront-cta"/);
	assert.match(page, /<span>前往賣場<\/span>/);
	assert.doesNotMatch(page, /官方網站|websiteUrl|normalizeWebsiteUrl/);
	assert.doesNotMatch(rail, /storefrontHref|逛完整商店/);
	assert.match(page, /avgRating > 0/);
	assert.match(page, /followers !== null/);
});

test("seller card cover uses the official horizontal Hawkr lockup", () => {
	const page = source("../features/sellers/card/SellerCardPage.tsx");
	const styles = source("../styles/features/seller-card-page.css");

	assert.match(
		page,
		/import BrandLogo from "@\/components\/shared\/BrandLogo"/,
	);
	assert.match(
		page,
		/<BrandLogo[\s\S]*?variant="horizontal"[\s\S]*?size="sm"[\s\S]*?surface="pill"[\s\S]*?className="seller-public-card__brand"/,
	);
	assert.doesNotMatch(page, /Hawkr 商家名片/);
	assert.doesNotMatch(styles, /seller-public-card__badge/);
	assert.match(
		styles,
		/seller-public-card__brand\.brand-logo--has-dark[\s\S]*?brand-logo__lockup--default[\s\S]*?opacity: 1/,
	);
});
