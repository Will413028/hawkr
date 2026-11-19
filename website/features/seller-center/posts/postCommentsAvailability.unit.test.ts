import { describe, expect, it } from "vitest";
import { PostPublishStatus } from "@/lib/codemeta/enums";
import { postCommentsNotice } from "./postCommentsAvailability";

describe("postCommentsNotice", () => {
	it("a published post of a public store needs no notice", () => {
		expect(
			postCommentsNotice({ publishStatus: PostPublishStatus.PUBLISHED }, true),
		).toBeNull();
	});

	it("a store that cannot go public: buyers cannot see the comments", () => {
		expect(
			postCommentsNotice({ publishStatus: PostPublishStatus.PUBLISHED }, false),
		).toBe("store-not-public");
	});

	it("an owner-unpublished post says so", () => {
		expect(
			postCommentsNotice(
				{ publishStatus: PostPublishStatus.UNPUBLISHED },
				true,
			),
		).toBe("unpublished");
	});

	it("a draft was never published and has no comments to explain", () => {
		expect(
			postCommentsNotice({ publishStatus: PostPublishStatus.DRAFT }, true),
		).toBeNull();
	});

	it("a platform takedown (measure id) wins over every other reason", () => {
		for (const publishStatus of [
			PostPublishStatus.DRAFT,
			PostPublishStatus.UNPUBLISHED,
			PostPublishStatus.PUBLISHED,
		]) {
			for (const storePublic of [true, false, undefined]) {
				expect(
					postCommentsNotice(
						{ publishStatus, takedownMeasureId: "m-1" },
						storePublic,
					),
				).toBe("platform-takedown");
			}
		}
	});

	it("a legacy hiddenAt counts as a platform takedown too", () => {
		expect(
			postCommentsNotice(
				{ publishStatus: PostPublishStatus.PUBLISHED, hiddenAt: 1760000000 },
				true,
			),
		).toBe("platform-takedown");
	});

	it("an unknown store or a missing publish status does not conclude", () => {
		expect(postCommentsNotice({}, undefined)).toBeNull();
	});
});
