import { describe, expect, it } from "vitest";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import {
	POST_COMMENTS_UNAVAILABLE_TEXT,
	postCommentsAvailability,
} from "./postCommentsAvailability";

describe("postCommentsAvailability", () => {
	it("a published, visible post of a public store is available", () => {
		expect(
			postCommentsAvailability(
				{ publishStatus: PostPublishStatus.PUBLISHED },
				true,
			),
		).toBe("available");
	});

	it("a store that cannot go public wins over the post state", () => {
		expect(
			postCommentsAvailability(
				{ publishStatus: PostPublishStatus.DRAFT },
				false,
			),
		).toBe("store-not-public");
	});

	it("platform-hidden wins over a draft, an unpublished post and a non-public store", () => {
		for (const publishStatus of [
			PostPublishStatus.DRAFT,
			PostPublishStatus.UNPUBLISHED,
			PostPublishStatus.PUBLISHED,
		]) {
			for (const storePublic of [true, false, undefined]) {
				expect(
					postCommentsAvailability(
						{ publishStatus, hiddenAt: 1760000000 },
						storePublic,
					),
				).toBe("platform-hidden");
			}
		}
	});

	it("draft and self-unpublished posts are not published", () => {
		for (const publishStatus of [
			PostPublishStatus.DRAFT,
			PostPublishStatus.UNPUBLISHED,
		]) {
			expect(postCommentsAvailability({ publishStatus }, true)).toBe(
				"not-published",
			);
		}
	});

	it("a published post hidden by the platform is platform-hidden", () => {
		expect(
			postCommentsAvailability(
				{ publishStatus: PostPublishStatus.PUBLISHED, hiddenAt: 1760000000 },
				true,
			),
		).toBe("platform-hidden");
	});

	it("an unknown store or a missing publish status does not conclude", () => {
		expect(postCommentsAvailability({}, undefined)).toBe("available");
	});

	it("uses the approved copy", () => {
		expect(POST_COMMENTS_UNAVAILABLE_TEXT).toEqual({
			"store-not-public": "店家公開後，買家的留言會顯示在這裡。",
			"not-published": "動態發布後，買家的留言會顯示在這裡。",
			"platform-hidden": "這則動態已被平台下架，買家看不到留言。",
		});
	});
});
