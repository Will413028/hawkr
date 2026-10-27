"use client";

/**
 * 自然日倒數（58A §B-2.3 (7) 的第二支）。
 *
 * 🔴 **與 `SlaCountdown` 不可合成一個**，兩者的權責方向相反：
 *   - `SlaCountdown`（`features/invoices/components/SlaCountdown.tsx`）走**工作天**，
 *     值全部由後端給、前端零計算——前端只扣得掉週末，後端用行事曆表扣國定假日，
 *     自己算會讓同一件案子在賣家面與 ADM 顯示不同的天數，而逾期會計入違規。
 *   - 本元件走**自然日**，由 `deadlineAt` 前端推導——因為自然日沒有行事曆問題。
 *
 * 🔴 推導一律走 `lib/invoices/display.ts` 的 `taipeiDaysUntil()`（台北日界相減），
 * 不得用 `Math.ceil(diff / 86400000)`：後者會讓賣家 23:00 看到 41、01:00 看到 42
 * （§B-2.0 T12）。
 *
 * 狀態不可只靠顏色 ⇒ 一律同時輸出文字，並掛 `aria-live="polite"`；不做每秒 tick
 * （整數天，秒級更新沒有資訊增益而且每秒 re-render）。
 */

import { taipeiDaysUntil } from "@/lib/invoices/display";

interface Props {
	/** ISO 8601 UTC。空字串或讀不出來 ⇒ 整個元件不渲染（見下）。 */
	deadlineAt: string;
	className?: string;
}

const TONE_CLASS = {
	neutral: "text-[var(--hkui-foreground-soft)]",
	warning: "text-warning-700",
	danger: "text-danger",
} as const;

export function DeadlineCountdown({ deadlineAt, className }: Props) {
	const days = taipeiDaysUntil(deadlineAt);
	// 讀不出來就整個不渲染。**不要顯示「剩餘 0 天」**——那會被讀成「今天到期」，
	// 而實際情況是我們根本不知道期限。
	if (days === null) return null;

	const tone = days < 0 ? "danger" : days <= 2 ? "warning" : "neutral";
	const text = days < 0 ? `已逾期 ${Math.abs(days)} 天` : `剩餘 ${days} 天`;

	return (
		<span
			aria-live="polite"
			className={[
				"text-[0.8125rem] font-bold tabular-nums",
				TONE_CLASS[tone],
				className,
			]
				.filter(Boolean)
				.join(" ")}
		>
			{text}
		</span>
	);
}

export default DeadlineCountdown;
