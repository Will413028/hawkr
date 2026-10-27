import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";
import { readTrackGeometry } from "../components/shared/MasonryGrid";
import { MARKET_PRODUCT_GRID_BREAKPOINTS } from "../features/market/components/marketProductGridBreakpoints";

/**
 * #423 guards, at the level a DOM-less unit test can reach.
 *
 * The end-to-end invariant ("the grid does not re-deal itself on hydration")
 * lives in tests/e2e/masonry-column-parity.spec.ts, because only a browser
 * resolves `auto-fill`. What is checkable here is the property that makes that
 * invariant hold by construction: there is exactly ONE declaration of the
 * column count, and both consumers read that same declaration. A regression
 * here is a regression there, and this runs in a second.
 */

const root = path.join(import.meta.dirname, "..");
const read = (relative: string) =>
	readFileSync(path.join(root, relative), "utf8");

const grid = read("components/shared/MasonryGrid.tsx");
const marketCss = read("styles/features/market.css");
const storefront = read("features/sellers/storefront/SellerProductsClient.tsx");

function fakeComputedStyle(
	gridTemplateColumns: string,
	columnGap = "18px",
): CSSStyleDeclaration {
	return { gridTemplateColumns, columnGap } as CSSStyleDeclaration;
}

test("readTrackGeometry reports what the engine resolved, not a recomputation", () => {
	const geometry = readTrackGeometry({} as HTMLElement, () =>
		fakeComputedStyle("275.5px 275.5px 275.5px 275.5px"),
	);
	assert.ok(geometry);
	assert.equal(geometry.columns, 4);
	// Sub-pixel track widths must survive: rounding them here is exactly how a
	// measured layout drifts away from the grid the fallback painted.
	assert.equal(geometry.colWidth, 275.5);
	assert.equal(geometry.gap, 18);
	assert.deepEqual(geometry.tracks, [275.5, 275.5, 275.5, 275.5]);
});

test("readTrackGeometry declines to guess when there is no grid to read", () => {
	assert.equal(readTrackGeometry(null), null);
	assert.equal(
		readTrackGeometry({} as HTMLElement, () => fakeComputedStyle("none")),
		null,
	);
	assert.equal(
		readTrackGeometry({} as HTMLElement, () => fakeComputedStyle("")),
		null,
	);
});

test("the fallback grid and the probe carry the SAME declaration", () => {
	// Two occurrences and no more: the container's fallback style and the
	// probe. If a refactor gives either of them its own track list, the two
	// stop being the same decision and #423 is back.
	const occurrences = grid.match(/var\(--masonry-fallback-track\)/g) ?? [];
	assert.equal(
		occurrences.length,
		2,
		"expected exactly the fallback container and the probe to reference --masonry-fallback-track",
	);
});

test("no JS-side column formula survives in MasonryGrid", () => {
	// The cap that caused #423 was `Math.ceil((containerWidth + gap) / ...)`.
	// Column count must come from readTrackGeometry alone.
	assert.equal(
		/Math\.(ceil|floor|round)\s*\(/.test(grid),
		false,
		"MasonryGrid must not compute a column count; it reads the resolved tracks",
	);
	assert.equal(
		grid.includes("maxColumnWidth"),
		true,
		"the historical name should survive in prose so the reason is not lost",
	);
	assert.equal(
		/maxColumnWidth[?:]/.test(grid),
		false,
		"maxColumnWidth must no longer be a prop",
	);
});

test("every track variable the component emits is consumed by the CSS", () => {
	const emitted = [
		...new Set(
			[...grid.matchAll(/"--masonry-(track|gap)-([a-z]+)"/g)].map(
				(match) => `--masonry-${match[1]}-${match[2]}`,
			),
		),
	];
	assert.equal(emitted.length, 10, "five breakpoints x (track, gap)");
	for (const variable of emitted) {
		assert.ok(
			marketCss.includes(`var(${variable})`),
			`${variable} is emitted by MasonryGrid but never read by market.css`,
		);
	}
});

test("both grid tables declare tracks, never a column count", () => {
	for (const entry of MARKET_PRODUCT_GRID_BREAKPOINTS) {
		assert.match(entry.track, /^repeat\(/);
		assert.equal(
			"columns" in entry,
			false,
			"a numeric column count would be a second source of truth",
		);
	}
	assert.equal(
		/columns:\s*\d/.test(storefront),
		false,
		"storefront table must declare tracks, not column counts",
	);
	assert.match(storefront, /repeat\(auto-fill, minmax\(\d+px, 1fr\)\)/);
});

test("storefront cards stay two columns on mobile without changing desktop", () => {
	const breakpointTable = storefront.match(
		/const SELLER_PRODUCT_GRID_BREAKPOINTS:[^=]+=\s*\[([\s\S]*?)\];/,
	)?.[1];
	assert.ok(breakpointTable, "missing seller storefront breakpoint table");
	const breakpointRule = (maxWidth: number) => {
		const match = breakpointTable.match(
			new RegExp(`\\{\\s*maxWidth:\\s*${maxWidth},[^{}]*\\}`),
		);
		assert.ok(match, `missing ${maxWidth}px storefront breakpoint`);
		return match[0];
	};
	assert.match(
		breakpointTable,
		/maxWidth:\s*360[\s\S]*maxWidth:\s*680[\s\S]*maxWidth:\s*840[\s\S]*maxWidth:\s*1120[\s\S]*maxWidth:\s*Number\.POSITIVE_INFINITY/,
		"storefront breakpoints must remain ordered from narrow to wide",
	);

	for (const maxWidth of [360, 680, 840]) {
		assert.match(
			breakpointRule(maxWidth),
			/track:\s*"repeat\(2, minmax\(0, 1fr\)\)"/,
			`${maxWidth}px storefront layout must keep the requested mobile two-column grid`,
		);
	}
	assert.match(
		breakpointRule(1120),
		/track:\s*"repeat\(3, minmax\(0, 1fr\)\)"/,
		"desktop storefront layout must preserve its existing three columns",
	);
	const wideRule = breakpointTable.match(
		/\{\s*maxWidth:\s*Number\.POSITIVE_INFINITY,[^{}]*\}/,
	)?.[0];
	assert.ok(wideRule, "missing wide storefront breakpoint");
	assert.match(
		wideRule,
		/track:\s*"repeat\(auto-fill, minmax\(280px, 1fr\)\)"/,
	);
	assert.match(wideRule, /gap:\s*18/);
});
