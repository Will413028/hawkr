"use client";

import { createLocalPrivateStore } from "@/lib/state/createLocalPrivateStore";

export type SellerOnboardingTaskId =
	| "profile"
	| "cert"
	| "other_requirement"
	| "shipping"
	| "product"
	| "food"
	| "description";

type SellerDashboardUiPersistedState = {
	onboardingActiveTaskId: SellerOnboardingTaskId | null;
	onboardingCollapsedOverride: boolean | null;
};

// `resetOnboardingUi` 曾定義在此但全專案零呼叫端（死碼），且它想做的事——身份
// 轉換時把 onboarding UI 還原成預設——現在由 createLocalPrivateStore 註冊的
// reset 統一負責，留著只會讓人以為清除邏輯有兩個入口。
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
