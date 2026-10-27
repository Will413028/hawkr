import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import AppIcon, { type AppIconName } from "../components/shared/AppIcon";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

test("produce icons rotate through generic Material glyphs without changing AppIcon", () => {
	const icons = source("../components/shared/AppIcon.tsx");
	const rotation = source("../components/shared/RotatingProduceIcon.tsx");
	const produce = source("../components/shared/ProduceIcon.tsx");
	const megaMenu = source("../components/shared/MegaMenuPanel.tsx");
	const namesBlock = produce.match(
		/PRODUCE_ICON_NAMES = \[([\s\S]*?)\] as const/,
	);

	assert.ok(namesBlock);
	const names = [...namesBlock[1].matchAll(/"([a-z-]+)"/g)].map(
		(match) => match[1],
	);
	assert.ok(names.length >= 2);
	assert.equal(new Set(names).size, names.length);
	assert.match(icons, /leaf: "eco"/);
	assert.match(megaMenu, /<RotatingProduceIcon[\s\S]*size=\{32\}/);
	assert.doesNotMatch(icons, /RotatingProduceIcon/);
	assert.match(rotation, /prefers-reduced-motion: reduce/);
	assert.match(rotation, /PRODUCE_ICON_NAMES\.length - 1/);
	assert.doesNotMatch(rotation, /new Image\(\)/);
});

test("seller navigation draws its icons through SellerSidebarIcon", () => {
	// 選單資料（含每一項的 icon）住在 sellerNav.tsx，登出鈕的 icon 仍在 shell —
	// ADM 代行工作區沿用同一份資料（56 號 §6.1.14）。這道斷言守的是「賣家導覽用
	// 專屬 icon 組」，它與資料放在哪一支檔案無關，所以兩支一起掃。
	const shell = [
		source("../features/seller-center/components/SellerCenterShell.tsx"),
		source("../features/seller-center/components/sellerNav.tsx"),
	].join("\n");
	const sidebarIcons = source("../components/shared/SellerSidebarIcon.tsx");

	for (const name of [
		"chart",
		"preview",
		"box",
		"goods",
		"stock",
		"edit",
		"ticket",
		"users",
		"wallet",
		"truck",
		"shield",
		"settings",
		"chat",
		"notification",
		"logout",
	]) {
		assert.match(shell, new RegExp(`<SellerSidebarIcon name="${name}"`));
	}
	assert.doesNotMatch(
		shell,
		/function Icon(?:Chart|Box|Bag|Cart|Edit|Ticket|Users|Wallet|Truck|Shield|Settings|Chat|SignOut)/,
	);
	assert.match(sidebarIcons, /data-seller-sidebar-icon=\{name\}/);
	assert.match(sidebarIcons, /<AppIcon/);
	assert.doesNotMatch(sidebarIcons, /<(?:svg|path)\b|\bd="M/);
});

test("the shared registry maps semantic names to Material Symbols and owns no geometry", () => {
	const icons = source("../components/shared/AppIcon.tsx");

	assert.match(icons, /from "@hawkr\/icons\/material-symbol"/);
	assert.match(icons, /const MATERIAL_SYMBOL_BY_NAME/);
	assert.doesNotMatch(icons, /<(?:path|line|polyline|svg)\b|\bd="M/);
});

test("the home post loading shell closes through AppIcon", () => {
	const boundary = source(
		"../features/home/components/HomePostDetailViewer.tsx",
	);

	assert.match(
		boundary,
		/import AppIcon from "@\/components\/shared\/AppIcon"/,
	);
	assert.match(boundary, /<AppIcon name="close" size=\{22\} \/>/);
});

test("every semantic name renders one Material path in the shared 960 box", () => {
	const names = [
		...source("../components/shared/AppIcon.tsx").matchAll(
			/^\t"?([a-z0-9-]+)"?: "[a-z0-9_]+",$/gm,
		),
	].map((match) => match[1]);
	assert.ok(names.length > 50);

	for (const name of names) {
		const html = renderToStaticMarkup(
			React.createElement(AppIcon, { name: name as AppIconName, size: 20 }),
		);
		assert.match(html, new RegExp(`data-icon-name="${name}"`), name);
		assert.match(html, /viewBox="0 -960 960 960"/, name);
		assert.equal(html.match(/<path\b/g)?.length, 1, name);
		assert.match(html, /aria-hidden="true"/, name);
	}
});

test("AppIcon is decorative by default, named with a title, and fill selects the filled glyph", () => {
	const outlined = renderToStaticMarkup(
		React.createElement(AppIcon, { name: "star" }),
	);
	const filled = renderToStaticMarkup(
		React.createElement(AppIcon, { name: "star", fill: true }),
	);
	assert.notEqual(outlined, filled);
	assert.doesNotMatch(outlined, /role=/);
	assert.doesNotMatch(outlined, /aria-label=/);

	const labelled = renderToStaticMarkup(
		React.createElement(AppIcon, {
			name: "close",
			title: "關閉",
			size: 19,
			className: "probe",
			spin: true,
			"data-probe": "ok",
		}),
	);
	assert.match(labelled, /role="img"/);
	assert.match(labelled, /aria-label="關閉"/);
	assert.doesNotMatch(labelled, /aria-hidden=/);
	assert.match(labelled, /width="19" height="19"/);
	// `spin` carries its own reduced-motion escape hatch.
	assert.match(
		labelled,
		/class="hk-app-icon animate-spin motion-reduce:animate-none probe"/,
	);
	assert.match(labelled, /data-probe="ok"/);
});

test("feature CSS does not reach into shared icon geometry", () => {
	const styles = [
		source("../styles/features/procurement-assistant.css"),
		source("../styles/features/seller-pricing.css"),
	];
	const primitiveSelector =
		/\.hk-app-icon\s+(?:path|line|polyline)|svg\s+(?:path|line|polyline):first-(?:child|of-type)/;

	for (const css of styles) assert.doesNotMatch(css, primitiveSelector);
});

test("shared error screens show the warning icon in the status disc", () => {
	const errorState = source("../components/shared/ErrorState.tsx");

	assert.match(
		errorState,
		/<AppIcon[\s\S]{0,120}?name="danger"[\s\S]{0,120}?className="size-full"/,
	);
});
