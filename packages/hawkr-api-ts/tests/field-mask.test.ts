import { describe, expect, it } from "vitest";
import { ProfileSchema } from "../src/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { KYCApplicationSchema } from "../src/gen/hawkr/seller/v1alpha1/seller_pb";
import { fieldMask } from "../src/proto/field-mask";

describe("fieldMask", () => {
	it("頂層欄位用 proto 的 snake_case 名稱", () => {
		expect(
			fieldMask(ProfileSchema, ["display_name", "birth_date"]).paths,
		).toEqual(["display_name", "birth_date"]);
	});

	it("巢狀訊息欄位用點串接", () => {
		expect(
			fieldMask(KYCApplicationSchema, ["business.rep_id_card_no"]).paths,
		).toEqual(["business.rep_id_card_no"]);
	});

	it("空的路徑清單得到空 mask", () => {
		expect(fieldMask(ProfileSchema, []).paths).toEqual([]);
	});

	it("未知的頂層欄位丟 Error", () => {
		expect(() => fieldMask(ProfileSchema, ["nope"])).toThrow(/不存在/);
	});

	it("camelCase 的 localName 不是 proto 路徑", () => {
		expect(() => fieldMask(ProfileSchema, ["displayName"])).toThrow(/不存在/);
	});

	it("未知的巢狀欄位丟 Error", () => {
		expect(() => fieldMask(KYCApplicationSchema, ["business.nope"])).toThrow(
			Error,
		);
	});

	it("純量欄位後面不能再接子路徑", () => {
		expect(() => fieldMask(ProfileSchema, ["display_name.x"])).toThrow(
			/不是訊息欄位/,
		);
	});

	it("repeated 欄位後面不能再接子路徑", () => {
		expect(() =>
			fieldMask(KYCApplicationSchema, ["business.company_documents.x"]),
		).toThrow(/不是訊息欄位/);
	});

	it("只要有一個路徑不合法就整個失敗", () => {
		expect(() => fieldMask(ProfileSchema, ["display_name", "nope"])).toThrow(
			Error,
		);
	});

	it("oneof 的子訊息可以往下一層（個人賣家）", () => {
		expect(
			fieldMask(KYCApplicationSchema, ["individual.id_card_no"]).paths,
		).toEqual(["individual.id_card_no"]);
	});

	it("非 oneof 的子訊息後面不能再接子路徑", () => {
		for (const path of ["bank_book.asset_id", "birth_date.year"]) {
			expect(() => fieldMask(KYCApplicationSchema, [path])).toThrow(
				/不是 oneof 的子訊息/,
			);
		}
	});

	it("下到 oneof 分支之後，內層的子訊息欄位不能再往下", () => {
		expect(
			fieldMask(KYCApplicationSchema, ["business.company_address"]).paths,
		).toEqual(["business.company_address"]);
		expect(() =>
			fieldMask(KYCApplicationSchema, ["business.company_address.town_code"]),
		).toThrow(/不是 oneof 的子訊息/);
	});
});
