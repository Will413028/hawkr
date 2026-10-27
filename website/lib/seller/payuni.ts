/**
 * 賣家看到的 PayUni 狀態（`KYCFlow.payuni`，由伺服器推導）在畫面上的呈現。
 *
 * 狀態、說明句、審核意見、能修正的欄位都由伺服器決定：這裡只有標題（共用文案目錄的名詞）、樣式與入口的判斷，
 * 沒有「值 → 動作」的表。`explanation` 是伺服器句子，直接顯示；`reason` 是 PayUni 的審核意見原文。
 */

import type { KYCField } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	PayUniState,
	type SellerPayUni,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { commonUnknown, sellerPayUniState } from "@hawkr/ui-text/keys";
import { optionalText, type UiText } from "../i18n/ui-text";
import { fieldsByControl, kycFieldLabel } from "./kyc";
import { isKnownStoreField, storeFieldLabel } from "./store-contact";

/** 有 PayUni 狀態要顯示：`flow.payuni` 有值而且不是 APPROVED（APPROVED 不顯示面板）。 */
export function payuniHasState(
	payuni: SellerPayUni | null | undefined,
): payuni is SellerPayUni {
	return payuni != null && payuni.state !== PayUniState.APPROVED;
}

/** 狀態的標題（共用文案目錄的名詞）；UNSPECIFIED 與不認得的值是「未知」。 */
export function payuniStateTitle(ut: UiText, state: PayUniState): string {
	return optionalText(ut, sellerPayUniState(state)) ?? ut(commonUnknown());
}

/** 樣式：審核中不是警示，其他（含不認得的值）是警示。 */
export function payuniStateAlert(state: PayUniState): boolean {
	return state !== PayUniState.REVIEWING;
}

/** 賣家要修正的欄位名稱（伺服器列出的申請欄位與店家欄位，不認得的略過）。 */
export function payuniFixFieldLabels(
	ut: UiText,
	payuni: SellerPayUni | null | undefined,
): string[] {
	const labels: string[] = [];
	// 申請欄位依表單控制項去重（與缺項清單同一條規則）：銀行選擇器同時被指出銀行名稱與代碼時只列「銀行名稱」。
	for (const { field } of fieldsByControl(
		(payuni?.fixFields ?? []) as KYCField[],
	)) {
		const label = kycFieldLabel(ut, field);
		if (label) labels.push(label);
	}
	for (const field of payuni?.fixStoreFields ?? []) {
		const label = storeFieldLabel(ut, field);
		if (label && !labels.includes(label)) labels.push(label);
	}
	return labels;
}

/** 要在店家設定修正的欄位（聯絡資料、地址）有認得的值：畫面要給店家設定的入口。 */
export function payuniFixPointsAtStore(
	payuni: SellerPayUni | null | undefined,
): boolean {
	return (payuni?.fixStoreFields ?? []).some(isKnownStoreField);
}
