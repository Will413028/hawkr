import {
	afterSaleDisplayLabel,
	afterSaleDisplayTone,
} from "@/lib/after-sales/status";
import type { AfterSaleTone } from "@/lib/after-sales/types";

/** Tone → token classes. Uses the shared semantic palette so the badge works
 * on both buyer and seller surfaces and flips correctly in dark mode. */
const TONE_CLASS: Record<AfterSaleTone, string> = {
	success:
		"bg-success-50 text-success-700 dark:bg-hk-ink-400/20 dark:text-hk-ink-200",
	warning:
		"bg-warning-50 text-warning-700 dark:bg-[#a9842c]/26 dark:text-[#d6c27e]",
	danger:
		"bg-danger-50 text-danger-700 dark:bg-[#b45644]/26 dark:text-[#e6b8a6]",
	info: "bg-info-50 text-info-700 dark:bg-[#487984]/22 dark:text-[#9cb9bd]",
	muted:
		"bg-default-100 text-default-700 dark:bg-hk-ink-100/10 dark:text-hk-ink-100/58",
};

const DOT_CLASS: Record<AfterSaleTone, string> = {
	success: "bg-success-700 dark:bg-hk-ink-300",
	warning: "bg-warning-700 dark:bg-[#c7ae62]",
	danger: "bg-danger-700 dark:bg-[#d49682]",
	info: "bg-info-700 dark:bg-[#6f9aa5]",
	muted: "bg-default-600 dark:bg-hk-ink-100/42",
};

interface Props {
	status: string;
	/** Case appeal state — when under appeal the pill folds to「申訴仲裁中」so
	 * list/card rows reflect arbitration instead of the stale status. */
	appealState?: string;
	/** Override the label (e.g. show a custom string). */
	label?: string;
	size?: "sm" | "md";
	showDot?: boolean;
	className?: string;
}

/** Canonical after-sale status pill. Single source of tone + label. */
export function AfterSaleStatusBadge({
	status,
	appealState,
	label,
	size = "md",
	showDot = true,
	className = "",
}: Props) {
	const tone = afterSaleDisplayTone(status, appealState);
	const text = label ?? afterSaleDisplayLabel(status, appealState);
	const pad =
		size === "sm" ? "px-2 py-0.5 text-[0.6875rem]" : "px-2.5 py-1 text-xs";
	return (
		<span
			className={`inline-flex items-center gap-1.5 rounded-full font-bold whitespace-nowrap ${pad} ${TONE_CLASS[tone]} ${className}`}
		>
			{showDot && (
				<span
					className={`inline-block w-1.5 h-1.5 rounded-full ${DOT_CLASS[tone]}`}
					aria-hidden="true"
				/>
			)}
			{text}
		</span>
	);
}

export default AfterSaleStatusBadge;
