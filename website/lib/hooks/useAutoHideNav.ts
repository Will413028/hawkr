"use client";

import * as React from "react";
import { usePathname } from "@/lib/intl/navigation";

/**
 * Mastercard-style auto-hide navbar.
 *
 * Rules
 * ────────────────────────────────────────────────────────
 *  · Always visible when the page is at the top (scrollY < threshold)
 *  · Hide on scroll-down, show on scroll-up — direction-based, so a
 *    1px scroll-up flick reveals the bar even mid-page (matches the
 *    behaviour Mastercard, Stripe Press, IKEA, etc. use)
 *  · Desktop only: cursor moving into the top reveal zone
 *    (window top → `revealZone` px) forces the bar visible while the
 *    cursor stays there. Useful for users on long pages who want to
 *    navigate without scrolling all the way up.
 *  · Anything inside the navbar that wants to keep it open can call
 *    the returned `pin()` (returns an unpin function); pinned-count
 *    > 0 forces visible. The MegaMenu uses this so the panel can't
 *    detach from a hidden bar.
 *  · `prefers-reduced-motion`: no auto-hide at all.
 *  · Touch devices: same scroll logic; mousemove is a no-op so the
 *    cost is just two scroll listeners.
 *
 * Implementation notes
 *  · Scroll handler is rAF-throttled to one frame, so even nervous
 *    mousewheels don't queue up state updates.
 *  · `lastShowAt` debounces the toggle by `minVisibleMs` to prevent
 *    rubber-banding when scroll direction wobbles around zero delta.
 */

interface Options {
	/** Disable all auto-hide listeners and keep the nav visible. */
	disabled?: boolean;
	/** scrollY < this px → always visible. Default 80. */
	topThreshold?: number;
	/** scroll delta px required to flip direction. Default 6. */
	directionDelta?: number;
	/** desktop: edge zone (px) where cursor proximity reveals. Default 64. */
	revealZone?: number;
	/** minimum ms a "shown" state must hold before the next "hide"
	 *  fires. Stops rubber-banding. Default 220. */
	minVisibleMs?: number;
	/**
	 * Which viewport edge the reveal affordances watch. Default `"top"`
	 * (the floating top navbar). `"bottom"` flips cursor proximity to the
	 * lower edge AND enables a touch swipe-up-from-bottom gesture so a hidden
	 * bottom bar can be summoned without scrolling content — the same way iOS
	 * Safari / Chrome reveal their bottom toolbar. `"none"` disables both
	 * (scroll-direction only).
	 */
	revealEdge?: "top" | "bottom" | "none";
}

interface UseAutoHideNavResult {
	hidden: boolean;
	/** Pin visible. Returns an unpin fn. Stack pins with multiple callers. */
	pin: () => () => void;
}

/** 路由切換後多久內不接受「往下捲即隱藏」。導航會觸發程式性捲動 ——
 *  跨頁捲回頂部、Tab 切換還原捲動記憶（PageTransition 的釘捲動最長
 *  維持 800ms，內容長高時會反覆釘）—— 這些不是使用者手勢，方向判斷
 *  會把「還原到較深的位置」誤讀成往下捲而在換頁瞬間把 bar 收掉。
 *  900ms = 釘捲動上限 + 事件抵達的緩衝。 */
const ROUTE_SETTLE_MS = 900;

export function useAutoHideNav(opts: Options = {}): UseAutoHideNavResult {
	const disabled = opts.disabled ?? false;
	const topThreshold = opts.topThreshold ?? 80;
	const directionDelta = opts.directionDelta ?? 6;
	const revealZone = opts.revealZone ?? 64;
	const minVisibleMs = opts.minVisibleMs ?? 220;
	const revealEdge = opts.revealEdge ?? "top";

	const [hidden, setHidden] = React.useState(false);
	const pinCount = React.useRef(0);
	const lastY = React.useRef(0);
	const lastShowAt = React.useRef(0);
	const ticking = React.useRef(false);
	const routeSettleUntil = React.useRef(0);

	const pin = React.useCallback(() => {
		pinCount.current += 1;
		setHidden(false);
		return () => {
			pinCount.current = Math.max(0, pinCount.current - 1);
		};
	}, []);

	// 路由一換就強制顯示、把 delta 基準重設到當下的捲動位置，並開一段
	// settle 視窗停用「往下捲即隱藏」（見 ROUTE_SETTLE_MS）。沒有這段，
	// Tab 切換釘回捲動記憶的那一下會被當成使用者往下捲，bar 在換頁瞬間
	// 縮走、內容長高再釘時又彈回 —— 就是切頁時的抖動。使用者真手勢最多
	// 晚不到 1 秒才收起，無感。
	const pathname = usePathname() ?? "";
	// biome-ignore lint/correctness/useExhaustiveDependencies: re-baseline on route change
	React.useEffect(() => {
		if (disabled) return;
		routeSettleUntil.current = Date.now() + ROUTE_SETTLE_MS;
		lastY.current = window.scrollY || 0;
		lastShowAt.current = Date.now();
		setHidden(false);
	}, [pathname, disabled]);

	React.useEffect(() => {
		if (disabled) {
			setHidden(false);
			return;
		}

		// Reduced motion: never auto-hide.
		if (
			typeof window === "undefined" ||
			window.matchMedia?.("(prefers-reduced-motion: reduce)").matches
		) {
			return;
		}

		lastY.current = window.scrollY || 0;
		const overlayOpen = () =>
			document.body.hasAttribute("data-hk-overlay-open");

		const evaluate = () => {
			ticking.current = false;
			if (overlayOpen()) return;
			const y = window.scrollY || 0;
			const delta = y - lastY.current;
			lastY.current = y;

			if (pinCount.current > 0) {
				setHidden(false);
				lastShowAt.current = Date.now();
				return;
			}
			if (y < topThreshold) {
				setHidden(false);
				lastShowAt.current = Date.now();
				return;
			}
			if (delta > directionDelta) {
				// scrolling down — but ignore the programmatic scroll burst right
				// after a route change (scroll-to-top / tab scroll restore), and
				// respect minVisibleMs after a recent show.
				if (Date.now() < routeSettleUntil.current) return;
				if (Date.now() - lastShowAt.current >= minVisibleMs) {
					setHidden(true);
				}
			} else if (delta < -directionDelta) {
				setHidden(false);
				lastShowAt.current = Date.now();
			}
		};

		const onScroll = () => {
			if (overlayOpen()) return;
			if (ticking.current) return;
			ticking.current = true;
			window.requestAnimationFrame(evaluate);
		};

		// Overlays (product quick-view, Dialog) commonly freeze
		// background scroll while open, so no `scroll` event ever arrives to let
		// `evaluate()` re-run — without this, the nav would stay stuck in
		// whatever shown/hidden state it happened to be in when the overlay
		// opened. React to the attribute directly: force visible immediately on
		// open, and re-evaluate against the live scroll position once it closes.
		const onOverlayAttrChange = () => {
			if (overlayOpen()) {
				setHidden(false);
				lastShowAt.current = Date.now();
			} else {
				lastY.current = window.scrollY || 0;
				evaluate();
			}
		};
		const overlayObserver = new MutationObserver(onOverlayAttrChange);
		overlayObserver.observe(document.body, {
			attributes: true,
			attributeFilter: ["data-hk-overlay-open"],
		});

		// Edge proximity: ALWAYS call setHidden(false) inside the zone.
		// React dedupes identical state, so this is cheap when already shown.
		// Crucially, the handler does NOT read `hidden` from a closure, so we
		// can keep `hidden` out of the effect deps and bind these listeners
		// exactly once for the component's lifetime — that fixes a flicker
		// where the listener was unbound + rebound on every state flip and
		// could miss a mousemove fired during the swap window.
		//
		// `revealEdge` picks which edge counts: "top" → cursor within the top
		// `revealZone`; "bottom" → cursor within the bottom `revealZone`.
		const inRevealZone = (clientY: number) => {
			if (revealEdge === "none") return false;
			if (revealEdge === "bottom") {
				return clientY >= window.innerHeight - revealZone;
			}
			return clientY <= revealZone;
		};
		const onMouseMove = (e: MouseEvent) => {
			if (overlayOpen()) return;
			if (inRevealZone(e.clientY)) {
				lastShowAt.current = Date.now();
				setHidden(false);
			}
		};
		// pointermove additionally covers pen / Apple Pencil; we ignore touch
		// to avoid scroll-induced false positives.
		const onPointerMove = (e: PointerEvent) => {
			if (overlayOpen()) return;
			if (e.pointerType !== "mouse" && e.pointerType !== "pen") return;
			if (inRevealZone(e.clientY)) {
				lastShowAt.current = Date.now();
				setHidden(false);
			}
		};
		// Mouse re-enters the document near the watched edge (e.g. came back
		// from outside the browser): also reveal.
		const onMouseEnter = (e: MouseEvent) => {
			if (overlayOpen()) return;
			if (inRevealZone(e.clientY)) setHidden(false);
		};

		// Touch swipe-up-from-bottom (bottom edge only): summon the bar even
		// when the page can't scroll further (e.g. already at the very bottom).
		// We only start tracking when the finger lands inside the bottom gesture
		// band, then reveal once it has travelled up past a small threshold.
		const bottomGestureBand = Math.max(revealZone, 72);
		const swipeRevealDistance = 24;
		let touchStartY: number | null = null;
		const onTouchStart = (e: TouchEvent) => {
			if (overlayOpen()) return;
			if (revealEdge !== "bottom") return;
			const t = e.touches[0];
			touchStartY =
				t && t.clientY >= window.innerHeight - bottomGestureBand
					? t.clientY
					: null;
		};
		const onTouchMove = (e: TouchEvent) => {
			if (overlayOpen()) return;
			if (revealEdge !== "bottom" || touchStartY === null) return;
			const t = e.touches[0];
			if (!t) return;
			if (touchStartY - t.clientY > swipeRevealDistance) {
				lastShowAt.current = Date.now();
				setHidden(false);
				touchStartY = null; // one reveal per gesture
			}
		};
		const onTouchEnd = () => {
			touchStartY = null;
		};

		// PageTransition 的 scrollToNow（捲頂 / Tab 捲動記憶釘回）每次程式性
		// 捲動都廣播這個事件：收到就把 delta 基準對齊到捲動後的位置，讓
		// 隨後的 scroll 事件讀到 delta≈0 —— 程式性捲動永遠不觸發方向判斷，
		// 即使它發生在路由 settle 視窗之外（釘捲動會等內容長高才釘到位）。
		const onProgrammaticScroll = () => {
			lastY.current = window.scrollY || 0;
		};
		window.addEventListener("hk:programmatic-scroll", onProgrammaticScroll);

		window.addEventListener("scroll", onScroll, { passive: true });
		window.addEventListener("mousemove", onMouseMove, { passive: true });
		window.addEventListener("pointermove", onPointerMove, { passive: true });
		document.addEventListener("mouseenter", onMouseEnter, { passive: true });
		if (revealEdge === "bottom") {
			window.addEventListener("touchstart", onTouchStart, { passive: true });
			window.addEventListener("touchmove", onTouchMove, { passive: true });
			window.addEventListener("touchend", onTouchEnd, { passive: true });
		}
		return () => {
			overlayObserver.disconnect();
			window.removeEventListener(
				"hk:programmatic-scroll",
				onProgrammaticScroll,
			);
			window.removeEventListener("scroll", onScroll);
			window.removeEventListener("mousemove", onMouseMove);
			window.removeEventListener("pointermove", onPointerMove);
			document.removeEventListener("mouseenter", onMouseEnter);
			window.removeEventListener("touchstart", onTouchStart);
			window.removeEventListener("touchmove", onTouchMove);
			window.removeEventListener("touchend", onTouchEnd);
		};
	}, [
		disabled,
		topThreshold,
		directionDelta,
		revealZone,
		minVisibleMs,
		revealEdge,
	]);

	return { hidden, pin };
}
