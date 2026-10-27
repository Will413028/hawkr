import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const dialogSource = readFileSync(
	new URL("../components/ui/Dialog.tsx", import.meta.url),
	"utf8",
);

test("dialog initial focus only follows the mounted open lifecycle", () => {
	const handlerStart = dialogSource.indexOf("const handleDocumentKeyDown");
	const lifecycleStart = dialogSource.indexOf(
		"React.useEffect(() => {",
		handlerStart,
	);
	const lifecycleEnd = dialogSource.indexOf(
		"\n\tconst completeExit",
		lifecycleStart,
	);
	const handler = dialogSource.slice(handlerStart, lifecycleStart);
	const lifecycle = dialogSource.slice(lifecycleStart, lifecycleEnd);

	assert.notEqual(handlerStart, -1);
	assert.notEqual(lifecycleStart, -1);
	assert.notEqual(lifecycleEnd, -1);
	assert.match(handler, /React\.useEffectEvent/);
	assert.match(handler, /isDismissable/);
	assert.match(handler, /onClose\(\)/);
	assert.match(handler, /nestedListboxOpen/);
	assert.match(handler, /event\.defaultPrevented/);
	assert.match(handler, /event\.key === "Escape" && open && isDismissable/);
	assert.match(handler, /aria-haspopup="listbox"/);
	assert.match(lifecycle, /if \(!mounted \|\| !present\) return/);
	assert.match(
		lifecycle,
		/panelRef\.current\?\.focus\(\{ preventScroll: true \}\)/,
	);
	assert.match(lifecycle, /\}, \[mounted, present\]\);/);
	assert.doesNotMatch(lifecycle, /\[isDismissable, onClose, open\]/);
});

test("dialog keeps a closing presence for a reversible exit transition", () => {
	assert.match(dialogSource, /const \[present, setPresent\]/);
	assert.match(dialogSource, /data-hk-overlay-state=\{visualState\}/);
	assert.match(dialogSource, /data-hk-overlay-motion=\{motion\}/);
	assert.match(dialogSource, /data-hk-overlay-motion-layer/);
	assert.match(dialogSource, /event\.propertyName === "opacity"/);
	assert.match(dialogSource, /if \(!mounted \|\| !present\) return null/);
	assert.doesNotMatch(dialogSource, /if \(!mounted \|\| !open\) return null/);
});
