import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

/**
 * S05 頁面對「審核進行中」與「已核定」兩種賣家都得講對話（DEC-20260905-015836 選 A）。
 *
 * 端點 29 的 allowed-from 放寬之後，這一頁第一次會被**還沒核定**的賣家看到。原本的
 * 副標與 Chip 都是無條件寫死「已核定」，對那批人逐字是假的。
 *
 * 🔴 **為什麼是原始碼契約而不是純函式測試**：這一頁沒有可抽出來的判準函式，兩句話
 * 直接寫在 JSX 裡，而本 repo 的 seller-center 沒有 component render harness
 * （同 `sellerInvoicePresentation.unit.test.ts` 的範圍誠實標註）。這裡驗的是「兩句
 * 話有沒有掛在 `approved` 上」，畫面實際長怎樣由 dev 截圖負責。
 */
const PAGE = join(
	import.meta.dirname,
	"..",
	"..",
	"..",
	"..",
	"features/seller-center/invoices/receipt-authorization/ReceiptAuthPage.tsx",
);
const src = readFileSync(PAGE, "utf8");

describe("ReceiptAuthPage 的核定／審核中文案", () => {
	it("對照組：讀到的是 S05 那一頁", () => {
		expect(src).toContain("export function ReceiptAuthPage");
	});

	it("用 approvedAt 分辨已核定與審核進行中", () => {
		// 端點 28 不回 taxReviewStatus，approvedAt 是唯一分得出來的欄位。
		expect(src).toContain("const approved = Boolean(authorization.approvedAt)");
	});

	it("副標不得無條件宣稱「已核定」", () => {
		// 舊寫法：subtitle="你已經平台核定免用統一發票，…"（寫死，對審核中的人是假的）。
		expect(src).not.toMatch(/subtitle="[^"]*核定/);
		expect(src).toContain("稅務身分審核中");
	});

	it("核定狀態 Chip 有第三態「審核中」，不是只有已核定與已失效", () => {
		expect(src).toContain('"審核中"');
		// 已核定那一態仍要在，不能為了修這件事把它拿掉。
		expect(src).toContain('"已核定免用統一發票"');
	});
});
