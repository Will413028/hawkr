import { StoreField } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	EMPTY_STORE_CONTACT,
	storeContactErrors,
	storeContactServerErrors,
	storeFieldLabel,
	storeMissingItemsOf,
	submitPreconditionMessage,
} from "./store-contact";
import { SUPPORT_EMAIL_MAX_LENGTH } from "./validators";

const ut = uiTextFor("zh-TW");

describe("storeMissingItemsOf", () => {
	it("只依伺服器的清單列出，帶區塊與名稱，不依本機輸入增減", () => {
		expect(
			storeMissingItemsOf(ut, [
				StoreField.ADDRESS,
				StoreField.CONTACT_NAME,
				StoreField.SUPPORT_EMAIL,
			]),
		).toEqual([
			{ sectionId: "other", label: "店家地址" },
			{ sectionId: "other", label: "聯絡人姓名" },
			{ sectionId: "other", label: "客服電子信箱" },
		]);
	});

	it("沒有缺項、不認得的欄位都不列出", () => {
		expect(storeMissingItemsOf(ut, [])).toEqual([]);
		expect(storeMissingItemsOf(ut, [StoreField.UNSPECIFIED])).toEqual([]);
	});
});

describe("submitPreconditionMessage", () => {
	it("伺服器重讀後有缺項就列缺項，沒有時顯示伺服器的訊息", () => {
		expect(
			submitPreconditionMessage(
				ut,
				[StoreField.CONTACT_MOBILE, StoreField.ADDRESS],
				"x",
			),
		).toBe("下列欄位尚未填寫：聯絡人手機、店家地址");
		expect(submitPreconditionMessage(ut, [], "無法通過認證，請聯絡客服")).toBe(
			"無法通過認證，請聯絡客服",
		);
	});
});

describe("店家欄位的名稱", () => {
	it("名稱與 StoreField 一對一", () => {
		expect(storeFieldLabel(ut, StoreField.CONTACT_PHONE)).toBe("商店電話");
		expect(storeFieldLabel(ut, StoreField.UNSPECIFIED)).toBeUndefined();
	});
});

describe("storeContactServerErrors", () => {
	it("UpdateMyStore 的欄位錯誤對應到聯絡欄位，其他欄位忽略", () => {
		expect(
			storeContactServerErrors({
				"store.contact_name": "必填",
				"store.support_email": "必填",
				"store.store_name": "x",
			}),
		).toEqual({ nameErr: "必填", mobileErr: null, emailErr: "必填" });
	});
});

describe("storeContactErrors", () => {
	it("只報已填欄位的格式錯誤", () => {
		expect(storeContactErrors(EMPTY_STORE_CONTACT)).toEqual({
			nameErr: null,
			mobileErr: null,
			emailErr: null,
		});
		expect(
			storeContactErrors({
				contactName: "王",
				contactMobile: "09",
				supportEmail: "x",
			}).mobileErr,
		).not.toBeNull();
	});

	it("客服信箱與聯絡人姓名的長度上限等於 PayUni 的上限", () => {
		const email = (n: number) => `${"a".repeat(n - 12)}@example.com`;
		expect(email(SUPPORT_EMAIL_MAX_LENGTH).length).toBe(
			SUPPORT_EMAIL_MAX_LENGTH,
		);
		const at = (supportEmail: string, contactName = "王") =>
			storeContactErrors({ contactName, contactMobile: "", supportEmail });
		expect(at(email(SUPPORT_EMAIL_MAX_LENGTH)).emailErr).toBeNull();
		expect(at(email(SUPPORT_EMAIL_MAX_LENGTH + 1)).emailErr).not.toBeNull();
		expect(at("a@example.com", "字".repeat(20)).nameErr).toBeNull();
		expect(at("a@example.com", "字".repeat(21)).nameErr).not.toBeNull();
	});
});
