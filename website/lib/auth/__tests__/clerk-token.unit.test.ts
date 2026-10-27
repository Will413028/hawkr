import { afterEach, describe, expect, it, vi } from "vitest";
import {
	AuthUnavailableError,
	ClerkSessionEndedError,
	clerkSessionToken,
	withClerkAuthorization,
} from "../clerk-token";

afterEach(() => {
	vi.useRealTimers();
	vi.unstubAllGlobals();
});

function clerk(
	getToken: (options?: { skipCache?: boolean }) => Promise<string | null>,
) {
	vi.stubGlobal("window", {
		Clerk: { loaded: true, session: { getToken } },
	});
}

describe("clerkSessionToken", () => {
	it("does not read browser credentials on the server", async () => {
		expect(await clerkSessionToken()).toBeNull();
	});

	it("waits for the browser SDK to load within one bounded budget", async () => {
		vi.useFakeTimers();
		const browser: { Clerk?: unknown } = {};
		vi.stubGlobal("window", browser);
		const pending = clerkSessionToken();
		await vi.advanceTimersByTimeAsync(250);
		browser.Clerk = { loaded: false };
		await vi.advanceTimersByTimeAsync(250);
		browser.Clerk = {
			loaded: true,
			session: { getToken: async () => "late-token" },
		};
		await vi.advanceTimersByTimeAsync(50);
		expect(await pending).toBe("late-token");
	});

	it("bounds waiting for a missing SDK", async () => {
		vi.useFakeTimers();
		vi.stubGlobal("window", {});
		const pending = expect(clerkSessionToken()).rejects.toBeInstanceOf(
			AuthUnavailableError,
		);
		await vi.advanceTimersByTimeAsync(10_000);
		await pending;
	});

	it("reads Clerk's current session and refreshes on demand", async () => {
		const getToken = vi.fn(async () => "token-1");
		clerk(getToken);
		expect(await clerkSessionToken({ skipCache: true })).toBe("token-1");
		expect(getToken).toHaveBeenCalledWith({ skipCache: true });
	});

	it("does not turn SDK failure into a request without a token", async () => {
		clerk(async () => {
			throw new Error("offline");
		});
		await expect(clerkSessionToken()).rejects.toBeInstanceOf(
			AuthUnavailableError,
		);
	});

	it("distinguishes a missing Clerk session from an unavailable SDK", async () => {
		vi.stubGlobal("window", { Clerk: { loaded: true, session: null } });
		await expect(clerkSessionToken()).rejects.toBeInstanceOf(
			ClerkSessionEndedError,
		);
	});

	it("bounds a stalled SDK request", async () => {
		vi.useFakeTimers();
		clerk(() => new Promise<string | null>(() => {}));
		const pending = expect(clerkSessionToken()).rejects.toBeInstanceOf(
			AuthUnavailableError,
		);
		await vi.advanceTimersByTimeAsync(10_000);
		await pending;
	});
});

describe("withClerkAuthorization", () => {
	it("sets the Bearer header", async () => {
		clerk(async () => "token-2");
		const headers = new Headers();
		await withClerkAuthorization(headers);
		expect(headers.get("Authorization")).toBe("Bearer token-2");
	});
});

it("removes a stale Authorization header when Clerk ends the session", async () => {
	vi.stubGlobal("window", { Clerk: { loaded: true, session: null } });
	const headers = new Headers({ Authorization: "Bearer stale" });
	await withClerkAuthorization(headers, { skipCache: true });
	expect(headers.has("Authorization")).toBe(false);
});

it("leaves server-provided authorization unchanged", async () => {
	const headers = new Headers({ Authorization: "Bearer server-token" });
	await withClerkAuthorization(headers);
	expect(headers.get("Authorization")).toBe("Bearer server-token");
});
