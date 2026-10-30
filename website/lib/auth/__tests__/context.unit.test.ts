// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
	SESSION_UNAVAILABLE: Symbol("session-unavailable"),
	fetchSession: vi.fn(),
	logout: vi.fn(async () => {}),
}));

vi.mock("../../api/auth", () => ({
	SESSION_UNAVAILABLE: api.SESSION_UNAVAILABLE,
	fetchSession: api.fetchSession,
	logout: api.logout,
	LogoutRevokeFailedError: class extends Error {
		reason = "server";
	},
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
	api.logout.mockReset();
	api.logout.mockResolvedValue(undefined);
	api.fetchSession.mockReset();
});

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	storage.clear();
});

describe("AuthProvider on the GetSession answer", () => {
	it("revokes the session of a suspended account and displays the server denial", async () => {
		api.fetchSession.mockRejectedValue(
			new ConnectError("account suspended", Code.PermissionDenied, undefined, [
				{
					desc: LocalizedMessageSchema,
					value: { locale: "zh-TW", message: "帳號已停權，請聯絡客服" },
				},
			]),
		);
		const { result } = render();
		await waitFor(() => expect(api.logout).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith("帳號已停權，請聯絡客服");
		expect(result.current.isLoggedIn).toBe(false);
	});

	it("without a LocalizedMessage the toast is the code's catalog text, never the developer message", async () => {
		vi.mocked(toast.error).mockClear();
		api.fetchSession.mockRejectedValue(
			new ConnectError("DEV-ONLY detail", Code.PermissionDenied),
		);
		render();
		await waitFor(() => expect(api.logout).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith(
			defaultErrorTextFor("zh-TW")(Code.PermissionDenied),
		);
		expect(toast.error).not.toHaveBeenCalledWith(
			expect.stringContaining("DEV-ONLY"),
		);
	});

	it("the cookie is HttpOnly, so every load asks GetSession; unauthenticated settles as a guest", async () => {
		api.fetchSession.mockResolvedValue(null);
		const { result } = render();
		await waitFor(() => expect(result.current.sessionSettled).toBe(true));
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.isLoggedIn).toBe(false);
		expect(result.current.identity).toBeNull();
		// a guest has no session to end
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("is not settled before the GetSession answer arrives", async () => {
		api.fetchSession.mockReturnValue(new Promise(() => {}));
		const { result } = render();
		await act(async () => {});
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.sessionSettled).toBe(false);
		expect(result.current.isLoggedIn).toBe(false);
	});

	it("resolves the account through GetSession", async () => {
		api.fetchSession.mockResolvedValue(buyer);
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(result.current.identity?.accountId).toBe("7");
		await waitFor(() => expect(result.current.activeRole).toBe("buyer"));
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("a 401 broadcast from any call turns the page into a guest without calling SignOut", async () => {
		api.fetchSession.mockResolvedValue(buyer);
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(result.current.isLoggedIn).toBe(false));
		expect(result.current.sessionSettled).toBe(true);
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("an outage that outlasts the backoff never ends the session", async () => {
		vi.useFakeTimers({ shouldAdvanceTime: true });
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
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("signOut calls the SignOut RPC and then the page is a guest", async () => {
		api.fetchSession.mockResolvedValue(buyer);
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		expect(api.logout).toHaveBeenCalledTimes(1);
		expect(result.current.isLoggedIn).toBe(false);
		expect(result.current.sessionSettled).toBe(true);
	});

	it("a failed SignOut keeps the account: the session was not ended", async () => {
		api.fetchSession.mockResolvedValue(buyer);
		api.logout.mockRejectedValueOnce(new Error("offline"));
		const { result } = render();
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(result.current.identity?.accountId).toBe("7");
	});
});

it("retries a failed rejection sign-out, coalesces concurrent calls and toasts once", async () => {
	vi.mocked(toast.error).mockClear();
	api.fetchSession.mockResolvedValue(buyer);
	const { result } = render();
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	const denied = new ConnectError("suspended", Code.PermissionDenied);
	api.logout.mockRejectedValueOnce(new Error("offline"));
	await act(async () => {
		await Promise.all([
			result.current.rejectSession(denied),
			result.current.rejectSession(denied),
		]);
	});
	expect(api.logout).toHaveBeenCalledTimes(1);
	await act(async () => {
		await result.current.rejectSession(denied);
	});
	expect(api.logout).toHaveBeenCalledTimes(2);
	expect(toast.error).toHaveBeenCalledTimes(1);
});

it("session cache refetch cannot replace identity with a transient result", async () => {
	api.fetchSession.mockResolvedValue(buyer);
	const client = new QueryClient();
	const { result } = render(client);
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	api.fetchSession.mockResolvedValue(api.SESSION_UNAVAILABLE);
	await act(async () => {
		await client.refetchQueries({ queryKey: ["session"] });
		const query = client.getQueryCache().find({ queryKey: ["session"] });
		if (!query) throw new Error("session query missing");
		await query
			.fetch({ ...query.options, retry: false })
			.catch(() => undefined);
	});
	expect(api.fetchSession).toHaveBeenCalledOnce();
	expect(result.current.identity).toEqual(buyer);
	expect(api.logout).not.toHaveBeenCalled();
});
