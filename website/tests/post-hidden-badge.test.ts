import assert from "node:assert/strict";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { PostHiddenBadge } from "../features/seller-center/posts/PostHiddenBadge";

test("PostHiddenBadge renders 平台下架", () => {
	const html = renderToStaticMarkup(React.createElement(PostHiddenBadge));
	assert.ok(html.includes("平台下架"), "should contain 平台下架");
});
