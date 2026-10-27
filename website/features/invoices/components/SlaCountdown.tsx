import { foldSlaCountdown } from "@/lib/invoices/slaCountdown";

const TONE_CLASS = {
	neutral: "border-divider bg-content2 text-foreground-soft",
	warning: "border-warning-200 bg-warning-50 text-warning-700",
	danger: "border-danger-200 bg-danger-50 text-danger-700",
} as const;

interface SlaCountdownProps {
	/** 端點 15 的 `remainingBusinessDays`（**數字字串、可為負、可為空**）。 */
	remainingBusinessDays: string;
	/** 端點 15 的 `overdueBusinessDays`；只在逾期時用得到。 */
	overdueBusinessDays: string;
	className?: string;
}

/**
 * N3｜工作天倒數（58A §0.3 的四條規則）。
 *
 * 分級與文字全部折在 `lib/invoices/slaCountdown.ts` 一處；本檔只負責畫出來。
 *
 * 三件事：
 *   1. **前端零計算**——值由後端給，空字串顯示「處理期限計算中」而不是退化成
 *      日曆天（§0.6 C10）。
 *   2. `aria-live="polite"` ＋ **不做每秒 tick**：工作天是整數天，秒級更新沒有
 *      資訊增益，只有每秒 re-render。
 *   3. 顏色之外一定同時輸出文字——狀態不可只靠顏色（§B-1.0.8）。
 */
export function SlaCountdown({
	remainingBusinessDays,
	overdueBusinessDays,
	className,
}: SlaCountdownProps) {
	const display = foldSlaCountdown(remainingBusinessDays, overdueBusinessDays);
	return (
		<span
			aria-live="polite"
			className={[
				"inline-flex shrink-0 items-center rounded-full border px-3 py-1 text-xs font-bold tabular-nums",
				TONE_CLASS[display.tone],
				className,
			]
				.filter(Boolean)
				.join(" ")}
		>
			{display.text}
		</span>
	);
}
