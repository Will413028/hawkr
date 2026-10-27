import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { PostPublishStatusBadge } from "../features/seller-center/posts/PostDraftBadge";
import {
	POST_PUBLISH_STATUS_LABEL,
	PostPublishStatus,
} from "../lib/codemeta/enums";

/**
 * 賣家自行下架的貼文（4403）原本落在「平台下架」與「未發布」兩個判斷之間，於是列表上
 * **零 badge**，與已發布貼文像素級相同。代行者正是靠這個列表判斷店家的經營狀態，
 * 一則已被撤下的貼文會被當成還在線上回報給賣家。
 *
 * ⚠️ pill 的文字一律取 `POST_PUBLISH_STATUS_LABEL`，**不在這裡再寫死一次中文**：
 * 寫死的話，CodeMeta 改 Label 時元件與測試會一起「同步地錯」——測試全綠、畫面上
 * 的字卻與後端下發的說法不一致（4401 從「草稿」改成「未發布」時就是這樣漏掉的）。
 */

const render = (publishStatus?: string) =>
	renderToStaticMarkup(
		React.createElement(PostPublishStatusBadge, { publishStatus }),
	);

const DRAFT_LABEL = POST_PUBLISH_STATUS_LABEL[PostPublishStatus.DRAFT];
const UNPUBLISHED_LABEL =
	POST_PUBLISH_STATUS_LABEL[PostPublishStatus.UNPUBLISHED];

test("兩個 Label 取得到、且彼此不同（否則下面兩條會互相空過）", () => {
	assert.ok(DRAFT_LABEL, "4401 的 CodeMeta Label 取不到");
	assert.ok(UNPUBLISHED_LABEL, "4403 的 CodeMeta Label 取不到");
	assert.notEqual(
		DRAFT_LABEL,
		UNPUBLISHED_LABEL,
		"兩態的 Label 相同 ⇒ 底下的互斥斷言恆真恆假，等於沒驗",
	);
});

test("4403 畫出 CodeMeta 的「已下架」pill", () => {
	const html = render(PostPublishStatus.UNPUBLISHED);
	assert.ok(
		html.includes(UNPUBLISHED_LABEL),
		`4403 應畫出 ${UNPUBLISHED_LABEL} pill，實得：${html}`,
	);
	// 平台強制下架是另一顆（danger tone、文字「平台下架」），不得混用。
	assert.ok(!html.includes("平台下架"), "4403 不是平台強制下架");
	assert.ok(!html.includes(DRAFT_LABEL), `4403 不是${DRAFT_LABEL}`);
});

// 這一顆曾經是裸字串「草稿」，於是 CodeMeta 把 4401 的 Label 改成「未發布」時，
// 後端回「未發布」而畫面仍寫「草稿」。斷言綁 Label 而非字面值才擋得住下一次。
test("4401 畫出 CodeMeta 的 Label，不是寫死的中文", () => {
	const html = render(PostPublishStatus.DRAFT);
	assert.ok(
		html.includes(DRAFT_LABEL),
		`4401 應畫出 ${DRAFT_LABEL} pill，實得：${html}`,
	);
	assert.ok(!html.includes(UNPUBLISHED_LABEL), "4401 不是已下架");
	// 元件不得留著任何與 CodeMeta 脫鉤的舊字面值。
	const source = readFileSync(
		new URL(
			"../features/seller-center/posts/PostDraftBadge.tsx",
			import.meta.url,
		),
		"utf8",
	);
	const rendered = source
		.split("\n")
		.filter((line) => {
			const t = line.trim();
			return !t.startsWith("*") && !t.startsWith("/*") && !t.startsWith("//");
		})
		.join("\n");
	assert.ok(
		!rendered.includes("草稿"),
		"PostDraftBadge 的 JSX 仍有寫死的「草稿」⇒ CodeMeta 改 Label 時它不會跟上",
	);
});

/**
 * 上面驗的是 pill 本身；bug 的現場其實是**呼叫端**（列表只問了 `isPostDraft`）。
 * 沒有 DOM 測試環境可以掛整個列表，所以兩個現場的接線由原始碼契約守：都必須走
 * 單一分派點，任何一邊改回只問未發布就轉紅。
 */
test("列表與詳情彈窗都走單一分派點", () => {
	for (const path of [
		"../features/seller-center/posts/SellerPostsClient.tsx",
		"../features/seller-center/posts/SellerPostDetailModal.tsx",
	]) {
		const source = readFileSync(new URL(path, import.meta.url), "utf8");
		assert.ok(
			source.includes("<PostPublishStatusBadge"),
			`${path} 沒有渲染 PostPublishStatusBadge`,
		);
		assert.ok(
			!/\bisPostDraft\s*\(/.test(source),
			`${path} 仍自行判斷未發布 ⇒ 4403 會再次落在判斷之間`,
		);
	}
});

test("已發布與缺值一律不畫（含不留空外框）", () => {
	// 已發布是常態，標記它只會讓真正需要注意的兩態淹掉。
	assert.equal(render(PostPublishStatus.PUBLISHED), "");
	// 缺值＝「這條資料路徑的上游沒給」，不是「已發布」。現在兩條清單路徑都帶
	// publishStatus，但述詞的防線守的是「任何一條漏帶」，不隨路徑補齊而撤除。
	assert.equal(render(undefined), "");
	assert.equal(render(""), "");
});

/**
 * 代行者排了發布時間的草稿，店主在列表上原本只看到「未發布」，按下「發布」會直接取代
 * 排程。排程時間（`scheduledPublishAt`，ISO 8601 UTC）以台北時間顯示在未發布 pill 旁；
 * 其他狀態與沒有排程時不畫。
 */
const renderScheduled = (publishStatus: string, scheduledPublishAt?: string) =>
	renderToStaticMarkup(
		React.createElement(PostPublishStatusBadge, {
			publishStatus,
			scheduledPublishAt,
		}),
	);

test("未發布且有排程：pill 旁顯示「排程於 … 發布」（台北時間）", () => {
	const html = renderScheduled(PostPublishStatus.DRAFT, "2026-10-20T01:05:00Z");
	assert.ok(html.includes(DRAFT_LABEL), html);
	assert.ok(
		html.includes("排程於 2026/10/20 09:05 發布") ||
			html.includes("排程於 <!-- -->2026/10/20 09:05<!-- --> 發布"),
		`應顯示台北時間 09:05，實得：${html}`,
	);
});

test("未發布但沒有排程：不畫排程時間", () => {
	assert.ok(
		!renderScheduled(PostPublishStatus.DRAFT).includes("排程於"),
		"沒有 scheduledPublishAt 時不得畫排程",
	);
});

test("非未發布的狀態即使帶了排程欄位也不畫排程時間", () => {
	for (const status of [
		PostPublishStatus.PUBLISHED,
		PostPublishStatus.UNPUBLISHED,
	]) {
		assert.ok(
			!renderScheduled(status, "2026-10-20T01:05:00Z").includes("排程於"),
			`${status} 不是未發布，不該顯示排程`,
		);
	}
});
