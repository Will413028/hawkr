import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = fileURLToPath(new URL("..", import.meta.url));
const read = (relativePath: string) =>
	readFileSync(path.join(root, relativePath), "utf8");

test("solid Button colours do not pin their text with !important", () => {
	const source = read("components/ui/Button.tsx");
	const block = source.slice(
		source.indexOf("const solidColorClass"),
		source.indexOf("const borderedColorClass"),
	);
	assert.ok(block.length > 0, "solidColorClass block not found");

	const pinned = [...block.matchAll(/text-\[[^\]]+\]!/g)].map((m) => m[0]);
	assert.deepEqual(
		pinned,
		[],
		`solid variants must not force their text colour: ${pinned.join(", ")}`,
	);
});

test("Button marks itself so the dark-mode anchor rule skips it", () => {
	const button = read("components/ui/Button.tsx");
	assert.equal(
		(button.match(/data-hkui-button=""/g) ?? []).length,
		2,
		"both the <Link> and <button> branches must carry data-hkui-button",
	);

	const theme = read("styles/foundation/01-tokens-theme.css");
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

test("cart snackbar renders a distinct error tone", () => {
	const source = read("components/shared/CartSnackbar.tsx");

	assert.match(source, /tone\?: CartToastTone/, "toasts must accept a tone");
	assert.match(
		source,
		/TONE_SURFACE_CLASS: Record<CartToastTone, string>/,
		"each tone needs its own surface",
	);
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

	for (const file of [
		"components/shared/AddToCartButton.tsx",
		"features/product/components/ProductPurchaseBox.tsx",
	]) {
		assert.match(read(file), /err\.code === 6006/, `${file} still maps 6006`);
	}
});

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

	assert.match(source, /isLoading=\{showPending\}/);
	assert.match(
		source,
		/aria-disabled=\{showPending \|\| authWaiting \|\| undefined\}/,
	);
	assert.match(source, /\n\t\t\t\tdisabled=\{disabled\}/);
	assert.match(source, /if \(disabled \|\| pending \|\| authWaiting\) return;/);
});

test("the add-to-cart flight has a landing target and degrades safely", () => {
	const fly = read("lib/cart/flyToCart.ts");

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
		["0", "0", "${dy}"],
		"one hold on the button, then a single uninterrupted run to the cart",
	);
	assert.equal(
		(chipKeyframes.match(/easing:/g) ?? []).length,
		2,
		"the chip needs one easing for the pop-in and one for the flight — no more",
	);
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
