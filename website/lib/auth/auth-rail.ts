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
	activeRole: "buyer" | "seller" | null;
}

export function resolveAuthRailMode({
	status,
	activeRole,
}: AuthRailInput): AuthRailMode {
	if (status === "unknown" || status === "unavailable") return "skeleton";
	if (status === "guest") return "guest";
	return activeRole === "buyer" ? "buyer" : "seller";
}
