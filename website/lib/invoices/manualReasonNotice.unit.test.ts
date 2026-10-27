import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { INVOICE_MANUAL_REASON_LABEL } from "@/lib/codemeta/generated";
import { manualReasonNotice } from "./manualReasonNotice";

/**
 * 賣家收到「電子發票開立失敗…請至賣家中心的『發票異常作業』**查看原因**並處理」，
 * 到了那裡必須真的看得到原因。
 *
 * 🔴 2026-09-05 第 6 輪稽核：後端四個入口都填了 manualReasonCode、前端型別接了、
 * CodeMeta 的 label 也生成了 —— 而畫面層零消費者。與 correctionRejectNotice
 * 是同一個形狀（後端說「請見下方原因」，而下方沒有原因）。
 */
describe("manualReasonNotice", () => {
	it("字軌不足要給行動指引，而且必須指向財政部（重試無效）", () => {
		const got = manualReasonNotice("SELLER_TRACK_INSUFFICIENT");
		expect(got.label).toBe("賣家字軌不足");
		expect(got.hint).toMatch(/財政部/);
		// 這一格的重點是「不要叫他按重試」——處置與其他失敗相反。
		expect(got.hint).toMatch(/不需要你按重試|自動繼續/);
	});

	it("認得的碼給 label、沒有專屬指引時 hint 為空（不亂編）", () => {
		const code = Object.keys(INVOICE_MANUAL_REASON_LABEL).find(
			(k) => k !== "SELLER_TRACK_INSUFFICIENT",
		);
		expect(code).toBeTruthy();
		const got = manualReasonNotice(code as string);
		expect(got.label).toBe(INVOICE_MANUAL_REASON_LABEL[code as string]);
		expect(got.hint).toBe("");
	});

	it("未知碼不落回代碼本身（否則把 SNAKE_CASE 丟給賣家）", () => {
		expect(manualReasonNotice("NO_SUCH_CODE")).toEqual({ label: "", hint: "" });
		expect(manualReasonNotice("")).toEqual({ label: "", hint: "" });
	});

	it("賣家 job 頁面真的有渲染它（零呼叫端＝未交付）", () => {
		const src = readFileSync(
			join(
				process.cwd(),
				"features/seller-center/invoices/jobs/SellerInvoiceJobPage.tsx",
			),
			"utf8",
		);
		expect(src).toMatch(/manualReasonNotice\(job\.manualReasonCode\)/);
		expect(src).toMatch(/reason\.label/);
		expect(src).toMatch(/reason\.hint/);
		// 🔴 已完成的作業不得顯示上一次的失敗理由。後端已在 MarkInvoiceDocumentIssued
		// 清掉，這裡是第二道防線——那條路只要漏一個清除者，畫面就會出現
		// 「已開立 ＋ 原因：賣家字軌不足」，而沒有任何東西會發現。
		expect(src).toMatch(/reason\.label && !done/);
	});
});
