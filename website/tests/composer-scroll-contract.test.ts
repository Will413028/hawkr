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
	// Assert on code, not prose: the comments deliberately name the rejected
	// alternatives, so a raw source match would fail on its own explanation.
	const code = read(HOOK)
		.replaceAll(/\/\*[\s\S]*?\*\//g, "")
		.replaceAll(/^\s*\/\/.*$/gm, "");

	// Each composer owns its height; shared caret scrolling must not override it.
	assert.doesNotMatch(
		code,
		/style\.height|style\.maxHeight|style\.minHeight/,
		"the hook must never resize the composer",
	);
	// `scrollHeight` is read here — the idle reveal needs it to know whether the
	// draft overflows at all. What must never happen is it being written back
	// as a height, which is how the rejected auto-grow version worked. (The
	// `style.height` family is already covered above; this closes the
	// setProperty route to the same thing.)
	assert.doesNotMatch(
		code,
		/setProperty\(\s*"(min-|max-)?height"/,
		"nothing may assign a height",
	);
});

test("the caret line is kept visible by measurement, not by scrolling to the end", () => {
	const hook = read(HOOK);

	// `scrollTop = scrollHeight` is only correct while the caret is at the end;
	// it throws the view to the bottom when someone clicks back into a draft.
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

	// The mirror carries no padding/border so offsetTop is content-relative;
	// copying the padding onto it put the caret ~11px off.
	assert.match(hook, /padding:0;border:0/);
	assert.match(hook, /paddingTop\) \|\| 0\) \+ m\.offsetTop/);
});

test("IME composition is not interrupted", () => {
	const hook = read(HOOK);
	// 注音 / 拼音 candidates are provisional: the committed value and the caret
	// disagree mid-composition, and scrolling then fights the input method.
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

	// 一般聊天室 keeps wrapping — changing it to nowrap was never approved, and
	// the two share `.thread-page__compose textarea`, so the opt-in is a
	// modifier class rather than a change to the base rule.
	const thread = read("features/account/screens/messages/ThreadClient.tsx");
	assert.match(thread, /useComposerScroll\(composerRef, draft\)/);
	assert.doesNotMatch(thread, /thread-page__input--nowrap/);
	assert.doesNotMatch(thread, /"inline"/);
});

test("the axis contract is explicit and matches the stylesheet", () => {
	const hook = read(HOOK);
	assert.match(hook, /export type ComposerAxis = "block" \| "inline";/);
	// Horizontal reads/writes scrollLeft against scrollWidth/clientWidth; a
	// copy-paste of the vertical axis would silently do nothing.
	assert.match(hook, /get: \(el\) => el\.scrollLeft/);
	assert.match(hook, /content: \(el\) => el\.scrollWidth/);
	assert.match(hook, /viewport: \(el\) => el\.clientWidth/);
	// The mirror must stop wrapping too, or the caret is measured on the wrong
	// geometry.
	assert.match(hook, /mirrorCss: \(\) => "white-space:pre;width:max-content;"/);

	const css = read("styles/features/messages.css");
	const modifier =
		css.match(
			/\.thread-page__compose textarea\.thread-page__input--nowrap \{([^}]*)\}/,
		)?.[1] ?? "";
	assert.ok(modifier.length > 0, "nowrap modifier rule not found");
	assert.match(modifier, /white-space: nowrap/);
	// The box is 40px tall: a horizontal scrollbar would eat the one line of
	// text from the inside.
	assert.match(modifier, /overflow-y: hidden/);
	assert.match(modifier, /scrollbar-width: none/);
	// Always one line now, so the vertical cap is dead — leaving it would imply
	// the box still grows.
	assert.match(modifier, /max-height: none/);

	// The base rule must stay wrapping for every other composer.
	const base =
		css.match(/\.thread-page__compose textarea \{([^}]*)\}/)?.[1] ?? "";
	assert.doesNotMatch(base, /white-space:\s*nowrap/);
});

test("the idle reveal only runs when nobody is working in the composer", () => {
	const hook = read(HOOK);

	// Moving text inside an editable field is hostile if it can start while
	// someone is typing, and WCAG 2.2.2 requires motion running past five
	// seconds to be stoppable. Every one of these must cancel it.
	for (const event of ["focus", "pointerenter", "pointerdown"]) {
		assert.match(
			hook,
			new RegExp(`addEventListener\\("${event}", stop\\)`),
			`${event} must stop the reveal`,
		);
	}
	assert.match(hook, /prefers-reduced-motion: reduce/);
	// Nothing to reveal when the draft already fits.
	assert.match(hook, /ops\.content\(el\) <= ops\.viewport\(el\)/);
	// It steps a line at a time and holds; a continuous crawl would be unreadable.
	assert.match(hook, /const IDLE_START_MS = [\d_]+;/);
	assert.match(hook, /const LINE_HOLD_MS = [\d_]+;/);
	assert.match(hook, /const END_HOLD_MS = [\d_]+;/);
});

test("one owner writes the composer's scrollTop", () => {
	// Caret-follow and idle reveal both drive `scrollTop`; split across two
	// hooks they would fight over it, so they ship together.
	const hook = read(HOOK);
	assert.match(hook, /useIdleReveal\(ref, value, axis\);/);
	assert.equal(
		(read("lib/hooks/useComposerScroll.ts").match(/^export function /gm) ?? [])
			.length,
		1,
		"only the combined hook is public",
	);
});
