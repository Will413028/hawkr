"use client";

import type * as React from "react";
import AppIcon from "@/components/shared/AppIcon";

interface AgentSidePanelProps {
	children?: React.ReactNode;
	onClose: () => void;
	hidden?: boolean;
}

/**
 * 成果工作台。思考與處理進度留在主對話區，這裡只承載候選、比較與詢價資料。
 * 收起時使用 hidden，保留 CandidateCanvas 的檢視、選取與確認草稿。
 */
export default function AgentSidePanel({
	children,
	onClose,
	hidden = false,
}: AgentSidePanelProps) {
	return (
		<aside
			id="hawkr-agent-side-panel"
			className="hawkr-agent-panel"
			hidden={hidden}
			aria-label="採購成果"
		>
			<header className="hawkr-agent-panel__header">
				<h2>成果</h2>
				<button
					type="button"
					className="hawkr-agent-panel__close"
					onClick={onClose}
				>
					<AppIcon name="arrow-left" size={17} aria-hidden="true" />
					回到對話
				</button>
			</header>
			<div className="hawkr-agent-panel__content">{children}</div>
		</aside>
	);
}
