// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
	logout: vi.fn(async () => {}),
}));

vi.mock("../../api/auth", () => ({
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
import type { SessionState } from "../session-state";

const buyer = { accountId: "7", isSeller: false };

const signedIn: SessionState = { status: "signedIn", identity: buyer };
const guest: SessionState = { status: "guest" };
const unavailable: SessionState = { status: "unavailable" };

/** Every context value the consumer rendered, in order: the first one is the first screen. */
async function render(
	session: SessionState | Promise<SessionState>,
	client = new QueryClient(),
) {
	const rendered: Array<ReturnType<typeof useAuth>> = [];
	const sessionPromise = Promise.resolve(session);
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(AuthProvider, {
					session: sessionPromise,
					children,
				}),
			),
		);
	let hook!: ReturnType<typeof renderHook<ReturnType<typeof useAuth>, unknown>>;
	await act(async () => {
		hook = renderHook(
			() => {
				const value = useAuth();
				rendered.push(value);
				return value;
			},
			{ wrapper },
		);
	});
	return { ...hook, rendered };
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
});

afterEach(() => {
	cleanup();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	storage.clear();
});

describe("AuthProvider on the server's session answer", () => {
	it("the first screen has no checking state: it is `unknown`, never a guess, and then the server's answer", async () => {
		const { result, rendered } = await render(signedIn);
		await waitFor(() => expect(result.current.status).toBe("signedIn"));
		expect(rendered[0]?.status).toBe("unknown");
		expect(rendered[0]?.isLoggedIn).toBe(false);
		expect(rendered[0]?.identity).toBeNull();
		const statuses = rendered.map((value) => value.status);
		expect(statuses).not.toContain("guest");
		expect(statuses.at(-1)).toBe("signedIn");
		expect(result.current.identity).toEqual(buyer);
	});

	it("stays `unknown` until the session promise settles, without suspending", async () => {
		let settle!: (state: SessionState) => void;
		const pending = new Promise<SessionState>((resolve) => {
			settle = resolve;
		});
		const { result } = await render(pending);
		expect(result.current.status).toBe("unknown");
		expect(result.current.isLoggedIn).toBe(false);
		await act(async () => {
			settle(guest);
		});
		await waitFor(() => expect(result.current.status).toBe("guest"));
	});

	it("a guest resolves from `unknown` to `guest`", async () => {
		const { result, rendered } = await render(guest);
		await waitFor(() => expect(result.current.status).toBe("guest"));
		expect(rendered[0]?.status).toBe("unknown");
		expect(result.current.isLoggedIn).toBe(false);
	});

	it("a rejected session promise is `unavailable`, not a guest", async () => {
		const { result } = await render(Promise.reject(new Error("stream failed")));
		await waitFor(() => expect(result.current.status).toBe("unavailable"));
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("revokes the session of a suspended account and displays the server denial", async () => {
		const { result } = await render({
			status: "guest",
			refusal: { message: "帳號已停權，請聯絡客服" },
		});
		await waitFor(() => expect(api.logout).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith("帳號已停權，請聯絡客服");
		expect(result.current.isLoggedIn).toBe(false);
	});

	it("without a message the toast is the code's catalog text, never a developer message", async () => {
		vi.mocked(toast.error).mockClear();
		await render({ status: "guest", refusal: { message: null } });
		await waitFor(() => expect(api.logout).toHaveBeenCalled());
		expect(toast.error).toHaveBeenCalledWith(
			defaultErrorTextFor("zh-TW")(Code.PermissionDenied),
		);
		expect(toast.error).not.toHaveBeenCalledWith(
			expect.stringContaining("session refused"),
		);
	});

	it("a guest has no session to end", async () => {
		const { result } = await render(guest);
		await waitFor(() => expect(result.current.status).toBe("guest"));
		expect(result.current.isLoggedIn).toBe(false);
		expect(result.current.identity).toBeNull();
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("resolves the account the server gave", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(result.current.identity?.accountId).toBe("7");
		await waitFor(() => expect(result.current.activeRole).toBe("buyer"));
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("a 401 broadcast from any call turns the page into a guest without calling SignOut", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(result.current.isLoggedIn).toBe(false));
		expect(result.current.status).toBe("guest");
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("an unavailable answer is not a guest and never ends the session", async () => {
		const { result } = await render(unavailable);
		await waitFor(() => expect(result.current.status).toBe("unavailable"));
		expect(result.current.isLoggedIn).toBe(false);
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("signOut calls the SignOut RPC and then the page is a guest", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		expect(api.logout).toHaveBeenCalledTimes(1);
		expect(result.current.isLoggedIn).toBe(false);
		expect(result.current.status).toBe("guest");
	});

	it("a failed SignOut keeps the account: the session was not ended", async () => {
		api.logout.mockRejectedValueOnce(new Error("offline"));
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(result.current.identity?.accountId).toBe("7");
	});
});

it("retries a failed rejection sign-out, coalesces concurrent calls and toasts once", async () => {
	vi.mocked(toast.error).mockClear();
	const { result } = await render(signedIn);
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

it("the session cache has no fetcher: a refetch cannot replace identity", async () => {
	const client = new QueryClient();
	const { result } = await render(signedIn, client);
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	await act(async () => {
		await client.refetchQueries({ queryKey: ["session"] });
	});
	expect(result.current.identity).toEqual(buyer);
	expect(api.logout).not.toHaveBeenCalled();
});
