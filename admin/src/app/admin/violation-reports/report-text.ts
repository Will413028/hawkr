import {
	type Case,
	CaseCloseReason,
	CaseDecision,
	CaseState,
	type Report,
	ReportState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { sellerReportOutcome } from "@hawkr/ui-text/keys";
import { staffTextOrUnknown, unknownText } from "@/lib/ui-text";


function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

export function reportOutcomeText(outcome: ReportOutcome | undefined): string {
	if (outcome === undefined) return unknownText();
	return staffTextOrUnknown(sellerReportOutcome(outcome));
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
