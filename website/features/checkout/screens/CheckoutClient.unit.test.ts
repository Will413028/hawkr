// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({
	value: {
		identity: null as { accountId: string; isSeller: boolean } | null,
		isLoggedIn: false,
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
vi.mock("@/lib/payment/useNativePayAvailability", () => ({
	useNativePayAvailability: () => ({
		applePay: false,
		googlePay: false,
		loaded: true,
	}),
}));

import CheckoutClient from "./CheckoutClient";

afterEach(cleanup);

describe("CheckoutClient while the session is not signed in", () => {
	it("settles instead of re-rendering itself forever", async () => {
		let commits = 0;
		const client = new QueryClient();
		render(
			React.createElement(
				QueryClientProvider,
				{ client },
				React.createElement(
					React.Profiler,
					{
						id: "checkout",
						onRender: () => {
							commits++;
							if (commits > 100) throw new Error("render loop");
						},
					},
					React.createElement(CheckoutClient),
				),
			),
		);
		await screen.findByText("請先登入。");
		await new Promise((done) => setTimeout(done, 100));
		const settled = commits;
		await new Promise((done) => setTimeout(done, 100));
		expect(commits).toBe(settled);
		expect(settled).toBeLessThan(10);
	});
});
