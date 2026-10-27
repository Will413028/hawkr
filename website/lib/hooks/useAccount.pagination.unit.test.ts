/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const favoriteRepo = vi.hoisted(() => ({
	fetchFavorites: vi.fn(),
	removeFavorite: vi.fn(),
}));
vi.mock("@/lib/favorites/repo", () => ({
	...favoriteRepo,
	batchCheckFavoriteProductIds: vi.fn(),
}));
const historyRepo = vi.hoisted(() => ({
	fetchHistory: vi.fn(),
	clearHistory: vi.fn(),
}));
vi.mock("@/lib/visits/repo", () => historyRepo);
const followRepo = vi.hoisted(() => ({
	fetchFollowing: vi.fn(),
	setSellerFollow: vi.fn(),
}));
vi.mock("@/lib/follow/repo", () => followRepo);
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ hydrated: true, isLoggedIn: true, activeRole: "buyer" }),
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "42", activeRole: "buyer" }),
}));

import { queryKeys } from "@/lib/query/keys";
import { useFavorites, useFollowing, useHistory } from "./useAccount";

afterEach(() => {
	cleanup();
	favoriteRepo.fetchFavorites.mockReset();
	historyRepo.fetchHistory.mockReset();
	followRepo.fetchFollowing.mockReset();
	followRepo.setSellerFollow.mockReset();
});

describe("buyer cursor list", () => {
	it("retains loaded pages on invalidation and ignores duplicate load-more taps", async () => {
		let finishSecond:
			| ((value: {
					items: Array<{ product: { id: string } }>;
					nextPageToken: string;
			  }) => void)
			| undefined;
		let secondCalls = 0;
		favoriteRepo.fetchFavorites.mockImplementation(
			({ pageToken }: { pageToken: string }) => {
				if (!pageToken)
					return Promise.resolve({
						items: [{ product: { id: "1" } }],
						nextPageToken: "next",
					});
				secondCalls++;
				if (secondCalls > 1)
					return Promise.resolve({
						items: [{ product: { id: "2" } }],
						nextPageToken: "",
					});
				return new Promise((resolve) => {
					finishSecond = resolve;
				});
			},
		);
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const wrapper = ({ children }: { children: React.ReactNode }) =>
			React.createElement(QueryClientProvider, { client }, children);
		const { result } = renderHook(() => useFavorites(), { wrapper });
		await waitFor(() =>
			expect(result.current.items.map((item) => item.product?.id)).toEqual([
				"1",
			]),
		);
		let first: Promise<void> | undefined;
		await act(async () => {
			first = result.current.loadMore();
			void result.current.loadMore();
			await Promise.resolve();
		});
		expect(favoriteRepo.fetchFavorites).toHaveBeenCalledTimes(2);
		await act(async () => {
			finishSecond?.({ items: [{ product: { id: "2" } }], nextPageToken: "" });
			await first;
		});
		await waitFor(() =>
			expect(result.current.items.map((item) => item.product?.id)).toEqual([
				"1",
				"2",
			]),
		);
		await act(async () => {
			await client.invalidateQueries({
				queryKey: ["buyer", "42", "buyer", "favorites"],
			});
		});
		expect(result.current.items.map((item) => item.product?.id)).toEqual([
			"1",
			"2",
		]);
	});

	it("ignores a second tap after re-render while the next page is in flight", async () => {
		let finishSecond:
			| ((value: {
					items: Array<{ id: string }>;
					nextPageToken: string;
			  }) => void)
			| undefined;
		historyRepo.fetchHistory.mockImplementation(
			({ pageToken }: { pageToken: string }) => {
				if (!pageToken)
					return Promise.resolve({
						items: [{ id: "1" }],
						nextPageToken: "next",
					});
				return new Promise((resolve) => {
					finishSecond = resolve;
				});
			},
		);
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const wrapper = ({ children }: { children: React.ReactNode }) =>
			React.createElement(QueryClientProvider, { client }, children);
		const { result } = renderHook(() => useHistory(), { wrapper });
		await waitFor(() => expect(result.current.hasMore).toBe(true));
		let first: Promise<void> | undefined;
		await act(async () => {
			first = result.current.loadMore();
			await Promise.resolve();
		});
		await waitFor(() =>
			expect(historyRepo.fetchHistory).toHaveBeenCalledTimes(2),
		);
		await act(async () => {
			void result.current.loadMore();
			await Promise.resolve();
		});
		expect(
			historyRepo.fetchHistory.mock.calls.filter(
				([arg]) => arg.pageToken === "next",
			),
		).toHaveLength(1);
		await act(async () => {
			finishSecond?.({ items: [{ id: "2" }], nextPageToken: "" });
			await first;
		});
		await waitFor(() =>
			expect(result.current.items.map((item) => item.id)).toEqual(["1", "2"]),
		);
		expect(result.current.hasMore).toBe(false);
	});

	it("unfollow also marks the store's subscription cache as not followed", async () => {
		followRepo.fetchFollowing.mockResolvedValue({
			items: [
				{ storeId: "7", store: { id: "7" } },
				{ storeId: "8", store: { id: "8" } },
				{ storeId: "9" },
			],
			nextPageToken: "",
		});
		followRepo.setSellerFollow.mockResolvedValue(undefined);
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const identity = { accountId: "42", activeRole: "buyer" } as const;
		client.setQueryData(queryKeys.buyer.subscription(identity, "7"), true);
		const wrapper = ({ children }: { children: React.ReactNode }) =>
			React.createElement(QueryClientProvider, { client }, children);
		const { result } = renderHook(() => useFollowing(), { wrapper });
		await waitFor(() =>
			expect(result.current.items.map((item) => item.storeId)).toEqual([
				"7",
				"8",
				"9",
			]),
		);
		await act(async () => {
			await result.current.unfollow("7");
		});
		expect(followRepo.setSellerFollow).toHaveBeenCalledWith("7", false);
		expect(
			client.getQueryData(queryKeys.buyer.subscription(identity, "7")),
		).toBe(false);
		await waitFor(() =>
			expect(result.current.items.map((item) => item.storeId)).toEqual([
				"8",
				"9",
			]),
		);
		await act(async () => {
			await result.current.unfollow("9");
		});
		await waitFor(() =>
			expect(result.current.items.map((item) => item.storeId)).toEqual(["8"]),
		);
	});
});
