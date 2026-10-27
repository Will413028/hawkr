/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const followRepo = vi.hoisted(() => ({
	batchCheckFollowedStoreIds: vi.fn(),
}));
vi.mock("@/lib/follow/repo", () => followRepo);
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({ hydrated: true, isLoggedIn: true, activeRole: "buyer" }),
}));
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "42", activeRole: "buyer" }),
}));

import { useFollowedSellers } from "./useFollowedSellers";

afterEach(() => {
	cleanup();
	followRepo.batchCheckFollowedStoreIds.mockReset();
});

describe("followed sellers in the feed", () => {
	it("keeps shown follows and checks only the new page's sellers", async () => {
		let finishSecond: ((ids: ReadonlySet<string>) => void) | undefined;
		followRepo.batchCheckFollowedStoreIds.mockImplementation(
			(ids: string[]) => {
				if (followRepo.batchCheckFollowedStoreIds.mock.calls.length === 1)
					return Promise.resolve(new Set(ids.filter((id) => id === "1")));
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
		const { result, rerender } = renderHook(
			({ sellerIds }: { sellerIds: string[] }) =>
				useFollowedSellers(true, sellerIds),
			{ wrapper, initialProps: { sellerIds: ["1", "2"] } },
		);
		await waitFor(() => expect(result.current.ready).toBe(true));
		expect([...result.current.ids]).toEqual(["1"]);

		rerender({ sellerIds: ["1", "2", "1", "3"] });
		expect([...result.current.ids]).toEqual(["1"]);
		await waitFor(() =>
			expect(followRepo.batchCheckFollowedStoreIds).toHaveBeenCalledTimes(2),
		);
		expect(followRepo.batchCheckFollowedStoreIds.mock.calls).toEqual([
			[["1", "2"]],
			[["3"]],
		]);
		expect([...result.current.ids]).toEqual(["1"]);

		finishSecond?.(new Set(["3"]));
		await waitFor(() =>
			expect([...result.current.ids].sort()).toEqual(["1", "3"]),
		);
		expect(followRepo.batchCheckFollowedStoreIds).toHaveBeenCalledTimes(2);
	});

	it("returns the same followed set when a re-render passes equal ids in a new array", async () => {
		followRepo.batchCheckFollowedStoreIds.mockImplementation((ids: string[]) =>
			Promise.resolve(new Set(ids.filter((id) => id === "1"))),
		);
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const wrapper = ({ children }: { children: React.ReactNode }) =>
			React.createElement(QueryClientProvider, { client }, children);
		const posts = [{ sellerId: "1" }, { sellerId: "2" }];
		const { result, rerender } = renderHook(
			({ items }: { items: Array<{ sellerId: string }> }) =>
				useFollowedSellers(
					true,
					items.map((post) => post.sellerId),
				),
			{ wrapper, initialProps: { items: posts } },
		);
		await waitFor(() => expect(result.current.ready).toBe(true));
		const first = result.current.ids;

		rerender({ items: posts });
		rerender({ items: [...posts] });
		expect(result.current.ids).toBe(first);
		expect(followRepo.batchCheckFollowedStoreIds).toHaveBeenCalledTimes(1);
	});
});
