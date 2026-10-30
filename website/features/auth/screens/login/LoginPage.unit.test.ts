// @vitest-environment jsdom

import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	search: "",
	auth: {} as Record<string, unknown>,
	navigateDocument: vi.fn(() => true),
	trackLogin: vi.fn(),
	switchRole: vi.fn(),
}));

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useSearchParams: () => new URLSearchParams(mocks.search),
}));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => mocks.auth }));
vi.mock("@/lib/navigation/observability", () => ({
	navigateDocument: mocks.navigateDocument,
}));
vi.mock("@/lib/analytics/events", () => ({ trackLogin: mocks.trackLogin }));
vi.mock("@/components/shared/AppIcon", () => ({
	default: () => React.createElement("span"),
}));
vi.mock("@/components/shared/AuthBrandLoading", () => ({
	default: () => React.createElement("div", { "data-testid": "auth-loading" }),
}));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children }: { children: React.ReactNode }) =>
		React.createElement("a", null, children),
}));
vi.mock("@/features/auth/components/login/WebViewBrowserGuide", () => ({
	default: () => null,
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import LoginPage from "./LoginPage";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

function guest() {
	mocks.auth = {
		identity: null,
		isLoggedIn: false,
		hydrated: true,
		activeRole: null,
		switchRole: mocks.switchRole,
	};
}

beforeEach(() => {
	mocks.search = "";
	mocks.navigateDocument.mockClear();
	mocks.trackLogin.mockClear();
	guest();
});
afterEach(cleanup);

function click(label: string) {
	fireEvent.click(screen.getByRole("button", { name: label }));
}

describe("LoginPage: the buttons hand the browser to the monolith's OAuth start", () => {
	it("offers Google and LINE, and no Apple", () => {
		render(React.createElement(LoginPage));
		expect(screen.getByRole("button", { name: "用 Google 繼續" })).toBeTruthy();
		expect(screen.getByRole("button", { name: "用 LINE 繼續" })).toBeTruthy();
		expect(document.body.textContent).not.toContain("Apple");
	});

	it("Google: a whole-page navigation to /api/auth/google/start whose return_to is this login page with the picked workspace and destination", () => {
		mocks.search = "role=seller&redirect=%2Fseller-center%2Forders";
		render(React.createElement(LoginPage));
		click("用 Google 繼續");
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(1);
		const [target] = mocks.navigateDocument.mock.calls[0] as unknown as [
			string,
		];
		const url = new URL(target, "https://hawkr.local");
		expect(url.pathname).toBe("/api/auth/google/start");
		const returnTo = url.searchParams.get("return_to") ?? "";
		// the monolith only accepts a relative path that starts with a single "/"
		expect(returnTo.startsWith("/")).toBe(true);
		expect(returnTo.startsWith("//")).toBe(false);
		const back = new URL(returnTo, "https://hawkr.local");
		expect(back.pathname).toBe("/login");
		expect(back.searchParams.get("role")).toBe("seller");
		expect(back.searchParams.get("redirect")).toBe("/seller-center/orders");
		expect(back.searchParams.get("provider")).toBe("google");
	});

	it("LINE goes to /api/auth/line/start", () => {
		render(React.createElement(LoginPage));
		click("用 LINE 繼續");
		const [target] = mocks.navigateDocument.mock.calls[0] as unknown as [
			string,
		];
		expect(new URL(target, "https://hawkr.local").pathname).toBe(
			"/api/auth/line/start",
		);
	});

	it("a second click while leaving does not start a second flow", () => {
		render(React.createElement(LoginPage));
		click("用 LINE 繼續");
		click("用 LINE 繼續");
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(1);
	});
});

describe("LoginPage: ?error= from the monolith's failed callback", () => {
	it.each([
		["access_denied", "你取消了授權，請再試一次"],
		["unavailable", "登入服務暫時無法使用，請稍後再試"],
		["invalid_flow", "登入失敗，請稍後再試"],
		["provider_error", "登入失敗，請稍後再試"],
		["login_failed", "登入失敗，請稍後再試"],
		["login_required", "登入失敗，請稍後再試"],
	])("%s shows its sentence", (code, text) => {
		mocks.search = `error=${code}`;
		render(React.createElement(LoginPage));
		expect(screen.getByRole("alert").textContent).toBe(text);
	});

	it("an unknown code shows nothing", () => {
		mocks.search = "error=whatever";
		render(React.createElement(LoginPage));
		expect(screen.queryByRole("alert")).toBeNull();
	});
});

describe("LoginPage: back from the provider (the monolith's 302 lands here)", () => {
	it("an account that GetSession resolved leaves for the requested destination and records the login once", () => {
		mocks.search = "role=buyer&redirect=%2Faccount%2Forders&provider=line";
		mocks.auth = {
			identity: { accountId: "7", isSeller: false },
			isLoggedIn: true,
			hydrated: true,
			activeRole: "buyer",
			switchRole: mocks.switchRole,
		};
		render(React.createElement(LoginPage));
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(1);
		const [target] = mocks.navigateDocument.mock.calls[0] as unknown as [
			string,
		];
		expect(target).toContain("/account/orders");
		expect(mocks.trackLogin).toHaveBeenCalledExactlyOnceWith("line");
	});

	it("an already signed-in visitor opening /login is sent on without a login event", () => {
		mocks.search = "role=buyer&redirect=%2Faccount%2Forders";
		mocks.auth = {
			identity: { accountId: "7", isSeller: false },
			isLoggedIn: true,
			hydrated: true,
			activeRole: "buyer",
			switchRole: mocks.switchRole,
		};
		render(React.createElement(LoginPage));
		expect(mocks.navigateDocument).toHaveBeenCalledTimes(1);
		expect(mocks.trackLogin).not.toHaveBeenCalled();
	});

	it("keeps the loading overlay up until GetSession answers", () => {
		mocks.search = "provider=google";
		mocks.auth = {
			identity: null,
			isLoggedIn: false,
			hydrated: false,
			activeRole: null,
			switchRole: mocks.switchRole,
		};
		render(React.createElement(LoginPage));
		expect(screen.getByTestId("auth-loading")).toBeTruthy();
	});
});
