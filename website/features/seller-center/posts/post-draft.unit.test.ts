import { expect, test } from "vitest";

import { PostPublishStatus } from "@/lib/codemeta/enums";

import { isPostDraft, isPostUnpublished } from "./PostDraftBadge";

test("publishStatus 是草稿碼時才回 true", () => {
	expectEqual(isPostDraft({ publishStatus: PostPublishStatus.DRAFT }), true);
	expectEqual(
		isPostDraft({ publishStatus: PostPublishStatus.PUBLISHED }),
		false,
	);
	expectEqual(
		isPostDraft({ publishStatus: PostPublishStatus.UNPUBLISHED }),
		false,
	);
});

test("缺值不得推定成任何狀態", () => {
	expectEqual(isPostDraft({}), false);
	expectEqual(isPostDraft({ publishStatus: undefined }), false);
	expectEqual(isPostDraft({ publishStatus: "" }), false);
});

test("草稿碼取自 CodeMeta 生成檔", () => {
	expectEqual(PostPublishStatus.DRAFT, "4401");
	expectEqual(isPostDraft({ publishStatus: "4401" }), true);
});

test("已下架是獨立述詞，不與草稿混用", () => {
	expectEqual(
		isPostUnpublished({ publishStatus: PostPublishStatus.UNPUBLISHED }),
		true,
	);
	expectEqual(
		isPostUnpublished({ publishStatus: PostPublishStatus.DRAFT }),
		false,
	);
	expectEqual(
		isPostUnpublished({ publishStatus: PostPublishStatus.PUBLISHED }),
		false,
	);
	expectEqual(isPostUnpublished({}), false);
	expectEqual(isPostUnpublished({ publishStatus: "" }), false);
});

test("已下架碼取自 CodeMeta 生成檔", () => {
	expectEqual(PostPublishStatus.UNPUBLISHED, "4403");
	expectEqual(isPostUnpublished({ publishStatus: "4403" }), true);
});

function expectEqual(actual: unknown, want: unknown) {
	expect(actual).toBe(want);
}
