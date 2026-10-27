import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = (path: string) =>
	readFile(new URL(`../${path}`, import.meta.url), "utf8");

test("merchant awards alone own the trophy lazy boundary and explicit fallback", async () => {
	const [merchants, shell, glyph] = await Promise.all([
		source("features/home/components/HomeCompactMerchants.tsx"),
		source("features/home/components/HomeMerchantPrizeIcon.tsx"),
		source("features/home/components/HomeMerchantPrizeGlyph.tsx"),
	]);
	const awardBranch = merchants.match(
		/\{seller\.prize \? \(([\s\S]{0,800}?)\) : null\}/,
	)?.[1];

	assert.ok(
		awardBranch,
		"the no-award branch must remain null instead of mounting the icon loader",
	);
	assert.match(awardBranch, /<HomeMerchantPrizeIcon \/>/);
	assert.match(awardBranch, /\{seller\.prize\}/);
	assert.doesNotMatch(merchants, /from "@\/components\/shared\/UiIcon"/);

	assert.match(
		shell,
		/const PrizeGlyph = React\.lazy\(\(\) => import\("\.\/HomeMerchantPrizeGlyph"\)\)/,
	);
	assert.match(
		shell,
		/<React\.Suspense fallback=\{<PrizeIconReservation \/>\}>[\s\S]{0,120}<PrizeGlyph \/>[\s\S]{0,80}<\/React\.Suspense>/,
	);
	assert.doesNotMatch(shell, /from "next\/dynamic"/);
	assert.doesNotMatch(shell, /ssr:\s*false/);
	assert.match(shell, /aria-hidden="true"/);
	assert.match(shell, /style=\{\{ width: 11, height: 11 \}\}/);
	assert.match(shell, /static getDerivedStateFromError/);
	assert.match(shell, /this\.state\.failed \? <PrizeIconReservation \/>/);

	assert.match(glyph, /<AppIcon/);
	assert.match(glyph, /name="trophy"/);
	assert.match(glyph, /size=\{11\}/);
	assert.doesNotMatch(glyph, /title=/);
});
