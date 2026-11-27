"use client";

import AppIcon from "@/components/shared/AppIcon";
import {
	type AgentActivityEvent,
	activityLabel,
	activityStatusLabel,
	groupActivityRows,
} from "./activity";
import type { AgentMotionState } from "./agent-motion";
import HawkrAgentSprite from "./HawkrAgentSprite";
import { type AgentStep, stepIcon, stepLabel } from "./steps";

interface AgentActivityRailProps {
	/** 這一輪目前為止收到的所有 public steps，依抵達順序。 */
	steps: AgentStep[];
	/** 這一輪目前為止收到的所有 graph 路徑 Activity 事件（T12 sub-project
	 * 3c）；非空時優先顯示——一輪只會走 legacy/durable 路徑（steps 非空）或
	 * graph 路徑（activity 非空）其中之一，不會同時有兩者。 */
	activity?: AgentActivityEvent[];
	/** Agent 現在的狀態，由 `deriveMotionState()` 從權威訊號算出。 */
	motion: AgentMotionState;
}

/**
 * Live Activity Rail——進行中那一輪的公開狀態軌（設計書 v6 §V.10.6、
 * §V.10.12-2、圖 00-B）。
 *
 * 取代舊的單行跑條。差別不是視覺偏好，是設計書的硬要求：
 *
 * - **收到幾個不同的 public step 就顯示幾列**（§V.10.6-1「不設固定兩列上限」）。
 *   舊版只顯示最新一行，前面做過什麼一送出就消失，使用者無從判斷它到底走到哪。
 * - **step 完成只完成那一列並保留**（§V.10.6-3）。step-level DONE 不得清空整軌，
 *   也不得冒充 run terminal。
 * - **只有 run terminal 或 RESET 才整軌原子替換**。session／訂閱層維護
 *   當前列，`AgentTaskPanel` 在處理結束後移除即時 surface。
 * - Slice A 的列身分鍵是（clientRequestId, code）——`session.liveSteps` 已經按
 *   code 收斂，所以這裡直接用 `step.code` 當 key。#2446 上線後才換成 backend
 *   發的 stepId／attemptId 與 seq。graph 路徑（`activity` 分支）不吃這條
 *   演進路徑，直接用後端指派的 `event.rowId` 當 key，本來就不是從
 *   clientRequestId／code 推導出來的。
 *
 * raw step code／label／payload 不進對話紀錄、history、artifact 或 analytics
 *（§V.10.6-3）。`AgentTaskPanel` 不保留已完成回合的快照；正式回覆使用另一個
 * surface。遙測只可在元件外送出列數、延遲與去重次數等聚合數字。
 *
 * 根節點是 `<div>`，由主對話區的獨立處理項目包住；右側成果區不呈現處理列。
 *
 * a11y：整條軌對 screen reader 隱藏——逐行冒出來會轟炸朗讀佇列。新步驟由父層
 * 的 atomic polite status 播報一次（§V.11.0 Accessibility）。
 */
/** 一列的內容。父列與 lane 子列長得一樣，只是包在不同層的 <ol> 裡——抽出來是
 * 為了不讓同一段 markup 存在兩份、改一邊忘另一邊。 */
function renderActivityRow(
	event: AgentActivityEvent,
	motion: AgentMotionState,
) {
	const running = event.status === "RUNNING";
	return (
		<div
			className="hawkr-agent-rail__row"
			data-status={event.status}
			data-row-id={event.rowId}
		>

			{running ? (
				<HawkrAgentSprite
					state={motion}
					className="hawkr-agent-rail__row-sprite"
					label=""
				/>
			) : (
				<AppIcon name={stepIcon(event.code)} size={16} aria-hidden="true" />
			)}
			<span className="hawkr-agent-rail__copy">{activityLabel(event)}</span>

			<em className="hawkr-agent-rail__state">
				{activityStatusLabel(event.status)}
			</em>

			{event.count !== undefined ? (
				<span className="hawkr-agent-rail__count">
					{running ? `已整理 ${event.count} 家` : `${event.count} 家`}
				</span>
			) : null}
		</div>
	);
}

export default function AgentActivityRail({
	steps,
	activity,
	motion,
}: AgentActivityRailProps) {
	const showActivity = activity !== undefined && activity.length > 0;

	return (
		<div
			className="hawkr-agent-rail"
			aria-hidden="true"
			data-testid="agent-activity-rail"
		>
			<ol className="hawkr-agent-rail__list">
				{showActivity
					? groupActivityRows(activity).map((row) => (
							<li key={row.rowId} className="hawkr-agent-rail__group">
								{renderActivityRow(row, motion)}

								{row.children.length > 0 ? (
									<ol className="hawkr-agent-rail__children">
										{row.children.map((child) => (
											<li key={child.rowId} className="hawkr-agent-rail__group">
												{renderActivityRow(child, motion)}
											</li>
										))}
									</ol>
								) : null}
							</li>
						))
					: steps.map((step) => {
							const running = step.status === "RUNNING";
							return (
								<li
									key={step.code}
									className="hawkr-agent-rail__row"
									data-status={step.status}
								>

									{running ? (
										<HawkrAgentSprite
											state={motion}
											className="hawkr-agent-rail__row-sprite"
											label=""
										/>
									) : (
										<AppIcon
											name={stepIcon(step.code)}
											size={16}
											aria-hidden="true"
										/>
									)}
									<span className="hawkr-agent-rail__copy">
										{stepLabel(step)}
									</span>

									{running ? (
										<em className="hawkr-agent-rail__state">進行中</em>
									) : null}
								</li>
							);
						})}
			</ol>
		</div>
	);
}
