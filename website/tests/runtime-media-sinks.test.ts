import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(relativePath: string): string {
	return readFileSync(new URL(`../${relativePath}`, import.meta.url), "utf8");
}

test("chat and notification image sinks normalize backend URLs before rendering", () => {
	const thread = source("features/account/screens/messages/ThreadClient.tsx");
	const toast = source("components/shared/NotificationToast.tsx");

	assert.match(thread, /const imageSrc = buildImageUrl\(url, "chat"\);/);
	assert.match(thread, /<img\s+src=\{imageSrc\}/);
	assert.doesNotMatch(thread, /<img\s+src=\{url\}/);

	assert.match(
		toast,
		/const imageSrc = buildImageUrl\(toast\.imageUrl, "thumb"\);/,
	);
	assert.match(toast, /<img\s+src=\{imageSrc\}/);
	assert.doesNotMatch(toast, /<img\s+src=\{toast\.imageUrl\}/);
});

test("shared upload previews and product gallery sinks normalize remote URLs but leave local blobs on their own path", () => {
	const uploader = source("components/ui/ImageUploader.tsx");
	const gallery = source("features/seller/components/ProductImageUploader.tsx");

	assert.match(uploader, /const previewUrl = resolveCdnUrl\(value\);/);
	assert.match(uploader, /<img src=\{previewUrl\}/);
	assert.doesNotMatch(uploader, /<img src=\{value as string\}/);

	assert.match(gallery, /src=\{buildImageUrl\(img\.url, "thumb"\)\}/);
	assert.match(gallery, /<img src=\{p\.previewBlob\}/);
});

test("seller cover, AI style, order review, and product form boundaries normalize backend media", () => {
	const sellerCover = source("features/seller/components/SellerCoverMedia.tsx");
	const aiDialog = source(
		"features/seller/components/AiImageEnhanceDialog.tsx",
	);
	const orderDetail = source(
		"features/account/screens/orders/OrderDetailClient.tsx",
	);
	const adapter = source(
		"features/seller-center/products/adapters/productFormAdapter.ts",
	);

	assert.match(sellerCover, /const imageSrc = buildImageUrl\(src, "banner"\);/);
	assert.match(aiDialog, /src=\{buildImageUrl\(s\.thumbnailUrl, "thumb"\)\}/);
	const aiRepo = source("lib/seller/ai-image.ts");
	assert.match(
		aiRepo,
		/thumbnailUrl: resolveStyleThumbnail\(s\.thumbnailUrl\)/,
	);
	assert.match(orderDetail, /src=\{resolveCdnUrl\(url\)\}/);
	assert.match(adapter, /const url = resolveCdnUrl\(toStr\(rec\.imageKey\)\);/);
	assert.match(
		adapter,
		/main_image: images\[0\]\?\.url \|\| resolveCdnUrl\(toStr\(r\.imageKey\)\),/,
	);
	assert.doesNotMatch(adapter, /Array\.isArray\(r\.images\)/);
	assert.doesNotMatch(adapter, /toStr\(raw\.main_image\)/);
});

test("seller and product metadata never emit raw backend image URLs", () => {
	const storefrontPage = source(
		"app/[locale]/(public)/sellers/[id]/(storefront)/page.tsx",
	);
	const storefrontMetadata = source("features/sellers/storefront/metadata.ts");
	const product = source("app/[locale]/(public)/products/[id]/page.tsx");

	assert.match(
		storefrontMetadata,
		/const socialImage = sellerSocialImage\(s\);[\s\S]*?images: socialImages\(socialImage\)/,
	);
	assert.match(storefrontPage, /image: sellerSocialImage\(s\)/);
	assert.doesNotMatch(
		`${storefrontMetadata}\n${storefrontPage}`,
		/images:[\s\S]{0,100}\{ url: s\.(?:coverImage|bannerUrl|avatar|logo_url)/,
	);

	for (const route of [product]) {
		assert.match(
			route,
			/const imgUrl = buildImageUrl\(product\.imageKey, "detail"\);/,
		);
		assert.doesNotMatch(route, /product\.images\?\.\[0\]/);
		assert.doesNotMatch(route, /const imgUrl = product\.imageKey/);
	}
	assert.match(
		product,
		/const image = buildImageUrl\(product\.imageKey, "detail"\);[\s\S]*?productSchema\(\{[\s\S]*?image,/,
	);
});
