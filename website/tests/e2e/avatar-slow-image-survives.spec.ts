import { expect, type Page, test } from "@playwright/test";

/**
 * Browser-layer guard for the Avatar rewrite — hawkr-website#430 D10.
 *
 * The acceptance condition #430 states is a DOM invariant over time:
 *
 *   "On an SPA navigation, on 2G, with the cache disabled, the number of avatar
 *    <img> elements in the DOM is always > 0; an image that arrives 6 seconds
 *    late still turns back into a picture."
 *
 * Every other avatar test is `renderToStaticMarkup` plus a read of `Avatar.tsx`'s
 * own source text (tests/avatar-slow-image-survives.test.ts). Those pin the
 * component's shape; they cannot observe the thing that actually broke. The
 * defect (#410) was a 4-second `setTimeout` that swapped the <img> for the
 * fallback — a one-way door, because the swap unmounts the element and abandons
 * the in-flight request. All three of its preconditions are live-browser ones:
 *
 *   - the timer only ever armed on CLIENT-SIDE navigation (a server-rendered
 *     <img> is already `complete` when the effect runs), so the navigation has
 *     to be a soft one, not a `page.goto`;
 *   - the image has to still be in flight past the 4s threshold;
 *   - "the photo can never come back" is only observable by delivering the
 *     bytes late and looking again.
 *
 * ── On the "2G" in that sentence ─────────────────────────────────────────────
 * Emulating 2G on the WHOLE page destroys the condition instead of creating it.
 * Measured here 2026-08-05, same fixture, only the profile changed:
 *
 *   profile        soft navigation?                    market list rendered
 *   2G  150kbps/800ms   NO — fell back to a document load     ~105s
 *   Slow-3G 400/400     NO — fell back to a document load     ~88s
 *   Fast-3G 1.6M/150    yes                                   ~1.5s
 *
 * `next dev` does not prefetch <Link>s, so a soft navigation has to fetch the
 * RSC payload at click time; below ~Fast-3G that fetch loses and the App Router
 * falls back to an MPA navigation — which server-renders the avatars and takes
 * the bug's first precondition away. #430 hit the same wall from the other side
 * ("首頁 → /market/products 2G → armed=0，頁面 payload 才是瓶頸").
 *
 * So the network conditions are applied where the condition is actually about:
 * the cache is disabled globally (a memory-cache hit would make the avatar
 * instant and there would be nothing to observe), the shell runs at the fastest
 * profile that keeps the navigation client-side, and the avatar responses are
 * held for 6s — nearly 2× the old 4s threshold and roughly 2× the 3206ms #430
 * measured on real 2G. The trade is self-policing: the test asserts that no
 * document request was made, so if the navigation ever silently degrades back
 * into a reload this fails instead of quietly testing nothing.
 *
 * Discrimination is verified, not assumed: reinstating the 4s timer in
 * `Avatar.tsx` turns this red on both halves (the count collapses to 0 and the
 * late image never paints); removing it turns it green again.
 */

/**
 * Fastest mobile profile that keeps the App Router on the client — see the
 * measurement table above before lowering it.
 */
const MOBILE_SHELL = {
	offline: false,
	latency: 150,
	downloadThroughput: (1600 * 1024) / 8,
	uploadThroughput: (750 * 1024) / 8,
	connectionType: "cellular3g",
} as const;

/** The delay #430's acceptance condition names. Comfortably past the old 4s. */
const LATE_IMAGE_MS = 6_000;

/** How long to watch the DOM after the list appears: past 4s AND past 6s. */
const OBSERVATION_MS = LATE_IMAGE_MS + 4_000;

/** Avatars are the only images the media gateway serves under this preset. */
const AVATAR_GLOB = "**/i/avatar/**";
const AVATAR_SELECTOR = 'img[src*="/i/avatar/"]';

/** 1×1 opaque PNG — enough for `naturalWidth > 0` to mean "it painted". */
const ONE_PIXEL_PNG = Buffer.from(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	"base64",
);

interface AvatarSample {
	t: number;
	n: number;
}

declare global {
	interface Window {
		__hkAvatarSamples?: AvatarSample[];
		__hkResetAvatarSamples?: () => void;
	}
}

/**
 * Record the avatar-<img> count from inside the page, for as long as we watch.
 *
 * A MutationObserver catches every DOM edit — an unmount is exactly a DOM edit —
 * and a 100ms interval keeps sampling while nothing changes, so a count that
 * sits at 0 is recorded rather than inferred from the absence of an event.
 * The script survives the soft navigation because there is no new document,
 * which is also why `addInitScript` would be the wrong tool here.
 */
async function installAvatarCounter(page: Page, selector: string) {
	await page.evaluate((sel) => {
		let origin = performance.now();
		const samples: { t: number; n: number }[] = [];
		window.__hkAvatarSamples = samples;
		window.__hkResetAvatarSamples = () => {
			origin = performance.now();
			samples.length = 0;
		};
		const sample = () => {
			samples.push({
				t: Math.round(performance.now() - origin),
				n: document.querySelectorAll(sel).length,
			});
		};
		sample();
		new MutationObserver(sample).observe(document.documentElement, {
			childList: true,
			subtree: true,
			attributes: true,
			attributeFilter: ["src"],
		});
		setInterval(sample, 100);
	}, selector);
}

// A 6s stall plus a throttled shell plus a dev-server compile is not a 30s test.
test.describe.configure({ timeout: 300_000 });

test.describe("avatar survives a slow image on an SPA navigation (#430 D10)", () => {
	test("the <img> is never unmounted, and a 6s-late image still paints", {
		tag: ["@ui", "@live"],
	}, async ({ page }) => {
		// -------------------------------------------------------------- arrange
		// Land on the home page first: the navigation under test has to be a
		// client-side one, and `page.goto` is by definition not that.
		await page.goto("/", { waitUntil: "domcontentloaded" });
		await expect(page.locator(".initial-loader")).toBeHidden({
			timeout: 60_000,
		});

		const cdp = await page.context().newCDPSession(page);
		await cdp.send("Network.enable");
		await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });
		await cdp.send("Network.emulateNetworkConditions", MOBILE_SHELL);

		// Hold every avatar for 6 seconds, THEN answer with a real image. This
		// is the case #430 proved was unrecoverable: the old code fired at 4s,
		// the element was gone, and the bytes arriving at 6s had nothing to
		// paint into ("14 秒後 DOM 中 avatar <img> 仍為 0").
		let avatarRequests = 0;
		await page.route(AVATAR_GLOB, async (route) => {
			avatarRequests += 1;
			await new Promise((resolve) => setTimeout(resolve, LATE_IMAGE_MS));
			await route.fulfill({
				status: 200,
				contentType: "image/png",
				body: ONE_PIXEL_PNG,
				headers: { "cache-control": "no-store" },
			});
		});

		await installAvatarCounter(page, AVATAR_SELECTOR);

		// ----------------------------------------------------------------- act
		// Count document requests so "SPA navigation" is asserted, not assumed:
		// a reload server-renders the avatars, where the bug never existed.
		const marketDocumentRequests: string[] = [];
		page.on("request", (request) => {
			if (
				request.resourceType() === "document" &&
				new URL(request.url()).pathname.startsWith("/market/products")
			) {
				marketDocumentRequests.push(request.url());
			}
		});

		const toMarket = page
			.locator('.home-chip[href="/market/products"]')
			.first();
		await expect(toMarket).toBeVisible({ timeout: 60_000 });
		await toMarket.click();
		// NOT `waitForURL`: its default `waitUntil: "load"` waits on a load
		// event that a client-side navigation never fires.
		await expect(page).toHaveURL(/\/market\/products/, { timeout: 120_000 });

		// The list is client-rendered; its avatars are created by JS, which is
		// the only situation in which the old timer ever armed.
		await expect(
			page.locator('[data-hk-market-products-hydrated="true"]'),
		).toBeAttached({ timeout: 120_000 });
		await expect(page.locator(AVATAR_SELECTOR).first()).toBeAttached({
			timeout: 60_000,
		});

		// Start the observation window here. Before this point a momentary 0 is
		// legitimate — the previous route's avatars are gone and this route's
		// are not mounted yet — and asserting over it would test the router's
		// transition, not the component.
		await page.evaluate(() => window.__hkResetAvatarSamples?.());
		await page.waitForTimeout(OBSERVATION_MS);

		// -------------------------------------------------------------- assert
		const samples = (await page.evaluate(
			() => window.__hkAvatarSamples ?? [],
		)) as AvatarSample[];

		expect(
			marketDocumentRequests,
			"the click caused a full page load — the soft navigation this condition depends on did not happen, so nothing here was actually tested",
		).toEqual([]);
		// Guard against a vacuous pass: with no avatar and no request there is
		// no invariant to violate and "never dropped to 0" would be a lie.
		expect(
			avatarRequests,
			"no avatar image was requested — the fixture, not the component, is broken",
		).toBeGreaterThan(0);
		expect(
			samples.length,
			"the in-page counter produced no samples",
		).toBeGreaterThan(OBSERVATION_MS / 1000);
		expect(
			samples[0].n,
			"the observation window opened with no avatar in the DOM",
		).toBeGreaterThan(0);

		// THE invariant. Once an avatar exists, nothing may take it away while
		// its request is still alive — slowness is not failure.
		const collapsed = samples.filter((s) => s.n === 0);
		expect(
			collapsed,
			`avatar <img> count fell to 0 while the requests were still in flight — the element was unmounted mid-request. First collapse at t=${collapsed[0]?.t}ms of a ${OBSERVATION_MS}ms window that opened with ${samples[0].n} avatars`,
		).toEqual([]);

		// And the second half of the condition: the late bytes actually become a
		// picture. `complete && naturalWidth > 0` is the browser's own answer to
		// "did this element paint an image", so an element that merely exists
		// cannot satisfy it.
		await expect
			.poll(
				() =>
					page.evaluate((sel) => {
						const imgs = Array.from(
							document.querySelectorAll<HTMLImageElement>(sel),
						);
						return imgs.filter((img) => img.complete && img.naturalWidth > 0)
							.length;
					}, AVATAR_SELECTOR),
				{
					timeout: 60_000,
					message:
						"the 6s-late avatar never painted: the element survived but no image arrived in it",
				},
			)
			.toBeGreaterThan(0);
	});
});
