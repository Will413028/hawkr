import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const stabilitySource = readFileSync(
	new URL("./e2e/site-stability.spec.ts", import.meta.url),
	"utf8",
);

test("the ready handoff gate requires React to detach the initial loader", () => {
	assert.match(
		stabilitySource,
		/function assertInitialLoaderHandoff[\s\S]*?not\.toHaveClass\(\/\\binitial-loader-active\\b\/\)[\s\S]*?locator\("\.initial-loader"\)[\s\S]*?toHaveCount\(0\)/,
	);
	assert.match(
		stabilitySource,
		/await assertInitialLoaderHandoff\(page, targetRoute\);\s+await assertNoResidualLoading/,
	);
});

test("the production stability gate inspects requests and every media-bearing DOM attribute", () => {
	assert.match(
		stabilitySource,
		/const ARTIFACT_FLAVOR = process\.env\.NEXT_PUBLIC_FLAVOR/,
	);
	assert.ok(
		stabilitySource.includes('"img[src], img[srcset], link[href]"'),
		"the DOM observer must inspect img src/srcset and link href",
	);
	assert.match(stabilitySource, /attributeFilter: \["src", "srcset", "href"\]/);
	assert.match(
		stabilitySource,
		/page\.on\("request", onProductionMediaRequest\)/,
	);
	assert.match(
		stabilitySource,
		/expect\(\s*forbiddenMediaRequestUrls,[\s\S]*?\)\.toEqual\(\[\]\);[\s\S]*?expect\(\s*forbiddenMediaDomReferences,[\s\S]*?\)\.toEqual\(\[\]\);/,
	);
});

test("the passive-scroll stability matrix includes a real product detail fixture", () => {
	assert.match(stabilitySource, /"\/products\/:id"/);
	assert.match(
		stabilitySource,
		/"\/products\/:id": '\[data-hk-page-content-ready="product-detail"\]'/,
	);
	assert.match(
		stabilitySource,
		/\/api\/v1\/public\/products\?keyword=&page=1&page_size=50[\s\S]*?productId: productId \|\| undefined/,
	);
	assert.match(
		stabilitySource,
		/test\.skip\(\s*!fixtures\.productId,[\s\S]*?dev data and source contracts retain populated PDP coverage/,
	);
});
