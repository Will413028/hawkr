"use client";

import * as React from "react";

const subscribeHydration = () => () => {};

export const hydrationClientSnapshot = () => true;
export const hydrationServerSnapshot = () => false;

/**
 * Returns `false` for SSR and the matching first hydration render, then `true`
 * immediately after hydration. Components mounted later by client navigation
 * read `true` on their first render.
 *
 * This is the safe boundary for browser state that must never influence the
 * HTML React hydrates, but may synchronously seed a client-only mount.
 */
export function useHydrationSafeClientRender(): boolean {
	return React.useSyncExternalStore(
		subscribeHydration,
		hydrationClientSnapshot,
		hydrationServerSnapshot,
	);
}
