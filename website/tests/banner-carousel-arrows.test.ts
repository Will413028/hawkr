import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function source(path: string): string {
	return readFileSync(join(ROOT, path), "utf8");
}

const CAROUSEL = "components/shared/banner/BannerCarousel.tsx";

test("hero carousel ships 上一張 / 下一張 controls, megamenu compact does not", () => {
	const src = source(CAROUSEL);

	assert.match(src, /aria-label="上一張"/);
	assert.match(src, /aria-label="下一張"/);
	const ariaControls = src.match(/aria-controls=\{slideRegionId\}/g) ?? [];
	assert.equal(
		ariaControls.length,
		2,
		"both arrows must point at the slide region they drive",
	);

	assert.match(src, /\{multi && size !== "compact" \? \(/);

	assert.match(src, /import AppIcon from "@\/components\/shared\/AppIcon";/);
	assert.match(src, /name="chevron-left"/);
	assert.match(src, /name="chevron-right"/);
});

test("arrow chrome reuses the theme-toggle tokens and stays motion-safe", () => {
	const src = source(CAROUSEL);
	const chrome = src.match(/const ARROW_CHROME =[\s\S]*?;\n/)?.[0];
	assert.ok(chrome, "ARROW_CHROME must stay a single reviewed class list");

	for (const token of [
		"--page-theme-toggle-bg",
		"--page-theme-toggle-border",
		"--page-theme-toggle-fg",
		"--page-theme-toggle-hover-bg",
	]) {
		assert.match(chrome, new RegExp(token.replace(/-/g, "\\-")));
	}
	assert.doesNotMatch(chrome, /#[0-9a-fA-F]{3,8}\b/);
	assert.doesNotMatch(chrome, /rgba?\(/);

	assert.match(chrome, /(?:^| )size-11(?: |")/);
	assert.match(chrome, /"absolute top-1\/2 /);
	assert.match(chrome, /-translate-y-1\/2/);

	assert.match(chrome, /motion-reduce:transition-none/);
	assert.match(chrome, /motion-reduce:active:scale-100/);
	assert.doesNotMatch(chrome, /transition-all/);
});

test("arrow navigation wraps and restarts the autoplay clock", () => {
	const src = source(CAROUSEL);

	assert.match(
		src,
		/setActive\(\(i\) => \(i \+ dir \+ banners\.length\) % banners\.length\);/,
	);

	assert.match(src, /const stepByArrow = \(dir: number\) => \{/);
	assert.match(src, /setAutoplayRestart\(true\);/);
	assert.match(src, /paused: dragging \|\| autoplayRestart,/);
});
