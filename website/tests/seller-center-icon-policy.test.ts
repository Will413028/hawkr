import assert from "node:assert/strict";
import { readdirSync, readFileSync } from "node:fs";
import { extname, join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));

function filesUnder(directory: string): string[] {
	const absolute = join(ROOT, directory);
	return readdirSync(absolute, { withFileTypes: true }).flatMap((entry) => {
		const path = join(absolute, entry.name);
		if (entry.isDirectory()) return filesUnder(relative(ROOT, path));
		return extname(entry.name) === ".tsx" ? [path] : [];
	});
}

function source(path: string): string {
	return readFileSync(path, "utf8");
}

test("seller-center feature code does not own SVG icon geometry", () => {
	const files = filesUnder("features/seller-center");
	const allowedGraphics = new Map([
		// This is a plotted observed-price series, not an interface icon.
		["features/seller-center/insights/ObservedPriceSparkline.tsx", 1],
		["features/seller-center/inventory/InventoryPage.tsx", 1],
	]);

	for (const file of files) {
		const relativePath = relative(ROOT, file);
		const svgCount = source(file).match(/<svg\b/g)?.length ?? 0;
		assert.equal(
			svgCount,
			allowedGraphics.get(relativePath) ?? 0,
			`${relativePath} must use AppIcon; only declared data graphics are exempt`,
		);
		assert.doesNotMatch(source(file), /components\/shared\/UiIcon/);
		assert.doesNotMatch(source(file), /\/icons\/[^"']+\.svg/);
	}
});

test("seller workspace dependencies only keep declared non-icon SVG graphics", () => {
	const allowedGraphics = new Map([
		["features/seller/components/SellerBusinessCard.tsx", 1],
		["features/seller/components/SellerProfileBranding.tsx", 2],
	]);
	const files = [
		...filesUnder("features/seller/components"),
		...filesUnder("features/seller/onboarding"),
		...filesUnder("features/messages/components"),
		...filesUnder("features/account/screens/messages"),
		...filesUnder("features/account/screens/notifications"),
	];

	for (const file of files) {
		const relativePath = relative(ROOT, file);
		const svgCount = source(file).match(/<svg\b/g)?.length ?? 0;
		assert.equal(
			svgCount,
			allowedGraphics.get(relativePath) ?? 0,
			`${relativePath} introduced an undeclared inline SVG`,
		);
		assert.doesNotMatch(source(file), /components\/shared\/UiIcon/);
		assert.doesNotMatch(source(file), />\s*[←→↑↓✓★‹›]\s*</);
	}
});

test("shared static icons are drawn by @hawkr/icons and own no geometry", () => {
	const icons = source(join(ROOT, "components/shared/AppIcon.tsx"));

	assert.match(icons, /from "@hawkr\/icons\/material-symbol"/);
	assert.doesNotMatch(icons, /<(?:path|line|polyline)|\bd="M/);
});
