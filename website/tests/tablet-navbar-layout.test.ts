import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const navbarPath = new URL("../components/shared/Navbar.tsx", import.meta.url);
const navigationStylesPath = new URL(
	"../styles/components/navigation-final.css",
	import.meta.url,
);
const megaMenuPath = new URL(
	"../components/shared/MegaMenu.tsx",
	import.meta.url,
);
const categoryNavPath = new URL(
	"../features/home/components/HomeCompactCategoryNav.tsx",
	import.meta.url,
);
const homeStylesPath = new URL("../styles/features/home.css", import.meta.url);

test("841-1023px navbar uses the compact brand mark without hiding primary navigation", async () => {
	const [source, styles] = await Promise.all([
		readFile(navbarPath, "utf8"),
		readFile(navigationStylesPath, "utf8"),
	]);

	assert.match(source, /<BrandLogo className="site-nav-logo--full" \/>/);
	assert.match(
		source,
		/variant="mark"[\s\S]*?className="site-nav-logo--tablet"/,
	);
	assert.match(source, /SITE_NAV_CENTER[\s\S]*?max-width:840px/);
	assert.match(
		styles,
		/body \.site-nav \.site-nav-logo--tablet \{\s*display: none/,
	);
	assert.match(styles, /@media \(min-width: 841px\) and \(max-width: 1023px\)/);
	assert.match(styles, /\.site-nav-logo--full \{\s*display: none/);
	assert.match(styles, /\.site-nav-logo--tablet \{\s*display: inline-flex/);
});

test("841-1023px mega menu stays compact and bounded by the visible viewport", async () => {
	const source = (
		await Promise.all([
			readFile(megaMenuPath, "utf8"),
			readFile(
				new URL("../components/shared/MegaMenuSurface.tsx", import.meta.url),
				"utf8",
			),
			readFile(
				new URL("../components/shared/MegaMenuPanel.tsx", import.meta.url),
				"utf8",
			),
		])
	).join("\n");

	assert.match(
		source,
		/max-h-\[calc\(100dvh-var\(--site-nav-inner-height,100px\)-12px\)\]/,
	);
	assert.match(source, /overflow-y-auto/);
	assert.match(source, /overscroll-contain/);
	assert.match(source, /items-start justify-center overflow-y-auto/);
	assert.match(source, /max-\[1024px\]:gap-6/);
	assert.match(source, /\[@media\(max-width:840px\)\]:flex-col/);
	assert.match(source, /\[@media\(max-width:840px\)\]:flex-none/);
	assert.doesNotMatch(source, /max-\[1024px\]:flex-col/);
	assert.doesNotMatch(source, /max-\[1024px\]:w-full/);
});

test("home category shortcuts flow into two rows at tablet widths", async () => {
	const [component, styles] = await Promise.all([
		readFile(categoryNavPath, "utf8"),
		readFile(homeStylesPath, "utf8"),
	]);

	const railClasses = component
		.match(/className="([^"]*\bhome-category-rail\b[^"]*)"/)?.[1]
		.split(/\s+/);
	assert.ok(railClasses?.includes("home-category-rail"));
	assert.ok(railClasses?.includes("home-section-surface"));
	assert.ok(railClasses?.includes("flex"));
	assert.match(styles, /@media \(min-width: 841px\) and \(max-width: 1023px\)/);
	assert.match(styles, /grid-template-columns: repeat\(7, max-content\)/);
	assert.match(styles, /grid-auto-flow: row/);
});
