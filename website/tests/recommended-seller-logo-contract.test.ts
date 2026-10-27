import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import Avatar from "../components/shared/Avatar";
import { recommendedSellerLogoUrl } from "../lib/seller/recommended-logo";

const projectFile = (path: string) => new URL(`../${path}`, import.meta.url);

test("recommended sellers use logoUrl as their only logo source", () => {
	const mixedPayload = {
		logoUrl: " https://media.example/seller.png ",
		logoKey: "public/ignored.png",
		logo: "https://legacy.example/ignored.png",
		logo_url: "https://snake.example/ignored.png",
		avatar: "https://avatar.example/ignored.png",
	};
	assert.equal(
		recommendedSellerLogoUrl(mixedPayload),
		"https://media.example/seller.png",
	);

	const legacyOnlyPayload = {
		logoKey: "public/legacy-only.png",
		logo: "https://legacy.example/legacy-only.png",
		logo_url: "https://snake.example/legacy-only.png",
		avatar: "https://avatar.example/legacy-only.png",
	};
	assert.equal(recommendedSellerLogoUrl(legacyOnlyPayload), "");
	assert.equal(recommendedSellerLogoUrl({ logoUrl: "   " }), "");
	assert.equal(recommendedSellerLogoUrl(null), "");
});

test("missing recommended logo renders an empty slot without an image or initial", () => {
	const markup = renderToStaticMarkup(
		createElement(Avatar, {
			src: "",
			name: "陳記",
			alt: "陳記",
			entity: "seller",
			// Even an accidental caller override cannot reintroduce a seller
			// placeholder; the shared Avatar contract forces sellers to empty.
			fallback: "placeholder",
			className: "seller-logo-slot",
		}),
	);

	assert.match(
		markup,
		/<span class="seller-logo-slot" aria-hidden="true"><\/span>/,
	);
	assert.doesNotMatch(markup, /<img/);
	assert.doesNotMatch(markup, /陳/);
	assert.doesNotMatch(markup, /seller-avatar|seller_logo/);
});

test("every recommended-seller consumer uses the canonical helper without image fallback", async () => {
	const [api, home, feed, market, preview, homeCards, sellerCard, storyRail] =
		await Promise.all([
			readFile(projectFile("lib/api/index.ts"), "utf8"),
			// The recommended-seller → card transform (toSeller / sellerQualityScore)
			// lives in the shared home snapshot builder so the server render can reuse
			// it; the canonical-helper contract is asserted against that module.
			readFile(projectFile("features/home/homeSnapshot.ts"), "utf8"),
			readFile(
				projectFile("features/feed/components/FeedClientShell.tsx"),
				"utf8",
			),
			readFile(
				projectFile("features/market/products/MarketProductsListPage.tsx"),
				"utf8",
			),
			readFile(
				projectFile(
					"features/seller-center/products-import/pages/PreviewListingPage.tsx",
				),
				"utf8",
			),
			readFile(
				projectFile("features/home/components/HomeCompactMerchants.tsx"),
				"utf8",
			),
			readFile(
				projectFile("features/market/components/MarketSellerCard.tsx"),
				"utf8",
			),
			readFile(projectFile("features/feed/components/StoryRail.tsx"), "utf8"),
		]);

	const marketplaceSeller = api.match(
		/export interface MarketplaceSeller \{([\s\S]*?)\n\}/,
	)?.[1];
	assert.ok(marketplaceSeller);
	assert.match(marketplaceSeller, /\blogoUrl\?: string/);
	assert.doesNotMatch(marketplaceSeller, /\blogo\?: string/);

	assert.match(home, /logo_url: recommendedSellerLogoUrl\(seller\)/);
	assert.match(home, /if \(recommendedSellerLogoUrl\(seller\)\) score \+= 1/);
	assert.doesNotMatch(home, /DEFAULT_SELLER_LOGO/);

	assert.match(feed, /avatar: recommendedSellerLogoUrl\(seller\)/);
	assert.match(market, /avatar: recommendedSellerLogoUrl\(seller\)/);
	assert.match(preview, /avatar: recommendedSellerLogoUrl\(s\)/);
	for (const source of [feed, market, preview]) {
		assert.doesNotMatch(source, /avatarFallback|DEFAULT_SELLER/);
	}
	assert.match(storyRail, /fallback="empty"/);

	for (const source of [homeCards, sellerCard]) {
		assert.match(source, /src=\{seller\.logo_url\}/);
		assert.match(source, /fallback="empty"/);
	}
});
