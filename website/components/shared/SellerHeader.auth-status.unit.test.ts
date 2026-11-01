/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AuthStatus } from "@/lib/auth/session-state";

const auth = vi.hoisted(() => ({
	value: {
		status: "unknown" as AuthStatus,
		activeRole: null as "buyer" | "seller" | null,
		hasSeller: false,
		signOut: vi.fn(),
	},
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => auth.value,
	useOptionalAuth: () => auth.value,
}));
vi.mock("@/components/shared/useSellerIdentity", () => ({
	useSellerIdentity: () => ({
		setupIncomplete: false,
		avatar: null,
		ownerName: "",
		sellerId: "s1",
		storeName: "",
	}),
}));
vi.mock("@/components/shared/ChatBubble", () => ({ default: () => null }));
vi.mock("@/components/shared/NotificationBell", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/ThemeToggle", () => ({ default: () => null }));
vi.mock("@/lib/intl/navigation", () => ({ usePathname: () => "/" }));
vi.mock("@/components/shared/Link", () => ({
	default: ({
		href,
		children,
		...rest
	}: { href: string; children?: React.ReactNode } & Record<string, unknown>) =>
		React.createElement("a", { href, ...rest }, children),
}));

import { SellerHeader } from "./SellerNavbar";

afterEach(cleanup);

describe("SellerHeader auth status", () => {
	it("holds a neutral skeleton while the session answer is unknown", () => {
		auth.value = {
			...auth.value,
			status: "unknown",
			activeRole: null,
			hasSeller: false,
		};
		render(React.createElement(SellerHeader));
		expect(screen.queryByText(/繼續建立賣場/)).toBeNull();
		expect(screen.queryByRole("button", { name: /帳號|選單/ })).toBeNull();
		expect(screen.queryByText("登入")).toBeNull();
		expect(screen.queryByText("上架新商品")).toBeNull();
		expect(
			screen.getAllByTestId("seller-header-skeleton").length,
		).toBeGreaterThan(0);
	});

	it("shows the normal header for a signed-in seller", () => {
		auth.value = {
			...auth.value,
			status: "signedIn",
			activeRole: "seller",
			hasSeller: true,
		};
		render(React.createElement(SellerHeader));
		expect(screen.queryByTestId("seller-header-skeleton")).toBeNull();
		expect(screen.queryByText(/繼續建立賣場/)).toBeNull();
		expect(screen.getByText(/上架新商品/)).toBeTruthy();
	});
});
