interface SwipeClickState {
	current: boolean;
}

/**
 * Consumes the click suppression armed by a pointer swipe.
 *
 * Pointer-generated clicks have `detail > 0`; only the first one after a swipe
 * is blocked. Keyboard activation has `detail === 0`, is never blocked, and
 * clears any stale swipe state so accessibility cannot depend on pointer state.
 */
export function consumeBannerSwipeClick(
	state: SwipeClickState,
	detail: number,
): boolean {
	if (detail === 0) {
		state.current = false;
		return false;
	}
	if (!state.current) return false;
	state.current = false;
	return true;
}
