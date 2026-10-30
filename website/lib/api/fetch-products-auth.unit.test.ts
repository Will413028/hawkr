// @vitest-environment jsdom
import { afterEach, expect, it, vi } from "vitest";
import { fetchProducts } from "./index";

afterEach(() => vi.unstubAllGlobals());

it.each([
	"followedProductOnly",
	"followedSellerOnly",
] as const)("sends the session cookie, not a Bearer, for %s on the public products route", async (filter) => {
	const fetchMock = vi.fn(async () =>
		Response.json({ code: 0, data: { records: [] } }),
	);
	vi.stubGlobal("fetch", fetchMock);
	await fetchProducts("", 1, 12, undefined, undefined, { [filter]: true });
	expect(fetchMock).toHaveBeenCalledOnce();
	const [url, init] = fetchMock.mock.calls[0] as unknown as [
		string,
		RequestInit,
	];
	expect(url).toContain("/public/products");
	expect(new Headers(init.headers).has("Authorization")).toBe(false);
	expect(init.credentials).toBe("include");
});

it("keeps the anonymous products request without credentials", async () => {
	const fetchMock = vi.fn(async () =>
		Response.json({ code: 0, data: { records: [] } }),
	);
	vi.stubGlobal("fetch", fetchMock);
	await fetchProducts("", 1, 12);
	const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
	expect(new Headers(init.headers).has("Authorization")).toBe(false);
	expect(init.credentials).toBe("omit");
});
