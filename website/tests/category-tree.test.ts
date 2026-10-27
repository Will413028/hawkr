import assert from "node:assert/strict";
import { test } from "node:test";
import {
	type CategoryNode,
	upsertCategoryNode,
} from "../lib/products/seller-repo";

function categoryTree(): CategoryNode[] {
	return [
		{
			id: "1",
			name: "水果",
			children: [
				{
					id: "10",
					name: "柑橘類",
					parentId: "1",
					children: [{ id: "100", name: "柳橙", parentId: "10" }],
				},
			],
		},
	];
}

test("upsertCategoryNode adds a new leaf without mutating the previous tree", () => {
	const before = categoryTree();
	const next = upsertCategoryNode(before, "10", {
		id: "101",
		name: "珍珠柑",
		status: "9004",
	});

	assert.notEqual(next, before);
	assert.deepEqual(
		before[0]?.children?.[0]?.children?.map((node) => node.id),
		["100"],
	);
	assert.deepEqual(
		next[0]?.children?.[0]?.children?.map((node) => node.id),
		["100", "101"],
	);
	assert.equal(next[0]?.children?.[0]?.children?.[1]?.parentId, "10");
});

test("upsertCategoryNode replaces a returned existing category by id", () => {
	const next = upsertCategoryNode(categoryTree(), "10", {
		id: "100",
		name: "香甜柳橙",
		status: "9004",
	});
	const leaves = next[0]?.children?.[0]?.children ?? [];

	assert.equal(leaves.length, 1);
	assert.equal(leaves[0]?.name, "香甜柳橙");
	assert.equal(leaves[0]?.parentId, "10");
});

test("upsertCategoryNode leaves the cache untouched when the parent is absent", () => {
	const before = categoryTree();
	const next = upsertCategoryNode(before, "missing", {
		id: "101",
		name: "珍珠柑",
	});

	assert.equal(next, before);
});
