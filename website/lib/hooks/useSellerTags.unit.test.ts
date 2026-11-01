// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	fetchSellerTags: vi.fn(),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "unavailable", identity: null, activeRole: null }),
}));
vi.mock("@/lib/seller/repo", () => ({
	fetchSellerTags: mocks.fetchSellerTags,
}));

import { ZoneIdentityProvider } from "@/lib/auth/zone-context";
import { useSellerTags } from "./useSellerTags";

function wrapperIn(zone: boolean) {
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(
			QueryClientProvider,
			{ client: new QueryClient() },
			zone
				? React.createElement(ZoneIdentityProvider, {
						identity: { accountId: "7", isSeller: true },
						workspace: "seller",
						children,
					})
				: children,
		);
}

describe("useSellerTags gate", () => {
	it("runs on the first render inside the seller zone although the root answer is stale", async () => {
		mocks.fetchSellerTags.mockReset();
		mocks.fetchSellerTags.mockResolvedValue([{ id: "t1" }]);
		const { result } = renderHook(() => useSellerTags(), {
			wrapper: wrapperIn(true),
		});
		await waitFor(() => expect(result.current).toHaveLength(1));
		expect(mocks.fetchSellerTags).toHaveBeenCalledTimes(1);
	});

	it("stays idle outside a zone while the root has no seller role", async () => {
		mocks.fetchSellerTags.mockReset();
		renderHook(() => useSellerTags(), { wrapper: wrapperIn(false) });
		await new Promise((r) => setTimeout(r, 20));
		expect(mocks.fetchSellerTags).not.toHaveBeenCalled();
	});
});
