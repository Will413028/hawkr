/**
 * 商店設定表單與 `Store` 的對照：表單的欄位、與伺服器版本比對出要寫入的欄位。
 * 地址是一個單位（`Store.address`，update_mask 列 address 時整個換掉），所以縣市、鄉鎮市區、明細一起比。
 */

import type { Store } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	countyCodeOfTownCode,
	formatAddress,
	townByCode,
} from "@hawkr/twaddr/lookup";
import type { StoreUpdate } from "./repo";
import {
	type StoreContact,
	type StoreContactErrors,
	storeContactErrors,
} from "./store-contact";

export interface StoreForm {
	storeName: string;
	description: string;
	contactPhone: string;
	contactName: string;
	contactMobile: string;
	supportEmail: string;
	/** 縣市代碼（5 碼）；選了縣市還沒選鄉鎮市區時 townCode 是空字串。 */
	countyCode: string;
	townCode: string;
	addressDetail: string;
	prize: string;
	websiteUrl: string;
}

export type StoreChanges = Omit<StoreUpdate, "etag">;

export function storeFormOf(store: Store | undefined): StoreForm {
	const townCode = store?.address?.townCode ?? "";
	return {
		storeName: store?.storeName ?? "",
		description: store?.description ?? "",
		contactPhone: store?.contactPhone ?? "",
		contactName: store?.contactName ?? "",
		contactMobile: store?.contactMobile ?? "",
		supportEmail: store?.supportEmail ?? "",
		countyCode: townCode ? countyCodeOfTownCode(townCode) : "",
		townCode,
		addressDetail: store?.address?.addressDetail ?? "",
		prize: store?.prize ?? "",
		websiteUrl: store?.websiteUrl ?? "",
	};
}

/** 表單相對於伺服器版本改了哪些欄位；只回有改的，沒改的不出現（不列進 update_mask）。 */
export function storeFormChanges(
	form: StoreForm,
	baseline: StoreForm,
): StoreChanges {
	const changes: StoreChanges = {};
	if (form.storeName !== baseline.storeName) changes.storeName = form.storeName;
	if (form.description !== baseline.description) {
		changes.description = form.description;
	}
	if (form.contactPhone !== baseline.contactPhone) {
		changes.contactPhone = form.contactPhone;
	}
	if (form.contactName !== baseline.contactName) {
		changes.contactName = form.contactName;
	}
	if (form.contactMobile !== baseline.contactMobile) {
		changes.contactMobile = form.contactMobile;
	}
	if (form.supportEmail !== baseline.supportEmail) {
		changes.supportEmail = form.supportEmail;
	}
	if (form.prize !== baseline.prize) changes.prize = form.prize;
	if (form.websiteUrl !== baseline.websiteUrl) {
		changes.websiteUrl = form.websiteUrl;
	}
	if (
		form.townCode !== baseline.townCode ||
		form.addressDetail !== baseline.addressDetail
	) {
		if (form.townCode) {
			changes.address = {
				townCode: form.townCode,
				addressDetail: form.addressDetail,
			};
		} else if (form.addressDetail === "") {
			changes.address = null;
		}
	}
	return changes;
}

/** KYC 頁送出時寫進店家的資料：地址與聯絡資料相對於伺服器上的店家改了哪些欄位。 */
export function storeChangesFromKyc(
	store: Store | undefined,
	draft: {
		contact: StoreContact;
		address: { countyCode: string; townCode: string; detail: string };
	},
): StoreChanges {
	const baseline = storeFormOf(store);
	return storeFormChanges(
		{
			...baseline,
			contactName: draft.contact.contactName.trim(),
			contactMobile: draft.contact.contactMobile.trim(),
			supportEmail: draft.contact.supportEmail.trim(),
			countyCode: draft.address.countyCode,
			townCode: draft.address.townCode,
			addressDetail: draft.address.detail.trim(),
		},
		baseline,
	);
}

const ADDRESS_INCOMPLETE_MESSAGE = "請選擇縣市與鄉鎮市區";

/** 有地址明細就一定要有鄉鎮市區：契約不接受只有明細的地址。 */
export function storeFormAddressError(form: StoreForm): string | undefined {
	if (form.addressDetail.trim() !== "" && form.townCode === "") {
		return ADDRESS_INCOMPLETE_MESSAGE;
	}
	return undefined;
}

/** KYC 頁的地址輸入：選了縣市或填了明細卻沒有鄉鎮市區，是還沒完成的本機修改（不能當成已儲存，也不能送出）。 */
export function storeAddressInputError(address: {
	countyCode: string;
	townCode: string;
	detail: string;
}): string | undefined {
	if (
		address.townCode === "" &&
		(address.countyCode !== "" || address.detail.trim() !== "")
	) {
		return ADDRESS_INCOMPLETE_MESSAGE;
	}
	return undefined;
}

/** 已填的聯絡資料的格式錯誤；空白表示清掉，格式以契約為準（手機 09 開頭 10 碼、信箱、聯絡人最多 20 字）。 */
export function storeFormContactErrors(form: StoreForm): StoreContactErrors {
	return storeContactErrors(form);
}

/** 代行確認抽屜的變更摘要用的欄位標籤（取表單自己的標籤）。 */
export const STORE_CHANGE_LABELS = {
	storeName: "商店名稱",
	description: "商店介紹",
	contactPhone: "商店電話",
	contactName: "聯絡人姓名",
	contactMobile: "聯絡人手機",
	supportEmail: "客服電子信箱",
	address: "店家地址",
	prize: "得獎紀錄 / 店鋪亮點",
	websiteUrl: "官方網站",
} as const;

export type StoreChangeField = keyof typeof STORE_CHANGE_LABELS;

/** 某個欄位在表單上的顯示值（變更摘要的 before／after）。 */
export function storeFormDisplay(
	form: StoreForm,
	field: StoreChangeField,
): string {
	if (field === "address") {
		return formatAddress(form.townCode, form.addressDetail);
	}
	return form[field];
}

export function townLocation(townCode: string): {
	county: string;
	town: string;
} {
	const town = townByCode(townCode);
	return { county: town?.countyName ?? "", town: town?.name ?? "" };
}
