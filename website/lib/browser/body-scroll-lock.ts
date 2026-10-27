type ScrollLockMode = "overflow" | "fixed";

export interface ReleaseBodyScrollLockOptions {
	/**
	 * Scroll back to the offset that was pinned when the lock was taken.
	 *
	 * Defaults to `true` — an overlay closing over the same page must reveal it
	 * exactly where the user left it. Pass `false` when the release happens
	 * because the document moved on to a different route: the pinned offset
	 * belongs to the page underneath, and forcing it onto the new one drops the
	 * user into the middle of content they never scrolled.
	 */
	restoreScrollPosition?: boolean;
}

export interface BodyScrollLockOptions {
	/**
	 * Keep the current document offset by pinning the body. Use for full-screen
	 * viewers where dropping back to the top underneath the overlay is visible.
	 */
	preserveScrollPosition?: boolean;
	/** Explicit offset to preserve when the opener captured it before routing. */
	scrollY?: number;
	/** Lock the root element as well (needed by viewport-sized mobile sheets). */
	lockRoot?: boolean;
	/** Prevent scroll chaining while the overlay is active. */
	preventOverscroll?: boolean;
}

type LockRecord = {
	mode: ScrollLockMode;
	lockRoot: boolean;
	preventOverscroll: boolean;
	scrollY?: number;
};

type StyleSnapshot = {
	body: {
		left: string;
		overflow: string;
		overscrollBehavior: string;
		position: string;
		right: string;
		top: string;
		width: string;
	};
	root: {
		overflow: string;
		overscrollBehavior: string;
		scrollBehavior: string;
	};
};

const locks = new Map<symbol, LockRecord>();
let snapshot: StyleSnapshot | null = null;
let pinnedScrollY: number | null = null;

function captureSnapshot(): StyleSnapshot {
	const { body, documentElement: root } = document;
	return {
		body: {
			left: body.style.left,
			overflow: body.style.overflow,
			overscrollBehavior: body.style.overscrollBehavior,
			position: body.style.position,
			right: body.style.right,
			top: body.style.top,
			width: body.style.width,
		},
		root: {
			overflow: root.style.overflow,
			overscrollBehavior: root.style.overscrollBehavior,
			scrollBehavior: root.style.scrollBehavior,
		},
	};
}

function restorePinnedPosition(base: StyleSnapshot, restoreScroll: boolean) {
	if (pinnedScrollY === null) return;
	const { body, documentElement: root } = document;
	body.style.position = base.body.position;
	body.style.top = base.body.top;
	body.style.left = base.body.left;
	body.style.right = base.body.right;
	body.style.width = base.body.width;
	// Un-pinning alone already drops the document back to offset 0, which is
	// the correct start for whatever route replaced the pinned one.
	if (restoreScroll) {
		const previousBehavior = root.style.scrollBehavior;
		root.style.scrollBehavior = "auto";
		window.scrollTo(0, pinnedScrollY);
		root.style.scrollBehavior = previousBehavior;
	}
	pinnedScrollY = null;
}

function reconcileLocks(restoreScroll = true) {
	const base = snapshot;
	if (!base) return;
	const { body, documentElement: root } = document;
	const records = [...locks.values()];

	if (records.length === 0) {
		body.style.overflow = base.body.overflow;
		body.style.overscrollBehavior = base.body.overscrollBehavior;
		body.style.position = base.body.position;
		body.style.top = base.body.top;
		body.style.left = base.body.left;
		body.style.right = base.body.right;
		body.style.width = base.body.width;
		root.style.overflow = base.root.overflow;
		root.style.overscrollBehavior = base.root.overscrollBehavior;
		root.style.scrollBehavior = base.root.scrollBehavior;
		// Restore scrollability before scrollTo. With overflow still hidden,
		// browsers can clamp the saved offset to 0 even after body is unpinned.
		restorePinnedPosition(base, restoreScroll);
		snapshot = null;
		return;
	}

	const needsFixed = records.some((record) => record.mode === "fixed");
	const needsRootLock = records.some((record) => record.lockRoot);
	const preventsOverscroll = records.some((record) => record.preventOverscroll);

	body.style.overflow = "hidden";
	body.style.overscrollBehavior = preventsOverscroll
		? "none"
		: base.body.overscrollBehavior;
	root.style.overflow = needsRootLock ? "hidden" : base.root.overflow;
	root.style.overscrollBehavior = preventsOverscroll
		? "none"
		: base.root.overscrollBehavior;

	if (needsFixed) {
		if (pinnedScrollY === null) {
			pinnedScrollY =
				records.find((record) => record.mode === "fixed")?.scrollY ??
				window.scrollY;
		}
		body.style.position = "fixed";
		body.style.top = `-${pinnedScrollY}px`;
		body.style.left = "0";
		body.style.right = "0";
		body.style.width = "100%";
	} else {
		restorePinnedPosition(base, restoreScroll);
	}
}

/**
 * Acquire a re-entrant global page-scroll lock.
 *
 * Every caller receives an idempotent release function. The original inline
 * styles are captured once and restored only after the final caller releases,
 * preventing overlapping dialogs/routes from restoring a stale "hidden".
 */
export function acquireBodyScrollLock(
	options: BodyScrollLockOptions = {},
): (releaseOptions?: ReleaseBodyScrollLockOptions) => void {
	if (typeof document === "undefined" || typeof window === "undefined") {
		return () => {};
	}
	if (!snapshot) snapshot = captureSnapshot();

	const token = Symbol("body-scroll-lock");
	locks.set(token, {
		mode: options.preserveScrollPosition ? "fixed" : "overflow",
		lockRoot: options.lockRoot ?? false,
		preventOverscroll: options.preventOverscroll ?? false,
		scrollY: options.scrollY,
	});
	reconcileLocks();

	let released = false;
	return (releaseOptions) => {
		if (released) return;
		released = true;
		locks.delete(token);
		reconcileLocks(releaseOptions?.restoreScrollPosition ?? true);
	};
}

/** Test-only reset for isolated DOM fakes. */
export function resetBodyScrollLocksForTest() {
	locks.clear();
	if (snapshot && typeof document !== "undefined") {
		reconcileLocks();
	}
	snapshot = null;
	pinnedScrollY = null;
}
