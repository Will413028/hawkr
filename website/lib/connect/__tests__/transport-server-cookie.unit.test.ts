import { createClient } from "@connectrpc/connect";
import { IdentityAppService } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { authedTransport } from "../transport";

const fetchMock = vi.fn<typeof fetch>();
const COOKIE = "__Host-hk_session=opaque";

beforeEach(() => {
	fetchMock.mockImplementation(
		async () =>
			new Response("{}", {
				status: 200,
				headers: { "content-type": "application/json" },
			}),
	);
	vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => {
	vi.unstubAllGlobals();
	fetchMock.mockReset();
});

describe("authedTransport on the server (RSC, server action)", () => {
	it("a read (GET) carries the cookie the caller handed over, never a Bearer", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW", cookie: COOKIE }),
		).getSession({});
		const [, init] = fetchMock.mock.calls[0];
		expect(init?.method).toBe("GET");
		const headers = new Headers(init?.headers);
		expect(headers.get("Cookie")).toBe(COOKIE);
		expect(headers.has("Authorization")).toBe(false);
	});

	it("a write (POST) is not given the cookie: the monolith's CSRF check would refuse it, and writes go from the browser", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW", cookie: COOKIE }),
		).signOut({});
		const [, init] = fetchMock.mock.calls[0];
		expect(init?.method).toBe("POST");
		expect(new Headers(init?.headers).has("Cookie")).toBe(false);
	});

	it("without a handed-over cookie nothing is sent", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		).getSession({});
		const [, init] = fetchMock.mock.calls[0];
		expect(new Headers(init?.headers).has("Cookie")).toBe(false);
	});
});
