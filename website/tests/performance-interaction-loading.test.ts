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

test("navbar keeps menu data and heavy panel code out of its initial module", async () => {
	const navbar = executable(await source("components/shared/Navbar.tsx"));

	assert.doesNotMatch(navbar, /from "@tanstack\/react-query"/);
	assert.doesNotMatch(navbar, /fetchMegaMenuDynamicLinks|fetchCategoryTree/);
	assert.doesNotMatch(navbar, /fetchTopWeeklyPosts|fetchPopularHashtags/);
	assert.doesNotMatch(navbar, /fetchTrendingPostTags/);
	assert.doesNotMatch(navbar, /MegaMenuPanel/);
	assert.match(navbar, /megaMenuDynamicLinks\?: MegaMenuDynamicLinks/);
});

test("mega menu keeps SSR triggers while loading its heavy panel only from an async boundary", async () => {
	const [menu, surface, panel] = await Promise.all([
		source("components/shared/MegaMenu.tsx").then(executable),
		source("components/shared/MegaMenuSurface.tsx").then(executable),
		source("components/shared/MegaMenuPanel.tsx").then(executable),
	]);

	assert.match(menu, /motion\.span/);
	// The panel is a content section, not a menu, so the trigger dropped
	// aria-haspopup for the disclosure pair (aria-expanded + aria-controls).
	assert.match(menu, /aria-expanded=\{isOpen\}/);
	assert.match(menu, /aria-controls=\{panelId\}/);
	assert.match(menu, /<section/);
	assert.match(
		menu,
		/import\(["'](?:\.\/|@\/components\/shared\/)MegaMenuPanel["']\)/,
	);
	assert.doesNotMatch(
		menu,
		/^import\s+(?!type\b)[^;\n]*MegaMenuPanel[^;\n]*from ["'][^"']+["'];$/m,
	);
	for (const heavySymbol of [
		"AppImage",
		"BannerCarousel",
		"RotatingProduceIcon",
		"postCardPresetForMedia",
	]) {
		assert.doesNotMatch(menu, new RegExp(`\\b${heavySymbol}\\b`));
		assert.match(panel, new RegExp(`\\b${heavySymbol}\\b`));
	}
	assert.match(menu, /data-mega-menu-loading/);
	assert.match(panel, /data-mega-menu-content/);
	assert.match(menu, /activeItem\?\.panel\s*\?/);
	assert.match(menu, /<Panel\s+[\s\S]{0,120}item=\{activeItem\}/);
	assert.match(surface, /MegaMenuSurface/);
});

test("menu intent preloads one retryable, deduplicated panel import", async () => {
	const menu = executable(await source("components/shared/MegaMenu.tsx"));
	assert.match(menu, /const OPEN_DELAY_MS = 140/);
	assert.match(
		menu,
		/export\s+(?:const\s+loadMegaMenuPanel\s*=|function\s+loadMegaMenuPanel)/,
	);

	const loaderStart = menu.indexOf("loadMegaMenuPanel");
	const loaderContract = menu.slice(loaderStart, loaderStart + 1_500);
	assert.match(loaderContract, /if\s*\(!panelRequest\)/);
	assert.match(loaderContract, /return panelRequest/);
	assert.match(
		loaderContract,
		/\(error\)\s*=>\s*\{[\s\S]{0,160}panelRequest\s*=\s*null;[\s\S]{0,80}throw error;/,
		"a failed import must clear its request cache before rethrowing",
	);

	const prewarm = menu.slice(
		menu.indexOf("function prewarmPanel"),
		menu.indexOf("export default function MegaMenu"),
	);
	assert.match(prewarm, /loadMegaMenuPanel\s*\(\)\.catch/);

	const effect = menu.slice(
		menu.indexOf("React.useEffect(() =>"),
		menu.indexOf("const openTimer"),
	);
	assert.match(effect, /if \(!openId \|\| Panel\) return/);
	assert.match(effect, /loadMegaMenuPanel\s*\(\)/);
	assert.match(effect, /\[openId, Panel, retry\]/);

	const scheduleOpen = menu.slice(
		menu.indexOf("const scheduleOpen"),
		menu.indexOf("const scheduleClose"),
	);
	assert.match(scheduleOpen, /setTimeout\s*\(/);
	assert.match(scheduleOpen, /prewarmPanel\s*\(\)/);
	assert.match(scheduleOpen, /setOpenId\(id\)/);

	const open = menu.slice(
		menu.indexOf("const open ="),
		menu.indexOf("const close ="),
	);
	assert.match(open, /prewarmPanel\s*\(\)/);
	assert.match(open, /setOpenId\(id\)/);

	const panelTrigger = menu.slice(menu.indexOf("aria-controls={panelId}"));
	assert.match(panelTrigger, /onPointerDown=/);
	assert.match(panelTrigger, /pointerType !== "mouse"\) prewarmPanel\s*\(\)/);
	assert.match(
		panelTrigger,
		/matches\(":focus-visible"\)[\s\S]{0,80}open\(item\.label\)/,
	);
	assert.match(menu, /onClick=\{\(\) => setRetry\(\(value\) => value \+ 1\)\}/);
});

test("seller and development role chrome stay outside the guest and buyer entry modules", async () => {
	const [layout, roleOwner, navbar, loader, seller, devSlot] =
		await Promise.all([
			source("components/shared/RootDocument.tsx").then(executable),
			source("components/shared/RoleAwareNavbar.tsx").then(executable),
			source("components/shared/Navbar.tsx").then(executable),
			source("components/shared/SellerNavbarLoader.tsx").then(executable),
			source("components/shared/SellerNavbar.tsx").then(executable),
			source("components/shared/DevRoleSwitcherSlot.tsx").then(executable),
		]);

	assert.match(
		roleOwner,
		/<SellerHeaderLoader onboarding=\{isSellerOnboardingRoute\} \/>/,
	);
	assert.match(navbar, /<SellerMenuLoader/);
	for (const initialOwner of [roleOwner, navbar, loader, devSlot]) {
		assert.doesNotMatch(initialOwner, /from "@\/components\/shared\/UiIcon"/);
		assert.doesNotMatch(
			initialOwner,
			/from "@\/components\/shared\/AppIcon(?:\/Control)?"/,
		);
		assert.doesNotMatch(initialOwner, /from "@hawkr\/icons\//);
	}
	assert.doesNotMatch(
		roleOwner,
		/^import\s+(?!type\b)[^;\n]*SellerNavbar[^;\n]*from/m,
	);
	assert.doesNotMatch(
		navbar,
		/^import\s+(?!type\b)[^;\n]*SellerNavbar["'][^;\n]*from/m,
	);
	assert.match(
		loader,
		/createSellerNavbarResource\(\s*\(\) => import\("@\/components\/shared\/SellerNavbar"\)/,
	);
	// The UiIcon shim is retired; the seller header names the registry directly.
	assert.match(seller, /from "@\/components\/shared\/AppIcon"/);

	assert.match(layout, /DEV_OVERLAYS_ENABLED &&/);
	assert.match(layout, /<DevRoleSwitcherSlot \/>/);
	assert.doesNotMatch(layout, /import RoleSwitcher from/);
	assert.match(
		devSlot,
		/dynamic\(\(\) => import\("@\/components\/shared\/RoleSwitcher"\)/,
	);
	assert.doesNotMatch(devSlot, /ssr:\s*false/);
});
