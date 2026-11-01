import type { AuthStatus } from "./session-state";

/**
 * Which auth affordance the navbar's right rail shows. Single decision point so
 * the "登入 link vs skeleton vs member menu" choice can't drift between the places
 * the JSX branches.
 */
export type AuthRailMode = "skeleton" | "guest" | "buyer" | "seller";

export interface AuthRailInput {
	/** The server's session answer; `unknown` until it arrives. */
	status: AuthStatus;
	isLoggedIn: boolean;
	activeRole: "buyer" | "seller" | null;
}

export function resolveAuthRailMode({
	status,
	isLoggedIn,
	activeRole,
}: AuthRailInput): AuthRailMode {
	// Neither `unknown` nor `unavailable` (B-199) is a claim that the visitor is a
	// guest or a member: hold the neutral skeleton instead of a sign-in button or
	// an avatar.
	if (status === "unknown" || status === "unavailable") return "skeleton";
	if (!isLoggedIn) return "guest";
	return activeRole === "buyer" ? "buyer" : "seller";
}
