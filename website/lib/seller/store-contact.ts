/**
 * 店家的聯絡資料（聯絡人姓名、聯絡人手機、客服電子信箱）與送出 KYC 之前還缺的店家欄位。
 * 這些欄位屬於店家（`UpdateMyStore`）：PayUni 綁定與同步都讀店家的值，KYC 申請上沒有它們的副本。
 */

import type { StoreField } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { sellerStoreField } from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "../i18n/ui-text";
import type { CertificationSectionId, MissingItem } from "./kyc";
import {
	validateContactName,
	validateEmail,
	validateMobile,
} from "./validators";

export interface StoreContact {
	contactName: string;
	contactMobile: string;
	supportEmail: string;
}

export const EMPTY_STORE_CONTACT: StoreContact = {
	contactName: "",
	contactMobile: "",
	supportEmail: "",
};

export interface StoreContactErrors {
	nameErr: string | null;
	mobileErr: string | null;
	emailErr: string | null;
}

/** 已填的欄位的格式錯誤；空白不在這裡報（是否必填由伺服器的缺項決定）。 */
export function storeContactErrors(contact: StoreContact): StoreContactErrors {
	return {
		nameErr: contact.contactName
			? validateContactName(contact.contactName)
			: null,
		mobileErr: contact.contactMobile
			? validateMobile(contact.contactMobile)
			: null,
		emailErr: contact.supportEmail ? validateEmail(contact.supportEmail) : null,
	};
}

export function hasStoreContactErrors(errors: StoreContactErrors): boolean {
	return (
		errors.nameErr !== null ||
		errors.mobileErr !== null ||
		errors.emailErr !== null
	);
}

/** 店家欄位的呈現名稱（共用文案目錄）；UNSPECIFIED 與不認得的欄位回 undefined。 */
export function storeFieldLabel(
	ut: UiText,
	field: StoreField,
): string | undefined {
	return optionalText(ut, sellerStoreField(field));
}

/** 目錄認得的店家欄位（UNSPECIFIED 與不認得的值不是）；不需要文字時判斷用。 */
export function isKnownStoreField(field: StoreField): boolean {
	return sellerStoreField(field) !== undefined;
}

const CONTACT_FIELD_OF_PATH: Record<string, keyof StoreContactErrors> = {
	"store.contact_name": "nameErr",
	"store.contact_mobile": "mobileErr",
	"store.support_email": "emailErr",
};

/** UpdateMyStore 的欄位錯誤（`store.` 加 Store 的欄位名）對應到聯絡欄位的錯誤文字；其他欄位忽略。 */
export function storeContactServerErrors(
	fields: Record<string, string>,
): StoreContactErrors {
	const out: StoreContactErrors = {
		nameErr: null,
		mobileErr: null,
		emailErr: null,
	};
	for (const [path, message] of Object.entries(fields)) {
		const key = CONTACT_FIELD_OF_PATH[path];
		if (key) out[key] = message;
	}
	return out;
}

/** 伺服器的 `missing_store_fields` 轉成畫面的缺項清單：缺什麼只由伺服器決定，這裡不依本機輸入增減。 */
export function storeMissingItemsOf(
	ut: UiText,
	missing: readonly StoreField[],
	sectionId: CertificationSectionId = "other",
): MissingItem[] {
	const out: MissingItem[] = [];
	for (const field of missing) {
		const label = storeFieldLabel(ut, field);
		if (label) out.push({ sectionId, label });
	}
	return out;
}

/**
 * SubmitKYC 回 failed_precondition 之後重讀申請：伺服器的 `missing_store_fields` 有缺項就列出缺哪些，沒有缺項時
 * （例如身分比對的固定訊息）原樣顯示伺服器的訊息。
 */
export function submitPreconditionMessage(
	ut: UiText,
	missing: readonly StoreField[],
	serverMessage: string,
): string {
	const items = storeMissingItemsOf(ut, missing);
	return items.length > 0
		? `下列欄位尚未填寫：${items.map((item) => item.label).join("、")}`
		: serverMessage;
}
