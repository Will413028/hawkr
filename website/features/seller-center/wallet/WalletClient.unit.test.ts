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
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	cleanup,
	render as renderWithoutIntl,
	screen,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const state = vi.hoisted(() => ({
	cert: {} as Record<string, unknown>,
}));

vi.mock("@/lib/seller/cert-gate", () => ({
	CERT_PAGE_HREF: "/seller-center/me/certifications",
	useCertGate: () => state.cert,
}));
vi.mock("@/lib/hooks/useQueryIdentity", () => ({
	useQueryIdentity: () => ({ accountId: "1", activeRole: "seller" }),
}));
vi.mock("@/lib/orders/repo", () => ({
	fetchOrders: async () => ({ records: [] }),
}));
vi.mock("@/features/seller-center", () => ({
	SellerCertGate: () =>
		React.createElement("div", { "data-testid": "cert-gate" }),
}));
vi.mock(
	"@/features/seller-center/components/SellerMobileDisclosureList",
	() => ({
		SellerMobileDisclosureIcon: () => null,
		SellerMobileDisclosureList: () => null,
	}),
);
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/shared/Link", () => ({
	default: ({ children, href }: { children: React.ReactNode; href: string }) =>
		React.createElement("a", { href }, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		children,
		href,
	}: {
		children: React.ReactNode;
		href?: string;
	}) =>
		href
			? React.createElement("a", { href }, children)
			: React.createElement("button", { type: "button" }, children),
}));

import { uiTextFor } from "@/lib/i18n/ui-text";
import { onboardingFixture } from "@/lib/seller/onboarding-fixtures";
import { paymentAccountCopy } from "@/lib/seller/payment-account";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import WalletClient from "./WalletClient";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

function mount() {
	return render(
		React.createElement(
			QueryClientProvider,
			{ client: new QueryClient() },
			React.createElement(WalletClient),
		),
	);
}

function setCert(
	kycStatus: KYCStatus,
	paymentStatus?: PaymentAccountStatus,
	payoutsActive = false,
	paymentAccountCompleted = paymentStatus === PaymentAccountStatus.ENABLED,
	taskAction = OnboardingAction.NONE,
) {
	state.cert = {
		loading: false,
		application: create(KYCApplicationSchema, { status: kycStatus }),
		paymentAccount:
			paymentStatus === undefined
				? null
				: create(PaymentAccountSchema, {
						status: paymentStatus,
						payoutsActive,
					}),
		seller: { capabilities: [{ capability: 2, active: true, missing: [] }] },
		approved: kycStatus === KYCStatus.KYC_STATUS_APPROVED,
		platformReviewCompleted: kycStatus === KYCStatus.KYC_STATUS_APPROVED,
		paymentAccountCompleted,
		onboarding: onboardingFixture([
			{
				id: OnboardingTaskId.PAYMENT_ACCOUNT,
				state: paymentAccountCompleted
					? OnboardingTaskState.COMPLETED
					: OnboardingTaskState.IN_PROGRESS,
				action: taskAction,
				status: "伺服器的收款句",
			},
		]),
		refresh: () => {},
	};
}

afterEach(() => {
	cleanup();
});

describe("WalletClient 的收款帳戶狀態", () => {
	it("已啟用：沒有面板、閘門，也沒有刷新按鈕", () => {
		setCert(KYCStatus.KYC_STATUS_APPROVED, PaymentAccountStatus.ENABLED, true);
		mount();
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		expect(screen.queryByText("待設定")).toBeNull();
		expect(screen.queryByRole("button", { name: /重新整理|同步/ })).toBeNull();
	});

	it("待設定：徽章讀帳戶狀態，說明句讀任務，只有一個固定的「前往商家認證」連結", () => {
		setCert(
			KYCStatus.KYC_STATUS_APPROVED,
			PaymentAccountStatus.ACTION_REQUIRED,
			false,
			false,
			OnboardingAction.START_PAYMENT_ONBOARDING,
		);
		mount();
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		expect(screen.getByText("待設定")).toBeTruthy();
		expect(screen.getByText("伺服器的收款句")).toBeTruthy();
		expect(screen.getAllByRole("link")).toHaveLength(1);
		expect(
			screen.getByRole("link", { name: "前往商家認證" }).getAttribute("href"),
		).toBe("/seller-center/me/certifications");
		expect(screen.queryByText("前往設定收款帳戶")).toBeNull();
		expect(document.body.textContent).not.toMatch(/逾期|停用|PayUni/);
	});

	it("審核中：說明句是任務的，沒有「重新整理」", () => {
		setCert(KYCStatus.KYC_STATUS_APPROVED, PaymentAccountStatus.UNDER_REVIEW);
		mount();
		expect(screen.getByText("伺服器的收款句")).toBeTruthy();
		expect(screen.queryByText("重新整理")).toBeNull();
	});

	it("無法啟用：連結仍然只去商家認證頁，不依帳戶狀態或任務動作換成客服", () => {
		setCert(
			KYCStatus.KYC_STATUS_APPROVED,
			PaymentAccountStatus.DISABLED,
			false,
			false,
			OnboardingAction.CONTACT_SUPPORT,
		);
		mount();
		expect(screen.getByText("無法啟用")).toBeTruthy();
		expect(screen.queryByText("聯絡客服")).toBeNull();
		expect(
			screen.getByRole("link", { name: "前往商家認證" }).getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("平台審核還沒核准：顯示認證閘，沒有「帳戶與認證」列", () => {
		setCert(KYCStatus.KYC_STATUS_SUBMITTED);
		mount();
		expect(screen.getByTestId("cert-gate")).toBeTruthy();
		expect(screen.queryByText("帳戶與認證")).toBeNull();
	});

	it("平台審核與收款帳戶都完成後顯示「商家認證」列", () => {
		setCert(KYCStatus.KYC_STATUS_APPROVED, PaymentAccountStatus.ENABLED, true);
		mount();
		expect(screen.getByText("商家認證")).toBeTruthy();
		expect(screen.getByText("平台審核與收款帳戶")).toBeTruthy();
	});

	it("摘要說收款帳戶任務未完成：帳戶 status 是 ENABLED 也顯示面板", () => {
		setCert(
			KYCStatus.KYC_STATUS_APPROVED,
			PaymentAccountStatus.ENABLED,
			true,
			false,
		);
		mount();
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		const badge = paymentAccountCopy(
			uiTextFor("zh-TW"),
			create(PaymentAccountSchema, { status: PaymentAccountStatus.ENABLED }),
		).badge;
		expect(badge).toBeTruthy();
		expect(screen.getByText(badge as string)).toBeTruthy();
	});

	it("摘要說收款帳戶任務完成：帳戶 status 是 UNDER_REVIEW 也不顯示面板與閘門", () => {
		setCert(
			KYCStatus.KYC_STATUS_APPROVED,
			PaymentAccountStatus.UNDER_REVIEW,
			false,
			true,
		);
		mount();
		expect(screen.queryByTestId("cert-gate")).toBeNull();
		expect(
			screen.queryByText("金流商正在審核你的資料，不需要額外操作。"),
		).toBeNull();
	});
});
