// @vitest-environment jsdom

import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { LocalizedMessageSchema } from "@hawkr/api/gen/google/rpc/error_details_pb";
import {
	CapabilityStatusSchema,
	type GetMyOnboardingResponse,
	GetMyStoreResponseSchema,
	OnboardingAction,
	OnboardingTaskId,
	OnboardingTaskState,
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
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { publicListingStatus } from "@/lib/seller/capability-fixtures";
import {
	onboardingFixture,
	requiredFixture,
} from "@/lib/seller/onboarding-fixtures";

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
		onClick,
	}: {
		href?: string;
		children: React.ReactNode;
		disabled?: boolean;
		onClick?: () => void;
	}) =>
		href !== undefined
			? React.createElement(
					"a",
					{ href, "aria-disabled": disabled ? "true" : "false" },
					children,
				)
			: React.createElement(
					"button",
					{ type: "button", disabled, onClick },
					children,
				),
}));
const refreshPaymentAccount = vi.fn(async () => {});
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useRefreshPaymentAccount: () => refreshPaymentAccount,
}));

import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import OnboardingProgressCard from "./OnboardingProgressCard";

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

const R = SellerRequirement;
const S = OnboardingTaskState;
const A = OnboardingAction;
const ID = OnboardingTaskId;

const PENDING = { state: S.ACTION_REQUIRED, action: A.NONE } as const;

function renderCard({
	onboarding,
	loading = false,
	missing = [],
}: {
	onboarding: GetMyOnboardingResponse | null;
	loading?: boolean;
	missing?: SellerRequirement[];
}) {
	const my = create(GetMyStoreResponseSchema, {
		store: { id: "1", description: "介紹" },
		capabilities: [
			publicListingStatus(missing),
			create(CapabilityStatusSchema, {
				capability: SellerCapability.SHIPPING_SETUP,
				active: true,
			}),
		],
	});
	render(
		React.createElement(OnboardingProgressCard, {
			self: my,
			onboarding,
			onboardingLoading: loading,
			shippingRows: [],
			productCount: 0,
		}),
	);
}

function expand() {
	const toggle = screen.queryByRole("button", { name: /展開任務/ });
	if (toggle) fireEvent.click(toggle);
}

beforeEach(() => {
	refreshPaymentAccount.mockClear();
});

afterEach(() => {
	cleanup();
});

describe("OnboardingProgressCard", () => {
	it("splits the steps into 公開前必須 and 建議完成", () => {
		renderCard({ onboarding: requiredFixture(PENDING, PENDING, PENDING) });
		expand();

		const lists = screen.getAllByRole("list");
		expect(lists).toHaveLength(2);
		const required = within(lists[0] as HTMLElement);
		const recommended = within(lists[1] as HTMLElement);
		for (const id of [ID.PROFILE, ID.PLATFORM_REVIEW, ID.PAYMENT_ACCOUNT]) {
			expect(
				required.getByRole("button", {
					name: new RegExp(`^伺服器標題${id}，`),
				}),
			).toBeTruthy();
		}
		for (const title of ["物流設定", "上架商品", "商店介紹"]) {
			expect(
				recommended.getByRole("button", { name: new RegExp(`^${title}，`) }),
			).toBeTruthy();
		}
		expect(screen.getByText("公開前必須")).toBeTruthy();
		expect(screen.getByText("建議完成")).toBeTruthy();
	});

	it("progress counts the server's COMPLETED tasks in the required group only", () => {
		renderCard({
			onboarding: requiredFixture(
				{ state: S.COMPLETED, action: A.NONE },
				PENDING,
				{ state: S.LOCKED, action: A.NONE },
			),
		});
		expect(screen.getByText("1/3")).toBeTruthy();
	});

	it("follows the summary, not the store's capabilities", () => {
		// 能力說沒有缺任何條件，摘要說三項都沒完成：不顯示「任務完成」。
		renderCard({
			onboarding: requiredFixture(PENDING, PENDING, PENDING),
			missing: [],
		});
		expect(screen.queryByText("任務完成")).toBeNull();
		expect(screen.getByText("0/3")).toBeTruthy();
	});

	it("is collapsed with the completion mark once the server says every task is COMPLETED", () => {
		const done = { state: S.COMPLETED, action: A.NONE } as const;
		renderCard({
			onboarding: requiredFixture(done, done, done, true),
			missing: [R.ONBOARDING_COMPLETE],
		});
		expect(screen.getByText("任務完成")).toBeTruthy();
		expect(screen.queryByRole("list")).toBeNull();
	});

	it("shows the server status sentence and the catalog button for the selected task", () => {
		renderCard({
			onboarding: onboardingFixture([
				{ id: ID.PROFILE, state: S.COMPLETED },
				{
					id: ID.PLATFORM_REVIEW,
					state: S.ACTION_REQUIRED,
					action: A.SUBMIT_CERTIFICATION,
					status: "伺服器說尚未送出",
				},
				{ id: ID.PAYMENT_ACCOUNT, state: S.LOCKED },
			]),
		});
		expand();
		fireEvent.click(
			screen.getByRole("button", {
				name: `伺服器標題${ID.PLATFORM_REVIEW}，伺服器說尚未送出`,
			}),
		);
		expect(screen.getAllByText("伺服器說尚未送出").length).toBeGreaterThan(0);
		expect(
			screen.getByRole("link", { name: "前往商家認證" }).getAttribute("href"),
		).toBe("/seller-center/me/certifications");
	});

	it("a task with action NONE has no button", () => {
		renderCard({
			onboarding: onboardingFixture([
				{ id: ID.PLATFORM_REVIEW, state: S.IN_PROGRESS, action: A.NONE },
			]),
		});
		expand();
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題2，/ }));
		const detail = document.querySelector(
			".seller-onboarding-progress__detail",
		) as HTMLElement;
		expect(within(detail).queryByRole("link")).toBeNull();
		expect(within(detail).queryByRole("button")).toBeNull();
	});

	it("REFRESH_PAYMENT_ACCOUNT is an in-place button that calls the refresh once", async () => {
		renderCard({
			onboarding: onboardingFixture([
				{ id: ID.PROFILE, state: S.COMPLETED },
				{ id: ID.PLATFORM_REVIEW, state: S.COMPLETED },
				{
					id: ID.PAYMENT_ACCOUNT,
					state: S.IN_PROGRESS,
					action: A.REFRESH_PAYMENT_ACCOUNT,
				},
			]),
		});
		expand();
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題3，/ }));
		const button = screen.getByRole("button", { name: "重新整理" });
		expect(button.closest("a")).toBeNull();
		fireEvent.click(button);
		expect(refreshPaymentAccount).toHaveBeenCalledTimes(1);
	});

	it("a failed refresh shows the default error text", async () => {
		refreshPaymentAccount.mockRejectedValueOnce(new Error("boom"));
		renderCard({
			onboarding: onboardingFixture([
				{
					id: ID.PAYMENT_ACCOUNT,
					state: S.IN_PROGRESS,
					action: A.REFRESH_PAYMENT_ACCOUNT,
				},
			]),
		});
		expand();
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題3，/ }));
		fireEvent.click(screen.getByRole("button", { name: "重新整理" }));
		expect((await screen.findByRole("alert")).textContent).toBe("系統錯誤");
	});

	it("a failed refresh shows the server's sentence", async () => {
		const err = new ConnectError("blocked", Code.FailedPrecondition);
		err.details.push({
			desc: LocalizedMessageSchema,
			value: create(LocalizedMessageSchema, {
				locale: "zh-TW",
				message: "商店已暫停營業",
			}),
		});
		refreshPaymentAccount.mockRejectedValueOnce(err);
		renderCard({
			onboarding: onboardingFixture([
				{
					id: ID.PAYMENT_ACCOUNT,
					state: S.IN_PROGRESS,
					action: A.REFRESH_PAYMENT_ACCOUNT,
				},
			]),
		});
		expand();
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題3，/ }));
		fireEvent.click(screen.getByRole("button", { name: "重新整理" }));
		expect((await screen.findByRole("alert")).textContent).toBe(
			"商店已暫停營業",
		);
	});

	it("a refresh error is cleared when the active task changes", async () => {
		refreshPaymentAccount.mockRejectedValueOnce(new Error("boom"));
		renderCard({
			onboarding: onboardingFixture([
				{ id: ID.PROFILE, state: S.COMPLETED },
				{
					id: ID.PAYMENT_ACCOUNT,
					state: S.IN_PROGRESS,
					action: A.REFRESH_PAYMENT_ACCOUNT,
				},
			]),
		});
		expand();
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題3，/ }));
		fireEvent.click(screen.getByRole("button", { name: "重新整理" }));
		await screen.findByRole("alert");
		fireEvent.click(screen.getByRole("button", { name: /^伺服器標題1，/ }));
		expect(screen.queryByRole("alert")).toBeNull();
	});

	it("GetMyOnboarding failed: the required group is one load-failure state, recommended stay", () => {
		renderCard({ onboarding: null });
		expand();
		const alert = screen.getByRole("alert");
		expect(alert.textContent).toBe("系統錯誤");
		expect(screen.getByText("公開前必須")).toBeTruthy();
		// 沒有必要任務可數：不顯示進度，也不是「任務完成」。
		expect(screen.queryByText(/\/\d/)).toBeNull();
		expect(screen.queryByText("任務完成")).toBeNull();
		const lists = screen.getAllByRole("list");
		expect(lists).toHaveLength(1);
		for (const title of ["物流設定", "上架商品", "商店介紹"]) {
			expect(
				within(lists[0] as HTMLElement).getByRole("button", {
					name: new RegExp(`^${title}，`),
				}),
			).toBeTruthy();
		}
	});

	it("while GetMyOnboarding loads the required group shows the loading text, not an error", () => {
		renderCard({ onboarding: null, loading: true });
		expand();
		expect(screen.getByText("載入中…")).toBeTruthy();
		expect(screen.queryByRole("alert")).toBeNull();
	});
});
