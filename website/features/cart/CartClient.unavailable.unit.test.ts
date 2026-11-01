// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.hoisted(() => {
	const memory = new Map<string, string>();
	Object.defineProperty(globalThis, "localStorage", {
		configurable: true,
		value: {
			getItem: (k: string) => memory.get(k) ?? null,
			setItem: (k: string, v: string) => void memory.set(k, v),
			removeItem: (k: string) => void memory.delete(k),
		},
	});
});

const auth = vi.hoisted(() => ({
	value: {
		identity: null as { accountId: string; isSeller: boolean } | null,
		status: "unavailable",
		activeRole: null as string | null,
	},
}));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => auth.value }));
vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("@/components/shared/Link", () => ({
	default: (props: { children?: React.ReactNode }) =>
		React.createElement("a", null, props.children),
}));
vi.mock("@/lib/seller/useStoreCardViews", () => ({
	useStoreCardViews: () => ({ views: [] }),
}));
vi.mock("@/components/shared/ErrorState", () => ({
	default: () => React.createElement("p", null, "retry-state"),
}));

import CartClient from "./CartClient";

afterEach(cleanup);

function renderCart() {
	render(
		React.createElement(
			QueryClientProvider,
			{ client: new QueryClient() },
			React.createElement(CartClient),
		),
	);
}

describe("CartClient when the session answer is unavailable", () => {
	it("shows the retry state, never the guest gate", async () => {
		auth.value = { identity: null, status: "unavailable", activeRole: null };
		renderCart();
		await screen.findByText("retry-state");
		expect(screen.queryByText("登入後才能查看購物車")).toBeNull();
	});

	it("still shows the guest gate for a confirmed guest", async () => {
		auth.value = { identity: null, status: "guest", activeRole: null };
		renderCart();
		await screen.findByText("登入後才能查看購物車");
	});
});
