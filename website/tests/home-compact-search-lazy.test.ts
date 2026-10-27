import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const shellPath = new URL(
	"../features/home/components/HomeCompactSearch.tsx",
	import.meta.url,
);
const experiencePath = new URL(
	"../features/home/components/HomeCompactSearchExperience.tsx",
	import.meta.url,
);

test("home search keeps the SSR form stable and loads its heavy experience on intent", async () => {
	const [shell, experience, queries] = await Promise.all([
		readFile(shellPath, "utf8"),
		readFile(experiencePath, "utf8"),
		readFile(
			new URL(
				"../features/home/components/useHomeSearchQueries.ts",
				import.meta.url,
			),
			"utf8",
		),
	]);

	assert.match(shell, /useLocalizedHref\("\/search"\)/);
	assert.match(shell, /action=\{searchAction\}/);
	assert.match(shell, /name="q"/);
	assert.match(shell, /value=\{query\}/);
	assert.match(shell, /搜尋新鮮食材\.\.\./);
	assert.match(shell, /import\("\.\/HomeCompactSearchExperience"\)/);
	assert.match(shell, /onFocusCapture=\{ensureExperience\}/);
	assert.match(shell, /onPointerEnter=\{scheduleExperience\}/);
	assert.match(shell, /window\.setTimeout\([\s\S]*140/);
	assert.match(shell, /onPointerLeave=\{cancelScheduledExperience\}/);
	assert.match(shell, /onPointerDown=\{ensureExperience\}/);
	assert.doesNotMatch(shell, /from "@tanstack\/react-query"/);
	assert.doesNotMatch(shell, /\/api\/public\/search\/products/);

	assert.match(experience, /from "@tanstack\/react-query"/);
	assert.match(experience, /useHomeSearchQueries\(open, requestQuery\)/);
	assert.match(queries, /\/api\/public\/search\/products/);
	assert.match(experience, /enabled: open/);
});

test("home search preserves draft, IME, retry, close, and abort recovery guards", async () => {
	const [shell, experience] = await Promise.all([
		readFile(shellPath, "utf8"),
		readFile(experiencePath, "utf8"),
	]);

	assert.match(shell, /const \[query, setQuery\] = React\.useState\(""\)/);
	assert.match(shell, /const \[committedQuery, setCommittedQuery\]/);
	assert.match(
		shell,
		/composingRef\.current \|\| event\.nativeEvent\.isComposing/,
	);
	assert.match(shell, /setCommittedQuery\(""\)/);
	assert.match(shell, /requestQuery=\{committedQuery\}/);
	assert.match(shell, /searchExperiencePromise = null/);
	assert.match(shell, /data-home-search-import-retry/);
	assert.match(shell, /suppressNextFocusOpenRef\.current = true/);
	assert.match(shell, /previousPathnameRef\.current !== pathname/);
	assert.match(shell, /target\.closest\("\[data-home-search-backdrop\]"\)/);
	assert.match(shell, /event\.key !== "Escape"/);
});
