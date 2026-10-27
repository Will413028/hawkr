import assert from "node:assert/strict";
import test from "node:test";
import { mapServerComment } from "../lib/social/repo";

// Comment 主鍵是 64-bit TSID，超過 JS Number.MAX_SAFE_INTEGER (2^53-1)。先前
// mapServerComment 以 Number() 把 id 轉成 JS number，full-width TSID 末位被
// 四捨五入；又與 SSE CommentSnapshot（已是字串）型別不一致，導致 REST 列表與
// SSE 推送的同一則留言 dedup 對不上、重複渲染。
//
// 本測試鎖住：REST 留言的所有 id 欄位一律保字串、且 TSID > 2^53 不丟精度。
// （後端翻字串後 String(string) 為 no-op；翻字串前 String(number) 與 SSE 同步
// 丟同一個值，故 dedup 仍一致。）
const TSID = "7234567890123456789"; // > 2^53，number 無法精確表示

test("mapServerComment：id 為 JSON string 時精確保留字串", () => {
	const c = mapServerComment({
		id: TSID,
		postId: TSID,
		parentId: TSID,
		replyToId: TSID,
		isMine: true,
		content: "hi",
	});
	assert.equal(c.id, TSID);
	assert.equal(c.postId, TSID);
	assert.equal(c.parentId, TSID);
	assert.equal(c.replyToId, TSID);
	assert.equal(c.isMine, true);
	assert.equal(typeof c.id, "string");
});

test("mapServerComment：id 為 JSON number 時轉字串（型別一致，不再是 number）", () => {
	const c = mapServerComment({ id: 123, postId: 456 });
	assert.equal(c.id, "123");
	assert.equal(c.postId, "456");
	assert.equal(typeof c.id, "string");
});

test("mapServerComment：parentId / replyToId 的 0 / '0' / null 一律視為無", () => {
	const top = mapServerComment({ id: TSID, parentId: 0, replyToId: "0" });
	assert.equal(top.parentId, null);
	assert.equal(top.replyToId, null);

	const missing = mapServerComment({ id: TSID });
	assert.equal(missing.parentId, null);
	assert.equal(missing.replyToId, null);
});

// 契約守門（backend-payuni#2235）：留言 wire 已全面 camelCase，snake_case 欄名
// 不再存在。這裡刻意只餵舊欄名 —— 只要有人補回 `c.post_id ?? c.postId` 之類的
// 雙讀 fallback，下面的斷言就會轉紅。fallback 的代價不是「多讀一個欄位」，
// 而是後端真的改名時前端靜默顯示空白，而不是壞給你看。
test("mapServerComment：只餵舊 snake_case 欄名 → 全部讀不到（禁止雙讀 fallback）", () => {
	const c = mapServerComment({
		id: TSID,
		post_id: TSID,
		parent_id: TSID,
		reply_to_id: TSID,
		reply_to_name: "阿明",
		member_id: TSID,
		member_name: "阿明",
		user_id: TSID,
		user_name: "阿明",
		like_count: 7,
		reply_count: 3,
		is_liked: true,
		created_at: 1785904375,
		content: "hi",
	});

	assert.equal(c.postId, "");
	assert.equal(c.parentId, null);
	assert.equal(c.replyToId, null);
	assert.equal(c.replyToName, "");
	assert.equal(c.isMine, false);
	assert.equal(c.userName, "匿名");
	assert.equal(c.likeCount, 0);
	assert.equal(c.replyCount, 0);
	assert.equal(c.isLiked, false);
	assert.equal(c.createdAt, 0);
});
