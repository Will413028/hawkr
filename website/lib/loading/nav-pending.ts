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

	// Every producer, not just the router hold: a wait that is still open when
	// the document leaves (auth hard-navigation, the watchdog's recovery
	// assign, a role switch) would otherwise be frozen into the bfcache entry
	// and come back trickling forever the moment the user presses Back.
	installLifecycleGuard();
	activeCount += 1;
	if (clearTimer !== null) {
		clearTimeout(clearTimer);
		clearTimer = null;
	}
	// A value change alone restarts the trickle: "done"/"abort" → "" swaps
	// the matched animation-name, which resets the animation.
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
		// Ended inside the reveal window: the rail was never visible — clear
		// silently. Entering the sweep here would flash a full-width bar on
		// every fast navigation (quick-view open/close being the hottest).
		if (Date.now() - pendingSince < MIN_VISIBLE_FOR_SWEEP_MS) {
			rootEl.removeAttribute(ATTR);
			return;
		}
		const phase = options?.abort ? "abort" : "done";
		rootEl.setAttribute(ATTR, phase);
		clearTimer = setTimeout(
			() => {
				clearTimer = null;
				// Only clear if no new wait started during the sweep/fade.
				if (activeCount === 0) root()?.removeAttribute(ATTR);
			},
			phase === "abort" ? ABORT_VISIBLE_MS : DONE_VISIBLE_MS,
		);
	};
}

/* ── Router-transition hold ─────────────────────────────────────────────
 * 單一 producer 蓋住「每一個 push/replace soft nav」從發起到 URL commit 的
 * 等待窗。發起點是 instrumentation-client 的 onRouterTransitionStart——Next
 * 在導航開始時同步呼叫，因此程式化導航（bottom-nav 滑動手勢、分類 chip、
 * 搜尋鈕的 router.push）與 anchor 點擊一視同仁；相對地 patched pushState
 * 要到 route commit（HistoryUpdater 的 insertion effect）才跑，蓋不到窗。
 * commit 判定沿用 Link.tsx 的 location 輪詢（App Router 在 commit 時更新
 * location）。traverse 不收：popstate 當下 location 已是新 URL、輪詢無從
 * 判 commit，由 PageTransition 的 popstate 分支（React pathname commit
 * 訊號）負責。與 Link ensure hold / PageTransition loading hold 的重疊由
 * refcount 收斂。supersede（前一導航未 commit 又發起新導航）沿用同一個
 * hold、僅重掛保險絲——被 Next 放棄的導航不會 done，由保險絲 fade 收尾。 */

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
	// 同 URL 的 transition（refresh 類）沒有可觀察的 commit 邊界，不掛。
	if (target === window.location.href) return;
	installLifecycleGuard();
	routerHoldDeadline = Date.now() + ROUTER_HOLD_FUSE_MS;
	if (routerHoldEnd) return; // supersede：hold 沿用，保險絲已重掛
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
