"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { Button } from "@/components/UI";
import { errorMessage } from "@/lib/error-text";
import { unknownText } from "@/lib/ui-text";
import { DetailCardShell } from "./DetailCardShell";
import {
	type OnboardingSummaryState,
	onboardingBadge,
	onboardingTaskPresentation,
} from "./onboarding-summary";

// 商家詳情的開通摘要卡（SellerAdminService.GetStoreOnboarding）：與賣家看到的同一份任務，標題與狀態句由伺服器產生、逐字顯示。
// 狀態只決定圖示與色調，client 不依 id、state、action 造文字；action 是賣家的下一步，後台不代辦所以不顯示。

const SHELL = {
	titleId: "onboarding-summary-title",
	icon: "checklist",
	title: "開通進度",
	className: "onboarding-summary-card",
} as const;

/** 沒有伺服器文字時顯示目錄的「未知」，不另造句子。 */
function serverText(message: string | undefined): string {
	return message ? message : unknownText();
}

/** 開通摘要卡（純呈現）：資料由頁面的 useStoreOnboarding 接上。 */
export function OnboardingSummaryCard({
	state,
	onReload,
}: {
	state: OnboardingSummaryState;
	onReload: () => void;
}) {
	if (state.state === "loading") {
		return (
			<DetailCardShell {...SHELL}>
				<p className="payment-desc" role="status">
					開通進度載入中…
				</p>
			</DetailCardShell>
		);
	}
	if (state.state === "error") {
		return (
			<DetailCardShell {...SHELL}>
				<p className="payment-desc" role="alert">
					開通進度讀取失敗
					{state.error !== undefined && `：${errorMessage(state.error)}`}
				</p>
				<div>
					<Button variant="outline" onClick={onReload}>
						重試
					</Button>
				</div>
			</DetailCardShell>
		);
	}
	return (
		<DetailCardShell
			{...SHELL}
			badge={onboardingBadge(state.tasks, state.complete)}
		>
			<p className="payment-desc">以下是賣家看到的開通任務與狀態文字。</p>
			<ol className="onboarding-task-list">
				{state.tasks.map((task) => {
					const { icon, tone } = onboardingTaskPresentation(task.state);
					return (
						<li
							key={task.id}
							className={`onboarding-task onboarding-task--${tone}`}
						>
							<MaterialSymbol name={icon} className="onboarding-task-icon" />
							<div>
								<div className="onboarding-task-title">
									{serverText(task.title?.message)}
								</div>
								<div className="onboarding-task-status">
									{serverText(task.status?.message)}
								</div>
							</div>
						</li>
					);
				})}
			</ol>
		</DetailCardShell>
	);
}
