/**
 * N3｜工作天倒數的折疊表（58A §0.3 的四條規則）。
 *
 * 🔴 **前端零計算工作天**（§0.6 C10）。值全部由後端給：前端只扣得掉週末，後端
 * 用行事曆表扣國定假日 ⇒ 自己算會讓同一件案子在買家面顯示「剩 1 天」、在 ADM
 * 顯示「已逾期」，而逾期會計入賣家違規、90 天 3 次即停權。
 *
 * 🔴 **後端回空字串時顯示「處理期限計算中」，不得退化成日曆天**。空字串是設計
 * 指定的 fallback（行事曆鏡像沒有資料時後端不猜），不是 bug。
 *
 * 🔴 `remainingBusinessDays` 是**數字字串且可為負**：直接 `>= 3` 是字典序比較，
 * `"-2" >= "3"` 為 false 而 `"10" >= "3"` 也為 false ⇒ 逾期與充裕會落進同一級，
 * 而畫面看起來完全正常（§B-1.0.9 B06t）。所以順序固定是「先判空 → 再 Number
 * → 才比大小」。
 */

export type SlaTone = "neutral" | "warning" | "danger";

export interface SlaCountdownDisplay {
	/** 已經是完整的一句話，可直接放進 DOM。 */
	text: string;
	tone: SlaTone;
	/** 後端算不出來（空字串／非數字）時為 true，供除錯與測試辨識。 */
	unknown: boolean;
}

/** 後端算不出來時的唯一呈現。**不得換算成日曆天**。 */
const PENDING: SlaCountdownDisplay = {
	text: "處理期限計算中",
	tone: "neutral",
	unknown: true,
};

/**
 * 分級（§0.3 第 2 條）：`>= 3` neutral／`1–2` warning／逾期 danger，且**同時輸出
 * 文字**（狀態不可只靠顏色）。
 *
 * `overdueBusinessDays` 只在逾期時用得到；後端沒給時退回 `-remaining`，兩者是
 * 同一個數的兩種表達。
 */
export function foldSlaCountdown(
	remainingBusinessDays: string,
	overdueBusinessDays: string,
): SlaCountdownDisplay {
	if (remainingBusinessDays === "") return PENDING;
	const remaining = Number(remainingBusinessDays);
	if (!Number.isFinite(remaining)) return PENDING;

	if (remaining < 0) {
		const parsedOverdue =
			overdueBusinessDays === "" ? Number.NaN : Number(overdueBusinessDays);
		const overdue =
			Number.isFinite(parsedOverdue) && parsedOverdue > 0
				? parsedOverdue
				: -remaining;
		return {
			text: `已逾期 ${overdue} 個工作天`,
			tone: "danger",
			unknown: false,
		};
	}

	// 0 有自己的一句話：「剩餘 0 個工作天」會被讀成「還有時間」，而它其實是
	// **今天最後一天**（58A §S09 的逐字文案表）。分級仍是 warning（1–2 那一級），
	// 不升成 danger——今天到期還沒逾期。
	if (remaining === 0) {
		return { text: "今天到期", tone: "warning", unknown: false };
	}

	return {
		text: `剩餘 ${remaining} 個工作天`,
		tone: remaining >= 3 ? "neutral" : "warning",
		unknown: false,
	};
}
