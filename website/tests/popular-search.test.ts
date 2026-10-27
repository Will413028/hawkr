import assert from "node:assert/strict";
import { test } from "node:test";
import {
	buildPopularSearchItems,
	buildPopularSearchPlaceholder,
	POPULAR_HASHTAG_LIMIT,
} from "../features/home/components/popularSearch";

test("popular search uses ranked product hashtags and their filter route", () => {
	const items = buildPopularSearchItems([
		{ name: "冷凍水產", label: "#冷凍水產", count: 35 },
		{ name: " EC限定優惠 ", label: "#EC限定優惠", count: 8 },
		{ name: "#冷凍水產", label: "#冷凍水產", count: 1 },
	]);

	assert.deepEqual(items, [
		{
			name: "冷凍水產",
			href: "/market/products?hashtags=%E5%86%B7%E5%87%8D%E6%B0%B4%E7%94%A2",
			source: "popular-hashtag",
		},
		{
			name: "EC限定優惠",
			href: "/market/products?hashtags=EC%E9%99%90%E5%AE%9A%E5%84%AA%E6%83%A0",
			source: "popular-hashtag",
		},
	]);
});

test("popular search caps the API list at the homepage density limit", () => {
	const tags = Array.from(
		{ length: POPULAR_HASHTAG_LIMIT + 3 },
		(_, index) => ({
			name: `標籤${index + 1}`,
			label: `#標籤${index + 1}`,
			count: POPULAR_HASHTAG_LIMIT + 3 - index,
		}),
	);

	assert.equal(buildPopularSearchItems(tags).length, POPULAR_HASHTAG_LIMIT);
});

test("popular search does not invent popularity data when the API is empty", () => {
	assert.deepEqual(buildPopularSearchItems([]), []);
	assert.equal(buildPopularSearchPlaceholder([]), "搜尋新鮮食材...");
});

test("search placeholder follows the first three live popular items", () => {
	const items = buildPopularSearchItems([
		{ name: "冷凍水產", label: "#冷凍水產", count: 35 },
		{ name: "EC限定優惠", label: "#EC限定優惠", count: 8 },
		{ name: "貝類", label: "#貝類", count: 7 },
		{ name: "蟹類", label: "#蟹類", count: 4 },
	]);

	assert.equal(
		buildPopularSearchPlaceholder(items),
		"搜尋冷凍水產、EC限定優惠、貝類...",
	);
});
