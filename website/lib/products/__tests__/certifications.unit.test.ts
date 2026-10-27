import { describe, expect, it } from "vitest";
import { CERTIFICATION_TYPE_LABEL } from "@/lib/codemeta/generated";
import {
	CERTIFICATION_LABEL_BY_VALUE,
	certLabel,
	getProductCertCatalogItem,
	PRODUCT_CERT_CATALOG,
	toCertBadges,
} from "../certifications";

/**
 * 這一支守的是 website#507 的那一類失效：**碼值漏了對照就靜默顯示成數字**。
 *
 * 買家看到的「3701」不是錯誤、不會拋例外、不會有任何既有測試轉紅 —— 認證是法規
 * 宣稱，顯示成一串數字等於這個宣稱對買家完全無效，而唯一會發現的人是買家自己。
 * 所以每一條斷言都下在「**渲染時會拿到什麼**」這一層，不是「對照表裡有沒有這一筆」。
 */
describe("product certification catalog", () => {
	it("每個碼值都解析得出顯示名，不會退回碼值本身", () => {
		for (const item of PRODUCT_CERT_CATALOG) {
			const label = certLabel(item.code);
			expect(label).toBe(item.label);
			// 這一條才是重點：退回碼值 = 買家看到數字。
			expect(label).not.toBe(item.code);
		}
	});

	it("可線上查驗的類型才提供官方入口", () => {
		for (const item of PRODUCT_CERT_CATALOG) {
			if (item.verifiable) {
				expect(item.officialLookup?.href).toMatch(/^https:\/\//);
				expect(item.officialLookup?.label).toContain("官方查詢");
			} else {
				expect(item.officialLookup).toBeUndefined();
			}
		}
	});

	/**
	 * CodeMeta 是碼值的 SSOT（`docs/代碼管理（CodeMeta）/業務類型碼.md`）。這一條是
	 * 「後端新增第七個認證類型，前端忘了補」的唯一機械保證 —— 沒有它，那個新碼值會
	 * 在買家商品頁上顯示成數字，而三套 lint / test / test:unit 全綠。
	 */
	it("碼值集合與 CodeMeta 完全一致", () => {
		const catalogCodes = PRODUCT_CERT_CATALOG.map((item) => item.code).sort();
		const codeMetaCodes = Object.keys(CERTIFICATION_TYPE_LABEL).sort();
		expect(catalogCodes).toEqual(codeMetaCodes);
	});

	it("短標籤表由 catalog 導出，兩者不會各自漂移", () => {
		for (const item of PRODUCT_CERT_CATALOG) {
			expect(CERTIFICATION_LABEL_BY_VALUE[item.code]).toBe(item.shortLabel);
		}
		expect(Object.keys(CERTIFICATION_LABEL_BY_VALUE).sort()).toEqual(
			PRODUCT_CERT_CATALOG.map((item) => item.code).sort(),
		);
	});

	describe("認不得的值", () => {
		it("顯示名退回原值而不是空字串", () => {
			// 空白處會讓操作者以為「這裡本來就沒有東西」，進而把一個看不見的宣稱刪掉。
			expect(certLabel("9999")).toBe("9999");
			expect(certLabel("")).toBe("");
		});
	});

	it("共享 resolver 會去除匯入資料的前後空白", () => {
		expect(getProductCertCatalogItem(" 3702 ")?.label).toBe("產銷履歷 TAP");
		expect(certLabel(" 3702 ")).toBe("產銷履歷 TAP");
	});

	it("商品卡 badge 用短標籤並去重", () => {
		expect(
			toCertBadges([
				{ name: "3701", certNo: "A" },
				{ name: "3701", certNo: "B" },
				{ name: "3702" },
			]),
		).toEqual([
			{ code: "3701", label: "有機" },
			{ code: "3702", label: "產銷履歷" },
		]);
	});
});
