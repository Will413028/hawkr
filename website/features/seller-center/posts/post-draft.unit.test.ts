import { expect, test } from "vitest";

import { PostPublishStatus } from "@/lib/codemeta/enums";

import { isPostDraft, isPostUnpublished } from "./PostDraftBadge";

// 草稿在列表上原本沒有任何標記，而平台下架的有 pill ⇒ 一則從未發布的草稿看起來
// 就是一則普通的已發布貼文。代行者靠這個列表判斷店家的經營狀態。
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

// 這一條是本檔的重點：缺值代表「這條資料路徑的上游沒給」，不是「已發布」。
// （賣家自己那條 /app/sellers/me/posts 現在也帶 publishStatus 了，但述詞的防線
// 不因此撤除 —— 它守的是「任何一條路徑漏帶」，不是某一條特定路徑。）
// 推定成已發布 → 不畫 pill，看起來與真的已發布無法分辨；推定成草稿 → 把整個賣家
// 列表都畫上草稿。兩種推定都是憑空宣稱，正解是不畫。
test("缺值不得推定成任何狀態", () => {
	expectEqual(isPostDraft({}), false);
	expectEqual(isPostDraft({ publishStatus: undefined }), false);
	expectEqual(isPostDraft({ publishStatus: "" }), false);
});

// 碼值必須來自 CodeMeta 生成檔，不得在前端硬編。這一條會在有人把 "4401" 寫死、
// 而 CodeMeta 那側改了碼段時轉紅。
test("草稿碼取自 CodeMeta 生成檔", () => {
	expectEqual(PostPublishStatus.DRAFT, "4401");
	expectEqual(isPostDraft({ publishStatus: "4401" }), true);
});

// 4403「已下架」是店家自己撤下的，落在「平台下架（hiddenAt）」與「草稿」兩個判斷
// 之間 ⇒ 原本列表上零 badge，與已發布貼文完全相同。它必須是**獨立述詞**：放寬
// isPostDraft 去涵蓋它，會把已下架畫成草稿，比原本的缺口更糟。
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
