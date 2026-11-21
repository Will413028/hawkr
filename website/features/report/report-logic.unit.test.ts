import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	DESCRIPTION_MAX,
	reportCategoryOptions,
	reportSuccessText,
	validateReportDraft,
} from "./report-logic";

describe("reportCategoryOptions", () => {
	const options = reportCategoryOptions(uiTextFor("zh-TW"));
	const values = options.map((o) => o.value);

	it("excludes PLATFORM_ABUSE and UNSPECIFIED", () => {
		expect(values).not.toContain(PolicyCategory.PLATFORM_ABUSE);
		expect(values).not.toContain(PolicyCategory.UNSPECIFIED);
	});

	it("lists every other contract value with a catalog label", () => {
		const expected = Object.values(PolicyCategory).filter(
			(v): v is PolicyCategory =>
				typeof v === "number" &&
				v !== PolicyCategory.UNSPECIFIED &&
				v !== PolicyCategory.PLATFORM_ABUSE,
		);
		expect(values.slice().sort()).toEqual(expected.sort());
		for (const o of options) expect(o.label).not.toContain("PolicyCategory");
	});
});

describe("validateReportDraft", () => {
	it("requires a selectable category and a non-blank description", () => {
		expect(
			validateReportDraft({
				category: undefined,
				description: "  ",
				orderNo: "",
			}),
		).toEqual({
			category: "categoryRequired",
			description: "descriptionRequired",
		});
		expect(
			validateReportDraft({
				category: PolicyCategory.PLATFORM_ABUSE,
				description: "x",
				orderNo: "",
			}).category,
		).toBe("categoryRequired");
	});

	it("counts runes, not UTF-16 units", () => {
		const ok = "\u{1F600}".repeat(DESCRIPTION_MAX);
		expect(
			validateReportDraft({
				category: PolicyCategory.SPAM,
				description: ok,
				orderNo: "",
			}),
		).toEqual({});
		expect(
			validateReportDraft({
				category: PolicyCategory.SPAM,
				description: `${ok}x`,
				orderNo: "",
			}).description,
		).toBe("descriptionTooLong");
	});
});

describe("reportSuccessText", () => {
	it("shows the server notice when supplemented", () => {
		expect(
			reportSuccessText(
				{ supplemented: true, notice: { message: "server sentence" } as never },
				"received",
			),
		).toBe("server sentence");
	});
	it("falls back to the received text otherwise", () => {
		expect(
			reportSuccessText({ supplemented: false, notice: undefined }, "received"),
		).toBe("received");
	});
});
