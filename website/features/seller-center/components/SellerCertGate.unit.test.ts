// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	KYCApplicationSchema,
	KYCStatus,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
	PaymentAccountSchema,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Button", () => ({
	default: ({ children, href }: { children: React.ReactNode; href?: string }) =>
		React.createElement("a", { href }, children),
}));

import { onboardingFixture } from "@/lib/seller/onboarding-fixtures";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import { SellerCertGate } from "./SellerCertGate";

const render = (ui: React.ReactElement) =>
	renderWithoutIntl(ui, { wrapper: IntlTestWrapper });

afterEach(cleanup);

describe("SellerCertGate", () => {
	it("沒有申請：未送出，動作去商家認證頁，內容帶 subject", () => {
		render(React.createElement(SellerCertGate, { subject: "設定物流與退貨" }));
		expect(screen.getByText("尚未通過商家認證")).toBeTruthy();
		expect(screen.getByText(/設定物流與退貨/)).toBeTruthy();
		expect(screen.getByText("目前狀態：未送出")).toBeTruthy();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("平台審核中：狀態是目錄的徽章文字", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_SUBMITTED,
				}),
			}),
		);
		expect(screen.getByText("目前狀態：審核中")).toBeTruthy();
	});

	const approved = create(KYCApplicationSchema, {
		status: KYCStatus.KYC_STATUS_APPROVED,
	});
	const onboardingWith = (action: OnboardingAction) =>
		onboardingFixture([
			{
				id: OnboardingTaskId.PAYMENT_ACCOUNT,
				state: OnboardingTaskState.ACTION_REQUIRED,
				action,
				status: "伺服器的收款句",
			},
		]);

	it("平台審核核准、收款帳戶待設定：徽章讀帳戶狀態、說明句讀任務，動作固定去商家認證頁", () => {
		render(
			React.createElement(SellerCertGate, {
				application: approved,
				platformReviewCompleted: true,
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.ACTION_REQUIRED,
				}),
				onboarding: onboardingWith(OnboardingAction.START_PAYMENT_ONBOARDING),
			}),
		);
		expect(screen.getByText("目前狀態：待設定")).toBeTruthy();
		expect(screen.getByText("伺服器的收款句")).toBeTruthy();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
	});

	it("動作不讀 action 也不讀狀態：任務是 CONTACT_SUPPORT、帳戶無法啟用，連結仍是商家認證頁", () => {
		render(
			React.createElement(SellerCertGate, {
				application: approved,
				platformReviewCompleted: true,
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.DISABLED,
				}),
				onboarding: onboardingWith(OnboardingAction.CONTACT_SUPPORT),
			}),
		);
		expect(screen.queryByText("聯絡客服")).toBeNull();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("收款帳戶準備中：不顯示「重新整理」，動作去商家認證頁", () => {
		render(
			React.createElement(SellerCertGate, {
				application: approved,
				platformReviewCompleted: true,
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.NOT_CREATED,
				}),
				onboarding: onboardingWith(OnboardingAction.NONE),
			}),
		);
		expect(screen.queryByText("重新整理")).toBeNull();
		expect(
			screen.getByText("前往商家認證").closest("a")?.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("申請是 APPROVED 但開通摘要的平台審核還沒完成：走核准前的分支，名詞仍來自申請", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_APPROVED,
				}),
				platformReviewCompleted: false,
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.ACTION_REQUIRED,
				}),
			}),
		);
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
		expect(screen.getByText("前往商家認證")).toBeTruthy();
	});

	it("開通摘要的平台審核已完成但申請不是 APPROVED：走收款帳戶分支", () => {
		render(
			React.createElement(SellerCertGate, {
				application: create(KYCApplicationSchema, {
					status: KYCStatus.KYC_STATUS_SUBMITTED,
				}),
				platformReviewCompleted: true,
				paymentAccount: create(PaymentAccountSchema, {
					status: PaymentAccountStatus.ACTION_REQUIRED,
				}),
			}),
		);
		expect(screen.getByText("前往商家認證")).toBeTruthy();
	});
});
