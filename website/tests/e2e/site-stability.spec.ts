import {
	type APIRequestContext,
	type ConsoleMessage,
	expect,
	type Page,
	type Request,
	test,
} from "@playwright/test";
import { canonicalSellerSegment } from "./helpers/canonical-seller";
import { captureHydrationDiagnostics } from "./helpers/hydration-diagnostics";
import { fulfillConnectError, GET_SESSION_URL } from "./helpers/mock-gateway";
import {
	assertNoResidualLoading,
	RESIDUAL_LOADING_SELECTOR,
} from "./helpers/page-stability";
import {
	type DocumentResourceHints,
	partitionStabilityWarnings,
} from "./helpers/preload-hygiene";
import {
	isForbiddenProductionMediaReference,
	isProductionArtifact,
	TEST_MEDIA_HOSTNAME,
} from "./helpers/production-media-guard";
import { gotoStreamed } from "./helpers/streamed-navigation";

const LONG_PUBLIC_ROUTES = [
	"/",
	"/feed",
	"/market/products",
	"/products/:id",
	"/search",
	"/seller",
	"/about",
	"/sellers/:id",
	"/sellers/:id/posts",
	"/sellers/:id/reviews",
	"/sellers/:id/messages",
] as const;

const PROGRESSIVE_PUBLIC_IMAGE_ROUTES = new Set<
	(typeof LONG_PUBLIC_ROUTES)[number]
>(["/", "/feed", "/market/products", "/products/:id", "/search"]);

const READY_SELECTOR: Partial<
	Record<(typeof LONG_PUBLIC_ROUTES)[number], string>
> = {
	"/": '[data-hk-initial-content-ready="home"]',
	"/feed": '[data-hk-tab-content-ready="feed"]',
	"/market/products": '[data-hk-tab-content-ready="market"]',
	"/products/:id": '[data-hk-page-content-ready="product-detail"]',
};

const ARTIFACT_FLAVOR = process.env.NEXT_PUBLIC_FLAVOR;
const IS_PRODUCTION_ARTIFACT = isProductionArtifact(ARTIFACT_FLAVOR);

const INITIAL_CLS_LIMIT = {
	chromium: 0.05,
	mobile: 0.12,
} as const;

const STABILITY_WARNING_PATTERN =
	/preloaded using link preload but not used|preloaded.+not used within|inappropriate.+\bas\b|appropriate [`'"]?as[`'"]? value|has either width or height modified/iu;

interface LayoutShiftEntry {
	value: number;
	hadRecentInput: boolean;
	sources?: Array<{
		node?: Node | null;
		previousRect?: DOMRectReadOnly;
		currentRect?: DOMRectReadOnly;
	}>;
}

type SellerContentKind = "posts" | "reviews" | "forum";

interface PageFixtures {
	productId?: string;
	/**
	 * Canonical storefront path segment (`store_code`), never the `store_id` the
	 * product feed exposes. See `discoverPageFixtures`.
	 */
	defaultSellerId?: string;
	/** Canonical `store_code` per content kind — same contract as above. */
	content: Partial<Record<SellerContentKind, string>>;
}

interface ForbiddenMediaDomReference {
	element: "img" | "link";
	attribute: "src" | "srcset" | "href";
	value: string;
	resolvedUrl: string;
}

type StabilityWindow = Window & {
	__hkLayoutShifts?: number[];
	__hkLayoutShiftDetails?: string[];
	__hkGeometryTimeline?: string[];
	__hkFallbackEvents?: string[];
	__hkFallbackObserver?: MutationObserver;
	__hkForbiddenMediaDomReferences?: ForbiddenMediaDomReference[];
	__hkForbiddenMediaDomObserver?: MutationObserver;
	__hkAuthoredPreloads?: string[];
};

function isImageRequest(request: Request): boolean {
	return request.method() === "GET" && request.resourceType() === "image";
}

function isStabilityWarning(message: ConsoleMessage): boolean {
	return (
		(message.type() === "warning" || message.type() === "error") &&
		STABILITY_WARNING_PATTERN.test(message.text())
	);
}

function installProductionMediaDomObserver({
	enabled,
	forbiddenHostname,
}: {
	enabled: boolean;
	forbiddenHostname: string;
}) {
	const target = window as StabilityWindow;
	target.__hkForbiddenMediaDomReferences = [];
	target.__hkForbiddenMediaDomObserver?.disconnect();
	if (!enabled) return;

	const normalizeHostname = (hostname: string) =>
		hostname.toLowerCase().replace(/\.+$/, "");
	const seen = new Set<string>();
	const recordAttribute = (
		element: HTMLImageElement | HTMLLinkElement,
		attribute: "src" | "srcset" | "href",
	) => {
		const value = element.getAttribute(attribute);
		if (!value) return;
		const candidates =
			attribute === "srcset"
				? value
						.split(",")
						.map((candidate) => candidate.trim().split(/\s+/, 1)[0])
						.filter((candidate): candidate is string => Boolean(candidate))
				: [value];

		for (const candidate of candidates) {
			try {
				const resolvedUrl = new URL(candidate, document.baseURI);
				if (
					normalizeHostname(resolvedUrl.hostname) !==
					normalizeHostname(forbiddenHostname)
				) {
					continue;
				}
				const reference: ForbiddenMediaDomReference = {
					element: element.localName as "img" | "link",
					attribute,
					value,
					resolvedUrl: resolvedUrl.href,
				};
				const signature = JSON.stringify(reference);
				if (seen.has(signature)) continue;
				seen.add(signature);
				target.__hkForbiddenMediaDomReferences?.push(reference);
			} catch {}
		}
	};
	const inspectElement = (element: Element) => {
		if (element instanceof HTMLImageElement) {
			recordAttribute(element, "src");
			recordAttribute(element, "srcset");
		}
		if (element instanceof HTMLLinkElement) {
			recordAttribute(element, "href");
		}
	};
	const inspectSubtree = (root: Document | Element) => {
		if (root instanceof Element) inspectElement(root);
		for (const element of root.querySelectorAll(
			"img[src], img[srcset], link[href]",
		)) {
			inspectElement(element);
		}
	};

	inspectSubtree(document);
	target.__hkForbiddenMediaDomObserver = new MutationObserver((mutations) => {
		for (const mutation of mutations) {
			if (mutation.type === "attributes") {
				if (mutation.target instanceof Element) {
					inspectElement(mutation.target);
				}
				continue;
			}
			for (const node of mutation.addedNodes) {
				if (node instanceof Element) inspectSubtree(node);
			}
		}
	});
	target.__hkForbiddenMediaDomObserver.observe(document, {
		attributes: true,
		attributeFilter: ["src", "srcset", "href"],
		childList: true,
		subtree: true,
	});
}

async function watchProductionMediaDom(page: Page) {
	await page.addInitScript(installProductionMediaDomObserver, {
		enabled: IS_PRODUCTION_ARTIFACT,
		forbiddenHostname: TEST_MEDIA_HOSTNAME,
	});
}

/**
 * 記下「這份文件自己宣告的」預載提示。DOMContentLoaded 代表 HTML（含串流的
 * 部分）已經全部解析完，所以此刻在場的 link[rel=preload] 就是作者寫進頁面的
 * 那些；router 為下一條路由插入的暖身提示一律晚於此。兩者的差別是這條 gate
 * 唯一分得清缺陷與暖身的依據，見 helpers/preload-hygiene.ts。
 */
async function recordAuthoredResourceHints(page: Page) {
	await page.addInitScript(() => {
		const target = window as StabilityWindow;
		const snapshot = () => {
			target.__hkAuthoredPreloads = Array.from(
				document.querySelectorAll<HTMLLinkElement>('link[rel="preload"]'),
			).map((link) => link.href);
		};
		if (document.readyState === "loading") {
			document.addEventListener("DOMContentLoaded", snapshot, { once: true });
		} else {
			snapshot();
		}
	});
}

async function assertInitialLoaderHandoff(page: Page, route: string) {
	await expect(
		page.locator("html"),
		`${route} kept the initial-loader-active document lock after ready`,
	).not.toHaveClass(/\binitial-loader-active\b/);
	await expect(
		page.locator(".initial-loader"),
		`${route} hid the initial loader without detaching it after ready`,
	).toHaveCount(0);
}

async function installLayoutShiftObserver(page: Page) {
	await page.addInitScript(() => {
		const target = window as StabilityWindow;
		target.__hkLayoutShifts = [];
		target.__hkLayoutShiftDetails = [];
		target.__hkGeometryTimeline = [];
		let previousGeometry = "";
		const recordGeometry = () => {
			const body = document.body;
			const nav = document.querySelector<HTMLElement>(".auto-nav");
			const home = document.querySelector<HTMLElement>(".hkh-page");
			const loader = document.querySelector<HTMLElement>(".initial-loader");
			const wrapper = document.querySelector<HTMLElement>(".page-wrapper");
			const loaderStyle = loader ? getComputedStyle(loader) : null;
			const wrapperStyle = wrapper ? getComputedStyle(wrapper) : null;
			const geometry = [
				`body=${body?.getBoundingClientRect().height ?? "none"}`,
				`nav=${nav?.getBoundingClientRect().height ?? "none"}`,
				`homeY=${home?.getBoundingClientRect().y ?? "none"}`,
				`loader=${loaderStyle?.visibility ?? "none"}/${loaderStyle?.opacity ?? "none"}`,
				`wrapper=${wrapperStyle?.visibility ?? "none"}`,
				`active=${document.documentElement.classList.contains("initial-loader-active")}`,
			].join(",");
			if (
				geometry !== previousGeometry &&
				(target.__hkGeometryTimeline?.length ?? 0) < 80
			) {
				target.__hkGeometryTimeline?.push(
					`${performance.now().toFixed(1)} ${geometry}`,
				);
				previousGeometry = geometry;
			}
			window.requestAnimationFrame(recordGeometry);
		};
		window.requestAnimationFrame(recordGeometry);
		try {
			new PerformanceObserver((list) => {
				for (const rawEntry of list.getEntries()) {
					const entry = rawEntry as PerformanceEntry & LayoutShiftEntry;
					if (!entry.hadRecentInput) {
						target.__hkLayoutShifts?.push(entry.value);
						const sources = (entry.sources ?? []).map((source) => {
							const element =
								source.node instanceof Element
									? source.node
									: source.node?.parentElement;
							const label = element
								? `${element.tagName.toLowerCase()}${element.id ? `#${element.id}` : ""}${Array.from(
										element.classList,
									)
										.slice(0, 3)
										.map((name) => `.${name}`)
										.join("")}`
								: "unknown";
							const previous = source.previousRect;
							const current = source.currentRect;
							return `${label} ${previous?.x ?? "?"},${previous?.y ?? "?"},${previous?.width ?? "?"}x${previous?.height ?? "?"} -> ${current?.x ?? "?"},${current?.y ?? "?"},${current?.width ?? "?"}x${current?.height ?? "?"}`;
						});
						target.__hkLayoutShiftDetails?.push(
							`t=${entry.startTime.toFixed(1)} ${entry.value.toFixed(6)} ${sources.join(" | ")}`,
						);
					}
				}
			}).observe({ type: "layout-shift", buffered: true });
		} catch {}
	});
}

async function prepareGuestSession(page: Page) {
	await page.route(GET_SESSION_URL, async (route) => {
		await fulfillConnectError(route, "unauthenticated", "guest");
	});
}

function startFallbackObserver({
	selector,
	ignoreInitialLoader,
}: {
	selector: string;
	ignoreInitialLoader: boolean;
}) {
	const target = window as StabilityWindow;
	target.__hkFallbackEvents = [];
	target.__hkFallbackObserver?.disconnect();

	const isVisible = (element: HTMLElement) => {
		const style = getComputedStyle(element);
		const rect = element.getBoundingClientRect();
		return (
			!element.hidden &&
			rect.width > 0 &&
			rect.height > 0 &&
			style.display !== "none" &&
			style.visibility !== "hidden" &&
			Number(style.opacity || "1") > 0
		);
	};

	const recordVisibleFallbacks = () => {
		const initialLoader =
			document.querySelector<HTMLElement>(".initial-loader");
		if (ignoreInitialLoader && initialLoader && isVisible(initialLoader)) {
			return;
		}
		const visible = Array.from(
			document.querySelectorAll<HTMLElement>(selector),
		).filter((element) => {
			if (
				ignoreInitialLoader &&
				element.closest<HTMLElement>(".initial-loader")
			) {
				return false;
			}
			return isVisible(element);
		});
		for (const element of visible) {
			const signature = [
				element.tagName.toLowerCase(),
				element.className,
				element.getAttribute("aria-label") ?? "",
			].join(":");
			if (!target.__hkFallbackEvents?.includes(signature)) {
				target.__hkFallbackEvents?.push(signature);
			}
		}
	};

	target.__hkFallbackObserver = new MutationObserver(recordVisibleFallbacks);
	target.__hkFallbackObserver.observe(document, {
		attributes: true,
		childList: true,
		subtree: true,
		attributeFilter: ["aria-busy", "class", "hidden", "style"],
	});
	recordVisibleFallbacks();
}

async function watchHardNavigationFallbacks(page: Page) {
	await page.addInitScript(startFallbackObserver, {
		selector: RESIDUAL_LOADING_SELECTOR,
		ignoreInitialLoader: true,
	});
}

let pageFixturesPromise: Promise<PageFixtures> | undefined;

async function collectionCount(
	request: APIRequestContext,
	sellerId: string,
	kind: SellerContentKind,
): Promise<number> {
	const response = await request.get(
		`/api/v1/public/sellers/${sellerId}/${kind}?page=1&page_size=10`,
		{ failOnStatusCode: false, timeout: 15_000 },
	);
	if (response.status() !== 200) return 0;
	const envelope = (await response.json()) as {
		data?:
			| unknown[]
			| {
					records?: unknown[];
					comments?: unknown[];
					items?: unknown[];
			  };
	};
	if (Array.isArray(envelope.data)) return envelope.data.length;
	const data = envelope.data ?? {};
	return Math.max(
		data.records?.length ?? 0,
		data.comments?.length ?? 0,
		data.items?.length ?? 0,
	);
}

/**
 * Builds the fixtures the long-page tests navigate to.
 *
 * Seller fixtures are resolved to CANONICAL urls (`/sellers/{store_code}`)
 * even though the product feed only exposes `store_id` — see
 * `canonicalSellerSegment`. These tests measure whether a settled long page
 * shifts, remounts a fallback, or self-paginates while the user scrolls, i.e.
 * the stability of the storefront page itself; entering by `store_id` measures
 * the `308` in front of it instead. Sellers the backend says have no store_code
 * are dropped rather than visited non-canonically, so the set that is measured
 * is always the set that renders directly; a lookup that could not be answered
 * at all fails the run instead of silently shrinking that set.
 */
async function discoverPageFixtures(
	request: APIRequestContext,
): Promise<PageFixtures> {
	const response = await request.get(
		"/api/v1/public/products?keyword=&page=1&page_size=50",
		{ failOnStatusCode: false, timeout: 15_000 },
	);
	expect(response.status(), "seller fixture response").toBe(200);
	const envelope = (await response.json()) as {
		data?: {
			records?: Array<{
				id?: unknown;
				storeId?: unknown;
				account_id?: unknown;
				seller?: {
					id?: unknown;
					storeId?: unknown;
					store_id?: unknown;
				};
			}>;
		};
	};
	const records = envelope.data?.records ?? [];
	const productId = records
		.map((record) => String(record.id ?? ""))
		.find((id) => /^\d+$/.test(id));
	const storeIds = Array.from(
		new Set(
			records
				.map((record) =>
					String(
						record.storeId ??
							record.account_id ??
							record.seller?.storeId ??
							record.seller?.store_id ??
							record.seller?.id ??
							"",
					),
				)
				.filter((id) => /^\d+$/.test(id)),
		),
	);
	const sellerIds = Array.from(
		new Set(
			(
				await Promise.all(
					storeIds.map((storeId) => canonicalSellerSegment(request, storeId)),
				)
			).filter((storeCode): storeCode is string => Boolean(storeCode)),
		),
	);
	const content: Partial<Record<SellerContentKind, string>> = {};
	const contentCounts: Partial<Record<SellerContentKind, number>> = {};
	for (const sellerId of sellerIds) {
		const kinds = ["posts", "reviews", "forum"] as const;
		const counts = await Promise.all(
			kinds.map((kind) => collectionCount(request, sellerId, kind)),
		);
		for (const [index, kind] of kinds.entries()) {
			const count = counts[index] ?? 0;
			if (count > (contentCounts[kind] ?? 0)) {
				content[kind] = sellerId;
				contentCounts[kind] = count;
			}
		}
	}

	return {
		productId: productId || undefined,
		defaultSellerId: sellerIds[0],
		content,
	};
}

async function pageFixtures(request: APIRequestContext): Promise<PageFixtures> {
	pageFixturesPromise ??= discoverPageFixtures(request);
	return pageFixturesPromise;
}

async function sellerFixtureForRoute(
	request: APIRequestContext,
	route: (typeof LONG_PUBLIC_ROUTES)[number],
): Promise<{ sellerId: string | null; hasExpectedContent: boolean }> {
	const fixtures = await pageFixtures(request);
	const expectedKind = route.endsWith("/posts")
		? "posts"
		: route.endsWith("/reviews")
			? "reviews"
			: route.endsWith("/messages")
				? "forum"
				: undefined;
	const sellerId = expectedKind
		? fixtures.content[expectedKind]
		: fixtures.defaultSellerId;
	return {
		sellerId: sellerId ?? fixtures.defaultSellerId ?? null,
		hasExpectedContent: expectedKind ? Boolean(sellerId) : true,
	};
}

async function watchForFallbackReappearance(page: Page) {
	await page.evaluate(startFallbackObserver, {
		selector: RESIDUAL_LOADING_SELECTOR,
		ignoreInitialLoader: false,
	});
}

async function inspectInitialImageScheduling(page: Page) {
	return page.evaluate(async () => {
		const scheduled: string[] = [];
		const declared: string[] = [];
		const lazy: string[] = [];
		const pending: string[] = [];
		const failed: string[] = [];

		const candidateUrls = (image: HTMLImageElement) => {
			const urls = [image.currentSrc, image.src].filter(Boolean);
			for (const entry of (image.getAttribute("srcset") ?? "").split(",")) {
				const candidate = entry.trim().split(/\s+/)[0];
				if (!candidate) continue;
				try {
					urls.push(new URL(candidate, document.baseURI).href);
				} catch {}
			}
			return urls;
		};

		for (const image of document.querySelectorAll<HTMLImageElement>("img")) {
			const style = getComputedStyle(image);
			const rect = image.getBoundingClientRect();
			const visible =
				!image.hidden &&
				rect.width > 0 &&
				rect.height > 0 &&
				style.display !== "none" &&
				style.visibility !== "hidden";
			const urls = Array.from(
				new Set([image.currentSrc, image.src].filter(Boolean)),
			);
			declared.push(...candidateUrls(image));
			if (urls.length === 0) continue;
			if (image.loading === "lazy") {
				if (visible) lazy.push(...urls);
				continue;
			}
			scheduled.push(...urls);
			if (!image.complete) {
				pending.push(...urls);
				continue;
			}
			if (image.naturalWidth <= 0 || image.naturalHeight <= 0) {
				failed.push(...urls);
				continue;
			}
			try {
				await image.decode();
			} catch {
				failed.push(...urls);
			}
		}

		return {
			scheduled: Array.from(new Set(scheduled)),
			declared: Array.from(new Set(declared)),
			lazy: Array.from(new Set(lazy)),
			pending: Array.from(new Set(pending)),
			failed: Array.from(new Set(failed)),
		};
	});
}

/**
 * The user-facing half of "no pop-in": every image the viewport currently shows
 * must be decoded, not an empty box the visitor watches fill in.
 *
 * Deliberately expressed over what is ON SCREEN rather than over request
 * timing. Whether Chromium has already fetched a below-fold `loading="lazy"`
 * image by the time we scroll is decided by its lazy-load root margin, which is
 * derived from the effective connection type (measured: 4g ⇒ ~1250px,
 * 3g/unknown ⇒ 2500-5000px) — an environment property, not a property of this
 * page. Asserting on it made this suite disagree between a dev machine and CI
 * (hawkr-website#370). A jump-scroll also removes the head start that
 * margin normally provides, so the check polls: the contract is "the visitor
 * does not sit in front of a blank image", not "loading takes zero time".
 */
async function assertViewportImagesDecoded(page: Page, route: string) {
	await expect
		.poll(
			() =>
				page.evaluate(() => {
					const blank: string[] = [];
					for (const image of document.querySelectorAll<HTMLImageElement>(
						"img",
					)) {
						const rect = image.getBoundingClientRect();
						const style = getComputedStyle(image);
						const onScreen =
							rect.bottom > 0 &&
							rect.top < window.innerHeight &&
							rect.right > 0 &&
							rect.left < window.innerWidth;
						const rendered =
							!image.hidden &&
							rect.width > 0 &&
							rect.height > 0 &&
							style.display !== "none" &&
							style.visibility !== "hidden" &&
							Number(style.opacity || "1") > 0;
						if (!onScreen || !rendered) continue;
						if (!image.complete || image.naturalWidth <= 0) {
							blank.push(image.currentSrc || image.src);
						}
					}
					return Array.from(new Set(blank));
				}),
			{
				message: `${route} showed an image inside the viewport that never finished loading`,
				timeout: 8_000,
			},
		)
		.toEqual([]);
}

test.describe("long-page stability after ready", {
	tag: ["@ui", "@live"],
}, () => {
	test.describe.configure({ retries: 0 });

	for (const route of LONG_PUBLIC_ROUTES) {
		test(`${route} does not shift, remount a fallback, or paginate while scrolling`, async ({
			page,
			request,
		}, testInfo) => {
			test.setTimeout(95_000);
			let sellerFixture: Awaited<
				ReturnType<typeof sellerFixtureForRoute>
			> | null = null;
			let targetRoute: string = route;
			if (route === "/products/:id") {
				const fixtures = await pageFixtures(request);
				test.skip(
					!fixtures.productId,
					"the current public dataset has no product fixture; dev data and source contracts retain populated PDP coverage",
				);
				if (!fixtures.productId) return;
				targetRoute = route.replace(":id", fixtures.productId);
			} else if (route.includes(":id")) {
				sellerFixture = await sellerFixtureForRoute(request, route);
				test.skip(
					!sellerFixture.sellerId,
					"the current public dataset has no seller fixture; dev data retains populated storefront coverage",
				);
				if (!sellerFixture.sellerId) return;
				targetRoute = route.replace(":id", sellerFixture.sellerId);
			}
			if (sellerFixture && !sellerFixture.hasExpectedContent) {
				testInfo.annotations.push({
					type: "fixture-gap",
					description: `${route} has no content-bearing seller in the current dev public dataset; the empty state remains runtime-tested and populated rendering is covered by source contracts`,
				});
			}
			const stabilityWarnings: string[] = [];
			const pageErrors: string[] = [];
			const failedImageResponses: string[] = [];
			const failedImageRequests: string[] = [];
			const forbiddenMediaRequestUrls: string[] = [];
			const onConsole = (message: ConsoleMessage) => {
				if (isStabilityWarning(message)) {
					stabilityWarnings.push(message.text());
				}
			};
			const onPageError = (error: Error) => {
				pageErrors.push(error.stack ?? error.message);
			};
			const onResponse = (response: import("@playwright/test").Response) => {
				if (isImageRequest(response.request()) && response.status() >= 400) {
					failedImageResponses.push(
						`${response.status()} ${response.request().url()}`,
					);
				}
			};
			const onRequestFailed = (request: Request) => {
				if (isImageRequest(request)) {
					failedImageRequests.push(
						`${request.failure()?.errorText ?? "request failed"} ${request.url()}`,
					);
				}
			};
			const onProductionMediaRequest = (request: Request) => {
				if (
					isForbiddenProductionMediaReference(ARTIFACT_FLAVOR, request.url())
				) {
					forbiddenMediaRequestUrls.push(request.url());
				}
			};
			captureHydrationDiagnostics(page);
			page.on("console", onConsole);
			page.on("pageerror", onPageError);
			page.on("response", onResponse);
			page.on("requestfailed", onRequestFailed);
			page.on("request", onProductionMediaRequest);
			await installLayoutShiftObserver(page);
			await watchHardNavigationFallbacks(page);
			await watchProductionMediaDom(page);
			await recordAuthoredResourceHints(page);
			await prepareGuestSession(page);

			await gotoStreamed(page, targetRoute, {
				timeout: 25_000,
			});
			await expect(page.locator(".initial-loader")).toBeHidden({
				timeout: 12_000,
			});
			await expect(
				page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
			).toBeAttached({
				timeout: 12_000,
			});
			const readySelector = READY_SELECTOR[route];
			if (readySelector) {
				await expect(
					page.locator(`main.page-content ${readySelector}`),
				).toBeAttached({
					timeout: 12_000,
				});
			}
			if (sellerFixture?.hasExpectedContent && route.endsWith("/posts")) {
				await expect(
					page.locator(".seller-posts-board .hkui-feed-card").first(),
				).toBeVisible();
			}
			if (sellerFixture?.hasExpectedContent && route.endsWith("/reviews")) {
				await expect(
					page.locator("[data-hk-seller-review-item]").first(),
				).toBeVisible();
			}
			if (sellerFixture?.hasExpectedContent && route.endsWith("/messages")) {
				await expect(
					page.locator("[data-hk-seller-forum-comment]").first(),
				).toBeVisible();
			}
			await assertInitialLoaderHandoff(page, targetRoute);
			await assertNoResidualLoading(page, targetRoute);
			await page.waitForTimeout(500);

			const initialCls = await page.evaluate(() => {
				const target = window as StabilityWindow;
				const value = (target.__hkLayoutShifts ?? []).reduce(
					(sum, shift) => sum + shift,
					0,
				);
				const details = target.__hkLayoutShiftDetails ?? [];
				const geometry = target.__hkGeometryTimeline ?? [];
				target.__hkLayoutShifts = [];
				target.__hkLayoutShiftDetails = [];
				target.__hkGeometryTimeline = [];
				window.scrollTo(0, 0);
				return { value, details, geometry };
			});
			const initialClsLimit =
				INITIAL_CLS_LIMIT[
					testInfo.project.name as keyof typeof INITIAL_CLS_LIMIT
				] ?? INITIAL_CLS_LIMIT.chromium;
			console.log(
				`[cls] project=${testInfo.project.name} route=${targetRoute} initial=${initialCls.value.toFixed(4)} limit=${initialClsLimit}`,
			);
			expect(
				initialCls.value,
				`${targetRoute} shifted during hard load or the loader-to-content handoff: ${initialCls.details.join("; ")} | geometry: ${initialCls.geometry.join("; ")}`,
			).toBeLessThanOrEqual(initialClsLimit);
			const initialHeight = await page.evaluate(
				() => document.documentElement.scrollHeight,
			);
			await expect
				.poll(
					async () => {
						const images = await inspectInitialImageScheduling(page);
						return { pending: images.pending, failed: images.failed };
					},
					{
						message: `${targetRoute} still had an incomplete or undecodable image before scroll`,
						timeout: 45_000,
					},
				)
				.toEqual({ pending: [], failed: [] });
			const initialImageScheduling = await inspectInitialImageScheduling(page);
			if (!PROGRESSIVE_PUBLIC_IMAGE_ROUTES.has(route)) {
				expect(
					initialImageScheduling.lazy,
					`${targetRoute} left an initially-rendered image lazy after ready`,
				).toEqual([]);
			}
			await assertViewportImagesDecoded(page, targetRoute);
			const declaredImageUrls = new Set(initialImageScheduling.declared);
			const heights = [initialHeight];
			const midScrollPageRequests: string[] = [];
			const undeclaredImageRequests: string[] = [];
			const onRequest = (request: Request) => {
				const url = new URL(request.url());
				if (
					request.method() === "GET" &&
					/\/api\/v1\/(?:public|app)\//.test(url.pathname)
				) {
					midScrollPageRequests.push(`${url.pathname}${url.search}`);
				}
				if (isImageRequest(request)) {
					if (!declaredImageUrls.has(request.url())) {
						undeclaredImageRequests.push(request.url());
					}
				}
			};
			page.on("request", onRequest);

			for (const ratio of [0.18, 0.35, 0.55, 0.75, 0.92, 1]) {
				await page.evaluate((nextRatio) => {
					const max = Math.max(
						0,
						document.documentElement.scrollHeight - window.innerHeight,
					);
					window.scrollTo(0, max * nextRatio);
				}, ratio);
				await page.waitForTimeout(ratio === 1 ? 700 : 350);
				heights.push(
					await page.evaluate(() => document.documentElement.scrollHeight),
				);
				await assertNoResidualLoading(page, targetRoute);
				await assertViewportImagesDecoded(page, targetRoute);
			}
			await page.waitForTimeout(3_500);
			page.off("request", onRequest);
			page.off("console", onConsole);
			page.off("pageerror", onPageError);
			page.off("response", onResponse);
			page.off("requestfailed", onRequestFailed);
			page.off("request", onProductionMediaRequest);

			const cls = await page.evaluate(() =>
				((window as StabilityWindow).__hkLayoutShifts ?? []).reduce(
					(sum, value) => sum + value,
					0,
				),
			);
			const fallbackEvents = await page.evaluate(() => {
				const target = window as StabilityWindow;
				target.__hkFallbackObserver?.disconnect();
				return target.__hkFallbackEvents ?? [];
			});
			const forbiddenMediaDomReferences = await page.evaluate(() => {
				const target = window as StabilityWindow;
				target.__hkForbiddenMediaDomObserver?.disconnect();
				return target.__hkForbiddenMediaDomReferences ?? [];
			});
			const resourceHints: DocumentResourceHints = await page.evaluate(() => {
				const target = window as StabilityWindow;
				const hrefs = (selector: string) =>
					Array.from(document.querySelectorAll<HTMLLinkElement>(selector)).map(
						(link) => link.href,
					);
				return {
					authoredPreloads: target.__hkAuthoredPreloads ?? [],
					styleHints: hrefs('link[rel="preload"][as="style"]'),
					appliedStylesheets: hrefs('link[rel="stylesheet"]'),
				};
			});
			const { defects: preloadDefects, routeWarmups } =
				partitionStabilityWarnings(stabilityWarnings, resourceHints);
			for (const warmup of routeWarmups) {
				testInfo.annotations.push({
					type: "route-warmup-preload",
					description: warmup,
				});
			}
			expect(
				Math.max(...heights) - Math.min(...heights),
				`${targetRoute} changed document height after the page-ready handoff`,
			).toBeLessThanOrEqual(1);
			expect(
				cls,
				`${targetRoute} accumulated layout shift after the page-ready handoff`,
			).toBeLessThanOrEqual(0.001);
			expect(
				fallbackEvents,
				`${targetRoute} remounted a fallback after the page-ready handoff`,
			).toEqual([]);
			expect(
				midScrollPageRequests,
				`${targetRoute} started a content request only because the user scrolled`,
			).toEqual([]);
			expect(
				undeclaredImageRequests,
				`${targetRoute} fetched an image the initial render never declared, only because the user scrolled`,
			).toEqual([]);
			expect(
				forbiddenMediaRequestUrls,
				`${targetRoute} requested the test media gateway from a production artifact`,
			).toEqual([]);
			expect(
				forbiddenMediaDomReferences,
				`${targetRoute} rendered the test media gateway in img src/srcset or link href from a production artifact`,
			).toEqual([]);
			expect(
				failedImageResponses,
				`${targetRoute} received an unsuccessful image response`,
			).toEqual([]);
			expect(
				failedImageRequests,
				`${targetRoute} had a failed image request`,
			).toEqual([]);
			expect(
				preloadDefects,
				`${targetRoute} emitted an unused/invalid preload or image geometry warning`,
			).toEqual([]);
			expect(
				pageErrors,
				`${targetRoute} threw during streaming or hydration`,
			).toEqual([]);
		});
	}
});

test.describe("soft-navigation stability", { tag: ["@ui", "@live"] }, () => {
	test("keeps the committed page visible until a delayed destination is complete", async ({
		page,
	}) => {
		test.setTimeout(30_000);
		await prepareGuestSession(page);
		await gotoStreamed(page, "/about", {
			timeout: 25_000,
		});
		await expect(page.locator(".initial-loader")).toBeHidden({
			timeout: 12_000,
		});
		await expect(
			page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
		).toBeAttached({
			timeout: 12_000,
		});
		await assertInitialLoaderHandoff(page, "/about");
		await assertNoResidualLoading(page, "/about");

		const sourceHeading = page.getByText("認識 Hawkr · ABOUT", {
			exact: true,
		});
		await expect(sourceHeading).toBeVisible();
		await watchForFallbackReappearance(page);

		let releaseDestination = () => {};
		let destinationRequestSeen = false;
		const destinationGate = new Promise<void>((resolve) => {
			releaseDestination = resolve;
		});
		await page.route("**/faq**", async (route) => {
			const request = route.request();
			const url = new URL(request.url());
			const isRoutePayload =
				request.method() === "GET" &&
				(request.headers().rsc === "1" || url.searchParams.has("_rsc"));
			if (!isRoutePayload) {
				await route.continue();
				return;
			}
			destinationRequestSeen = true;
			await destinationGate;
			await route.continue();
		});

		const faqLink = page.locator('a[href="/faq"]').last();
		await faqLink.scrollIntoViewIfNeeded();
		const click = faqLink.click();
		try {
			await expect
				.poll(() => destinationRequestSeen, { timeout: 5_000 })
				.toBe(true);
			await page.waitForTimeout(500);
			await expect(sourceHeading).toBeVisible();
			await expect(page.locator("main.page-content")).toContainText(
				"認識 Hawkr",
			);
			await assertNoResidualLoading(page, "delayed /about → /faq");
		} finally {
			releaseDestination();
		}
		await click;

		await expect(page).toHaveURL(/\/faq$/);
		await expect(
			page.getByText("Hawkr 的常見問題與解答", { exact: true }),
		).toBeVisible();
		await assertNoResidualLoading(page, "/faq");
		const fallbackEvents = await page.evaluate(() => {
			const target = window as StabilityWindow;
			target.__hkFallbackObserver?.disconnect();
			return target.__hkFallbackEvents ?? [];
		});
		expect(
			fallbackEvents,
			"soft navigation exposed a loading/fallback surface",
		).toEqual([]);
	});

	test("keeps the previous seller tab intact while the next tab is delayed", async ({
		page,
		request,
	}) => {
		test.setTimeout(35_000);
		const { sellerId } = await sellerFixtureForRoute(
			request,
			"/sellers/:id/posts",
		);
		test.skip(
			!sellerId,
			"the current public dataset has no seller fixture for tab-navigation coverage",
		);
		if (!sellerId) return;
		await prepareGuestSession(page);

		let releaseDestination = () => {};
		let destinationRequestSeen = false;
		const destinationGate = new Promise<void>((resolve) => {
			releaseDestination = resolve;
		});
		await page.route(`**/sellers/${sellerId}/posts**`, async (route) => {
			const targetRequest = route.request();
			const url = new URL(targetRequest.url());
			const isRoutePayload =
				targetRequest.method() === "GET" &&
				(targetRequest.headers().rsc === "1" || url.searchParams.has("_rsc"));
			if (!isRoutePayload) {
				await route.continue();
				return;
			}
			destinationRequestSeen = true;
			await destinationGate;
			await route.continue();
		});

		const sourceRoute = `/sellers/${sellerId}`;
		await gotoStreamed(page, sourceRoute, {
			timeout: 25_000,
		});
		await expect(page.locator(".initial-loader")).toBeHidden({
			timeout: 12_000,
		});
		await expect(
			page.locator('[data-auth-status]:not([data-auth-status="unknown"])'),
		).toBeAttached({
			timeout: 12_000,
		});
		await assertInitialLoaderHandoff(page, sourceRoute);
		await assertNoResidualLoading(page, sourceRoute);
		const sellerFrame = page.locator(".seller-tab-frame");
		await expect(sellerFrame).toBeVisible();
		await page.waitForTimeout(500);
		const sourceText = await sellerFrame.innerText();
		const sourceHeight = await page.evaluate(
			() => document.documentElement.scrollHeight,
		);
		await watchForFallbackReappearance(page);

		const postsTab = page.getByRole("tab", { name: "貼文" });
		const click = postsTab.click();
		try {
			await expect
				.poll(() => destinationRequestSeen, { timeout: 5_000 })
				.toBe(true);
			await page.waitForTimeout(500);
			expect(
				await sellerFrame.innerText(),
				"seller tab content changed while its route payload was still blocked",
			).toBe(sourceText);
			const pendingHeight = await page.evaluate(
				() => document.documentElement.scrollHeight,
			);
			expect(
				Math.abs(pendingHeight - sourceHeight),
				"seller tab navigation changed page height before its data was ready",
			).toBeLessThanOrEqual(1);
			await assertNoResidualLoading(
				page,
				`delayed ${sourceRoute} → ${sourceRoute}/posts`,
			);
		} finally {
			releaseDestination();
		}
		await click;

		await expect(page).toHaveURL(new RegExp(`/sellers/${sellerId}/posts$`));
		await expect(page.locator(".seller-posts-board")).toBeVisible();
		await assertNoResidualLoading(page, `${sourceRoute}/posts`);
		const fallbackEvents = await page.evaluate(() => {
			const target = window as StabilityWindow;
			target.__hkFallbackObserver?.disconnect();
			return target.__hkFallbackEvents ?? [];
		});
		expect(
			fallbackEvents,
			"seller tab navigation exposed a loading/fallback surface",
		).toEqual([]);
	});
});
