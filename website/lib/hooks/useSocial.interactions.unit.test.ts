/** @vitest-environment jsdom */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, renderHook, waitFor } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const posts = vi.hoisted(() => ({
	setPostLiked: vi.fn(),
	fetchPostViewerState: vi.fn(),
}));

vi.mock("@/lib/auth/context", () => ({ useAuth: () => ({ identity: null }) }));
vi.mock("@/lib/posts/repo", () => posts);
vi.mock("./useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: null }),
}));

import { queryKeys } from "@/lib/query/keys";
import { usePostInteractions } from "./useSocial";

let client: QueryClient;
function wrapper({ children }: { children: React.ReactNode }) {
	return React.createElement(QueryClientProvider, { client }, children);
}

beforeEach(() => {
	client = new QueryClient({
		defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } },
	});
	posts.setPostLiked.mockReset();
});
afterEach(cleanup);

describe("post like toggle", () => {
	it("shows the like at once and keeps the server's answer", async () => {
		posts.setPostLiked.mockResolvedValue({ liked: true });
		const { result } = renderHook(
			() => usePostInteractions("p", { liked: false, likeCount: 4 }),
			{ wrapper },
		);
		await act(async () => {
			await result.current.toggleLike();
		});
		await waitFor(() => expect(result.current.liked).toBe(true));
		expect(result.current.likeCount).toBe(5);
	});

	it("on failure undoes only its own change and keeps a count another update wrote meanwhile", async () => {
		let reject!: (error: unknown) => void;
		posts.setPostLiked.mockReturnValue(
			new Promise((_resolve, rej) => {
				reject = rej;
			}),
		);
		const { result } = renderHook(
			() => usePostInteractions("p", { liked: false, likeCount: 4 }),
			{ wrapper },
		);
		let toggled!: Promise<boolean | undefined>;
		act(() => {
			toggled = result.current.toggleLike();
		});
		await waitFor(() => expect(result.current.likeCount).toBe(5));
		act(() => {
			client.setQueryData(
				queryKeys.feed.postInteractions(
					{ accountId: null, activeRole: null },
					"p",
				),
				{
					liked: true,
					likeCount: 12,
				},
			);
		});
		await act(async () => {
			reject(new Error("boom"));
			await toggled;
		});
		await waitFor(() => expect(result.current.liked).toBe(false));
		expect(result.current.likeCount).toBe(11);
	});
});
