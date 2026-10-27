import { describe, expect, it } from "vitest";
import { certLabel } from "@/lib/products/certifications";
import {
	addCertification,
	availableCertOptions,
	CERTIFICATION_MAX,
	certChipText,
	certificationsSummary,
	removeCertification,
	sameCertifications,
} from "./certifications";
import type { DelegatedProductCertification } from "./repo";

/**
 * 認證標章在代行這一面上的三個「不會轉紅」的壞法：
 * ① 把碼值當中文名印出去（畫面顯示 `3701`）
 * ② 把證書編號在整份替換時掉掉（賣家的憑據靜默消失、回 `code: 0`）
 * ③ 把「沒改」判成「有改」或反過來（前者送出一筆空變更，後者是按鈕按不動）
 */

const cert = (name: string, certNo: string): DelegatedProductCertification => ({
	name,
	certNo,
});

describe("認證標章的顯示", () => {
	it("碼值換成 catalog 的完整標籤（mockup chip 上的字）", () => {
		expect(certLabel("3701")).toBe("有機農產品");
		expect(certLabel("3702")).toBe("產銷履歷 TAP");
	});

	it("認不得的碼值退回碼值本身，不得變成空白", () => {
		// 空白會讓操作者看到一顆沒有字的 chip，而它背後是一個真實的宣稱。
		expect(certLabel("9999")).toBe("9999");
		expect(certLabel("")).toBe("");
	});

	it("chip 連證書編號一起顯示；沒有編號時只顯示名稱", () => {
		expect(certChipText(cert("3701", "1-1-0001"))).toBe("有機農產品・1-1-0001");
		expect(certChipText(cert("3703", ""))).toBe("CAS 優良農產品");
	});

	it("三欄表摘要以頓號串接；空集合回空字串（讓抽屜自己畫成破折號）", () => {
		expect(certificationsSummary([])).toBe("");
		expect(
			certificationsSummary([cert("3701", "A1"), cert("3702", "B2")]),
		).toBe("有機農產品・A1、產銷履歷 TAP・B2");
	});

	it("摘要裡不會出現原始碼值", () => {
		const summary = certificationsSummary([cert("3701", "A1")]);
		expect(summary).not.toContain("3701");
	});
});

describe("認證標章的等值判定", () => {
	it("內容相同即等值（不同的物件實例不算改動）", () => {
		expect(sameCertifications([cert("3701", "A1")], [cert("3701", "A1")])).toBe(
			true,
		);
	});

	it("只有證書編號不同也算改動 —— 否則整份替換會把編號吃掉", () => {
		expect(sameCertifications([cert("3701", "A1")], [cert("3701", "A2")])).toBe(
			false,
		);
	});

	it("筆數不同、順序不同都算改動", () => {
		expect(sameCertifications([cert("3701", "A1")], [])).toBe(false);
		expect(
			sameCertifications(
				[cert("3701", "A1"), cert("3702", "B2")],
				[cert("3702", "B2"), cert("3701", "A1")],
			),
		).toBe(false);
	});
});

describe("新增與移除", () => {
	it("已掛上的類型不再出現在下拉裡", () => {
		const options = availableCertOptions([cert("3701", "A1")]);
		expect(options.map((o) => o.value)).not.toContain("3701");
		expect(options.map((o) => o.value)).toContain("3702");
	});

	it("沒選類型／沒填編號都拒絕，且回一句可行動的話", () => {
		expect(addCertification([], "", "A1")).toEqual({
			ok: false,
			error: "請先選擇認證類型",
		});
		expect(addCertification([], "3701", "   ")).toEqual({
			ok: false,
			error: "請填寫證書編號",
		});
	});

	it("同一個類型不得掛兩次", () => {
		const result = addCertification([cert("3701", "A1")], "3701", "A2");
		expect(result.ok).toBe(false);
	});

	it("成功時回一份新陣列，原陣列不變（表單狀態不可就地改）", () => {
		const before = [cert("3701", "A1")];
		const result = addCertification(before, "3702", " B2 ");
		expect(result).toEqual({
			ok: true,
			next: [cert("3701", "A1"), cert("3702", "B2")],
		});
		expect(before).toHaveLength(1);
	});

	it(`超過 ${CERTIFICATION_MAX} 筆時拒絕（Gateway 會回一個只剩代碼的 9260）`, () => {
		const full = Array.from({ length: CERTIFICATION_MAX }, (_, i) =>
			cert(`900${i}`, `N${i}`),
		);
		const result = addCertification(full, "3701", "A1");
		expect(result.ok).toBe(false);
	});

	it("以索引移除：認不得的舊碼值也移除得掉，且只移除那一筆", () => {
		const list = [cert("9999", "X"), cert("9999", "Y"), cert("3701", "A1")];
		expect(removeCertification(list, 1)).toEqual([
			cert("9999", "X"),
			cert("3701", "A1"),
		]);
	});
});
