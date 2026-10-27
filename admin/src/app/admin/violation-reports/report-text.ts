import {
	type Case,
	CaseCloseReason,
	CaseDecision,
	CaseState,
	FDAChannel,
	type Report,
	ReportState,
	SupplementMatch,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { unknownText } from "@/lib/ui-text";

// 通報與案件的名稱。這些列舉只有 admin 用，不在共用文案目錄裡，所以是 admin 的單端表（UNSPECIFIED 不在表裡）；
// 不認得的值與 UNSPECIFIED 顯示「未知」。結果與決定的詞在 10f 做「我的檢舉」、A8 寫通知時才進術語表與目錄。

function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

/** 結果的名稱：已採取措施｜未違規｜內容不足｜重複｜濫報。 */
const REPORT_OUTCOME_TEXT: Record<
	Exclude<ReportOutcome, ReportOutcome.UNSPECIFIED>,
	string
> = {
	[ReportOutcome.ACTION_TAKEN]: "已採取措施",
	[ReportOutcome.NO_VIOLATION]: "未違規",
	[ReportOutcome.INSUFFICIENT]: "內容不足",
	[ReportOutcome.DUPLICATE]: "重複",
	[ReportOutcome.ABUSIVE]: "濫報",
};

export function reportOutcomeText(outcome: ReportOutcome): string {
	return textOf<ReportOutcome>(REPORT_OUTCOME_TEXT, outcome);
}

/** 決定的名稱：採取措施｜不採取措施。 */
const CASE_DECISION_TEXT: Record<
	Exclude<CaseDecision, CaseDecision.UNSPECIFIED>,
	string
> = {
	[CaseDecision.ACTION_TAKEN]: "採取措施",
	[CaseDecision.NO_ACTION]: "不採取措施",
};

export function caseDecisionText(decision: CaseDecision): string {
	return textOf<CaseDecision>(CASE_DECISION_TEXT, decision);
}

/** 還沒有結果的通報的狀態；有結果（RESOLVED）時列出的是結果的名稱，見 reportStatusText。 */
const REPORT_STATE_TEXT: Record<
	Exclude<ReportState, ReportState.UNSPECIFIED | ReportState.RESOLVED>,
	string
> = {
	[ReportState.AWAITING_TRIAGE]: "待分案",
	[ReportState.IN_CASE]: "處理中",
};

/** 通報的狀態：待分案｜處理中｜{結果}。 */
export function reportStatusText(
	report: Pick<Report, "state" | "outcome">,
): string {
	if (report.state === ReportState.RESOLVED) {
		return reportOutcomeText(report.outcome);
	}
	return textOf<ReportState>(REPORT_STATE_TEXT, report.state);
}

const CASE_STATE_TEXT: Record<
	Exclude<CaseState, CaseState.UNSPECIFIED>,
	string
> = {
	[CaseState.OPEN]: "進行中",
	[CaseState.CLOSED]: "已結案",
};

/** 通報都移到其他案件而由系統關閉：不是一次決定，所以沒有決定的名稱。 */
const CASE_CLOSE_REASON_TEXT: Record<
	Exclude<
		CaseCloseReason,
		CaseCloseReason.UNSPECIFIED | CaseCloseReason.DECIDED
	>,
	string
> = {
	[CaseCloseReason.NO_REPORTS]: "沒有通報",
};

/** 案件的狀態：進行中｜已結案：採取措施｜已結案：不採取措施｜已結案：沒有通報。 */
export function caseStatusText(
	kase: Pick<Case, "state" | "closeReason" | "decision">,
): string {
	if (kase.state !== CaseState.CLOSED) {
		return textOf<CaseState>(CASE_STATE_TEXT, kase.state);
	}
	const closed = textOf<CaseState>(CASE_STATE_TEXT, kase.state);
	const detail =
		kase.closeReason === CaseCloseReason.DECIDED
			? caseDecisionText(kase.decision)
			: textOf<CaseCloseReason>(CASE_CLOSE_REASON_TEXT, kase.closeReason);
	return `${closed}：${detail}`;
}

/** 轉報主管機關的管道。 */
const FDA_CHANNEL_TEXT: Record<
	Exclude<FDAChannel, FDAChannel.FDA_CHANNEL_UNSPECIFIED>,
	string
> = {
	[FDAChannel.FDA_CHANNEL_HOTLINE_1919]: "1919 食安專線",
	[FDAChannel.FDA_CHANNEL_ONLINE_FORM]: "線上表單",
	[FDAChannel.FDA_CHANNEL_FAX_HEALTH_DEPT]: "傳真衛生局",
	[FDAChannel.FDA_CHANNEL_IN_PERSON]: "親自送件",
	[FDAChannel.FDA_CHANNEL_OTHER]: "其他",
};

/** 管道的選項，順序同契約。 */
export const FDA_CHANNEL_OPTIONS: readonly Exclude<
	FDAChannel,
	FDAChannel.FDA_CHANNEL_UNSPECIFIED
>[] = [
	FDAChannel.FDA_CHANNEL_HOTLINE_1919,
	FDAChannel.FDA_CHANNEL_ONLINE_FORM,
	FDAChannel.FDA_CHANNEL_FAX_HEALTH_DEPT,
	FDAChannel.FDA_CHANNEL_IN_PERSON,
	FDAChannel.FDA_CHANNEL_OTHER,
];

export function fdaChannelText(channel: FDAChannel): string {
	return textOf<FDAChannel>(FDA_CHANNEL_TEXT, channel);
}

/** 匿名的補充以哪一種聯絡方式比對到原通報；兩者都相同時，以及登入者的補充，不需要提醒。 */
const SUPPLEMENT_MATCH_NOTICE: Record<
	Exclude<SupplementMatch, SupplementMatch.UNSPECIFIED | SupplementMatch.BOTH>,
	string
> = {
	[SupplementMatch.PHONE]:
		"以電話比對到這筆通報的匿名送出，送出者不一定是原檢舉人。",
	[SupplementMatch.EMAIL]:
		"以聯絡信箱比對到這筆通報的匿名送出，送出者不一定是原檢舉人。",
};

/** 補充的比對提醒；不需要提醒時回 undefined。 */
export function supplementMatchNotice(
	match: SupplementMatch,
): string | undefined {
	return (SUPPLEMENT_MATCH_NOTICE as Partial<Record<SupplementMatch, string>>)[
		match
	];
}
