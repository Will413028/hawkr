import { expect, type Page, type Route, test } from "@playwright/test";
import { fulfillConnectError, GET_SESSION_URL } from "./helpers/mock-gateway";

/**
 * #423 — the SSR fallback grid and the measured (absolutely positioned) pass
 * must never disagree about how many columns the grid has.
 *
 * WHY THIS SUITE EXISTS AND WHY IT IS SHAPED LIKE THIS
 *
 * The bug it guards was not a wrong number, it was a SECOND number: the
 * fallback painted the breakpoint's column count while JS added columns from
 * a container-width cap CSS could not see. Every card then moved sideways on
 * hydration — CLS 0.19 at 1920, 0.28 at 1536 — and none of it was visible to
 * CI, because the only two viewports CI ran (1280 desktop, Pixel 5) happen to
 * be widths where the two answers coincided. A CLS budget at those two widths
 * is therefore not a gate for this class of bug at all; the gate has to be the
 * invariant itself, asserted across the widths where the two rules can differ.
 *
 * Two independent assertions, because they fail for different reasons:
 *
 *   1. `fallback === settled` — loads the route twice, once with the
 *      hydration bundle blocked so the page stays on the SSR/CSS fallback,
 *      once normally. This is the user-visible invariant, end to end.
 *   2. `settled === the CSS's own resolved tracks` — reads the resolved track
 *      list off the probe element and compares it against where the cards
 *      actually ended up. This one goes red the moment anyone reintroduces a
 *      JS-side column formula, even if that formula happens to agree with CSS
 *      at the widths in assertion 1.
 */

/** Widths chosen from the measured mismatch bands in #423, plus controls. */
const MARKET_WIDTHS = [375, 768, 900, 1120, 1280, 1440, 1920, 2560];
const STOREFRONT_WIDTHS = [375, 900, 1280, 1920];

const GRID = "[data-masonry-grid]";
const PROBE = "[data-masonry-probe]";
/**
 * A real card, never the probe. The probe is a zero-height, out-of-flow child
 * of the grid, so `${GRID} > *` resolves to it first and then waits forever for
 * something invisible by design to become visible.
 */
const CARD = `${GRID} > *:not(${PROBE})`;
const HYDRATION_BUNDLE = "**/_next/static/chunks/**/*.js";
const abortRoute = (route: Route) => route.abort();
/**
 * Proof that the storefront subtree itself rendered, not just that the URL is
 * right. `SellerProductsClient` emits it unconditionally, above its own
 * grid/empty-state fork, so its absence means the page never got as far as
 * having products to show.
 */
const STOREFRONT_RENDERED = '[data-hk-page-content-ready="seller-storefront"]';

interface GridGeometry {
	containerWidth: number;
	/** Column count derived from where the cards actually are. */
	columns: number;
	/** Distinct left offsets, rounded — one per column. */
	offsets: number[];
	cardWidth: number;
	cardCount: number;
	measured: boolean;
	/** Track list the CSS resolved, read off the probe. */
	tracks: number[];
	gap: number;
}

async function readGrid(page: Page): Promise<GridGeometry | null> {
	return page.evaluate(
		({ gridSelector, probeSelector }) => {
			const grid = document.querySelector<HTMLElement>(gridSelector);
			if (!grid) return null;
			const probe = grid.querySelector<HTMLElement>(probeSelector);
			const cards = Array.from(grid.children).filter(
				(child) => child !== probe,
			) as HTMLElement[];
			if (cards.length === 0) return null;
			const rects = cards.map((card) => card.getBoundingClientRect());
			const offsets = [...new Set(rects.map((r) => Math.round(r.x)))].sort(
				(a, b) => a - b,
			);
			const probeStyle = probe ? getComputedStyle(probe) : null;
			const tracks = (probeStyle?.gridTemplateColumns ?? "")
				.trim()
				.split(/\s+/)
				.map((value) => Number.parseFloat(value))
				.filter((value) => Number.isFinite(value));
			const gap = Number.parseFloat(probeStyle?.columnGap ?? "");
			return {
				containerWidth: grid.clientWidth,
				columns: offsets.length,
				offsets,
				cardWidth: Math.round(rects[0].width),
				cardCount: cards.length,
				measured: getComputedStyle(grid).display !== "grid",
				tracks,
				gap: Number.isFinite(gap) ? gap : 0,
			};
		},
		{ gridSelector: GRID, probeSelector: PROBE },
	);
}

/**
 * Waits until the grid stops gaining cards.
 *
 * React streams the product section in, so `waitForSelector` can resolve on
 * the first card while later ones are still arriving. Reading then compares a
 * half-delivered fallback against a complete settled layout and reports it as a
 * column change — blaming this suite's subject for a race in its own probe.
 */
async function waitForStableCardCount(page: Page) {
	let previous = -1;
	await expect
		.poll(
			async () => {
				const count = await page.locator(CARD).count();
				const stable = count > 0 && count === previous;
				previous = count;
				return stable;
			},
			{ timeout: 20_000, intervals: [250] },
		)
		.toBe(true);
}

/**
 * Tears the current page down before the next navigation is set up.
 *
 * The chunk-blocking route below is armed while the PREVIOUS page is still
 * running its own JS. The moment that page asks for a lazily-loaded chunk —
 * a viewport change alone is enough to pull one in — the blocked request
 * raises a ChunkLoadError, and the App Router's recovery for that is a hard
 * reload of the URL it is still on. That reload CANCELS the goto already in
 * flight. Blanking first leaves nothing running that could do it.
 */
const blankSlate = (page: Page) =>
	page.goto("about:blank", { waitUntil: "domcontentloaded", timeout: 30_000 });

/**
 * Navigates, and proves we arrived before a single number is read.
 *
 * Without this the cancellation described above is SILENT: the suite reads
 * whatever page happens to be on screen as the answer for the one it asked
 * for. That is what made CI run 30864943472 red — the fallback read never left
 * `/market/products`, so the market feed's 9 cards were compared against the
 * storefront's 1 product and reported as "a card disappearing on hydration".
 * The `measured === false` guard cannot catch it either: the page it got
 * bounced back to has its own hydration blocked too, so it is equally
 * un-hydrated and looks exactly like a legitimate fallback read.
 */
async function landOn(
	page: Page,
	route: string,
	width: number,
	rendered?: string,
) {
	await page.goto(route, { waitUntil: "domcontentloaded", timeout: 30_000 });
	expect(
		new URL(page.url()).pathname,
		`${route} @${width}: the navigation did not land on the route being graded (ended on ${page.url()}) — every number read after this would describe a different page`,
	).toBe(route);
	if (!rendered) return;
	// A pathname match is NOT proof the page rendered. `notFound()` thrown from
	// a layout keeps the URL and answers 200 (the shell is already flushed), so
	// the only outward sign is an empty <main> — and the card wait downstream
	// reports that as "the card count never settled", i.e. as a masonry defect.
	// It is not one: it is the seller lookup having failed. Naming it here costs
	// one selector and saves the next person the hour it cost this time.
	//
	// The failure this caught is real and lives in production: a gateway 429
	// (anonymous public reads are capped per IP) made getCachedSeller return
	// null, the storefront layout called notFound(), and ISR then served that
	// 404 for the route's full 1800s window. See lib/api/prerender.ts.
	await expect(
		page.locator(rendered),
		`${route} @${width}: the route answered, but its content subtree is absent (${rendered} never appeared) — this is a not-found/error shell, not the storefront this suite grades. Nothing below this line would be measuring the layout.`,
	).toHaveCount(1, { timeout: 10_000 });
}

async function guestSession(page: Page) {
	await page.route(GET_SESSION_URL, async (route) => {
		await fulfillConnectError(route, "unauthenticated", "guest");
	});
}

/**
 * The SSR/CSS state a visitor sees before hydration.
 *
 * Only `*.js` is blocked, deliberately: Next serves stylesheets from the same
 * `/_next/static/chunks/` prefix, and blocking those too strips the layout the
 * assertion is about — it then "passes" against an unstyled page.
 */
async function readFallbackGeometry(
	page: Page,
	route: string,
	width: number,
	rendered?: string,
): Promise<GridGeometry | null> {
	await page.setViewportSize({ width, height: 1080 });
	await guestSession(page);
	await blankSlate(page);
	await page.route(HYDRATION_BUNDLE, abortRoute);
	try {
		await landOn(page, route, width, rendered);
		await waitForStableCardCount(page);
		const geometry = await readGrid(page);
		expect(
			geometry,
			`${route} @${width}: no fallback grid rendered`,
		).not.toBeNull();
		expect(
			geometry?.measured,
			`${route} @${width}: expected the pre-hydration CSS grid, got the measured layout`,
		).toBe(false);
		return geometry;
	} finally {
		// The same `page` is reused for the settled read below. Leaving this
		// route armed would block hydration there too, and the settled read
		// would then time out waiting for a measured layout that can never
		// happen — a false red that looks exactly like the bug.
		await page.unroute(HYDRATION_BUNDLE, abortRoute);
	}
}

async function readSettledGeometry(
	page: Page,
	route: string,
	width: number,
	rendered?: string,
): Promise<GridGeometry | null> {
	await page.setViewportSize({ width, height: 1080 });
	await guestSession(page);
	// Same blank slate as the fallback read, so the two passes differ ONLY by
	// the blocked bundle — which is the entire premise of comparing them.
	await blankSlate(page);
	await landOn(page, route, width, rendered);
	await waitForStableCardCount(page);
	await expect
		.poll(async () => (await readGrid(page))?.measured, { timeout: 20_000 })
		.toBe(true);
	return readGrid(page);
}

/**
 * Picks a storefront out of the market feed's own rendered links.
 *
 * Deliberately NOT the `/api/v1/public/*` proxy the other suites use: that
 * route is opt-in (`PLAYWRIGHT_E2E_API_PROXY`), so a fixture built on it 404s
 * against any server started without it and the suite fails for a reason that
 * has nothing to do with the layout it grades. The links in the market HTML are
 * already the canonical storefront segments the app itself routes to.
 */
async function discoverStorefront(page: Page): Promise<string> {
	await page.goto("/market/products", {
		waitUntil: "domcontentloaded",
		timeout: 30_000,
	});
	await page.waitForSelector(CARD, { timeout: 20_000 });
	// ⚠️ 只從**商品卡片內**取賣家連結，不要拿頁面上任何 `a[href^="/sellers/"]`。
	//
	// 市集頁除了商品卡，還有「推薦商家」欄，而 `/public/sellers/recommended`
	// **不過濾商品數**：dev 上第一名 `2662192214` 的商品數是 0。原本的寫法掃
	// 整份文件，於是挑中那家空店 —— 店面渲染正常（`ready=true`、頁面寫著
	// 「沒有商品」），但 grid 與 card 都是 0，`waitForStableCardCount` 要求
	// `count > 0` 就永遠不成立，20 秒後逾時。
	//
	// 失敗訊息會指向 `waitForStableCardCount`，看起來像 masonry 有 race，
	// 實際上是這個 fixture 挑了一個**沒有欄可比**的頁面。
	//
	// 卡片內的賣家連結必然對應一個至少有一個商品的店家——那正是本測試要量的對象。
	const href = await page.evaluate((cardSelector) => {
		for (const card of document.querySelectorAll(cardSelector)) {
			const link = Array.from(
				card.querySelectorAll<HTMLAnchorElement>('a[href^="/sellers/"]'),
			).find((anchor) =>
				/^\/sellers\/[^/]+$/.test(anchor.getAttribute("href") ?? ""),
			);
			if (link) return link.getAttribute("href");
		}
		return null;
	}, CARD);
	expect(
		href,
		"no storefront link inside a market product card to build a fixture from",
	).not.toBeNull();
	return href as string;
}

/**
 * Viewport is set per test, so running the same width matrix again under the
 * mobile project would re-measure identical numbers at twice the cost.
 */
function onlyOneProject() {
	test.skip(
		test.info().project.name !== "chromium",
		"width matrix is project-independent; one project is enough",
	);
}

test.describe("masonry column parity (#423)", { tag: ["@ui", "@live"] }, () => {
	for (const width of MARKET_WIDTHS) {
		test(`/market/products @${width}: fallback and settled agree`, async ({
			page,
		}) => {
			onlyOneProject();
			const fallback = await readFallbackGeometry(
				page,
				"/market/products",
				width,
			);
			const settled = await readSettledGeometry(
				page,
				"/market/products",
				width,
			);
			assertParity("/market/products", width, fallback, settled);
		});
	}

	test("storefront: fallback and settled agree across widths", async ({
		page,
	}) => {
		onlyOneProject();
		test.slow();
		const route = await discoverStorefront(page);
		for (const width of STOREFRONT_WIDTHS) {
			const fallback = await readFallbackGeometry(
				page,
				route,
				width,
				STOREFRONT_RENDERED,
			);
			const settled = await readSettledGeometry(
				page,
				route,
				width,
				STOREFRONT_RENDERED,
			);
			assertParity(route, width, fallback, settled);
		}
	});
});

function assertParity(
	route: string,
	width: number,
	fallback: GridGeometry | null,
	settled: GridGeometry | null,
) {
	expect(fallback, `${route} @${width}: no fallback geometry`).not.toBeNull();
	expect(settled, `${route} @${width}: no settled geometry`).not.toBeNull();
	if (!fallback || !settled) return;

	const context = `${route} @${width} (container ${settled.containerWidth}px)`;

	// 0. Precondition, asserted rather than assumed: both reads saw the same
	//    cards. A differing count is a real problem, but a DIFFERENT one, and
	//    without this it surfaces as a bogus "column count changed".
	expect(
		settled.cardCount,
		`${context}: SSR rendered ${fallback.cardCount} cards and the hydrated page shows ${settled.cardCount} — a card appearing or disappearing, not a column change`,
	).toBe(fallback.cardCount);

	// 1. The user-visible invariant: hydration must not re-deal the grid.
	expect(
		settled.columns,
		`${context}: column count changed on hydration — fallback ${fallback.columns} (${fallback.cardWidth}px cards), settled ${settled.columns} (${settled.cardWidth}px cards). That is the #423 shift.`,
	).toBe(fallback.columns);
	expect(
		settled.cardWidth,
		`${context}: card width changed on hydration (${fallback.cardWidth}px -> ${settled.cardWidth}px)`,
	).toBe(fallback.cardWidth);
	expect(
		settled.offsets,
		`${context}: cards landed at different column offsets after hydration`,
	).toEqual(fallback.offsets);

	// 2. The structural invariant: the measured layout IS the CSS's own track
	//    list, not a JS recomputation that merely happens to agree here.
	//
	//    A fixture can hold fewer products than there are columns (dev
	//    storefronts routinely do), and then only the first N tracks get
	//    occupied — that is correct, not a mismatch.
	expect(
		settled.columns,
		`${context}: the CSS resolved ${settled.tracks.length} tracks and there are ${settled.cardCount} cards, but the cards occupy ${settled.columns} columns — something other than the grid declaration decided the layout`,
	).toBe(Math.min(settled.tracks.length, settled.cardCount));

	const expectedOffsets = settled.offsets.map(
		(_, index) =>
			settled.offsets[0] +
			settled.tracks
				.slice(0, index)
				.reduce((sum, track) => sum + track + settled.gap, 0),
	);
	// 1px of slack, and no more: track edges land on fractional pixels
	// (275.5px here) and `getBoundingClientRect().x` is rounded to report them.
	// A reintroduced JS column formula moves offsets by ~100px, so this stays
	// far from the noise it tolerates.
	const drift = settled.offsets.map((offset, index) =>
		Math.abs(offset - expectedOffsets[index]),
	);
	expect(
		Math.max(...drift) <= 1,
		`${context}: card offsets ${settled.offsets.join(", ")} do not line up with the resolved tracks ${settled.tracks.join(" ")} (gap ${settled.gap}); expected ${expectedOffsets.map((value) => Math.round(value)).join(", ")}`,
	).toBe(true);
}
