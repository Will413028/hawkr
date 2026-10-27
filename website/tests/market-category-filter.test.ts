import assert from "node:assert/strict";
import { test } from "node:test";
import {
	type CategoryNode,
	findCategoryId,
	findCategoryNode,
	normaliseCategorySelections,
} from "../lib/market/listFilters";

const tree: CategoryNode[] = [
	{
		id: "1",
		name: "蔬菜",
		children: [
			{
				id: "12",
				name: "小白菜",
				children: [{ id: "535", name: "水耕" }],
			},
			{
				id: "13",
				name: "青江白菜",
				children: [{ id: "539", name: "水耕" }],
			},
		],
	},
];

test("category selections canonicalise legacy names to stable ids", () => {
	assert.deepEqual(normaliseCategorySelections(tree, ["小白菜", "13"]), [
		"12",
		"13",
	]);
	assert.equal(findCategoryId(tree, "小白菜"), "12");
	assert.equal(findCategoryId(tree, "13"), "13");
});

test("stable ids disambiguate duplicate category labels", () => {
	assert.equal(findCategoryNode(tree, "535")?.name, "水耕");
	assert.equal(findCategoryNode(tree, "539")?.name, "水耕");
	assert.equal(findCategoryId(tree, "539"), "539");
	assert.deepEqual(normaliseCategorySelections(tree, ["535", "539"]), [
		"535",
		"539",
	]);
});

test("unknown legacy values remain removable instead of disappearing", () => {
	assert.deepEqual(normaliseCategorySelections(tree, ["未知分類"]), [
		"未知分類",
	]);
});
