import { describe, expect, it } from "vitest";
import { SellerTaxReviewStatus } from "@/lib/codemeta/enums";
import { invoiceTaxStatusBanner } from "./invoiceTaxStatusBanner";

const ALL = Object.values(SellerTaxReviewStatus);

describe("invoiceTaxStatusBanner", () => {
	it("covers all 11 SellerTaxReviewStatus values without falling to default", () => {
		expect(ALL).toHaveLength(11);
		const titles = new Set<string>();
		for (const status of ALL) {
			const banner = invoiceTaxStatusBanner({ reviewStatus: status });
			expect(banner.unknown, `${status} fell to the default branch`).toBe(
				false,
			);
			expect(banner.title).not.toBe("狀態更新中");
			titles.add(banner.title);
		}
		expect(titles.size).toBe(10);
	});

	it("falls to 狀態更新中 for an unknown value instead of a blank screen", () => {
		const banner = invoiceTaxStatusBanner({ reviewStatus: "FOO" });
		expect(banner.unknown).toBe(true);
		expect(banner.title).toBe("狀態更新中");
		expect(banner.tone).toBe("neutral");
		expect(banner.ctaHref).toBeUndefined();
	});

	it("only shows the deadline countdown for the pending-review group", () => {
		const pending: string[] = [
			SellerTaxReviewStatus.SUBMITTED,
			SellerTaxReviewStatus.UNDER_REVIEW,
			SellerTaxReviewStatus.SELF_ISSUE_PENDING,
			SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
		];
		for (const status of ALL) {
			expect(
				invoiceTaxStatusBanner({ reviewStatus: status }).showDeadline,
				status,
			).toBe(pending.includes(status));
		}
	});

	it("marks the editable group correctly", () => {
		const editable: string[] = [
			SellerTaxReviewStatus.NOT_STARTED,
			SellerTaxReviewStatus.DRAFT,
			SellerTaxReviewStatus.REJECTED,
			SellerTaxReviewStatus.EXPIRED,
			SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
		];
		for (const status of ALL) {
			expect(
				invoiceTaxStatusBanner({ reviewStatus: status }).editable,
				status,
			).toBe(editable.includes(status));
		}
	});

	it("threads the reapply cooldown text into REJECTED / EXPIRED", () => {
		const banner = invoiceTaxStatusBanner({
			reviewStatus: SellerTaxReviewStatus.REJECTED,
			cooldownUntilText: "2026/09/06",
		});
		expect(banner.description).toContain("可於 2026/09/06 後重新申請。");
	});

	it("只有審核進行中的三個狀態提示「可以先簽收據代製授權」", () => {
		const hinted: string[] = [
			SellerTaxReviewStatus.SUBMITTED,
			SellerTaxReviewStatus.UNDER_REVIEW,
			SellerTaxReviewStatus.SUPPLEMENT_REQUIRED,
			SellerTaxReviewStatus.SELF_ISSUE_PENDING,
		];
		for (const status of ALL) {
			expect(
				invoiceTaxStatusBanner({ reviewStatus: status }).description.includes(
					"收據代製授權",
				),
				status,
			).toBe(hinted.includes(status) || status === "APPROVED_INVOICE_EXEMPT");
		}
		expect(
			invoiceTaxStatusBanner({
				reviewStatus: SellerTaxReviewStatus.UNDER_REVIEW,
			}).description,
		).toContain("若你屬於免用統一發票");
	});

	it("adds the navigation CTA only when the caller asks for it", () => {
		const withCta = invoiceTaxStatusBanner({
			reviewStatus: SellerTaxReviewStatus.DRAFT,
			withNavigationCta: true,
		});
		expect(withCta.ctaHref).toBe("/seller-center/invoices/tax-status");

		const withoutCta = invoiceTaxStatusBanner({
			reviewStatus: SellerTaxReviewStatus.DRAFT,
		});
		expect(withoutCta.ctaHref).toBeUndefined();
	});
});
