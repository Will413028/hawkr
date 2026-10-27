import assert from "node:assert/strict";
import test from "node:test";
import {
	type DocumentResourceHints,
	partitionStabilityWarnings,
	warnedResourceUrl,
} from "./e2e/helpers/preload-hygiene";

const CSS = "https://web.example/_next/static/chunks/seller.css?dpl=abc";
const IMG = "https://web.example/hero.png";

function unusedPreloadWarning(url: string): string {
	return `The resource ${url} was preloaded using link preload but not used within a few seconds from the window's load event. Please make sure it has an appropriate \`as\` value and it is preloaded intentionally.`;
}

function hints(
	overrides: Partial<DocumentResourceHints>,
): DocumentResourceHints {
	return {
		authoredPreloads: [],
		styleHints: [],
		appliedStylesheets: [],
		...overrides,
	};
}

test("names the resource Chrome complained about", () => {
	assert.equal(warnedResourceUrl(unusedPreloadWarning(CSS)), CSS);
	assert.equal(warnedResourceUrl("something else entirely"), null);
});

test("a stylesheet hint the router added for another route is a warm-up, not a defect", () => {
	// 首頁的商家 rail 用 router.prefetch() 為 storefront 暖身，連帶預載了
	// storefront 的 CSS。訪客沒點進去，Chrome 就報 unused —— 但那支 CSS 本來
	// 就不屬於這一頁，這條 gate 不該因此紅。
	const { defects, routeWarmups } = partitionStabilityWarnings(
		[unusedPreloadWarning(CSS)],
		hints({ styleHints: [CSS] }),
	);

	assert.deepEqual(defects, []);
	assert.equal(routeWarmups.length, 1);
});

test("a stylesheet this page actually applied stays a defect", () => {
	// 掛成 rel=stylesheet 就代表是本頁的 CSS，預載到卻沒用上是真問題。
	const { defects, routeWarmups } = partitionStabilityWarnings(
		[unusedPreloadWarning(CSS)],
		hints({ styleHints: [CSS], appliedStylesheets: [CSS] }),
	);

	assert.equal(defects.length, 1);
	assert.deepEqual(routeWarmups, []);
});

test("a hand-authored stylesheet preload stays a defect", () => {
	// 文件解析完成當下就存在 = 寫在 layout 裡的提示，沒被用到就是寫錯了。
	const { defects, routeWarmups } = partitionStabilityWarnings(
		[unusedPreloadWarning(CSS)],
		hints({ styleHints: [CSS], authoredPreloads: [CSS] }),
	);

	assert.equal(defects.length, 1);
	assert.deepEqual(routeWarmups, []);
});

test("an unused preload that is not a stylesheet stays a defect", () => {
	// script / font / image 預載不到就是寫錯了，一律照舊。
	const { defects, routeWarmups } = partitionStabilityWarnings(
		[unusedPreloadWarning(IMG)],
		hints({ styleHints: [CSS] }),
	);

	assert.equal(defects.length, 1);
	assert.deepEqual(routeWarmups, []);
});

test("warnings that are not about unused preloads stay defects", () => {
	// 圖片幾何與 `as` 值寫錯這兩類警告完全不經過暖身判斷。
	const geometry =
		"Image with src https://web.example/a.png has either width or height modified, but not the other.";
	const badAs =
		"The resource https://web.example/a.woff2 was preloaded using link preload but not used within a few seconds. Please make sure it has an appropriate `as` value and it is preloaded intentionally.";
	const { defects } = partitionStabilityWarnings(
		[geometry, badAs],
		hints({ styleHints: [CSS] }),
	);

	assert.equal(defects.length, 2);
});
