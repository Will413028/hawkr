/**
 * Account resolution race (GetSession on every load), decoupled
 * from React so it's unit testable without a DOM rendering harness (see
 * __tests__).
 */

import { Code, ConnectError } from "@connectrpc/connect";
import { SESSION_UNAVAILABLE, type SessionResult } from "../api/auth";

// Bounded backoff (ms) for re-probing the account after the backend was
// momentarily unavailable on load (B-199). Bounded so a prolonged outage can't
// strand the UI on a skeleton forever: after the last delay still-unavailable,
// resolveSession gives up (onGaveUp) and the UI degrades to guest.
export const SESSION_RETRY_BACKOFF_MS = [1500, 3000, 6000] as const;

// Backstop for a slow answer. A *guest* is never gated — their 登入 link is
// live from first paint, because resolveAuthRailMode() renders the clickable
// guest rail while `authReady` is false (auth-rail.ts). The sign-in state comes
// from GetSession alone (the session cookie is HttpOnly), which can run up to
// apiTimeoutMs (15-30s) on a slow/cold backend. Un-gate after a human-scale
// wait so neither can strand the UI: onResolved/onUnavailable still apply the
// real answer whenever it lands — this only bounds how long the UI can look
// stuck. lib/auth/context.tsx starts this gate on mount, before GetSession answers.
export const HYDRATION_GATE_MS = 2500;

export interface ResolveSessionCallbacks {
	/**
	 * Fires once the probe settles with an identity, or null: the monolith
	 * refuses the session (a definitive guest).
	 */
	onResolved: (
		result: Exclude<SessionResult, typeof SESSION_UNAVAILABLE>,
	) => void;
	/** Backend momentarily unavailable — not a logout (B-199). */
	onUnavailable: () => void;
	onDenied?: (error: ConnectError) => void;
	/**
	 * Still unavailable after the last backoff. NOT an answer about the session:
	 * the UI stops asserting "checking", but nothing may end the session on it.
	 */
	onGaveUp: () => void;
}

/**
 * Resolve the signed-in account (GetSession; a visitor with no session resolves null);
 * returns a cancel function (effect cleanup).
 */
export function resolveSession(
	fetchSession: () => Promise<SessionResult>,
	callbacks: ResolveSessionCallbacks,
): () => void {
	let cancelled = false;
	let retryTimer: ReturnType<typeof setTimeout> | undefined;

	const run = async (attempt: number) => {
		let result: SessionResult;
		try {
			result = await fetchSession();
		} catch (error) {
			if (cancelled) return;
			const denied = ConnectError.from(error);
			if (denied.code === Code.PermissionDenied) {
				callbacks.onDenied?.(denied);
				return;
			}
			result = SESSION_UNAVAILABLE;
		}
		if (cancelled) return;
		if (result === SESSION_UNAVAILABLE) {
			callbacks.onUnavailable();
			const delay = SESSION_RETRY_BACKOFF_MS[attempt];
			if (delay !== undefined) {
				retryTimer = setTimeout(() => run(attempt + 1), delay);
			} else {
				// Backoff exhausted, still unavailable: stop asserting "checking" so
				// the rail can't hang on a skeleton indefinitely.
				callbacks.onGaveUp();
			}
			return;
		}
		callbacks.onResolved(result);
	};

	run(0);

	return () => {
		cancelled = true;
		if (retryTimer) clearTimeout(retryTimer);
	};
}
