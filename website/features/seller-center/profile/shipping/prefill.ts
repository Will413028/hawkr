import {
	postalCodeByTownCode,
	townByCode,
	townByName,
} from "@hawkr/twaddr/lookup";
import { toLogisticsName } from "@/lib/seller/certification-options";
import type { ShippingOnboarding } from "@/lib/seller/shipping";
import type { MyStore } from "@/lib/seller/store";

/**
 * 物流設定表單的初值。寄件人來源（兩個物流商相同）：
 *   - 該物流商有寄件人紀錄（onboarding row 的寄件人欄位有值，任何狀態，含失敗）就用紀錄；
 *   - 沒有才用店家（聯絡人姓名、聯絡人手機、店家地址）；店家缺的欄位留空，不退回商店電話或店名。
 * 地址（縣市、區、明細）整組取同一個來源。
 */

export interface HomeForm {
	senderName: string;
	senderPhone: string;
	senderCity: string;
	senderDistrict: string;
	senderAddressDetail: string;
}

export interface CvsSender {
	senderName: string;
	senderPhone: string;
}

function hasHomeSender(row: ShippingOnboarding | undefined): boolean {
	if (!row) return false;
	return [
		row.senderName,
		row.senderPhone,
		row.senderCity,
		row.senderDistrict,
		row.senderAddressDetail,
	].some((v) => (v ?? "").trim() !== "");
}

function hasCvsSender(row: ShippingOnboarding | undefined): boolean {
	if (!row) return false;
	return [row.senderName, row.senderPhone].some((v) => (v ?? "").trim() !== "");
}

export function homeFormInitial(
	row: ShippingOnboarding | undefined,
	store: MyStore | null | undefined,
): HomeForm {
	if (row && hasHomeSender(row)) {
		return {
			senderName: row.senderName ?? "",
			senderPhone: row.senderPhone ?? "",
			senderCity: toLogisticsName(row.senderCity),
			senderDistrict: toLogisticsName(row.senderDistrict),
			senderAddressDetail: row.senderAddressDetail ?? "",
		};
	}
	const s = store?.store;
	const town = s?.address ? townByCode(s.address.townCode) : undefined;
	return {
		senderName: s?.contactName ?? "",
		senderPhone: s?.contactMobile ?? "",
		senderCity: town ? toLogisticsName(town.countyName) : "",
		senderDistrict: town ? toLogisticsName(town.name) : "",
		senderAddressDetail: s?.address?.addressDetail ?? "",
	};
}

/** 7-11 常溫、冷凍兩筆 row 中被選定的那一筆：常溫優先，沒有才冷凍。寄件人、鎖定、狀態都取自它。 */
export function cvsReferenceRow(
	ambientRow: ShippingOnboarding | undefined,
	frozenRow: ShippingOnboarding | undefined,
): ShippingOnboarding | undefined {
	return ambientRow ?? frozenRow;
}

/**
 * 7-11 的寄件人：`reference` 是選定的那一筆 row（常溫，沒有才冷凍；鎖定與狀態也取自同一筆）。
 * 該筆有寄件人紀錄就用紀錄，沒有才用店家。
 */
export function cvsSenderInitial(
	reference: ShippingOnboarding | undefined,
	store: MyStore | null | undefined,
): CvsSender {
	if (reference && hasCvsSender(reference)) {
		return {
			senderName: reference.senderName ?? "",
			senderPhone: reference.senderPhone ?? "",
		};
	}
	return {
		senderName: store?.store?.contactName ?? "",
		senderPhone: store?.store?.contactMobile ?? "",
	};
}

/**
 * 表單選的縣市與區（物流名稱表的寫法）對應的郵遞區號；對不到代碼回空字串，呼叫端必須擋下送出。
 */
export function senderPostalCodeOf(city: string, district: string): string {
	const town = townByName(city, district);
	return town ? postalCodeByTownCode(town.code) : "";
}
