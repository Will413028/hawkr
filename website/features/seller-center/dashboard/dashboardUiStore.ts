"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export type SellerOnboardingTaskId =
	| "profile"
	| "platform_review"
	| "payment_account"
	| "shipping"
	| "product"
	| "description"
	/** 伺服器新增、這個版本的網頁還沒有專屬呈現的必要任務（`required_<OnboardingTaskId 數值>`）。 */
	| `required_${number}`;

type SellerDashboardUiPersistedState = {
	onboardingActiveTaskId: SellerOnboardingTaskId | null;
	onboardingCollapsedOverride: boolean | null;
};

type SellerDashboardUiState = SellerDashboardUiPersistedState & {
	setOnboardingActiveTaskId: (taskId: SellerOnboardingTaskId | null) => void;
	setOnboardingCollapsedOverride: (collapsed: boolean | null) => void;
	toggleOnboardingCollapsed: (defaultCollapsed: boolean) => void;
};

const defaultState: SellerDashboardUiPersistedState = {
	onboardingActiveTaskId: null,
	onboardingCollapsedOverride: null,
};

export const useSellerDashboardUiStore = createLocalPrivateStore<
	SellerDashboardUiState,
	SellerDashboardUiPersistedState
>(
	(set) => ({
		...defaultState,
		setOnboardingActiveTaskId: (onboardingActiveTaskId) =>
			set(
				{ onboardingActiveTaskId },
				false,
				"seller-dashboard/setOnboardingActiveTaskId",
			),
		setOnboardingCollapsedOverride: (onboardingCollapsedOverride) =>
			set(
				{ onboardingCollapsedOverride },
				false,
				"seller-dashboard/setOnboardingCollapsedOverride",
			),
		toggleOnboardingCollapsed: (defaultCollapsed) =>
			set(
				(state) => ({
					onboardingCollapsedOverride: !(
						state.onboardingCollapsedOverride ?? defaultCollapsed
					),
				}),
				false,
				"seller-dashboard/toggleOnboardingCollapsed",
			),
	}),
	{
		keyName: "sellerDashboardUi",
		devtoolsName: "seller-dashboard-ui-store",
		defaults: defaultState,
		partialize: (state): SellerDashboardUiPersistedState => ({
			onboardingActiveTaskId: state.onboardingActiveTaskId,
			onboardingCollapsedOverride: state.onboardingCollapsedOverride,
		}),
	},
);
