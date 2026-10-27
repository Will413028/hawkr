"use client";

import {
	useCallback,
	useEffect,
	useLayoutEffect,
	useRef,
	useState,
} from "react";
import { usePathname } from "@/lib/intl/navigation";
import { stripLocale } from "@/lib/intl/paths";
import { beginNavPending } from "@/lib/loading/nav-pending";
import {
	isScrollManagedHistoryState,
	isScrollManagedSamePathHistoryState,
	scheduleLocationChangeEvent,
} from "@/lib/navigation/location-change";
import { consumeProgrammaticRouteMotion } from "@/lib/navigation/programmatic-route-motion";
import {
	getPersistentShell,
	hasSamePersistentShell,
	isProductDetailPath,
	isSameTabHop,
	isTabToTabNav,
	tabKeyFor,
} from "@/lib/navigation/route-transition-policy";
import { isStandaloneFlowTransition } from "@/lib/navigation/standalone-flow";
import {
	isQuickViewOpener,
	setQuickViewOrigin,
} from "@/lib/product/quickViewOrigin";
import type { RouteLoadingVariant } from "./RouteLoadingSkeleton";

const MAX_LOADING_VISIBLE_MS = 4000;
/* 釘捲動的「落定期」：目標高度可達之後再釘這麼久，吃掉 Next 的延遲
 * 捲頂與內容 streaming 的高度抖動。注意不是從啟動起算 —— 內容還沒長到
 * 目標高度前（skeleton、/market redirect hop 未落地）計時不開始，否則
 * hop 一慢（dev ~1s+、慢網路）鎖先死、捲頂後到，記憶就被蓋成 0。 */
const PRESERVED_SCROLL_LOCK_MS = 800;
/* 等待目標高度可達的絕對上限（對齊 MAX_LOADING_VISIBLE_MS）：列表真的
 * 變短（目標永遠不可達）時的兜底，不讓 rAF 迴圈無限跑。 */
const PRESERVED_SCROLL_MAX_WAIT_MS = 6000;
const SAME_PATH_LOADING_FALLBACK_MS = 700;

// 底部導覽的四個分頁像 App 的 Tab（IG 式）切換：內容瞬間換、不播轉場動畫、
// 不蓋 loading overlay，且每個 Tab 記住自己的捲動位置，切回時原地接續而不是
// 捲回頂部。
// 市集 Tab 直接連 /market/products；舊書籤仍可能從 /market 308 過來（見
// app/[locale]/(public)/market/page.tsx），所以兩個路徑仍視為同一個 Tab，legacy
// redirect hop 也走瞬切，避免多播一次進場動畫。
// 捲動記憶（tabScrollMemory）在離開 Tab 的當下存 scrollY、切回時釘回。
// feed / market 的「資料」由它們自己的 list snapshot 還原（InfiniteFeed /
// MarketProductsList，與本記憶在同一次點擊存下同一個 Y，兩者釘同一目標不打架）；
// 而 Next 保留 segment fiber 時 snapshot 不會重讀 —— 資料
// 原地都在，這裡的釘回就是唯一的捲動還原者，所以兩種情況都要釘。
const tabScrollMemory = new Map<string, number>();

function getRouteLoadingVariant(
	targetUrl: URL | null | undefined,
): RouteLoadingVariant {
	const pathname = stripLocale(targetUrl?.pathname ?? window.location.pathname);
	if (
		/^\/products\/[^/]+/.test(pathname) ||
		/^\/sellers\/[^/]+\/products\/[^/]+/.test(pathname)
	) {
		return "product-detail";
	}
	if (
		pathname === "/market/products" ||
		pathname === "/products" ||
		pathname === "/sellers" ||
		pathname.startsWith("/market/")
	) {
		return "market-list";
	}
	if (
		pathname === "/cart" ||
		pathname.startsWith("/checkout") ||
		pathname.startsWith("/orders") ||
		pathname.startsWith("/payment")
	) {
		return "checkout";
	}
	if (pathname.startsWith("/login") || pathname.startsWith("/register")) {
		return "auth";
	}
	if (pathname === "/search") return "search";
	if (/^\/sellers\/[^/]+/.test(pathname)) return "seller-storefront";
	if (
		pathname === "/feed" ||
		pathname.startsWith("/daily-posts") ||
		pathname.startsWith("/topics")
	) {
		return "feed";
	}
	if (
		pathname.startsWith("/account") ||
		pathname.startsWith("/seller-center")
	) {
		return "workspace";
	}
	return "generic";
}

type TransitionType = "fade" | "rise" | "zoom" | "slide" | "none";

// Map the already-resolved loading variant to a transition character.
// Derived purely from the URL, so server-rendered pages stay server
// components. The click/history handlers stamp the result on
// `<html data-vt>` right before navigation; the actual old→new animation is
// the browser View Transition triggered by components/shared/Link, styled in
// styles/components/page-transition.css.
function getTransitionType(variant: RouteLoadingVariant): TransitionType {
	switch (variant) {
		case "product-detail":
			return "zoom";
		case "checkout":
			return "slide";
		case "auth":
		case "workspace":
			return "fade";
		default:
			// market-list | feed | search | seller-storefront | generic
			return "rise";
	}
}

// 在導航「當下」把轉場型態蓋到 <html data-vt>，讓 View Transition 的
// ::view-transition-* CSS 選到正確動畫；"none" = 瞬切（Tab / persistent
// shell / 商品快速檢視 modal）。值會留到下一次導航覆寫，無需清除。
function stampViewTransitionType(type: TransitionType) {
	document.documentElement.dataset.vt = type;
}

function clearTimer(timerRef: { current: number | null }) {
	if (timerRef.current !== null) {
		window.clearTimeout(timerRef.current);
		timerRef.current = null;
	}
}

function scrollToNow(top = 0) {
	const root = document.documentElement;
	const body = document.body;
	const previousRootBehavior = root.style.scrollBehavior;
	const previousBodyBehavior = body.style.scrollBehavior;
	root.style.scrollBehavior = "auto";
	body.style.scrollBehavior = "auto";
	window.scrollTo(0, top);
	root.style.scrollBehavior = previousRootBehavior;
	body.style.scrollBehavior = previousBodyBehavior;
	// 告知 auto-hide navbar 這是程式性捲動（捲頂 / Tab 捲動記憶釘回），
	// 不是使用者手勢 —— 釘捲動要等內容長高，可能晚於路由 settle 視窗，
	// 靠時間窗猜不準，直接標記來源（見 useAutoHideNav）。
	window.dispatchEvent(new Event("hk:programmatic-scroll"));
}

function isCurrentTabContentReady(): boolean {
	const pathname = stripLocale(window.location.pathname);
	const selector =
		pathname === "/feed"
			? '[data-hk-tab-content-ready="feed"]'
			: pathname === "/market" || pathname === "/market/products"
				? '[data-hk-tab-content-ready="market"]'
				: null;
	if (!selector) return true;

	return Array.from(document.querySelectorAll<HTMLElement>(selector)).some(
		(element) =>
			element.getClientRects().length > 0 && !element.closest("[hidden]"),
	);
}

function getSameOriginNavigationUrl(value: string | URL | null | undefined) {
	if (!value) return null;

	try {
		const url = new URL(value, window.location.href);
		const currentUrl = new URL(window.location.href);

		if (!["http:", "https:"].includes(url.protocol)) return null;
		if (url.origin !== currentUrl.origin) return null;
		if (
			url.pathname === currentUrl.pathname &&
			url.search === currentUrl.search
		) {
			return null;
		}

		return url;
	} catch {
		return null;
	}
}

function getAnchorNavigationUrl(event: MouseEvent) {
	if (
		event.defaultPrevented ||
		event.button !== 0 ||
		event.altKey ||
		event.ctrlKey ||
		event.metaKey ||
		event.shiftKey
	) {
		return null;
	}

	if (!(event.target instanceof Element)) return null;

	const anchor = event.target.closest<HTMLAnchorElement>("a[href]");
	if (!anchor) return null;
	if (anchor.target && anchor.target !== "_self") return null;
	if (anchor.hasAttribute("download")) return null;
	if (anchor.getAttribute("aria-disabled") === "true") return null;

	const href = anchor.getAttribute("href");
	if (!href || href.startsWith("#")) return null;

	return getSameOriginNavigationUrl(href);
}

export default function PageTransition({
	children,
	controllerOnly = false,
}: {
	children?: React.ReactNode;
	controllerOnly?: boolean;
}) {
	const pathname = usePathname() ?? "";
	const [displayChildren, setDisplayChildren] = useState(children);
	const [isLoading, setIsLoadingState] = useState(false);
	const prevPathname = useRef(pathname);
	const containerRef = useRef<HTMLDivElement>(null);
	const loadingRef = useRef(false);
	const finishLoadingTimer = useRef<number | null>(null);
	const maxLoadingTimer = useRef<number | null>(null);
	const preservedScrollY = useRef<number | null>(null);
	const preservedScrollFrame = useRef<number | null>(null);
	// Tab 間 popstate（瀏覽器返回/前進）時捲動還原交給 Next，本元件不搶。
	const tabPopRef = useRef(false);
	const preservedScrollIntent = useRef<AbortController | null>(null);
	const scrollBehaviorRestoreTimer = useRef<number | null>(null);
	const savedRootScrollBehavior = useRef<string | null>(null);
	const savedBodyScrollBehavior = useRef<string | null>(null);
	const programmaticArrivalAnimation = useRef<Animation | null>(null);

	// Direct router.push/replace calls keep the source page interactive while
	// the RSC payload is in flight. Once the new pathname commits, soften only
	// that landing frame. Opacity is intentional: transforming the root wrapper
	// would change the containing block for fixed/sticky descendants.
	useLayoutEffect(() => {
		programmaticArrivalAnimation.current?.cancel();
		programmaticArrivalAnimation.current = null;
		if (!consumeProgrammaticRouteMotion(pathname)) return;
		if (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches) {
			return;
		}

		const routeSurface = document.querySelector<HTMLElement>(
			"main.page-content > .page-transition",
		);
		if (!routeSurface || typeof routeSurface.animate !== "function") return;

		const easing =
			getComputedStyle(document.documentElement)
				.getPropertyValue("--ease-emphasized")
				.trim() || "cubic-bezier(0.16, 1, 0.3, 1)";
		const animation = routeSurface.animate(
			[{ opacity: 0.92 }, { opacity: 1 }],
			{
				duration: 200,
				easing,
				fill: "none",
			},
		);
		animation.id = "hk-programmatic-route-arrival";
		programmaticArrivalAnimation.current = animation;
		animation.onfinish = () => {
			if (programmaticArrivalAnimation.current === animation) {
				programmaticArrivalAnimation.current = null;
			}
		};
		animation.oncancel = animation.onfinish;
	}, [pathname]);

	const endNavPendingRef = useRef<
		((options?: { abort?: boolean }) => void) | null
	>(null);

	const setLoading = useCallback(
		(value: boolean, options?: { abort?: boolean }) => {
			loadingRef.current = value;
			// Single choke point for the loading flag — mirror it onto the global
			// html[data-nav-pending] signal that drives the top progress rail
			// (lib/loading/nav-pending.ts). Attribute writes are synchronous and
			// safe inside the insertion-effect callstack (unlike the state write
			// below), so the rail's reveal-delay clock starts at the real click.
			// `abort` (the MAX_LOADING_VISIBLE_MS fuse) fades the rail out instead
			// of playing the completion sweep — the navigation hasn't landed, so a
			// "done" here would be a visible lie followed by a signal-less commit.
			if (value) {
				endNavPendingRef.current ??= beginNavPending();
			} else {
				endNavPendingRef.current?.(options);
				endNavPendingRef.current = null;
			}
			// Defer the React state write to a microtask. `startLoading()` is
			// reachable from our patched `history.pushState/replaceState`, and
			// upstream libraries (Next.js router, framer-motion, css-in-js
			// runtimes) call those inside their own `useInsertionEffect` —
			// where React forbids scheduling state updates. The microtask
			// drops us out of the insertion-effect callstack before the
			// dispatch hits the fiber tree. `loadingRef` stays synchronous so
			// the dedupe check in `startLoading()` still sees the latest state.
			queueMicrotask(() => {
				setIsLoadingState(value);
			});
		},
		[],
	);

	// Tab 間「返回/前進」（traverse）的進度軌 hold：popstate 當下 location
	// 已是新 URL，nav-pending 的 router-transition hold（靠 location 輪詢判
	// commit）蓋不到，改由這裡發起、pathname commit（React 訊號）解除。
	// push/replace 類導航（含程式化 router.push）一律由
	// onRouterTransitionStart → beginRouterTransitionPending 負責，不在此。
	// 4s 保險絲對齊 MAX_LOADING_VISIBLE_MS：導航被 Next 放棄（快速反向
	// back/forward）時 fade 出而非謊報完成。refcount 讓與其他 hold 的重疊
	// 自然收斂。
	const tabNavEndRef = useRef<((options?: { abort?: boolean }) => void) | null>(
		null,
	);
	const tabNavFuseTimer = useRef<number | null>(null);

	const beginTabNavPending = useCallback(() => {
		// supersede（前一導航未 commit 又發起新的）沿用 hold，但保險絲必須
		// 重掛——繼承殘餘額度會讓第二段導航在飛行中被 abort fade。
		clearTimer(tabNavFuseTimer);
		tabNavFuseTimer.current = window.setTimeout(() => {
			tabNavFuseTimer.current = null;
			tabNavEndRef.current?.({ abort: true });
			tabNavEndRef.current = null;
		}, MAX_LOADING_VISIBLE_MS);
		if (tabNavEndRef.current) return;
		tabNavEndRef.current = beginNavPending();
	}, []);

	const endTabNavPending = useCallback(() => {
		clearTimer(tabNavFuseTimer);
		tabNavEndRef.current?.();
		tabNavEndRef.current = null;
	}, []);

	// The controllerOnly instance lives in the root layout and never unmounts
	// in practice; the cleanup is correctness insurance for tests and any
	// future non-root mount so an in-flight wait can't leak the rail.
	useEffect(() => {
		return () => {
			programmaticArrivalAnimation.current?.cancel();
			programmaticArrivalAnimation.current = null;
			endNavPendingRef.current?.({ abort: true });
			endNavPendingRef.current = null;
			clearTimer(tabNavFuseTimer);
			tabNavEndRef.current?.({ abort: true });
			tabNavEndRef.current = null;
		};
	}, []);

	const restorePreservedScroll = useCallback(() => {
		if (preservedScrollY.current === null) return;
		scrollToNow(preservedScrollY.current);
	}, []);

	const restoreAutoScrollBehavior = useCallback(() => {
		clearTimer(scrollBehaviorRestoreTimer);
		if (savedRootScrollBehavior.current !== null) {
			document.documentElement.style.scrollBehavior =
				savedRootScrollBehavior.current;
			savedRootScrollBehavior.current = null;
		}
		if (savedBodyScrollBehavior.current !== null) {
			document.body.style.scrollBehavior = savedBodyScrollBehavior.current;
			savedBodyScrollBehavior.current = null;
		}
	}, []);

	const holdAutoScrollBehavior = useCallback(() => {
		const root = document.documentElement;
		const body = document.body;
		if (savedRootScrollBehavior.current === null) {
			savedRootScrollBehavior.current = root.style.scrollBehavior;
		}
		if (savedBodyScrollBehavior.current === null) {
			savedBodyScrollBehavior.current = body.style.scrollBehavior;
		}
		root.style.scrollBehavior = "auto";
		body.style.scrollBehavior = "auto";
		clearTimer(scrollBehaviorRestoreTimer);
		scrollBehaviorRestoreTimer.current = window.setTimeout(
			restoreAutoScrollBehavior,
			1400,
		);
	}, [restoreAutoScrollBehavior]);

	const stopPreservedScrollLock = useCallback(() => {
		preservedScrollIntent.current?.abort();
		preservedScrollIntent.current = null;
		if (preservedScrollFrame.current !== null) {
			window.cancelAnimationFrame(preservedScrollFrame.current);
			preservedScrollFrame.current = null;
		}
	}, []);

	const startPreservedScrollLock = useCallback(() => {
		if (preservedScrollFrame.current !== null) return;

		// The pin exists to keep the list steady while filtered results stream
		// in — never to fight the user. Any real scroll intent cancels both the
		// pin and the pending restore, so the page follows the wheel instantly.
		const intent = new AbortController();
		preservedScrollIntent.current = intent;
		const yieldToUser = () => {
			preservedScrollY.current = null;
			stopPreservedScrollLock();
		};
		window.addEventListener("wheel", yieldToUser, {
			passive: true,
			signal: intent.signal,
		});
		window.addEventListener("touchmove", yieldToUser, {
			passive: true,
			signal: intent.signal,
		});

		const startedAt = performance.now();
		// 落定計時從「目標高度可達」那一刻才開始：skeleton / redirect hop 期間
		// 內容不夠高，scrollTo 會被 clamp，這段等待不能吃掉鎖的壽命。內容
		// streaming 中高度縮回去（又不可達）就重置，等再次到位重新落定。
		let reachableSince: number | null = null;
		let lastScrollHeight = document.documentElement.scrollHeight;
		const tick = () => {
			const y = preservedScrollY.current;
			const now = performance.now();
			if (y === null || now - startedAt > PRESERVED_SCROLL_MAX_WAIT_MS) {
				preservedScrollFrame.current = null;
				preservedScrollIntent.current?.abort();
				preservedScrollIntent.current = null;
				return;
			}
			const scrollHeight = document.documentElement.scrollHeight;
			const contentReady = isCurrentTabContentReady();
			const heightChanged = Math.abs(scrollHeight - lastScrollHeight) > 1;
			lastScrollHeight = scrollHeight;
			const reachable = scrollHeight - window.innerHeight >= y - 1;
			if (!contentReady || !reachable || heightChanged) {
				reachableSince = null;
			} else if (reachableSince === null) {
				reachableSince = now;
			}
			if (
				reachableSince !== null &&
				now - reachableSince > PRESERVED_SCROLL_LOCK_MS
			) {
				preservedScrollFrame.current = null;
				preservedScrollIntent.current?.abort();
				preservedScrollIntent.current = null;
				return;
			}
			if (contentReady && Math.abs(window.scrollY - y) > 1) {
				scrollToNow(y);
			}
			preservedScrollFrame.current = window.requestAnimationFrame(tick);
		};
		preservedScrollFrame.current = window.requestAnimationFrame(tick);
	}, [stopPreservedScrollLock]);

	const startLoading = useCallback(
		(targetUrl: URL | null = null) => {
			clearTimer(finishLoadingTimer);
			holdAutoScrollBehavior();
			if (targetUrl) {
				if (targetUrl.pathname !== window.location.pathname) {
					preservedScrollY.current = null;
					stopPreservedScrollLock();
				} else if (targetUrl.search !== window.location.search) {
					preservedScrollY.current = window.scrollY;
					startPreservedScrollLock();
				}
			}

			if (!loadingRef.current) {
				setLoading(true);
			}

			clearTimer(maxLoadingTimer);
			maxLoadingTimer.current = window.setTimeout(() => {
				clearTimer(finishLoadingTimer);
				setLoading(false, { abort: true });
			}, MAX_LOADING_VISIBLE_MS);
		},
		[
			setLoading,
			startPreservedScrollLock,
			stopPreservedScrollLock,
			holdAutoScrollBehavior,
		],
	);

	const finishLoading = useCallback(() => {
		if (!loadingRef.current) return;

		clearTimer(finishLoadingTimer);
		clearTimer(maxLoadingTimer);
		const scrollYToRestore = preservedScrollY.current;
		if (scrollYToRestore !== null) {
			scrollToNow(scrollYToRestore);
			window.requestAnimationFrame(() => {
				scrollToNow(scrollYToRestore);
				preservedScrollY.current = null;
				stopPreservedScrollLock();
				restoreAutoScrollBehavior();
			});
		} else {
			window.requestAnimationFrame(restoreAutoScrollBehavior);
		}
		setLoading(false);
	}, [restoreAutoScrollBehavior, setLoading, stopPreservedScrollLock]);

	// Cross-page navigation: snap to top and swap immediately. The old→new
	// visual handoff is the browser View Transition triggered by
	// components/shared/Link（依 html[data-vt] 選動畫）—— wrapper 本身不再
	// 播動畫，所以頁內 fixed/sticky 元素不會被 transform 劫持，也不會與
	// View Transition 的截圖過渡疊成雙重動畫。
	const doTransition = useCallback(() => {
		scrollToNow();
		setDisplayChildren(children);
		finishLoading();
	}, [children, finishLoading]);

	useEffect(() => {
		const handleDocumentClick = (event: MouseEvent) => {
			const storefrontPreviewAnchor =
				event.target instanceof Element
					? event.target.closest<HTMLAnchorElement>(
							"a[data-storefront-product-preview-trigger]",
						)
					: null;
			if (storefrontPreviewAnchor) {
				// Seller preview opens a local read-only drawer. This capture listener
				// runs before StorefrontPreviewBoundary, so opt out before the href can
				// be misclassified as a route-level product quick view.
				stampViewTransitionType("none");
				return;
			}
			const hardNavigationAnchor =
				event.target instanceof Element
					? event.target.closest<HTMLAnchorElement>(
							"a[data-hk-hard-navigation]",
						)
					: null;
			if (hardNavigationAnchor) {
				// This capture listener runs before the shared Link's React onClick.
				// The DOM marker keeps full-document auth/recovery boundaries out of
				// the soft-nav loading and View Transition state machine.
				stampViewTransitionType("none");
				return;
			}
			const navigationUrl = getAnchorNavigationUrl(event);
			if (navigationUrl) {
				// Full-chrome flows must not cross-fade against the previous shell.
				// Keep the current page intact until the new route commits, with no
				// route skeleton layered between the two screens.
				if (
					isStandaloneFlowTransition(
						window.location.pathname,
						navigationUrl.pathname,
					)
				) {
					stampViewTransitionType("none");
					return;
				}
				// Tab 間切換：記下目前 Tab 的捲動位置後放行 —— 不啟動 overlay、
				// 不播 View Transition，內容換頁交給 pathname effect 的瞬切分支；
				// commit 前的等待由 onRouterTransitionStart 的進度軌 hold 補回饋
				// （lib/loading/nav-pending.ts，導航開始即發起、涵蓋程式化 push）。
				if (isTabToTabNav(window.location.pathname, navigationUrl.pathname)) {
					stampViewTransitionType("none");
					const sourceTab = tabKeyFor(window.location.pathname);
					if (sourceTab) tabScrollMemory.set(sourceTab, window.scrollY);
					return;
				}
				if (isSameTabHop(window.location.pathname, navigationUrl.pathname)) {
					stampViewTransitionType("none");
					return;
				}
				if (
					hasSamePersistentShell(
						window.location.pathname,
						navigationUrl.pathname,
					)
				) {
					stampViewTransitionType("none");
					return;
				}
				// Opening a product opens the quick-view modal (intercepting route);
				// remember the tapped card's viewport centre so the modal scales out
				// of it. The market list / page underneath is never unmounted, and the
				// modal owns its own zoom — no View Transition on top of it.
				if (isProductDetailPath(navigationUrl.pathname)) {
					stampViewTransitionType("none");
					const clickTarget =
						event.target instanceof Element ? event.target : null;
					const productCard = clickTarget?.closest<HTMLElement>(
						".market-product-card",
					);
					const originElement =
						clickTarget?.closest<HTMLElement>("[data-zoom-card]") ??
						productCard?.querySelector<HTMLElement>("[data-zoom-card]") ??
						clickTarget?.closest<HTMLAnchorElement>("a[href]") ??
						null;
					const openerUrl = `${window.location.pathname}${window.location.search}`;
					if (originElement) {
						const r = originElement.getBoundingClientRect();
						setQuickViewOrigin({
							cx: r.left + r.width / 2,
							cy: r.top + r.height / 2,
							scrollY: window.scrollY,
							openerUrl,
						});
					} else {
						setQuickViewOrigin({
							cx: window.innerWidth / 2,
							cy: window.innerHeight / 2,
							scrollY: window.scrollY,
							openerUrl,
						});
					}
				} else if (
					/^\/sellers\/[^/]+/.test(stripLocale(navigationUrl.pathname))
				) {
					// Storefront opens are dynamic RSC routes whose latency tracks the
					// backend (cold cache = full RTT chain). An animated View Transition
					// would freeze rendering AND input from click to commit — on a slow
					// storefront that reads as a dead click. Instant-commit instead: the
					// old page stays live and the nav progress rail owns the feedback.
					// PDPs under /sellers/:id/products/:pid were already caught by the
					// quick-view branch above.
					stampViewTransitionType("none");
				} else {
					stampViewTransitionType(
						getTransitionType(getRouteLoadingVariant(navigationUrl)),
					);
				}
				startLoading(navigationUrl);
			}
		};

		const originalPushState = window.history.pushState;
		const originalReplaceState = window.history.replaceState;

		const handleHistoryUrl = (url: string | URL | null | undefined) => {
			const navigationUrl = getSameOriginNavigationUrl(url);
			if (!navigationUrl) return false;
			if (
				isStandaloneFlowTransition(
					window.location.pathname,
					navigationUrl.pathname,
				)
			) {
				stampViewTransitionType("none");
				return false;
			}
			// 注意：patched pushState 在 route COMMIT 時才被 Next 呼叫
			// （HistoryUpdater 的 insertion effect），不是導航開始——在這裡發起
			// 進度軌 hold 蓋不到等待窗；hold 一律由 onRouterTransitionStart 起。
			if (isTabToTabNav(window.location.pathname, navigationUrl.pathname)) {
				stampViewTransitionType("none");
				const sourceTab = tabKeyFor(window.location.pathname);
				if (sourceTab) tabScrollMemory.set(sourceTab, window.scrollY);
				return false;
			}
			if (isSameTabHop(window.location.pathname, navigationUrl.pathname)) {
				stampViewTransitionType("none");
				return false;
			}
			if (
				hasSamePersistentShell(window.location.pathname, navigationUrl.pathname)
			) {
				stampViewTransitionType("none");
				return false;
			}

			startLoading(navigationUrl);
			return true;
		};

		const patchedPushState: History["pushState"] = function patchedPushState(
			this: History,
			...args: Parameters<History["pushState"]>
		) {
			const previousPathname = window.location.pathname;
			const navigationUrl = getSameOriginNavigationUrl(args[2]);
			const scrollManaged = Boolean(
				navigationUrl &&
					isScrollManagedHistoryState(args[0], navigationUrl.pathname),
			);
			if (!scrollManaged) handleHistoryUrl(args[2]);
			const result = originalPushState.apply(this, args);
			// URL is committed here (Next calls pushState at route commit). Notify
			// client hooks (useUrlSearchParams) so ISR pages can react to filter /
			// deep-link params without useSearchParams — which would bail the route
			// out of static prerendering and drop its SEO HTML body.
			scheduleLocationChangeEvent(window);
			if (!scrollManaged && window.location.pathname === previousPathname) {
				window.setTimeout(restorePreservedScroll, 0);
				window.requestAnimationFrame(restorePreservedScroll);
				clearTimer(finishLoadingTimer);
				finishLoadingTimer.current = window.setTimeout(() => {
					restorePreservedScroll();
					finishLoading();
				}, SAME_PATH_LOADING_FALLBACK_MS);
			}
			return result;
		};

		const patchedReplaceState: History["replaceState"] =
			function patchedReplaceState(
				this: History,
				...args: Parameters<History["replaceState"]>
			) {
				const previousPathname = window.location.pathname;
				const navigationUrl = getSameOriginNavigationUrl(args[2]);
				const scrollManaged = Boolean(
					navigationUrl &&
						isScrollManagedHistoryState(args[0], navigationUrl.pathname),
				);
				if (!scrollManaged) handleHistoryUrl(args[2]);
				const result = originalReplaceState.apply(this, args);
				// See patchedPushState: keep ISR pages reactive to URL changes
				// without useSearchParams (SEO-safe).
				scheduleLocationChangeEvent(window);
				if (!scrollManaged && window.location.pathname === previousPathname) {
					window.setTimeout(restorePreservedScroll, 0);
					window.requestAnimationFrame(restorePreservedScroll);
					clearTimer(finishLoadingTimer);
					finishLoadingTimer.current = window.setTimeout(() => {
						restorePreservedScroll();
						finishLoading();
					}, SAME_PATH_LOADING_FALLBACK_MS);
				}
				return result;
			};

		const handlePopState = (event: PopStateEvent) => {
			const previousPathname = prevPathname.current;
			if (
				isScrollManagedSamePathHistoryState(
					event.state,
					previousPathname,
					window.location.pathname,
				)
			) {
				stampViewTransitionType("none");
				preservedScrollY.current = null;
				stopPreservedScrollLock();
				finishLoading();
				return;
			}
			if (
				isStandaloneFlowTransition(previousPathname, window.location.pathname)
			) {
				stampViewTransitionType("none");
				finishLoading();
				return;
			}
			// Tab 間的返回/前進：DOM 還沒換頁，先記下要離開的 Tab 的捲動位置；
			// 到達頁的捲動還原交給瀏覽器/Next 的 popstate 恢復，不蓋 overlay。
			// popstate 的 RSC 重取同樣有 commit 前死窗，掛同一個進度軌 hold。
			if (isTabToTabNav(previousPathname, window.location.pathname)) {
				const sourceTab = tabKeyFor(previousPathname);
				if (sourceTab) tabScrollMemory.set(sourceTab, window.scrollY);
				tabPopRef.current = true;
				beginTabNavPending();
				return;
			}
			startLoading(new URL(window.location.href));

			window.setTimeout(() => {
				if (window.location.pathname === previousPathname) {
					finishLoading();
				}
			}, 0);
		};

		document.addEventListener("click", handleDocumentClick, true);
		window.addEventListener("popstate", handlePopState);
		window.history.pushState = patchedPushState;
		window.history.replaceState = patchedReplaceState;

		return () => {
			document.removeEventListener("click", handleDocumentClick, true);
			window.removeEventListener("popstate", handlePopState);
			window.history.pushState = originalPushState;
			window.history.replaceState = originalReplaceState;
		};
	}, [
		beginTabNavPending,
		finishLoading,
		restorePreservedScroll,
		startLoading,
		stopPreservedScrollLock,
	]);

	useEffect(() => {
		if (pathname !== prevPathname.current) {
			// 任何 commit 都解除 tab 導航的進度軌 hold（使用者中途點了別的連結
			// 改道也算落地）；未 commit 的放棄案由 4s 保險絲 fade 收尾。
			endTabNavPending();
			const standaloneTransition = isStandaloneFlowTransition(
				prevPathname.current,
				pathname,
			);
			// Navigation within persistent shells keeps the layout/sidebar
			// mounted via Next.js layout persistence. Skip the wrapper cross-fade
			// so the chrome stays visually static and the segment loading.tsx owns
			// only the content column.
			const currentShell = getPersistentShell(pathname);
			const previousShell = getPersistentShell(prevPathname.current);
			const sameShell = Boolean(
				currentShell && previousShell && currentShell === previousShell,
			);
			// Opening/closing a product is an intercepting-route modal layered over a
			// still-mounted page (parallel @modal slot). The page underneath must NOT
			// move — so swap instantly with NO scroll-to-top and no wrapper animation;
			// the modal owns its own open/close animation.
			//
			// Leaving a product for anything OTHER than the page it was opened over is
			// not a close, it is an ordinary forward navigation (店家頁、購物車…) — and
			// a full-page PDP (/sellers/:id/products/:pid) has no modal to restore
			// anything at all. Those must land at the top of the new route like every
			// other link, so they fall through to the normal transition below.
			const enteringProduct = isProductDetailPath(pathname);
			const productModal =
				enteringProduct ||
				(isProductDetailPath(prevPathname.current ?? "") &&
					isQuickViewOpener(window.location.href));
			const tabSwap = isTabToTabNav(prevPathname.current, pathname);
			const sameTabHop = isSameTabHop(prevPathname.current, pathname);
			const viaTabPop = tabPopRef.current;
			tabPopRef.current = false;

			prevPathname.current = pathname;
			if (!standaloneTransition && !sameShell && !tabSwap && !sameTabHop) {
				startLoading(new URL(window.location.href));
			}
			if (standaloneTransition) {
				setDisplayChildren(children);
				finishLoading();
			} else if (productModal) {
				setDisplayChildren(children);
				finishLoading();
			} else if (sameShell) {
				setDisplayChildren(children);
				finishLoading();
			} else if (tabSwap) {
				// App-Tab 式瞬切：內容立即換、不播動畫、不捲回頂部。
				// 先清掉可能殘留的 same-path 保留捲動，避免 finishLoading 誤還原。
				preservedScrollY.current = null;
				stopPreservedScrollLock();
				setDisplayChildren(children);
				finishLoading();
				if (!viaTabPop) {
					const savedY = tabScrollMemory.get(tabKeyFor(pathname) ?? "") ?? 0;
					if (savedY > 0) {
						// 內容還在掛載時 scrollTo 會被 clamp；沿用 same-path 篩選的
						// 釘住機制（800ms 內反覆釘住、使用者一捲動就讓位）。經過
						// /market redirect hop 時，這個鎖會跨兩段 commit 繼續釘，
						// 直到列表內容到位、目標高度可達為止。
						preservedScrollY.current = savedY;
						scrollToNow(savedY);
						startPreservedScrollLock();
					} else {
						// 第一次造訪該 Tab（無記憶）：立即歸零當基準，否則會殘留
						// 上一個 Tab 的捲動、再被 Next 的延遲捲頂跳一次。
						scrollToNow();
					}
				}
			} else if (sameTabHop) {
				// 同 Tab 內的 redirect hop（/market → /market/products）：瞬切、
				// 不主動動捲動 —— 基準歸零已在第一段 Tab 切換做過，這裡再動
				// 會蓋掉列表 snapshot 的還原。
				setDisplayChildren(children);
				finishLoading();
				// 捲動記憶還在釘（使用者沒讓位）就在 hop 落地的 commit 重新
				// 上鎖：hop 的網路/編譯延遲沒有上限（dev 可 4s+），第一段
				// Tab 切換啟動的鎖可能已到壽命；Next 對新 segment 的延遲捲頂
				// 發生在這個 commit 之後，鎖必須從這裡重新起算才擋得住。
				if (preservedScrollY.current !== null) {
					stopPreservedScrollLock();
					startPreservedScrollLock();
				}
			} else {
				doTransition();
			}
		} else {
			// Same path or initial render — just show content
			setDisplayChildren(children);
			finishLoading();
		}
	}, [
		pathname,
		children,
		doTransition,
		endTabNavPending,
		finishLoading,
		startLoading,
		startPreservedScrollLock,
		stopPreservedScrollLock,
	]);

	useEffect(() => {
		return () => {
			clearTimer(finishLoadingTimer);
			clearTimer(maxLoadingTimer);
			clearTimer(scrollBehaviorRestoreTimer);
			stopPreservedScrollLock();
			restoreAutoScrollBehavior();
		};
	}, [restoreAutoScrollBehavior, stopPreservedScrollLock]);

	// While a product quick-view is open, render LIVE children (not the snapshot)
	// so the intercepting `@modal` slot flows straight through — including its
	// reset to `default` (null) on close. Snapshotting it would leave the modal
	// stuck on screen after back, since the snapshot only updates via an effect.
	const onProductPath = isProductDetailPath(pathname);

	return (
		<>
			{controllerOnly ? null : (
				<div
					aria-busy={isLoading}
					className="page-transition"
					ref={containerRef}
				>
					{onProductPath ? children : displayChildren}
				</div>
			)}
		</>
	);
}
