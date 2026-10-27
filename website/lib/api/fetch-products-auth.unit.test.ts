// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { fetchProducts } from "./index";

afterEach(() => vi.unstubAllGlobals());

it.each([
	"followedProductOnly",
	"followedSellerOnly",
] as const)("sends Bearer for %s on the public products route", async (filter) => {
	const fetchMock = vi.fn(async () =>
		Response.json({ code: 0, data: { records: [] } }),
	);
	vi.stubGlobal("fetch", fetchMock);
	Object.assign(window, {
		Clerk: { loaded: true, session: { getToken: async () => "buyer-token" } },
	});
	await fetchProducts("", 1, 12, undefined, undefined, { [filter]: true });
	expect(fetchMock).toHaveBeenCalledOnce();
	const [url, init] = fetchMock.mock.calls[0] as unknown as [
		string,
		RequestInit,
	];
	expect(url).toContain("/public/products");
	expect(new Headers(init.headers).get("Authorization")).toBe(
		"Bearer buyer-token",
	);
});

it("keeps the anonymous products request without Bearer", async () => {
	const fetchMock = vi.fn(async () =>
		Response.json({ code: 0, data: { records: [] } }),
	);
	vi.stubGlobal("fetch", fetchMock);
	await fetchProducts("", 1, 12);
	const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
	expect(new Headers(init.headers).has("Authorization")).toBe(false);
	expect(init.credentials).toBe("omit");
});
