import { beforeEach, describe, expect, it, vi } from "vitest";

const rpc = vi.hoisted(() => ({ getSession: vi.fn() }));
const store = vi.hoisted(() => ({ cookies: vi.fn() }));
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ cookies: store.cookies }));
vi.mock("@connectrpc/connect", async (importOriginal) => ({
	...(await importOriginal<typeof import("@connectrpc/connect")>()),
	createClient: () => rpc,
}));
const transport = vi.hoisted(() => ({ authedTransport: vi.fn(() => ({})) }));
vi.mock("@/lib/connect/transport", () => ({
	authedTransport: transport.authedTransport,
	SUPPRESS_AUTH_EXPIRED: { id: Symbol("suppress"), defaultValue: false },
}));

import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { Role } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { SESSION_COOKIE_NAME } from "./session-cookie";

const { classifySession, verifySession } = await import("./dal");

function withCookie(value: string | null) {
	store.cookies.mockResolvedValue({
		get: (name: string) =>
			value !== null && name === SESSION_COOKIE_NAME
				? { name, value }
				: undefined,
	});
}

// `verifySession` is wrapped in React `cache()`, which outside a request scope
// does not memoise (react.react-server), so each call below is a fresh read.
describe("verifySession", () => {
	beforeEach(() => {
		rpc.getSession.mockReset();
		store.cookies.mockReset();
		transport.authedTransport.mockClear();
	});

	it("a visitor without the cookie is a guest and the monolith is not asked", async () => {
		withCookie(null);
		expect(await verifySession()).toEqual({ status: "guest" });
		expect(rpc.getSession).not.toHaveBeenCalled();
	});

	it("401 from GetSession is a guest", async () => {
		withCookie("opaque");
		rpc.getSession.mockRejectedValue(
			new ConnectError("no session", Code.Unauthenticated),
		);
		expect(await verifySession()).toEqual({ status: "guest" });
	});

	it("an account is signedIn, and a seller role is carried", async () => {
		withCookie("opaque");
		rpc.getSession.mockResolvedValue({
			account: { id: "8", roles: [Role.SELLER] },
		});
		expect(await verifySession()).toEqual({
			status: "signedIn",
			identity: { accountId: "8", isSeller: true },
		});
	});

	it.each([
		["5xx", new ConnectError("boom", Code.Internal)],
		["network", new ConnectError("down", Code.Unavailable)],
		["timeout", new ConnectError("slow", Code.DeadlineExceeded)],
		["abort", new DOMException("timed out", "TimeoutError")],
		["unexpected error", new Error("socket hang up")],
	])("%s is unavailable, never a guest", async (_name, failure) => {
		withCookie("opaque");
		rpc.getSession.mockRejectedValue(failure);
		expect(await verifySession()).toEqual({ status: "unavailable" });
	});

	it("an answer without an account is unavailable", async () => {
		withCookie("opaque");
		rpc.getSession.mockResolvedValue({ account: undefined });
		expect(await verifySession()).toEqual({ status: "unavailable" });
	});

	it("only reads: forwards the cookie and a bounded signal to GetSession", async () => {
		withCookie("opaque");
		rpc.getSession.mockResolvedValue({ account: { id: "7", roles: [] } });
		await verifySession();
		expect(transport.authedTransport).toHaveBeenCalledWith(
			expect.objectContaining({
				cookie: `${SESSION_COOKIE_NAME}=opaque`,
			}),
		);
		const callOptions = rpc.getSession.mock.calls[0]?.[1] as {
			signal?: AbortSignal;
		};
		expect(callOptions.signal).toBeInstanceOf(AbortSignal);
	});

	it("Next's dynamic-rendering signal from cookies() is not swallowed", async () => {
		const signal = Object.assign(new Error("Dynamic server usage"), {
			digest: "DYNAMIC_SERVER_USAGE",
		});
		store.cookies.mockRejectedValue(signal);
		await expect(verifySession()).rejects.toBe(signal);
	});
});

describe("classifySession", () => {
	it("403 is a guest that carries the monolith's refusal message", async () => {
		const state = await classifySession(() =>
			Promise.reject(
				new ConnectError("suspended", Code.PermissionDenied, undefined, [
					{
						desc: LocalizedMessageSchema,
						value: { locale: "zh-TW", message: "帳號已停權" },
					},
				]),
			),
		);
		expect(state).toEqual({
			status: "guest",
			refusal: { message: "帳號已停權" },
		});
	});

	it("403 without a LocalizedMessage carries a null message", async () => {
		const state = await classifySession(() =>
			Promise.reject(new ConnectError("no", Code.PermissionDenied)),
		);
		expect(state).toEqual({ status: "guest", refusal: { message: null } });
	});
});
