// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const api = vi.hoisted(() => ({
	logout: vi.fn(async () => {}),
	fetchSession: vi.fn(),
	UNAVAILABLE: Symbol("session-unavailable"),
}));
const router = vi.hoisted(() => ({
	push: vi.fn(),
	replace: vi.fn(),
	refresh: vi.fn(),
}));

vi.mock("../../api/auth", () => ({
	logout: api.logout,
	fetchSession: api.fetchSession,
	SESSION_UNAVAILABLE: api.UNAVAILABLE,
	LogoutRevokeFailedError: class extends Error {
		reason = "server";
	},
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
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
import { SESSION_RETRY_BACKOFF_MS, type SessionState } from "../session-state";

const buyer = { accountId: "7", isSeller: false };

const signedIn: SessionState = { status: "signedIn", identity: buyer };
const guest: SessionState = { status: "guest" };
const unavailable: SessionState = { status: "unavailable" };

class FakeBroadcastChannel {
	static open = new Set<FakeBroadcastChannel>();
	static sent: unknown[] = [];
	onmessage: ((event: { data: unknown }) => void) | null = null;
	constructor(readonly name: string) {
		FakeBroadcastChannel.open.add(this);
	}
	postMessage(data: unknown) {
		FakeBroadcastChannel.sent.push(data);
		for (const peer of FakeBroadcastChannel.open) {
			if (peer !== this && peer.name === this.name) peer.onmessage?.({ data });
		}
	}
	close() {
		FakeBroadcastChannel.open.delete(this);
	}
}

/** Commits what is pending, then lets React Query's batched notifications (a macrotask each) reach the observers. */
async function flushMacrotasks() {
	for (let i = 0; i < 3; i++) {
		await act(async () => {
			await new Promise<void>((done) => setTimeout(done, 0));
		});
	}
}

/** A message from another tab. */
function peerAnnounces(data: unknown) {
	const peer = new FakeBroadcastChannel("hk-auth-session");
	peer.postMessage(data);
	peer.close();
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

/** Every context value the consumer rendered, in order: the first one is the first screen. */
async function render(
	session: SessionState | Promise<SessionState>,
	client = new QueryClient(),
) {
	const rendered: Array<ReturnType<typeof useAuth>> = [];
	let sessionPromise = Promise.resolve(session);
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
	return {
		...hook,
		rendered,
		/** What the refreshed root layout does: hand the provider a new promise. */
		async adopt(next: SessionState | Promise<SessionState>) {
			sessionPromise = Promise.resolve(next);
			await act(async () => {
				hook.rerender();
			});
			await act(async () => {
				await Promise.resolve(next).catch(() => {});
			});
			await flushMacrotasks();
		},
	};
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
	router.refresh.mockReset();
	FakeBroadcastChannel.open.clear();
	FakeBroadcastChannel.sent = [];
	vi.stubGlobal("BroadcastChannel", FakeBroadcastChannel);
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

describe("AuthProvider retries an unavailable session", () => {
	async function advance(ms: number) {
		await act(async () => {
			await vi.advanceTimersByTimeAsync(ms);
		});
	}

	it("re-asks GetSession on the backoff schedule and becomes signedIn when it answers", async () => {
		vi.useFakeTimers();
		api.fetchSession
			.mockResolvedValueOnce(api.UNAVAILABLE)
			.mockResolvedValueOnce(buyer);
		const { result } = await render(unavailable);
		expect(result.current.status).toBe("unavailable");
		await advance(SESSION_RETRY_BACKOFF_MS[0] - 1);
		expect(api.fetchSession).not.toHaveBeenCalled();
		await advance(1);
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		await advance(SESSION_RETRY_BACKOFF_MS[1]);
		expect(api.fetchSession).toHaveBeenCalledTimes(2);
		await advance(1);
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
		await advance(SESSION_RETRY_BACKOFF_MS[2] * 2);
		expect(api.fetchSession).toHaveBeenCalledTimes(2);
	});

	it("a 401 answer on retry is a guest", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValueOnce(null);
		const { result } = await render(unavailable);
		await advance(SESSION_RETRY_BACKOFF_MS[0]);
		await advance(1);
		expect(result.current.status).toBe("guest");
	});

	it("stays unavailable, never a guest, after the schedule runs out", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValue(api.UNAVAILABLE);
		const { result } = await render(unavailable);
		for (const delay of SESSION_RETRY_BACKOFF_MS) await advance(delay);
		expect(api.fetchSession).toHaveBeenCalledTimes(
			SESSION_RETRY_BACKOFF_MS.length,
		);
		await advance(60_000);
		expect(api.fetchSession).toHaveBeenCalledTimes(
			SESSION_RETRY_BACKOFF_MS.length,
		);
		expect(result.current.status).toBe("unavailable");
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("does not ask again when the server's first answer was definitive", async () => {
		vi.useFakeTimers();
		await render(guest);
		await advance(60_000);
		expect(api.fetchSession).not.toHaveBeenCalled();
	});
});

describe("AuthProvider adopts the server's newest answer after an event", () => {
	it("after sign-out the header shows no avatar: guest now, the server's refresh is requested and its guest answer is adopted", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(router.refresh).not.toHaveBeenCalled();
		await act(async () => {
			await result.current.signOut();
		});
		expect(result.current.status).toBe("guest");
		expect(router.refresh).toHaveBeenCalledTimes(1);
		await adopt(guest);
		expect(result.current.status).toBe("guest");
		expect(result.current.identity).toBeNull();
	});

	it("a failed SignOut does not refresh", async () => {
		api.logout.mockRejectedValueOnce(new Error("offline"));
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("the server value wins over the transient target state", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(result.current.status).toBe("guest"));
		await adopt({ status: "signedIn", identity: { ...buyer, isSeller: true } });
		await waitFor(() => expect(result.current.status).toBe("signedIn"));
		expect(result.current.hasSeller).toBe(true);
	});

	it("a newer promise wins over a late older one", async () => {
		const older = deferred<SessionState>();
		const { result, adopt } = await render(older.promise);
		await adopt(signedIn);
		await waitFor(() => expect(result.current.status).toBe("signedIn"));
		await act(async () => {
			older.resolve(guest);
		});
		await flushMacrotasks();
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
	});

	it("a refresh that answers unavailable keeps the identity instead of signing out", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await adopt(unavailable);
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("a refresh that rejects keeps the identity too", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await adopt(Promise.reject(new Error("stream failed")));
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
	});

	it("a refresh to the same account keeps the profile nickname the client backfilled", async () => {
		const client = new QueryClient();
		const { result, adopt } = await render(signedIn, client);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => {
			client.setQueryData(["session"], { ...buyer, nickname: "Mei" });
		});
		await adopt(signedIn);
		expect(result.current.identity?.nickname).toBe("Mei");
	});

	it("a 401 from any call is guest now and asks the server again", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(result.current.status).toBe("guest"));
		expect(router.refresh).toHaveBeenCalledTimes(1);
	});

	it("a rejection by the server refreshes only after the session was really ended", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		const denied = new ConnectError("suspended", Code.PermissionDenied);
		api.logout.mockRejectedValueOnce(new Error("offline"));
		await act(async () => {
			await result.current.rejectSession(denied);
		});
		expect(router.refresh).not.toHaveBeenCalled();
		await act(async () => {
			await result.current.rejectSession(denied);
		});
		expect(router.refresh).toHaveBeenCalledTimes(1);
	});
});

describe("AuthProvider across tabs", () => {
	it("another tab signing out refreshes this tab from the server without writing client state", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => peerAnnounces({ accountId: null }));
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
	});

	it("another tab signing in refreshes a guest tab", async () => {
		const { result } = await render(guest);
		await waitFor(() => expect(result.current.status).toBe("guest"));
		act(() => peerAnnounces({ accountId: "7" }));
		expect(router.refresh).toHaveBeenCalledTimes(1);
	});

	it("a peer that agrees, a malformed message and a first answer still on its way do not refresh", async () => {
		const pending = deferred<SessionState>();
		const { result } = await render(pending.promise);
		act(() => peerAnnounces({ accountId: "7" }));
		expect(router.refresh).not.toHaveBeenCalled();
		await act(async () => {
			pending.resolve(signedIn);
		});
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => peerAnnounces({ accountId: "7" }));
		act(() => peerAnnounces({ accountId: 7 }));
		act(() => peerAnnounces("7"));
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("announces the account it sees, once per change, including its own sign-out", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(FakeBroadcastChannel.sent).toEqual([{ accountId: "7" }]);
		await act(async () => {
			await result.current.signOut();
		});
		expect(FakeBroadcastChannel.sent).toEqual([
			{ accountId: "7" },
			{ accountId: null },
		]);
	});

	it("does not announce while unknown or unavailable", async () => {
		await render(unavailable);
		expect(FakeBroadcastChannel.sent).toEqual([]);
	});
});

describe("AuthProvider retries an unavailable session on reconnect and return", () => {
	it("asks again when the browser comes back online, before the backoff elapses", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValueOnce(buyer);
		const { result } = await render(unavailable);
		expect(api.fetchSession).not.toHaveBeenCalled();
		await act(async () => {
			window.dispatchEvent(new Event("online"));
			await vi.advanceTimersByTimeAsync(0);
		});
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("signedIn");
	});

	it("asks again when the page becomes visible, not when it becomes hidden", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValue(api.UNAVAILABLE);
		const { result } = await render(unavailable);
		const visibility = vi.spyOn(document, "visibilityState", "get");
		visibility.mockReturnValue("hidden");
		await act(async () => {
			document.dispatchEvent(new Event("visibilitychange"));
		});
		expect(api.fetchSession).not.toHaveBeenCalled();
		visibility.mockReturnValue("visible");
		await act(async () => {
			document.dispatchEvent(new Event("visibilitychange"));
		});
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		visibility.mockRestore();
	});

	it("still asks after the backoff schedule ran out, a 401 is a guest and any other failure stays unavailable", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValue(api.UNAVAILABLE);
		const { result } = await render(unavailable);
		for (const delay of SESSION_RETRY_BACKOFF_MS) {
			await act(async () => {
				await vi.advanceTimersByTimeAsync(delay);
			});
		}
		api.fetchSession.mockRejectedValueOnce(new Error("network"));
		await act(async () => {
			window.dispatchEvent(new Event("online"));
			await vi.advanceTimersByTimeAsync(0);
		});
		expect(result.current.status).toBe("unavailable");
		api.fetchSession.mockResolvedValueOnce(null);
		await act(async () => {
			window.dispatchEvent(new Event("online"));
			await vi.advanceTimersByTimeAsync(0);
		});
		expect(result.current.status).toBe("guest");
	});

	it("does not ask while the server's answer was definitive", async () => {
		await render(guest);
		await act(async () => {
			window.dispatchEvent(new Event("online"));
		});
		expect(api.fetchSession).not.toHaveBeenCalled();
	});
});
