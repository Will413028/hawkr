import type { OnboardingTask } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { OnboardingTaskState } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import type { MaterialSymbolName } from "@hawkr/icons/material-symbol";
import type { BadgeVariant } from "@/components/UI";

// 商家詳情開通摘要卡的純邏輯（讀取狀態、狀態的圖示與色調、總徽章）；元件在 OnboardingSummaryCard.tsx，查詢 hook 在 use-store-detail.ts。

export type OnboardingSummaryState =
	| { state: "loading" }
	| { state: "error"; error?: unknown }
	| { state: "ready"; tasks: OnboardingTask[]; complete: boolean };

/** 讀取狀態的規則同 paymentAccountOf：有錯誤就是錯誤（not_found 也是），成功卻沒有任務是讀取錯誤。 */
export function onboardingSummaryOf(result: {
	error: unknown;
	data: { tasks: OnboardingTask[]; complete: boolean } | undefined;
}): OnboardingSummaryState {
	if (result.error != null) return { state: "error", error: result.error };
	if (result.data === undefined) return { state: "loading" };
	if (result.data.tasks.length === 0) return { state: "error" };
	return {
		state: "ready",
		tasks: result.data.tasks,
		complete: result.data.complete,
	};
}

export type OnboardingTaskTone =
	| "success"
	| "warning"
	| "info"
	| "muted"
	| "unknown";

export type OnboardingTaskPresentation = {
	icon: MaterialSymbolName;
	tone: OnboardingTaskTone;
};

const UNKNOWN_PRESENTATION: OnboardingTaskPresentation = {
	icon: "help",
	tone: "unknown",
};

const PRESENTATION: Readonly<
	Partial<Record<OnboardingTaskState, OnboardingTaskPresentation>>
> = {
	[OnboardingTaskState.COMPLETED]: { icon: "check_circle", tone: "success" },
	[OnboardingTaskState.ACTION_REQUIRED]: { icon: "error", tone: "warning" },
	[OnboardingTaskState.IN_PROGRESS]: { icon: "schedule", tone: "info" },
	[OnboardingTaskState.LOCKED]: { icon: "lock", tone: "muted" },
	[OnboardingTaskState.UNKNOWN]: UNKNOWN_PRESENTATION,
};

/** 任務狀態的圖示與色調；UNSPECIFIED 與表裡沒有的值一律是「未知」的呈現，不會借用已完成的。 */
export function onboardingTaskPresentation(
	state: OnboardingTaskState,
): OnboardingTaskPresentation {
	return PRESENTATION[state] ?? UNKNOWN_PRESENTATION;
}

/** 總徽章：complete 是已開通；未完成且每項都認得是未開通；有任何一項狀態不明就不給徽章（來源讀不到時不能宣稱未開通）。 */
export function onboardingBadge(
	tasks: readonly OnboardingTask[],
	complete: boolean,
): { label: string; variant: BadgeVariant } | undefined {
	if (complete) return { label: "已開通", variant: "success" };
	const anyUnknown = tasks.some(
		(task) => onboardingTaskPresentation(task.state).tone === "unknown",
	);
	return anyUnknown ? undefined : { label: "未開通", variant: "gray" };
}
