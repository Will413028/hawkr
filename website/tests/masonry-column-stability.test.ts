import assert from "node:assert/strict";
import { test } from "node:test";
import { planMasonryLayout } from "../components/shared/MasonryGrid";

const COLUMNS = 3;
const COL_WIDTH = 299;
const GAP = 18;

function plan(
	heights: number[],
	options: {
		previous?: Map<string, number> | null;
		adoptFallbackOrder?: boolean;
		keys?: string[];
	} = {},
) {
	const keys = options.keys ?? heights.map((_, index) => `p${index}`);
	return planMasonryLayout({
		entries: heights.map((height, index) => ({
			key: keys[index],
			height,
			index,
		})),
		columns: COLUMNS,
		colWidth: COL_WIDTH,
		gap: GAP,
		previous: options.previous ?? null,
		adoptFallbackOrder: options.adoptFallbackOrder ?? false,
	});
}

function columns(plan: { assignment: Map<string, number> }): number[] {
	return [...plan.assignment.values()];
}

// hawkr-website#421: the SSR fallback is a plain CSS grid, so it paints
// item `index` in column `index % columns`. If the first measured pass packs
// by height instead, cards translate a whole column sideways on the handoff —
// measured at 0.0771 CLS on /market/products against a 0.05 budget.
test("the fallback-adopting pass keeps the CSS grid's column for every card", () => {
	const heights = [436, 441, 436, 441, 441, 436, 436, 436, 436, 436];
	const greedy = plan(heights);
	const adopted = plan(heights, { adoptFallbackOrder: true });

	assert.deepEqual(
		columns(greedy),
		[0, 1, 2, 0, 2, 1, 0, 1, 2, 0],
		"the packing this replaces really does disagree with row-major, otherwise this test proves nothing",
	);
	assert.deepEqual(columns(adopted), [0, 1, 2, 0, 1, 2, 0, 1, 2, 0]);
});

test("a card never changes column when its measured height moves", () => {
	const heights = [436, 441, 436, 441, 441, 436];
	const first = plan(heights, { adoptFallbackOrder: true });
	// A title re-wraps and one card grows; every later card's shortest-column
	// answer changes with it, which is exactly what used to re-deal the grid.
	const grown = [436, 441, 640, 441, 441, 436];
	const second = plan(grown, { previous: first.assignment });

	assert.deepEqual(columns(second), columns(first));
	for (const [key, position] of first.positions) {
		assert.equal(
			second.positions.get(key)?.x,
			position.x,
			`${key} moved horizontally`,
		);
	}
});

test("appended items pack into the shortest column without moving the rest", () => {
	const initial = plan([436, 441, 436], { adoptFallbackOrder: true });
	const next = plan([436, 441, 436, 100, 100], {
		previous: initial.assignment,
	});

	// p0/p2 are the short columns (436), so the appended pair lands there.
	assert.deepEqual(columns(next), [0, 1, 2, 0, 2]);
	for (const [key, position] of initial.positions) {
		assert.deepEqual(next.positions.get(key), position, `${key} moved`);
	}
});

test("a column-count change re-deals the grid instead of stranding columns", () => {
	const wide = plan([436, 441, 436, 441], { adoptFallbackOrder: true });
	const narrow = planMasonryLayout({
		entries: [436, 441, 436, 441].map((height, index) => ({
			key: `p${index}`,
			height,
			index,
		})),
		columns: 2,
		colWidth: 450,
		gap: GAP,
		// Callers pass null on a column change; a stale map is also survivable.
		previous: wide.assignment,
		adoptFallbackOrder: false,
	});

	for (const column of columns(narrow)) {
		assert.ok(column < 2, `column ${column} is outside a 2-column grid`);
	}
});

test("an item that has not been measured yet does not shift the others' columns", () => {
	// The second card has no element on this pass, so it is skipped — the third
	// card must still adopt the column the CSS fallback painted it in (2), not
	// the one its position in the compacted array would suggest (1).
	const { assignment } = planMasonryLayout({
		entries: [
			{ key: "p0", height: 436, index: 0 },
			{ key: "p2", height: 436, index: 2 },
			{ key: "p3", height: 441, index: 3 },
		],
		columns: COLUMNS,
		colWidth: COL_WIDTH,
		gap: GAP,
		previous: null,
		adoptFallbackOrder: true,
	});
	assert.deepEqual([...assignment.values()], [0, 2, 0]);
});

test("height is the tallest column without a trailing gap", () => {
	const { height } = plan([100, 200, 300], { adoptFallbackOrder: true });
	assert.equal(height, 300);
});

test("an empty grid has no height", () => {
	assert.equal(plan([]).height, 0);
});
