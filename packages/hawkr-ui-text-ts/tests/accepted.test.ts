import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { describe, expect, it } from "vitest";
import {
	MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY,
	SELLER_CREATE_REPORT_REQUEST_CATEGORY,
} from "../src/accepted.gen";

const COMMUNITY = [
	PolicyCategory.SPAM,
	PolicyCategory.HARASSMENT,
	PolicyCategory.HATE,
	PolicyCategory.SEXUAL_CONTENT,
];

describe("accepted_policy_categories constants", () => {
	it("never contain UNSPECIFIED or duplicates", () => {
		for (const list of [
			SELLER_CREATE_REPORT_REQUEST_CATEGORY,
			MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY,
		]) {
			expect(list).not.toContain(PolicyCategory.UNSPECIFIED);
			expect(new Set(list).size).toBe(list.length);
		}
	});

	it("store reports take 12 values and none of the 4 community categories", () => {
		expect(SELLER_CREATE_REPORT_REQUEST_CATEGORY).toHaveLength(12);
		for (const c of [...COMMUNITY, PolicyCategory.PLATFORM_ABUSE]) {
			expect(SELLER_CREATE_REPORT_REQUEST_CATEGORY).not.toContain(c);
		}
	});

	it("community reports take 8 values, the 4 community categories included", () => {
		expect(MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY).toHaveLength(8);
		for (const c of COMMUNITY) {
			expect(MODERATION_SUBMIT_REPORT_REQUEST_CATEGORY).toContain(c);
		}
	});
});
