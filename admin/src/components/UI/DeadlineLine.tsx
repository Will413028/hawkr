import type { Deadline } from "@hawkr/api/gen/hawkr/common/v1alpha1/deadline_pb";
import { deadlineView } from "@/lib/deadline";
import DeadlineText from "./DeadlineText";

export interface DeadlineLineProps {
	/** 欄位名，例如「裁定期限」。 */
	label: string;
	/** 伺服器給的期限；沒有（義務不在）時顯示「{label}：-」。 */
	deadline: Deadline | undefined;
	now: Date;
}

/** 「{label}：{日期時間}（剩／已逾期 …）」；沒有期限是「{label}：-」。 */
export default function DeadlineLine({
	label,
	deadline,
	now,
}: DeadlineLineProps) {
	const view = deadlineView(deadline, now);
	if (view.dueText === undefined) return <>{label}：-</>;
	return (
		<>
			{label}：{view.dueText}（<DeadlineText display={view} />）
		</>
	);
}
