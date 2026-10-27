/**
 * Request-shape decisions for the `/sellers/{id}` subtree, for `middleware.ts`:
 * canonicalize the id segment, and reject an id no seller could ever have.
 *
 * A storefront's canonical path segment is its `store_code`; a `store_id` (the
 * key the public product feed and every not-yet-migrated internal href still
 * expose, plus old bookmarks and whatever search engines already indexed) has
 * to be sent to it.
 *
 * WHY MIDDLEWARE AND NOT THE PAGE (issue #365) — the four tab pages used to
 * call `permanentRedirect()` themselves. Every one of them is statically
 * prerendered (`export const revalidate = 1800` + `generateStaticParams`), and
 * a prerender produces a cached HTML document, not a live response: there is no
 * status line or Location header for the thrown redirect to occupy, so Next
 * baked it into the document body as
 * `<meta id="__next-page-redirect" http-equiv="refresh">`. On the wire that is
 * `200` + `x-nextjs-prerender: 1`, which means
 *   - the visitor loaded two full documents (measured on a reviews tab against
 *     the dev backend: 2 documents / 60 image requests / 1 aborted document
 *     entering by store_id, versus 1 / 30 / 0 by store_code), and
 *   - crawlers and link-preview bots stopped at the 200, so the canonical
 *     consolidation the redirect existed for never reached them.
 * Emitting a live response from the pages themselves would require
 * request-time input, which opts the whole seller subtree out of static
 * rendering — the regression #338 removed. Middleware runs before the
 * prerendered document is served and can answer with a real status, so it is
 * the one layer that fixes this without touching the pages' static-ness. The
 * same argument is why the malformed-id rejection lives here: `notFound()` from
 * inside this subtree renders the 404 body but answers `200` (measured on a
 * cache MISS as well as a HIT), so a real 404 has to come from here too.
 *
 * WHY THE NON-EXISTENT SELLER IS REJECTED HERE TOO (issue #376) — the same
 * `200` applied to an id that is well-formed but belongs to nobody, which is
 * the textbook soft-404: measured on prod, `/sellers/9223372036854775807`
 * answered `200` with a "找不到賣家" body and two contradicting robots tags in
 * one document (`index, follow` from the root metadata, then the `noindex`
 * Next injects at the `notFound()` boundary). The objection that kept this out
 * of #365 was that "no store_code" and "no such seller" are different facts and
 * only the second may 404 — a backend blip must never take every storefront
 * down with it. They are different facts, and they are also separable ones:
 * `seller-service` answers a settled `404 {"code":404,"message":"seller not
 * found"}` for an id nobody holds, while an outage is a 5xx, a timeout or a
 * socket error. `fetchSeller` splits exactly there, so this file fails CLOSED
 * on the authoritative answer and OPEN on every no-answer — rather than picking
 * one of the two globally. It costs no extra request either: every well-formed
 * id was already being resolved for canonicalization.
 */
import { apiClient } from "@/lib/api/client";
import { canonicalSellerRedirectTarget } from "@/lib/seo/urls";

/** `/sellers/{segment}` plus whatever the request asked for after it. */
const SELLER_PATH = /^\/sellers\/([^/]+)(\/.*)?$/;

/**
 * Largest value the backend will parse as a seller key. Both keys are numeric
 * and share one query parameter
 * (`seller-service/db/queries/sellers.sql`, `GetPublicSellerProfileBySellerAccountID`
 * matches `store_id = $1 OR store_code = $1`), so anything outside positive
 * signed-64-bit range answers `400 invalid seller id` — never `404`.
 */
const MAX_SELLER_KEY = "9223372036854775807";

/**
 * Budget for the store_code lookup. Deliberately far below `env.apiTimeoutMs`
 * (15–30s): this sits in front of a document the visitor is waiting for, and
 * failing to canonicalize is recoverable (the request falls through and the
 * page renders) while a stalled middleware is not. `apiClient` composes this as
 * the budget for the whole call including its idempotent retry.
 */
const LOOKUP_TIMEOUT_MS = 1_500;

/**
 * Resolved `id → store_code`, memoised per isolate.
 *
 * Load-bearing rather than an optimisation: every well-formed id is resolved
 * (see `resolveSellerRequest`), so without this each storefront request would
 * carry its own gateway lookup. A store_code is immutable once minted, so a hit
 * can be trusted; the TTL only bounds how long a mapping survives a seller
 * being closed and re-opened as a new stint.
 */
const storeCodeMemo = new Map<string, { code: string | null; at: number }>();
const MEMO_TTL_MS = 30 * 60_000;
const MEMO_MAX_ENTRIES = 500;

/** Test-only: drops the per-isolate memo so cases cannot leak into each other. */
export function __resetSellerStoreCodeMemoForTest(): void {
	storeCodeMemo.clear();
}

/**
 * What the backend was able to tell us about an id. Three outcomes rather than
 * two: "this seller does not exist" is authoritative and must produce a 404,
 * while "we could not ask" must fall through — collapsing them into one
 * nullable store_code is what forced the old fail-open-on-everything.
 */
export type SellerLookup =
	/** The backend answered about this id. `storeCode` is null when it has none. */
	| { kind: "resolved"; storeCode: string | null }
	/** Settled `404` — nobody holds this id. */
	| { kind: "absent" }
	/** No usable answer: 5xx, timeout, socket error, unreadable body. */
	| { kind: "unknown" };

const ABSENT: SellerLookup = { kind: "absent" };
const UNKNOWN: SellerLookup = { kind: "unknown" };

/** What middleware should do with a storefront request. */
export type SellerRequestOutcome =
	| { kind: "pass" }
	| { kind: "redirect"; path: string }
	| { kind: "not-found" };

const PASS: SellerRequestOutcome = { kind: "pass" };

/** Splits a storefront path into its id segment and the sub-path after it. */
export function parseSellerPath(
	pathname: string,
): { id: string; tail: string } | null {
	const match = SELLER_PATH.exec(pathname);
	if (!match) return null;
	return { id: match[1], tail: match[2] ?? "" };
}

/**
 * True when `id` could be a seller key at all — i.e. when asking the backend
 * about it can yield a seller or an honest `404`, rather than the `400` that
 * this subtree turns into a crawler-visible `500`.
 *
 * Compared as a string rather than a number: the range boundary is past
 * `Number.MAX_SAFE_INTEGER`, and BigInt literals are unavailable at this
 * tsconfig target. For two digit strings of EQUAL length, lexicographic order
 * is numeric order — hence the length check first. A longer segment is rejected
 * by the regex, so an arbitrarily long path costs nothing.
 */
export function isSellerKeyShape(id: string): boolean {
	if (!/^\d{1,19}$/.test(id)) return false;
	// Zero is not a key either: the backend answers 400 for it, not 404.
	if (/^0+$/.test(id)) return false;
	return id.length < MAX_SELLER_KEY.length || id <= MAX_SELLER_KEY;
}

async function fetchSeller(
	id: string,
	visitorIp?: string | null,
): Promise<SellerLookup> {
	try {
		const { code, data, message } = await apiClient.public.get<{
			storeCode?: unknown;
		} | null>(`/sellers/${id}`, {
			credentials: "omit",
			// This lookup is made on behalf of the visitor whose request is being
			// canonicalized, so it is charged to that person's rate-limit subject
			// rather than to the site's shared SSR one (backend#2264). Middleware is
			// one of the two places on the site that can see who that is.
			visitorIp,
			// `softFailNotFound`, not `softFail` — this single flag is what makes a
			// real 404 reachable at all. `softFail` flattens 404, 5xx, timeout and
			// socket error into one null-data envelope, and a decision that cannot
			// see the difference has no choice but to fail open on all four. Its
			// narrower sibling softens ONLY a settled 404 and lets every no-answer
			// throw, so the `catch` below IS the outage branch, precisely.
			softFailNotFound: true,
			timeoutMs: LOOKUP_TIMEOUT_MS,
		});
		// Reachable only from that 404 soften: a 2xx whose envelope carries a
		// non-zero code has already thrown by the time we get here.
		//
		// The `message` guard separates seller-service's answer from the gateway's
		// router. Only the former is authoritative. A 404 that never reached the
		// handler — route dropped or renamed by a deploy — comes back as Gin's
		// plain-text `404 page not found`, which fails to parse as an envelope and
		// so arrives here with no message; seller-service's own answer always
		// carries one (`{"code":404,"message":"seller not found"}`, verified
		// against both the dev and prod gateways). Without this, a single
		// mis-routed deploy would 404 every storefront on the site at once. Note
		// which way it fails: an unrecognisable 404 degrades to "unknown", i.e.
		// open.
		if (code === 404) return message ? ABSENT : UNKNOWN;
		// camelCase: this endpoint's DTO is the gateway's `PublicSellerInfo`
		// (`gateway/internal/handler/seller.go`, `json:"storeCode"` since
		// backend#2235), and the same-origin `/api/v1/[...path]` proxy forwards
		// the body verbatim. Reading the retired `store_code` here would make
		// every storefront look code-less and cost a 308 on every entry.
		const storeCode = String(data?.storeCode ?? "").trim();
		return {
			kind: "resolved",
			storeCode: /^\d+$/.test(storeCode) ? storeCode : null,
		};
	} catch {
		// A 5xx (after its retry), a timeout, a socket error, an unreadable body,
		// or a 4xx that is not 404. None of these say the seller is missing, so
		// none of them may 404 a storefront.
		return UNKNOWN;
	}
}

/** Resolves what the backend knows about an id, memoised. */
export async function lookupSeller(
	id: string,
	visitorIp?: string | null,
): Promise<SellerLookup> {
	const hit = storeCodeMemo.get(id);
	if (hit && Date.now() - hit.at < MEMO_TTL_MS)
		return { kind: "resolved", storeCode: hit.code };

	const result = await fetchSeller(id, visitorIp);
	// Every RESOLVED answer is memoised, including "this seller exists and has no
	// store_code" — that answer is just as authoritative as one carrying a code,
	// and leaving it out meant such an id paid a gateway lookup on every single
	// request while a coded one paid roughly one per isolate per TTL (backend#2264:
	// the middleware lookup is per-request fan-out, and one uncacheable class of id
	// is enough to keep it that way). The bounded cost is that a store_code minted
	// during the window is not honoured until the entry expires: the storefront
	// still renders, it is only the canonical 308 that waits.
	//
	// Still never "absent": pinning a 404 would make a seller created later at that
	// id inherit it for the whole TTL, which is a wrong ANSWER rather than a late
	// one. "unknown" is likewise never memoised — a backend blip must not be frozen
	// into a verdict.
	if (result.kind === "resolved") {
		if (storeCodeMemo.size >= MEMO_MAX_ENTRIES) storeCodeMemo.clear();
		storeCodeMemo.set(id, { code: result.storeCode, at: Date.now() });
	}
	return result;
}

/**
 * Decides a storefront request: reject a malformed id, send a non-canonical one
 * to its store_code, otherwise pass through.
 *
 * EVERY well-formed id is resolved — there is deliberately no shape-based
 * short-circuit for "this already looks like a store_code". Shape cannot decide
 * it: a minted store_code is `YY` + 8 random digits (10 digits, backend
 * `seller-service/internal/domain/store_code.go`), but a grandfathered seller's
 * `store_id` equals its `account_id`, which is also 10 digits, and its
 * store_code was minted fresh whenever there was no PayUni merchant_code to
 * copy — dev holds exactly that seller (`store_id 2636840881` → `store_code
 * 2649605557`, both 10 digits). Short-circuiting 10-digit ids would leave that
 * class silently un-canonicalized, which is the hardest kind of defect to
 * notice. The memo is what keeps resolving everything at roughly one lookup per
 * seller per isolate per TTL.
 *
 * `lookup` is injectable so the rules can be exercised without a backend — and
 * it is also how middleware binds the visitor's IP to the lookup
 * (`(id) => lookupSeller(id, visitorIp)`), which keeps the rules here free of a
 * request-shaped argument they have no use for.
 */
export async function resolveSellerRequest(
	pathname: string,
	lookup: (id: string) => Promise<SellerLookup> = lookupSeller,
): Promise<SellerRequestOutcome> {
	const parsed = parseSellerPath(pathname);
	if (!parsed) return PASS;
	const { id, tail } = parsed;
	if (!isSellerKeyShape(id)) return { kind: "not-found" };

	const result = await lookup(id);
	// The backend said nobody holds this id. Answering 404 here is the only way
	// the subtree can say so on the wire (#376) — `notFound()` from inside a
	// prerender still ships a 200.
	if (result.kind === "absent") return { kind: "not-found" };
	// "unknown" keeps the pre-#376 behaviour: no canonicalization, no rejection,
	// the page renders. A blip must degrade to today's soft-404, not to an
	// outage that takes every storefront with it.
	const storeCode = result.kind === "resolved" ? result.storeCode : null;
	const target = canonicalSellerRedirectTarget(id, storeCode, tail);
	return target ? { kind: "redirect", path: target } : PASS;
}
