import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("seller posts keep the desktop feed and expose one shared phone disclosure list", () => {
	const posts = source("features/seller-center/posts/SellerPostsClient.tsx");

	assert.match(posts, /seller-posts__card seller-mobile-disclosure-desktop/);
	assert.match(posts, /<SellerMobileDisclosureList/);
	assert.match(posts, /getKey=\{postKey\}/);
	assert.match(
		posts,
		/<MobilePostThumbnail post=\{post\} enabled=\{showMobileMedia\} \/>/,
	);
	assert.match(posts, /title: postFirstLine\(post\)/);
	assert.match(posts, /meta: postInteractionMeta\(post, delegated\)/);
	assert.match(posts, /status: <MobilePostStatus post=\{post\} \/>/);
	for (const name of ["heart", "chat", "share", "show"]) {
		assert.match(
			posts,
			new RegExp(`<AppIcon name="${name}" size=\\{16\\}`),
			`mobile details must preserve the existing ${name} icon`,
		);
	}
	assert.match(posts, />\s*查看完整動態\s*</);
	assert.match(posts, /<PostPublishControl[\s\S]*?onToggle=/);
	assert.match(
		posts,
		/href=\{`\/seller-center\/posts\/\$\{post\.id\}\/edit`\}/,
	);
	assert.match(posts, /onClick=\{\(\) => void handleDelete\(post\.id\)\}/);
});

test("seller posts mount only the active responsive image tree and fall back cleanly", () => {
	const posts = source("features/seller-center/posts/SellerPostsClient.tsx");
	const modal = source(
		"features/seller-center/posts/SellerPostDetailModal.tsx",
	);

	assert.match(
		posts,
		/import AppImage from "@\/components\/shared\/AppImage"/,
		"seller post media must reuse the shared error fallback",
	);
	assert.match(posts, /useSellerPostsViewport\(\)/);
	assert.match(posts, /showDesktopMedia && p\.imageKeys\[0\]/);
	assert.match(
		posts,
		/<MobilePostThumbnail post=\{post\} enabled=\{showMobileMedia\} \/>/,
	);
	assert.match(posts, /deferDetailsUntilExpanded/);
	assert.doesNotMatch(
		posts,
		/<img\b/,
		"raw images expose the browser broken-image UI instead of ImageFallback",
	);
	assert.match(
		modal,
		/import AppImage from "@\/components\/shared\/AppImage"/,
		"the full post modal must use the same media fallback",
	);
	assert.doesNotMatch(modal, /<img\b/);
});

test("storefront preview folds only its seller notice and preserves the real buyer subtree", () => {
	const layout = source(
		"app/[locale]/(seller)/seller-center/preview/[id]/layout.tsx",
	);
	const banner = source(
		"features/seller-center/preview/SellerStorefrontPreviewBanner.tsx",
	);

	assert.match(layout, /<SellerStorefrontPreviewBanner \/>/);
	assert.match(
		layout,
		/<StorefrontPreviewBoundary sellerId=\{id\}>\s*\{children\}\s*<\/StorefrontPreviewBoundary>/,
	);
	assert.match(banner, /seller-mobile-disclosure-desktop/);
	assert.match(banner, /<SellerMobileDisclosureList/);
	assert.match(banner, /title: "買家視角預覽"/);
	assert.match(banner, /status: "唯讀"/);
	assert.match(layout, /<h1 className="sr-only">買家視角預覽<\/h1>/);
	assert.doesNotMatch(banner, /<h1\b/);
	assert.match(banner, /使用正式公開商店資料/);
	assert.match(banner, /href="\/seller-center\/me"/);
	assert.match(banner, /product, post, forum, or review/);
	assert.doesNotMatch(banner, /MarketProductCard|PostCardV2|ReviewList/);
});
