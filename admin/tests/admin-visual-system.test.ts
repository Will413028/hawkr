import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import test from "node:test";

const read = (path: string) => readFile(path, "utf8");

test("Admin shell uses the current Hawkr brand assets and visual layer", async () => {
	const [rootLayout, brand, sidebar, visualCss] = await Promise.all([
		read("src/app/layout.tsx"),
		read("src/components/Layout/AdminBrand.tsx"),
		read("src/components/Layout/Sidebar.tsx"),
		read("src/styles/AdminVisualRefresh.css"),
	]);

	await Promise.all([
		access("public/brand/logo-horizontal.svg"),
		access("public/brand/logo-horizontal-light.svg"),
		access("public/brand/logo-mark.svg"),
	]);

	assert.ok(
		rootLayout.indexOf('import "./globals.css"') <
			rootLayout.indexOf('import "@/styles/AdminVisualRefresh.css"'),
		"visual compatibility layer must load after legacy globals",
	);
	assert.match(rootLayout, /icon: "\/brand\/logo-mark\.svg"/u);
	assert.match(brand, /\/brand\/logo-horizontal\.svg/u);
	assert.match(brand, /\/brand\/logo-horizontal-light\.svg/u);
	assert.match(sidebar, /<AdminBrand tone="light"/u);
	assert.match(sidebar, /aria-current=/u);
	assert.match(visualCss, /--admin-primary-strong: #b8441f/u);
	assert.match(visualCss, /--admin-cream: #f8f6ef/u);
	assert.match(visualCss, /--sidebar-width: 256px/u);
	assert.doesNotMatch(visualCss, /linear-gradient|radial-gradient/u);
});

test("Admin shell exposes an operable mobile navigation and skip link", async () => {
	const [layout, sidebar, topbar, visualCss] = await Promise.all([
		read("src/app/admin/layout.tsx"),
		read("src/components/Layout/Sidebar.tsx"),
		read("src/components/Layout/Topbar.tsx"),
		read("src/styles/AdminVisualRefresh.css"),
	]);

	assert.match(layout, /admin-skip-link/u);
	assert.match(layout, /admin-sidebar-scrim/u);
	assert.match(layout, /id="admin-main-content"/u);
	assert.match(layout, /useEscapeKey/u);
	assert.match(layout, /useFocusTrap/u);
	assert.match(layout, /useModalStack/u);
	assert.match(sidebar, /aria-label="關閉主選單"/u);
	assert.match(sidebar, /aria-hidden=/u);
	assert.match(sidebar, /aria-modal=/u);
	assert.match(sidebar, /inert=/u);
	assert.match(topbar, /aria-label="開啟主選單"/u);
	assert.match(visualCss, /@media \(max-width: 960px\)/u);
	assert.match(visualCss, /\.sidebar\.open/u);
});

test("Primary Admin identity surfaces no longer reference retired raster logos", async () => {
	const sidebar = await read("src/components/Layout/Sidebar.tsx");
	assert.doesNotMatch(sidebar, /Logo_Square|logo_word|\/favicon\.png/u);
});
