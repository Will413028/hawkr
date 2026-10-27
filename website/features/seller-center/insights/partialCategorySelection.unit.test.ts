import { describe, expect, it } from "vitest";
import type { CategoryNode } from "@/lib/products/seller-repo";
import { resolvePartialCategorySelection } from "./partialCategorySelection";

const TREE: CategoryNode[] = [
	{
		id: 1,
		name: "蔬菜",
		children: [
			{
				id: 11,
				name: "瓜果類",
				children: [{ id: 111, name: "花胡瓜" }],
			},
			{ id: 12, name: "葉菜類", children: [] },
		],
	},
];

describe("resolvePartialCategorySelection", () => {
	it("returns the L1 node when only L1 is picked", () => {
		expect(resolvePartialCategorySelection(TREE, ["1", "", ""])).toEqual({
			level: 1,
			categoryId: "1",
			label: "蔬菜",
		});
	});

	it("returns the L2 node when L1+L2 are picked but not L3", () => {
		expect(resolvePartialCategorySelection(TREE, ["1", "11", ""])).toEqual({
			level: 2,
			categoryId: "11",
			label: "瓜果類",
		});
	});

	it("returns null once L3 is also picked — the full resolution flow takes over", () => {
		expect(
			resolvePartialCategorySelection(TREE, ["1", "11", "111"]),
		).toBeNull();
	});

	it("returns null when nothing is picked", () => {
		expect(resolvePartialCategorySelection(TREE, ["", "", ""])).toBeNull();
	});

	it("returns null for an id that doesn't resolve against the tree instead of throwing", () => {
		expect(resolvePartialCategorySelection(TREE, ["1", "99", ""])).toBeNull();
		expect(resolvePartialCategorySelection(TREE, ["9", "", ""])).toBeNull();
	});
});
