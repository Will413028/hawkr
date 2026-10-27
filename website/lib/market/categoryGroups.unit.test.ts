import { describe, expect, it } from "vitest";
import {
	categoryGroupMemberIds,
	categoryGroupsFromTree,
	findCategoryGroupByName,
} from "./categoryGroups";

// Shape of /public/categories/tree as served by test-api on 2026-09-10: nine
// lv1 roots, lv2 children, lv3 grandchildren, ids unique across the tree.
const TREE = [
	{
		id: "2",
		name: "水果",
		parentId: "0",
		children: [
			{ id: "154", name: "柚子", children: [{ id: "763", name: "文旦" }] },
			{ id: "134", name: "鳳梨", children: [] },
		],
	},
	{
		id: 5,
		name: "海鮮",
		children: [{ id: 206, name: "魚類" }],
	},
	{ id: "", name: "沒有 id 的節點", children: [{ id: "999", name: "x" }] },
	{ id: "9", name: "  ", children: [] },
	null,
	"junk",
];

describe("categoryGroupsFromTree", () => {
	it("indexes every lv1 with all of its descendants, at any depth", () => {
		expect(categoryGroupsFromTree(TREE)).toEqual([
			{ id: "2", name: "水果", descendantIds: ["154", "763", "134"] },
			{ id: "5", name: "海鮮", descendantIds: ["206"] },
		]);
	});

	it("accepts the { list } envelope the market filters pass around", () => {
		expect(categoryGroupsFromTree({ list: TREE })).toHaveLength(2);
	});

	it("treats anything that is not a tree as no tree", () => {
		expect(categoryGroupsFromTree(null)).toEqual([]);
		expect(categoryGroupsFromTree(undefined)).toEqual([]);
		expect(categoryGroupsFromTree("nope")).toEqual([]);
		expect(categoryGroupsFromTree({ data: TREE })).toEqual([]);
	});

	it("answers membership for the group's own id and its subtree", () => {
		const groups = categoryGroupsFromTree(TREE);
		const fruit = findCategoryGroupByName(groups, " 水果 ");
		expect(fruit).toBeDefined();
		const members = categoryGroupMemberIds(fruit as NonNullable<typeof fruit>);
		expect(members.has("2")).toBe(true);
		expect(members.has("763")).toBe(true);
		expect(members.has("206")).toBe(false);
		expect(findCategoryGroupByName(groups, "肉品")).toBeUndefined();
	});
});
