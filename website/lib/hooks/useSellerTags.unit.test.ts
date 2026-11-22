// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	listTags: vi.fn(),
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "unavailable", identity: null, activeRole: null }),
}));
vi.mock("@/lib/seller/crm", () => ({
	listTags: mocks.listTags,
}));
vi.mock("./useCrmStoreId", () => ({
	useCrmStoreId: () => ({ storeId: "9", loading: false }),
}));
vi.mock("@/lib/i18n/ui-text", () => ({
	useDefaultErrorText: () => (code: Code) => `default-${code}`,
}));

import { ZoneIdentityProvider } from "@/lib/auth/zone-context";
import { useSellerTags } from "./useSellerTags";

function wrapperIn(zone: boolean) {
	return ({ children }: { children: React.ReactNode }) =>
		React.createElement(
			QueryClientProvider,
			{
				client: new QueryClient({
					defaultOptions: { queries: { retry: false } },
				}),
			},
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
		mocks.listTags.mockReset();
		mocks.listTags.mockResolvedValue([{ id: "t1", name: "a" }]);
		const { result } = renderHook(() => useSellerTags(), {
			wrapper: wrapperIn(true),
		});
		await waitFor(() => expect(result.current.tags).toHaveLength(1));
		expect(mocks.listTags).toHaveBeenCalledTimes(1);
		expect(mocks.listTags).toHaveBeenCalledWith("9");
		expect(result.current.error).toBeNull();
	});

	it("stays idle outside a zone while the root has no seller role", async () => {
		mocks.listTags.mockReset();
		renderHook(() => useSellerTags(), { wrapper: wrapperIn(false) });
		await new Promise((r) => setTimeout(r, 20));
		expect(mocks.listTags).not.toHaveBeenCalled();
	});

	it("reports a read failure as an error instead of an empty pool", async () => {
		mocks.listTags.mockReset();
		mocks.listTags.mockRejectedValue(
			new ConnectError("boom", Code.Unavailable),
		);
		const { result } = renderHook(() => useSellerTags(), {
			wrapper: wrapperIn(true),
		});
		await waitFor(() => expect(result.current.error).not.toBeNull());
		expect(result.current.tags).toEqual([]);
	});
});
