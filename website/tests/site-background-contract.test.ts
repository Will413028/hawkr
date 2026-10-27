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

test("RFQ preview exposes the site canvas without clearing its content sections", async () => {
	const path =
		"styles/features/procurement-rfq-prototype/rfq-prototype.module.css";
	assert.deepEqual(await values(path, ".prototype", "background"), [
		"transparent",
	]);
	assert.deepEqual(await values(path, ".section", "background"), [
		"var(--hkui-content1)",
	]);
});

test("external vendor canvas stays transparent at every breakpoint while cards retain their surfaces", async () => {
	const path =
		"styles/features/procurement-rfq-prototype/external-vendor-portal.module.css";
	// walkRules visits nested media queries too: a mobile override must not
	// silently bring back a second page illustration or opaque canvas.
	assert.deepEqual(await values(path, ".portal", "background"), [
		"transparent",
	]);
	assert.ok(
		(await values(path, ".portal", "background-color")).every(
			(value) => value === "transparent",
		),
	);
	assert.ok(
		(await values(path, ".portal", "background-image")).every(
			(value) => value === "none",
		),
	);
	assert.doesNotMatch(await source(path), /page-bg-tall(?:-mobile)?\.webp/);
	assert.deepEqual(await values(path, ".card", "background"), [
		"var(--hkui-content1)",
		"var(--portal-glass-strong)",
	]);
	const topbarBackgrounds = await values(path, ".topbar", "background");
	assert.ok(topbarBackgrounds.length > 0);
	assert.ok(topbarBackgrounds.every((value) => value !== "transparent"));
});

test("external vendor pale panels retain readable foreground roles over the dark site canvas", async () => {
	const path =
		"styles/features/procurement-rfq-prototype/external-vendor-portal.module.css";
	for (const [property, expected] of [
		["--hk-text-body", "var(--color-neutral-body)"],
		["--hk-text-strong", "var(--color-neutral-ink)"],
		["--hk-text-soft", "var(--color-neutral-muted-text)"],
	]) {
		assert.deepEqual(await values(path, ".portal", property), [expected]);
	}
	assert.deepEqual(
		await values(
			path,
			'[data-theme="dark"] .portal a:not([data-hkui-button])',
			"color",
		),
		["var(--hkui-primary)"],
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
