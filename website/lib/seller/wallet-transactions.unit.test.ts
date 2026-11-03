import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	KYCApplicationSchema,
	KYCStatus,
	PaymentAccountSchema,
	PaymentAccountStatus,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { publicListingStatus } from "./capability-fixtures";
import {
	platformReviewApproved,
	walletShowsTransactions,
} from "./wallet-transactions";

const store = (missing: SellerRequirement[] | undefined) =>
	create(GetMyStoreResponseSchema, {
		store: { id: "1" },
		capabilities: missing === undefined ? [] : [publicListingStatus(missing)],
	});

const account = (status: PaymentAccountStatus) =>
	create(PaymentAccountSchema, { status });

describe("walletShowsTransactions", () => {
	it("已認證而且收款帳戶是 ENABLED", () => {
		expect(
			walletShowsTransactions(store([]), account(PaymentAccountStatus.ENABLED)),
		).toBe(true);
	});

	it("還缺 ONBOARDING_COMPLETE、沒有 PUBLIC_LISTING、沒有收款帳戶：都不顯示", () => {
		const enabled = account(PaymentAccountStatus.ENABLED);
		expect(
			walletShowsTransactions(
				store([SellerRequirement.ONBOARDING_COMPLETE]),
				enabled,
			),
		).toBe(false);
		expect(walletShowsTransactions(store(undefined), enabled)).toBe(false);
		expect(walletShowsTransactions(store([]), null)).toBe(false);
		expect(walletShowsTransactions(null, enabled)).toBe(false);
	});

	it("收款帳戶還不能收款（含 UNSPECIFIED 與不認得的狀態）：不顯示", () => {
		for (const status of [
			PaymentAccountStatus.NOT_CREATED,
			PaymentAccountStatus.UNDER_REVIEW,
			PaymentAccountStatus.ACTION_REQUIRED,
			PaymentAccountStatus.DISABLED,
			PaymentAccountStatus.UNSPECIFIED,
			99 as PaymentAccountStatus,
		]) {
			expect(
				walletShowsTransactions(store([]), account(status)),
				String(status),
			).toBe(false);
		}
	});
});

describe("platformReviewApproved", () => {
	it("只有 APPROVED 的申請算核准", () => {
		const app = (status: KYCStatus) => create(KYCApplicationSchema, { status });
		expect(platformReviewApproved(app(KYCStatus.KYC_STATUS_APPROVED))).toBe(
			true,
		);
		for (const status of [
			KYCStatus.KYC_STATUS_DRAFT,
			KYCStatus.KYC_STATUS_SUBMITTED,
			KYCStatus.KYC_STATUS_REJECTED,
		]) {
			expect(platformReviewApproved(app(status)), String(status)).toBe(false);
		}
		expect(platformReviewApproved(null)).toBe(false);
		expect(platformReviewApproved(undefined)).toBe(false);
	});
});
