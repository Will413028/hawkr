import { describe, expect, it } from "vitest";
import type { ApiEnvelope } from "@/lib/api/client";
import type { RepoFetcher } from "@/lib/api/delegated";
import {
	createDelegatedPostRepo,
	mapDelegatedPostDetail,
} from "@/lib/seller-data/delegated-repos";

function recordingFetcher(data: unknown) {
	const seen: Array<{ path: string; method: string }> = [];
	const fetcher = (async <T>(path: string, opts: { method?: string } = {}) => {
		seen.push({ path, method: opts.method ?? "GET" });
		return { code: 0, data: data as T } satisfies ApiEnvelope<T>;
	}) as RepoFetcher;
	return { seen, fetcher };
}

describe("seller post detail read", () => {
	it("the delegated repo reads its own detail route and maps the DTO", async () => {
		const { seen, fetcher } = recordingFetcher({
			postId: "88",
			version: "3",
			content: "代行草稿",
			imageKeys: ["k"],
			tags: ["t"],
			publishStatus: "4401",
			createdAt: "2026-10-01T00:00:00Z",
			products: [{ productId: "5", name: "品", imageKey: "pk", price: "10" }],
		});
		const detail = await createDelegatedPostRepo(
			fetcher,
			"9",
		).fetchSellerPostDetail("88");

		expect(seen[0]?.path).toBe("/posts/9/88");
		expect(detail).toMatchObject({
			id: "88",
			sellerId: "9",
			content: "代行草稿",
			imageKeys: ["k"],
			publishStatus: "4401",
			createdAt: 1790812800,
			products: [{ id: "5", name: "品", imageKey: "pk", price: "10" }],
		});
	});

	it("carries the platform-hidden state of a delegated post", () => {
		const hidden = mapDelegatedPostDetail(
			{
				postId: "89",
				publishStatus: "4402",
				hiddenAt: "2026-10-01T08:00:00Z",
				hiddenReason: "違規",
			},
			"9",
		);
		expect(hidden.hiddenAt).toBe(Date.parse("2026-10-01T08:00:00Z") / 1000);
		expect(hidden.hiddenReason).toBe("違規");
		const live = mapDelegatedPostDetail({ postId: "90", hiddenAt: null }, "9");
		expect(live.hiddenAt).toBeUndefined();
		expect(live.hiddenReason).toBeUndefined();
	});

	it("maps a sparse delegated DTO to safe defaults", () => {
		expect(mapDelegatedPostDetail({}, "9")).toMatchObject({
			id: "",
			sellerId: "9",
			imageKeys: [],
			tags: [],
			products: [],
			createdAt: 0,
		});
	});
});

describe("delegated post count", () => {
	it("reads the server's summary, not the length of a list page", async () => {
		const { seen, fetcher } = recordingFetcher({ total: 17 });
		const count = await createDelegatedPostRepo(fetcher, "9").fetchPostCount();
		expect(count).toBe(17);
		expect(seen).toEqual([{ path: "/posts/9/summary", method: "GET" }]);
	});
});
