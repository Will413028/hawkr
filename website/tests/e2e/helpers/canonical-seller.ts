import type { APIRequestContext } from "@playwright/test";

/**
 * Maps the `store_id` the public product feed exposes to the storefront's
 * canonical path segment, its `store_code`.
 *
 * WHY EVERY SELLER FIXTURE NEEDS THIS — `/sellers/{store_id}` is not the
 * storefront, it is a `308` to the storefront (`middleware.ts` →
 * `lib/seller/canonical-redirect.ts`). A test that enters by `store_id`
 * therefore measures the redirect: its landing URL differs from the URL it
 * asked for, and a raw request sees the `308` rather than the document. Both
 * are correct behaviour, and neither is what a page-level assertion is about.
 *
 * TWO OUTCOMES THAT MUST NOT BE CONFLATED, because only one of them is safe to
 * fall back from:
 *
 *   - The backend answered, and the answer carries no usable `store_code`
 *     (`200` without one, or `404` for a seller that no longer exists). Then
 *     `null`, and the caller keeps the `store_id` it already has. What that id
 *     then serves depends on which of the two answers it was: a `200` without a
 *     code passes middleware through, so the storefront really does serve from
 *     the `store_id` path; a `404` is now answered as a real `404` rather than
 *     a 200 soft-404 (#376), so a fixture pointing at a seller that no longer
 *     exists fails on the status — which is the honest outcome, and names the
 *     stale fixture instead of hiding it behind an error page served as 200.
 *
 *   - We never got an answer (5xx, rate limit, timeout, socket error,
 *     unparseable body). Then THROW, after retrying. The tempting fallback is
 *     wrong here and used to be this helper's documented behaviour: our
 *     transport failing says nothing about middleware's, and middleware asks
 *     the gateway server-side, through its own memo and budget. It resolves the
 *     code, answers `308`, and every downstream page assertion fails against a
 *     redirect it cannot explain. That is how one flaky lookup renders as a
 *     wall of red "308" lines that read like the storefront is broken — the
 *     shape that cost a team half a day of blocked deploys. Failing here names
 *     the actual fault instead.
 *
 * Costs one extra request per distinct seller — retries are spent only on a
 * fault, never on the happy path, and the E2E proxy caches public GETs
 * (`app/api/v1/[...path]/route.ts`) so repeat lookups do not reach the shared
 * test gateway at all. `/public/sellers/{id}` accepts either key
 * (the seller module matches `store_id = $1 OR store_code = $1`), as do the
 * storefront collection endpoints, so every downstream fixture request can use
 * the canonical code.
 */

const ATTEMPTS = 3;

/** Same rule as the fixture helpers: only 5xx is worth asking twice. */
const RETRYABLE = (status: number) => status >= 500 || status === 429;

export async function canonicalSellerSegment(
	request: APIRequestContext,
	storeId: string,
): Promise<string | null> {
	const url = `/api/v1/public/sellers/${storeId}`;
	let lastFailure = "request was not attempted";

	for (let attempt = 1; attempt <= ATTEMPTS; attempt++) {
		try {
			const response = await request.get(url, {
				failOnStatusCode: false,
				timeout: 15_000,
			});
			const status = response.status();

			// Authoritative "this seller has no canonical segment". Middleware's
			// lookup reaches the same conclusion and passes the request through.
			if (status === 404) return null;

			if (status !== 200) {
				lastFailure = `${status} ${url}`;
				// A deterministic 4xx will answer the same way three times over, and
				// it means the caller handed us something that is not a seller key —
				// a broken fixture contract, not a transport fault.
				if (!RETRYABLE(status)) break;
				continue;
			}

			// A 200 whose body will not parse is a fault, not an answer: let it fall
			// into the catch and be retried rather than read as "no store_code".
			// camelCase only — the gateway's `PublicSellerInfo` DTO emits `storeCode`
			// (verified against test-api `/public/sellers/{id}`: the payload carries
			// `storeCode`, and `store_code` is not in it). Reading the retired name
			// resolved to `null` for every seller, so `?? storeId` handed each
			// fixture the store_id — the very key middleware 308s away from — and
			// eight `@live` seller checks went red on redirects that were correct
			// behaviour. That is the exact "wall of unexplained 308s" this file's
			// header warns about; it arrived through the field name, not transport.
			const envelope = (await response.json()) as {
				data?: { storeCode?: unknown };
			};
			const storeCode = String(envelope.data?.storeCode ?? "").trim();
			return /^\d+$/.test(storeCode) ? storeCode : null;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}

	throw new Error(
		`could not resolve the canonical segment of seller ${storeId} after ` +
			`${ATTEMPTS} attempts: ${lastFailure}. NOT falling back to the store_id: ` +
			`middleware resolves the store_code server-side and would 308 that url, ` +
			`so every assertion downstream would report a redirect instead of this.`,
	);
}
