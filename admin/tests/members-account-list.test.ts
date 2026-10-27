import assert from "node:assert/strict";
import test from "node:test";
import { timestampFromMs } from "@bufbuild/protobuf/wkt";
import { formatCreateDate } from "../src/app/admin/members/account-list";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	pageCount,
	previousPage,
} from "../src/lib/keyset-pages";

// 會員頁以 ListAccounts 的不透明 page token 翻頁（AIP-158）：只能往下拿，所以「上一頁」靠記住走過的 token。

test("第一頁的 token 是空字串", () => {
	const tokens = firstPage();
	assert.equal(currentPage(tokens), 1);
	assert.equal(currentToken(tokens), "");
});

test("往下翻兩頁再退回：頁碼與 token 對得上走過的路", () => {
	const p2 = nextPage(firstPage(), "t2");
	const p3 = nextPage(p2, "t3");
	assert.equal(currentPage(p3), 3);
	assert.equal(currentToken(p3), "t3");

	const back = previousPage(p3);
	assert.equal(currentPage(back), 2);
	assert.equal(currentToken(back), "t2");
	assert.equal(currentToken(previousPage(back)), "");
});

test("最後一頁（next_page_token 是空字串）不能再往下，第一頁不能再往上", () => {
	const p2 = nextPage(firstPage(), "t2");
	assert.equal(nextPage(p2, ""), p2);
	const first = firstPage();
	assert.equal(previousPage(first), first);
});

test("總頁數由 total_size 算出", () => {
	assert.equal(pageCount(0, 10), 0);
	assert.equal(pageCount(10, 10), 1);
	assert.equal(pageCount(11, 10), 2);
});

test("建立時間以台北的日期顯示，沒有值時是 -", () => {
	// 2026-09-26T16:30:00Z 是台北的 9 月 27 日 00:30。
	assert.equal(
		formatCreateDate(timestampFromMs(Date.UTC(2026, 8, 26, 16, 30))),
		"2026-09-27",
	);
	assert.equal(formatCreateDate(undefined), "-");
});
