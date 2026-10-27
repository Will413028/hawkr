import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import {
	cartItemToGA4,
	marketItemToGA4,
	orderItemToGA4,
	sumValue,
} from "../lib/analytics/items";

test("marketItemToGA4 maps fields and uses numeric priceValue", () => {
	const item = marketItemToGA4(
		{
			id: "843856384063471760",
			name: "有機高麗菜",
			image: "x.jpg",
			sellerName: "陽光農場",
			sellerId: "9",
			origin: "宜蘭",
			price: "NT$120",
			isOnSale: false,
			hashtags: [],
			unit: "顆",
			rating: null,
			reviews: 0,
			href: "/products/843856384063471760",
			priceValue: 120,
			createdAt: 0,
			salesCount: 0,
		},
		3,
	);
	assert.equal(item.item_id, "843856384063471760");
	assert.equal(item.item_name, "有機高麗菜");
	assert.equal(item.price, 120);
	assert.equal(item.item_brand, "陽光農場");
	assert.equal(item.index, 3);
	assert.equal(item.item_variant, undefined); // no SKU in list context
});

test("cartItemToGA4 carries skuId variant and int64 id as string", () => {
	const item = cartItemToGA4({
		id: "101",
		skuId: "101",
		productId: "843856384063471760",
		name: "草莓",
		price: "NT$1,200",
		priceValue: 1200,
		quantity: 2,
		sellerName: "莓好農園",
	});
	assert.equal(item.item_id, "843856384063471760");
	assert.equal(item.item_variant, "101");
	assert.equal(item.price, 1200);
	assert.equal(item.quantity, 2);
});

test("orderItemToGA4 parses string price and quantity", () => {
	const item = orderItemToGA4({
		orderItemId: 1,
		productId: "555",
		skuId: "777",
		name: "蜜蘋果",
		price: "350",
		quantity: "3",
	});
	assert.equal(item.item_id, "555");
	assert.equal(item.item_variant, "777");
	assert.equal(item.price, 350);
	assert.equal(item.quantity, 3);
});

test("sumValue totals price*quantity", () => {
	assert.equal(
		sumValue([
			{ price: 100, quantity: 2 },
			{ price: 50, quantity: 1 },
		]),
		250,
	);
});

test("market search analytics never embeds the visitor's raw keyword", () => {
	const source = readFileSync(
		new URL(
			"../features/market/components/MarketProductsList.tsx",
			import.meta.url,
		),
		"utf8",
	);

	assert.match(source, /fetchParams\.keyword\s*\?\s*"搜尋結果"/);
	assert.doesNotMatch(source, /`搜尋：\$\{fetchParams\.keyword\}`/);
});
