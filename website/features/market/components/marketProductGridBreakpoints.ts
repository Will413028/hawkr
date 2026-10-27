import type { MasonryBreakpoint } from "@/components/shared/MasonryGrid";

/**
 * The market feed's column rules — and the ONLY place they are written down.
 * Both the SSR fallback grid and the measured (absolutely positioned) pass
 * resolve their column count from these exact declarations, so the two can
 * never answer "how many columns?" differently (#423).
 *
 * Phones and small tablets pin an explicit count, because "two-up waterfall"
 * there is a deliberate design property, not a consequence of how wide the
 * container happens to be.
 *
 * From 841px up the count is left to `auto-fill`, which is what keeps cards
 * from blowing up on a wide screen. That upper bound used to live in JS as
 * `maxColumnWidth={360}` — invisible to CSS, which is exactly the mismatch
 * this file's shape removes. `234px` / `246px` are the smallest a card may
 * get before another column is worth adding; they are card-design numbers,
 * NOT derived from the page shell, so changing the shell degrades gracefully
 * (slightly wider or narrower cards) instead of resurrecting the bug.
 */
export const MARKET_PRODUCT_GRID_BREAKPOINTS: MasonryBreakpoint[] = [
	{ maxWidth: 360, track: "repeat(1, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 680, track: "repeat(2, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 840, track: "repeat(3, minmax(0, 1fr))", gap: 12 },
	{ maxWidth: 1120, track: "repeat(auto-fill, minmax(234px, 1fr))", gap: 18 },
	{
		maxWidth: Number.POSITIVE_INFINITY,
		track: "repeat(auto-fill, minmax(246px, 1fr))",
		gap: 18,
	},
];
