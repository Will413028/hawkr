import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

/* ── A solid Button's label must follow its surface ─────────────────────── */

test("solid Button colours do not pin their text with !important", () => {
	const source = read("components/ui/Button.tsx");
	const block = source.slice(
		source.indexOf("const solidColorClass"),
		source.indexOf("const borderedColorClass"),
	);
	assert.ok(block.length > 0, "solidColorClass block not found");

	// `text-…!` here made every solid button's label unrepaintable by its
	// caller, so a custom light surface (`bg-[#c4a93a]`) kept white text at
	// ~2.2:1 while its icon painted dark — the label and the glyph disagreed.
	const pinned = [...block.matchAll(/text-\[[^\]]+\]!/g)].map((m) => m[0]);
	assert.deepEqual(
		pinned,
		[],
		`solid variants must not force their text colour: ${pinned.join(", ")}`,
	);
});

test("Button marks itself so the dark-mode anchor rule skips it", () => {
	const button = read("components/ui/Button.tsx");
	// Both branches: the anchor is the one the CSS rule targets, and the
	// <button> carries it so the marker means "a Button", not "a Button link".
	assert.equal(
		(button.match(/data-hkui-button=""/g) ?? []).length,
		2,
		"both the <Link> and <button> branches must carry data-hkui-button",
	);

	const theme = read("styles/foundation/01-tokens-theme.css");
	// Must *not match* rather than be listed in the `color: inherit` opt-out:
	// that opt-out outranks Tailwind's single class, so it would bury the
	// button's own foreground token just as `!important` did.
	assert.match(
		theme,
		/\[data-theme="dark"\] body a:not\(\[data-hkui-button\]\) \{/,
		"the dark link rule must exclude buttons via :not()",
	);
	assert.doesNotMatch(
		theme,
		/\[data-theme="dark"\] body a \{/,
		"an unqualified dark `body a` rule would repaint button labels again",
	);

	// The same class of bug one rule up: `[data-theme="dark"] body span` also
	// matched the label a button wraps its text in, so in dark mode the text
	// went `--hk-text-body` while the button and its icon kept the intended
	// colour — a two-tone CTA. Must cover hand-rolled buttons too (聊天 is a
	// plain `<button>`, not the shared component), and must not use `:where()`,
	// which scores zero specificity and would lose to the rule it overrides.
	assert.match(
		theme,
		/\[data-theme="dark"\] body button span,\n\[data-theme="dark"\] body button label,/,
		"any button's label must inherit its button's colour in dark mode",
	);
	assert.doesNotMatch(
		theme,
		/\[data-theme="dark"\] body [^\n]*:where\(span, label\)/,
		":where() cannot outrank the `body span` rule",
	);
});

/* ── A refusal must not look like a confirmation ────────────────────────── */

test("cart snackbar renders a distinct error tone", () => {
	const source = read("components/shared/CartSnackbar.tsx");

	assert.match(source, /tone\?: CartToastTone/, "toasts must accept a tone");
	assert.match(
		source,
		/TONE_SURFACE_CLASS: Record<CartToastTone, string>/,
		"each tone needs its own surface",
	);
	// The tick is the specific thing the buyer reads as "done". An error must
	// not render it.
	assert.match(
		source,
		/toast\.tone === "error" \? \([\s\S]{0,400}?<circle/,
		"the error tone must swap the checkmark for a non-success glyph",
	);
	assert.match(
		source,
		/role=\{toast\?\.tone === "error" \? "alert" : "status"\}/,
		"an error must interrupt, not queue politely behind other live regions",
	);
});

test("every add-to-cart refusal carries tone: error", () => {
	const callSites: Array<[string, RegExp]> = [
		[
			"components/shared/AddToCartButton.tsx",
			/emitCartToast\(\{ message: cartErrorMessage\(err\), tone: "error" \}\)/,
		],
		[
			"features/product/components/ProductPurchaseBox.tsx",
			/emitCartToast\(\{ message: buyNowErrorMessage\(err\), tone: "error" \}\)/,
		],
	];
	for (const [file, pattern] of callSites) {
		assert.match(read(file), pattern, `${file} must report failures as errors`);
	}

	// 6006 (自家商品) is the case that surfaced this: the buyer was told
	// "不可購買自己上架的商品" under a green tick.
	for (const file of [
		"components/shared/AddToCartButton.tsx",
		"features/product/components/ProductPurchaseBox.tsx",
	]) {
		assert.match(read(file), /err\.code === 6006/, `${file} still maps 6006`);
	}
});

/* ── The button must not destabilise itself on a fast add ───────────────── */

test("AddToCartButton delays its pending skin below the perception threshold", () => {
	const source = read("components/shared/AddToCartButton.tsx");

	assert.match(source, /const PENDING_VISIBLE_AFTER_MS = \d+/);
	const threshold = Number(
		/const PENDING_VISIBLE_AFTER_MS = (\d+)/.exec(source)?.[1],
	);
	assert.ok(
		threshold >= 150 && threshold <= 400,
		`threshold ${threshold}ms should sit in the "too fast to read" band`,
	);

	// `pending` gates input, `showPending` gates pixels. The loading skin must
	// follow the delayed one, or the button flashes on every tap.
	assert.match(source, /isLoading=\{showPending\}/);
	// The in-flight state is reported with `aria-disabled`, never the DOM
	// `disabled`: a focused button that becomes disabled drops focus to <body>,
	// so a keyboard buyer lost their place on every add. `disabled` is reserved
	// for the caller's own "this product cannot be added" state.
	assert.match(
		source,
		/aria-disabled=\{showPending \|\| authWaiting \|\| undefined\}/,
	);
	assert.match(source, /\n\t\t\t\tdisabled=\{disabled\}/);
	// Input is still guarded from the first millisecond.
	assert.match(source, /if \(disabled \|\| pending \|\| authWaiting\) return;/);
});

/* ── The added quantity flies to the cart ───────────────────────────────── */

test("the add-to-cart flight has a landing target and degrades safely", () => {
	const fly = read("lib/cart/flyToCart.ts");

	// Monotonic axes: every vertical keyframe is a fraction of `dy`, so the
	// chip can never travel past the cart. An earlier version added a distance
	// -scaled lift on top of `dy` and threw the chip off the top of the
	// viewport, because the cart icon already sits near y = 0.
	const chipKeyframes = fly.slice(
		fly.indexOf("chip.animate("),
		fly.indexOf("window.setTimeout(bumpCartBadge"),
	);
	const verticalOffsets = [
		...chipKeyframes.matchAll(/translate3d\(0, ([^)]*?)(?:px)?, 0\)/g),
	].map((m) => m[1].trim());
	assert.deepEqual(
		verticalOffsets,
		// biome-ignore lint/suspicious/noTemplateCurlyInString: 這支測試掃的是**源碼字串**，
		// 而源碼裡那一格就是未展開的 `${dy}`（動畫的垂直位移在執行期才代入）。寫成 template
		// literal 會讓它在這裡就被求值成別的東西，斷言就測不到源碼長什麼樣了。
		["0", "0", "${dy}"],
		"one hold on the button, then a single uninterrupted run to the cart",
	);
	// Two easings meeting mid-path is a visible corner. Each axis gets exactly
	// one cubic for the travel segment.
	assert.equal(
		(chipKeyframes.match(/easing:/g) ?? []).length,
		2,
		"the chip needs one easing for the pop-in and one for the flight — no more",
	);
	// The flight must be long enough to follow, and must not front-load its
	// travel into the first few frames (easeOutExpo did exactly that).
	const flightMs = Number(/const FLIGHT_MS = (\d+)/.exec(fly)?.[1]);
	assert.ok(
		flightMs >= 800,
		`flight of ${flightMs}ms is too short to read as a flight`,
	);
	assert.match(
		fly,
		/const LIFTOFF_OFFSET = 0\.\d+/,
		"the chip must hold on the button before departing",
	);
	assert.doesNotMatch(
		chipKeyframes,
		/cubic-bezier\(0\.16, 1, 0\.3, 1\)/,
		"easeOutExpo made the flight visually instant",
	);

	assert.match(fly, /prefersReducedMotion/, "must honour reduced motion");
	assert.match(
		fly,
		/if \(!originRect \|\| !targetRect\) \{\s*bumpCartBadge\(\);\s*return;\s*\}/,
		"a missing target must degrade to the badge pop, not throw",
	);

	const bubble = read("components/shared/CartBubble.tsx");
	assert.match(bubble, /CART_FLY_TARGET_ATTR/, "cart icon must be the target");
	assert.match(bubble, /CART_FLY_BADGE_ATTR/, "count bubble must be poppable");

	assert.match(
		read("components/shared/AddToCartButton.tsx"),
		/flyToCart\(buttonRef\.current, quantity\)/,
		"the flight must start from the button that was pressed",
	);
});
