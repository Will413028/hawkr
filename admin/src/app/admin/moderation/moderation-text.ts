import {
	CaseAction,
	CaseDecision,
	type CaseReport,
	CaseState,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import {
	type ReportOutcome,
	ReportTargetKind,
} from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import {
	adminModerationCaseReportOutcomeUnset,
	moderationReportOutcome,
	moderationReportTargetKind,
} from "@hawkr/ui-text/keys";
import {
	COMMUNITY_ASSESSABLE_CATEGORIES,
	CONTENT_TAKEDOWN_CATEGORIES,
} from "@/lib/policy-category";
import { staffTextOrUnknown, unknownText } from "@/lib/ui-text";

// 社群檢舉案件的名稱。檢舉對象種類與檢舉結果取目錄的後台受眾文字；其餘列舉只有 admin 用，是 admin 的單端表（UNSPECIFIED 不在表裡）；
// 不認得的值與 UNSPECIFIED 顯示「未知」。

function textOf<E extends number>(
	table: Partial<Record<E, string>>,
	value: E,
): string {
	return table[value] ?? unknownText();
}

/** 篩選下拉的選項順序：enum 的順序，不含 UNSPECIFIED。 */
export const TARGET_KIND_OPTIONS: readonly ReportTargetKind[] = [
	ReportTargetKind.POST,
	ReportTargetKind.COMMENT,
	ReportTargetKind.FORUM_ENTRY,
	ReportTargetKind.BUYER,
	ReportTargetKind.REVIEW,
	ReportTargetKind.CHAT_MESSAGE,
	ReportTargetKind.REVIEW_REPLY,
];

export function targetKindText(kind: ReportTargetKind): string {
	return staffTextOrUnknown(moderationReportTargetKind(kind));
}

const CASE_STATE_TEXT: Record<
	Exclude<CaseState, CaseState.UNSPECIFIED>,
	string
> = {
	[CaseState.OPEN]: "進行中",
	[CaseState.CLOSED]: "已結案",
};

export function caseStateText(state: CaseState): string {
	return textOf(CASE_STATE_TEXT, state);
}

export function caseStateVariant(state: CaseState): "warning" | "gray" {
	return state === CaseState.OPEN ? "warning" : "gray";
}

const DECISION_TEXT: Record<
	Exclude<CaseDecision, CaseDecision.UNSPECIFIED>,
	string
> = {
	[CaseDecision.VIOLATION]: "違規",
	[CaseDecision.NO_VIOLATION]: "不違規",
};

export function caseDecisionText(decision: CaseDecision): string {
	return textOf(DECISION_TEXT, decision);
}

const ACTION_TEXT: Record<
	Exclude<CaseAction, CaseAction.UNSPECIFIED>,
	string
> = {
	[CaseAction.HIDE]: "隱藏內容",
	[CaseAction.WARN]: "警告買家",
	[CaseAction.BAN]: "停權買家",
	[CaseAction.TAKEDOWN]: "下架店家內容",
};

export function caseActionText(action: CaseAction): string {
	return textOf(ACTION_TEXT, action);
}

export function reportOutcomeText(outcome: ReportOutcome | undefined): string {
	return staffTextOrUnknown(
		outcome === undefined
			? adminModerationCaseReportOutcomeUnset()
			: moderationReportOutcome(outcome),
	);
}

/** 這種對象可以用的處置（與伺服器的 `Action.AllowedOn` 一致）；作者是買家或店家由伺服器向 social 核對。 */
export function actionsFor(kind: ReportTargetKind): readonly CaseAction[] {
	switch (kind) {
		case ReportTargetKind.POST:
			return [CaseAction.TAKEDOWN];
		case ReportTargetKind.COMMENT:
		case ReportTargetKind.FORUM_ENTRY:
			return [
				CaseAction.HIDE,
				CaseAction.WARN,
				CaseAction.BAN,
				CaseAction.TAKEDOWN,
			];
		case ReportTargetKind.BUYER:
			return [CaseAction.WARN, CaseAction.BAN];
		case ReportTargetKind.REVIEW:
			return [CaseAction.HIDE, CaseAction.WARN, CaseAction.BAN];
		case ReportTargetKind.CHAT_MESSAGE:
			return [
				CaseAction.HIDE,
				CaseAction.WARN,
				CaseAction.BAN,
				CaseAction.TAKEDOWN,
			];
		case ReportTargetKind.REVIEW_REPLY:
			return [CaseAction.TAKEDOWN];
		default:
			return [];
	}
}

/** 這種處置接受的認定類別（與伺服器的 `Action.CategoryAllowed` 一致）。 */
export function categoriesFor(action: CaseAction): readonly PolicyCategory[] {
	return action === CaseAction.TAKEDOWN
		? CONTENT_TAKEDOWN_CATEGORIES
		: COMMUNITY_ASSESSABLE_CATEGORIES;
}

/** 檢舉人：買家或店家（`CaseReport.reporter` 恰好設一個）；兩者都沒有（不認得的值）顯示「未知」。 */
export function reporterText(report: CaseReport): string {
	switch (report.reporter.case) {
		case "reporterBuyerId":
			return `檢舉人（買家）${report.reporter.value}`;
		case "reporterStoreId":
			return `檢舉人（店家）${report.reporter.value}`;
		default:
			return `檢舉人 ${unknownText()}`;
	}
}
