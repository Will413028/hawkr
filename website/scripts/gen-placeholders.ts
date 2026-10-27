/**
 * Placeholder images for the spots that used to hold photos, illustrations and
 * screenshots: a flat Hawkr brand colour with one Material Symbols glyph.
 *
 * Usage (from `website/`):
 *
 *   pnpm gen:placeholders
 *
 * Writes `public/placeholders/<name>.svg` from the `PLACEHOLDERS` table below
 * and removes files in that directory the table no longer lists. The table is
 * the only input besides two sources of truth:
 *
 *   - colours: `brand.*` in `design-system/cross-platform-tokens.json`
 *   - glyph paths: `@hawkr/icons` (Material Symbols, outlined)
 *
 * Output is deterministic (no timestamps, fixed attribute order, numbers
 * rounded to two decimals, one trailing newline), so a rerun leaves
 * `git status` clean; `tests/placeholders.test.ts` regenerates in memory and
 * compares byte for byte.
 *
 * To add a placeholder, add a row. Rows share a file when the spot has the same
 * purpose and aspect ratio; the width and height are the frame the image fills.
 * A new glyph name must first be added to `packages/hawkr-icons-ts/src/names.ts`.
 */
import {
	mkdirSync,
	readdirSync,
	readFileSync,
	unlinkSync,
	writeFileSync,
} from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import type { MaterialSymbolName } from "@hawkr/icons/material-symbol";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";

export const PLACEHOLDER_DIR = "public/placeholders";

type BrandToken =
	| "primary"
	| "secondary"
	| "ink"
	| "primarySoft"
	| "secondarySoft";

interface Cell {
	icon: MaterialSymbolName;
	/** Glyph colour token. */
	color: BrandToken;
}

interface PlaceholderSpec {
	/** File name without extension. */
	name: string;
	width: number;
	height: number;
	/** Background colour token; `null` leaves the image transparent. */
	background: BrandToken | null;
	/** Cells in row-major order; a plain placeholder is one cell. */
	cells: readonly Cell[];
	cols?: number;
	rows?: number;
	/** Glyph edge as a fraction of the shorter edge of its cell. */
	iconRatio?: number;
	/** Accessible name; omitted for purely decorative images. */
	label?: string;
}

const DEFAULT_ICON_RATIO = 0.3;

function one(icon: MaterialSymbolName, color: BrandToken): readonly Cell[] {
	return [{ icon, color }];
}

export const PLACEHOLDERS: readonly PlaceholderSpec[] = [
	// Avatars and generic image slots.
	{
		name: "avatar",
		width: 200,
		height: 200,
		background: "primarySoft",
		cells: one("person", "ink"),
		iconRatio: 0.5,
		label: "預設頭像",
	},
	{
		name: "post",
		width: 800,
		height: 600,
		background: "secondarySoft",
		cells: one("image", "ink"),
		label: "貼文圖片",
	},
	{
		name: "ai-style",
		width: 512,
		height: 512,
		background: "secondarySoft",
		cells: one("wand_stars", "ink"),
		label: "風格範例",
	},
	{
		name: "badge-character",
		width: 512,
		height: 512,
		background: "secondary",
		cells: one("celebration", "ink"),
		iconRatio: 0.6,
	},
	{
		name: "vision",
		width: 1024,
		height: 1024,
		background: "primarySoft",
		cells: one("yard", "primary"),
		iconRatio: 0.4,
	},

	// Screens of the product: one placeholder per frame size.
	{
		name: "phone-screen",
		width: 640,
		height: 1334,
		background: "primarySoft",
		cells: one("mobile", "ink"),
		label: "手機畫面",
	},
	{
		name: "app-screen",
		width: 720,
		height: 1564,
		background: "primarySoft",
		cells: one("mobile", "ink"),
		label: "App 畫面",
	},
	{
		name: "web-screen",
		width: 1040,
		height: 650,
		background: "primarySoft",
		cells: one("dashboard", "ink"),
		label: "網頁畫面",
	},

	// Illustrated scenes of the home and seller story trails.
	...(
		[
			["scene-sunrise", "wb_twilight"],
			["scene-orchard", "nutrition"],
			["scene-sunset", "grain"],
			["scene-irrigation", "water_drop"],
			["scene-greenhouse", "potted_plant"],
			["scene-market", "storefront"],
		] as const
	).map(
		([name, icon]): PlaceholderSpec => ({
			name,
			width: 1200,
			height: 1712,
			background: "secondarySoft",
			cells: one(icon, "primary"),
			iconRatio: 0.3,
		}),
	),

	// Seller recruitment scroll world: one poster per stage.
	...(
		[
			["birth", "groups"],
			["sorting", "inventory_2"],
			["distribution", "local_shipping"],
			["harvest", "agriculture"],
			["processing", "factory"],
			["retail", "storefront"],
			["table", "restaurant"],
		] as const
	).map(
		([stage, icon]): PlaceholderSpec => ({
			name: `journey-${stage}`,
			width: 640,
			height: 1136,
			background: "secondarySoft",
			cells: one(icon, "primary"),
			iconRatio: 0.4,
		}),
	),
];

const websiteRoot = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"..",
);

interface TokenFile {
	tokens: { id: string; value: string }[];
}

function loadBrandColors(): Record<BrandToken, string> {
	const file = JSON.parse(
		readFileSync(
			path.join(websiteRoot, "design-system/cross-platform-tokens.json"),
			"utf8",
		),
	) as TokenFile;
	const colors = {} as Record<BrandToken, string>;
	for (const name of [
		"primary",
		"secondary",
		"ink",
		"primarySoft",
		"secondarySoft",
	] as const) {
		const token = file.tokens.find((entry) => entry.id === `brand.${name}`);
		if (!token)
			throw new Error(`brand.${name} is missing from the tokens file`);
		colors[name] = token.value;
	}
	return colors;
}

function glyphPath(icon: MaterialSymbolName): string {
	const markup = renderToStaticMarkup(
		createElement(MaterialSymbol, { name: icon }),
	);
	const match = /<path d="([^"]+)"/.exec(markup);
	if (!match) throw new Error(`no path for icon ${icon}`);
	return match[1];
}

function num(value: number): string {
	return String(Number(value.toFixed(2)));
}

function renderSpec(
	spec: PlaceholderSpec,
	colors: Record<BrandToken, string>,
): string {
	const cols = spec.cols ?? 1;
	const rows = spec.rows ?? 1;
	if (spec.cells.length !== cols * rows) {
		throw new Error(
			`${spec.name}: ${spec.cells.length} cells for ${cols}x${rows}`,
		);
	}
	const cellWidth = spec.width / cols;
	const cellHeight = spec.height / rows;
	const iconSize =
		Math.min(cellWidth, cellHeight) * (spec.iconRatio ?? DEFAULT_ICON_RATIO);
	const a11y = spec.label
		? `role="img" aria-label="${spec.label}"`
		: 'aria-hidden="true"';
	const lines = [
		`<svg xmlns="http://www.w3.org/2000/svg" width="${spec.width}" height="${spec.height}" viewBox="0 0 ${spec.width} ${spec.height}" ${a11y}>`,
	];
	if (spec.background) {
		lines.push(
			`<rect width="${spec.width}" height="${spec.height}" fill="${colors[spec.background]}"/>`,
		);
	}
	spec.cells.forEach((cell, index) => {
		const x = (index % cols) * cellWidth + (cellWidth - iconSize) / 2;
		const y =
			Math.floor(index / cols) * cellHeight + (cellHeight - iconSize) / 2;
		lines.push(
			`<svg x="${num(x)}" y="${num(y)}" width="${num(iconSize)}" height="${num(iconSize)}" viewBox="0 -960 960 960"><path d="${glyphPath(cell.icon)}" fill="${colors[cell.color]}"/></svg>`,
		);
	});
	lines.push("</svg>");
	return `${lines.join("\n")}\n`;
}

/** Every placeholder as `{ file, svg }`, in table order. */
export function renderPlaceholders(): { file: string; svg: string }[] {
	const colors = loadBrandColors();
	const names = new Set<string>();
	return PLACEHOLDERS.map((spec) => {
		if (names.has(spec.name)) throw new Error(`duplicate name ${spec.name}`);
		names.add(spec.name);
		return { file: `${spec.name}.svg`, svg: renderSpec(spec, colors) };
	});
}

function main(): void {
	const dir = path.join(websiteRoot, PLACEHOLDER_DIR);
	mkdirSync(dir, { recursive: true });
	const outputs = renderPlaceholders();
	const wanted = new Set(outputs.map(({ file }) => file));
	for (const entry of readdirSync(dir)) {
		if (!wanted.has(entry)) unlinkSync(path.join(dir, entry));
	}
	for (const { file, svg } of outputs) {
		writeFileSync(path.join(dir, file), svg);
	}
	console.log(`wrote ${outputs.length} files to ${PLACEHOLDER_DIR}`);
}

if (
	process.argv[1] &&
	import.meta.url === pathToFileURL(process.argv[1]).href
) {
	main();
}
