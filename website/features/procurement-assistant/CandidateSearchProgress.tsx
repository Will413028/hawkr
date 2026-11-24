"use client";

import AppIcon from "@/components/shared/AppIcon";
import { Button, Chip } from "@/components/ui";
import type { ProcurementCandidateSearch } from "./types";

type SearchStatus = "PENDING" | "SUCCEEDED" | "FAILED";

const SEARCH_STATUS_LABEL: Record<SearchStatus, string> = {
	PENDING: "搜尋中",
	SUCCEEDED: "已完成",
	FAILED: "失敗",
};

/** 搜尋 run 的狀態收斂成進度列的三態：還沒有結果、成功、失敗（取消與失敗同樣沒有候選）。 */
function searchStatusOf(
	candidateSet: ProcurementCandidateSearch | null,
): SearchStatus {
	if (!candidateSet) return "PENDING";
	if (candidateSet.status === "SUCCEEDED") return "SUCCEEDED";
	if (candidateSet.status === "FAILED" || candidateSet.status === "CANCELLED")
		return "FAILED";
	return "PENDING";
}

/** 輪詢掛掉時的重試入口。抽出來是因為 searching 階段有兩種畫面（合併後的軌、
 * 退回的輪詢進度卡片），兩邊都必須看得到它——留在卡片裡的話，軌取代卡片時
 * 輪詢死掉就沒有任何重試的地方。 */
export function SearchPollErrorAlert({
	onRetryPolling,
}: {
	onRetryPolling: () => void;
}) {
	return (
		<div className="assistant-invalidated" role="alert">
			<AppIcon name="warning" size={20} aria-hidden="true" />
			<div>
				<strong>連線異常</strong>
				<p>暫時無法確認搜尋進度，請檢查網路連線後重試。</p>
			</div>
			<Button variant="bordered" color="default" onClick={onRetryPolling}>
				重試
			</Button>
		</div>
	);
}

interface CandidateSearchProgressProps {
	candidateSet: ProcurementCandidateSearch | null;
	pollError: boolean;
	onRetryPolling: () => void;
}

export default function CandidateSearchProgress({
	candidateSet,
	pollError,
	onRetryPolling,
}: CandidateSearchProgressProps) {
	const status = searchStatusOf(candidateSet);
	return (
		<section
			className="assistant-artifact assistant-progress"
			aria-labelledby="progress-title"
		>
			<header className="assistant-artifact__head">
				<div>
					<h2 id="progress-title">正在搜尋候選</h2>
				</div>
				<Chip variant="dot" color="primary" size="sm">
					搜尋中
				</Chip>
			</header>

			{pollError ? (
				<SearchPollErrorAlert onRetryPolling={onRetryPolling} />
			) : (
				<ol className="assistant-progress__list">
					<li data-status={status === "SUCCEEDED" ? "completed" : "current"}>
						<AppIcon
							name={status === "SUCCEEDED" ? "check" : "clock"}
							size={17}
							aria-hidden="true"
						/>
						<div>
							<strong>搜尋候選供應商</strong>
							<span>{SEARCH_STATUS_LABEL[status]}</span>
						</div>
					</li>
				</ol>
			)}
		</section>
	);
}
