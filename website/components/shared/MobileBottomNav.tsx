"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { usePrefersReducedMotion } from "@/lib/a11y/reduced-motion";
import { useAuth } from "@/lib/auth/context";
import { useAutoHideNav } from "@/lib/hooks/useAutoHideNav";
import { usePathname } from "@/lib/intl/navigation";
import { isNavPathActive } from "@/lib/navigation/active-link";
import { isDelegatedWorkspacePath } from "@/lib/navigation/delegated-workspace";
import {
	cancelMainNavPointerIntent,
	prefetchMainNavOnIntent,
	scheduleMainNavIdleWarm,
} from "@/lib/navigation/main-nav-prefetch";
import {
	getMainTabPendingServerSnapshot,
	getMainTabPendingSnapshot,
	subscribeMainTabPending,
} from "@/lib/navigation/main-tab-pending";
import { isAccountMessageThreadPath } from "@/lib/navigation/message-thread";
import { isStandaloneFlowPath } from "@/lib/navigation/standalone-flow";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { cn } from "@/lib/utils/cn";

type MobileNavIconName =
	| "home"
	| "feed"
	| "market"
	| "search"
	| "user"
	| "login";

const baseItems = [
	{ label: "首頁", href: "/", icon: "home" },
	{ label: "動態", href: "/feed", icon: "feed" },
	{ label: "市集", href: "/market/products", icon: "market" },
	{ label: "搜尋", href: "/search", icon: "search" },
] as const;

const MOBILE_NAV_GLYPH: Record<MobileNavIconName, AppIconName> = {
	home: "home",
	feed: "discovery",
	market: "bag",
	search: "search",
	user: "user",
	login: "login",
};

type Slot = { left: number; top: number; width: number; height: number };

// How far a pointer must travel before a press becomes a drag (rather than a
// tap). Below this, the native <Link> click handles navigation untouched.
const DRAG_THRESHOLD_PX = 6;

export default function MobileBottomNav() {
	const pathname = usePathname() ?? "";
	const pendingTab = React.useSyncExternalStore(
		subscribeMainTabPending,
		getMainTabPendingSnapshot,
		getMainTabPendingServerSnapshot,
	);
	const mainTabPending =
		pendingTab?.sourcePathname === pathname ? pendingTab : null;
	const router = useObservedRouter("mobile_bottom_nav");
	const { isLoggedIn, activeRole } = useAuth();
	const reduceMotion = usePrefersReducedMotion();
	const [mounted, setMounted] = React.useState(false);

	const prefetchMainTab = React.useCallback(
		(href: string, intent: "pointer" | "focus" | "touch") => {
			prefetchMainNavOnIntent({
				router,
				href,
				currentPathname: pathname,
				intent,
			});
		},
		[pathname, router],
	);

	React.useEffect(() => {
		setMounted(true);
	}, []);

	// This controller lives in the globally mounted bottom-nav shell rather than
	// the buyer desktop navbar, so sellers browsing a canonical public tab get
	// the same fast-network idle warm. The coordinator itself refuses private,
	// auth and transactional routes.
	React.useEffect(
		() =>
			scheduleMainNavIdleWarm({
				router,
				currentPathname: pathname,
			}),
		[pathname, router],
	);

	// Threads-style auto-hide: same direction-based scroll logic as the top
	// navbar (down → hide, up → reveal), tuned to match. The bar slides down
	// off the bottom edge instead of up. `revealEdge: "bottom"` adds the
	// summon affordances — cursor near the bottom edge, or a swipe-up from the
	// bottom — so a hidden bar can be called back without scrolling content.
	// The hook already no-ops under prefers-reduced-motion.
	const { hidden } = useAutoHideNav({
		topThreshold: 56,
		directionDelta: 4,
		minVisibleMs: 160,
		revealEdge: "bottom",
	});

	// Logged-in users get their role-aware settings destination. Guests get an
	// explicit login destination; calling the
	// tab 「設定」 before an account exists made the action's outcome ambiguous.
	// 依 activeRole 決定目的地：/account 是 buyer 專區、/seller-center 是
	// seller 專區，RoleZoneGuard 對不上角色會直接 replace("/") —— 賣家身分
	// 點 /account 的「設定」會看起來像「馬上跳回首頁」。桌面 Navbar 已依
	// 角色分流（商店設定 → /seller-center/me），這裡對齊同一規則。
	const items = React.useMemo(() => {
		const meItem = {
			label: isLoggedIn ? "設定" : "登入",
			href: isLoggedIn
				? activeRole === "seller"
					? "/seller-center/me"
					: "/account"
				: "/login?role=buyer",
			icon: (isLoggedIn ? "user" : "login") as MobileNavIconName,
		};
		return [...baseItems, meItem];
	}, [isLoggedIn, activeRole]);

	const activeIndex = React.useMemo(() => {
		if (!pathname) return 0;
		const idx = items.findIndex((item) => isNavPathActive(pathname, item.href));
		return idx === -1 ? 0 : idx;
	}, [pathname, items]);

	const navRef = React.useRef<HTMLElement | null>(null);
	const itemEls = React.useRef<(HTMLAnchorElement | null)[]>([]);

	// Measured geometry of each tab cell, so the sliding pill can sit exactly
	// over a cell regardless of padding / gap / safe-area width.
	const [slots, setSlots] = React.useState<Slot[]>([]);
	const measured = slots.length === items.length;

	const measure = React.useCallback(() => {
		const els = itemEls.current;
		if (els.length < items.length) return;
		const next: Slot[] = [];
		for (let i = 0; i < items.length; i += 1) {
			const el = els[i];
			if (!el) return;
			// The nav is `hidden` above 840px, and everything inside a
			// `display: none` subtree reports zero. Committing that produced a
			// full set of zero-sized cells — which passed the count-based
			// `measured` check below, so the pill rendered with `opacity: 1` and
			// `width: 0`: present in the DOM, invisible on screen, and never
			// re-measured because nothing had changed size. Refuse the reading
			// instead; the ResizeObserver fires again once the nav gains a box.
			if (el.offsetWidth === 0 || el.offsetHeight === 0) return;
			next.push({
				left: el.offsetLeft,
				top: el.offsetTop,
				width: el.offsetWidth,
				height: el.offsetHeight,
			});
		}
		setSlots(next);
	}, [items.length]);

	React.useEffect(() => {
		if (!mounted) return;
		measure();
		const nav = navRef.current;
		if (!nav || typeof ResizeObserver === "undefined") return;
		const ro = new ResizeObserver(() => measure());
		ro.observe(nav);
		return () => ro.disconnect();
	}, [measure, mounted]);

	// Drag state. Refs drive the imperative pointer handlers (no stale
	// closures); the paired state drives the spring + colour re-render.
	const pressedRef = React.useRef(false);
	const startXRef = React.useRef(0);
	const draggingRef = React.useRef(false);
	const hoverRef = React.useRef(activeIndex);
	const swallowClickRef = React.useRef(false);

	const [dragging, setDragging] = React.useState(false);
	const [hoverIndex, setHoverIndex] = React.useState(activeIndex);
	// After a drag/tap we optimistically point the pill at the chosen tab until
	// the route resolves, so it never springs back to the old tab first.
	const [pendingIndex, setPendingIndex] = React.useState<number | null>(null);
	const pendingNavigationRef = React.useRef<{
		sourcePathname: string;
		observedPending: boolean;
	} | null>(null);
	const clearPendingSelection = React.useCallback(() => {
		pendingNavigationRef.current = null;
		setPendingIndex(null);
	}, []);
	const selectPendingTarget = React.useCallback(
		(index: number) => {
			if (index === activeIndex) {
				clearPendingSelection();
				return;
			}
			pendingNavigationRef.current = {
				sourcePathname: window.location.pathname,
				observedPending:
					document.documentElement.getAttribute("data-nav-pending") === "",
			};
			setPendingIndex(index);
		},
		[activeIndex, clearPendingSelection],
	);

	// Once the route resolves (pathname changed), drop the optimistic target
	// so the pill follows the real active tab again.
	// biome-ignore lint/correctness/useExhaustiveDependencies: clear on route change
	React.useEffect(() => {
		clearPendingSelection();
	}, [pathname]);

	React.useEffect(() => {
		const root = document.documentElement;
		const observer = new MutationObserver((records) => {
			const pending = pendingNavigationRef.current;
			if (!pending) return;
			// Read the current phase, not an old mutation's value: a previous
			// abort and the latest navigation's begin can share one observer batch.
			if (root.getAttribute("data-nav-pending") === "") {
				pending.observedPending = true;
				return;
			}
			const completedPendingPhase =
				pending.observedPending ||
				records.some((record) => record.oldValue === "");
			if (
				completedPendingPhase &&
				window.location.pathname === pending.sourcePathname
			) {
				// A canceled/failed navigation has no pathname commit to clear the
				// optimistic pill. The existing progress lifecycle owns that result.
				clearPendingSelection();
			}
		});
		observer.observe(root, {
			attributes: true,
			attributeFilter: ["data-nav-pending"],
			attributeOldValue: true,
		});
		window.addEventListener("popstate", clearPendingSelection);
		window.addEventListener("pagehide", clearPendingSelection);
		return () => {
			observer.disconnect();
			window.removeEventListener("popstate", clearPendingSelection);
			window.removeEventListener("pagehide", clearPendingSelection);
		};
	}, [clearPendingSelection]);

	const loadingIndex = mainTabPending
		? items.findIndex((item) =>
				isNavPathActive(mainTabPending.targetPathname, item.href),
			)
		: -1;
	const targetIndex = dragging
		? hoverIndex
		: loadingIndex >= 0
			? loadingIndex
			: pendingIndex !== null
				? pendingIndex
				: activeIndex;
	const target = slots[targetIndex];

	const slotFromClientX = React.useCallback(
		(clientX: number) => {
			const nav = navRef.current;
			if (!nav || !slots.length) return activeIndex;
			const x = clientX - nav.getBoundingClientRect().left;
			let best = 0;
			let bestDist = Number.POSITIVE_INFINITY;
			for (let i = 0; i < slots.length; i += 1) {
				const centre = slots[i].left + slots[i].width / 2;
				const dist = Math.abs(x - centre);
				if (dist < bestDist) {
					bestDist = dist;
					best = i;
				}
			}
			return best;
		},
		[slots, activeIndex],
	);

	const setHover = React.useCallback(
		(idx: number, withFeedback: boolean) => {
			if (idx === hoverRef.current) return;
			hoverRef.current = idx;
			setHoverIndex(idx);
			if (withFeedback) {
				if (!reduceMotion && typeof navigator !== "undefined") {
					navigator.vibrate?.(8);
				}
			}
		},
		[reduceMotion],
	);

	const handlePointerDown = React.useCallback(
		(event: React.PointerEvent<HTMLElement>) => {
			if (event.button !== 0 && event.pointerType === "mouse") return;
			// A touch drag may emit no click at all. Its suppression must never
			// survive into this new gesture and swallow the user's next real tap.
			swallowClickRef.current = false;
			pressedRef.current = true;
			draggingRef.current = false;
			startXRef.current = event.clientX;
			hoverRef.current = activeIndex;
		},
		[activeIndex],
	);

	const handlePointerMove = React.useCallback(
		(event: React.PointerEvent<HTMLElement>) => {
			if (!pressedRef.current) return;
			if (!draggingRef.current) {
				if (Math.abs(event.clientX - startXRef.current) < DRAG_THRESHOLD_PX) {
					return;
				}
				draggingRef.current = true;
				setDragging(true);
				navRef.current?.setPointerCapture?.(event.pointerId);
			}
			event.preventDefault();
			setHover(slotFromClientX(event.clientX), true);
		},
		[setHover, slotFromClientX],
	);

	const endPress = React.useCallback(
		(event: React.PointerEvent<HTMLElement>) => {
			if (!pressedRef.current) return;
			pressedRef.current = false;
			if (!draggingRef.current) return;
			draggingRef.current = false;
			setDragging(false);
			navRef.current?.releasePointerCapture?.(event.pointerId);
			const idx = hoverRef.current;
			swallowClickRef.current = true;
			if (idx !== activeIndex || mainTabPending) {
				selectPendingTarget(idx);
				router.push(items[idx].href);
			}
		},
		[activeIndex, items, router, selectPendingTarget, mainTabPending],
	);

	const handlePointerCancel = React.useCallback(() => {
		swallowClickRef.current = false;
		pressedRef.current = false;
		if (draggingRef.current) {
			draggingRef.current = false;
			setDragging(false);
		}
	}, []);
	// Arrow keys walk the tab strip. The links already navigate on Enter/Space
	// (they are anchors), so the gesture that was missing is purely "move to the
	// neighbouring tab" — the drag interaction had no keyboard equivalent at all.
	const handleKeyDown = React.useCallback((event: React.KeyboardEvent) => {
		const { key } = event;
		if (
			key !== "ArrowLeft" &&
			key !== "ArrowRight" &&
			key !== "Home" &&
			key !== "End"
		) {
			return;
		}
		const links = itemEls.current.filter(Boolean) as HTMLAnchorElement[];
		if (links.length === 0) return;
		const current = links.indexOf(document.activeElement as HTMLAnchorElement);
		if (current < 0) return;
		event.preventDefault();
		let next: number;
		if (key === "Home") next = 0;
		else if (key === "End") next = links.length - 1;
		else if (key === "ArrowRight") next = (current + 1) % links.length;
		else next = (current - 1 + links.length) % links.length;
		links[next]?.focus();
	}, []);

	const handleClickCapture = React.useCallback(
		(event: React.MouseEvent<HTMLElement>) => {
			if (!swallowClickRef.current) return;
			swallowClickRef.current = false;
			// Capture also sees a release-click targeted at the nav itself. Keyboard
			// activation has no pointer gesture to suppress and must remain usable.
			if (event.detail !== 0) event.preventDefault();
		},
		[],
	);

	// Inside the seller workspace the sidebar shell handles navigation. Buyer
	// message threads also own their bottom edge with a persistent composer and
	// an in-thread back action, so mounting the global nav there would cover the
	// conversation and leave its reserved clearance behind.
	// On public pages a logged-in seller is still browsing the buyer storefront
	// and needs the same bottom nav as everyone else.
	// Framer Motion's reduced-motion preference and the measured tab geometry are
	// browser-only. On mobile production hydration they can settle before React
	// claims the server markup, producing recoverable #418 mismatches. The branded
	// initial loader already owns first paint, so mount this fixed navigation one
	// client commit later with deterministic browser state.
	if (
		!mounted ||
		!pathname ||
		pathname?.startsWith("/seller-center") ||
		isDelegatedWorkspacePath(pathname) ||
		isAccountMessageThreadPath(pathname) ||
		isStandaloneFlowPath(pathname)
	)
		return null;

	// Colour follows the finger while dragging (and the chosen tab while a tap
	// route resolves); aria-current stays on the real route for a11y.
	const highlightIndex = targetIndex;

	return (
		<nav
			ref={navRef}
			// Hidden on desktop; floating selector bar ≤840px (buyer phone storefront).
			// Its plain beige base and white frosted selection lens intentionally
			// match the avatar fit switch instead of introducing another material.
			// `touch-none select-none` lets a horizontal finger drag move the pill
			// without the page scrolling or text selecting. Auto-hide: when
			// [data-hidden], slide down past the bottom edge (clear the 10px gap +
			// safe-area) and fade; pointer-events drop so taps fall through.
			// Mirrors the top bar's 240ms easing; respects reduced-motion.
			// `view-transition-name`: 切頁時自成截圖 group（疊在 page-content 的
			// 新舊截圖之上、動畫全關，見 page-transition.css），bar 才不會被
			// 內容轉場蓋住而看起來在抖。
			className={cn(
				"hidden max-[840px]:grid fixed left-2.5 right-2.5 bottom-2.5 z-[200] grid-cols-5 gap-0.5 p-2 overflow-hidden isolate",
				"rounded-[26px] border border-hk-border-soft bg-hk-surface-muted",
				"[box-shadow:0_1px_0_rgba(255,255,255,0.72)_inset,0_8px_22px_-18px_rgba(0,0,0,0.28)]",
				"dark:[box-shadow:0_1px_0_rgba(255,255,255,0.1)_inset,0_8px_22px_-18px_rgba(0,0,0,0.64)]",
				"touch-none select-none will-change-transform [transform:translate3d(0,0,0)] [view-transition-name:mobile-bottom-nav]",
				"[transition:transform_240ms_cubic-bezier(0.22,1,0.36,1),opacity_240ms_cubic-bezier(0.22,1,0.36,1)]",
				"data-[hidden=true]:[transform:translate3d(0,calc(100%+1.25rem),0)] data-[hidden=true]:opacity-0 data-[hidden=true]:pointer-events-none",
				"motion-reduce:[transition:none]",
			)}
			data-hidden={hidden && !mainTabPending ? "true" : "false"}
			data-dragging={dragging ? "true" : "false"}
			aria-label="手機底部導覽"
			onKeyDown={handleKeyDown}
			onPointerDown={handlePointerDown}
			onPointerMove={handlePointerMove}
			onPointerUp={endPress}
			onPointerCancel={handlePointerCancel}
			onClickCapture={handleClickCapture}
		>
			{/* Sliding indicator pill — the "magnetic" background. It springs
			    between cells as the finger drags, and lifts slightly while held. */}
			{target ? (
				<span
					aria-hidden="true"
					data-mobile-nav-active-indicator="true"
					data-dragging={dragging ? "true" : "false"}
					className={cn(
						"absolute left-0 top-0 z-[1] rounded-[16px] pointer-events-none will-change-transform overflow-hidden",
						// Selected state is one plain white frosted layer. Luminance and
						// depth provide emphasis without a branded colour wash.
						"border border-[rgba(255,255,255,0.82)] bg-[rgba(255,255,255,0.72)]",
						"[backdrop-filter:blur(10px)_saturate(115%)] [-webkit-backdrop-filter:blur(10px)_saturate(115%)]",
						"[box-shadow:0_1px_0_rgba(255,255,255,0.92)_inset,0_5px_14px_-8px_rgba(0,0,0,0.38)]",
						"dark:border-[rgba(255,255,255,0.18)] dark:bg-[rgba(255,255,255,0.14)]",
						// One animation system, not two: the pill used to run a
						// framer-motion spring for its position AND a CSS transition
						// for its shadow on the same element. Both are CSS now, and
						// framer-motion is no longer imported into a globally mounted
						// component for a single tween.
						"transition-[transform,box-shadow] duration-200 ease-[var(--hkui-ease)] motion-reduce:transition-none",
						dragging
							? "[box-shadow:0_1px_0_rgba(255,255,255,0.92)_inset,0_8px_18px_-9px_rgba(0,0,0,0.44)]"
							: "",
					)}
					style={{
						width: target.width,
						height: target.height,
						opacity: measured ? 1 : 0,
						transform: `translate3d(${target.left}px, ${target.top}px, 0) scale(${
							!reduceMotion && dragging ? 1.05 : 1
						})`,
					}}
				/>
			) : null}

			{items.map((item, index) => (
				<Link
					ref={(el) => {
						itemEls.current[index] = el;
					}}
					href={item.href}
					// The four public tabs share a cross-navbar single-flight
					// coordinator below. Account/login intentionally stay on the
					// shared Link's role-aware path, and guest login keeps the
					// authentication hard-navigation boundary.
					prefetch={index < baseItems.length ? false : undefined}
					hardNavigation={!isLoggedIn && item.icon === "login"}
					aria-current={index === activeIndex ? "page" : undefined}
					className={cn(
						// `text-hk-*!` is important on purpose: the unlayered
						// `[data-theme="dark"] body a { color: --color-ink-200 }` rule in
						// styles/foundation/01-tokens-theme.css has higher specificity
						// (0,1,2) than the plain utility (0,1,0) and otherwise repaints
						// dark-mode link text. See memory tailwind-layer-vs-unlayered-css.
						"relative z-10 grid place-items-center gap-[3px] min-h-12 rounded-[16px] text-[0.6875rem] font-extrabold no-underline touch-manipulation",
						index === highlightIndex
							? "text-[#101010]! dark:text-[#f5f5f5]!"
							: "text-hk-muted!",
					)}
					key={item.label}
					onFocus={(event) => {
						if (
							index < baseItems.length &&
							event.currentTarget.matches(":focus-visible")
						) {
							prefetchMainTab(item.href, "focus");
						}
					}}
					onPointerEnter={(event) => {
						// Touch gets the stronger pointer-down signal below. Starting a
						// hover timer from synthetic touch pointer-enter would duplicate it.
						if (index < baseItems.length && event.pointerType !== "touch") {
							prefetchMainTab(item.href, "pointer");
						}
					}}
					onPointerLeave={() => {
						if (index < baseItems.length) {
							cancelMainNavPointerIntent(item.href);
						}
					}}
					onPointerDown={(event) => {
						if (index < baseItems.length && event.pointerType === "touch") {
							prefetchMainTab(item.href, "touch");
						}
					}}
					onClick={(event) => {
						// Modified clicks belong to the browser's new-tab/window action;
						// they cannot commit a route in this document to clear the pill.
						if (
							event.defaultPrevented ||
							event.button !== 0 ||
							event.ctrlKey ||
							event.metaKey ||
							event.shiftKey ||
							event.altKey ||
							(event.currentTarget.target &&
								event.currentTarget.target !== "_self")
						)
							return;
						if (index < baseItems.length) {
							cancelMainNavPointerIntent(item.href);
						}
						// Plain tap: let <Link> navigate, but move the pill at once.
						selectPendingTarget(index);
					}}
				>
					<span
						className="grid place-items-center size-[30px] overflow-hidden leading-[0] [&_svg]:block [&_svg]:size-[30px]"
						aria-hidden="true"
					>
						<AppIcon name={MOBILE_NAV_GLYPH[item.icon]} size={30} />
					</span>
					<span>{item.label}</span>
				</Link>
			))}
			{/* The optimistic pill is the only feedback a pending tab switch had.
			    Announce it too, so a screen-reader user knows the tap registered
			    before the route commits. */}
			<span className="sr-only" aria-live="polite">
				{mainTabPending ? `正在前往${items[loadingIndex]?.label ?? ""}` : ""}
			</span>
		</nav>
	);
}
