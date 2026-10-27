import { describe, expect, it } from "vitest";
import { carrierRowKindOf } from "./carrierDisplay";

describe("carrierRowKindOf", () => {
	it("有 carrierType ⇒ 這是載具", () => {
		expect(carrierRowKindOf("/DEF5678", "MOBILE_BARCODE")).toBe("carrier");
		expect(carrierRowKindOf("AB••••••••••••34", "CITIZEN_CERT")).toBe(
			"carrier",
		);
	});

	// 🔴 本檔的核心（backend#3084）：後端在沒有載具時**刻意**把買受人統編放進
	// carrierSummary，只看它分不出兩者。dev 實測 document 7501121593846792192：
	// carrier_type 與 carrier_id1_ciphertext 皆為 NULL，而詳情頁同時出現
	// 「買受人統編 22223223」與「載具 22223223」兩列、值一模一樣。
	it("沒有 carrierType 但有值 ⇒ 那是買受人統編，不是載具", () => {
		expect(carrierRowKindOf("22223223", "")).toBe("recipientTaxId");
	});

	// 鑑別力：8 碼數字既可能是統編、也可能是遮罩後的載具尾碼。
	// 少了這一條，把實作寫成「值是不是 8 碼數字」也會通過上一條。
	it("同一個 8 碼數字，帶不帶 carrierType 結果必須不同", () => {
		expect(carrierRowKindOf("22223223", "")).toBe("recipientTaxId");
		expect(carrierRowKindOf("22223223", "MOBILE_BARCODE")).toBe("carrier");
	});

	it("兩者皆空 ⇒ 沒有東西可顯示", () => {
		expect(carrierRowKindOf("", "")).toBe("none");
	});

	// 矛盾輸入：後端保證 summary 與 type 同進同出（carrierPresentationOf），
	// 真的發生時寧可少顯示也不要標錯。
	it("有 carrierType 卻沒有值 ⇒ 歸到 none，不標成載具", () => {
		expect(carrierRowKindOf("", "MOBILE_BARCODE")).toBe("none");
	});
});
