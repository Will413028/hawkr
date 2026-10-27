import { create } from "@bufbuild/protobuf";
import { StoreSchema } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import {
	storeChangesFromKyc,
	storeFormAddressError,
	storeFormChanges,
	storeFormContactErrors,
	storeFormDisplay,
	storeFormOf,
} from "./store-form";

const TAIPEI_XINYI = "63000010";

describe("storeFormOf", () => {
	it("an unset store is an empty form", () => {
		expect(storeFormOf(undefined)).toEqual({
			storeName: "",
			description: "",
			contactPhone: "",
			contactName: "",
			contactMobile: "",
			supportEmail: "",
			countyCode: "",
			townCode: "",
			addressDetail: "",
			prize: "",
			websiteUrl: "",
		});
	});

	it("derives the county from the 8-digit town code", () => {
		const form = storeFormOf(
			create(StoreSchema, {
				storeName: "山田",
				address: { townCode: TAIPEI_XINYI, addressDetail: "信義路 1 號" },
			}),
		);
		expect(form.townCode).toBe(TAIPEI_XINYI);
		expect(form.countyCode).toBe(TAIPEI_XINYI.slice(0, 5));
		expect(form.addressDetail).toBe("信義路 1 號");
	});
});

describe("storeFormChanges", () => {
	const baseline = storeFormOf(
		create(StoreSchema, {
			storeName: "山田",
			description: "產地直送",
			address: { townCode: TAIPEI_XINYI, addressDetail: "信義路 1 號" },
		}),
	);

	it("an untouched form has no changes", () => {
		expect(storeFormChanges(baseline, baseline)).toEqual({});
	});

	it("only the edited fields appear", () => {
		expect(
			storeFormChanges({ ...baseline, storeName: "山田農場" }, baseline),
		).toEqual({ storeName: "山田農場" });
	});

	it("clearing a text field is a change to the empty string", () => {
		expect(
			storeFormChanges({ ...baseline, description: "" }, baseline),
		).toEqual({
			description: "",
		});
	});

	it("an address change replaces the whole address", () => {
		expect(
			storeFormChanges({ ...baseline, addressDetail: "信義路 2 號" }, baseline),
		).toEqual({
			address: { townCode: TAIPEI_XINYI, addressDetail: "信義路 2 號" },
		});
	});

	it("clearing town and detail clears the address", () => {
		expect(
			storeFormChanges(
				{ ...baseline, countyCode: "", townCode: "", addressDetail: "" },
				baseline,
			),
		).toEqual({ address: null });
	});

	it("never sends a detail without a town", () => {
		expect(storeFormChanges({ ...baseline, townCode: "" }, baseline)).toEqual(
			{},
		);
		expect(storeFormAddressError({ ...baseline, townCode: "" })).toBeTruthy();
		expect(storeFormAddressError(baseline)).toBeUndefined();
	});
});

describe("storeFormDisplay", () => {
	it("shows the address as county, town and detail", () => {
		const form = storeFormOf(
			create(StoreSchema, {
				address: { townCode: TAIPEI_XINYI, addressDetail: "信義路 1 號" },
			}),
		);
		expect(storeFormDisplay(form, "address")).toMatch(/信義路 1 號$/);
		expect(storeFormDisplay(form, "address")).toMatch(/^臺北市/);
	});
});

describe("聯絡資料", () => {
	const baseline = storeFormOf(
		create(StoreSchema, {
			contactName: "王小明",
			contactMobile: "0912345678",
			supportEmail: "shop@example.com",
		}),
	);

	it("三個聯絡欄位各自是一項變更", () => {
		expect(
			storeFormChanges(
				{ ...baseline, contactName: "李大華", supportEmail: "a@example.com" },
				baseline,
			),
		).toEqual({ contactName: "李大華", supportEmail: "a@example.com" });
	});

	it("格式錯誤：手機、信箱、聯絡人最長 20 字；空白不報錯", () => {
		expect(storeFormContactErrors(baseline)).toEqual({
			nameErr: null,
			mobileErr: null,
			emailErr: null,
		});
		const bad = storeFormContactErrors({
			...baseline,
			contactName: "字".repeat(21),
			contactMobile: "0212345678",
			supportEmail: "not-an-email",
		});
		expect(bad.nameErr).not.toBeNull();
		expect(bad.mobileErr).not.toBeNull();
		expect(bad.emailErr).not.toBeNull();
		expect(
			storeFormContactErrors({
				...baseline,
				contactName: "",
				contactMobile: "",
				supportEmail: "",
			}),
		).toEqual({ nameErr: null, mobileErr: null, emailErr: null });
	});
});

describe("storeChangesFromKyc", () => {
	const store = create(StoreSchema, {
		contactName: "王小明",
		address: { townCode: TAIPEI_XINYI, addressDetail: "信義路 1 號" },
	});
	const draft = {
		contact: {
			contactName: "王小明",
			contactMobile: "",
			supportEmail: "",
		},
		address: {
			countyCode: TAIPEI_XINYI.slice(0, 5),
			townCode: TAIPEI_XINYI,
			detail: "信義路 1 號",
		},
	};

	it("沒有改動不寫店家", () => {
		expect(storeChangesFromKyc(store, draft)).toEqual({});
	});

	it("只列改過的欄位，輸入頭尾空白去掉", () => {
		expect(
			storeChangesFromKyc(store, {
				...draft,
				contact: { ...draft.contact, contactMobile: " 0912345678 " },
			}),
		).toEqual({ contactMobile: "0912345678" });
	});
});
