// @vitest-environment jsdom

import { Code, ConnectError } from "@connectrpc/connect";
import {
	CustomerSort,
	CustomerType,
} from "@hawkr/api/gen/hawkr/crm/v1alpha1/crm_pb";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	listCustomers: vi.fn(),
	store: {
		storeId: "9" as string | undefined,
		loading: false,
		error: null as Error | null,
	},
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ status: "unavailable", identity: null, activeRole: null }),
}));
vi.mock("@/lib/seller/crm", () => ({ listCustomers: mocks.listCustomers }));
vi.mock("./useCrmStoreId", () => ({
	useCrmStoreId: () => mocks.store,
}));
vi.mock("@/lib/i18n/ui-text", () => ({
	useDefaultErrorText: () => (code: Code) => `default-${code}`,
}));

import { ZoneIdentityProvider } from "@/lib/auth/zone-context";
import { useSellerCustomers } from "./useSellerCustomers";

function Wrapper({ children }: { children: React.ReactNode }) {
	const [client] = React.useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	return React.createElement(
		QueryClientProvider,
		{ client },
		React.createElement(ZoneIdentityProvider, {
			identity: { accountId: "7", isSeller: true },
			workspace: "seller",
			children,
		}),
	);
}

const row = (id: string) => ({ id, name: id, tags: [] });

describe("useSellerCustomers", () => {
	beforeEach(() => {
		mocks.listCustomers.mockReset();
		mocks.store.storeId = "9";
		mocks.store.error = null;
	});

	it("surfaces a store read failure as an error instead of an empty list", async () => {
		mocks.store.storeId = undefined;
		mocks.store.error = new ConnectError("x", Code.Unavailable);
		const { result } = renderHook(() => useSellerCustomers({}), {
			wrapper: Wrapper,
		});
		await waitFor(() =>
			expect(result.current.error).toBe(`default-${Code.Unavailable}`),
		);
		expect(result.current.data).toBeNull();
		expect(mocks.listCustomers).not.toHaveBeenCalled();
	});

	it("passes filters, sort and the store id, and pages with the page token", async () => {
		mocks.listCustomers.mockImplementation(async (...args: unknown[]) => {
			const token = (args[1] as { pageToken?: string } | undefined)?.pageToken;
			return token
				? { customers: [row("2")], nextPageToken: "" }
				: { customers: [row("1")], nextPageToken: "p2" };
		});
		const params = {
			type: CustomerType.VIP,
			tagId: "t1",
			sort: CustomerSort.TOTAL_SPENT,
			descending: true,
		};
		const { result } = renderHook(() => useSellerCustomers(params), {
			wrapper: Wrapper,
		});
		await waitFor(() =>
			expect(result.current.data?.map((c) => c.id)).toEqual(["1"]),
		);
		expect(result.current.hasMore).toBe(true);
		expect(mocks.listCustomers).toHaveBeenCalledWith("9", {
			...params,
			pageToken: "",
		});
		await act(async () => {
			await result.current.loadMore();
		});
		await waitFor(() =>
			expect(result.current.data?.map((c) => c.id)).toEqual(["1", "2"]),
		);
		expect(result.current.hasMore).toBe(false);
		expect(mocks.listCustomers).toHaveBeenLastCalledWith("9", {
			...params,
			pageToken: "p2",
		});
	});

	it("distinguishes a read failure (data null + error text) from an empty list", async () => {
		mocks.listCustomers.mockRejectedValueOnce(
			new ConnectError("down", Code.Unavailable),
		);
		const failed = renderHook(() => useSellerCustomers({}), {
			wrapper: Wrapper,
		});
		await waitFor(() => expect(failed.result.current.error).not.toBeNull());
		expect(failed.result.current.data).toBeNull();

		mocks.listCustomers.mockResolvedValueOnce({
			customers: [],
			nextPageToken: "",
		});
		const empty = renderHook(() => useSellerCustomers({}), {
			wrapper: Wrapper,
		});
		await waitFor(() => expect(empty.result.current.data).toEqual([]));
		expect(empty.result.current.error).toBeNull();
	});
});
