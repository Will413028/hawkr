import assert from "node:assert/strict";
import { mock, test } from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	type Favorite,
	FavoriteSchema,
} from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

/** The overview rail shows active products and reserves its empty state for an empty collection. */

function favorite(id: string, deleted = false): Favorite {
	return create(FavoriteSchema, {
		product: {
			id,
			name: deleted ? "" : "彰化溪州珍珠芭樂",
			image: deleted
				? undefined
				: { uriTemplate: "https://media.example/i/{preset}/product.png" },
			available: !deleted,
			deleted,
		},
	});
}

const EMPTY_STATE = "還沒有收藏商品";

async function renderOverview(items: Favorite[]): Promise<string> {
	mock.reset();
	mock.module("@/lib/auth/context", {
		exports: {
			useAuth: () => ({ identity: { nickname: "阿明" } }),
			useOptionalAuth: () => null,
		},
	});
	mock.module("@/components/shared/AppImage", {
		exports: {
			default: (props: { src?: string; alt?: string }) =>
				React.createElement("img", { src: props.src, alt: props.alt }),
		},
	});
	mock.module("@/components/shared/Link", {
		exports: {
			default: (props: {
				href?: unknown;
				className?: string;
				children?: React.ReactNode;
			}) =>
				React.createElement(
					"a",
					{ href: String(props.href ?? ""), className: props.className },
					props.children,
				),
		},
	});
	mock.module("@/lib/hooks/useAccount", {
		exports: {
			useCoupons: () => ({ my: [] }),
			useMemberCards: () => ({ cards: [] }),
			useFollowing: () => ({ items: [] }),
			useFavorites: () => ({
				items,
				loading: false,
				error: null,
				remove: () => {},
			}),
		},
	});
	mock.module("@/lib/hooks/useOrders", {
		exports: {
			useOrderList: () => ({
				orders: [],
				total: 0,
				loading: false,
				error: null,
			}),
			useOrderStatusCounts: () => ({ counts: {} }),
		},
	});
	// 措施與申覆入口自己讀 React Query；這個測試不渲染它（入口另有單元測試）。
	mock.module("@/features/seller/measures/MeasuresAccountEntry", {
		exports: { default: () => null },
	});
	const { default: AccountOverviewClient } = await import(
		`@/features/account/screens/overview/AccountOverviewClient?t=${Math.random()}`
	);
	return renderToStaticMarkup(React.createElement(AccountOverviewClient));
}

test("有收藏且載得到：照常畫卡片，不出現空狀態", async () => {
	const html = await renderOverview([favorite("100")]);
	assert.ok(html.includes("彰化溪州珍珠芭樂"), "卡片應該畫出來");
	assert.ok(!html.includes(EMPTY_STATE), "有卡片就不該說沒有收藏");
});

test("已刪除商品不畫進回購軌，也不宣稱買家沒有收藏", async () => {
	const html = await renderOverview([favorite("100", true)]);
	assert.ok(!html.includes(EMPTY_STATE));
	assert.ok(!html.includes("/products/100"));
});

test("真的沒有收藏：維持原本的「還沒有收藏商品。」", async () => {
	const html = await renderOverview([]);
	assert.ok(html.includes(EMPTY_STATE), "空收藏的空狀態是實話，要保留");
});

test("已刪除商品不阻止其他商品顯示", async () => {
	const html = await renderOverview([favorite("100"), favorite("200", true)]);
	assert.ok(html.includes("彰化溪州珍珠芭樂"), "載得到的照畫");
	assert.ok(!html.includes("/products/200"));
	assert.ok(!html.includes(EMPTY_STATE));
});
