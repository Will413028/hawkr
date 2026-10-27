// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	auth: {} as Record<string, unknown>,
	storedRole: null as string | null,
	switchRole: vi.fn(() => false),
	replace: vi.fn(),
	reload: vi.fn(),
}));

vi.mock("@sentry/nextjs", () => ({ captureMessage: () => {} }));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => mocks.auth }));
vi.mock("@/lib/auth/storage", () => ({
	getActiveRole: () => mocks.storedRole,
}));
vi.mock("@/components/shared/ErrorState", () => ({ default: () => null }));
vi.mock("@/components/shared/RouteLoadingSkeleton", () => ({
	default: () => React.createElement("div", { "data-testid": "pending" }),
}));

import RoleZoneGuard from "@/components/shared/RoleZoneGuard";

function signedIn(over: {
	activeRole: "buyer" | "seller" | null;
	hasSeller: boolean;
}) {
	mocks.auth = {
		...over,
		hydrated: true,
		sessionSettled: true,
		isLoggedIn: true,
		switchRole: mocks.switchRole,
	};
}

function renderZone(required: "buyer" | "seller" | "account") {
	return render(
		React.createElement(RoleZoneGuard, {
			required,
			children: React.createElement("p", null, "zone content"),
		}),
	);
}

describe("RoleZoneGuard 的 account zone：登入即可，不看工作區", () => {
	beforeEach(() => {
		mocks.storedRole = null;
		mocks.switchRole.mockClear();
		mocks.replace.mockClear();
		mocks.reload.mockClear();
		Object.defineProperty(window, "location", {
			configurable: true,
			value: {
				...window.location,
				replace: mocks.replace,
				reload: mocks.reload,
			},
		});
	});
	afterEach(cleanup);

	it.each([
		["店家全關的買家視角（沒有賣家角色）", "buyer", false],
		["賣家視角", "seller", true],
		["有賣家角色但在買家視角", "buyer", true],
	] as const)("%s：直接放行，標記是 account，不切換也不重載", (_name, activeRole, hasSeller) => {
		signedIn({ activeRole, hasSeller });
		const { container } = renderZone("account");
		expect(screen.getByText("zone content")).toBeTruthy();
		expect(
			container.querySelector('[data-hk-role-zone-ready="account"]'),
		).not.toBeNull();
		expect(mocks.switchRole).not.toHaveBeenCalled();
		expect(mocks.replace).not.toHaveBeenCalled();
		expect(mocks.reload).not.toHaveBeenCalled();
	});

	it("有 Clerk session 但單體不認（沒有登入的帳號）：走既有的 fallback，不放行", () => {
		mocks.auth = {
			activeRole: null,
			hasSeller: false,
			hydrated: true,
			sessionSettled: true,
			isLoggedIn: false,
			switchRole: mocks.switchRole,
		};
		renderZone("account");
		expect(screen.queryByText("zone content")).toBeNull();
		expect(mocks.replace).toHaveBeenCalledWith("/");
		expect(mocks.switchRole).not.toHaveBeenCalled();
	});

	it("對照：seller zone 仍要求賣家角色，沒有就導回", () => {
		signedIn({ activeRole: "buyer", hasSeller: false });
		renderZone("seller");
		expect(screen.queryByText("zone content")).toBeNull();
		expect(mocks.replace).toHaveBeenCalledWith("/");
	});

	it("對照：seller zone 在買家視角且有賣家角色時會切換工作區", () => {
		signedIn({ activeRole: "buyer", hasSeller: true });
		renderZone("seller");
		expect(mocks.switchRole).toHaveBeenCalledWith("seller");
	});
});
