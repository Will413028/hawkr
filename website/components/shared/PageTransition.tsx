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
const PRESERVED_SCROLL_LOCK_MS = 800;
const PRESERVED_SCROLL_MAX_WAIT_MS = 6000;
const SAME_PATH_LOADING_FALLBACK_MS = 700;

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
			return "rise";
	}
}

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
	const tabPopRef = useRef(false);
	const preservedScrollIntent = useRef<AbortController | null>(null);
	const scrollBehaviorRestoreTimer = useRef<number | null>(null);
	const savedRootScrollBehavior = useRef<string | null>(null);
	const savedBodyScrollBehavior = useRef<string | null>(null);
	const programmaticArrivalAnimation = useRef<Animation | null>(null);

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
			if (value) {
				endNavPendingRef.current ??= beginNavPending();
			} else {
				endNavPendingRef.current?.(options);
				endNavPendingRef.current = null;
			}
			queueMicrotask(() => {
				setIsLoadingState(value);
			});
		},
		[],
	);

	const tabNavEndRef = useRef<((options?: { abort?: boolean }) => void) | null>(
		null,
	);
	const tabNavFuseTimer = useRef<number | null>(null);

	const beginTabNavPending = useCallback(() => {
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
				stampViewTransitionType("none");
				return;
			}
			const navigationUrl = getAnchorNavigationUrl(event);
			if (navigationUrl) {
				if (
					isStandaloneFlowTransition(
						window.location.pathname,
						navigationUrl.pathname,
					)
				) {
					stampViewTransitionType("none");
					return;
				}
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
			endTabNavPending();
			const standaloneTransition = isStandaloneFlowTransition(
				prevPathname.current,
				pathname,
			);
			const currentShell = getPersistentShell(pathname);
			const previousShell = getPersistentShell(prevPathname.current);
			const sameShell = Boolean(
				currentShell && previousShell && currentShell === previousShell,
			);
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
				preservedScrollY.current = null;
				stopPreservedScrollLock();
				setDisplayChildren(children);
				finishLoading();
				if (!viaTabPop) {
					const savedY = tabScrollMemory.get(tabKeyFor(pathname) ?? "") ?? 0;
					if (savedY > 0) {
						preservedScrollY.current = savedY;
						scrollToNow(savedY);
						startPreservedScrollLock();
					} else {
						scrollToNow();
					}
				}
			} else if (sameTabHop) {
				setDisplayChildren(children);
				finishLoading();
				if (preservedScrollY.current !== null) {
					stopPreservedScrollLock();
					startPreservedScrollLock();
				}
			} else {
				doTransition();
			}
		} else {
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
