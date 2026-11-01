// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
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
const buyerRpc = vi.hoisted(() => ({ getProfile: vi.fn(async () => ({})) }));
vi.mock("../../connect/buyer", () => ({ buyerAppClient: () => buyerRpc }));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));
vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));
const toasts = vi.hoisted(() => ({ error: vi.fn() }));
vi.mock("@/lib/feedback/toast", () => ({ toast: toasts }));
const nav = vi.hoisted(() => ({ commit: vi.fn(), reload: vi.fn() }));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: nav.commit,
}));

import { LogoutRevokeFailedError } from "@/lib/api/auth";
import { AUTH_EXPIRED_EVENT } from "@/lib/api/client";
import { messagesFor } from "@/lib/intl/config";
import { ACTIVE_ROLE_STORAGE_KEY } from "../active-role";
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
	locale: "zh-TW" | "en" = "zh-TW",
) {
	const rendered: Array<ReturnType<typeof useAuth>> = [];
	let sessionPromise = Promise.resolve(session);
	const wrapper = ({ children }: { children: React.ReactNode }) =>
		React.createElement(
			QueryClientProvider,
			{ client },
			React.createElement(NextIntlClientProvider, {
				locale,
				messages: messagesFor(locale),
				children: React.createElement(AuthProvider, {
					session: sessionPromise,
					children,
				}),
			}),
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
	nav.commit.mockReset();
	nav.reload.mockReset();
	vi.stubGlobal("location", { pathname: "/account", reload: nav.reload });
	FakeBroadcastChannel.open.clear();
	FakeBroadcastChannel.sent = [];
	vi.stubGlobal("BroadcastChannel", FakeBroadcastChannel);
	api.logout.mockReset();
	api.logout.mockResolvedValue(undefined);
	api.fetchSession.mockReset();
	buyerRpc.getProfile.mockClear();
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

	it("a 401 from any call leaves the document for the home page without calling SignOut or asking the server softly", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(nav.commit).toHaveBeenCalledWith("/", true));
		expect(api.logout).not.toHaveBeenCalled();
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("a 401 on the home page still replaces the document: a soft refresh there could be overtaken by an older answer", async () => {
		vi.stubGlobal("location", { pathname: "/", reload: nav.reload });
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
		});
		await waitFor(() => expect(nav.commit).toHaveBeenCalledWith("/", true));
	});

	it.each([
		["guest", guest],
		["unavailable", unavailable],
		["unknown", new Promise<SessionState>(() => {})],
	])("a 401 while the state is %s has no identity to clear and never replaces the document, on the home page or elsewhere", async (_name, state) => {
		for (const pathname of ["/", "/account"]) {
			vi.stubGlobal("location", { pathname, reload: nav.reload });
			const { result, unmount } = await render(state);
			await flushMacrotasks();
			expect(result.current.isLoggedIn).toBe(false);
			await act(async () => {
				window.dispatchEvent(new CustomEvent(AUTH_EXPIRED_EVENT));
			});
			await flushMacrotasks();
			expect(nav.commit).not.toHaveBeenCalled();
			unmount();
		}
	});

	it("signOut of a session that is already gone replaces the document without a failure toast", async () => {
		api.logout.mockResolvedValueOnce(undefined);
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		expect(toasts.error).not.toHaveBeenCalled();
		expect(nav.commit).toHaveBeenCalledExactlyOnceWith("/", false);
	});

	it("toasts the English text resource in the English locale", async () => {
		api.logout.mockRejectedValueOnce(
			Object.assign(new LogoutRevokeFailedError("network"), {
				reason: "network",
			}),
		);
		const { result } = await render(signedIn, new QueryClient(), "en");
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(toasts.error).toHaveBeenCalledWith(
			"Could not log out. Check your connection and try again.",
		);
	});

	it.each([
		[
			"offline",
			Object.assign(new LogoutRevokeFailedError("network"), {
				reason: "network",
			}),
			"登出失敗，請確認網路後再試一次",
		],
		["refused", new LogoutRevokeFailedError("server"), "登出失敗，請再試一次"],
	])("a failed SignOut (%s) toasts the text resource", async (_name, error, text) => {
		api.logout.mockRejectedValueOnce(error);
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(toasts.error).toHaveBeenCalledWith(text);
	});

	it("an unavailable answer is not a guest and never ends the session", async () => {
		const { result } = await render(unavailable);
		await waitFor(() => expect(result.current.status).toBe("unavailable"));
		expect(result.current.isLoggedIn).toBe(false);
		expect(api.logout).not.toHaveBeenCalled();
	});

	it("signOut revokes the session and then replaces the document, writing no client identity", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		expect(api.logout).toHaveBeenCalledTimes(1);
		expect(nav.commit).toHaveBeenCalledExactlyOnceWith("/", false);
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("signOut goes where the caller says, and replaces history when asked", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut("/login?role=seller", true);
		});
		expect(nav.commit).toHaveBeenCalledExactlyOnceWith(
			"/login?role=seller",
			true,
		);
	});

	it("a failed SignOut keeps the account and the document: the session was not ended", async () => {
		api.logout.mockRejectedValueOnce(new Error("offline"));
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut().catch(() => {});
		});
		expect(result.current.identity?.accountId).toBe("7");
		expect(nav.commit).not.toHaveBeenCalled();
	});

	it("a server answer read before the sign-out and delivered after it cannot bring an identity back", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		expect(nav.commit).toHaveBeenCalledExactlyOnceWith("/", false);
		FakeBroadcastChannel.sent = [];
		await adopt({
			status: "signedIn",
			identity: { accountId: "9", isSeller: false },
		});
		expect(result.current.identity?.accountId).toBe("7");
		expect(FakeBroadcastChannel.sent).toEqual([]);
		expect(nav.commit).toHaveBeenCalledTimes(1);
	});
});

it("signOut replaces the document only after the session was really ended, retries after a failure and coalesces concurrent calls", async () => {
	const { result } = await render(signedIn);
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	api.logout.mockRejectedValueOnce(new Error("offline"));
	await act(async () => {
		const results = await Promise.allSettled([
			result.current.signOut("/login", true),
			result.current.signOut("/login", true),
		]);
		expect(results.map((r) => r.status)).toEqual(["rejected", "rejected"]);
	});
	expect(api.logout).toHaveBeenCalledTimes(1);
	expect(nav.commit).not.toHaveBeenCalled();
	await act(async () => {
		await result.current.signOut("/login", true);
	});
	expect(api.logout).toHaveBeenCalledTimes(2);
	expect(nav.commit).toHaveBeenCalledExactlyOnceWith("/login", true);
	expect(router.refresh).not.toHaveBeenCalled();
});

it("identity comes only from the server: no client profile fetch, and the React Query cache cannot replace it", async () => {
	const client = new QueryClient();
	const { result } = await render(signedIn, client);
	await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
	act(() => {
		client.setQueryData(["session"], { accountId: "99", isSeller: true });
	});
	await flushMacrotasks();
	expect(result.current.identity).toEqual(buyer);
	expect(result.current.identity?.nickname).toBeUndefined();
	expect(buyerRpc.getProfile).not.toHaveBeenCalled();
});

describe("AuthProvider retries an unavailable session", () => {
	async function advance(ms: number) {
		await act(async () => {
			await vi.advanceTimersByTimeAsync(ms);
		});
	}

	it("re-asks GetSession on the backoff schedule and, once it answers, asks the server for the new value instead of writing the cache", async () => {
		vi.useFakeTimers();
		api.fetchSession
			.mockResolvedValueOnce(api.UNAVAILABLE)
			.mockResolvedValueOnce(buyer);
		const { result, adopt } = await render(unavailable);
		expect(result.current.status).toBe("unavailable");
		await advance(SESSION_RETRY_BACKOFF_MS[0] - 1);
		expect(api.fetchSession).not.toHaveBeenCalled();
		await advance(1);
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		await advance(SESSION_RETRY_BACKOFF_MS[1]);
		expect(api.fetchSession).toHaveBeenCalledTimes(2);
		await advance(1);
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		expect(result.current.identity).toBeNull();
		vi.useRealTimers();
		await adopt(signedIn);
		expect(result.current.status).toBe("signedIn");
		expect(result.current.identity?.accountId).toBe("7");
		expect(api.fetchSession).toHaveBeenCalledTimes(2);
	});

	it("a 401 answer on retry asks the server and becomes a guest only through its answer", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValueOnce(null);
		const { result, adopt } = await render(unavailable);
		await advance(SESSION_RETRY_BACKOFF_MS[0]);
		await advance(1);
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		vi.useRealTimers();
		await adopt(guest);
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

	it("keeps backing off while the server still answers unavailable after a successful probe", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValue(buyer);
		const { result } = await render(unavailable);
		await advance(SESSION_RETRY_BACKOFF_MS[0]);
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		await advance(SESSION_RETRY_BACKOFF_MS[1]);
		expect(api.fetchSession).toHaveBeenCalledTimes(2);
		expect(router.refresh).toHaveBeenCalledTimes(2);
	});

	it("does not ask again when the server's first answer was definitive", async () => {
		vi.useFakeTimers();
		await render(guest);
		await advance(60_000);
		expect(api.fetchSession).not.toHaveBeenCalled();
	});
});

describe("AuthProvider adopts the server's newest answer", () => {
	it("a refreshed promise from the server replaces the older answer", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await adopt(guest);
		expect(result.current.status).toBe("guest");
		expect(result.current.identity).toBeNull();
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

	it("display name and avatar are the server's newest answer, never merged with an older one", async () => {
		const withProfile: SessionState = {
			status: "signedIn",
			identity: {
				...buyer,
				nickname: "Mei",
				avatarKey: "https://m/i/avatar/a",
			},
		};
		const { result, adopt } = await render(withProfile);
		await waitFor(() => expect(result.current.identity?.nickname).toBe("Mei"));
		expect(result.current.identity?.avatarKey).toBe("https://m/i/avatar/a");
		await adopt(signedIn);
		expect(result.current.identity).toEqual(buyer);
		await adopt({
			status: "signedIn",
			identity: { ...buyer, nickname: "Mei2" },
		});
		expect(result.current.identity?.nickname).toBe("Mei2");
		expect(buyerRpc.getProfile).not.toHaveBeenCalled();
	});

	it("a rejection announces nothing before the session was really ended: the new document announces the guest", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		const revoke = deferred<void>();
		api.logout.mockReturnValueOnce(revoke.promise);
		let pending!: Promise<void>;
		act(() => {
			pending = result.current.signOut("/login", true);
		});
		await flushMacrotasks();
		expect(FakeBroadcastChannel.sent).toEqual([{ accountId: "7" }]);
		expect(nav.commit).not.toHaveBeenCalled();
		await act(async () => {
			revoke.resolve();
			await pending;
		});
		expect(nav.commit).toHaveBeenCalledExactlyOnceWith("/login", true);
		expect(FakeBroadcastChannel.sent).toEqual([{ accountId: "7" }]);
	});
});

describe("AuthProvider across tabs", () => {
	it("a signed-in tab reloads the document when another tab signed out", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => peerAnnounces({ accountId: null }));
		await waitFor(() => expect(nav.reload).toHaveBeenCalledTimes(1));
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("a signed-in tab reloads the document when another tab signed in as a different account", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => peerAnnounces({ accountId: "8" }));
		await waitFor(() => expect(nav.reload).toHaveBeenCalledTimes(1));
		expect(router.refresh).not.toHaveBeenCalled();
	});

	it("a guest tab only asks the server again when another tab signed in, never reloads", async () => {
		const { result } = await render(guest);
		await waitFor(() => expect(result.current.status).toBe("guest"));
		act(() => peerAnnounces({ accountId: "7" }));
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(nav.reload).not.toHaveBeenCalled();
	});

	it("a peer that agrees, a malformed message and a first answer still on its way do nothing", async () => {
		const pending = deferred<SessionState>();
		const { result } = await render(pending.promise);
		act(() => peerAnnounces({ accountId: "7" }));
		expect(router.refresh).not.toHaveBeenCalled();
		expect(nav.reload).not.toHaveBeenCalled();
		await act(async () => {
			pending.resolve(signedIn);
		});
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => peerAnnounces({ accountId: "7" }));
		act(() => peerAnnounces({ accountId: 7 }));
		act(() => peerAnnounces("7"));
		expect(router.refresh).not.toHaveBeenCalled();
		expect(nav.reload).not.toHaveBeenCalled();
	});

	it("announces the account it sees once, and the guest only from the next document", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(FakeBroadcastChannel.sent).toEqual([{ accountId: "7" }]);
		await act(async () => {
			await result.current.signOut();
		});
		expect(FakeBroadcastChannel.sent).toEqual([{ accountId: "7" }]);
	});

	it("does not announce while unknown or unavailable", async () => {
		await render(unavailable);
		expect(FakeBroadcastChannel.sent).toEqual([]);
	});

	it("reloads a page restored from the back-forward cache, not a normal page show", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		act(() => {
			window.dispatchEvent(
				new PageTransitionEvent("pageshow", { persisted: false }),
			);
		});
		expect(nav.reload).not.toHaveBeenCalled();
		act(() => {
			window.dispatchEvent(
				new PageTransitionEvent("pageshow", { persisted: true }),
			);
		});
		expect(nav.reload).toHaveBeenCalledTimes(1);
	});

	it("leaves a restored guest page alone: it holds no identity to clear and keeps the back-forward cache", async () => {
		const { result } = await render(guest);
		await waitFor(() => expect(result.current.status).toBe("guest"));
		act(() => {
			window.dispatchEvent(
				new PageTransitionEvent("pageshow", { persisted: true }),
			);
		});
		expect(nav.reload).not.toHaveBeenCalled();
	});

	it("takes answers again after the user cancels the beforeunload prompt of a document change", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		const guard = (event: BeforeUnloadEvent) => event.preventDefault();
		window.addEventListener("beforeunload", guard);
		try {
			await act(async () => {
				await result.current.signOut();
			});
			act(() => {
				window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
			});
			await flushMacrotasks();
			await adopt(guest);
			expect(result.current.status).toBe("guest");
		} finally {
			window.removeEventListener("beforeunload", guard);
		}
	});

	it("after a cancelled leave the revoked session is not kept: the server is asked again and signOut runs again", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		const guard = (event: BeforeUnloadEvent) => event.preventDefault();
		window.addEventListener("beforeunload", guard);
		try {
			await act(async () => {
				await result.current.signOut();
			});
			expect(router.refresh).not.toHaveBeenCalled();
			act(() => {
				window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
			});
			await flushMacrotasks();
			expect(router.refresh).toHaveBeenCalledTimes(1);
			await act(async () => {
				await result.current.signOut();
			});
			expect(api.logout).toHaveBeenCalledTimes(2);
		} finally {
			window.removeEventListener("beforeunload", guard);
		}
	});

	it("keeps ignoring answers when the document change goes ahead without a prompt", async () => {
		const { result, adopt } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		await act(async () => {
			await result.current.signOut();
		});
		act(() => {
			window.dispatchEvent(new Event("beforeunload", { cancelable: true }));
		});
		await flushMacrotasks();
		await adopt(guest);
		expect(result.current.status).toBe("signedIn");
	});
});

describe("switchRole with an identity the server just confirmed", () => {
	it("uses the confirmed identity, not the one the page loaded with, so a new seller reaches the seller workspace", async () => {
		const { result } = await render(signedIn);
		await waitFor(() => expect(result.current.isLoggedIn).toBe(true));
		expect(result.current.switchRole("seller")).toBe(false);
		let switched = false;
		act(() => {
			switched = result.current.switchRole("seller", {
				accountId: "7",
				isSeller: true,
			});
		});
		expect(switched).toBe(true);
		expect(result.current.activeRole).toBe("seller");
		expect(storage.get(ACTIVE_ROLE_STORAGE_KEY)).toBe("seller");
	});
});

describe("AuthProvider retries an unavailable session on reconnect and return", () => {
	it("asks again when the browser comes back online, before the backoff elapses", async () => {
		vi.useFakeTimers();
		api.fetchSession.mockResolvedValueOnce(buyer);
		const { result, adopt } = await render(unavailable);
		expect(api.fetchSession).not.toHaveBeenCalled();
		await act(async () => {
			window.dispatchEvent(new Event("online"));
			await vi.advanceTimersByTimeAsync(0);
		});
		expect(api.fetchSession).toHaveBeenCalledTimes(1);
		expect(router.refresh).toHaveBeenCalledTimes(1);
		expect(result.current.status).toBe("unavailable");
		vi.useRealTimers();
		await adopt(signedIn);
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
		const { result, adopt } = await render(unavailable);
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
		expect(router.refresh).not.toHaveBeenCalled();
		api.fetchSession.mockResolvedValueOnce(null);
		await act(async () => {
			window.dispatchEvent(new Event("online"));
			await vi.advanceTimersByTimeAsync(0);
		});
		expect(router.refresh).toHaveBeenCalledTimes(1);
		vi.useRealTimers();
		await adopt(guest);
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
