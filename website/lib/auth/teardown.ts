import type { QueryClient } from "@tanstack/react-query";

// The one namespace whose cache is identity-free and survives an identity
// change. Everything else embeds per-viewer state — feed `is_liked`, followed
// lists, order history, etc. — and must be dropped so the next viewer can't
// see the previous account's data.
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
 *     so the first session query result of a cold-loaded logged-in session
 *     hits this case. Firing here needlessly `cancelQueries()` +
 *     `removeQueries()` the in-flight PUBLIC `home.snapshot` query while its
 *     observer stays mounted, which never repopulates → the content skeleton
 *     locks (hawkr-website#281). Guests never trip this: `undefined →
 *     undefined` compares equal, so no teardown, which is exactly why the bug
 *     only ever reproduced for logged-in users.
 *   - Same-account backfill (account → same account): a cold-load buyer session
 *     updates twice for one account (the session probe, then
 *     the /buyers/me nickname/avatar backfill). Tearing down on that second,
 *     same-account event flushed every just-mounted query back to loading — the
 *     "flash back to skeleton" half of hawkr-website#259.
 */
export function shouldTeardownPrivateCache(
	prevAccountId: string | undefined,
	nextAccountId: string | undefined,
): boolean {
	// No prior real account → nothing private to tear down (first landing).
	if (prevAccountId === undefined) return false;
	// A different account (switch) or no account (logout) replacing a real one.
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
	// One structured line per genuine switch/logout (never on a first landing or
	// same-account backfill). If a logged-in homepage ever strands on the content
	// skeleton again, this pins whether a private-cache purge fired next to it —
	// the forensic hook hawkr-website#281 lacked at repro time.
	console.warn("[auth] private cache purged on identity change", {
		removedQueryCount: purged.length,
		removedNamespaces: [...new Set(purged.map((q) => String(q.queryKey[0])))],
	});
}
