import { Gender } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	GENDER_CHOICES,
	genderFromValue,
	genderOptions,
	genderText,
} from "./genderOptions";

describe("gender options", () => {
	it("zh-TW labels come from the catalog, in proto order, with the empty entry first", () => {
		const options = genderOptions(uiTextFor("zh-TW"));
		expect(options.map((o) => o.label)).toEqual([
			"尚未設定",
			"男性",
			"女性",
			"其他",
		]);
		expect(options[0]).toMatchObject({ value: "", empty: true });
		expect(options.slice(1).some((o) => o.empty)).toBe(false);
	});

	it("en labels come from the catalog", () => {
		expect(genderOptions(uiTextFor("en")).map((o) => o.label)).toEqual([
			"Not set",
			"Male",
			"Female",
			"Other",
		]);
	});

	it("never offers the unspecified value", () => {
		expect(GENDER_CHOICES).toEqual([Gender.MALE, Gender.FEMALE, Gender.OTHER]);
		expect(genderFromValue(String(Gender.UNSPECIFIED))).toBeUndefined();
	});

	it("maps a menu value back to the enum, and the empty entry to undefined", () => {
		expect(genderFromValue(String(Gender.FEMALE))).toBe(Gender.FEMALE);
		expect(genderFromValue("")).toBeUndefined();
	});

	it("shows a value without catalog text as not set, same as no value", () => {
		const ut = uiTextFor("zh-TW");
		expect(genderText(ut, 99 as Gender)).toBe(genderText(ut, undefined));
		expect(genderText(ut, Gender.UNSPECIFIED)).toBe("尚未設定");
	});

	it("shows an unset gender as the placeholder text", () => {
		expect(genderText(uiTextFor("zh-TW"), undefined)).toBe("尚未設定");
	});
});
