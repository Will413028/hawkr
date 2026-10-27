// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	clerk: { signOut: vi.fn(async () => {}) },
	state: { isLoaded: true, isSignedIn: true, userId: "user" },
	callback: vi.fn(() => null),
	fetchSession: vi.fn(),
	SESSION_UNAVAILABLE: Symbol("unavailable"),
	router: { replace: vi.fn() },
	completeSignIn: vi.fn(),
	completeLogin: vi.fn(),
	error: vi.fn(),
}));
vi.mock("@clerk/nextjs", () => ({
	useClerk: () => mocks.clerk,
	useAuth: () => mocks.state,
	AuthenticateWithRedirectCallback: mocks.callback,
}));
vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/shared/AuthBrandLoading", () => ({
	default: () => null,
}));
vi.mock("@/lib/api/auth", () => ({
	completeSignIn: mocks.completeSignIn,
	fetchSession: mocks.fetchSession,
	SESSION_UNAVAILABLE: mocks.SESSION_UNAVAILABLE,
}));
vi.mock("@/lib/auth/useCompleteLogin", () => ({
	useCompleteLogin: () => ({ completeLogin: mocks.completeLogin }),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => mocks.router,
}));
vi.mock("@/lib/feedback/toast", () => ({ toast: { error: mocks.error } }));

import { AuthProvider } from "@/lib/auth/context";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import SsoCallbackPage from "./SsoCallbackPage";

/** 伺服器的拒絕：開發者訊息是英文，使用者看到的是 LocalizedMessage。 */
function suspended() {
	return new ConnectError(
		"account suspended",
		Code.PermissionDenied,
		undefined,
		[
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "帳號已停權" },
			},
		],
	);
}

function page() {
	return React.createElement(
		QueryClientProvider,
		{ client: new QueryClient() },
		React.createElement(
			IntlTestWrapper,
			null,
			React.createElement(
				AuthProvider,
				null,
				React.createElement(SsoCallbackPage),
			),
		),
	);
}
beforeEach(() => {
	mocks.state = { isLoaded: true, isSignedIn: true, userId: "user" };
	mocks.fetchSession.mockResolvedValue(mocks.SESSION_UNAVAILABLE);
	vi.stubGlobal("localStorage", {
		getItem: () => null,
		setItem: vi.fn(),
		removeItem: vi.fn(),
		length: 0,
	});
});

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});
it("ends a suspended Clerk session and preserves the server message across navigation", async () => {
	mocks.completeSignIn.mockRejectedValue(suspended());
	render(page());
	await waitFor(() => expect(mocks.router.replace).toHaveBeenCalled());
	expect(mocks.clerk.signOut).toHaveBeenCalledOnce();
	expect(mocks.error).toHaveBeenCalledWith("帳號已停權");
	expect(mocks.completeLogin).not.toHaveBeenCalled();
});
it("retains Clerk on temporary failure", async () => {
	mocks.completeSignIn.mockRejectedValue(
		new ConnectError("offline", Code.Unavailable),
	);
	render(page());
	await waitFor(() => expect(mocks.router.replace).toHaveBeenCalled());
	expect(mocks.clerk.signOut).not.toHaveBeenCalled();
});

it.each([
	"provider",
	"callback",
])("deduplicates denial when %s resolves first and never remounts redirect callback", async (first) => {
	let rejectProbe!: (error: Error) => void;
	let rejectCallback!: (error: Error) => void;
	mocks.fetchSession.mockReturnValue(
		new Promise((_, reject) => {
			rejectProbe = reject;
		}),
	);
	mocks.completeSignIn.mockReturnValue(
		new Promise((_, reject) => {
			rejectCallback = reject;
		}),
	);
	const view = render(page());
	const denial = suspended();
	await act(async () => {
		(first === "provider" ? rejectProbe : rejectCallback)(denial);
	});
	mocks.state = { isLoaded: true, isSignedIn: false, userId: "" };
	view.rerender(page());
	await act(async () => {
		(first === "provider" ? rejectCallback : rejectProbe)(denial);
	});
	await waitFor(() => expect(mocks.router.replace).toHaveBeenCalledOnce());
	expect(mocks.clerk.signOut).toHaveBeenCalledOnce();
	expect(mocks.error).toHaveBeenCalledOnce();
	expect(mocks.error).toHaveBeenCalledWith("帳號已停權");
	expect(mocks.callback).not.toHaveBeenCalled();
});
