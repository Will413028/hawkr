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
 * The sign-in state is the server's answer, which reaches the client after
 * hydration, so every full page load is `unknown` until it arrives. A handler
 * that branches on a signed-in boolean directly would open the login modal for an
 * actually-logged-in user who taps during that window (common on mobile —
 * the gated buttons sit right under the thumb on first paint).
 *
 * `authGate(action)` absorbs the race: before hydration it parks the
 * action and flips `authWaiting` (callers render `<AuthGateWaitModal>`);
 * once the session is known it replays the action — or calls `onGuest`
 * (open the login modal) if the visitor really is logged out. `unavailable` is
 * not an answer: the action stays parked until the session resolves.
 */
export function useAuthGate(onGuest: () => void): {
	authGate: (action: GatedAction) => void;
	authWaiting: boolean;
} {
	const { status, activeRole } = useAuth();
	const signedIn = status === "signedIn";
	const decided = status === "signedIn" || status === "guest";
	const [pending, setPending] = React.useState<GatedAction | null>(null);
	// Latest-callback ref: callers pass inline closures, and renders can
	// happen between the click and hydration completing.
	const onGuestRef = React.useRef(onGuest);
	onGuestRef.current = onGuest;

	React.useEffect(() => {
		if (!decided || !pending) return;
		setPending(null);
		if (signedIn) pending({ activeRole });
		else onGuestRef.current();
	}, [decided, signedIn, activeRole, pending]);

	const authGate = React.useCallback(
		(action: GatedAction) => {
			if (!decided) {
				setPending(() => action);
				return;
			}
			if (!signedIn) {
				onGuestRef.current();
				return;
			}
			action({ activeRole });
		},
		[decided, signedIn, activeRole],
	);

	return { authGate, authWaiting: pending !== null };
}
