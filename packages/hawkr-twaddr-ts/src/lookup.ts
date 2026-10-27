// 台灣鄉鎮市區代碼的查詢。資料表由 backend/internal/platform/twaddr/generate.go 生成到 twaddr.gen.ts，
// 這裡只放手寫的型別與查詢（內政部寫法「臺」，順序是郵遞區號）。
import { COUNTIES as COUNTY_ROWS, TOWNS as TOWN_ROWS } from "./twaddr.gen";

export interface County {
	code: string;
	name: string;
}

export interface Town {
	code: string;
	name: string;
	countyCode: string;
	countyName: string;
	postalCode: string;
}

export const COUNTIES: readonly County[] = COUNTY_ROWS;

const COUNTIES_BY_CODE: ReadonlyMap<string, County> = new Map(
	COUNTIES.map((c) => [c.code, c]),
);

export const TOWNS: readonly Town[] = TOWN_ROWS.map((t) => ({
	...t,
	// 縣市名稱取自縣市表；每個鄉鎮市區的縣市代碼都在表內（測試保證）。
	countyName: COUNTIES_BY_CODE.get(t.countyCode)?.name ?? "",
}));

const TOWNS_BY_CODE: ReadonlyMap<string, Town> = new Map(
	TOWNS.map((t) => [t.code, t]),
);

const TOWNS_BY_COUNTY: ReadonlyMap<string, readonly Town[]> = (() => {
	const map = new Map<string, Town[]>();
	for (const town of TOWNS) {
		const list = map.get(town.countyCode);
		if (list) list.push(town);
		else map.set(town.countyCode, [town]);
	}
	return map;
})();

/** 以 8 碼代碼找鄉鎮市區；不是現行的代碼回 undefined。 */
export function townByCode(code: string): Town | undefined {
	return TOWNS_BY_CODE.get(code);
}

/** 以 5 碼代碼找縣市；不是現行的代碼回 undefined。 */
export function countyByCode(code: string): County | undefined {
	return COUNTIES_BY_CODE.get(code);
}

/** 指定縣市代碼下的鄉鎮市區（郵遞區號順序）。 */
export function townsByCounty(countyCode: string): readonly Town[] {
	return TOWNS_BY_COUNTY.get(countyCode) ?? [];
}

/** 縣市代碼是鄉鎮市區代碼的前 5 碼。 */
export function countyCodeOfTownCode(code: string): string {
	return code.slice(0, 5);
}

/** 以 8 碼鄉鎮市區代碼找郵遞區號；不認得的代碼回空字串。 */
export function postalCodeByTownCode(code: string): string {
	return TOWNS_BY_CODE.get(code)?.postalCode ?? "";
}

/** 把「台」換成內政部寫法的「臺」（整個字串）。 */
function toMoiSpelling(name: string): string {
	return name.trim().replaceAll("台", "臺");
}

const TOWNS_BY_NAME: ReadonlyMap<string, Town> = new Map(
	TOWNS.map((t) => [`${t.countyName}\u0000${t.name}`, t]),
);

/**
 * 以縣市名稱與鄉鎮市區名稱找鄉鎮市區（精確比對）。輸入的「台」先換成「臺」，所以物流名稱表的
 * 「台北市」「台西鄉」也找得到；找不到回 null。
 */
export function townByName(countyName: string, townName: string): Town | null {
	return (
		TOWNS_BY_NAME.get(
			`${toMoiSpelling(countyName)}\u0000${toMoiSpelling(townName)}`,
		) ?? null
	);
}

/** 縣市＋鄉鎮市區＋明細；不認得的代碼只回明細。 */
export function formatAddress(townCode: string, detail: string): string {
	const town = TOWNS_BY_CODE.get(townCode);
	if (!town) return detail;
	return `${town.countyName}${town.name}${detail}`;
}
