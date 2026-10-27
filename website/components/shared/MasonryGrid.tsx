"use client";

import * as React from "react";

export interface MasonryBreakpoint {
	/** Apply when the viewport is <= maxWidth (inclusive), matching the CSS. */
	maxWidth: number;
	/**
	 * A CSS `<track-list>` for `grid-template-columns` — the SINGLE definition
	 * of "how many columns this grid has at this width".
	 *
	 * It is used verbatim in two places that must never disagree: the SSR
	 * fallback grid the visitor sees first, and a zero-height probe the
	 * measured pass reads its geometry back from. Because both are the same
	 * declaration resolved by the same engine against the same box, the column
	 * count they produce is identical BY CONSTRUCTION — there is no second
	 * formula that could drift out of step with this one.
	 *
	 * Use `repeat(N, minmax(0, 1fr))` to pin a column count, or
	 * `repeat(auto-fill, minmax(<min>, 1fr))` to let the container width decide
	 * (which is how a wide viewport is kept from blowing the cards up — the
	 * job the old `maxColumnWidth` prop used to do in JS, where CSS could not
	 * see it).
	 */
	track: string;
	gap: number;
}

interface MasonryGridProps<T> {
	items: T[];
	getKey: (item: T, index: number) => string | number;
	renderItem: (item: T, index: number) => React.ReactNode;
	className?: string;
	/** Responsive layout table. First matching breakpoint wins. */
	breakpoints?: MasonryBreakpoint[];
}

interface Position {
	x: number;
	y: number;
	w: number;
}

export interface MasonryPlanInput<K> {
	/**
	 * Measurable items, in render order. Items with no element yet are left
	 * out, so `index` carries each entry's position in the FULL render order —
	 * that is the position the CSS fallback laid it out at, and dropping to
	 * the compacted array index would silently shift the adopted columns.
	 */
	entries: Array<{ key: K; height: number; index: number }>;
	columns: number;
	colWidth: number;
	gap: number;
	/**
	 * Column each key was placed in last time, or null when the column count
	 * changed and the whole grid is legitimately re-dealt.
	 */
	previous: Map<K, number> | null;
	/**
	 * True for the pass that replaces the SSR fallback. That fallback is a
	 * plain CSS grid, so it has already painted item `index` in column
	 * `index % columns`; adopting it is what keeps the handoff from moving
	 * cards sideways.
	 */
	adoptFallbackOrder: boolean;
}

/**
 * Decides where every card goes. Pure on purpose: the invariant that matters
 * ("a card never changes column while it is on screen") is a property of this
 * function alone, so it is testable without a DOM.
 *
 * A key keeps its previous column. A key without one is appended to the
 * currently shortest column — except on the fallback-adopting pass, where the
 * arrangement already on screen wins over a better-balanced one.
 */
export function planMasonryLayout<K>({
	entries,
	columns,
	colWidth,
	gap,
	previous,
	adoptFallbackOrder,
}: MasonryPlanInput<K>): {
	positions: Map<K, Position>;
	assignment: Map<K, number>;
	height: number;
} {
	const colHeights = new Array<number>(columns).fill(0);
	const positions = new Map<K, Position>();
	const assignment = new Map<K, number>();

	for (const entry of entries) {
		let column = previous?.get(entry.key);
		if (column === undefined || column >= columns) {
			if (adoptFallbackOrder) {
				column = entry.index % columns;
			} else {
				column = 0;
				for (let i = 1; i < columns; i++) {
					if (colHeights[i] < colHeights[column]) column = i;
				}
			}
		}
		assignment.set(entry.key, column);
		positions.set(entry.key, {
			x: column * (colWidth + gap),
			y: colHeights[column],
			w: colWidth,
		});
		colHeights[column] += entry.height + gap;
	}

	return {
		positions,
		assignment,
		height: Math.max(0, Math.max(...colHeights, 0) - gap),
	};
}

const DEFAULT_BREAKPOINTS: MasonryBreakpoint[] = [
	{ maxWidth: 840, track: "repeat(3, minmax(0, 1fr))", gap: 8 },
	{ maxWidth: 1120, track: "repeat(2, minmax(0, 1fr))", gap: 18 },
	{
		maxWidth: Number.POSITIVE_INFINITY,
		track: "repeat(3, minmax(0, 1fr))",
		gap: 18,
	},
];

function pickBreakpoint(
	width: number,
	table: MasonryBreakpoint[],
): MasonryBreakpoint {
	return table.find((b) => width <= b.maxWidth) ?? table[table.length - 1];
}

export interface MasonryTrackGeometry {
	columns: number;
	colWidth: number;
	gap: number;
	/** Every resolved track, so callers can assert they came out equal. */
	tracks: number[];
}

/**
 * Reads the column geometry the CSS engine ACTUALLY resolved, instead of
 * recomputing it.
 *
 * This is the whole point of the component's design. Any JS-side column
 * formula is a second opinion, and a second opinion is exactly what produced
 * #423: the SSR fallback painted the breakpoint's column count while the
 * measured pass added columns from a container-width cap that CSS could not
 * see, so the first hydration re-dealt every card across a different number
 * of columns (CLS 0.19 at 1920). Here the used track sizes come back from
 * `getComputedStyle`, i.e. from the same resolution that painted the
 * fallback — including sub-pixel track widths, which is why there is no
 * integer-rounding hazard left to reason about.
 */
export function readTrackGeometry(
	probe: HTMLElement | null,
	getComputed: (el: HTMLElement) => CSSStyleDeclaration = (el) =>
		window.getComputedStyle(el),
): MasonryTrackGeometry | null {
	if (!probe) return null;
	const style = getComputed(probe);
	const template = style.gridTemplateColumns;
	if (!template || template === "none") return null;
	const tracks = template
		.trim()
		.split(/\s+/)
		.map((value) => Number.parseFloat(value))
		.filter((value) => Number.isFinite(value));
	if (tracks.length === 0) return null;
	const gap = Number.parseFloat(style.columnGap);
	return {
		columns: tracks.length,
		colWidth: tracks[0],
		gap: Number.isFinite(gap) ? gap : 0,
		tracks,
	};
}

/**
 * JS-driven masonry / 小紅書-style waterfall.
 *
 * Column count and column width are NOT decided here — they are read back
 * from the CSS grid declaration in `breakpoints[].track` (see
 * `readTrackGeometry`). JS only decides which column each card lives in and
 * how the cards stack vertically, which is the part CSS cannot express.
 *
 * A card's column is decided ONCE — from the SSR fallback's own order on the
 * first measured pass, greedily into the shortest column for anything
 * appended later — and then held for as long as the column count does not
 * change. Only the vertical stacking is re-derived from the measured heights.
 *
 * That split is the whole point: an item's height keeps moving after paint
 * (media, fonts, a title that re-wraps), and a packing that is re-derived
 * from heights on every pass turns any of those into a HORIZONTAL jump — one
 * card growing a pixel flips which column is shortest and re-deals every card
 * after it. Appending new items likewise extends the bottom without
 * disturbing the cards above, which is what CSS `column-count` cannot do
 * under infinite scroll.
 */
export default function MasonryGrid<T>({
	items,
	getKey,
	renderItem,
	className,
	breakpoints = DEFAULT_BREAKPOINTS,
}: MasonryGridProps<T>) {
	const containerRef = React.useRef<HTMLDivElement>(null);
	const probeRef = React.useRef<HTMLDivElement>(null);
	const itemElsRef = React.useRef(new Map<string | number, HTMLDivElement>());
	const fallbackVars = React.useMemo(() => {
		const at = (maxWidth: number) =>
			breakpoints.find((entry) => entry.maxWidth === maxWidth) ??
			pickBreakpoint(maxWidth, breakpoints);
		const wide = breakpoints[breakpoints.length - 1];
		const xs = at(360);
		const sm = at(680);
		const md = at(840);
		const lg = at(1120);
		return {
			"--masonry-track-xs": xs.track,
			"--masonry-gap-xs": `${xs.gap}px`,
			"--masonry-track-sm": sm.track,
			"--masonry-gap-sm": `${sm.gap}px`,
			"--masonry-track-md": md.track,
			"--masonry-gap-md": `${md.gap}px`,
			"--masonry-track-lg": lg.track,
			"--masonry-gap-lg": `${lg.gap}px`,
			"--masonry-track-wide": wide.track,
			"--masonry-gap-wide": `${wide.gap}px`,
		} as React.CSSProperties;
	}, [breakpoints]);

	// Keep prop callbacks stable inside recompute to avoid re-running
	// the layout loop just because the parent re-rendered.
	const getKeyRef = React.useRef(getKey);
	getKeyRef.current = getKey;

	const [layout, setLayout] = React.useState<{
		height: number;
		colWidth: number;
		positions: Map<string | number, Position>;
	}>(() => ({ height: 0, colWidth: 0, positions: new Map() }));

	/**
	 * Which column each already-placed item lives in, plus the column count
	 * that assignment was made under. Re-deriving the packing from scratch on
	 * every pass is what made this grid shuffle cards SIDEWAYS after paint:
	 * the greedy walk below is a function of the measured heights, so a card
	 * whose height moves by a pixel can flip which column is shortest and
	 * cascade into a different assignment for everything after it.
	 */
	const assignmentRef = React.useRef<{
		columns: number;
		byKey: Map<string | number, number>;
	}>({ columns: 0, byKey: new Map() });
	/**
	 * False until a measured pass has committed, i.e. while what the visitor
	 * is looking at is still the SSR fallback grid below.
	 */
	const measuredOnceRef = React.useRef(false);

	const recompute = React.useCallback(() => {
		const container = containerRef.current;
		if (!container) return;
		if (container.clientWidth <= 0) return;
		// The one and only source of "how many columns": whatever the CSS
		// grid declaration resolved to on the probe, which is the very same
		// declaration the SSR fallback painted with.
		const geometry = readTrackGeometry(probeRef.current);
		if (!geometry) return;
		const { columns, colWidth, gap } = geometry;

		const localGetKey = getKeyRef.current;

		const entries: MasonryPlanInput<string | number>["entries"] = [];
		for (let index = 0; index < items.length; index++) {
			const key = localGetKey(items[index], index);
			const el = itemElsRef.current.get(key);
			if (!el) continue;
			entries.push({ key, height: el.offsetHeight, index });
		}

		const { positions, assignment, height } = planMasonryLayout({
			entries,
			columns,
			colWidth,
			gap,
			// A column-count change is a genuine re-layout (breakpoint / resize),
			// so the sticky assignment is dropped; anything short of that keeps
			// every card in the column it was already painted in.
			previous:
				assignmentRef.current.columns === columns
					? assignmentRef.current.byKey
					: null,
			adoptFallbackOrder: !measuredOnceRef.current,
		});

		assignmentRef.current = { columns, byKey: assignment };
		measuredOnceRef.current = true;
		setLayout({ height, colWidth, positions });
	}, [items]);

	React.useLayoutEffect(() => {
		recompute();
	}, [recompute]);

	React.useEffect(() => {
		const container = containerRef.current;
		if (!container) return;
		const ro = new ResizeObserver(() => recompute());
		ro.observe(container);
		if (probeRef.current) ro.observe(probeRef.current);
		for (const el of itemElsRef.current.values()) ro.observe(el);
		// A media-query change can swap the track list WITHOUT changing the
		// container's width (e.g. three fixed columns -> auto-fill at the same
		// width), and a ResizeObserver never fires for that. The window
		// listener is what makes those transitions land.
		window.addEventListener("resize", recompute);
		return () => {
			ro.disconnect();
			window.removeEventListener("resize", recompute);
		};
	}, [recompute]);

	// Until the first measure pass lands, lay the items out as a PLAIN CSS
	// grid in normal flow. This is what the SSR HTML paints while JS is
	// still loading — previously every card was `visibility:hidden` inside
	// a 0-height container, so /market/products painted a blank grid and
	// then jumped to full height after hydration (massive CLS).
	//
	// This fallback DOES get painted on its own whenever hydration is slower
	// than first paint (every loaded CI runner, every slow phone) — the
	// useLayoutEffect recompute is only pre-paint relative to the hydration
	// commit, not to the SSR paint that preceded it. So the arrangement it
	// paints is load-bearing, not a formality: `adoptFallbackOrder` above
	// makes the first measured pass agree with it instead of re-dealing the
	// cards across columns.
	const measured = layout.positions.size > 0;

	return (
		<div
			ref={containerRef}
			className={className}
			data-masonry-grid
			style={
				measured
					? {
							...fallbackVars,
							// `relative` in BOTH modes, so the probe below always
							// resolves against this element's padding box.
							position: "relative",
							height: layout.height || undefined,
						}
					: {
							...fallbackVars,
							position: "relative",
							display: "grid",
							gridTemplateColumns: "var(--masonry-fallback-track)",
							gap: "var(--masonry-fallback-gap)",
							alignItems: "start",
						}
			}
		>
			{/*
			 * Zero-height, out-of-flow carrier of the SAME grid declaration the
			 * fallback paints with. Once the measured pass swaps the container
			 * to absolute positioning, the container itself stops being a grid,
			 * so without this there would be nothing left to read the resolved
			 * column count back from — and reading it back is precisely what
			 * removes the need for a second, JS-side formula. Being absolutely
			 * positioned, it occupies no grid cell in fallback mode.
			 */}
			<div
				ref={probeRef}
				data-masonry-probe
				aria-hidden="true"
				style={{
					position: "absolute",
					left: 0,
					right: 0,
					top: 0,
					height: 0,
					display: "grid",
					gridTemplateColumns: "var(--masonry-fallback-track)",
					gap: "var(--masonry-fallback-gap)",
					pointerEvents: "none",
				}}
			/>
			{items.map((item, index) => {
				const key = getKey(item, index);
				const pos = layout.positions.get(key);
				return (
					<div
						key={key}
						ref={(el) => {
							if (el) itemElsRef.current.set(key, el);
							else itemElsRef.current.delete(key);
						}}
						style={
							measured
								? {
										position: "absolute",
										left: pos ? `${pos.x}px` : 0,
										top: pos ? `${pos.y}px` : 0,
										width: pos ? `${pos.w}px` : `${layout.colWidth || 0}px`,
										// Freshly appended items render hidden until the
										// next layout pass assigns them a position, so
										// they don't flash a stack at (0,0).
										visibility: pos ? "visible" : "hidden",
									}
								: undefined
						}
					>
						{renderItem(item, index)}
					</div>
				);
			})}
		</div>
	);
}
