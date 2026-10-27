/**
 * The card the product quick-view modal should grow out of / shrink back into.
 * Set (viewport-centre of the tapped card) by the global product-link click
 * capture in PageTransition the instant the link is clicked; read once by
 * ProductModal on mount. Deep links / non-card opens read null → centre scale.
 */

import { STRIP_LOCALE_SCRIPT } from "@/lib/intl/paths";

interface QuickViewOrigin {
	cx: number;
	cy: number;
	/** window.scrollY at click time — the modal pins the page here so Next's
	 *  scroll-to-top on the product nav can't move it, and restores it on close. */
	scrollY: number;
	/** pathname+search of the page the quick-view opens over. The pinned scroll
	 *  belongs to that page: it may only be restored when the user actually
	 *  returns to it (close / back), never when they navigate onwards. */
	openerUrl: string;
}

export const PRODUCT_QUICK_VIEW_PENDING_EVENT = "hk:product-quick-view-pending";
const PRODUCT_QUICK_VIEW_PENDING_MAX_MS = 15_000;

type ProductQuickViewPendingController = {
	show: () => void;
	hide: () => boolean;
	origin: QuickViewOrigin | null;
	originSetAt: number;
};

type ProductQuickViewWindow = Window & {
	__hkProductQuickViewPending?: ProductQuickViewPendingController;
};

/**
 * Runs before hydration so native mouse clicks retain their card origin.
 * Keep this deliberately dependency-free: it is emitted inline near the top of
 * <body>, before the streamed home page and its product links.
 */
export const PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT = `(()=>{var strip=${STRIP_LOCALE_SCRIPT},e=${JSON.stringify(
	PRODUCT_QUICK_VIEW_PENDING_EVENT,
)},a=null,l=0,c={show:function(){},hide:function(){return false},origin:a,originSetAt:l};self.__hkProductQuickViewPending=c;self.addEventListener(e,c.show);document.addEventListener("click",function(n){if(n.defaultPrevented||n.button!==0||n.altKey||n.ctrlKey||n.metaKey||n.shiftKey||!(n.target instanceof Element))return;var r=n.target.closest("a[href]");if(!r||r.hasAttribute("data-storefront-product-preview-trigger")||r.hasAttribute("download")||r.getAttribute("aria-disabled")==="true"||r.target&&r.target!=="_self")return;try{var o=new URL(r.href,location.href);if(o.origin===location.origin&&/^\\/products\\/[^/]+\\/?$/.test(strip(o.pathname))){var f=r.closest(".market-product-card"),d=r.querySelector("[data-zoom-card]")||(f&&f.querySelector("[data-zoom-card]"))||r,u=d.getBoundingClientRect();a={cx:u.left+u.width/2,cy:u.top+u.height/2,scrollY:self.scrollY,openerUrl:location.pathname+location.search};l=performance.now();c.origin=a;c.originSetAt=l}}catch{}},true)})();`;

/** Compatibility no-op: source content now remains visible while RSC resolves. */
export function announceProductQuickViewPending(): void {
	if (typeof window === "undefined") return;
	window.dispatchEvent(new Event(PRODUCT_QUICK_VIEW_PENDING_EVENT));
}

/** Hand the pre-hydration loading shell over to the real product modal. */
export function hideProductQuickViewPending(): boolean {
	if (typeof window === "undefined") return false;
	const pendingWindow = window as ProductQuickViewWindow;
	if (pendingWindow.__hkProductQuickViewPending) {
		return pendingWindow.__hkProductQuickViewPending.hide();
	}
	return false;
}

let origin: QuickViewOrigin | null = null;
let setAt = 0;

export function setQuickViewOrigin(next: QuickViewOrigin | null): void {
	origin = next;
	setAt = typeof performance !== "undefined" ? performance.now() : 0;
	if (typeof window !== "undefined") {
		const controller = (window as ProductQuickViewWindow)
			.__hkProductQuickViewPending;
		if (controller) {
			controller.origin = next;
			controller.originSetAt = setAt;
		}
	}
}

/**
 * The page the currently-open quick-view was launched from.
 *
 * Deliberately separate from `origin` and free of its freshness window: the
 * origin is a click-time measurement (only meaningful for the open animation),
 * while this is the modal's *exit contract* and must stay valid for as long as
 * the user reads the product — a shopper who lingers 30s and then taps 店家頁
 * still has to leave the quick-view correctly. Published by ProductModal on
 * open and overwritten by the next open; never cleared on close, so the
 * comparison below stays order-independent with respect to the sibling effects
 * that run on the same commit.
 */
let quickViewOpenerUrl: string | null = null;

export function setQuickViewOpener(next: string | null): void {
	quickViewOpenerUrl = next;
}

/**
 * Is `href` the page the quick-view was opened over?
 *
 * True only for a genuine return (close / back / Escape). Every other
 * destination — 店家頁、購物車、結帳 — is a forward navigation that owns its
 * own scroll position; restoring the opener's offset there strands the new
 * route mid-content, which reads as "the tap did nothing".
 */
export function isQuickViewOpener(href: string): boolean {
	if (!quickViewOpenerUrl || typeof window === "undefined") return false;
	try {
		const url = new URL(href, window.location.href);
		return `${url.pathname}${url.search}` === quickViewOpenerUrl;
	} catch {
		return false;
	}
}

export function readQuickViewOrigin(): QuickViewOrigin | null {
	// Honour only a very recent capture (the click that opened this modal) so a
	// stale origin can't anchor an unrelated open.
	const now = typeof performance !== "undefined" ? performance.now() : 0;
	if (
		origin &&
		(typeof performance === "undefined" ||
			now - setAt < PRODUCT_QUICK_VIEW_PENDING_MAX_MS)
	) {
		return origin;
	}
	if (typeof window === "undefined") return null;
	const controller = (window as ProductQuickViewWindow)
		.__hkProductQuickViewPending;
	if (
		controller?.origin &&
		now - controller.originSetAt < PRODUCT_QUICK_VIEW_PENDING_MAX_MS
	) {
		return controller.origin;
	}
	return null;
}
