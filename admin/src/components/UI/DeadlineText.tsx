import type { DeadlineDisplay } from "@/lib/deadline";
import "./SlaCountdown.css";

export interface DeadlineTextProps {
	display: DeadlineDisplay;
	/** 滑過時顯示的絕對時間。 */
	title?: string;
}

/**
 * 期限的相對文字（`deadlineDisplay`）。色階同時輸出文字（剩／已逾期），顏色只是輔助；
 * 樣式與 SlaCountdown 共用，兩者並排時看起來一致。
 */
export default function DeadlineText({ display, title }: DeadlineTextProps) {
	const tone = display.tone ? ` adm-sla--${display.tone}` : "";
	return (
		<span className={`adm-sla${tone}`} title={title}>
			{display.text}
		</span>
	);
}
