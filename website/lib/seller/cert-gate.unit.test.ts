// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import {
	GetMyStoreResponseSchema,
	KYCApplicationSchema,
	KYCStatus,
	OnboardingTaskId,
	OnboardingTaskState,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { publicListingStatus } from "./capability-fixtures";
import { onboardingFixture } from "./onboarding-fixtures";

const mocks = vi.hoisted(() => ({
	seller: null as unknown,
	application: null as unknown,
	onboarding: null as unknown,
	onboardingLoading: false,
	paymentEnabled: [] as boolean[],
}));

vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => ({
		data: mocks.seller,
		loading: false,
		error: null,
		refresh: async () => {},
	}),
	useMyKyc: () => ({
		application: mocks.application,
		loading: false,
		error: null,
	}),
	useMyOnboarding: () => ({
		onboarding: mocks.onboarding,
		loading: mocks.onboardingLoading,
		error: null,
		refetch: async () => {},
	}),
	useMyPaymentAccount: (options?: { enabled?: boolean }) => {
		mocks.paymentEnabled.push(options?.enabled ?? true);
		return {
			account: null,
			loading: false,
			error: null,
			refetch: async () => {},
		};
	},
}));

import { useCertGate } from "./cert-gate";

const review = (state: OnboardingTaskState) =>
	onboardingFixture([{ id: OnboardingTaskId.PLATFORM_REVIEW, state }]);
const application = (status: KYCStatus) =>
	create(KYCApplicationSchema, { status });
const lastEnabled = () => mocks.paymentEnabled[mocks.paymentEnabled.length - 1];

beforeEach(() => {
	mocks.seller = create(GetMyStoreResponseSchema, {
		store: { id: "1" },
		capabilities: [
			publicListingStatus([SellerRequirement.ONBOARDING_COMPLETE]),
		],
	});
	mocks.application = null;
	mocks.onboarding = null;
	mocks.onboardingLoading = false;
	mocks.paymentEnabled = [];
});

describe("useCertGate：收款帳戶只在摘要說平台審核完成之後讀", () => {
	it("申請已核准而摘要說還沒完成：不讀", () => {
		mocks.application = application(KYCStatus.KYC_STATUS_APPROVED);
		mocks.onboarding = review(OnboardingTaskState.ACTION_REQUIRED);
		renderHook(() => useCertGate());
		expect(lastEnabled()).toBe(false);
	});

	it("申請還在審核而摘要說已完成：讀", () => {
		mocks.application = application(KYCStatus.KYC_STATUS_SUBMITTED);
		mocks.onboarding = review(OnboardingTaskState.COMPLETED);
		renderHook(() => useCertGate());
		expect(lastEnabled()).toBe(true);
	});

	it("摘要沒讀到或是 UNKNOWN：不當成已完成，不讀", () => {
		mocks.application = application(KYCStatus.KYC_STATUS_APPROVED);
		mocks.onboarding = null;
		renderHook(() => useCertGate());
		expect(lastEnabled()).toBe(false);
		mocks.onboarding = review(OnboardingTaskState.UNKNOWN);
		renderHook(() => useCertGate());
		expect(lastEnabled()).toBe(false);
	});

	it("摘要還在載入：整個閘還在載入", () => {
		mocks.onboardingLoading = true;
		const { result } = renderHook(() => useCertGate());
		expect(result.current.loading).toBe(true);
	});

	it("approved 仍是店家能力的 ONBOARDING_COMPLETE，不看摘要", () => {
		mocks.seller = create(GetMyStoreResponseSchema, {
			store: { id: "1" },
			capabilities: [publicListingStatus([])],
		});
		mocks.onboarding = review(OnboardingTaskState.ACTION_REQUIRED);
		const { result } = renderHook(() => useCertGate());
		expect(result.current.approved).toBe(true);
	});

	it("platformReviewCompleted 是摘要的答案，不看申請狀態", () => {
		mocks.application = application(KYCStatus.KYC_STATUS_APPROVED);
		mocks.onboarding = review(OnboardingTaskState.ACTION_REQUIRED);
		expect(
			renderHook(() => useCertGate()).result.current.platformReviewCompleted,
		).toBe(false);
		mocks.application = application(KYCStatus.KYC_STATUS_SUBMITTED);
		mocks.onboarding = review(OnboardingTaskState.COMPLETED);
		expect(
			renderHook(() => useCertGate()).result.current.platformReviewCompleted,
		).toBe(true);
	});
});
