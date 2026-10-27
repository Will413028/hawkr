/**
 * Which auth affordance the navbar's right rail shows. Single decision point so
 * the "登入 link vs skeleton vs member menu" choice can't drift between the two
 * places the JSX used to branch (that drift was the login-button-dead bug: a
 * client-only `bootRole` flipped the clickable 登入 <Link> into an inert
 * skeleton <span> and then held it there for the whole GetSession round-trip).
 */
export type AuthRailMode = "skeleton" | "guest" | "buyer" | "seller";

export interface AuthRailInput {
	/** The sign-in state has settled (or its gate elapsed). */
	authReady: boolean;
	isLoggedIn: boolean;
	activeRole: "buyer" | "seller" | null;
	/**
	 * The account probe SETTLED but was inconclusive (backend 5xx/network — see
	 * B-199 / SESSION_UNAVAILABLE). This is NOT "resolved guest": `isLoggedIn`
	 * still reflects whatever identity was known before the probe, so trusting
	 * it here could flash a wrong guest/member state (hawkr-website#259 —
	 * the 登入 link appearing then disappearing again once a background retry
	 * lands).
	 */
	sessionUnavailable: boolean;
}

export function resolveAuthRailMode({
	authReady,
	isLoggedIn,
	activeRole,
	sessionUnavailable,
}: AuthRailInput): AuthRailMode {
	if (sessionUnavailable) {
		// Once the probe has actually failed, do not claim the visitor is a guest.
		// The navbar is intentionally static on Workers (no request-time cookie
		// read), so an inconclusive probe must use the neutral skeleton until a
		// background retry can establish the real identity.
		return "skeleton";
	}
	if (!authReady) {
		// The navbar is static on Workers, so no request-time cookie read exists to
		// tell a returning member from a first-time visitor here. Render the
		// clickable guest rail rather than an inert placeholder: a visitor who
		// cannot click 登入 is the worse of the two wrong states, and a member's
		// rail swaps in as soon as the probe settles.
		return "guest";
	}
	// Probe settled — the authoritative answer. A stale cookie that looked like a
	// session resolves to guest here (isLoggedIn === false), swapping the
	// skeleton for the clickable 登入 link.
	if (!isLoggedIn) return "guest";
	return activeRole === "buyer" ? "buyer" : "seller";
}
