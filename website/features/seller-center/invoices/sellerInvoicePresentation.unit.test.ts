import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * 賣家面必須**複用**買家面已有的兩個判準，不得直出後端原始值。
 *
 * 兩個缺陷同一個形狀（2026-09-05 R5 lane A3 dev 實測），也是
 * `.claude/rules/02-judgment.md` §2.1 的形狀：改了前提沒有回灌到引用它的每一處。
 *
 *   ① backend#3084 只修了買家面。沒有載具時後端**刻意**把買受人統編放進
 *      `carrierSummary`（B2B 列表要看得到開給哪個統編），判別欄是 `carrierType`。
 *      賣家詳情頁只看 `carrierSummary` ⇒ document 7501682805689024512（carrier 欄
 *      全 NULL）畫面出現「載具 10000004」，而 10000004 是上一行的買受人統編。
 *   ② `fileUnavailableReason` 是代碼（NOT_APPLICABLE／CARRIER_NOT_WON／EXPIRED）。
 *      買家面走 `invoiceFileUnavailableHint()` 翻譯，賣家兩處直出
 *      ⇒ S07 訂單頁實際渲染出裸字串 `NOT_APPLICABLE`。
 *
 * 🔴 **為什麼是原始碼契約而不是純函式測試**：`carrierRowKindOf` 與
 * `invoiceFileUnavailableHint` 本身已經有各自的單元測試（`carrierDisplay.unit.test.ts`、
 * `orderInvoiceState.unit.test.ts`）——再寫一份對它們的斷言只是重複，抓不到本次的缺陷。
 * 本次的缺陷是「賣家面沒有呼叫它們」，所以判準要錨在呼叫端。
 *
 * 範圍誠實標註：本 repo 的 seller-center 沒有 component render harness ⇒ 這裡驗的是
 * 原始碼有沒有走那兩支 helper，不是 JSX 真的渲染出什麼。渲染那一半由 dev 截圖負責。
 */
const ROOT = join(import.meta.dirname, "..", "..", "..");
const read = (p: string) => readFileSync(join(ROOT, p), "utf8");

const DETAIL = "features/seller-center/invoices/SellerInvoiceDetailPage.tsx";
const CARD = "features/seller-center/invoices/SellerOrderInvoiceCard.tsx";

describe("賣家面複用共用判準（backend#3084 / fileUnavailableReason）", () => {
	it("載具列必須用 carrierRowKindOf 判別，不得只看 carrierSummary", () => {
		const src = read(DETAIL);
		// 對照組：先證明讀到的是對的檔案。
		expect(src).toContain("SellerInvoiceDetailPage");
		expect(src).toContain("carrierRowKindOf(summary.carrierSummary");
	});

	it("fileUnavailableReason 兩處都要翻譯，不得直出代碼", () => {
		for (const p of [DETAIL, CARD]) {
			const src = read(p);
			expect(src).toContain("invoiceFileUnavailableHint");
			// 直出的形態：把 reason 本身放進 JSX 的 {} 裡。
			expect(src).not.toMatch(
				/\{\s*detail\.summary\.fileUnavailableReason\s*\}/,
			);
			expect(src).not.toMatch(/\{\s*reason\s*\}/);
		}
	});
});
