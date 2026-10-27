"use client";

import {
	delegatedCapabilityChipLabels,
	delegatedSessionStateCopy,
} from "../labels";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import { formatRemainingDuration } from "../state/session";

/**
 * 金色情境列（mockup §3.2、L1031）。
 *
 * 由左而右的定版順序：綠點 ＋ `ADM 代行中` → 店名 → `管理員：{name}` →
 * `Session {displayCode}` → `剩餘 mm:ss` → capability chips → 右端 `結束代行`。
 *
 * ⚠️ 倒數是**本地推算**；真正的終態偵測靠每 30 秒一次的 `GET current`，`status`
 * 變 expired／revoked 時整個工作區切阻擋畫面，**最壞 30 秒延遲是預期行為**。
 * ⚠️ 那支輪詢 `counts_as_activity=false`（輪詢不算活動）—— 前端不得為了「保持
 * 連線」另外打任何端點，否則閒置逾時永遠不會發生。
 */
export default function OperationContextBar({
	onRequestEnd,
}: {
	onRequestEnd: () => void;
}) {
	const { summary, state, now, ending } = useDelegatedSession();
	if (!summary) return null;
	const copy = delegatedSessionStateCopy(state);
	const chips = delegatedCapabilityChipLabels(summary.capabilities);

	return (
		<section className="adm-context-bar" aria-label="代行工作階段">
			<span className="adm-context-bar__state">
				<span className="adm-context-bar__dot" aria-hidden="true" />
				{copy.label}
			</span>
			<strong className="adm-context-bar__store">
				{summary.target.storeName}
			</strong>
			<span className="adm-context-bar__meta">
				管理員：{summary.actor.displayName}
			</span>
			<span className="adm-context-bar__meta">
				Session {summary.displayCode}
			</span>
			<span className="adm-context-bar__timer">
				剩餘 {formatRemainingDuration(summary.expiresAt, now)}
			</span>
			<span className="adm-context-bar__chips">
				{chips.map((chip) => (
					<span className="adm-chip" key={chip}>
						{chip}
					</span>
				))}
			</span>
			<button
				type="button"
				className="adm-context-bar__end"
				onClick={onRequestEnd}
				disabled={ending}
			>
				{ending ? "正在確認結束" : "結束代行"}
			</button>
			<span className="sr-only" aria-live="polite" aria-atomic="true">
				{copy.label}
			</span>
		</section>
	);
}
