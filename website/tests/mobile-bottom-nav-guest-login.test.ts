import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const mobileBottomNavPath = new URL(
	"../components/shared/MobileBottomNav.tsx",
	import.meta.url,
);

test("guest mobile navbar exposes the login icon instead of settings", async () => {
	const source = await readFile(mobileBottomNavPath, "utf8");

	assert.match(source, /login: "login"/);
	assert.match(source, /label:\s*isLoggedIn\s*\?\s*"設定"\s*:\s*"登入"/);
	assert.match(source, /icon:\s*\(isLoggedIn\s*\?\s*"user"\s*:\s*"login"\)/);
	assert.match(source, /: "\/login\?role=buyer"/);
});

test("mobile navbar uses a plain beige shell and white glass lens", async () => {
	const source = await readFile(mobileBottomNavPath, "utf8");

	assert.match(source, /bg-hk-surface-muted/);
	assert.match(source, /bg-\[rgba\(255,255,255,0\.72\)\]/);
	assert.match(source, /backdrop-filter:blur\(10px\)_saturate\(115%\)/);
	assert.doesNotMatch(source, /gradient/);
	assert.doesNotMatch(source, /Curved caustic|chromatic split/);
	assert.match(source, /data-mobile-nav-active-indicator="true"/);
	assert.match(source, /setPointerCapture\?\.\(event\.pointerId\)/);
	// The indicator is a CSS transform transition now — framer-motion is no
	// longer imported into this globally mounted component just to run one
	// spring, and the pill no longer mixes two animation systems on one node.
	assert.doesNotMatch(source, /from "framer-motion"/);
	assert.match(source, /transition-\[transform,box-shadow\]/);
	assert.match(source, /translate3d\(\$\{target\.left\}px/);
	assert.match(source, /motion-reduce:\[transition:none\]/);
});
