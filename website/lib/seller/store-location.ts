import { townByCode } from "@hawkr/twaddr/lookup";

export interface StoreLocation {
	city: string;
	district: string;
}

/** 卡片的 8 碼鄉鎮市區代碼轉成縣市與鄉鎮市區名稱（內政部寫法「臺」）；沒有設定或不是現行代碼回 undefined。 */
export function storeLocation(townCode: string): StoreLocation | undefined {
	const town = townByCode(townCode);
	return town ? { city: town.countyName, district: town.name } : undefined;
}
