import assert from "node:assert/strict";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { PostHiddenBadge } from "../features/seller-center/posts/PostHiddenBadge";

// 「平台下架」而不是「已下架」：CodeMeta 的 4403 標籤就叫「已下架」，指的是店家
// 自己撤下的貼文。同一份列表裡兩種意義共用一個詞，代行者就分不出是誰下的架。
test("PostHiddenBadge renders 平台下架", () => {
	const html = renderToStaticMarkup(React.createElement(PostHiddenBadge));
	assert.ok(html.includes("平台下架"), "should contain 平台下架");
});
