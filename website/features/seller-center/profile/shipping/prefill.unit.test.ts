import { create } from "@bufbuild/protobuf";
import { PostalAddressSchema } from "@hawkr/api/gen/hawkr/common/v1alpha1/postal_address_pb";
import {
	GetMyStoreResponseSchema,
	StoreSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { TOWNS, townByName } from "@hawkr/twaddr/lookup";
import { describe, expect, it } from "vitest";
import {
	getDistrictOptions,
	getKnownDistrict,
	TAIWAN_CITY_OPTIONS,
	toLogisticsName,
} from "@/lib/seller/certification-options";
import {
	PAYUNI_SUBMIT_STATUS,
	SHIPPING_TYPE,
	type ShippingOnboarding,
} from "@/lib/seller/shipping";
import {
	cvsSenderInitial,
	homeFormInitial,
	senderPostalCodeOf,
} from "./prefill";

function storeOf(
	over: {
		contactName?: string;
		contactMobile?: string;
		townCode?: string;
		addressDetail?: string;
		contactPhone?: string;
		name?: string;
	} = {},
) {
	return create(GetMyStoreResponseSchema, {
		store: create(StoreSchema, {
			storeName: over.name ?? "某某商店",
			contactName: over.contactName ?? "",
			contactMobile: over.contactMobile ?? "",
			contactPhone: over.contactPhone ?? "",
			address:
				over.townCode !== undefined || over.addressDetail !== undefined
					? create(PostalAddressSchema, {
							townCode: over.townCode ?? "",
							addressDetail: over.addressDetail ?? "",
						})
					: undefined,
		}),
	});
}

const FULL_STORE = storeOf({
	contactName: "王小明",
	contactMobile: "0912345678",
	townCode: "63000050", // 臺北市中正區
	addressDetail: "重慶南路一段 122 號",
});

const HOME_ROW: ShippingOnboarding = {
	shippingType: SHIPPING_TYPE.HOME_DELIVERY,
	payuniSubmitStatus: PAYUNI_SUBMIT_STATUS.SUCCEEDED,
	senderName: "紀錄人",
	senderPhone: "0987654321",
	senderCity: "高雄市",
	senderDistrict: "左營區",
	senderAddressDetail: "博愛二路 1 號",
	senderPostalCode: "813",
};

describe("homeFormInitial", () => {
	it("uses the store when there is no onboarding row", () => {
		expect(homeFormInitial(undefined, FULL_STORE)).toEqual({
			senderName: "王小明",
			senderPhone: "0912345678",
			senderCity: "台北市",
			senderDistrict: "中正區",
			senderAddressDetail: "重慶南路一段 122 號",
		});
	});

	it("uses the whole row when a record exists, never mixing in store fields", () => {
		const form = homeFormInitial(HOME_ROW, FULL_STORE);
		expect(form).toEqual({
			senderName: "紀錄人",
			senderPhone: "0987654321",
			senderCity: "高雄市",
			senderDistrict: "左營區",
			senderAddressDetail: "博愛二路 1 號",
		});
	});

	it("writes a row's 臺 names in the logistics-table 台 spelling, so the dropdowns have no duplicate option", () => {
		const row: ShippingOnboarding = {
			...HOME_ROW,
			senderCity: "雲林縣",
			senderDistrict: "臺西鄉",
		};
		const form = homeFormInitial(row, FULL_STORE);
		expect(form.senderDistrict).toBe("台西鄉");
		expect(getKnownDistrict(form.senderCity, form.senderDistrict)).toBe(
			"台西鄉",
		);
		const tw = homeFormInitial(
			{ ...HOME_ROW, senderCity: "臺北市", senderDistrict: "中正區" },
			undefined,
		);
		expect(tw.senderCity).toBe("台北市");
		expect(TAIWAN_CITY_OPTIONS).toContain(tw.senderCity);
	});

	it("still uses the row when its status is failed (4304)", () => {
		const failed = {
			...HOME_ROW,
			payuniSubmitStatus: PAYUNI_SUBMIT_STATUS.FAILED,
		};
		expect(homeFormInitial(failed, FULL_STORE).senderName).toBe("紀錄人");
		expect(homeFormInitial(failed, FULL_STORE).senderCity).toBe("高雄市");
	});

	it("falls back to the store when the row has no sender values", () => {
		const empty: ShippingOnboarding = {
			shippingType: SHIPPING_TYPE.HOME_DELIVERY,
			payuniSubmitStatus: PAYUNI_SUBMIT_STATUS.NONE,
			senderName: " ",
		};
		expect(homeFormInitial(empty, FULL_STORE).senderName).toBe("王小明");
	});

	it("leaves store fields empty when the store lacks them (no landline, nickname or shop-name fallback)", () => {
		const sparse = storeOf({
			contactPhone: "02-12345678",
			name: "店名不該被用",
		});
		expect(homeFormInitial(undefined, sparse)).toEqual({
			senderName: "",
			senderPhone: "",
			senderCity: "",
			senderDistrict: "",
			senderAddressDetail: "",
		});
		expect(homeFormInitial(undefined, null)).toEqual({
			senderName: "",
			senderPhone: "",
			senderCity: "",
			senderDistrict: "",
			senderAddressDetail: "",
		});
	});

	it("maps the store town code to the logistics-table 台 spelling", () => {
		const cases = [
			["10009160", "雲林縣", "台西鄉"],
			["10013270", "屏東縣", "霧台鄉"],
			["10014010", "台東縣", "台東市"],
		];
		for (const [townCode, city, district] of cases) {
			const form = homeFormInitial(undefined, storeOf({ townCode }));
			expect([form.senderCity, form.senderDistrict]).toEqual([city, district]);
			expect(TAIWAN_CITY_OPTIONS).toContain(form.senderCity);
			expect(getKnownDistrict(form.senderCity, form.senderDistrict)).toBe(
				district,
			);
		}
	});

	it("keeps city and district empty for an unknown town code", () => {
		const bad = storeOf({ townCode: "99999999", addressDetail: "某路" });
		const form = homeFormInitial(undefined, bad);
		expect(form.senderCity).toBe("");
		expect(form.senderDistrict).toBe("");
	});
});

describe("cvsSenderInitial", () => {
	const ambient: ShippingOnboarding = {
		shippingType: SHIPPING_TYPE.CVS,
		senderName: "寄件人",
		senderPhone: "0911111111",
	};

	it("uses the row's sender when it has one", () => {
		expect(cvsSenderInitial(ambient, FULL_STORE)).toEqual({
			senderName: "寄件人",
			senderPhone: "0911111111",
		});
	});

	it("when the row has no sender, uses the store", () => {
		const emptyAmbient = { ...ambient, senderName: "", senderPhone: "" };
		expect(cvsSenderInitial(emptyAmbient, FULL_STORE)).toEqual({
			senderName: "王小明",
			senderPhone: "0912345678",
		});
	});

	it("uses a failed row's values rather than the store", () => {
		const failed = {
			...ambient,
			payuniSubmitStatus: PAYUNI_SUBMIT_STATUS.FAILED,
		};
		expect(cvsSenderInitial(failed, FULL_STORE).senderName).toBe("寄件人");
	});

	it("falls back to the store, leaving missing fields empty", () => {
		expect(cvsSenderInitial(undefined, FULL_STORE)).toEqual({
			senderName: "王小明",
			senderPhone: "0912345678",
		});
		expect(
			cvsSenderInitial(undefined, storeOf({ contactPhone: "02-1" })),
		).toEqual({ senderName: "", senderPhone: "" });
	});
});

describe("senderPostalCodeOf", () => {
	it("derives the code from the logistics-table names", () => {
		expect(senderPostalCodeOf("台北市", "中正區")).toBe("100");
		expect(senderPostalCodeOf("雲林縣", "台西鄉")).toBe("638");
		expect(senderPostalCodeOf("屏東縣", "霧台鄉")).toBe("902");
		expect(senderPostalCodeOf("台東縣", "台東市")).toBe("950");
	});

	it("is empty when the district cannot be mapped", () => {
		expect(senderPostalCodeOf("台北市", "不存在區")).toBe("");
		expect(senderPostalCodeOf("台北市", "")).toBe("");
		expect(senderPostalCodeOf("", "")).toBe("");
	});
});

describe("logistics name table vs twaddr", () => {
	it("every (county, district) in the logistics table maps to a twaddr town (22 / 368)", () => {
		let count = 0;
		for (const city of TAIWAN_CITY_OPTIONS) {
			for (const district of getDistrictOptions(city)) {
				count++;
				expect(townByName(city, district), `${city}${district}`).not.toBeNull();
				expect(
					senderPostalCodeOf(city, district),
					`${city}${district}`,
				).not.toBe("");
			}
		}
		expect(TAIWAN_CITY_OPTIONS).toHaveLength(22);
		expect(count).toBe(368);
	});

	it("every twaddr town, written in the logistics spelling, is in the logistics table", () => {
		for (const town of TOWNS) {
			const city = toLogisticsName(town.countyName);
			const district = toLogisticsName(town.name);
			expect(TAIWAN_CITY_OPTIONS, city).toContain(city);
			expect(getKnownDistrict(city, district), `${city}${district}`).toBe(
				district,
			);
		}
	});
});
