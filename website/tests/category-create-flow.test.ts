import assert from "node:assert/strict";
import { test } from "node:test";
import {
	categoryPathDraftFromIds,
	categoryPathLabel,
	categoryPathPlan,
	markDraftL2Created,
	selectDraftL1,
	selectDraftL2,
	selectDraftL3,
	startDraftL2,
	startDraftL3,
	updateDraftCategoryName,
	validateCategoryPathDraft,
} from "../features/seller/components/categoryCreateFlow";
import type { CategoryNode } from "../lib/products/seller-repo";

function categoryTree(): CategoryNode[] {
	return [
		{
			id: "1",
			name: "水果",
			children: [
				{
					id: "10",
					name: "柑橘類",
					children: [{ id: "100", name: "柳橙" }],
				},
			],
		},
		{ id: "2", name: "蔬菜", children: [] },
	];
}

function categoryFixture() {
	const tree = categoryTree();
	const fruit = tree[0];
	const vegetable = tree[1];
	const citrus = fruit?.children?.[0];
	const orange = citrus?.children?.[0];
	assert.ok(fruit);
	assert.ok(vegetable);
	assert.ok(citrus);
	assert.ok(orange);
	return { tree, fruit, vegetable, citrus, orange };
}

test("modal draft changes stay isolated from the committed outer path", () => {
	const committedPath: [string, string, string] = ["1", "10", "100"];
	const seeded = categoryPathDraftFromIds(categoryTree(), committedPath);
	const edited = updateDraftCategoryName(
		startDraftL2(seeded),
		"l2",
		"新的柑橘",
	);

	assert.deepEqual(committedPath, ["1", "10", "100"]);
	assert.deepEqual(categoryPathPlan(seeded), {
		kind: "commit",
		l1Id: "1",
		l2Id: "10",
		l3Id: "100",
	});
	assert.equal(edited.l2.kind, "new");
	assert.equal(categoryPathLabel(seeded), "水果 › 柑橘類 › 柳橙");
});

test("changing a parent clears every dependent child selection", () => {
	const { tree, fruit, vegetable, citrus } = categoryFixture();
	const seeded = categoryPathDraftFromIds(tree, ["1", "10", "100"]);
	const changedL1 = selectDraftL1(seeded, vegetable);
	assert.equal(changedL1.l2.kind, "empty");
	assert.equal(changedL1.l3.kind, "empty");

	const restoredL1 = selectDraftL1(changedL1, fruit);
	const changedL2 = selectDraftL2(restoredL1, citrus);
	assert.equal(changedL2.l2.kind, "existing");
	assert.equal(changedL2.l3.kind, "empty");
});

test("reselecting the same parent preserves its dependent path", () => {
	const { tree, fruit, citrus } = categoryFixture();
	const seeded = categoryPathDraftFromIds(tree, ["1", "10", "100"]);

	const sameL1 = selectDraftL1(seeded, fruit);
	assert.strictEqual(sameL1, seeded);
	assert.equal(sameL1.l3.kind, "existing");

	const sameL2 = selectDraftL2(sameL1, citrus);
	assert.strictEqual(sameL2, seeded);
	assert.equal(sameL2.l3.kind, "existing");
});

test("a new L2 forces a new L3 and plans two sequential creates", () => {
	const { tree, fruit } = categoryFixture();
	let draft = selectDraftL1(
		categoryPathDraftFromIds(tree, ["", "", ""]),
		fruit,
	);
	draft = startDraftL2(draft);
	draft = updateDraftCategoryName(draft, "l2", "特色柑橘");
	draft = updateDraftCategoryName(draft, "l3", "珍珠柑");

	assert.equal(draft.l3.kind, "new");
	assert.deepEqual(categoryPathPlan(draft), {
		kind: "create-l2-l3",
		l1Id: "1",
		l2Name: "特色柑橘",
		l3Name: "珍珠柑",
	});
});

test("partial L2 success preserves the typed L3 and retries only L3", () => {
	const { tree, fruit } = categoryFixture();
	let draft = selectDraftL1(
		categoryPathDraftFromIds(tree, ["", "", ""]),
		fruit,
	);
	draft = updateDraftCategoryName(startDraftL2(draft), "l2", "特色柑橘");
	draft = updateDraftCategoryName(draft, "l3", "珍珠柑");

	const retryDraft = markDraftL2Created(draft, {
		id: "11",
		name: "特色柑橘",
	});

	assert.deepEqual(categoryPathPlan(retryDraft), {
		kind: "create-l3",
		l1Id: "1",
		l2Id: "11",
		l3Name: "珍珠柑",
	});
	assert.deepEqual(retryDraft.l3, { kind: "new", name: "珍珠柑" });
});

test("an existing L2 may select an existing L3 or start one new L3", () => {
	const { tree, fruit, citrus, orange } = categoryFixture();
	let draft = selectDraftL1(
		categoryPathDraftFromIds(tree, ["", "", ""]),
		fruit,
	);
	draft = selectDraftL2(draft, citrus);
	const existing = selectDraftL3(draft, orange);
	assert.equal(categoryPathPlan(existing)?.kind, "commit");

	const newLeaf = updateDraftCategoryName(startDraftL3(draft), "l3", "珍珠柑");
	assert.deepEqual(categoryPathPlan(newLeaf), {
		kind: "create-l3",
		l1Id: "1",
		l2Id: "10",
		l3Name: "珍珠柑",
	});
});

test("completion requires L1, L2, and a valid L3 leaf", () => {
	const { tree, fruit, citrus } = categoryFixture();
	let draft = selectDraftL1(
		categoryPathDraftFromIds(tree, ["", "", ""]),
		fruit,
	);
	draft = selectDraftL2(draft, citrus);
	assert.deepEqual(validateCategoryPathDraft(draft), {
		l3: "請選擇或新增第三層細項。",
	});
	assert.equal(categoryPathPlan(draft), null);
});
