"use client";

import { useMemo } from "react";
import { useAuth } from "@/lib/auth/context";
import { useOptionalZone } from "@/lib/auth/zone-context";
import type { QueryIdentity } from "@/lib/query/keys";

/**
 * Identity scope for private TanStack Query keys. Pairing the account id with
 * the active role means a logout or an account/role switch lands on a fresh
 * cache slot instead of serving the previous identity's data.
 *
 * Inside a protected zone the account id is the server-verified zone identity
 * and the role is the zone's workspace (a zone with no workspace uses the
 * client active role), so the first render already has the final key.
 *
 * Memoised: a fresh object each render would break every `useMemo`/`useEffect`/
 * `useCallback` that lists this identity (or a key derived from it) as a dep —
 * churning WS subscriptions and re-running cache-sync effects every render.
 */
export function useQueryIdentity(): QueryIdentity {
	const { identity, activeRole } = useAuth();
	const zone = useOptionalZone();
	const accountId = zone
		? zone.identity.accountId
		: (identity?.accountId ?? null);
	const role = zone?.workspace ?? activeRole;
	return useMemo(() => ({ accountId, activeRole: role }), [accountId, role]);
}
