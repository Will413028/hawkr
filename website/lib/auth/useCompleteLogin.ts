"use client";

import * as React from "react";
import { navigateDocument } from "@/lib/navigation/observability";
import { useAuth } from "./context";
import { redirectForRole } from "./redirect";
import type { Identity, RoleSlot } from "./storage";

/**
 * Final step of the login flow (SsoCallbackPage): apply the picked workspace
 * role, then leave with ONE hard navigation. A soft router.replace here would
 * keep the pre-login QueryClient and RSC cache — window.location.assign
 * re-runs the server with the new session in place (= what F5 does).
 */
export function useCompleteLogin() {
	const { commitIdentity, switchRole } = useAuth();
	const leavingRef = React.useRef(false);
	const completeLogin = React.useCallback(
		(redirectAfter: string, role: RoleSlot | null, identity: Identity) => {
			// A second completion in the same tick must not replace the first
			// navigation target.
			if (leavingRef.current) return;
			leavingRef.current = true;
			try {
				commitIdentity(identity);
				if (role) switchRole(role);
				if (
					!navigateDocument(redirectForRole(redirectAfter, role), {
						componentId: "login",
						actionId: "login_completed",
					})
				) {
					leavingRef.current = false;
				}
			} catch (error) {
				leavingRef.current = false;
				throw error;
			}
		},
		[commitIdentity, switchRole],
	);
	return { completeLogin };
}
