// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AuthUnavailableError } from "../auth/clerk-token";
import { checkFavorite, toggleFavorite } from "../favorites/repo";
import { toggleFollow } from "../follow/repo";
import { confirmOrderReceipt } from "../orders/repo";
import { toggleCommentLike } from "../social/repo";
import { SseClient } from "../sse/client";
import { AUTH_EXPIRED_EVENT, apiClient, UnauthorizedError } from "./client";

const fetchMock = vi.fn();
beforeEach(() => {
	vi.stubGlobal("fetch", fetchMock);
	Object.assign(window, { Clerk: { loaded: true, session: null } });
	vi.stubGlobal("localStorage", { getItem: () => null });
	fetchMock.mockReset();
	fetchMock.mockImplementation(async () => new Response(null, { status: 401 }));
});
afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

it("missing session uses 401 callbacks, error and expiry event after an unauthenticated fetch", async () => {
	const expired = vi.fn();
	window.addEventListener(AUTH_EXPIRED_EVENT, expired);
	const onUnauthorized = vi.fn();
	try {
		await expect(
			apiClient.authed.get("/private", { onUnauthorized }),
		).rejects.toBeInstanceOf(UnauthorizedError);
		expect(onUnauthorized).toHaveBeenCalledOnce();
		expect(expired).toHaveBeenCalledOnce();
		expect(fetchMock).toHaveBeenCalledTimes(1);
	} finally {
		window.removeEventListener(AUTH_EXPIRED_EVENT, expired);
	}
});
it("checkFavorite rejects unauthenticated Connect response", async () => {
	await expect(checkFavorite("123")).rejects.toMatchObject({
		code: Code.Unauthenticated,
	});
	expect(fetchMock).toHaveBeenCalledTimes(1);
});
it("suppressAuthExpired still suppresses a missing-session event", async () => {
	const expired = vi.fn();
	window.addEventListener(AUTH_EXPIRED_EVENT, expired);
	try {
		await expect(
			apiClient.authed.get("/private", { suppressAuthExpired: true }),
		).rejects.toBeInstanceOf(UnauthorizedError);
		expect(expired).not.toHaveBeenCalled();
	} finally {
		window.removeEventListener(AUTH_EXPIRED_EVENT, expired);
	}
});
it("SSE stops when signed out, including online and visibility changes", async () => {
	vi.useFakeTimers();
	const statuses: string[] = [];
	const client = new SseClient({
		path: "/notifications/stream",
		mode: "authed",
		handlers: {
			onEvent: vi.fn(),
			onReady: vi.fn(),
			onStatusChange: (s) => statuses.push(s),
		},
	});
	client.open();
	await vi.advanceTimersByTimeAsync(0);
	expect(statuses.at(-1)).toBe("unavailable");
	expect(vi.getTimerCount()).toBe(0);
	await vi.advanceTimersByTimeAsync(60_000);
	window.dispatchEvent(new Event("online"));
	document.dispatchEvent(new Event("visibilitychange"));
	await vi.advanceTimersByTimeAsync(60_000);
	expect(statuses.at(-1)).toBe("unavailable");
	expect(fetchMock).toHaveBeenCalledTimes(1);
	expect(vi.getTimerCount()).toBe(0);
	client.close();
});

it("session ending during a 401 refresh still uses the unauthorized path", async () => {
	const clerk: {
		loaded: boolean;
		session: { getToken: () => Promise<string | null> } | null;
	} = {
		loaded: true,
		session: {
			getToken: vi
				.fn()
				.mockResolvedValueOnce("expired")
				.mockImplementationOnce(async () => {
					clerk.session = null;
					return null;
				}),
		},
	};
	Object.assign(window, { Clerk: clerk });
	fetchMock.mockResolvedValue(new Response(null, { status: 401 }));
	const onUnauthorized = vi.fn();
	await expect(
		apiClient.authed.get("/private", { onUnauthorized }),
	).rejects.toBeInstanceOf(UnauthorizedError);
	expect(onUnauthorized).toHaveBeenCalledOnce();
	expect(fetchMock).toHaveBeenCalledTimes(2);
});

for (const failure of [
	new TypeError("offline"),
	new DOMException("aborted", "AbortError"),
]) {
	for (const options of [{ softFail: true }, { throwOnHttpError: false }]) {
		it(`refresh fetch ${failure.name} follows softFail with ${JSON.stringify(options)}`, async () => {
			Object.assign(window, {
				Clerk: { loaded: true, session: { getToken: async () => "token" } },
			});
			fetchMock.mockImplementation(async (_url, init) => {
				expect(new Headers(init.headers).get("Authorization")).toBe(
					"Bearer token",
				);
				if (fetchMock.mock.calls.length === 1)
					return new Response(null, { status: 401 });
				throw failure;
			});
			const request = apiClient.authed.get("/private", options);
			if ("softFail" in options) {
				await expect(request).resolves.toMatchObject({ code: 503, data: null });
			} else {
				await expect(request).rejects.toBe(failure);
			}
		});
	}
}
for (const refresh of [false, true]) {
	it(`surfaces unavailable token acquisition (refresh=${refresh})`, async () => {
		const getToken = vi.fn().mockRejectedValue(new Error("offline"));
		if (refresh) getToken.mockResolvedValueOnce("expired");
		Object.assign(window, { Clerk: { loaded: true, session: { getToken } } });
		await expect(checkFavorite("123")).rejects.toMatchObject({
			code: Code.Unavailable,
		});
	});
}
it("strict callers still receive refresh network errors", async () => {
	Object.assign(window, {
		Clerk: { loaded: true, session: { getToken: async () => "token" } },
	});
	fetchMock.mockRejectedValue(new TypeError("offline"));
	await expect(apiClient.authed.get("/private")).rejects.toThrow("offline");
});

for (const [name, write] of [
	["confirmOrderReceipt", () => confirmOrderReceipt("123")],
	["toggleCommentLike POST", () => toggleCommentLike("123", false)],
	["toggleCommentLike DELETE", () => toggleCommentLike("123", true)],
] as const) {
	it(`${name} rejects network failure even with throwOnHttpError:false`, async () => {
		fetchMock.mockRejectedValue(new TypeError("offline"));
		await expect(write()).rejects.toThrow("offline");
		expect(fetchMock).toHaveBeenCalledOnce();
	});
}

it("request abort cancels pending token acquisition before fetch", async () => {
	const getToken = vi.fn(() => new Promise<string>(() => {}));
	Object.assign(window, { Clerk: { loaded: true, session: { getToken } } });
	const controller = new AbortController();
	const request = apiClient.authed.get("/private", {
		signal: controller.signal,
	});
	const rejected = expect(request).rejects.toMatchObject({
		name: "AbortError",
	});
	controller.abort();
	await rejected;
	expect(getToken).toHaveBeenCalledOnce();
	expect(fetchMock).not.toHaveBeenCalled();
});
it("GET retries reuse the token acquired once", async () => {
	const getToken = vi.fn(async () => "token");
	Object.assign(window, { Clerk: { loaded: true, session: { getToken } } });
	fetchMock
		.mockRejectedValueOnce(new TypeError("offline"))
		.mockResolvedValueOnce(
			new Response(JSON.stringify({ code: 0, data: true })),
		);
	await apiClient.authed.get("/private");
	expect(getToken).toHaveBeenCalledOnce();
	expect(fetchMock).toHaveBeenCalledTimes(2);
});
it("guest POST 401 is never resent", async () => {
	await expect(apiClient.authed.post("/private")).rejects.toBeInstanceOf(
		UnauthorizedError,
	);
	expect(fetchMock).toHaveBeenCalledOnce();
});

for (const collected of [false, true]) {
	it(`toggleFavorite ${collected ? "DELETE" : "POST"} rejects write network failure`, async () => {
		fetchMock
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ favorited: collected }), {
					headers: { "Content-Type": "application/json" },
				}),
			)
			.mockRejectedValue(new TypeError("offline"));
		await expect(toggleFavorite("123")).rejects.toBeInstanceOf(ConnectError);
		expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2);
		expect(fetchMock.mock.calls[1][1].method).toBe("POST");
		expect(String(fetchMock.mock.calls[1][0])).toContain(
			collected ? "/RemoveFavorite" : "/AddFavorite",
		);
	});
}

it("request abort cancels waiting for Clerk SDK hydration", async () => {
	Object.assign(window, { Clerk: { loaded: false } });
	const controller = new AbortController();
	const request = apiClient.authed.get("/private", {
		signal: controller.signal,
	});
	const rejected = expect(request).rejects.toMatchObject({
		name: "AbortError",
	});
	controller.abort();
	await rejected;
	expect(fetchMock).not.toHaveBeenCalled();
});

for (const options of [{}, { throwOnHttpError: false }, { softFail: true }]) {
	it(`failed Clerk refresh never resends stale Bearer: ${JSON.stringify(options)}`, async () => {
		const getToken = vi
			.fn()
			.mockResolvedValueOnce("stale")
			.mockRejectedValue(new Error("offline"));
		Object.assign(window, { Clerk: { loaded: true, session: { getToken } } });
		const expired = vi.fn();
		window.addEventListener(AUTH_EXPIRED_EVENT, expired);
		try {
			const result = apiClient.authed.get("/private", options);
			if ("softFail" in options)
				await expect(result).resolves.toMatchObject({ code: 503, data: null });
			else await expect(result).rejects.toBeInstanceOf(AuthUnavailableError);
			expect(fetchMock).toHaveBeenCalledOnce();
			expect(getToken).toHaveBeenLastCalledWith({ skipCache: true });
			expect(expired).not.toHaveBeenCalled();
		} finally {
			window.removeEventListener(AUTH_EXPIRED_EVENT, expired);
		}
	});
}
for (const [name, read] of [
	["toggleFavorite", () => toggleFavorite("123")],
	["toggleFollow", () => toggleFollow("123")],
] as const) {
	it(`${name} surfaces a failed read without mutating`, async () => {
		fetchMock.mockRejectedValue(new TypeError("offline"));
		await expect(read()).rejects.toThrow("offline");
		expect(fetchMock.mock.calls.length).toBeGreaterThanOrEqual(2);
		expect(
			fetchMock.mock.calls.every(([, init]) => init.method === "GET"),
		).toBe(true);
	});
}
