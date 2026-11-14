import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	OnboardingTaskState as S,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it } from "vitest";
import { publicListingStatus } from "./capability-fixtures";
import { requiredFixture } from "./onboarding-fixtures";
import { walletShowsTransactions } from "./wallet-transactions";

const store = (missing: SellerRequirement[] | undefined) =>
	create(GetMyStoreResponseSchema, {
		store: { id: "1" },
		capabilities: missing === undefined ? [] : [publicListingStatus(missing)],
	});

const none = { state: S.ACTION_REQUIRED } as const;
const done = { state: S.COMPLETED } as const;
const summary = (payment: { state: S }) => requiredFixture(done, done, payment);

describe("walletShowsTransactions", () => {
	it("已認證而且摘要的收款帳戶任務是 COMPLETED", () => {
		expect(walletShowsTransactions(store([]), summary(done))).toBe(true);
	});

	it("還缺 ONBOARDING_COMPLETE、沒有 PUBLIC_LISTING、沒有店家、沒有摘要：都不顯示", () => {
		expect(
			walletShowsTransactions(
				store([SellerRequirement.ONBOARDING_COMPLETE]),
				summary(done),
			),
		).toBe(false);
		expect(walletShowsTransactions(store(undefined), summary(done))).toBe(
			false,
		);
		expect(walletShowsTransactions(null, summary(done))).toBe(false);
		expect(walletShowsTransactions(store([]), null)).toBe(false);
	});

	it("收款帳戶任務不是 COMPLETED（含 UNKNOWN、UNSPECIFIED）：不顯示", () => {
		for (const state of [
			S.ACTION_REQUIRED,
			S.IN_PROGRESS,
			S.LOCKED,
			S.UNKNOWN,
			S.UNSPECIFIED,
		]) {
			expect(
				walletShowsTransactions(store([]), summary({ state })),
				String(state),
			).toBe(false);
		}
		expect(walletShowsTransactions(store([]), summary(none))).toBe(false);
	});
});
