// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import {
	CapabilityStatusSchema,
	GetMyStoreResponseSchema,
	KYCStatus,
	PaymentAccountSchema,
	PaymentAccountStatus,
	SellerCapability,
	SellerRequirement,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	within,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { publicListingStatus } from "@/lib/seller/capability-fixtures";

// 真的 store 以 localStorage 持久化；這裡只要它的狀態與動作，用不持久化的同形 store 取代。
vi.mock("./dashboardUiStore", async () => {
	const { create } = await import("zustand");
	return {
		useSellerDashboardUiStore: create(
			(
				set: (
					fn: (state: {
						onboardingCollapsedOverride: boolean | null;
					}) => object,
				) => void,
			) => ({
				onboardingActiveTaskId: null as string | null,
				onboardingCollapsedOverride: null as boolean | null,
				setOnboardingActiveTaskId: (onboardingActiveTaskId: string | null) =>
					set(() => ({ onboardingActiveTaskId })),
				toggleOnboardingCollapsed: (defaultCollapsed: boolean) =>
					set((state) => ({
						onboardingCollapsedOverride: !(
							state.onboardingCollapsedOverride ?? defaultCollapsed
						),
					})),
			}),
		),
	};
});
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/components/ui/Button", () => ({
	default: ({
		href,
		children,
		disabled,
	}: {
		href?: string;
		children: React.ReactNode;
		disabled?: boolean;
	}) =>
		React.createElement(
			"a",
			{ href, "aria-disabled": disabled ? "true" : "false" },
			children,
		),
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import OnboardingProgressCard from "./OnboardingProgressCard";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

const R = SellerRequirement;

function renderCard(
	missing: SellerRequirement[],
	kycStatus?: KYCStatus,
	paymentStatus?: PaymentAccountStatus,
) {
	const my = create(GetMyStoreResponseSchema, {
		store: { id: "1", description: "介紹" },
		capabilities: [
			publicListingStatus(missing),
			// 照伺服器：物流建店缺認證時才缺 ONBOARDING_COMPLETE。
			create(CapabilityStatusSchema, {
				capability: SellerCapability.SHIPPING_SETUP,
				active: !missing.includes(R.ONBOARDING_COMPLETE),
				missing: missing.includes(R.ONBOARDING_COMPLETE)
					? [R.ONBOARDING_COMPLETE]
					: [],
			}),
		],
	});
	render(
		React.createElement(OnboardingProgressCard, {
			self: my,
			kycStatus,
			paymentAccount:
				paymentStatus === undefined
					? null
					: create(PaymentAccountSchema, { status: paymentStatus }),
			shippingRows: [],
			productCount: 0,
		}),
	);
}

function expand() {
	const toggle = screen.queryByRole("button", { name: /展開任務/ });
	if (toggle) fireEvent.click(toggle);
}

afterEach(() => {
	cleanup();
});

describe("OnboardingProgressCard", () => {
	it("splits the steps into 公開前必須 and 建議完成", () => {
		renderCard([R.ONBOARDING_COMPLETE]);
		expand();

		const lists = screen.getAllByRole("list");
		expect(lists).toHaveLength(2);
		const required = within(lists[0] as HTMLElement);
		const recommended = within(lists[1] as HTMLElement);
		expect(required.getByRole("button", { name: /^商店資料，/ })).toBeTruthy();
		expect(required.getByRole("button", { name: /^平台審核，/ })).toBeTruthy();
		expect(required.getByRole("button", { name: /^收款帳戶，/ })).toBeTruthy();
		for (const title of ["物流設定", "上架商品", "商店介紹"]) {
			expect(
				recommended.getByRole("button", { name: new RegExp(`^${title}，`) }),
			).toBeTruthy();
		}
		expect(screen.getByText("公開前必須")).toBeTruthy();
		expect(screen.getByText("建議完成")).toBeTruthy();
	});

	it("progress only counts the required group", () => {
		renderCard([R.ONBOARDING_COMPLETE]);
		// 三個必須項目完成一個；建議完成的項目不進分母。
		expect(screen.getByText("1/3")).toBeTruthy();
	});

	it("is collapsed with the completion mark once nothing is missing", () => {
		renderCard([]);
		expect(screen.getByText("任務完成")).toBeTruthy();
		expect(screen.queryByRole("list")).toBeNull();
	});

	it("selecting 收款帳戶 after approval shows its catalog status and action", () => {
		renderCard(
			[R.ONBOARDING_COMPLETE],
			KYCStatus.KYC_STATUS_APPROVED,
			PaymentAccountStatus.ACTION_REQUIRED,
		);
		expand();

		fireEvent.click(screen.getByRole("button", { name: /^收款帳戶，/ }));
		expect(screen.getAllByText("待設定").length).toBeGreaterThan(0);
		expect(
			screen
				.getByRole("link", { name: "前往設定收款帳戶" })
				.getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("selecting 平台審核 before submit offers the certification page", () => {
		renderCard([R.ONBOARDING_COMPLETE]);
		expand();

		fireEvent.click(screen.getByRole("button", { name: /^平台審核，/ }));
		expect(
			screen.getByRole("link", { name: "前往商家認證" }).getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("names what the profile step still lacks", () => {
		renderCard([R.LOGO, R.TOWN_CODE]);
		expand();
		expect(
			screen.getByRole("button", { name: /^商店資料，缺少：logo、所在地/ }),
		).toBeTruthy();
	});
});
