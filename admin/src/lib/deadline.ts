// 期限的顯示：日曆時間，不看工作日。相對時間隨清單每 30 秒更新（`useNow`），絕對時間放在 title 或詳情頁。

import { timestampDate } from "@bufbuild/protobuf/wkt";
import type { Deadline } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import { formatDateTimeShort } from "./format";

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;

export type DeadlineTone = "" | "warning" | "danger";

export interface DeadlineDisplay {
	/** 「剩 {時長}」、「已逾期 {時長}」，沒有期限是「-」。 */
	text: string;
	/** 逾期是 danger；剩下的時間不到整段期限的四分之一是 warning；其餘沒有色階。 */
	tone: DeadlineTone;
}

/**
 * 時長的文字，一律無條件捨去：
 * 滿 1 天「{d} 天 {h} 小時」（剛好整天「{d} 天」）、
 * 1 小時到 1 天「{h} 小時 {m} 分」（剛好整點「{h} 小時」）、
 * 不到 1 小時「{m} 分」、不到 1 分「不到 1 分」。
 * 傳入的是非負的毫秒數；負數與非數字當成 0。
 */
export function durationText(ms: number): string {
	const total = Number.isFinite(ms) && ms > 0 ? ms : 0;
	if (total >= DAY_MS) {
		const days = Math.floor(total / DAY_MS);
		const hours = Math.floor((total % DAY_MS) / HOUR_MS);
		return hours === 0 ? `${days} 天` : `${days} 天 ${hours} 小時`;
	}
	if (total >= HOUR_MS) {
		const hours = Math.floor(total / HOUR_MS);
		const minutes = Math.floor((total % HOUR_MS) / MINUTE_MS);
		return minutes === 0 ? `${hours} 小時` : `${hours} 小時 ${minutes} 分`;
	}
	if (total >= MINUTE_MS) return `${Math.floor(total / MINUTE_MS)} 分`;
	return "不到 1 分";
}

/**
 * 期限欄的文字與色階。`due` 是伺服器給的期限（沒有期限，或義務已經結束，伺服器就不輸出，這裡顯示「-」）；
 * `start` 是這段期限的開始時間，只用來算「整段期限的四分之一」（沒有開始時間就不轉 warning）。
 * 時間一律由呼叫端傳入（now），這裡不讀時鐘。現在晚於期限才算逾期。
 */
export function deadlineDisplay(input: {
	due: Date | undefined;
	start: Date | undefined;
	now: Date;
}): DeadlineDisplay {
	const { due, start, now } = input;
	if (!due || Number.isNaN(due.getTime())) return { text: "-", tone: "" };
	const remaining = due.getTime() - now.getTime();
	if (remaining < 0) {
		return { text: `已逾期 ${durationText(-remaining)}`, tone: "danger" };
	}
	const length = start ? due.getTime() - start.getTime() : 0;
	const warning = length > 0 && remaining < length / 4;
	return {
		text: `剩 ${durationText(remaining)}`,
		tone: warning ? "warning" : "",
	};
}

export interface DeadlineView extends DeadlineDisplay {
	/** 期限的絕對時間（`formatDateTimeShort`）；沒有期限是 undefined。 */
	dueText: string | undefined;
}

/**
 * 伺服器給的 `Deadline{startTime, dueTime}` 的顯示：相對文字與色階用 `deadlineDisplay`，開始時間就是 `startTime`
 * （不再用資源的建立時間自己算）；沒有期限（義務不在，伺服器不輸出）是「-」。
 */
export function deadlineView(
	deadline: Deadline | undefined,
	now: Date,
): DeadlineView {
	const due = deadline?.dueTime ? timestampDate(deadline.dueTime) : undefined;
	const start = deadline?.startTime
		? timestampDate(deadline.startTime)
		: undefined;
	return {
		...deadlineDisplay({ due, start, now }),
		dueText:
			due && !Number.isNaN(due.getTime())
				? formatDateTimeShort(due)
				: undefined,
	};
}
