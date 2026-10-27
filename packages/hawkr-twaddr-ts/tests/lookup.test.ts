import { describe, expect, it } from "vitest";
import {
	COUNTIES,
	countyByCode,
	countyCodeOfTownCode,
	formatAddress,
	postalCodeByTownCode,
	TOWNS,
	townByCode,
	townByName,
	townsByCounty,
} from "../src/lookup";

describe("twaddr", () => {
	it("has 22 counties and 368 towns, every town under a listed county", () => {
		expect(COUNTIES).toHaveLength(22);
		expect(TOWNS).toHaveLength(368);
		for (const town of TOWNS) {
			expect(countyByCode(town.countyCode)).toBeDefined();
			expect(town.countyName).toBe(countyByCode(town.countyCode)?.name);
			expect(countyCodeOfTownCode(town.code)).toBe(town.countyCode);
		}
	});

	it("looks up by code with the MOI spelling", () => {
		expect(townByCode("63000050")).toMatchObject({
			name: "中正區",
			countyName: "臺北市",
			postalCode: "100",
		});
		expect(townByCode("")).toBeUndefined();
		expect(postalCodeByTownCode("63000050")).toBe("100");
		expect(postalCodeByTownCode("unknown")).toBe("");
		expect(townsByCounty("63000")).toHaveLength(12);
		expect(townsByCounty("none")).toEqual([]);
	});

	it("formats an address from the code", () => {
		expect(formatAddress("63000050", "重慶南路一段 122 號")).toBe(
			"臺北市中正區重慶南路一段 122 號",
		);
		expect(formatAddress("unknown", "某地")).toBe("某地");
	});

	it("orders counties and towns by postal code", () => {
		expect(COUNTIES[0].name).toBe("臺北市");
		for (let i = 1; i < TOWNS.length; i++) {
			const a = TOWNS[i - 1];
			const b = TOWNS[i];
			expect(
				a.postalCode < b.postalCode ||
					(a.postalCode === b.postalCode && a.code < b.code),
			).toBe(true);
		}
	});

	it("finds a town by county and town name, accepting the 台 spelling", () => {
		expect(townByName("臺北市", "中正區")?.code).toBe("63000050");
		expect(townByName("台北市", "中正區")?.code).toBe("63000050");
		const xi = townByName("雲林縣", "台西鄉");
		expect(xi).toMatchObject({ name: "臺西鄉", countyName: "雲林縣" });
		expect(townByName("屏東縣", "霧台鄉")).toMatchObject({ name: "霧臺鄉" });
		expect(townByName("台東縣", "台東市")).toMatchObject({
			name: "臺東市",
			countyName: "臺東縣",
		});
	});

	it("trims surrounding whitespace on both names", () => {
		expect(townByName(" 台北市 ", "\t中正區\n")?.code).toBe("63000050");
		expect(townByName("雲林縣", " 台西鄉 ")?.code).toBe("10009160");
	});

	it("returns null when the county, town or pairing is unknown", () => {
		expect(townByName("台北市", "不存在區")).toBeNull();
		expect(townByName("不存在市", "中正區")).toBeNull();
		expect(townByName("新北市", "信義區")).toBeNull();
		expect(townByName("", "")).toBeNull();
	});

	it("round-trips every town through its own names", () => {
		for (const town of TOWNS) {
			expect(townByName(town.countyName, town.name)?.code).toBe(town.code);
		}
	});
});
