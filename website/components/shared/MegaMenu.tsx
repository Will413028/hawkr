"use client";

import { motion, useReducedMotion } from "framer-motion";
import * as React from "react";
import { useNavPin } from "@/components/shared/AutoHideNavShell";
import Link from "@/components/shared/Link";
import { getFocusableElements } from "@/lib/a11y/focusable";
import type { PostListItem } from "@/lib/api/openapi-types";
import type { NavItem, NavMegaLink } from "@/lib/content/data";
import { usePathname } from "@/lib/intl/navigation";
import {
	cancelMainNavPointerIntent,
	prefetchMainNavOnIntent,
} from "@/lib/navigation/main-nav-prefetch";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import type { MegaMenuPanelProps } from "./MegaMenuPanel";
import MegaMenuSurface from "./MegaMenuSurface";

/**
 * Mastercard-style mega menu.
 *
 * Behaviour
 * ────────────────────────────────────────────────────────────
 *  · Hover / focus on a top-level trigger opens a full-width panel that
 *    slides down from underneath the navbar.
 *  · A semi-transparent backdrop dims the rest of the page so the menu
 *    feels like a sheet instead of a tooltip.
 *  · 150 ms close-intent delay so the user can swing the cursor from
 *    the trigger to the panel without it snapping shut.
 *  · Esc / click-outside / route navigation closes.
 *  · Mobile (<768px) collapses each panel into stacked accordions.
 *
 * The data shape lives on `NavItem.panel` (lib/data.ts). Items without
 * `panel` render as plain links.
 */

export type MegaMenuDynamicLinks = {
	categories?: NavMegaLink[];
	hashtags?: NavMegaLink[];
	/** Drives the Hawkr 動態 panel's right-slot "本週主題" carousel
	 *  (top 5 most-viewed posts of the recent batch). Empty array when
	 *  /public/posts returned nothing — the slot then renders a tasteful
	 *  empty state (`<WeeklyTopPostEmpty>`) inviting the buyer into the
	 *  feed, never fake post copy. */
	topPosts?: PostListItem[];
};

interface MegaMenuProps {
	items: NavItem[];
	/** className passed straight onto each top-level trigger so the host
	 *  navbar can keep its existing active/hover styling. */
	triggerClassName: (item: NavItem) => string;
	/** Route matcher for the shared active-page indicator and aria-current. */
	isItemActive?: (item: NavItem) => boolean;
	/**
	 * Prefetched data for columns whose `source` is set in `navItems`.
	 * Each entry is the final rendered link list — layout-level fetchers
	 * are responsible for transforming the raw API response into
	 * `NavMegaLink[]`. Empty / missing entries skip the column.
	 */
	dynamicLinks?: MegaMenuDynamicLinks;
}

const OPEN_DELAY_MS = 140;
const CLOSE_DELAY_MS = 220;

type PanelComponent = React.ComponentType<MegaMenuPanelProps>;
let cachedPanel: PanelComponent | null = null;
let panelRequest: Promise<PanelComponent> | null = null;

/** Intent-only resource; a failed speculative request must not poison retry. */
export function loadMegaMenuPanel(): Promise<PanelComponent> {
	if (cachedPanel) return Promise.resolve(cachedPanel);
	if (!panelRequest) {
		panelRequest = import("./MegaMenuPanel").then(
			(module) => {
				cachedPanel = module.default;
				return module.default;
			},
			(error) => {
				panelRequest = null;
				throw error;
			},
		);
	}
	return panelRequest;
}
function prewarmPanel() {
	void loadMegaMenuPanel().catch(() => {});
}

export default function MegaMenu({
	items,
	triggerClassName,
	isItemActive,
	dynamicLinks,
}: MegaMenuProps) {
	const pathname = usePathname() ?? "";
	const router = useObservedRouter("mega_menu");
	const reduceMotion = useReducedMotion();
	const [openId, setOpenId] = React.useState<string | null>(null);
	const [Panel, setPanel] = React.useState<PanelComponent | null>(
		() => cachedPanel,
	);
	const [loadError, setLoadError] = React.useState(false);
	const [retry, setRetry] = React.useState(0);
	const activeTriggerRef = React.useRef<HTMLAnchorElement | null>(null);
	const suppressFocusOpen = React.useRef(false);
	const navRef = React.useRef<HTMLElement | null>(null);
	const panelRef = React.useRef<HTMLElement | null>(null);
	const panelId = `${React.useId()}-mega-panel`;
	React.useEffect(() => {
		void retry; // An explicit retry starts a new request after a failed import.
		if (!openId || Panel) return;
		let active = true;
		setLoadError(false);
		loadMegaMenuPanel().then(
			(component) => {
				if (active) setPanel(() => component);
			},
			() => {
				if (active) setLoadError(true);
			},
		);
		return () => {
			active = false;
		};
	}, [openId, Panel, retry]);
	const openTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
	const closeTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);

	const cancelOpen = React.useCallback(() => {
		if (openTimer.current) {
			clearTimeout(openTimer.current);
			openTimer.current = null;
		}
	}, []);

	const cancelClose = React.useCallback(() => {
		if (closeTimer.current) {
			clearTimeout(closeTimer.current);
			closeTimer.current = null;
		}
	}, []);

	const scheduleOpen = React.useCallback(
		(id: string) => {
			cancelOpen();
			cancelClose();
			openTimer.current = setTimeout(
				() => {
					prewarmPanel();
					setOpenId(id);
					openTimer.current = null;
				},
				openId ? OPEN_DELAY_MS / 2 : OPEN_DELAY_MS,
			);
		},
		[cancelClose, cancelOpen, openId],
	);

	const scheduleClose = React.useCallback(() => {
		cancelOpen();
		cancelClose();
		closeTimer.current = setTimeout(() => setOpenId(null), CLOSE_DELAY_MS);
	}, [cancelClose, cancelOpen]);

	const open = (id: string) => {
		prewarmPanel();
		cancelOpen();
		cancelClose();
		setOpenId(id);
	};

	const close = React.useCallback(() => {
		cancelOpen();
		cancelClose();
		setOpenId(null);
	}, [cancelClose, cancelOpen]);

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
		return () => {
			cancelOpen();
			cancelClose();
		};
	}, [cancelClose, cancelOpen]);

	// Route changes must not leave an old overlay or delayed hover open.
	React.useEffect(() => {
		void pathname;
		close();
	}, [pathname, close]);

	// Esc to close
	React.useEffect(() => {
		if (!openId) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") {
				close();
				const trigger = activeTriggerRef.current;
				if (trigger && document.activeElement !== trigger) {
					suppressFocusOpen.current = true;
					trigger.focus({ preventScroll: true });
					suppressFocusOpen.current = false;
				}
			}
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [openId, close]);

	// Keep the auto-hide navbar pinned visible while the panel is open —
	// otherwise scrolling the page would slide the bar (and our anchor
	// for the panel) off-screen mid-interaction.
	const pinNav = useNavPin();
	React.useEffect(() => {
		if (!openId || !pinNav) return;
		const release = pinNav();
		return release;
	}, [openId, pinNav]);

	// Outside-click close. This used to be a full-viewport `pointer-events-auto`
	// backdrop div, which swallowed the first click on any page content while a
	// panel was open (dismissing the menu cost the user a click). A capture-phase
	// document listener does the same job without preventing the click from
	// reaching what the user actually aimed at.
	React.useEffect(() => {
		if (!openId) return;
		const onDocumentPointerDown = (event: PointerEvent) => {
			const target = event.target as Node | null;
			if (!target) return;
			if (navRef.current?.contains(target)) return;
			if (panelRef.current?.contains(target)) return;
			close();
		};
		document.addEventListener("pointerdown", onDocumentPointerDown, true);
		return () =>
			document.removeEventListener("pointerdown", onDocumentPointerDown, true);
	}, [openId, close]);

	// Roving arrow-key movement across the top-level triggers. The panel keeps
	// following focus, so arrowing along the bar previews each section the same
	// way tabbing already did.
	const onNavKeyDown = React.useCallback((event: React.KeyboardEvent) => {
		const { key } = event;
		if (
			key !== "ArrowLeft" &&
			key !== "ArrowRight" &&
			key !== "Home" &&
			key !== "End"
		) {
			return;
		}
		const nav = navRef.current;
		if (!nav) return;
		const triggers = Array.from(
			nav.querySelectorAll<HTMLAnchorElement>("a[href]"),
		);
		if (triggers.length === 0) return;
		const current = triggers.indexOf(
			document.activeElement as HTMLAnchorElement,
		);
		if (current < 0) return;
		event.preventDefault();
		let next: number;
		if (key === "Home") next = 0;
		else if (key === "End") next = triggers.length - 1;
		else if (key === "ArrowRight") next = (current + 1) % triggers.length;
		else next = (current - 1 + triggers.length) % triggers.length;
		triggers[next]?.focus();
	}, []);

	// Tab from an open trigger enters its panel instead of walking the remaining
	// triggers first; Shift+Tab off the panel's first control returns to it.
	const onTriggerKeyDown = React.useCallback(
		(event: React.KeyboardEvent, isOpen: boolean) => {
			if (event.key !== "Tab" || event.shiftKey || !isOpen) return;
			const panel = panelRef.current;
			if (!panel) return;
			const first = getFocusableElements(panel)[0];
			if (!first) return;
			event.preventDefault();
			first.focus();
		},
		[],
	);

	const onPanelKeyDown = React.useCallback((event: React.KeyboardEvent) => {
		if (event.key !== "Tab" || !event.shiftKey) return;
		const panel = panelRef.current;
		const trigger = activeTriggerRef.current;
		if (!panel || !trigger) return;
		if (getFocusableElements(panel)[0] !== document.activeElement) return;
		event.preventDefault();
		trigger.focus({ preventScroll: true });
	}, []);

	// Body scroll-lock removed: when the page has a vertical scrollbar,
	// switching `overflow: hidden` removes ~15px of width and the entire
	// content (incl. the navbar) jumps right by that amount on every
	// open/close. The mega menu is a hover affordance, not a modal, so
	// just letting the page scroll behind the backdrop is fine. The
	// backdrop + visible panel still keeps focus on the menu.

	const activeItem = openId
		? items.find((it) => it.label === openId && it.panel)
		: null;

	return (
		<>
			<nav
				ref={navRef}
				className="relative z-[42] inline-flex items-center gap-[var(--space-2)]"
				// Distinct from the host navbar's own 「主要導覽」 landmark: two
				// nested nav elements with the same name read as one duplicated
				// entry in a screen reader's landmark list.
				aria-label="商品分類"
				onMouseLeave={scheduleClose}
				onMouseEnter={cancelClose}
				onKeyDown={onNavKeyDown}
			>
				{items.map((item) => {
					const hasPanel = !!item.panel;
					const isOpen = openId === item.label;
					const isCurrent = isItemActive?.(item) ?? false;
					const className = triggerClassName(item);
					const activeIndicator = isCurrent ? (
						<motion.span
							aria-hidden="true"
							data-nav-active-indicator="true"
							layoutId={
								reduceMotion ? undefined : "buyer-primary-nav-active-indicator"
							}
							className="absolute inset-0 z-0 rounded-[inherit] bg-[var(--theme-primary-subtle)] shadow-[var(--page-nav-active-indicator-ring)]"
							transition={
								reduceMotion
									? { duration: 0 }
									: { type: "spring", stiffness: 430, damping: 34, mass: 0.8 }
							}
						/>
					) : null;
					if (!hasPanel) {
						// Hovering / focusing a panel-less item (關於我們, 招商合作, 搜尋)
						// must dismiss any open panel — otherwise sweeping the cursor
						// off 市集 onto a neighbour leaves its panel stuck open (you
						// never leave <nav>, so the nav-level onMouseLeave never fires).
						return (
							<Link
								key={item.label}
								href={item.href}
								prefetch={false}
								className={className}
								aria-current={isCurrent ? "page" : undefined}
								onMouseEnter={scheduleClose}
								onPointerEnter={(event) => {
									if (event.pointerType !== "touch") {
										prefetchMainTab(item.href, "pointer");
									}
								}}
								onPointerLeave={() => cancelMainNavPointerIntent(item.href)}
								onPointerDown={(event) => {
									if (event.pointerType === "touch") {
										prefetchMainTab(item.href, "touch");
									}
								}}
								onFocus={(event) => {
									scheduleClose();
									if (event.currentTarget.matches(":focus-visible")) {
										prefetchMainTab(item.href, "focus");
									}
								}}
								onClick={() => cancelMainNavPointerIntent(item.href)}
							>
								{activeIndicator}
								<span className="relative z-10">{item.label}</span>
							</Link>
						);
					}
					return (
						<div key={item.label} className="relative inline-flex">
							<Link
								href={item.href}
								prefetch={false}
								className={`${className} inline-flex items-center gap-1`}
								aria-current={isCurrent ? "page" : undefined}
								// Not aria-haspopup="menu": the panel is a content
								// section, not a menu. expanded + controls is the
								// disclosure pattern that actually describes it.
								aria-expanded={isOpen}
								aria-controls={panelId}
								onKeyDown={(event) => onTriggerKeyDown(event, isOpen)}
								onMouseEnter={(event) => {
									activeTriggerRef.current = event.currentTarget;
									scheduleOpen(item.label);
								}}
								onPointerEnter={(event) => {
									if (event.pointerType !== "touch") {
										prefetchMainTab(item.href, "pointer");
									}
								}}
								onPointerLeave={() => cancelMainNavPointerIntent(item.href)}
								onPointerDown={(event) => {
									if (event.pointerType !== "mouse") prewarmPanel();
								}}
								onFocus={(event) => {
									activeTriggerRef.current = event.currentTarget;
									if (suppressFocusOpen.current) return;
									if (event.currentTarget.matches(":focus-visible"))
										open(item.label);
									if (event.currentTarget.matches(":focus-visible")) {
										prefetchMainTab(item.href, "focus");
									}
								}}
								onClick={(e) => {
									cancelMainNavPointerIntent(item.href);
									// Tap the trigger on touch toggles the panel instead of
									// navigating away. Holding link semantics for keyboard
									// users (Tab → Enter still navigates).
									// `pointer: coarse` is included on purpose: a hybrid
									// laptop/tablet reports `hover: hover` yet is driven by a
									// finger, so the hover-open path never fires there and
									// the panel was unreachable.
									if (
										typeof window !== "undefined" &&
										window.matchMedia("(hover: none), (pointer: coarse)")
											.matches
									) {
										e.preventDefault();
										if (isOpen) close();
										else open(item.label);
									}
								}}
							>
								{activeIndicator}
								<span className="relative z-10">{item.label}</span>
								<svg
									className={`relative z-10 fill-none stroke-current stroke-[2.4] stroke-linecap-round stroke-linejoin-round w-[14px] h-[14px] transition-transform duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] motion-reduce:transition-none ${isOpen ? "rotate-180" : ""}`}
									viewBox="0 0 24 24"
									aria-hidden="true"
								>
									<path d="m6 9 6 6 6-6" />
								</svg>
							</Link>
						</div>
					);
				})}
			</nav>

			{/* No backdrop element: it was a transparent full-viewport layer whose
			 *  only job was the click-outside close, and `pointer-events-auto`
			 *  made it eat the first click on the page underneath. The
			 *  capture-phase document listener above closes on the same gesture
			 *  and lets the click through. */}

			{/* Single shared panel — content swaps based on activeItem.
			 *  Anchored to the navbar bottom via CSS (top: var(--nav-height)).
			 *  Outer <section> is a transparent positioning shell so the white
			 *  card chrome (bg/border/radius/shadow) lives on the inner div —
			 *  that's what makes the panel read as a centered popup instead of
			 *  a full-bleed slab across the viewport. */}
			<section
				ref={panelRef}
				id={panelId}
				className={`fixed left-0 right-0 top-[var(--site-nav-inner-height,100px)] z-[40] flex max-h-[calc(100dvh-var(--site-nav-inner-height,100px)-12px)] items-start justify-center overflow-y-auto overscroll-contain px-4 pointer-events-none transition-[opacity,transform,visibility] duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] motion-reduce:transition-none max-[768px]:top-[var(--nav-height,56px)] max-[768px]:max-h-[calc(100dvh-var(--nav-height,56px))] max-[768px]:px-0 ${activeItem ? "opacity-100 translate-y-0 visible [transition-delay:0s]" : "opacity-0 -translate-y-3 invisible [transition-delay:0s,0s,var(--hkui-dur-base)]"}`}
				aria-label={activeItem ? `${activeItem.label} 子選單` : undefined}
				onKeyDown={onPanelKeyDown}
			>
				{activeItem?.panel ? (
					<MegaMenuSurface
						contentKey={activeItem.label}
						onMouseEnter={cancelClose}
						onMouseLeave={scheduleClose}
						onFocus={cancelClose}
					>
						{Panel ? (
							<Panel
								item={activeItem}
								dynamicLinks={dynamicLinks}
								close={close}
							/>
						) : (
							<>
								<div
									data-mega-menu-loading
									className="flex-[0_0_280px] min-h-[320px] max-w-full [@media(max-width:840px)]:w-full [@media(max-width:840px)]:flex-none"
								>
									<p
										role={loadError ? "alert" : "status"}
										aria-busy={!loadError}
										className="m-0 text-sm text-foreground-soft"
									>
										{loadError
											? "選單暫時載入失敗，請再試一次。"
											: "選單載入中…"}
									</p>
									{loadError ? (
										<button
											type="button"
											className="mt-4 min-h-11 rounded-full border border-hk-border-soft px-4 text-sm text-hk-primary"
											onClick={() => setRetry((value) => value + 1)}
										>
											重新載入選單
										</button>
									) : null}
									<Link
										href={activeItem.href}
										onClick={close}
										className="mt-4 inline-flex min-h-11 items-center text-sm text-hk-primary"
									>
										前往{activeItem.label}
									</Link>
								</div>
								<div
									aria-hidden="true"
									className="w-[var(--mega-feature-w,440px)] flex-[0_0_var(--mega-feature-w,440px)] max-w-full min-h-[320px] rounded-2xl bg-hk-surface-glass [@media(max-width:840px)]:hidden"
								/>
							</>
						)}
					</MegaMenuSurface>
				) : null}
			</section>
		</>
	);
}
