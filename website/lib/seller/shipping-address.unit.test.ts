import { describe, expect, it } from "vitest";
import {
	addressNameMatches,
	validationMatchesSelection,
} from "@/lib/seller/shipping";

describe("addressNameMatches", () => {
	it("ignores 臺／台 anywhere in the string", () => {
		expect(addressNameMatches("臺北市", "台北市")).toBe(true);
		expect(addressNameMatches("臺西鄉", "台西鄉")).toBe(true);
		expect(addressNameMatches("霧臺鄉", "霧台鄉")).toBe(true);
		expect(addressNameMatches("臺東市", "台東市")).toBe(true);
		expect(addressNameMatches("臺東縣", "台東縣")).toBe(true);
	});

	it("still tells different names apart", () => {
		expect(addressNameMatches("臺東市", "台西鄉")).toBe(false);
		expect(addressNameMatches("中正區", "大安區")).toBe(false);
	});
});

describe("validationMatchesSelection", () => {
	it("accepts TGOS 臺 answers for the form's 台 names (non-leading 台)", () => {
		expect(
			validationMatchesSelection(
				{ valid: true, county: "雲林縣", district: "臺西鄉" },
				"雲林縣",
				"台西鄉",
			),
		).toBe(true);
		expect(
			validationMatchesSelection(
				{ valid: true, county: "屏東縣", district: "霧臺鄉" },
				"屏東縣",
				"霧台鄉",
			),
		).toBe(true);
		expect(
			validationMatchesSelection(
				{ valid: true, county: "臺東縣", district: "臺東市" },
				"台東縣",
				"台東市",
			),
		).toBe(true);
	});

	it("rejects invalid results and mismatching county or district", () => {
		expect(validationMatchesSelection(undefined, "台北市", "中正區")).toBe(
			false,
		);
		expect(
			validationMatchesSelection({ valid: false }, "台北市", "中正區"),
		).toBe(false);
		expect(
			validationMatchesSelection(
				{ valid: true, county: "新北市", district: "中正區" },
				"台北市",
				"中正區",
			),
		).toBe(false);
		expect(
			validationMatchesSelection(
				{ valid: true, county: "臺北市", district: "大安區" },
				"台北市",
				"中正區",
			),
		).toBe(false);
	});

	it("skips a field TGOS did not return", () => {
		expect(
			validationMatchesSelection({ valid: true }, "台北市", "中正區"),
		).toBe(true);
	});
});
