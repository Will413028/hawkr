import { HttpResponse, http } from "msw";
import { NextRequest } from "next/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { getEnv } from "@/lib/config/env";
import { siteMiddleware } from "@/middleware";
import { server } from "@/tests/mocks/server";
import {
	__resetSellerStoreCodeMemoForTest,
	isSellerKeyShape,
	resolveSellerRequest,
} from "./canonical-redirect";

/**
 * Storefront request handling (#365).
 *
 * Two layers are covered on purpose: the decision rules with an injected lookup
 * (cheap, exhaustive), and the real middleware against a mocked gateway (proves
 * the wire contract — a genuine 308, the query string, no Set-Cookie).
 */

/** 19-digit TSID store_id — the key every seller minted since the cutover has. */
const STORE_ID = "7487768729113814280";
/** 10-digit minted store_code (YY + 8 random digits). */
const STORE_CODE = "2662192214";
/**
 * The grandfathered pair that killed the shape short-circuit: a 10-digit
 * store_id whose freshly minted store_code is also 10 digits (dev fixture).
 */
const GRANDFATHER_STORE_ID = "2636840881";
const GRANDFATHER_STORE_CODE = "2649605557";

const SELLER_ENDPOINT = `${getEnv().serverApiBase}/public/sellers/:id`;

/** Mocks the gateway lookup and records which ids it was asked about. */
function mockSellerLookup(byId: Record<string, string>): { calls: string[] } {
	const calls: string[] = [];
	server.use(
		http.get(SELLER_ENDPOINT, ({ params }) => {
			const id = String(params.id);
			calls.push(id);
			const storeCode = byId[id];
			if (!storeCode) {
				// Byte-for-byte what the dev and prod gateways answer for an id
				// nobody holds — the `message` is load-bearing, see fetchSeller.
				return HttpResponse.json(
					{ code: 404, message: "seller not found", data: null },
					{ status: 404 },
				);
			}
			return HttpResponse.json({ code: 0, data: { storeCode } });
		}),
	);
	return { calls };
}

/**
 * Mocks the gateway being unable to answer — the branch that must stay fail-
 * OPEN now that a settled 404 fails closed (#376). `null` status = transport
 * failure (socket error / DNS / timeout).
 */
function mockSellerOutage(status: number | null): { calls: string[] } {
	const calls: string[] = [];
	server.use(
		http.get(SELLER_ENDPOINT, ({ params }) => {
			calls.push(String(params.id));
			return status === null
				? HttpResponse.error()
				: HttpResponse.json({ code: status, data: null }, { status });
		}),
	);
	return { calls };
}

/** Shorthand for the injected-lookup cases. */
const resolved = (storeCode: string | null) =>
	({ kind: "resolved", storeCode }) as const;

beforeEach(() => {
	__resetSellerStoreCodeMemoForTest();
});

describe("isSellerKeyShape", () => {
	it("accepts what the backend can parse as a key", () => {
		for (const id of [
			"1",
			STORE_CODE,
			GRANDFATHER_STORE_ID,
			"100000000001", // 12-digit legacy seed store_code
			STORE_ID,
			"9223372036854775807", // int64 max — backend answers 404, not 400
		]) {
			expect(isSellerKeyShape(id), id).toBe(true);
		}
	});

	it("rejects every shape the backend answers 400 for", () => {
		for (const id of [
			"not-an-id",
			"12a",
			"",
			"0", // backend: 400 invalid seller id
			"9999999999999999999", // 19 digits but past int64
			"9223372036854775808", // int64 max + 1
			"99999999999999999999", // 20 digits
			"9".repeat(40),
		]) {
			expect(isSellerKeyShape(id), id).toBe(false);
		}
	});
});

describe("resolveSellerRequest", () => {
	it("keeps the sub-path the visitor asked for", async () => {
		const lookup = vi.fn().mockResolvedValue(resolved(STORE_CODE));
		for (const tail of ["/reviews", "/posts", "/messages"]) {
			await expect(
				resolveSellerRequest(`/sellers/${STORE_ID}${tail}`, lookup),
			).resolves.toEqual({
				kind: "redirect",
				path: `/sellers/${STORE_CODE}${tail}`,
			});
		}
	});

	it("canonicalizes the id segment of a product detail path", async () => {
		const lookup = vi.fn().mockResolvedValue(resolved(STORE_CODE));
		await expect(
			resolveSellerRequest(`/sellers/${STORE_ID}/products/4004`, lookup),
		).resolves.toEqual({
			kind: "redirect",
			path: `/sellers/${STORE_CODE}/products/4004`,
		});
	});

	it("redirects the bare storefront path with no sub-path", async () => {
		const lookup = vi.fn().mockResolvedValue(resolved(STORE_CODE));
		await expect(
			resolveSellerRequest(`/sellers/${STORE_ID}`, lookup),
		).resolves.toEqual({ kind: "redirect", path: `/sellers/${STORE_CODE}` });
	});

	it("redirects a grandfathered seller whose store_id is store_code-shaped", async () => {
		// The reason there is no shape short-circuit: both keys are 10 digits, so
		// skipping the lookup on 10-digit ids left this seller un-canonicalized.
		const lookup = vi.fn().mockResolvedValue(resolved(GRANDFATHER_STORE_CODE));
		await expect(
			resolveSellerRequest(`/sellers/${GRANDFATHER_STORE_ID}/reviews`, lookup),
		).resolves.toEqual({
			kind: "redirect",
			path: `/sellers/${GRANDFATHER_STORE_CODE}/reviews`,
		});
		expect(lookup).toHaveBeenCalledWith(GRANDFATHER_STORE_ID);
	});

	it("resolves a store_code-shaped id instead of assuming it is canonical", async () => {
		const lookup = vi.fn().mockResolvedValue(resolved(STORE_CODE));
		await expect(
			resolveSellerRequest(`/sellers/${STORE_CODE}/reviews`, lookup),
		).resolves.toEqual({ kind: "pass" });
		expect(lookup).toHaveBeenCalledWith(STORE_CODE);
	});

	it("falls through for a seller that has no store_code", async () => {
		const lookup = vi.fn().mockResolvedValue(resolved(null));
		await expect(
			resolveSellerRequest(`/sellers/${STORE_ID}/reviews`, lookup),
		).resolves.toEqual({ kind: "pass" });
	});

	it("rejects an id the backend settled as absent", async () => {
		const lookup = vi.fn().mockResolvedValue({ kind: "absent" });
		for (const tail of ["", "/reviews", "/products/4004"]) {
			await expect(
				resolveSellerRequest(`/sellers/${STORE_ID}${tail}`, lookup),
			).resolves.toEqual({ kind: "not-found" });
		}
	});

	it("falls through when the backend could not answer at all", async () => {
		// The half of #376 that must NOT 404: an unreachable backend says nothing
		// about whether the seller exists, so the request degrades to the
		// pre-#376 behaviour rather than taking every storefront offline.
		const lookup = vi.fn().mockResolvedValue({ kind: "unknown" });
		await expect(
			resolveSellerRequest(`/sellers/${STORE_ID}/reviews`, lookup),
		).resolves.toEqual({ kind: "pass" });
	});

	it("rejects a malformed id without asking the backend", async () => {
		const lookup = vi.fn();
		for (const id of [
			"not-an-id",
			"0",
			"99999999999999999999",
			"9".repeat(21),
		]) {
			await expect(
				resolveSellerRequest(`/sellers/${id}/reviews`, lookup),
			).resolves.toEqual({ kind: "not-found" });
		}
		expect(lookup).not.toHaveBeenCalled();
	});

	it("ignores paths outside the storefront subtree", async () => {
		const lookup = vi.fn();
		for (const pathname of [
			"/sellers",
			"/sellers/",
			"/market/products",
			`/products/${STORE_ID}`,
		]) {
			await expect(resolveSellerRequest(pathname, lookup)).resolves.toEqual({
				kind: "pass",
			});
		}
		expect(lookup).not.toHaveBeenCalled();
	});
});

describe("middleware storefront handling", () => {
	/**
	 * A pass is next-intl's rewrite of the unprefixed URL to its zh-TW route (`app/[locale]`),
	 * never the 404 rewrite.
	 */
	function expectServedAsZhTw(response: Response) {
		const rewrite = response.headers.get("x-middleware-rewrite") ?? "";
		expect(new URL(rewrite).pathname).toMatch(/^\/zh-TW\/sellers\//);
		expect(rewrite).not.toContain("/_not-found");
	}

	function get(path: string) {
		return siteMiddleware(
			new NextRequest(new URL(`https://web-dev.example.com${path}`)),
			async () => ({ userId: null }),
		);
	}

	it("answers a real 308 whose Location carries the sub-path", async () => {
		mockSellerLookup({ [STORE_ID]: STORE_CODE });
		const response = await get(`/sellers/${STORE_ID}/reviews`);
		expect(response.status).toBe(308);
		expect(new URL(response.headers.get("location") ?? "").pathname).toBe(
			`/sellers/${STORE_CODE}/reviews`,
		);
		// A canonical redirect must not mutate the visitor's session state.
		expect(response.headers.get("set-cookie")).toBeNull();
	});

	it("preserves the query string across the redirect", async () => {
		mockSellerLookup({ [STORE_ID]: STORE_CODE });
		const response = await get(
			`/sellers/${STORE_ID}/reviews?page=2&utm_source=line`,
		);
		expect(response.status).toBe(308);
		const location = new URL(response.headers.get("location") ?? "");
		expect(location.pathname).toBe(`/sellers/${STORE_CODE}/reviews`);
		expect(location.search).toBe("?page=2&utm_source=line");
	});

	it("redirects the grandfathered seller the shape check used to miss", async () => {
		mockSellerLookup({ [GRANDFATHER_STORE_ID]: GRANDFATHER_STORE_CODE });
		const response = await get(`/sellers/${GRANDFATHER_STORE_ID}`);
		expect(response.status).toBe(308);
		expect(new URL(response.headers.get("location") ?? "").pathname).toBe(
			`/sellers/${GRANDFATHER_STORE_CODE}`,
		);
	});

	it("passes a canonical request through", async () => {
		const { calls } = mockSellerLookup({ [STORE_CODE]: STORE_CODE });
		const response = await get(`/sellers/${STORE_CODE}`);
		expect(response.status).toBe(200);
		expect(response.headers.get("location")).toBeNull();
		expect(response.headers.get("set-cookie")).toBeNull();
		expect(calls).toEqual([STORE_CODE]);
	});

	it("resolves a given id once and serves later hits from the memo", async () => {
		const { calls } = mockSellerLookup({ [STORE_ID]: STORE_CODE });
		for (const tail of ["", "/posts", "/reviews"]) {
			const response = await get(`/sellers/${STORE_ID}${tail}`);
			expect(response.status).toBe(308);
		}
		expect(calls).toEqual([STORE_ID]);
	});

	it("answers a real 404 for a well-formed id no seller holds", async () => {
		// #376: the backend settles this as 404, and without this the prerendered
		// subtree serves its "找不到賣家" body under a 200 — a soft-404.
		mockSellerLookup({});
		for (const tail of ["", "/reviews", "/products/4004"]) {
			const response = await get(`/sellers/${STORE_ID}${tail}`);
			expect(response.status, tail).toBe(404);
			expect(response.headers.get("x-middleware-rewrite"), tail).toContain(
				"/_not-found",
			);
			expect(response.headers.get("set-cookie"), tail).toBeNull();
		}
	});

	it.each([
		500,
		503,
		null,
	])("serves the page when the gateway cannot answer (%s)", async (status) => {
		// The fail-OPEN half. A blip must degrade to the old soft-404, never
		// 404 a storefront that exists.
		const { calls } = mockSellerOutage(status);
		const response = await get(`/sellers/${STORE_ID}/reviews`);
		expect(response.status).toBe(200);
		expect(response.headers.get("location")).toBeNull();
		expectServedAsZhTw(response);
		// The gateway really was asked — otherwise this 200 would prove
		// nothing about how a failed answer is treated.
		expect(calls).toContain(STORE_ID);
	});

	it("serves the page when a 404 came from the router, not the handler", async () => {
		// Gin's plain-text `404 page not found` — the shape a dropped or renamed
		// gateway route produces. Reading it as "seller absent" would 404 every
		// storefront on the site off one bad deploy.
		server.use(
			http.get(SELLER_ENDPOINT, () =>
				HttpResponse.text("404 page not found", { status: 404 }),
			),
		);
		const response = await get(`/sellers/${STORE_ID}/reviews`);
		expect(response.status).toBe(200);
		expectServedAsZhTw(response);
	});

	it("does not memoise an absent id, so a later seller can claim it", async () => {
		mockSellerLookup({});
		expect((await get(`/sellers/${STORE_ID}`)).status).toBe(404);
		// Same isolate, same id — now minted.
		mockSellerLookup({ [STORE_ID]: STORE_CODE });
		const response = await get(`/sellers/${STORE_ID}`);
		expect(response.status).toBe(308);
		expect(new URL(response.headers.get("location") ?? "").pathname).toBe(
			`/sellers/${STORE_CODE}`,
		);
	});

	it("answers 404 for an id the backend would reject, with no gateway call", async () => {
		const { calls } = mockSellerLookup({});
		for (const id of ["not-an-id", "0", "99999999999999999999"]) {
			const response = await get(`/sellers/${id}`);
			expect(response.status, id).toBe(404);
			// Rewritten rather than a bare body: the visitor still gets the app's
			// 404 page, while the status comes from here.
			expect(response.headers.get("x-middleware-rewrite"), id).toContain(
				"/_not-found",
			);
		}
		expect(calls).toEqual([]);
	});
});
