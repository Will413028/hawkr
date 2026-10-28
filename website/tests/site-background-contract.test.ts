import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import postcss from "postcss";

const source = (path: string) =>
	readFile(new URL(`../${path}`, import.meta.url), "utf8");

async function values(path: string, selector: string, property: string) {
	const found: string[] = [];
	postcss.parse(await source(path)).walkRules((rule) => {
		if (!rule.selectors.includes(selector)) return;
		rule.each((node) => {
			if (node.type === "decl" && node.prop === property)
				found.push(node.value);
		});
	});
	return found;
}

test("the decorative host is visible without a route allowlist", async () => {
	const path = "styles/components/site-grainient.css";
	assert.deepEqual(await values(path, ".site-grainient", "display"), ["block"]);
	assert.deepEqual(await values(path, ".site-grainient", "opacity"), ["0.25"]);
	assert.doesNotMatch(await source(path), /:has\(/);
	assert.deepEqual(await values(path, ".site-background-control", "display"), [
		"block",
	]);
});

test("the shared base stays the original market token in both themes", async () => {
	const colors = "styles/foundation/00-color-config.css";
	assert.deepEqual(await values(colors, ":root", "--site-grainient-base"), [
		"var(--theme-surface-page)",
	]);
	assert.deepEqual(
		await values("styles/foundation/02-reset-layout.css", "body", "background"),
		["var(--site-grainient-base)"],
	);
	assert.deepEqual(
		await values(
			"styles/foundation/01-tokens-theme.css",
			'[data-theme="dark"] body',
			"background",
		),
		["var(--site-grainient-base)"],
	);
});

test("page owners expose the canvas while loading masks remain opaque", async () => {
	for (const [path, selector] of [
		["styles/features/home.css", ".hkh-page"],
		["styles/features/home.css", '[data-theme="dark"] .hkh-page'],
		["styles/globals.css", "body .page-wrapper:has(.account-shell)"],
		["styles/globals.css", "body .page-wrapper:has(.seller-shell)"],
		[
			"styles/features/market-products-list.css",
			"body .page-content:has(.market-page)",
		],
		[
			"styles/features/feed-components.css",
			"body .page-content:has(.seller-tab-frame)",
		],
		[
			"styles/components/route-loading-skeleton.css",
			".route-loading-skeleton--page",
		],
	]) {
		assert.deepEqual(
			await values(path, selector, "background"),
			["transparent"],
			selector,
		);
	}
	assert.notDeepEqual(
		await values(
			"styles/components/route-loading-skeleton.css",
			".route-loading-skeleton--scoped",
			"background",
		),
		["transparent"],
	);
});

test("the persistent owner stays outside route content and has no route subscription", async () => {
	const providers = await source("components/shared/AppProviders.tsx");
	assert.equal(providers.match(/<SiteGrainientBackground\s*\/>/g)?.length, 1);
	assert.ok(
		providers.indexOf("<SiteGrainientBackground />") <
			providers.indexOf("{children}"),
	);
	assert.doesNotMatch(
		await source("components/shared/SiteGrainientBackground.tsx"),
		/next\/navigation|usePathname|key=|setInterval/,
	);
	assert.match(
		providers,
		/import "@\/styles\/components\/site-grainient\.css"/,
	);
});

test("standalone and delegated flows keep a pause control without role-specific footer links", async () => {
	const footer = await source("components/shared/RoleAwareFooter.tsx");
	assert.match(
		footer,
		/if \(isStandaloneFlowPath\(pathname\) \|\| isDelegatedWorkspacePath\(pathname\)\) \{\s*return \(\s*<footer className="site-background-footer">\s*<SiteBackgroundToggle variant="page" \/>/,
	);
});

test("Agent route state shares the canvas but preserves its status mark", async () => {
	const path = "styles/features/procurement-assistant.css";
	assert.deepEqual(
		await values(path, ".assistant-route-state.thread-page", "background"),
		["transparent"],
	);
	const statusBackgrounds = await values(
		path,
		".assistant-route-state__mark",
		"background",
	);
	assert.equal(statusBackgrounds.length, 1);
	assert.notEqual(statusBackgrounds[0], "transparent");
});
