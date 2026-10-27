"use client";

import { useMemo } from "react";
import { useAuth } from "@/lib/auth/context";
import type { QueryIdentity } from "@/lib/query/keys";

/**
 * Identity scope for private TanStack Query keys. Pairing the account id with
 * the active role means a logout or an account/role switch lands on a fresh
 * cache slot instead of serving the previous identity's data.
 *
 * Memoised: a fresh object each render would break every `useMemo`/`useEffect`/
 * `useCallback` that lists this identity (or a key derived from it) as a dep —
 * churning WS subscriptions and re-running cache-sync effects every render.
 */
export function useQueryIdentity(): QueryIdentity {
	const { identity, activeRole } = useAuth();
	const accountId = identity?.accountId ?? null;
	return useMemo(() => ({ accountId, activeRole }), [accountId, activeRole]);
}
