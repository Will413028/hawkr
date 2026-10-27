"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { Button } from "@/components/ui";
import HawkrAgentMark from "./HawkrAgentMark";

interface AgentTaskCardProps {
	pending?: boolean;
	onAssign: () => void | Promise<void>;
}

/**
 * REQUIREMENT_READY 對應的真實下一步。這不是泛用 AI 建議卡：唯一 action 直接
 * 呼叫 POST /candidate-searches，並把 v5.0 的邊界（只找候選、不自動下單）放在
 * 指派按鈕旁，讓使用者知道自己正在授權哪一件事。
 */
export default function AgentTaskCard({
	pending = false,
	onAssign,
}: AgentTaskCardProps) {
	const [assigning, setAssigning] = React.useState(false);
	const assign = async () => {
		if (pending || assigning) return;
		setAssigning(true);
		try {
			await onAssign();
		} finally {
			setAssigning(false);
		}
	};

	return (
		<li className="hawkr-agent-task-row">
			<HawkrAgentMark
				label="Hawkr Agent"
				className="assistant-mark hawkr-agent-message-avatar hawkr-agent-message-avatar--agent"
			/>
			<section
				className="hawkr-agent-task-card"
				aria-labelledby="candidate-search-task-title"
			>
				<header className="hawkr-agent-task-card__head">
					<span className="hawkr-agent-task-card__icon" aria-hidden="true">
						<AppIcon name="search" size={20} />
					</span>
					<div>
						<h2 id="candidate-search-task-title">搜尋候選供應商</h2>
						<p>需求已經可以開始搜尋，確認後我會接手下一段工作。</p>
					</div>
					<span className="hawkr-agent-task-card__status">可指派</span>
				</header>

				<ul className="hawkr-agent-task-card__scope" aria-label="搜尋範圍">
					<li>
						<AppIcon name="bag" size={17} aria-hidden="true" />
						<span>平台賣家</span>
					</li>
					<li>
						<AppIcon name="discovery" size={17} aria-hidden="true" />
						<span>外部業者</span>
					</li>
				</ul>

				<footer className="hawkr-agent-task-card__footer">
					<span>
						<AppIcon name="shield" size={17} aria-hidden="true" />
						只整理候選，不會替你下單
					</span>
					<Button
						onClick={() => void assign()}
						disabled={pending || assigning}
						isLoading={assigning}
						loadingLabel="正在指派…"
						endContent={<AppIcon name="send" size={18} aria-hidden="true" />}
					>
						交給 Hawkr Agent 搜尋
					</Button>
				</footer>
			</section>
		</li>
	);
}
