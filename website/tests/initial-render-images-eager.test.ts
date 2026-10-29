import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

function imageOpeningTag(relativePath: string, marker: string): string {
	const text = source(relativePath);
	const markerIndex = text.indexOf(marker);
	assert.notEqual(
		markerIndex,
		-1,
		`${relativePath} must keep the ${marker} image surface`,
	);

	const tagStart = Math.max(
		text.lastIndexOf("<AppImage", markerIndex),
		text.lastIndexOf("<Avatar", markerIndex),
		text.lastIndexOf("<Image", markerIndex),
		text.lastIndexOf("<img", markerIndex),
	);
	const tagEnd = text.indexOf("/>", markerIndex);
	assert.ok(
		tagStart >= 0 && tagEnd > markerIndex,
		`${relativePath} must keep ${marker} inside an image opening tag`,
	);
	return text.slice(tagStart, tagEnd + 2);
}

function assertEager(relativePath: string, marker: string): void {
	const tag = imageOpeningTag(relativePath, marker);
	assert.match(
		tag,
		/\bloading="eager"/,
		`${relativePath} ${marker} must begin loading with the initial render`,
	);
	assert.doesNotMatch(
		tag,
		/\bloading=(?:"lazy"|'lazy'|\{"lazy"\})/,
		`${relativePath} ${marker} must never defer loading until scroll`,
	);
}

function assertSellerAvatarEagerEmpty(
	relativePath: string,
	marker: string,
): void {
	const tag = imageOpeningTag(relativePath, marker);
	assertEager(relativePath, marker);
	assert.match(tag, /\bentity="seller"/);
	assert.match(tag, /\bfallback="empty"/);
}

test("PDP gallery prioritizes reusable previews and nearby detail while other detail media stays eager", () => {
	const galleryPath = "features/product/components/ProductGallery.tsx";
	const gallery = source(galleryPath);
	assert.match(gallery, /<GalleryImage[\s\S]*?first=\{index === 0\}/);
	assert.match(gallery, /nearby=\{Math\.abs\(index - activeIndex\) <= 1\}/);
	assert.match(
		gallery,
		/<GalleryImage[\s\S]*?active=\{index === activeIndex\}/,
	);
	const galleryMain = imageOpeningTag(galleryPath, "data-gallery-preview");
	assert.match(galleryMain, /priority=\{first\}/);
	assert.match(
		galleryMain,
		/loading=\{first \? undefined : nearby \? "eager" : "lazy"\}/,
	);
	assert.match(galleryMain, /fetchPriority=\{active \? "high" : "low"\}/);
	assert.match(galleryMain, /src=\{image\}/);
	assert.match(
		galleryMain,
		/preset=\{postCardPresetForMedia\(image, "card"\)\}/,
	);
	assert.doesNotMatch(galleryMain, /\bpreload=/);

	const galleryDetail = imageOpeningTag(galleryPath, "data-gallery-detail");
	assert.match(gallery, /needsLargerImage && \(\s*<GalleryDetailImage/);
	assert.match(galleryDetail, /preset=\{preset\}/);
	assert.match(galleryDetail, /loading=\{nearby \? "eager" : "lazy"\}/);
	assert.match(galleryDetail, /fetchPriority=\{active \? "high" : "low"\}/);
	assert.doesNotMatch(galleryDetail, /\bpriority(?:=|\s)|\bpreload=/);

	const galleryThumb = imageOpeningTag(galleryPath, "src={thumb}");
	assert.match(
		galleryThumb,
		/preset=\{postCardPresetForMedia\(thumb, "card"\)\}/,
	);
	assert.match(galleryThumb, /loading="lazy"/);
	assert.match(galleryThumb, /fetchPriority="low"/);
	assertEager(
		"features/product/ProductDetailLayout.tsx",
		"src={resolveCdnUrl(rawSrc)}",
	);
	assertEager(
		"features/product/components/ProductReviewsSection.tsx",
		"src={textOrEmpty(review.avatar)}",
	);
	assertEager(
		"features/product/components/ProductReviewsSection.tsx",
		"src={photoSrc}",
	);
	assertSellerAvatarEagerEmpty(
		"features/product/components/ProductSellerIntro.tsx",
		"src={avatarSrc}",
	);
});

test("buyer list, cart, checkout, and order media never waits for scroll", () => {
	const surfaces = [
		[
			"features/account/screens/favorites/FavoritesClient.tsx",
			'src={productImage || ""}',
		],
		["features/account/screens/recent/RecentClient.tsx", "src={resolveCdnUrl("],
		["features/account/screens/orders/OrdersClient.tsx", "src={coverSrc}"],
		[
			"features/account/screens/reviews/PendingReviewsList.tsx",
			"src={resolveCdnUrl(cover)}",
		],
		[
			"features/account/screens/reviews/ReviewedList.tsx",
			"src={resolveCdnUrl(r.productImageKey)}",
		],
		[
			"features/account/screens/reviews/ReviewedList.tsx",
			"src={resolveCdnUrl(img)}",
		],
		["features/cart/components/CartProductTile.tsx", "src={imageSrc}"],
		[
			"features/cart/CartClient.tsx",
			'src={ossImageUrl(item.image, "productThumb")}',
		],
		[
			"features/checkout/screens/CheckoutClient.tsx",
			"src={resolveCdnUrl(src)}",
		],
		[
			"features/account/screens/orders/OrderDetailClient.tsx",
			"src={resolveCdnUrl(it.image)}",
		],
	] as const;

	for (const [relativePath, marker] of surfaces) {
		assertEager(relativePath, marker);
	}

	const sellerAvatars = [
		[
			"features/account/screens/favorites/FavoritesClient.tsx",
			"src={sellerLogo}",
		],
		[
			"features/account/screens/recent/RecentClient.tsx",
			"src={storeView.logoUrl}",
		],
		[
			"features/cart/components/SellerGroupCard.tsx",
			"src={group.sellerLogoUrl}",
		],
		["features/checkout/screens/CheckoutClient.tsx", "src={src}"],
		[
			"features/account/screens/orders/OrderDetailClient.tsx",
			"src={order.sellerLogo}",
		],
	] as const;

	for (const [relativePath, marker] of sellerAvatars) {
		assertSellerAvatarEagerEmpty(relativePath, marker);
	}
});

test("seller management list media never waits for scroll", () => {
	const surfaces = [
		[
			"features/seller-center/products/ProductsPage.tsx",
			'src={ossImageUrl(p.main_image, "productThumb")}',
		],
		[
			"features/seller-center/posts/SellerPostsClient.tsx",
			"postCardPresetForMedia(p.imageKeys[0])",
		],
		["features/seller-center/customers/CustomersClient.tsx", "src={c.avatar}"],
	] as const;

	for (const [relativePath, marker] of surfaces) {
		assertEager(relativePath, marker);
	}
});
