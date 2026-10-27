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

test("attachment picker normalizes product and order images before preview and chat serialization", () => {
	const picker = source("features/messages/components/AttachmentPicker.tsx");

	assert.equal(
		picker.match(/buildImageUrl\([\s\S]*?,\s*"thumb",?\s*\)/g)?.length,
		3,
	);
	assert.doesNotMatch(picker, /imageUrl:\s*firstItem\?\.image\s*\?\?\s*""/);
	assert.doesNotMatch(picker, /imageUrl:\s*pickString\(raw,/);
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
	// #430 D3-S12: the slot is filled from the wire's `thumbnailKey` (objectKey
	// SSOT), not its Gateway-composed `thumbnailUrl` twin.
	const aiRepo = source("lib/seller/ai-image.ts");
	assert.match(
		aiRepo,
		/thumbnailUrl: resolveStyleThumbnail\(s\.thumbnailKey as string\)/,
	);
	assert.doesNotMatch(aiRepo, /s\.thumbnailUrl as string/);
	assert.match(orderDetail, /src=\{resolveCdnUrl\(url\)\}/);
	// 圖庫與主圖都由 objectKey 經 resolveCdnUrl 組 URL（49 §6.1／§6.5）。
	// #430 A1 之前這裡釘的是「讀 r.images／raw.main_image」的舊寫法——那兩個欄位後端
	// 從來沒有回過，於是這條斷言替一個壞掉的邊界背書。改釘實際存在的來源欄位。
	// backend#2235：gallery 條目的圖片欄從 `objectKey`/`url` 收斂成
	// `imageKey`/`imageUrl`（與商品卡、購物車項、貼文商品同名，49 §6.1）。
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
	const nestedProduct = source(
		"app/[locale]/(public)/sellers/[id]/products/[pid]/page.tsx",
	);

	// The pinned value is the NORMALIZED one, and it reaches og:image through
	// socialImages() so an imageless seller falls back to the site card instead
	// of the empty array this used to pin (tests/seo-social-image.test.ts).
	// Which FIELD it normalizes is `sellerSocialImage`'s business and is pinned
	// once, for all four seller routes, in tests/seller-social-image.test.ts.
	assert.match(
		storefrontMetadata,
		/const socialImage = sellerSocialImage\(s\);[\s\S]*?images: socialImages\(socialImage\)/,
	);
	assert.match(storefrontPage, /image: sellerSocialImage\(s\)/);
	assert.doesNotMatch(
		`${storefrontMetadata}\n${storefrontPage}`,
		/images:[\s\S]{0,100}\{ url: s\.(?:coverImage|bannerUrl|avatar|logo_url)/,
	);

	// #430 D4-R3/R4/R5: `images` was never on the wire (PublicProductDetailItem
	// has imageKey / imageKeys / imageUrl / imageUrls and nothing else), so the
	// old pin endorsed a dead fallback. Pin the objectKey SSOT instead — passing
	// the bare key lets buildImageUrl apply the `detail` preset directly rather
	// than peeling a `/i/original/` URL back apart first.
	for (const route of [product, nestedProduct]) {
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
