import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { descriptionEditorMobileToolbarTop } from "../lib/products/descriptionEditorLayout";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("mobile block actions align to the first editable line", () => {
	assert.equal(
		descriptionEditorMobileToolbarTop({
			anchorTop: 100,
			anchorHeight: 80,
			parentTop: 20,
			parentClientTop: 0,
			paddingTop: 6,
			isEditable: true,
			lineHeight: 24,
			actionHeight: 32,
		}),
		82,
	);
});

test("mobile media actions align to the media frame instead of its caption", () => {
	assert.equal(
		descriptionEditorMobileToolbarTop({
			anchorTop: 200,
			anchorHeight: 180,
			parentTop: 20,
			parentClientTop: 0,
			paddingTop: 0,
			isEditable: false,
			lineHeight: Number.NaN,
			actionHeight: 32,
		}),
		180,
	);
});

test("mobile toolbar geometry never escapes above the editor", () => {
	assert.equal(
		descriptionEditorMobileToolbarTop({
			anchorTop: 10,
			anchorHeight: 24,
			parentTop: 20,
			parentClientTop: 1,
			paddingTop: 0,
			isEditable: true,
			lineHeight: 24,
			actionHeight: 32,
		}),
		0,
	);
});

test("DescriptionEditor owns mobile positioning and uses the swap glyph for reorder", () => {
	const editor = source("../features/seller/components/DescriptionEditor.tsx");
	const icons = source("../components/shared/AppIcon.tsx");

	assert.match(editor, /EDITOR_JS_MOBILE_MEDIA = "\(max-width: 649px\)"/);
	assert.match(editor, /matchMedia\(EDITOR_JS_MOBILE_MEDIA\)/);
	assert.match(editor, /getCurrentBlockIndex\(\)/);
	assert.match(editor, /getBlockByIndex\(blockIndex\)\?\.holder/);
	assert.match(editor, /new MutationObserver\(scheduleSync\)/);
	assert.match(editor, /new ResizeObserver\(scheduleSync\)/);
	assert.match(editor, /addEventListener\("pointerover", onBlockIntent/);
	assert.match(editor, /<AppIcon name="reorder" size=\{20\}/);
	assert.match(icons, /reorder: "swap_vert"/);
	assert.doesNotMatch(editor, /<AppIcon name="more" size=\{20\}/);
});
