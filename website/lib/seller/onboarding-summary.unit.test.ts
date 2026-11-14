import {
	OnboardingTaskId,
	OnboardingTaskState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { describe, expect, it, vi } from "vitest";
import { requiredFixture } from "./onboarding-fixtures";
import {
	fetchMyOnboarding,
	onboardingTaskState,
	paymentAccountCompleted,
	platformReviewCompleted,
	refreshMyPaymentAccount,
} from "./onboarding-summary";

const S = OnboardingTaskState;
const none = { state: S.ACTION_REQUIRED } as const;

describe("platformReviewCompleted", () => {
	it("只有 PLATFORM_REVIEW 這一項是 COMPLETED 才是完成；其他項目與其他狀態都不是", () => {
		const done = { state: S.COMPLETED } as const;
		expect(platformReviewCompleted(requiredFixture(none, done, none))).toBe(
			true,
		);
		expect(platformReviewCompleted(requiredFixture(done, none, done))).toBe(
			false,
		);
		for (const state of [
			S.ACTION_REQUIRED,
			S.IN_PROGRESS,
			S.LOCKED,
			S.UNKNOWN,
			S.UNSPECIFIED,
		]) {
			expect(
				platformReviewCompleted(requiredFixture(none, { state }, none)),
				String(state),
			).toBe(false);
		}
	});

	it("沒有摘要或摘要沒有這一項：不是完成", () => {
		expect(platformReviewCompleted(null)).toBe(false);
		expect(platformReviewCompleted(undefined)).toBe(false);
		expect(
			onboardingTaskState(
				requiredFixture(none, none, none),
				OnboardingTaskId.UNSPECIFIED,
			),
		).toBeUndefined();
	});
});

describe("paymentAccountCompleted", () => {
	it("只有 PAYMENT_ACCOUNT 這一項是 COMPLETED 才是完成", () => {
		const done = { state: S.COMPLETED } as const;
		expect(paymentAccountCompleted(requiredFixture(none, none, done))).toBe(
			true,
		);
		expect(paymentAccountCompleted(requiredFixture(done, done, none))).toBe(
			false,
		);
		for (const state of [
			S.ACTION_REQUIRED,
			S.IN_PROGRESS,
			S.LOCKED,
			S.UNKNOWN,
			S.UNSPECIFIED,
		]) {
			expect(
				paymentAccountCompleted(requiredFixture(none, none, { state })),
				String(state),
			).toBe(false);
		}
	});

	it("沒有摘要：不是完成", () => {
		expect(paymentAccountCompleted(null)).toBe(false);
		expect(paymentAccountCompleted(undefined)).toBe(false);
	});
});

describe("RPC 包裝", () => {
	it("fetchMyOnboarding 與 refreshMyPaymentAccount 各呼叫一次對應的方法", async () => {
		const getMyOnboarding = vi.fn(async () =>
			requiredFixture(none, none, none),
		);
		const refreshMyPaymentAccountRpc = vi.fn(async () => ({}));
		const client = () =>
			({
				getMyOnboarding,
				refreshMyPaymentAccount: refreshMyPaymentAccountRpc,
			}) as never;
		await fetchMyOnboarding(client);
		await refreshMyPaymentAccount(client);
		expect(getMyOnboarding).toHaveBeenCalledTimes(1);
		expect(refreshMyPaymentAccountRpc).toHaveBeenCalledTimes(1);
	});
});
