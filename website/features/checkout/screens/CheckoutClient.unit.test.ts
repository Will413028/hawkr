// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const auth = vi.hoisted(() => ({
	value: {
		identity: null as { accountId: string; isSeller: boolean } | null,
		status: "unknown",
		activeRole: null as string | null,
	},
}));
vi.mock("@/lib/auth/context", () => ({ useAuth: () => auth.value }));
vi.mock("@/lib/auth/zone-identity", () => ({
	useZoneIdentity: () => ({ accountId: "7", isSeller: false }),
}));
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

import CheckoutClient, { transientReducer } from "./CheckoutClient";

afterEach(cleanup);

describe("CheckoutClient before the root auth state has an answer", () => {
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
		await screen.findByText("正在讀取結帳資料…");
		await new Promise((done) => setTimeout(done, 100));
		const settled = commits;
		await new Promise((done) => setTimeout(done, 100));
		expect(commits).toBe(settled);
		expect(settled).toBeLessThan(10);
	});
});

describe("transientReducer SET", () => {
	it("returns the same state when the patch changes no field", () => {
		const state = transientReducer(undefined as never, { type: "NEW_ATTEMPT" });
		expect(
			transientReducer(state, { type: "SET", patch: { submitting: false } }),
		).toBe(state);
		expect(
			transientReducer(state, {
				type: "SET",
				patch: { preview: null, previewNonce: 0 },
			}),
		).toBe(state);
	});

	it("returns a new state when any field changes", () => {
		const state = transientReducer(undefined as never, { type: "NEW_ATTEMPT" });
		const next = transientReducer(state, {
			type: "SET",
			patch: { submitting: true },
		});
		expect(next).not.toBe(state);
		expect(next.submitting).toBe(true);
	});
});
