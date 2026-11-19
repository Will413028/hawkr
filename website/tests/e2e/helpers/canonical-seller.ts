import type { APIRequestContext } from "@playwright/test";

/**
 * Maps the `store_id` the public product feed exposes to the storefront's path
 * segment, its `store_code`, through `SellerPublicService.BatchGetStores`.
 *
 * The storefront is keyed by `store_code` only: `/sellers/{store_id}` is a 404,
 * so a fixture that enters by `store_id` would assert against the 404 page.
 *
 * Two outcomes that must not be conflated:
 *
 *   - The backend answered and has no card for this id (never publicly visible,
 *     or gone): `null`, and the caller keeps the `store_id` it has. The storefront
 *     then answers 404, which names the stale fixture on the status.
 *   - We never got an answer (5xx, rate limit, timeout, unparseable body): THROW
 *     after retrying. Falling back to the `store_id` would turn one flaky lookup
 *     into a wall of 404 lines that read like the storefront is broken.
 */

const ATTEMPTS = 3;
const URL =
	"/api/connect/hawkr.seller.v1alpha1.SellerPublicService/BatchGetStores";

/** Same rule as the fixture helpers: only 5xx is worth asking twice. */
const RETRYABLE = (status: number) => status >= 500 || status === 429;

export async function canonicalSellerSegment(
	request: APIRequestContext,
	storeId: string,
): Promise<string | null> {
	let lastFailure = "request was not attempted";

	for (let attempt = 1; attempt <= ATTEMPTS; attempt++) {
		try {
			const response = await request.post(URL, {
				data: { ids: [storeId] },
				headers: { "Connect-Protocol-Version": "1" },
				failOnStatusCode: false,
				timeout: 15_000,
			});
			const status = response.status();
			if (status !== 200) {
				lastFailure = `${status} ${URL}`;
				if (!RETRYABLE(status)) break;
				continue;
			}
			const body = (await response.json()) as {
				stores?: { id?: unknown; storeCode?: unknown }[];
			};
			const card = body.stores?.find((s) => String(s.id) === storeId);
			const storeCode = String(card?.storeCode ?? "").trim();
			return /^\d+$/.test(storeCode) ? storeCode : null;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}

	throw new Error(
		`could not resolve the store_code of store ${storeId} after ` +
			`${ATTEMPTS} attempts: ${lastFailure}. NOT falling back to the store_id: ` +
			`the storefront is keyed by store_code and would answer 404.`,
	);
}
