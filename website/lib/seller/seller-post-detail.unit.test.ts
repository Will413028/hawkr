import { describe, expect, it } from "vitest";
import { ApiClientError, type ApiEnvelope } from "@/lib/api/client";
import type { RepoFetcher } from "@/lib/api/delegated";
import {
	createDelegatedPostRepo,
	mapDelegatedPostDetail,
} from "@/lib/seller-data/delegated-repos";
import { createSellerPostRepo } from "./repo";

function recordingFetcher(data: unknown) {
	const seen: Array<{ path: string; method: string }> = [];
	const fetcher = (async <T>(path: string, opts: { method?: string } = {}) => {
		seen.push({ path, method: opts.method ?? "GET" });
		return { code: 0, data: data as T } satisfies ApiEnvelope<T>;
	}) as RepoFetcher;
	return { seen, fetcher };
}

describe("seller post detail read", () => {
	it("reads the owner route and keeps the owner-only fields", async () => {
		const { seen, fetcher } = recordingFetcher({
			id: "77",
			sellerId: "7",
			sellerName: "店",
			logoKey: " logo ",
			content: "草稿",
			imageKeys: null,
			tags: ["#a"],
			products: null,
			commentCount: 0,
			likeCount: 0,
			shareCount: 0,
			viewCount: 0,
			isLiked: false,
			createdAt: 1,
			publishStatus: "4401",
			hiddenAt: 1760000000,
			hiddenReason: "違規",
		});
		const detail =
			await createSellerPostRepo(fetcher).fetchSellerPostDetail("77");

		expect(seen).toEqual([{ path: "/app/sellers/posts/77", method: "GET" }]);
		expect(detail.logoKey).toBe("logo");
		expect(detail.imageKeys).toEqual([]);
		expect(detail.products).toEqual([]);
		expect(detail.publishStatus).toBe("4401");
		expect(detail.hiddenAt).toBe(1760000000);
		expect(detail.hiddenReason).toBe("違規");
	});

	it("encodes the id and treats an empty body as not found", async () => {
		const { seen, fetcher } = recordingFetcher(undefined);
		const error = await createSellerPostRepo(fetcher)
			.fetchSellerPostDetail("7/../8")
			.catch((e) => e);

		expect(seen[0]?.path).toBe("/app/sellers/posts/7%2F..%2F8");
		expect(error).toBeInstanceOf(ApiClientError);
		expect((error as ApiClientError).status).toBe(404);
	});

	it("propagates a transport failure", async () => {
		const fetcher = (async () => {
			throw new ApiClientError("HTTP 503", 503, 503);
		}) as RepoFetcher;
		await expect(
			createSellerPostRepo(fetcher).fetchSellerPostDetail("77"),
		).rejects.toMatchObject({ status: 503 });
	});

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
