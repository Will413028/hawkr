import { describe, expect, it } from "vitest";
import { carrierRowKindOf } from "./carrierDisplay";

describe("carrierRowKindOf", () => {
	it("有 carrierType ⇒ 這是載具", () => {
		expect(carrierRowKindOf("/DEF5678", "MOBILE_BARCODE")).toBe("carrier");
		expect(carrierRowKindOf("AB••••••••••••34", "CITIZEN_CERT")).toBe(
			"carrier",
		);
	});

	it("沒有 carrierType 但有值 ⇒ 那是買受人統編，不是載具", () => {
		expect(carrierRowKindOf("22223223", "")).toBe("recipientTaxId");
	});

	it("同一個 8 碼數字，帶不帶 carrierType 結果必須不同", () => {
		expect(carrierRowKindOf("22223223", "")).toBe("recipientTaxId");
		expect(carrierRowKindOf("22223223", "MOBILE_BARCODE")).toBe("carrier");
	});

	it("兩者皆空 ⇒ 沒有東西可顯示", () => {
		expect(carrierRowKindOf("", "")).toBe("none");
	});

	it("有 carrierType 卻沒有值 ⇒ 歸到 none，不標成載具", () => {
		expect(carrierRowKindOf("", "MOBILE_BARCODE")).toBe("none");
	});
});
