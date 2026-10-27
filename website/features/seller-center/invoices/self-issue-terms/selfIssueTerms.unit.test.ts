import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
	SELF_ISSUE_CONSENT_LABEL,
	SELF_ISSUE_DEADLINE_NOTICE,
	SELF_ISSUE_TERM_POINTS,
} from "./selfIssueTerms";

/**
 * 條款文本的 SSOT 是 `docs/legal/`，畫面只是它的投影。這支測試是兩者之間唯一的
 * 連結——沒有它，改了畫面文案而條款檔不動（或反過來）完全靜默。
 */
const LEGAL_DOC = readFileSync(
	new URL(
		"../../../../docs/legal/self-issue-attestation-terms.md",
		import.meta.url,
	),
	"utf8",
);

describe("自行開票責任條款", () => {
	it("五條要點與 docs/legal 逐條一致", () => {
		expect(SELF_ISSUE_TERM_POINTS).toHaveLength(5);
		for (const point of SELF_ISSUE_TERM_POINTS) {
			expect(LEGAL_DOC, `條款檔缺少要點標題：${point.title}`).toContain(
				point.title,
			);
			expect(LEGAL_DOC, `條款檔缺少要點說明：${point.description}`).toContain(
				point.description,
			);
		}
	});

	it("期限性質的告知逐字一致（不得改寫、刪除或摺疊）", () => {
		expect(LEGAL_DOC).toContain(SELF_ISSUE_DEADLINE_NOTICE);
	});

	it("勾選框文字表達的是「自行負責開立及交付」而不是「同意條款」", () => {
		// 勾選本身就是明示同意 ⇒ 文字必須說出承擔的是什麼，不能只寫「我同意」。
		expect(SELF_ISSUE_CONSENT_LABEL).toContain("自行負責開立及交付發票");
	});

	it("前端沒有硬編碼條款版本號", () => {
		// 版本的權威是後端的 SelfIssueTermsVersion，經端點 21 的 currentTermsVersion
		// 送來。前端寫死一份必然漂移，而漂移的症狀是按「同意」被 400 拒。
		const source = readFileSync(
			new URL("./selfIssueTerms.ts", import.meta.url),
			"utf8",
		);
		expect(source).not.toMatch(/["'`]\d{4}-\d{2}-\d{2}["'`]/);
	});
});
