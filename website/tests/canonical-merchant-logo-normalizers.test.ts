import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	FavoriteSchema,
	FollowSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import type { PostListItem } from "../lib/api/openapi-types";
import { normalizePost } from "../lib/api/posts";
import { type CartGroupWire, flattenGroups } from "../lib/cart/types";
import { normalizeConversation } from "../lib/chat/repo";
import { mapProduct } from "../lib/market/listFilters";
import { normalizeMemberCard } from "../lib/member-cards/repo";
import { normalizeOrder } from "../lib/orders/repo";

const CANONICAL = "https://media.example.com/i/avatar/canonical.png";
const LEGACY = "https://legacy.example/avatar.png";

function cartGroup(
	logoUrl: string | undefined,
): CartGroupWire & Record<string, unknown> {
	return {
		storeId: "seller-1",
		sellerName: "Canonical Store",
		logoUrl,
		logo_url: LEGACY,
		sellerAvatarUrl: LEGACY,
		items: [
			{
				skuId: "sku-1",
				name: "Product",
				price: "100",
				quantity: "1",
			},
		],
	};
}

test("merchant logo normalizers use only each endpoint's canonical logo field", () => {
	const cart = flattenGroups([cartGroup(` ${CANONICAL} `)])[0];
	assert.equal(cart.sellerLogoUrl, CANONICAL);
	assert.equal(flattenGroups([cartGroup(" ")])[0].sellerLogoUrl, undefined);

	const product = mapProduct(
		{
			id: "product-1",
			name: "Product",
			seller: { name: "Canonical Store", logoUrl: ` ${CANONICAL} ` },
			sellerAvatarUrl: LEGACY,
			logo_url: LEGACY,
		} as Parameters<typeof mapProduct>[0],
		0,
	);
	assert.equal(product.sellerAvatar, CANONICAL);
	assert.equal(
		mapProduct(
			{
				id: "product-2",
				name: "Product",
				seller: { name: "Legacy Store", logoUrl: " " },
				sellerAvatarUrl: LEGACY,
				logo_url: LEGACY,
			} as Parameters<typeof mapProduct>[0],
			1,
		).sellerAvatar,
		"",
	);

	const favorite = create(FavoriteSchema, {
		product: {
			id: "product-1",
			store: { id: "seller-1", logo: { uriTemplate: CANONICAL } },
		},
	});
	assert.equal(favorite.product?.store?.logo?.uriTemplate, CANONICAL);
	assert.equal(
		create(FavoriteSchema, {
			product: { id: "product-2", store: { id: "seller-2" } },
		}).product?.store?.logo,
		undefined,
	);

	const memberCard = normalizeMemberCard({
		sellerId: "seller-1",
		logoUrl: ` ${CANONICAL} `,
		seller_logo_url: LEGACY,
		seller_logo: LEGACY,
		logo: LEGACY,
		avatar: LEGACY,
	});
	assert.equal(memberCard.sellerLogo, CANONICAL);
	assert.equal(
		normalizeMemberCard({
			sellerId: "seller-2",
			logoUrl: " ",
			seller_logo: LEGACY,
		}).sellerLogo,
		"",
	);

	const order = normalizeOrder({
		order_no: "order-1",
		logoUrl: ` ${CANONICAL} `,
		sellerAvatarUrl: LEGACY,
		seller_logo_url: LEGACY,
		seller: { logo_url: LEGACY, logoUrl: LEGACY },
	});
	assert.equal(order.sellerLogo, CANONICAL);
	assert.equal(
		normalizeOrder({
			order_no: "order-2",
			logoUrl: " ",
			sellerAvatarUrl: LEGACY,
			seller_logo_url: LEGACY,
			seller: { logo_url: LEGACY },
		}).sellerLogo,
		undefined,
	);

	// 貼文 wire 形狀（全 camelCase，backend#2235；單數 imageKey 已移除，
	// 只剩 imageKeys/imageUrls 且無圖時為 []）。
	const postBase = {
		id: "post-1",
		sellerId: "seller-1",
		sellerName: "Canonical Store",
		content: "",
		imageKeys: [],
		imageUrls: [],
		tags: [],
		commentCount: 0,
		likeCount: 0,
		shareCount: 0,
		viewCount: 0,
		isLiked: false,
		createdAt: 0,
	};
	// §231 (#430 D3-S13): the post wire carries `logoKey` (objectKey SSOT) beside
	// the Gateway-composed `logoUrl`; the canonical field the normalizer reads is
	// the key, and every other logo-ish alias stays ignored.
	const post = normalizePost({
		...postBase,
		logoKey: ` ${CANONICAL} `,
		logoUrl: LEGACY,
		seller_logo_url: LEGACY,
		sellerLogo: LEGACY,
	} as PostListItem);
	assert.equal(post.logoKey, CANONICAL);
	assert.equal(
		normalizePost({
			...postBase,
			id: "post-2",
			logoKey: " ",
			logoUrl: LEGACY,
			seller_logo_url: LEGACY,
		} as PostListItem).logoKey,
		"",
	);

	const follow = create(FollowSchema, {
		store: {
			id: "seller-1",
			name: "Canonical Store",
			logo: { uriTemplate: CANONICAL },
		},
	});
	assert.equal(follow.store?.logo?.uriTemplate, CANONICAL);
	assert.equal(
		create(FollowSchema, { store: { id: "seller-2" } }).store?.logo,
		undefined,
	);

	const conversation = normalizeConversation({
		targetUserId: "seller-1",
		targetNickname: "Canonical Store",
		logoUrl: ` ${CANONICAL} `,
		logo_url: LEGACY,
		seller_logo_url: LEGACY,
		lastMessage: "",
	});
	assert.equal(conversation.logo_url, CANONICAL);
	assert.equal(
		normalizeConversation({
			targetUserId: "seller-2",
			targetNickname: "Legacy Store",
			logoUrl: " ",
			logo_url: LEGACY,
			lastMessage: "",
		}).logo_url,
		undefined,
	);
});
