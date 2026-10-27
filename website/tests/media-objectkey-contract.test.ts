/**
 * Media contract guard — hawkr-website#430 D2 / D3 / D4.
 *
 * Every fixture in this file is a VERBATIM slice of a live dev response
 * (`https://test-api.example.com/api/v1`, captured 2026-08-05), never a
 * hand-written wire shape. That distinction is the whole point of the file:
 * #430 exists because a normalizer read `r.images`, a unit test fed it a
 * fabricated `images: [...]`, the test went green, and the seller's product
 * gallery was empty in production for months.
 *
 * Two things are asserted for each surface:
 *
 *   1. **Pair discipline (§231 / 49 §6.1)** — the wire sends `xxxKey` (the bare
 *      objectKey, SSOT) beside `xxxUrl` (the URL the Gateway composed from it).
 *      The normalizer must read the KEY. Both members are present in the
 *      fixture, so an edit that goes back to the URL turns this red.
 *
 *   2. **No guessing** — a payload carrying ONLY the field names that were
 *      deleted in #430 must produce nothing. Re-introducing any candidate as a
 *      fallback turns this red, whatever priority it is given.
 */

import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	FavoriteSchema,
	RestockSubscriptionSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { buildHomeSnapshot } from "../features/home/homeSnapshot";
import { deriveDetailLayoutProps } from "../features/product/deriveDetailLayoutProps";
import { normalizeDetail } from "../lib/after-sales/normalize";
import { flattenGroups } from "../lib/cart/types";
import { normalizeConversation } from "../lib/chat/repo";
import { mapProduct } from "../lib/market/listFilters";
import { imageUri } from "../lib/media/image";
import { normalizeMemberCard } from "../lib/member-cards/repo";
import { normalizeOrder } from "../lib/orders/repo";
import { restockFromConnect } from "../lib/restock-subscriptions/repo";
import { normalizeSellerForumList } from "../lib/seller/forum-repo";

/* ------------------------------------------------------------------ orders */

// GET /app/orders?view=seller — one live record, trimmed to the media fields.
const LIVE_SELLER_ORDER = {
	orderNo: "O578806254",
	status: "50",
	buyerId: "7489760880179860732",
	buyerNickname: "林小明",
	avatarKey:
		"public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png",
	avatarUrl:
		"https://test-media.example.com/i/original/public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png",
	logoKey: "public/defaults/seller_logo.png",
	logoUrl:
		"https://test-media.example.com/i/original/public/defaults/seller_logo.png",
	shippingAddress: "台北市中正區測試路1號",
	items: [
		{
			id: "870450039209774211",
			productId: "866353737479521547",
			productName: "R8-01 媒體回歸測試商品（陳記）",
			skuId: "308",
			skuName: "預設規格",
			quantity: "2",
			unitPrice: "120",
			imageKey:
				"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
			imageUrl:
				"https://test-media.example.com/i/original/public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
		},
	],
};

test("order item + buyer avatar read the objectKey, not the composed URL", () => {
	const order = normalizeOrder(LIVE_SELLER_ORDER);

	assert.equal(
		order.items[0].image,
		"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
	);
	assert.equal(
		order.buyerAvatar,
		"public/buyer_avatar/2623900520/20260719/008b97af-544d-4efb-8bc4-8f11e2b4e027.png",
	);
});

test("orders ignore the item/root/nested image names the wire has never carried", () => {
	// D2 + D4-R10/R11/R12. `skuImage…cover` (12 names) were the item candidate
	// list; `image…cover` at the ROOT was `rootImage`; `buyer.headImg/profileImage/
	// logo` was a nested object `OrderDetailResponse` does not have at all.
	const order = normalizeOrder({
		orderNo: "O-guessing",
		status: "10",
		// former root-image candidates
		image: "https://cdn/root-image.jpg",
		img: "https://cdn/root-img.jpg",
		imageUrl: "https://cdn/root-imageurl.jpg",
		pic: "https://cdn/root-pic.jpg",
		picUrl: "https://cdn/root-picurl.jpg",
		cover: "https://cdn/root-cover.jpg",
		// former nested buyer object
		buyer: {
			name: "巢狀買家",
			avatar: "https://cdn/nested-avatar.jpg",
			headImg: "https://cdn/nested-headimg.jpg",
			profileImage: "https://cdn/nested-profile.jpg",
			logo: "https://cdn/nested-logo.jpg",
		},
		// the subject-prefixed pair the wire carried until backend#2216 flattened
		// every buyer-avatar field to `avatarKey`/`avatarUrl`. Reading either of
		// these again — as a fallback or otherwise — turns this red.
		buyerAvatarKey: "public/buyer_avatar/2623900520/retired-key.png",
		buyerAvatarUrl: "https://cdn/retired-url.png",
		// former per-item candidate list
		items: [
			{
				id: "1",
				skuImage: "https://cdn/skuimage.jpg",
				img: "https://cdn/img.jpg",
				imageUrl: "https://cdn/imageurl.jpg",
				pic: "https://cdn/pic.jpg",
				spuImage: "https://cdn/spuimage.jpg",
				goodsImg: "https://cdn/goodsimg.jpg",
				thumbnail: "https://cdn/thumbnail.jpg",
				cover: "https://cdn/cover.jpg",
			},
		],
	});

	assert.equal(order.items[0].image, undefined);
	assert.equal(order.buyerAvatar, undefined);
	assert.equal(order.buyerName, undefined);
});

test("shipment / receipt photos come from the *Keys arrays", () => {
	const order = normalizeOrder({
		orderNo: "O-photos",
		status: "50",
		shipmentPhotoKeys: ["public/shipment/a.png"],
		shipmentPhotoUrls: [
			"https://test-media.example.com/i/original/public/shipment/a.png",
		],
		receiptPhotoKeys: ["public/shipment/b.png"],
		receiptPhotoUrls: [
			"https://test-media.example.com/i/original/public/shipment/b.png",
		],
	});

	assert.deepEqual(order.shipping?.shipmentPhotoUrls, [
		"public/shipment/a.png",
	]);
	assert.deepEqual(order.shipping?.receiptPhotoUrls, ["public/shipment/b.png"]);
});

/* -------------------------------------------------------------------- cart */

// GET /public/carts → groups[0].items[0], verbatim.
test("cart item image comes from imageKey; `image` stays a LOCAL-cache-only field", () => {
	const [item] = flattenGroups([
		{
			storeId: "2685914682",
			sellerName: "陳記",
			items: [
				{
					skuId: "308",
					quantity: "2",
					name: "R8-01 媒體回歸測試商品（陳記）",
					price: "120",
					status: "0",
					productId: "866353737479521547",
					specName: "預設規格",
					imageKey:
						"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
					imageUrl:
						"https://test-media.example.com/i/original/public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
				},
			],
		},
	]);

	assert.equal(
		item.image,
		"public/product/2685914682/20260718/f8d18047-9b3a-4c1b-8a21-7b8f2fdabecc.png",
	);

	// A guest cart persisted before the wire rename only has `image`; that path
	// is a localStorage compatibility need, not a backend field guess.
	const [legacy] = flattenGroups([
		{
			sellerName: "陳記",
			items: [
				{
					skuId: "308",
					quantity: "1",
					name: "x",
					price: "1",
					image: "public/product/legacy-local-cache.png",
				},
			],
		},
	]);
	assert.equal(legacy.image, "public/product/legacy-local-cache.png");
});

/* -------------------------------------------------------------- after-sale */

// GET /app/after_sales/A724174691 — evidence[] + items[], verbatim.
test("after-sale evidence and items read imageKey", () => {
	const detail = normalizeDetail({
		caseNo: "A724174691",
		status: "40",
		evidence: [
			{
				assetId: "872389366881812552",
				imageKey: "public/aftersale/745405000/20260804/762fd921.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/aftersale/745405000/20260804/762fd921.png",
				evidenceType: "8601",
				createdAt: "2026-08-04T08:03:07.363959Z",
			},
		],
		items: [
			{
				orderItemId: "871950810292079207",
				skuId: "336",
				skuName: "測試規格",
				quantity: "1",
				imageKey: "public/product/aftersale-item.png",
				imageUrl:
					"https://test-media.example.com/i/original/public/product/aftersale-item.png",
			},
		],
	});

	assert.deepEqual(
		detail.photos.map((p) => p.url),
		["public/aftersale/745405000/20260804/762fd921.png"],
	);
	assert.equal(detail.items[0].image, "public/product/aftersale-item.png");
});

test("after-sale ignores evidences/photos/photoUrls and url/previewUrl", () => {
	// D4-R13/R14: `AfterSaleCaseDetailResponse` only has `evidence`, and an
	// `EvidenceResponse` only has assetId / imageKey / imageUrl / evidenceType /
	// createdAt.
	const viaDeadContainer = normalizeDetail({
		caseNo: "A-guess",
		evidences: [{ imageKey: "public/a.png" }],
		photos: [{ imageKey: "public/b.png" }],
		photoUrls: [{ imageKey: "public/c.png" }],
	});
	assert.deepEqual(viaDeadContainer.photos, []);
	assert.deepEqual(viaDeadContainer.sellerPhotos, []);

	const viaDeadField = normalizeDetail({
		caseNo: "A-guess-2",
		evidence: [
			{ url: "https://cdn/url.png" },
			{ previewUrl: "https://cdn/preview.png" },
		],
		items: [{ orderItemId: "1", image: "https://cdn/x.png", cover: "y.png" }],
	});
	assert.deepEqual(viaDeadField.photos, []);
	assert.equal(viaDeadField.items[0].image, undefined);
});

/* ------------------------------------------------------ list-shaped repos */

test("member card cover/logo read the *Key members; `cover`/`banner` are gone", () => {
	// GET /app/buyers/me/member-cards → records[0], verbatim.
	const card = normalizeMemberCard({
		seller_id: "2685914682",
		seller_name: "陳記",
		store_code: "2662192214",
		coverKey: "public/seller_cover/2685914682/38d9f99d.webp",
		coverUrl:
			"https://test-media.example.com/i/original/public/seller_cover/2685914682/38d9f99d.webp",
		logoKey: "public/user_avatar/2685914682/2c7c6d22.png",
		logoUrl:
			"https://test-media.example.com/i/original/public/user_avatar/2685914682/2c7c6d22.png",
	});
	assert.equal(card.coverUrl, "public/seller_cover/2685914682/38d9f99d.webp");

	assert.equal(
		normalizeMemberCard({
			seller_id: "x",
			cover: "https://cdn/cover.png",
			banner: "https://cdn/banner.png",
		}).coverUrl,
		"",
	);
});

test("generated favorite resolves its image URI template", () => {
	const fav = create(FavoriteSchema, {
		product: {
			id: "866353737479521547",
			name: "R8-01 媒體回歸測試商品（陳記）",
			image: { uriTemplate: "https://media.example/i/{preset}/product.png" },
		},
	});
	assert.equal(
		imageUri(fav.product?.image, "card"),
		"https://media.example/i/card/product.png",
	);
	assert.equal(
		imageUri(
			create(FavoriteSchema, { product: { id: "9" } }).product?.image,
			"card",
		),
		undefined,
	);
});

test("chat conversation avatar reads avatarKey, not the URL twin", () => {
	// GET /app/chat/conversations → conversations[0], verbatim.
	const conv = normalizeConversation({
		targetUserId: 2636840881,
		targetNickname: "Cora Chou",
		avatarKey: "public/buyer_avatar/2636840881/abc.png",
		avatarUrl:
			"https://test-media.example.com/i/original/public/buyer_avatar/2636840881/abc.png",
	});
	assert.equal(conv.targetAvatar, "public/buyer_avatar/2636840881/abc.png");

	assert.equal(
		normalizeConversation({
			targetUserId: 1,
			avatarUrl: "https://cdn/only-the-url.png",
		}).targetAvatar,
		undefined,
	);

	// The subject-prefixed pair retired by backend#2216. A payload carrying only
	// those must resolve to nothing — any revived fallback turns this red.
	assert.equal(
		normalizeConversation({
			targetUserId: 1,
			targetAvatarKey: "public/buyer_avatar/2636840881/retired.png",
			targetAvatarUrl: "https://cdn/retired.png",
		}).targetAvatar,
		undefined,
	);
});

test("seller forum avatar reads avatarKey; logo/header/avatar are gone", () => {
	// GET /public/sellers/{id}/forum → records[0], verbatim.
	const list = normalizeSellerForumList({
		records: [
			{
				id: "7490313091974749192",
				isMine: false,
				userName: "林小明",
				content: "avatar-fix 驗證留言 A",
				avatarKey: "public/buyer_avatar/2623900520/008b97af.png",
				avatarUrl:
					"https://test-media.example.com/i/original/public/buyer_avatar/2623900520/008b97af.png",
			},
			{
				id: "2",
				isMine: false,
				userName: "猜欄位",
				content: "x",
				logo: "https://cdn/logo.png",
				header: "https://cdn/header.png",
				avatar: "https://cdn/avatar.png",
				// retired by backend#2216 — reading either again turns this red
				userAvatarKey: "public/buyer_avatar/3/retired.png",
				userAvatarUrl: "https://cdn/retired.png",
			},
		],
	});

	assert.equal(
		list.records[0].avatarKey,
		"public/buyer_avatar/2623900520/008b97af.png",
	);
	assert.equal(list.records[1].avatarKey, "");
});

/* ------------------------------------------- product cards (#430 D4 residual)
 *
 * The four §231 product-image fields are NOT four independent candidates. The
 * gateway composes the URL twins from the very keys we already read —
 *   ImageURL  = mediaurl.BuildURL(base, p.ImageKey)
 *   ImageUrls = buildMediaURLs(base, p.ImageKeys)
 * (product.go:323-326 / 1119-1123, search.go:226-229) — so `imageUrl`/
 * `imageUrls` can never carry an image the keys lack, and `image`/`images` are
 * not on the wire at all. Every card mapper reads the key and only the key.
 *
 * Live-verified 2026-08-05 across /public/products, /public/marketplace,
 * /public/search/products, /public/sellers/{id}/products, /app/products and
 * /public/products/{id}: imageKey === imageKeys[0] and imageKey empty ⟺
 * imageUrl empty, in 30/30 records.
 */

// GET /public/products?page=1&page_size=10 → the first record with imagery,
// verbatim (media + identity fields only).
const LIVE_PRODUCT_CARD = {
	id: "871936508549696815",
	name: "高麗菜",
	origin: "新北市",
	status: "4004",
	stock: "108",
	priceMin: "111",
	priceMax: "111",
	storeId: "7489859548420286732",
	sellerName: "測試",
	imageKey:
		"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg",
	imageUrl:
		"https://test-media.example.com/i/original/public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg",
	imageKeys: [
		"public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg",
	],
	imageUrls: [
		"https://test-media.example.com/i/original/public/product/2644305813/20260803/fc710bd6-2c52-42a8-8055-c58d5e82dbdf.jpg",
	],
};

/**
 * The same live record with its two surviving image fields swapped for every
 * name #430 D4 retired, all populated at once. Identity/price fields are kept
 * so the card passes the same reality filters as the live one — the ONLY
 * difference under test is which field name carries the image.
 */
const RETIRED_ONLY_PRODUCT = {
	...LIVE_PRODUCT_CARD,
	imageKey: undefined,
	imageKeys: undefined,
	image: "public/product/retired-image.jpg",
	images: ["public/product/retired-images.jpg"],
	imageUrl:
		"https://test-media.example.com/i/original/public/product/retired-imageurl.jpg",
	imageUrls: [
		"https://test-media.example.com/i/original/public/product/retired-imageurls.jpg",
	],
};

test("market list card reads the objectKey, not the composed URL", () => {
	assert.equal(
		mapProduct(LIVE_PRODUCT_CARD, 0).image,
		LIVE_PRODUCT_CARD.imageKey,
	);
});

test("market list card yields no image when only the retired names are present", () => {
	assert.equal(mapProduct(RETIRED_ONLY_PRODUCT, 0).image, "");
});

test("home snapshot cards read the objectKey, not the composed URL", () => {
	const snapshot = buildHomeSnapshot({
		schemaVersion: "home.v1",
		marketplace: null,
		products: { records: [LIVE_PRODUCT_CARD] },
		recommendedSellers: [],
		posts: { records: [], total: 0 },
	} as never);
	assert.equal(snapshot.newArrivalProducts.length, 1);
	assert.equal(
		snapshot.newArrivalProducts[0].image,
		LIVE_PRODUCT_CARD.imageKey,
	);
});

test("home snapshot yields no image when only the retired names are present", () => {
	const snapshot = buildHomeSnapshot({
		schemaVersion: "home.v1",
		marketplace: null,
		products: { records: [RETIRED_ONLY_PRODUCT] },
		recommendedSellers: [],
		posts: { records: [], total: 0 },
	} as never);
	assert.equal(snapshot.newArrivalProducts.length, 1);
	assert.equal(snapshot.newArrivalProducts[0].image, "");
});

test("PDP thumbnails read imageKeys, not the composed URL arrays", () => {
	const derived = deriveDetailLayoutProps({
		product: { ...LIVE_PRODUCT_CARD },
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(derived.thumbnails, LIVE_PRODUCT_CARD.imageKeys);
});

test("PDP thumbnails stay empty when only the retired names are present", () => {
	const derived = deriveDetailLayoutProps({
		product: { ...RETIRED_ONLY_PRODUCT },
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(derived.thumbnails, []);
});

test("PDP treats reviewCount as authoritative even when embedded reviews disagree", () => {
	const derived = deriveDetailLayoutProps({
		product: {
			reviewCount: "0",
			reviews: [{ id: "preview-only", rating: 5, content: "preview" }],
		},
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.equal(derived.reviewTotal, 0);
});

test("PDP exposes backend purchasable as the purchase-box authority", () => {
	const derived = deriveDetailLayoutProps({
		product: { purchasable: false },
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		descriptionImageDimEntries: [],
	});
	assert.equal(derived.purchasable, false);
});

test("PDP review photos read the normalizer's field, not wire names", () => {
	// reviewsResult arrives already normalized (lib/reviews/repo.ts). Since
	// backend#2235 the normalized shape mirrors the wire field-for-field, so
	// `imageKeys` is the name on BOTH sides — feeding it must produce the photo.
	// The retired alias `images` is asserted dead in the second case below.
	const derived = deriveDetailLayoutProps({
		product: {},
		seller: null,
		sellerStats: null,
		reviewsResult: {
			total: 1,
			reviews: [
				{
					id: "1",
					rating: 5,
					content: "x",
					imageKeys: ["public/order_review/wire-key.png"],
					imageUrls: ["https://test-media.example.com/i/original/x.png"],
				},
			],
		},
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(derived.reviews[0].photos, [
		"public/order_review/wire-key.png",
	]);

	// The pre-#2235 normalized name. Reading it again would silently resurrect a
	// second spelling for the same list.
	const viaRetiredName = deriveDetailLayoutProps({
		product: {},
		seller: null,
		sellerStats: null,
		reviewsResult: {
			total: 1,
			reviews: [
				{
					id: "1",
					rating: 5,
					content: "x",
					images: ["public/order_review/retired-name.png"],
				},
			],
		},
		descriptionImageDimEntries: [],
	});
	assert.deepEqual(viaRetiredName.reviews[0].photos, []);
});

/* ------------------------------ derived-URL twins are not fallbacks (#430 D4)
 *
 * These three surfaces each read `xxxKey ?? xxxUrl`. The second operand is
 * `buildURL(base, xxxKey)` on the gateway side (favorite.go:92-93,
 * restock_subscription.go:145-146, member_card.go:77-78) — composed from the
 * very key being read — so a payload that carries ONLY the URL is a shape the
 * gateway cannot emit, and honouring it could only ever mask a contract break.
 */

test("generated favorite without an Image has no product image", () => {
	const fav = create(FavoriteSchema, { product: { id: "3" } });
	assert.equal(imageUri(fav.product?.image, "card"), undefined);
});

test("member card ignores a coverUrl that arrives without its key", () => {
	const card = normalizeMemberCard({
		sellerId: "2685914682",
		coverUrl:
			"https://test-media.example.com/i/original/public/seller_cover/orphan-url.webp",
	});
	assert.equal(card.coverUrl, "");
});

test("generated restock subscription resolves its image URI template", () => {
	const record = restockFromConnect(
		create(RestockSubscriptionSchema, {
			product: {
				id: "866353737479521547",
				image: { uriTemplate: "https://media.example/i/{preset}/product.png" },
			},
		}),
	);
	assert.equal(record.productImage, "https://media.example/i/card/product.png");
	assert.equal(
		restockFromConnect(
			create(RestockSubscriptionSchema, { product: { id: "3" } }),
		).productImage,
		"",
	);
});
