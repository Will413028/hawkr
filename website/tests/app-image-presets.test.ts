import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("AppImage preserves the caller's named media preset", () => {
	const appImage = source("components/shared/AppImage.tsx");

	assert.match(appImage, /preset\?: ImagePreset/);
	assert.match(appImage, /preset = "original"/);
	assert.match(appImage, /buildImageUrl\(src, preset\)/);
	assert.doesNotMatch(appImage, /resolveCdnUrl\(src\)/);
});

test("initial market and search cards request bounded card derivatives", () => {
	const cardSurfaces = ["features/market/components/MarketProductCard.tsx"];

	for (const relativePath of cardSurfaces) {
		assert.match(
			source(relativePath),
			/preset="card"/,
			`${relativePath} must request the card derivative`,
		);
	}

	assert.match(
		source("features/home/components/HomeCompactMarketStage.tsx"),
		/preset="thumb"/,
	);
});

test("post thumbnails select a deployed media preset for each media type", () => {
	assert.match(
		source("features/feed/components/PostCardV2.tsx"),
		/preset=\{postCardPresetForMedia\(imageSrc\)\}/,
	);
	for (const relativePath of [
		"features/seller-center/posts/SellerPostsClient.tsx",
		"features/seller-center/posts/SellerPostDetailModal.tsx",
	]) {
		assert.match(source(relativePath), /postCardPresetForMedia\(/);
	}
});

test("home origin stories preserve still-image ratios and GIF animation", () => {
	const originStory = source("features/home/components/HomeStageFeedCard.tsx");

	assert.match(
		originStory,
		/preset=\{postCardPresetForMedia\(imageSrc, "detail"\)\}/,
	);
	// 6ea7a4e3 起照片改由自身尺寸決定（intrinsic width/height + max-w/max-h），
	// 不再是 fill 填滿固定框。斷言收斂成本測真正要守的那件事——still image 仍走
	// object-contain 不被裁切——而不是整串 className 的字面值。
	assert.match(originStory, /max-h-full w-auto max-w-full[^"]*object-contain/);
	assert.match(
		originStory,
		/grid-cols-\[minmax\(0,min\(48%,380px\)\)_minmax\(0,1fr\)\]/,
	);
	assert.doesNotMatch(originStory, /grid-cols-\[auto_minmax\(0,1fr\)\]/);
	assert.match(
		originStory,
		/relative min-w-0 w-full aspect-\[4\/3\] h-full overflow-hidden/,
	);
	assert.match(
		originStory,
		/className="relative flex min-w-0 flex-col px-6 py-5"/,
	);
	assert.doesNotMatch(originStory, /aspect-square|aspect-auto/);
	assert.doesNotMatch(originStory, /preset="card"/);
	assert.doesNotMatch(originStory, /className="size-full object-cover"/);
});

test("seller covers and campaign banners retain the banner derivative", () => {
	const bannerSurfaces = [
		["features/seller/components/SellerCoverMedia.tsx", 2],
		// Recommended-seller cards render the same canonical 4:1 cover as the
		// storefront. The 4:3 `card` transform would crop it before layout.
		["features/home/components/HomeCompactMerchants.tsx", 1],
		["components/shared/banner/BannerCarousel.tsx", 1],
		["components/shared/banner/BannerPopupDialog.tsx", 1],
	] as const;

	for (const [relativePath, expectedCount] of bannerSurfaces) {
		assert.equal(
			source(relativePath).match(/preset="banner"/g)?.length,
			expectedCount,
			`${relativePath} must not let AppImage rewrite banner URLs to original`,
		);
	}
});

test("compact banner frame follows the loaded image ratio without cropping", () => {
	const carousel = source("components/shared/banner/BannerCarousel.tsx");

	assert.match(carousel, /image\.naturalWidth \/ image\.naturalHeight/);
	assert.match(carousel, /aspectRatio: String\(compactAspectRatio\)/);
	assert.match(
		carousel,
		/size === "hero" \? "object-cover" : "object-contain"/,
	);
});

test("seller touch icons only fetch from the configured media origin", () => {
	const appleIcon = source("app/[locale]/(public)/sellers/[id]/apple-icon.tsx");

	assert.match(appleIcon, /isConfiguredMediaUrl\(url\)/);
	assert.doesNotMatch(appleIcon, /ALLOWED_AVATAR_HOSTS|\.example\.com/);
});
