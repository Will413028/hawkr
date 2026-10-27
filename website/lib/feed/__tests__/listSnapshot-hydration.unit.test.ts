import { describe, expect, it, vi } from "vitest";
import {
	type FeedListSnapshot,
	feedListSnapshotForInitialRender,
} from "@/lib/feed/listSnapshot";
import {
	hydrationClientSnapshot,
	hydrationServerSnapshot,
} from "@/lib/hooks/useHydrationSafeClientRender";

const snapshot: FeedListSnapshot = {
	key: "/feed",
	posts: [
		{
			id: "snapshot-post",
			sellerId: "42",
			storeCode: "2649605557",
			sellerName: "Snapshot seller",
			logoKey: "",
			content: "Snapshot content",
			imageKeys: [],
			tags: [],
			product: null,
			commentCount: 0,
			likeCount: 0,
			shareCount: 0,
			viewCount: 0,
			isLiked: false,
			createdAt: 0,
		},
	],
	page: 1,
	done: false,
	scrollY: 640,
	activeTab: "recommend",
	filterMode: "all",
	savedAt: 1,
};

describe("feed snapshot hydration boundary", () => {
	it("does not read browser state for SSR or the first hydration render", () => {
		const readSnapshot = vi.fn(() => snapshot);

		for (const phase of ["SSR", "first hydration render"]) {
			expect(
				feedListSnapshotForInitialRender(
					hydrationServerSnapshot(),
					readSnapshot,
				),
				phase,
			).toBeNull();
		}
		expect(readSnapshot).not.toHaveBeenCalled();
	});

	it("restores synchronously when mounted after hydration", () => {
		const readSnapshot = vi.fn(() => snapshot);

		expect(
			feedListSnapshotForInitialRender(hydrationClientSnapshot(), readSnapshot),
		).toBe(snapshot);
		expect(readSnapshot).toHaveBeenCalledOnce();
	});
});
