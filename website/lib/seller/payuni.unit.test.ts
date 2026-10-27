import { create } from "@bufbuild/protobuf";
import {
	KYCField,
	PayUniState,
	SellerPayUniSchema,
	StoreField,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	payuniFixFieldLabels,
	payuniFixPointsAtStore,
	payuniHasState,
	payuniStateAlert,
	payuniStateTitle,
} from "./payuni";

const ut = uiTextFor("zh-TW");

const payuni = (
	init: Parameters<typeof create<typeof SellerPayUniSchema>>[1],
) => create(SellerPayUniSchema, init);

describe("payuniHasState", () => {
	it("有值而且不是 APPROVED 才有狀態要顯示", () => {
		expect(payuniHasState(null)).toBe(false);
		expect(payuniHasState(undefined)).toBe(false);
		expect(payuniHasState(payuni({ state: PayUniState.APPROVED }))).toBe(false);
		expect(payuniHasState(payuni({ state: PayUniState.REVIEWING }))).toBe(true);
		expect(payuniHasState(payuni({ state: PayUniState.FIXABLE }))).toBe(true);
	});

	it("不認得的狀態（含 UNSPECIFIED）仍顯示，走「未知」", () => {
		expect(payuniHasState(payuni({ state: PayUniState.UNSPECIFIED }))).toBe(
			true,
		);
		expect(payuniHasState(payuni({ state: 99 as PayUniState }))).toBe(true);
	});
});

describe("payuniStateTitle", () => {
	it("標題取自共用文案目錄的名詞；每個值都有", () => {
		expect(payuniStateTitle(ut, PayUniState.REVIEWING)).toBe("PayUni 審核中");
		for (const state of [
			PayUniState.PENDING_INFO,
			PayUniState.FIXABLE,
			PayUniState.FIX_VIA_SUPPORT,
		]) {
			expect(payuniStateTitle(ut, state)).toBe("PayUni 需要補件");
		}
		expect(payuniStateTitle(ut, PayUniState.CONTACT_SUPPORT)).toBe(
			"PayUni 申請未通過",
		);
		expect(payuniStateTitle(ut, PayUniState.RETRY_LATER)).toBe(
			"PayUni 暫時無法處理",
		);
		expect(payuniStateTitle(ut, PayUniState.TERMINATED)).toBe(
			"PayUni 收款帳戶已終止",
		);
		expect(payuniStateTitle(ut, PayUniState.APPROVED)).toBe(
			"PayUni 收款已啟用",
		);
	});

	it("UNSPECIFIED 與不認得的值是「未知」", () => {
		expect(payuniStateTitle(ut, PayUniState.UNSPECIFIED)).toBe("未知");
		expect(payuniStateTitle(ut, 99 as PayUniState)).toBe("未知");
	});
});

describe("payuniStateAlert", () => {
	it("審核中不是警示，其他是警示", () => {
		expect(payuniStateAlert(PayUniState.REVIEWING)).toBe(false);
		expect(payuniStateAlert(PayUniState.FIXABLE)).toBe(true);
		expect(payuniStateAlert(PayUniState.UNSPECIFIED)).toBe(true);
	});
});

describe("payuniFixFieldLabels 與 payuniFixPointsAtStore", () => {
	it("申請欄位依表單控制項去重：銀行名稱與代碼只列一個；店家欄位接在後面", () => {
		const labels = payuniFixFieldLabels(
			ut,
			payuni({
				fixFields: [
					KYCField.KYC_FIELD_BANK_NAME,
					KYCField.KYC_FIELD_BANK_CODE,
					KYCField.KYC_FIELD_ENGLISH_NAME,
				],
				fixStoreFields: [StoreField.ADDRESS],
			}),
		);
		expect(labels.filter((l) => l.includes("銀行"))).toHaveLength(1);
		expect(labels.length).toBe(3);
	});

	it("沒有清單是空的；店家入口只看認得的店家欄位", () => {
		expect(payuniFixFieldLabels(ut, payuni({}))).toEqual([]);
		expect(payuniFixFieldLabels(ut, null)).toEqual([]);
		expect(payuniFixPointsAtStore(payuni({}))).toBe(false);
		expect(payuniFixPointsAtStore(null)).toBe(false);
		expect(
			payuniFixPointsAtStore(
				payuni({ fixStoreFields: [StoreField.UNSPECIFIED] }),
			),
		).toBe(false);
		expect(
			payuniFixPointsAtStore(payuni({ fixStoreFields: [StoreField.ADDRESS] })),
		).toBe(true);
	});
});
