"use client";

import * as React from "react";
import type { RoleSlot } from "./active-role";
import { useAuth } from "./context";

interface AuthGateSnapshot {
	/** Active role resolved AFTER hydration — don't capture the render-time one. */
	activeRole: RoleSlot | null;
}

type GatedAction = (auth: AuthGateSnapshot) => void;

/**
 * Defer auth-gated click actions until the sign-in state has resolved.
 *
 * Account identity lives in a Clerk-user-keyed query, so every full page
 * load starts as a guest until Clerk's SDK loads and GetSession returns. A handler
 * that branches on `isLoggedIn` directly would open the login modal for an
 * actually-logged-in user who taps during that window (common on mobile —
 * the gated buttons sit right under the thumb on first paint).
 *
 * `authGate(action)` absorbs the race: before hydration it parks the
 * action and flips `authWaiting` (callers render `<AuthGateWaitModal>`);
 * once the session is known it replays the action — or calls `onGuest`
 * (open the login modal) if the visitor really is logged out.
 */
export function useAuthGate(onGuest: () => void): {
	authGate: (action: GatedAction) => void;
	authWaiting: boolean;
} {
	const { isLoggedIn, hydrated, activeRole } = useAuth();
	const [pending, setPending] = React.useState<GatedAction | null>(null);
	// Latest-callback ref: callers pass inline closures, and renders can
	// happen between the click and hydration completing.
	const onGuestRef = React.useRef(onGuest);
	onGuestRef.current = onGuest;

	React.useEffect(() => {
		if (!hydrated || !pending) return;
		setPending(null);
		if (isLoggedIn) pending({ activeRole });
		else onGuestRef.current();
	}, [hydrated, isLoggedIn, activeRole, pending]);

	const authGate = React.useCallback(
		(action: GatedAction) => {
			if (!hydrated) {
				setPending(() => action);
				return;
			}
			if (!isLoggedIn) {
				onGuestRef.current();
				return;
			}
			action({ activeRole });
		},
		[hydrated, isLoggedIn, activeRole],
	);

	return { authGate, authWaiting: pending !== null };
}
