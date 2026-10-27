import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { delegatedAuditUrl } from "../features/delegated-operations/state/session";

/**
 * 左欄「操作紀錄」是設計書 §3871 明訂的入口：「讓操作者查得到自己剛留下的稽核」。
 *
 * 它曾經指向 `{admOrigin}/admin/merchants/{storeId}#delegated-operation-audit`，
 * 而 admin repo 全域沒有任何元素帶那個 id ⇒ 點下去只是回到商家頁。這個壞法**沒有
 * 任何錯誤訊號**：瀏覽器對不存在的 fragment 不報錯、HTTP 200、頁面正常渲染，操作者
 * 只會以為自己的稽核還沒寫進去。
 */

const ADM = "https://adm.example.com";

test("有 sessionId 時指向該工作階段的稽核主控台", () => {
	assert.equal(
		delegatedAuditUrl(ADM, "7494024509996597248"),
		`${ADM}/admin/delegated-operations/7494024509996597248`,
	);
});

test("summary 未到手時退到查詢頁，而不是組出 /undefined 或 /null", () => {
	for (const value of [null, undefined, ""]) {
		assert.equal(
			delegatedAuditUrl(ADM, value),
			`${ADM}/admin/delegated-operations`,
		);
	}
});

test("sessionId 一律編碼，不得直接串進路徑", () => {
	assert.equal(
		delegatedAuditUrl(ADM, "a/b?c#d"),
		`${ADM}/admin/delegated-operations/a%2Fb%3Fc%23d`,
	);
});

/**
 * 錨點式連結不會壞得很大聲，所以它由 source contract 守：任何人想改回
 * `#delegated-operation-audit`（或任何 fragment）都得先讓這條轉紅。
 */
test("AdmRail 用 auditUrl，不得再組任何 fragment 連結", () => {
	const source = readFileSync(
		"features/delegated-operations/components/AdmRail.tsx",
		"utf8",
	);
	assert.match(source, /href=\{auditUrl\}/u);
	assert.doesNotMatch(
		source,
		/delegated-operation-audit/u,
		"錨點在 admin repo 不存在，連回去就是一條死路",
	);
	assert.doesNotMatch(
		source,
		/href=\{`\$\{returnUrl\}#/u,
		"returnUrl 是商家詳情頁；掛 fragment 到它上面等於把人丟回商家頁",
	);
});
