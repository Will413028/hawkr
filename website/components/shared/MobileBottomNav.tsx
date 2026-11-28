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
	const { status, activeRole } = useAuth();
	const signedIn = status === "signedIn";
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

	React.useEffect(
		() =>
			scheduleMainNavIdleWarm({
				router,
				currentPathname: pathname,
			}),
		[pathname, router],
	);

	const { hidden } = useAutoHideNav({
		topThreshold: 56,
		directionDelta: 4,
		minVisibleMs: 160,
		revealEdge: "bottom",
	});

	const items = React.useMemo(() => {
		const meItem = {
			pending: status === "unknown",
			label: signedIn ? "設定" : "登入",
			href: signedIn
				? activeRole === "seller"
					? "/seller-center/me"
					: "/account"
				: "/login?role=buyer",
			icon: (signedIn ? "user" : "login") as MobileNavIconName,
		};
		return [...baseItems.map((item) => ({ ...item, pending: false })), meItem];
	}, [status, signedIn, activeRole]);

	const activeIndex = React.useMemo(() => {
		if (!pathname) return 0;
		const idx = items.findIndex((item) => isNavPathActive(pathname, item.href));
		return idx === -1 ? 0 : idx;
	}, [pathname, items]);

	const navRef = React.useRef<HTMLElement | null>(null);
	const itemEls = React.useRef<(HTMLAnchorElement | null)[]>([]);

	const [slots, setSlots] = React.useState<Slot[]>([]);
	const measured = slots.length === items.length;

	const measure = React.useCallback(() => {
		const els = itemEls.current;
		if (els.length < items.length) return;
		const next: Slot[] = [];
		for (let i = 0; i < items.length; i += 1) {
			const el = els[i];
			if (!el) return;
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

	const pressedRef = React.useRef(false);
	const startXRef = React.useRef(0);
	const draggingRef = React.useRef(false);
	const hoverRef = React.useRef(activeIndex);
	const swallowClickRef = React.useRef(false);

	const [dragging, setDragging] = React.useState(false);
	const [hoverIndex, setHoverIndex] = React.useState(activeIndex);
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

	// biome-ignore lint/correctness/useExhaustiveDependencies: clear on route change
	React.useEffect(() => {
		clearPendingSelection();
	}, [pathname]);

	React.useEffect(() => {
		const root = document.documentElement;
		const observer = new MutationObserver((records) => {
			const pending = pendingNavigationRef.current;
			if (!pending) return;
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
			if (event.detail !== 0) event.preventDefault();
		},
		[],
	);

	// an in-thread back action, so mounting the global nav there would cover the
	if (
		!mounted ||
		!pathname ||
		pathname?.startsWith("/seller-center") ||
		isDelegatedWorkspacePath(pathname) ||
		isAccountMessageThreadPath(pathname) ||
		isStandaloneFlowPath(pathname)
	)
		return null;

	const highlightIndex = targetIndex;

	return (
		<nav
			ref={navRef}
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
			{target ? (
				<span
					aria-hidden="true"
					data-mobile-nav-active-indicator="true"
					data-dragging={dragging ? "true" : "false"}
					className={cn(
						"absolute left-0 top-0 z-[1] rounded-[16px] pointer-events-none will-change-transform overflow-hidden",
						"border border-[rgba(255,255,255,0.82)] bg-[rgba(255,255,255,0.72)]",
						"[backdrop-filter:blur(10px)_saturate(115%)] [-webkit-backdrop-filter:blur(10px)_saturate(115%)]",
						"[box-shadow:0_1px_0_rgba(255,255,255,0.92)_inset,0_5px_14px_-8px_rgba(0,0,0,0.38)]",
						"dark:border-[rgba(255,255,255,0.18)] dark:bg-[rgba(255,255,255,0.14)]",
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

			{items.map((item, index) =>
				item.pending ? (
					<span
						key={item.label}
						aria-hidden="true"
						className="relative z-10 min-h-12"
					/>
				) : (
					<Link
						ref={(el) => {
							itemEls.current[index] = el;
						}}
						href={item.href}
						prefetch={index < baseItems.length ? false : undefined}
						hardNavigation={!signedIn && item.icon === "login"}
						aria-current={index === activeIndex ? "page" : undefined}
						className={cn(
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
				),
			)}

			<span className="sr-only" aria-live="polite">
				{mainTabPending ? `正在前往${items[loadingIndex]?.label ?? ""}` : ""}
			</span>
		</nav>
	);
}
