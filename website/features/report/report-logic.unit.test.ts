import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { SELLER_CREATE_REPORT_REQUEST_CATEGORY } from "@hawkr/ui-text/accepted";
import { describe, expect, it } from "vitest";
import { uiTextFor } from "@/lib/i18n/ui-text";
import {
	DESCRIPTION_MAX,
	reportCategoryOptions,
	reportSuccessText,
	validateReportDraft,
} from "./report-logic";

const COMMUNITY_ONLY = [
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
	PolicyCategory.SEXUAL_CONTENT,
];

describe("reportCategoryOptions", () => {
	const options = reportCategoryOptions(uiTextFor("zh-TW"));
	const values = options.map((o) => o.value);

	it("is the 12 values of CreateReportRequest.category, in contract order", () => {
		expect(values).toEqual([...SELLER_CREATE_REPORT_REQUEST_CATEGORY]);
		expect(values).toHaveLength(12);
	});

	it("excludes the 4 community categories, PLATFORM_ABUSE and UNSPECIFIED", () => {
		for (const c of [
			...COMMUNITY_ONLY,
			PolicyCategory.PLATFORM_ABUSE,
			PolicyCategory.UNSPECIFIED,
		]) {
			expect(values).not.toContain(c);
		}
	});

	it("gives every option a catalog label", () => {
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

	it("rejects every community category as a store-report category", () => {
		for (const category of COMMUNITY_ONLY) {
			expect(
				validateReportDraft({ category, description: "x", orderNo: "" })
					.category,
			).toBe("categoryRequired");
		}
	});

	it("counts runes, not UTF-16 units", () => {
		const ok = "\u{1F600}".repeat(DESCRIPTION_MAX);
		expect(
			validateReportDraft({
				category: PolicyCategory.FRAUD,
				description: ok,
				orderNo: "",
			}),
		).toEqual({});
		expect(
			validateReportDraft({
				category: PolicyCategory.FRAUD,
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
