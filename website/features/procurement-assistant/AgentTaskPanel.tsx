"use client";

import type { ProcurementCandidateSearch } from "@/lib/api/openapi-types";
import AgentActivityRail from "./AgentActivityRail";
import type { AgentActivityEvent } from "./activity";
import type { AgentMotionState } from "./agent-motion";
import CandidateSearchProgress, {
	SearchPollErrorAlert,
} from "./CandidateSearchProgress";
import HawkrAgentSprite from "./HawkrAgentSprite";
import type { AgentStep } from "./steps";
import type { ProcurementStage } from "./types";

interface AgentTaskPanelProps {
	stage: ProcurementStage;
	pending: boolean;
	liveSteps: AgentStep[];
	liveActivity: AgentActivityEvent[];
	/** 目前搜尋 run 的公開列，已由訂閱層按 rowId 收斂；尚未收到或環境
	 * 未開放此能力時，以既有 candidateSet 輪詢狀態顯示進度。 */
	searchActivity: AgentActivityEvent[];
	motion: AgentMotionState;
	candidateSet: ProcurementCandidateSearch | null;
	pollError: boolean;
	onRetryPolling: () => void;
}

/** 主對話區的即時處理 surface，與正式回覆分開。
 *
 * 只顯示當前 turn 或搜尋 run 的公開狀態。完成後立即移除，不保留上一輪
 * 快照，也不把處理列寫入對話或成果。換 run／同 rowId 更新由 session 與
 * 訂閱層負責；此處直接呈現它們已收斂的當前資料。
 * Screen reader 的即時播報統一由 AssistantWorkspace 的 status 負責。 */
export default function AgentTaskPanel({
	stage,
	pending,
	liveSteps,
	liveActivity,
	searchActivity,
	motion,
	candidateSet,
	pollError,
	onRetryPolling,
}: AgentTaskPanelProps) {
	if (stage !== "searching" && !pending) return null;

	if (stage === "searching") {
		return (
			<div className="hawkr-agent-processing" data-testid="agent-processing">
				{searchActivity.length > 0 ? (
					<>
						{pollError ? (
							<SearchPollErrorAlert onRetryPolling={onRetryPolling} />
						) : null}
						<h3 className="hawkr-agent-rail__heading">搜尋進度</h3>
						<AgentActivityRail
							steps={[]}
							activity={searchActivity}
							motion={motion}
						/>
					</>
				) : (
					<CandidateSearchProgress
						candidateSet={candidateSet}
						pollError={pollError}
						onRetryPolling={onRetryPolling}
					/>
				)}
			</div>
		);
	}

	const hasLiveRows = liveSteps.length > 0 || liveActivity.length > 0;

	return (
		<div className="hawkr-agent-processing" data-testid="agent-processing">
			{hasLiveRows ? (
				<>
					<h3 className="hawkr-agent-rail__heading">處理進度</h3>
					<AgentActivityRail
						steps={liveSteps}
						activity={liveActivity}
						motion={motion}
					/>
				</>
			) : (
				<div className="hawkr-agent-processing__waiting" aria-hidden="true">
					{/* 尚無公開步驟時只表達等待，不推測後端階段或循環播放。 */}
					<HawkrAgentSprite
						state="SUBMITTING"
						className="hawkr-agent-rail__sprite"
						label=""
					/>
					<span>等待回應…</span>
				</div>
			)}
		</div>
	);
}
