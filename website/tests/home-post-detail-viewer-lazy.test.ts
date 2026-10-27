import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const source = (path: string) =>
	readFile(new URL(`../${path}`, import.meta.url), "utf8");

function executable(value: string): string {
	return value
		.replaceAll(/\/\*[\s\S]*?\*\//g, "")
		.replaceAll(/^\s*\/\/.*$/gm, "");
}

test("home keeps its post cards in SSR while the heavy viewer has one async owner", async () => {
	const [home, stage, card, boundary, loader] = await Promise.all([
		source("features/home/HomePage.tsx").then(executable),
		source("features/home/components/HomeCompactMarketStage.tsx").then(
			executable,
		),
		source("features/home/components/HomeStageFeedCard.tsx").then(executable),
		source("features/home/components/HomePostDetailViewer.tsx").then(
			executable,
		),
		source("features/home/components/HomePostDetailViewerLoader.ts").then(
			executable,
		),
	]);

	assert.match(
		home,
		/import HomeCompactMarketStage from "@\/features\/home\/components\/HomeCompactMarketStage"/,
	);
	assert.match(home, /<HomeCompactMarketStage[\s\S]{0,160}posts=\{/);
	assert.doesNotMatch(home, /dynamic\([\s\S]{0,120}HomeCompactMarketStage/);
	assert.match(
		stage,
		/import HomeStageFeedCard from "@\/features\/home\/components\/HomeStageFeedCard"/,
	);
	assert.match(card, /role="button"/);
	assert.match(card, /tabIndex=\{0\}/);
	assert.match(card, /e\.key === "Enter" \|\| e\.key === " "/);

	const graph = [stage, card, boundary, loader].join("\n");
	assert.equal(
		graph.match(/import\("@\/features\/feed\/components\/PostDetailViewer"\)/g)
			?.length,
		1,
		"the loader must be the sole runtime owner of the heavy viewer import",
	);
	for (const [name, moduleSource] of [
		["stage", stage],
		["card", card],
		["boundary", boundary],
	] as const) {
		assert.doesNotMatch(
			moduleSource,
			/^import\s+(?!type\b)[^;\n]*from "@\/features\/feed\/components\/PostDetailViewer";$/m,
			`${name} must not statically own the heavy viewer`,
		);
	}
	assert.match(
		loader,
		/createHomePostDetailViewerResource\(\s*\(\)\s*=>\s*import\("@\/features\/feed\/components\/PostDetailViewer"\)/,
	);
});

test("home mounts and warms the post viewer only from explicit card intent", async () => {
	const [stage, card] = await Promise.all([
		source("features/home/components/HomeCompactMarketStage.tsx").then(
			executable,
		),
		source("features/home/components/HomeStageFeedCard.tsx").then(executable),
	]);

	assert.doesNotMatch(stage, /from "next\/dynamic"/);
	assert.doesNotMatch(stage, /const loadPostDetailViewer/);
	assert.doesNotMatch(
		stage,
		/React\.useEffect\(\(\) => \{\s*(?:void )?prewarmHomePostDetailViewer\(/,
	);
	assert.match(
		stage,
		/\{openFeedIndex !== null \? \([\s\S]{0,500}<HomePostDetailViewer[\s\S]{0,500}returnFocusRef=\{openFeedOpenerRef\}[\s\S]{0,100}\) : null\}/,
	);
	assert.match(stage, /openFeedOpenerRef\.current = opener/);
	assert.match(stage, /onPrewarmPostDetail=\{prewarmHomePostDetailViewer\}/);

	assert.match(card, /const POST_DETAIL_HOVER_PREWARM_MS = 140/);
	assert.match(
		card,
		/onPointerEnter=\{[\s\S]{0,240}schedulePostDetailPrewarm\(\)/,
	);
	assert.match(card, /onPointerLeave=\{cancelPostDetailPrewarm\}/);
	assert.match(card, /onPointerDown=\{[\s\S]{0,240}prewarmPostDetailNow\(\)/);
	assert.match(card, /onFocus=\{[\s\S]{0,180}prewarmPostDetailNow\(\)/);
	assert.match(
		card,
		/target instanceof Element && target\.closest\("a, button"\)/,
	);
	assert.match(
		card,
		/React\.useEffect\([\s\S]{0,140}\(\) => cancelPostDetailPrewarm\(\)/,
	);
	assert.match(
		card,
		/onOpenPost\(post, opener\?\.getBoundingClientRect\(\), opener\)/,
	);
});

test("the cold boundary exposes cancellable loading and retry states without changing the homepage route", async () => {
	const boundary = executable(
		await source("features/home/components/HomePostDetailViewer.tsx"),
	);

	assert.match(boundary, /data-home-post-detail-state=\{state\}/);
	assert.match(boundary, /aria-busy=\{state === "loading" \|\| undefined\}/);
	assert.match(boundary, /role="status"/);
	assert.match(boundary, /role="alert"/);
	assert.match(boundary, /data-home-post-detail-retry/);
	assert.match(boundary, /event\.key === "Escape"[\s\S]{0,100}onClose\(\)/);
	assert.match(boundary, /event\.key !== "Tab"/);
	assert.match(
		boundary,
		/restoreOpener\(\)[\s\S]{0,100}setState\(\{ status: "ready", Component \}\)/,
	);
	assert.match(boundary, /if \(!active\) return/);
	assert.doesNotMatch(
		boundary,
		/router\.(?:push|replace)|location\.(?:assign|replace)/,
	);
});
