"use client";

import AppIcon from "@/components/shared/AppIcon";
import { Button, Chip } from "@/components/ui";
import type {
	ProcurementCandidateSearch,
	ProcurementLaneStatus,
} from "@/lib/api/openapi-types";

const LANE_LABEL = { PLATFORM: "平台賣家", EXTERNAL: "外部業者" } as const;

/**
 * key 型別直接引用 ProcurementLaneStatus，不再手寫字面值 union——這樣值域一改，
 * 這張表少一個 key 就是編譯錯誤。之前手寫的版本漏一個值只會在執行期查表回
 * undefined，那一列的文字整個空白，沒有任何訊號。
 */
const LANE_STATUS_LABEL: Record<ProcurementLaneStatus, string> = {
	PENDING: "搜尋中",
	SUCCEEDED: "已完成",
	FAILED: "失敗",
};

/** lane 已成功。 */
function isLaneSucceeded(status: ProcurementLaneStatus): boolean {
	return status === "SUCCEEDED";
}

/** 輪詢掛掉時的重試入口。抽出來是因為 searching 階段有兩種畫面（合併後的軌、
 * 退回的兩列 lane 卡片），兩邊都必須看得到它——留在卡片裡的話，軌取代卡片時
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
	return (
		<section
			className="assistant-artifact assistant-progress"
			aria-labelledby="progress-title"
		>
			<header className="assistant-artifact__head">
				<div>
					<h2 id="progress-title">正在搜尋候選</h2>
					<p>平台賣家與外部名單分開查詢，兩邊都完成後才會顯示結果。</p>
				</div>
				<Chip variant="dot" color="primary" size="sm">
					搜尋中
				</Chip>
			</header>

			{pollError ? (
				<SearchPollErrorAlert onRetryPolling={onRetryPolling} />
			) : (
				<ol className="assistant-progress__list">
					{(["PLATFORM", "EXTERNAL"] as const).map((lane) => {
						const status = candidateSet
							? lane === "PLATFORM"
								? candidateSet.platformLaneStatus
								: candidateSet.externalLaneStatus
							: "PENDING";
						return (
							<li
								key={lane}
								data-status={isLaneSucceeded(status) ? "completed" : "current"}
							>
								<AppIcon
									name={isLaneSucceeded(status) ? "check" : "clock"}
									size={17}
									aria-hidden="true"
								/>
								<div>
									<strong>{LANE_LABEL[lane]}</strong>
									<span>{LANE_STATUS_LABEL[status]}</span>
								</div>
							</li>
						);
					})}
				</ol>
			)}
		</section>
	);
}
