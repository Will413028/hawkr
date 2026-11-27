/**
 * Global navigation-pending signal → `html[data-nav-pending]`.
 *
 * Feeds the top progress rail (styles/components/nav-progress.css), the only
 * visible acknowledgement for slow soft navigations: the atomic keep-old-screen
 * doctrine (no loading.tsx, PageTransition renders no pixels in controllerOnly
 * mode) means a click on a slow destination otherwise changes nothing on screen
 * until the route commits.
 *
 * Deliberately DOM-attribute + CSS-animation based, not React state: the rail
 * must keep animating while the main thread is busy with the destination's
 * render work, and both producers (PageTransition's loading state machine and
 * Link's delayed-atomic ensure window) live outside a common React subtree.
 *
 * Ref-counted because the two producers overlap: Link's ensure window hands
 * over to PageTransition's history-patch signal on the same navigation.
 *
 * Lifecycle contract with the CSS:
 * - "" (pending)  → trickle, hidden for the first REVEAL_DELAY_MS
 * - "done"        → completion sweep — only entered when the rail actually
 *                   became visible; a wait that ends inside the reveal delay
 *                   clears silently, so fast navigations never flash a bar
 * - "abort"       → freeze + fade — the wait was cut off (loading-state fuse),
 *                   not completed; never claims a completion the user didn't get
 *
 * View Transition interplay: animated navs call document.startViewTransition
 * at click, freezing paint until commit — the reveal delay means the rail
 * never becomes visible inside that frozen window, so this signal is
 * effectively scoped to live-rendering waits: instant navs (data-vt="none"),
 * the ensure window, popstate, and same-path search-param navigations.
 */

const ATTR = "data-nav-pending";
/** Must match the CSS animation-delay on the trickle (nav-progress.css). */
const REVEAL_DELAY_MS = 250;
/** Reveal + a small margin: a rail visible for under ~2 frames still skips
 * the sweep — sweeping from a bar nobody saw reads as a random flash. */
const MIN_VISIBLE_FOR_SWEEP_MS = REVEAL_DELAY_MS + 80;
const DONE_VISIBLE_MS = 220;
const ABORT_VISIBLE_MS = 200;

export interface EndNavPendingOptions {
	/** The wait was cut off rather than completed (e.g. PageTransition's
	 * MAX_LOADING_VISIBLE_MS fuse, commit-poll cap). Plays the fade-out
	 * instead of the completion sweep. */
	abort?: boolean;
}

let activeCount = 0;
let clearTimer: ReturnType<typeof setTimeout> | null = null;
/** When the current pending phase (re)started — basis for the reveal check. */
let pendingSince = 0;

function root(): HTMLElement | null {
	return typeof document === "undefined" ? null : document.documentElement;
}

/**
 * Mark a navigation wait as started. Returns an idempotent end callback; when
 * the last active wait ends, the rail plays its completion sweep (or abort
 * fade) — or clears silently if it never got past the reveal delay.
 */
export function beginNavPending(): (options?: EndNavPendingOptions) => void {
	const el = root();
	if (!el) return () => {};

	installLifecycleGuard();
	activeCount += 1;
	if (clearTimer !== null) {
		clearTimeout(clearTimer);
		clearTimer = null;
	}
	if (el.getAttribute(ATTR) !== "") {
		el.setAttribute(ATTR, "");
		pendingSince = Date.now();
	}

	let ended = false;
	return (options) => {
		if (ended) return;
		ended = true;
		activeCount = Math.max(0, activeCount - 1);
		if (activeCount > 0) return;
		const rootEl = root();
		if (rootEl?.getAttribute(ATTR) !== "") return;
		if (Date.now() - pendingSince < MIN_VISIBLE_FOR_SWEEP_MS) {
			rootEl.removeAttribute(ATTR);
			return;
		}
		const phase = options?.abort ? "abort" : "done";
		rootEl.setAttribute(ATTR, phase);
		clearTimer = setTimeout(
			() => {
				clearTimer = null;
				if (activeCount === 0) root()?.removeAttribute(ATTR);
			},
			phase === "abort" ? ABORT_VISIBLE_MS : DONE_VISIBLE_MS,
		);
	};
}


const ROUTER_COMMIT_POLL_MS = 120;
/** 對齊 PageTransition 的 MAX_LOADING_VISIBLE_MS：超過視為導航被放棄。 */
const ROUTER_HOLD_FUSE_MS = 4000;

let routerHoldEnd: ((options?: EndNavPendingOptions) => void) | null = null;
let routerHoldSource = "";
let routerHoldDeadline = 0;
let routerHoldTimer: ReturnType<typeof setTimeout> | null = null;
let lifecycleGuardInstalled = false;

/** 硬導航/bfcache 快照前清乾淨：否則從 bfcache 返回的頁面會帶著亮軌復活，
 * 繼續 trickle 到保險絲。所有 producer 的殘存 end callback 之後再執行也安全
 * （attribute 已移除，end 內的 guard 直接 return）。 */
function installLifecycleGuard() {
	if (lifecycleGuardInstalled || typeof window === "undefined") return;
	lifecycleGuardInstalled = true;
	window.addEventListener("pagehide", () => {
		if (routerHoldTimer !== null) {
			clearTimeout(routerHoldTimer);
			routerHoldTimer = null;
		}
		routerHoldEnd = null;
		activeCount = 0;
		if (clearTimer !== null) {
			clearTimeout(clearTimer);
			clearTimer = null;
		}
		root()?.removeAttribute(ATTR);
	});
}

export function beginRouterTransitionPending(
	href: string,
	navigationType?: string,
): void {
	if (typeof window === "undefined" || !root()) return;
	if (navigationType === "traverse") return;
	let target: string;
	try {
		target = new URL(href, window.location.href).href;
	} catch {
		return;
	}
	if (target === window.location.href) return;
	installLifecycleGuard();
	routerHoldDeadline = Date.now() + ROUTER_HOLD_FUSE_MS;
	if (routerHoldEnd) return;
	routerHoldSource = window.location.href;
	routerHoldEnd = beginNavPending();
	const tick = () => {
		routerHoldTimer = null;
		if (!routerHoldEnd) return;
		if (window.location.href !== routerHoldSource) {
			routerHoldEnd();
			routerHoldEnd = null;
			return;
		}
		if (Date.now() >= routerHoldDeadline) {
			routerHoldEnd({ abort: true });
			routerHoldEnd = null;
			return;
		}
		routerHoldTimer = setTimeout(tick, ROUTER_COMMIT_POLL_MS);
	};
	routerHoldTimer = setTimeout(tick, ROUTER_COMMIT_POLL_MS);
}

/** Test-only: reset module state between cases. */
export function __resetNavPendingForTest() {
	activeCount = 0;
	pendingSince = 0;
	if (clearTimer !== null) {
		clearTimeout(clearTimer);
		clearTimer = null;
	}
	if (routerHoldTimer !== null) {
		clearTimeout(routerHoldTimer);
		routerHoldTimer = null;
	}
	routerHoldEnd = null;
	routerHoldSource = "";
	routerHoldDeadline = 0;
	root()?.removeAttribute(ATTR);
}
