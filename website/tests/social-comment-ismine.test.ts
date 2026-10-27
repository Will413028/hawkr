import assert from "node:assert/strict";
import test from "node:test";
import { mapSellerForumComment } from "../lib/seller/forum-repo";
import { mapServerComment } from "../lib/social/repo";
import { normLikeChanged, normSnapshot } from "../lib/sse/useCommentStream";

// 留言的「是不是我的」由伺服器算成 isMine；wire 上沒有 userId，前端不比對任何 id，
// 所以 buyer N 與 store N 同號時也不會被混淆。

test("mapServerComment：isMine 只認 JSON true，且輸出不帶 userId", () => {
	const mine = mapServerComment({ id: "1", isMine: true });
	const theirs = mapServerComment({ id: "2", isMine: false });
	const missing = mapServerComment({ id: "3" });
	const stringy = mapServerComment({ id: "4", isMine: "true" });
	assert.equal(mine.isMine, true);
	assert.equal(theirs.isMine, false);
	assert.equal(missing.isMine, false);
	assert.equal(stringy.isMine, false);
	assert.equal("userId" in mine, false);
});

test("mapServerComment：同號的留言（buyer 7 與 store 7）以 isMine 區分", () => {
	const buyerSeven = mapServerComment({ id: "10", role: 1, isMine: true });
	const storeSeven = mapServerComment({ id: "11", role: 2, isMine: false });
	assert.equal(buyerSeven.isMine, true);
	assert.equal(storeSeven.isMine, false);
});

test("normSnapshot：SSE 留言快照讀 isMine，不再讀 userId", () => {
	const mine = normSnapshot({ commentId: "1", isMine: true, userId: "7" });
	const theirs = normSnapshot({ commentId: "2", isMine: false, userId: "7" });
	assert.equal(mine.isMine, true);
	assert.equal(theirs.isMine, false);
	assert.equal("userId" in mine, false);
});

test("mapSellerForumComment：討論區留言讀 isMine", () => {
	assert.equal(mapSellerForumComment({ id: "1", isMine: true }).isMine, true);
	assert.equal(mapSellerForumComment({ id: "2", isMine: false }).isMine, false);
	assert.equal(mapSellerForumComment({ id: "3" }).isMine, false);
});

test("normLikeChanged：按讚事件讀 likedByViewer（按讚者是不是觀看者），不讀 isMine、userId", () => {
	const mine = normLikeChanged(
		{ commentId: "1", likedByViewer: true, isMine: false, userId: "7" },
		"e1",
	);
	const theirs = normLikeChanged(
		{ commentId: "1", likedByViewer: false, isMine: true },
		"e2",
	);
	assert.equal(mine.likedByViewer, true);
	assert.equal(theirs.likedByViewer, false);
	assert.equal("isMine" in mine, false);
	assert.equal("userId" in mine, false);
});
