import "./SlaCountdown.css";

export type SlaTone = "neutral" | "warning" | "danger";

export interface SlaCountdownProps {
	/** 後端算好的**剩餘工作天**，數字字串、可為負；空字串代表後端算不出來。 */
	remainingBusinessDays: string;
	/** 後端算好的**已逾期工作天**，數字字串；空字串代表不適用。 */
	overdueBusinessDays: string;
}

/**
 * SLA 期限（工作天）的顯示。
 *
 * 🔴 **值全部由後端給，前端一個字都不算。** 工作天要扣的不只週末還有國定假日
 * （後端有行事曆表），前端自己算的話同一件申訴會在 Seller Web 顯示「剩 1 天」、
 * 在 ADM 顯示「已逾期」——而逾期會計違規、90 天達 3 次時建議停權（由管理員建立處分）。
 *
 * 🔴 **分級一定要同時輸出文字**：`>= 3` neutral／`1–2` warning／逾期 danger，但顏色
 * 只是輔助。只靠底色的狀態對色覺障礙與黑白列印等於沒有標記。
 *
 * ⚠️ **不做每秒 tick 的倒數**：工作天是整數天，秒級更新沒有任何資訊增益，只會讓整張
 * 表格每秒重繪一次。商家通報清單的期限（`lib/deadline.ts`）是日曆時間的相對文字，不是工作天，不經這個元件。
 *
 * ⚠️ 後端回空字串時顯示「處理期限計算中」，**不得退化成日曆天**——那正是上面那條要
 * 避免的東西，而且退化之後畫面上看不出來它是估的。
 */
export default function SlaCountdown({
	remainingBusinessDays,
	overdueBusinessDays,
}: SlaCountdownProps) {
	const overdue = Number(overdueBusinessDays);
	if (overdueBusinessDays !== "" && Number.isFinite(overdue) && overdue > 0) {
		return (
			<span className="adm-sla adm-sla--danger" aria-live="polite">
				已逾期 {overdue} 個工作天
			</span>
		);
	}

	if (remainingBusinessDays === "") {
		return (
			<span className="adm-sla" aria-live="polite">
				處理期限計算中
			</span>
		);
	}

	const remaining = Number(remainingBusinessDays);
	if (!Number.isFinite(remaining)) {
		// 後端給了一個看不懂的值：原樣顯示。吞掉它的話沒有人會發現契約壞了。
		return (
			<span className="adm-sla" aria-live="polite">
				{remainingBusinessDays}
			</span>
		);
	}
	if (remaining < 0) {
		return (
			<span className="adm-sla adm-sla--danger" aria-live="polite">
				已逾期 {Math.abs(remaining)} 個工作天
			</span>
		);
	}

	const tone: SlaTone = remaining >= 3 ? "neutral" : "warning";
	return (
		<span className={`adm-sla adm-sla--${tone}`} aria-live="polite">
			剩餘 {remaining} 個工作天
		</span>
	);
}
