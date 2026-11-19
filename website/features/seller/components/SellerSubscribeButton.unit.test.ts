/** @vitest-environment jsdom */

import { cleanup, render } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const harness = vi.hoisted(() => ({ useSellerSubscription: vi.fn() }));

vi.mock("@/components/shared/AuthGateWaitModal", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ activeRole: "buyer", status: "signedIn" }),
}));
vi.mock("@/lib/auth/useAuthGate", () => ({
	useAuthGate: () => ({ authGate: vi.fn(), authWaiting: false }),
}));
vi.mock("@/lib/hooks/useFollowedSellers", () => ({
	useSetFollowedSeller: () => vi.fn(),
}));
vi.mock("@/lib/state/use-login-required-overlay", () => ({
	useLoginRequiredOverlay: () => vi.fn(),
}));
vi.mock("@/lib/hooks/useSellerSubscription", () => ({
	useSellerSubscription: (...args: unknown[]) => {
		harness.useSellerSubscription(...args);
		return { active: false, ready: true, pending: false, toggle: vi.fn() };
	},
}));

import SellerSubscribeButton from "./SellerSubscribeButton";

afterEach(() => {
	cleanup();
	harness.useSellerSubscription.mockClear();
});

describe("SellerSubscribeButton follow read", () => {
	it("does not enable the subscription read in previewMode", () => {
		render(
			React.createElement(SellerSubscribeButton, {
				sellerId: "1",
				label: "訂閱",
				previewMode: true,
			}),
		);
		expect(harness.useSellerSubscription).toHaveBeenCalledWith("1", {
			enabled: false,
		});
	});

	it("enables the subscription read outside previewMode", () => {
		render(
			React.createElement(SellerSubscribeButton, {
				sellerId: "1",
				label: "訂閱",
			}),
		);
		expect(harness.useSellerSubscription).toHaveBeenCalledWith("1", {
			enabled: true,
		});
	});
});
