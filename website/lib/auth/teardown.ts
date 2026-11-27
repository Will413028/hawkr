import type { QueryClient } from "@tanstack/react-query";

const IDENTITY_FREE_NAMESPACE = "products";

function isIdentityBoundQuery(query: {
	queryKey: readonly unknown[];
}): boolean {
	return (
		query.queryKey[0] !== IDENTITY_FREE_NAMESPACE &&
		query.queryKey[0] !== "session"
	);
}

/**
 * Whether an identity transition warrants tearing down the previous viewer's
 * private cache.
 *
 * Tears down ONLY when a prior *real* account is being replaced (a soft-nav
 * account switch) or dropped (logout). Two transitions must NOT tear down, and
 * getting either wrong strands the homepage on its content skeleton:
 *
 *   - First identity landing (guest/none → account): there is no previous
 *     viewer whose private cache could leak. `accountIdRef` starts `undefined`,
 *     so the server's first answer for a cold-loaded logged-in session
 *     hits this case. Firing here needlessly `cancelQueries()` +
 *     `removeQueries()` the in-flight PUBLIC `home.snapshot` query while its
 *     observer stays mounted, which never repopulates → the content skeleton
 *     locks (hawkr-website#281). Guests never trip this: `undefined →
 *     undefined` compares equal, so no teardown, which is exactly why the bug
 *     only ever reproduced for logged-in users.
 *   - Same-account backfill (account → same account): a cold-load buyer session
 *     gets a second answer for one account (a refreshed server answer
 *     after a profile change). Tearing down on that second,
 *     same-account event flushed every just-mounted query back to loading — the
 *     "flash back to skeleton" half of hawkr-website#259.
 */
export function shouldTeardownPrivateCache(
	prevAccountId: string | undefined,
	nextAccountId: string | undefined,
): boolean {
	if (prevAccountId === undefined) return false;
	return nextAccountId !== prevAccountId;
}

/**
 * Drop the previous identity's private caches, keeping only the identity-free
 * catalogue to avoid a needless refetch storm. In-flight queries are cancelled
 * first so a late resolution can't repopulate a cache we're about to clear.
 *
 * Only call when {@link shouldTeardownPrivateCache} returns true — invoking it
 * on a first landing or a same-account event is exactly the #281 / #259
 * regression.
 */
export async function teardownPrivateCache(
	queryClient: QueryClient,
): Promise<void> {
	await queryClient.cancelQueries();
	const purged = queryClient.getQueryCache().findAll({
		predicate: isIdentityBoundQuery,
	});
	queryClient.removeQueries({ predicate: isIdentityBoundQuery });
	console.warn("[auth] private cache purged on identity change", {
		removedQueryCount: purged.length,
		removedNamespaces: [...new Set(purged.map((q) => String(q.queryKey[0])))],
	});
}
