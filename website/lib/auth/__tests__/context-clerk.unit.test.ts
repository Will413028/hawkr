// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const clerk = vi.hoisted(() => ({
	state: {
		isLoaded: false,
		isSignedIn: false as boolean | undefined,
		userId: null as string | null,
	},
	signOut: vi.fn(async (_callback?: () => void) => {}),
	/** useClerk() hands out one stable instance, as the real provider does. */
	instance: {} as { signOut?: unknown },
}));
clerk.instance = { signOut: clerk.signOut };
const api = vi.hoisted(() => ({
	SESSION_UNAVAILABLE: Symbol("session-unavailable"),
	fetchSession: vi.fn(),
}));

vi.mock("@clerk/nextjs", () => ({
	useAuth: () => ({ ...clerk.state }),
	useClerk: () => clerk.instance,
}));
vi.mock("../../api/auth", () => ({
	SESSION_UNAVAILABLE: api.SESSION_UNAVAILABLE,
	fetchSession: api.fetchSession,
	logout: vi.fn(),
	LogoutRevokeFailedError: class extends Error {},
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));

import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { toast } from "@/lib/feedback/toast";
import { defaultErrorTextFor } from "@/lib/i18n/ui-text";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";

vi.mock("@/lib/feedback/toast", () => ({ toast: { error: vi.fn() } }));

import { AUTH_EXPIRED_EVENT } from "@/lib/api/client";
import { AuthProvider, useAuth } from "../context";
import { SESSION_RETRY_BACKOFF_MS } from "../session-resolver";

const buyer = { accountId: "7", isSeller: false };

function render(client = new QueryClient()) {
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(AuthProvider, null, children),
			),
		);
	return renderHook(() => useAuth(), { wrapper });
}

const storage = new Map<string, string>();

beforeEach(() => {
	// Node's own backendless `localStorage` global shadows jsdom's.
	vi.stubGlobal("localStorage", {
		getItem: (k: string) => storage.get(k) ?? null,
		setItem: (k: string, v: string) => void storage.set(k, v),
		removeItem: (k: string) => void storage.delete(k),
		key: (i: number) => [...storage.keys()][i] ?? null,
		get length() {
			return storage.size;
		},
		clear: () => storage.clear(),
	});
	clerk.state = { isLoaded: false, isSignedIn: false, userId: null };
	clerk.signOut.mockClear();
	api.fetchSession.mockReset();
});

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	storage.clear();
});

describe("AuthProvider on Clerk's sign-in state", () => {
	it("signs out a suspended account and displays the server denial", async () => {
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "suspended" };
		api.fetchSession.mockRejectedValue(
			new ConnectError("account suspended", Code.PermissionDenied, undefined, [
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "帳號已停權，請聯絡客服" },
				},
			]),
		);
		const { result } = render();
		await waitFor(() => expect(clerk.signOut).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith("帳號已停權，請聯絡客服");
		expect(result.current.isLoggedIn).toBe(false);
	});

	it("without a LocalizedMessage the toast is the code's catalog text, never the developer message", async () => {
		vi.mocked(toast.error).mockClear();
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "dev-msg" };
		api.fetchSession.mockRejectedValue(
			new ConnectError("DEV-ONLY detail", Code.PermissionDenied),
		);
		render();
		await waitFor(() => expect(clerk.signOut).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith(
			defaultErrorTextFor("zh-TW")(Code.PermissionDenied),
		);
		expect(toast.error).not.toHaveBeenCalledWith(
			expect.stringContaining("DEV-ONLY"),
		);
	});

	it("a Clerk guest settles as a guest without asking GetSession", async () => {
		clerk.state = { isLoaded: true, isSignedIn: false, userId: null };
		const { result } = render();
		await waitFor(() => expect(result.current.sessionSettled).toBe(true));
		expect(result.current.isLoggedIn).toBe(false);
		expect(api.fetchSession).not.toHaveBeenCalled();
		expect(clerk.signOut).not.toHaveBeenCalled();
	});

	it("waits for Clerk to load before deciding anything", async () => {
		const { result } = render();
		await act(async () => {});
		expect(result.current.sessionSettled).toBe(false);
		expect(api.fetchSession).not.toHaveBeenCalled();
	});

	it("a Clerk session resolves the account through GetSession", async () => {
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
		api.fetchSession.mockResolvedValue(buyer);
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(result.current.identity?.accountId).toBe("7");
		await waitFor(() => expect(result.current.activeRole).toBe("buyer"));
		expect(clerk.signOut).not.toHaveBeenCalled();
	});

	it("keeps account data when Clerk refuses an automatic sign-out", async () => {
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
		api.fetchSession.mockResolvedValue(buyer);
		clerk.signOut.mockRejectedValueOnce(new Error("offline"));
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		expect(clerk.signOut).toHaveBeenCalledTimes(1);
		expect(result.current.identity?.accountId).toBe("7");
	});

	it("a late GetSession answer cannot overwrite a different Clerk user", async () => {
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
		let finishFirst: ((value: typeof buyer) => void) | undefined;
		api.fetchSession
			.mockImplementationOnce(
				() =>
					new Promise((resolve) => {
						finishFirst = resolve;
					}),
			)
			.mockResolvedValueOnce({
				...buyer,
				accountId: "8",
			});
		const { result, rerender } = render();
		await waitFor(() => expect(api.fetchSession).toHaveBeenCalledTimes(1));
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_2" };
		rerender();
		await waitFor(() => expect(result.current.identity?.accountId).toBe("8"));
		await act(async () => finishFirst?.(buyer));
		expect(result.current.identity?.accountId).toBe("8");
	});

	it("a session the monolith refuses ends the Clerk session too", async () => {
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
		api.fetchSession.mockResolvedValue(null);
		const { result } = render();
		await waitFor(() => expect(clerk.signOut).toHaveBeenCalledTimes(1));
		expect(result.current.isLoggedIn).toBe(false);
		expect(result.current.sessionSettled).toBe(true);
	});

	it("an outage that outlasts the backoff never ends the Clerk session", async () => {
		vi.useFakeTimers({ shouldAdvanceTime: true });
		clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
		api.fetchSession.mockResolvedValue(api.SESSION_UNAVAILABLE);
		const { result } = render();
		for (const delay of SESSION_RETRY_BACKOFF_MS) {
			await act(async () => {
				await vi.advanceTimersByTimeAsync(delay);
			});
		}
		await waitFor(() =>
			expect(api.fetchSession).toHaveBeenCalledTimes(
				SESSION_RETRY_BACKOFF_MS.length + 1,
			),
		);
		await waitFor(() => expect(result.current.sessionSettled).toBe(true));
		expect(result.current.isLoggedIn).toBe(false);
		expect(clerk.signOut).not.toHaveBeenCalled();
	});
});

it("retries failed rejection signOut, coalesces concurrent calls and toasts once per session", async () => {
	vi.mocked(toast.error).mockClear();
	clerk.state = { isLoaded: true, isSignedIn: true, userId: "retry-user" };
	api.fetchSession.mockResolvedValue(buyer);
	const { result, rerender } = render();
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	const denied = new ConnectError("suspended", Code.PermissionDenied);
	clerk.signOut.mockRejectedValueOnce(new Error("offline"));
	await act(async () => {
		await Promise.all([
			result.current.rejectSession(denied),
			result.current.rejectSession(denied),
		]);
	});
	expect(clerk.signOut).toHaveBeenCalledTimes(1);
	await act(async () => {
		await result.current.rejectSession(denied);
	});
	expect(clerk.signOut).toHaveBeenCalledTimes(2);
	expect(toast.error).toHaveBeenCalledTimes(1);
	clerk.state = { ...clerk.state, userId: "next-user" };
	rerender();
	await act(async () => {
		await result.current.rejectSession(denied);
	});
	expect(clerk.signOut).toHaveBeenCalledTimes(3);
	expect(toast.error).toHaveBeenCalledTimes(2);
});

it("session cache refetch cannot replace identity with a transient result", async () => {
	clerk.state = { isLoaded: true, isSignedIn: true, userId: "user_1" };
	api.fetchSession.mockResolvedValue(buyer);
	const client = new QueryClient();
	const { result } = render(client);
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	api.fetchSession.mockResolvedValue(api.SESSION_UNAVAILABLE);
	await act(async () => {
		await client.refetchQueries({ queryKey: ["session"] });
		const query = client
			.getQueryCache()
			.find({ queryKey: ["session", "user_1"] });
		if (!query) throw new Error("session query missing");
		await query
			.fetch({ ...query.options, retry: false })
			.catch(() => undefined);
	});
	expect(api.fetchSession).toHaveBeenCalledOnce();
	expect(result.current.identity).toEqual(buyer);
	expect(clerk.signOut).not.toHaveBeenCalled();
});
