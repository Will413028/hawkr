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

// 2026-09-10 產品負責人拍板：hero 輪播補上 上一張／下一張 方向鍵。
// 這是 source contract 測試（元件是 client component，需要 Query provider 才能
// 渲染），釘住的是「不能被下一次改動悄悄拿掉」的幾件事。
test("hero carousel ships 上一張 / 下一張 controls, megamenu compact does not", () => {
	const src = source(CAROUSEL);

	// 兩顆鈕、繁中 aria-label、指向投影片區域
	assert.match(src, /aria-label="上一張"/);
	assert.match(src, /aria-label="下一張"/);
	const ariaControls = src.match(/aria-controls=\{slideRegionId\}/g) ?? [];
	assert.equal(
		ariaControls.length,
		2,
		"both arrows must point at the slide region they drive",
	);

	// compact（Hawkr 市集 megamenu 右欄）沒有空間，只在 hero 出現；
	// 單張 banner 也不出現（與 CarouselDots 的 count < 2 一致）。
	assert.match(src, /\{multi && size !== "compact" \? \(/);

	// 圖示走 AppIcon registry，不得內嵌 SVG path 或 emoji
	assert.match(src, /import AppIcon from "@\/components\/shared\/AppIcon";/);
	assert.match(src, /name="chevron-left"/);
	assert.match(src, /name="chevron-right"/);
});

test("arrow chrome reuses the theme-toggle tokens and stays motion-safe", () => {
	const src = source(CAROUSEL);
	const chrome = src.match(/const ARROW_CHROME =[\s\S]*?;\n/)?.[0];
	assert.ok(chrome, "ARROW_CHROME must stay a single reviewed class list");

	// 霧面玻璃色值一律走 --page-theme-toggle-* token，不得硬編碼色碼
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

	// 44×44 觸控目標，垂直置中疊在投影片上（不壓到底部字幕，也不壓到 dots）
	assert.match(chrome, /(?:^| )size-11(?: |")/);
	assert.match(chrome, /"absolute top-1\/2 /);
	assert.match(chrome, /-translate-y-1\/2/);

	// 動效：只做顏色回饋，reduced motion 全關
	assert.match(chrome, /motion-reduce:transition-none/);
	assert.match(chrome, /motion-reduce:active:scale-100/);
	assert.doesNotMatch(chrome, /transition-all/);
});

test("arrow navigation wraps and restarts the autoplay clock", () => {
	const src = source(CAROUSEL);

	// 方向鍵繞圈（swipe 的 `step` 會在頭尾夾住，方向鍵不該）
	assert.match(
		src,
		/setActive\(\(i\) => \(i \+ dir \+ banners\.length\) % banners\.length\);/,
	);

	// 點下去先給 autoplay 一個完整的間隔再自己往前：用 `paused` 的一次性脈衝
	// 讓 useCarouselAutoplay 重建 timer，不另外開 hook API。
	assert.match(src, /const stepByArrow = \(dir: number\) => \{/);
	assert.match(src, /setAutoplayRestart\(true\);/);
	assert.match(src, /paused: dragging \|\| autoplayRestart,/);
});
