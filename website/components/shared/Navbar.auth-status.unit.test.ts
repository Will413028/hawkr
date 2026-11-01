/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AuthStatus } from "@/lib/auth/session-state";

const auth = vi.hoisted(() => ({
	value: {
		status: "unknown" as AuthStatus,
		isLoggedIn: false,
		activeRole: null as "buyer" | "seller" | null,
		signOut: vi.fn(),
	},
}));

vi.mock("@/lib/auth/context", () => ({
	useAuth: () => auth.value,
	useOptionalAuth: () => auth.value,
}));
vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
	usePathname: () => "/",
	useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({
		push: vi.fn(),
		replace: vi.fn(),
		prefetch: vi.fn(),
	}),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import Navbar from "./Navbar";
import { ViewTransitions } from "./view-transitions";

function renderNavbar() {
	return render(
		React.createElement(
			QueryClientProvider,
			{ client: new QueryClient() },
			React.createElement(
				IntlTestWrapper,
				null,
				React.createElement(ViewTransitions, null, React.createElement(Navbar)),
			),
		),
	);
}

afterEach(() => {
	cleanup();
	auth.value = {
		status: "unknown",
		isLoggedIn: false,
		activeRole: null,
		signOut: vi.fn(),
	};
});

describe("Navbar right rail by auth status", () => {
	it("unknown renders the neutral skeleton: no sign-in link, no account menu", () => {
		renderNavbar();
		expect(screen.queryByText("登入")).toBeNull();
		expect(screen.queryByTestId("seller-center-link")).toBeNull();
		expect(document.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
			0,
		);
	});

	it("unavailable keeps the same skeleton", () => {
		auth.value = { ...auth.value, status: "unavailable" };
		renderNavbar();
		expect(screen.queryByText("登入")).toBeNull();
		expect(document.querySelectorAll(".animate-pulse").length).toBeGreaterThan(
			0,
		);
	});

	it("guest renders the sign-in link and no skeleton", () => {
		auth.value = { ...auth.value, status: "guest" };
		renderNavbar();
		expect(screen.getByText("登入")).toBeTruthy();
		expect(document.querySelectorAll(".animate-pulse").length).toBe(0);
	});
});
