import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

const HOOK = "lib/hooks/useComposerScroll.ts";

test("the shared scroll hook does not own composer height", () => {
	const code = read(HOOK)
		.replaceAll(/\/\*[\s\S]*?\*\//g, "")
		.replaceAll(/^\s*\/\/.*$/gm, "");

	assert.doesNotMatch(
		code,
		/style\.height|style\.maxHeight|style\.minHeight/,
		"the hook must never resize the composer",
	);
	assert.doesNotMatch(
		code,
		/setProperty\(\s*"(min-|max-)?height"/,
		"nothing may assign a height",
	);
});

test("the caret line is kept visible by measurement, not by scrolling to the end", () => {
	const hook = read(HOOK);

	assert.match(hook, /measureCaret\(/);
	assert.match(
		hook,
		/if \(caret < offset\) \{\s*ops\.set\(el, caret\);/,
		"must scroll back when the caret is before the view",
	);
	assert.match(
		hook,
		/ops\.set\(el, caret \+ extent - ops\.viewport\(el\)\);/,
		"must scroll on when the caret is past the view",
	);

	assert.match(hook, /padding:0;border:0/);
	assert.match(hook, /paddingTop\) \|\| 0\) \+ m\.offsetTop/);
});

test("IME composition is not interrupted", () => {
	const hook = read(HOOK);
	assert.match(hook, /compositionstart/);
	assert.match(hook, /compositionend/);
	assert.match(hook, /if \(!el \|\| composingRef\.current\) return;/);
});

test("both chat composers use it, on the axis matching their wrapping", () => {
	const agent = read(
		"features/procurement-assistant/ProcurementClarifyChat.tsx",
	);
	assert.match(agent, /useComposerScroll\(textareaRef, value\)/);
	assert.match(agent, /className="hawkr-agent-composer__input"/);
	assert.match(agent, /wrap="soft"/);
	assert.match(agent, /enterKeyHint="send"/);
	assert.match(agent, /event\.shiftKey/);
	assert.match(agent, /event\.nativeEvent\.isComposing/);

	const thread = read("features/account/screens/messages/ThreadClient.tsx");
	assert.match(thread, /useComposerScroll\(composerRef, draft\)/);
	assert.doesNotMatch(thread, /thread-page__input--nowrap/);
	assert.doesNotMatch(thread, /"inline"/);
});

test("the axis contract is explicit and matches the stylesheet", () => {
	const hook = read(HOOK);
	assert.match(hook, /export type ComposerAxis = "block" \| "inline";/);
	assert.match(hook, /get: \(el\) => el\.scrollLeft/);
	assert.match(hook, /content: \(el\) => el\.scrollWidth/);
	assert.match(hook, /viewport: \(el\) => el\.clientWidth/);
	assert.match(hook, /mirrorCss: \(\) => "white-space:pre;width:max-content;"/);

	const css = read("styles/features/messages.css");
	const modifier =
		css.match(
			/\.thread-page__compose textarea\.thread-page__input--nowrap \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.ok(modifier.length > 0, "nowrap modifier rule not found");
	assert.match(modifier, /white-space: nowrap/);
	assert.match(modifier, /overflow-y: hidden/);
	assert.match(modifier, /scrollbar-width: none/);
	assert.match(modifier, /max-height: none/);

	const base =
		css.match(/\.thread-page__compose textarea \{([^}]*)\}/)?.[1] ?? "";
	assert.doesNotMatch(base, /white-space:\s*nowrap/);
});

test("the idle reveal only runs when nobody is working in the composer", () => {
	const hook = read(HOOK);

	for (const event of ["focus", "pointerenter", "pointerdown"]) {
		assert.match(
			hook,
			new RegExp(`addEventListener\\("${event}", stop\\)`),
			`${event} must stop the reveal`,
		);
	}
	assert.match(hook, /prefers-reduced-motion: reduce/);
	assert.match(hook, /ops\.content\(el\) <= ops\.viewport\(el\)/);
	assert.match(hook, /const IDLE_START_MS = [\d_]+;/);
	assert.match(hook, /const LINE_HOLD_MS = [\d_]+;/);
	assert.match(hook, /const END_HOLD_MS = [\d_]+;/);
});

test("one owner writes the composer's scrollTop", () => {
	const hook = read(HOOK);
	assert.match(hook, /useIdleReveal\(ref, value, axis\);/);
	assert.equal(
		(read("lib/hooks/useComposerScroll.ts").match(/^export function /gm) ?? [])
			.length,
		1,
		"only the combined hook is public",
	);
});
