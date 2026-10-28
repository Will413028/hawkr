import assert from "node:assert/strict";
import test from "node:test";
import {
	discoverRedirectOnlyPages,
	isRedirectOnlyPageSource,
} from "./e2e/helpers/app-route-inventory";

/**
 * `discoverRedirectOnlyPages` 決定一個靜態頁屬於哪個 HTTP 回應類別
 * （site-links.spec.ts）。認錯的代價不是這支測試紅，而是**別支**測試紅：頁面
 * 會掉回「公開靜態頁」類、被斷言成 200，而它其實回 307。
 *
 * 2026-08-27 就是這樣擋掉一次 main 部署（run 33093420954）：偵測器原本要求
 * 函式主體「只有一行」，某個頁面在 redirect 前面加了一行
 * `if (!HAWKR_AGENT_ENABLED) notFound();` —— 行為一個字都沒變，類別卻換了。
 */
test("守衛行不會讓一個 redirect-only 頁換類別", () => {
	const guarded = `
import { notFound } from "next/navigation";

export default async function Page() {
	if (!ENABLED) notFound();
	redirect({
		href: "/target",
		locale: "zh-TW",
	});
}
`;
	assert.equal(isRedirectOnlyPageSource(guarded), true);

	const unguarded = `
export default function Page() {
	redirect("/target");
}
`;
	assert.equal(isRedirectOnlyPageSource(unguarded), true);

	const renders = `
export default function Page() {
	if (!ENABLED) notFound();
	redirect("/target");
	return <div />;
}
`;
	assert.equal(isRedirectOnlyPageSource(renders), false);

	const routes = discoverRedirectOnlyPages();
	assert.ok(routes.includes("/seller-center"));
});

test("會渲染的頁面不算 redirect-only", () => {
	const routes = discoverRedirectOnlyPages();
	// /about 是普通的 RSC 頁；它若被誤認，site-links 會期待它 307 而它回 200。
	assert.ok(!routes.includes("/about"));
	assert.ok(!routes.includes("/"));
	// 放寬只該多認「守衛 + redirect」這一種形狀，不該把清單放大到失控。
	assert.ok(
		routes.length <= 5,
		`redirect-only 頁暴增到 ${routes.length} 條，偵測器多半放太寬：${JSON.stringify(routes)}`,
	);
});
