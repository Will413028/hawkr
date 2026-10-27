import assert from "node:assert/strict";
import test from "node:test";
import {
	collectSelectedEditorBlockIds,
	collectSelectedEditorBlockIndexes,
	DESCRIPTION_HISTORY_ACTION_LIMIT,
	descriptionHistoryDirectionForInputType,
	descriptionHistoryInputBatch,
	editorBlocksToListItems,
	isAlignableEditorBlockType,
	isListConvertibleEditorBlock,
	limitDescriptionHistory,
	tableCellHtmlWithListMarker,
} from "../lib/products/descriptionEditorActions";

test("Word-like history groups letters but splits at words, pauses and deletes", () => {
	const first = descriptionHistoryInputBatch(
		"insertText",
		"a",
		"p1",
		100,
		null,
	);
	assert.equal(first.forceNewAction, true);
	const second = descriptionHistoryInputBatch(
		"insertText",
		"b",
		"p1",
		200,
		first.next,
	);
	assert.equal(second.forceNewAction, false);
	const space = descriptionHistoryInputBatch(
		"insertText",
		" ",
		"p1",
		300,
		second.next,
	);
	assert.equal(space.forceNewAction, false);
	const nextWord = descriptionHistoryInputBatch(
		"insertText",
		"c",
		"p1",
		400,
		space.next,
	);
	assert.equal(nextWord.forceNewAction, true);
	const deletion = descriptionHistoryInputBatch(
		"deleteContentBackward",
		null,
		"p1",
		500,
		nextWord.next,
	);
	assert.equal(deletion.forceNewAction, true);
	const afterPause = descriptionHistoryInputBatch(
		"deleteContentBackward",
		null,
		"p1",
		2000,
		deletion.next,
	);
	assert.equal(afterPause.forceNewAction, true);
});

test("Editor.js selected blocks remain actionable after native Range is cleared", () => {
	assert.deepEqual(
		collectSelectedEditorBlockIndexes([
			{ isEditorSelected: true, intersectsNativeRange: false },
			{ isEditorSelected: true, intersectsNativeRange: false },
			{ isEditorSelected: false, intersectsNativeRange: false },
		]),
		[0, 1],
	);
	assert.deepEqual(
		collectSelectedEditorBlockIndexes([
			{ isEditorSelected: false, intersectsNativeRange: true },
			{ isEditorSelected: true, intersectsNativeRange: false },
		]),
		[0, 1],
	);
	assert.deepEqual(
		collectSelectedEditorBlockIds([
			{ id: "p1", isEditorSelected: true, intersectsNativeRange: false },
			{ id: "h1", isEditorSelected: false, intersectsNativeRange: true },
			{ id: "", isEditorSelected: true, intersectsNativeRange: false },
		]),
		["p1", "h1"],
	);
});

test("alignment preflight accepts text/list blocks and rejects media containers", () => {
	assert.equal(isAlignableEditorBlockType("paragraph"), true);
	assert.equal(isAlignableEditorBlockType("header"), true);
	assert.equal(isAlignableEditorBlockType("list"), true);
	assert.equal(isAlignableEditorBlockType("image"), false);
	assert.equal(isAlignableEditorBlockType("table"), false);
});

test("native beforeinput history directions map to the custom snapshot stack", () => {
	assert.equal(descriptionHistoryDirectionForInputType("historyUndo"), -1);
	assert.equal(descriptionHistoryDirectionForInputType("historyRedo"), 1);
	assert.equal(descriptionHistoryDirectionForInputType("insertText"), null);
});

test("selected table cells receive sequential markers and can switch list type", () => {
	assert.equal(tableCellHtmlWithListMarker("愛文", false, 0), "• 愛文");
	assert.equal(tableCellHtmlWithListMarker("玉文", true, 1), "2. 玉文");
	assert.equal(tableCellHtmlWithListMarker("• 金煌", true, 0), "1. 金煌");
	assert.equal(tableCellHtmlWithListMarker("3. 夏雪", false, 2), "• 夏雪");
});

test("description history retains exactly 20 undoable actions", () => {
	const snapshots = Array.from({ length: 30 }, (_, index) => index);
	const limited = limitDescriptionHistory(snapshots);
	assert.equal(DESCRIPTION_HISTORY_ACTION_LIMIT, 20);
	assert.equal(limited.length, 21);
	assert.deepEqual(limited, snapshots.slice(-21));
});

test("selected paragraphs and headings become one flat list", () => {
	assert.deepEqual(
		editorBlocksToListItems([
			{ type: "paragraph", data: { text: "愛文芒果" } },
			{ type: "header", data: { text: "<b>玉文芒果</b>", level: 3 } },
			{ type: "paragraph", data: { text: "金煌<br>夏雪" } },
		]),
		["愛文芒果", "<b>玉文芒果</b>", "金煌", "夏雪"],
	);
});

test("existing list items keep inline HTML and nested-list content shape", () => {
	assert.deepEqual(
		editorBlocksToListItems([
			{
				type: "list",
				data: {
					items: ["<u>大果</u>", { content: "中果", items: [] }, "&nbsp;"],
				},
			},
		]),
		["<u>大果</u>", "中果"],
	);
});

test("table and image blocks are not list-convertible", () => {
	assert.equal(isListConvertibleEditorBlock({ type: "paragraph" }), true);
	assert.equal(isListConvertibleEditorBlock({ type: "list" }), true);
	assert.equal(isListConvertibleEditorBlock({ type: "table" }), false);
	assert.equal(isListConvertibleEditorBlock({ type: "image" }), false);
});
