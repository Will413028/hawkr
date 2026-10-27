import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type Case,
	CaseDecision,
	type CaseTargetSchema,
	type ChangeCaseSeverityRequestSchema,
	type CreateCaseNoteRequestSchema,
	type DecideCaseRequestSchema,
	type DecisionOption,
	type MoveReportRequestSchema,
	type OutcomeOption,
	type Report,
	type ResolveReportRequestSchema,
	type TriageReportRequestSchema,
	type TriageTarget,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	MeasureState,
	ViolationSeverity,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import {
	SOURCE_REPORTS_MAX,
	VIOLATION_SEVERITY_OPTIONS,
} from "@/app/admin/merchants/[id]/violation-measure";
import { policyCategoryText } from "@/lib/policy-category";
import { reportOutcomeText } from "./report-text";

// 商家通報與案件的對話框：輸入的驗證、請求組裝、失敗句。能不能做、有哪些選項都來自伺服器（Report.can_*、available_outcomes、
// Case.decision_options 與各個 can_*），這裡不推導；時間一律由呼叫端傳入（now）。

export const RESULT_NOTE_MAX_LENGTH = 2000;
export const SEVERITY_NOTE_MAX_LENGTH = 2000;

export const REPORT_CONFLICT_TEXT =
	"這筆通報剛被更新，頁面已重新載入；請確認最新狀態後再操作。";
export const CASE_CONFLICT_TEXT =
	"這個案件剛被更新，頁面已重新載入；請確認最新狀態後再操作。";

export const TARGET_CASE_CONFLICT_TEXT = "要移入的案件剛被更新，請重新選擇。";
export const CATEGORY_CASE_OPENED_TEXT =
	"這個類別剛有進行中的案件，請重新選擇。";

/**
 * 通報的操作遇到版本衝突時的說明：伺服器在 ErrorInfo.metadata 的 `conflictTarget` 說是通報（`report`）、分案或移動要加入的案件不成立（`target_case`：
 * 案件版本前進或已關閉），還是要開新案件的類別剛有了進行中的案件（`category_case_opened`）。沒有（不認得）時說通報：通報的 etag 是這些操作一定帶的那一個。
 */
export function reportConflictText(
	metadata: Readonly<Record<string, string>>,
): string {
	switch (metadata.conflictTarget) {
		case "target_case":
			return TARGET_CASE_CONFLICT_TEXT;
		case "category_case_opened":
			return CATEGORY_CASE_OPENED_TEXT;
		default:
			return REPORT_CONFLICT_TEXT;
	}
}

export const TRIAGE_FAILED_TEXT =
	"分案失敗，通報沒有變動；請確認網路狀態後再試一次。";
export const MOVE_FAILED_TEXT =
	"移動失敗，通報沒有變動；請確認網路狀態後再試一次。";
export const RESOLVE_FAILED_TEXT =
	"設定結果失敗，通報沒有變動；請確認網路狀態後再試一次。";
export const DECIDE_FAILED_TEXT =
	"結案失敗，案件沒有變動；請確認網路狀態後再試一次。";
export const SEVERITY_FAILED_TEXT =
	"改嚴重度失敗，案件沒有變動；請確認網路狀態後再試一次。";

/** 移動讓原案件因為通報都移走而關閉時，頁面顯示的橫幅。 */
export const SOURCE_CASE_CLOSED_TEXT =
	"原案件的通報都已移走，原案件已自動關閉。";
/** 案件裡有成員通報的補充還沒被看過（頁首與結案對話框共用）；筆數取自伺服器的 `unseen_supplement_report_count`。 */
export function unseenSupplementsText(count: number): string {
	return `有 ${count} 筆通報有還沒看過的補充。`;
}
/** 案件裡的通報列：這筆通報有還沒看過的補充（伺服器的 `has_unseen_supplements`）；打開通報頁看過之後消失。 */
export const NEW_SUPPLEMENTS_BADGE = "新的補充";

function tooLong(value: string, max: number): boolean {
	return goTrimmedRuneCount(value) > max;
}

// ---------------------------------------------------------------------------
// 分案與移動：平台判定類別，以及這家店在那個類別的去處（伺服器隨 GetReport 的 triage_targets 給：加入進行中的案件，或開新案件）

export interface AssessForm {
	category: PolicyCategory;
	severity: ViolationSeverity;
}

export const EMPTY_ASSESS_FORM: AssessForm = {
	category: PolicyCategory.UNSPECIFIED,
	severity: ViolationSeverity.UNSPECIFIED,
};

/** 嚴重度的選項（低到嚴重）。 */
export const SEVERITY_OPTIONS = VIOLATION_SEVERITY_OPTIONS;

/** 這個類別的去處；不在伺服器給的去處裡（選了不能選的類別）回 undefined。 */
export function targetOf(
	targets: readonly TriageTarget[],
	category: PolicyCategory,
): TriageTarget | undefined {
	return targets.find((target) => target.category === category);
}

/** 選了類別之後的提示：加入進行中的案件，或開一個新案件（要選嚴重度）。還沒選類別沒有提示。 */
export function assessNotice(
	category: PolicyCategory,
	targets: readonly TriageTarget[],
): { kind: "join" | "open"; text: string } | undefined {
	const target = targetOf(targets, category);
	if (!target) return undefined;
	const name = policyCategoryText(category);
	if (target.caseId) {
		return {
			kind: "join",
			text: `這家店的「${name}」已有進行中的案件（嚴重度：${violationSeverityText(target.severity)}，${target.caseReportCount} 筆通報），分案後會加入那個案件。`,
		};
	}
	return {
		kind: "open",
		text: `這家店的「${name}」沒有進行中的案件，分案後會開一個新案件，請選擇嚴重度。`,
	};
}

/**
 * 分案與移動共用的檢查：類別必須是伺服器給的去處之一；去處是開新案件時要選嚴重度
 * （加入進行中的案件，嚴重度以案件為準，不送）。回傳第一個錯誤，沒有錯誤回 undefined。
 */
export function validateAssess(
	form: AssessForm,
	targets: readonly TriageTarget[],
): string | undefined {
	const target = targetOf(targets, form.category);
	if (!target) return "請選擇平台判定類別。";
	if (!target.caseId && !SEVERITY_OPTIONS.includes(form.severity)) {
		return "請選擇嚴重度。";
	}
	return undefined;
}

/**
 * 請求的去處：原樣放回伺服器給的意圖，加入帶案件 id 與 etag，開新案件帶嚴重度。呼叫前須先通過 validateAssess。
 * 意圖在送出時由伺服器驗證仍然成立（讀取之後別人開了、關了或改了案件），不成立回版本衝突。
 */
export function buildTarget(
	form: AssessForm,
	targets: readonly TriageTarget[],
): MessageInitShape<typeof CaseTargetSchema> {
	const target = targetOf(targets, form.category);
	if (target?.caseId) {
		return {
			target: {
				case: "join",
				value: { caseId: target.caseId, etag: target.caseEtag },
			},
		};
	}
	return {
		target: { case: "openNew", value: { severity: form.severity } },
	};
}

export function buildTriage(
	report: Pick<Report, "id" | "etag" | "triageTargets">,
	form: AssessForm,
): MessageInitShape<typeof TriageReportRequestSchema> {
	return {
		id: report.id,
		assessedCategory: form.category,
		target: buildTarget(form, report.triageTargets),
		etag: report.etag,
	};
}

export function buildMove(
	report: Pick<Report, "id" | "etag" | "triageTargets">,
	form: AssessForm,
): MessageInitShape<typeof MoveReportRequestSchema> {
	return {
		id: report.id,
		assessedCategory: form.category,
		target: buildTarget(form, report.triageTargets),
		etag: report.etag,
	};
}

// ---------------------------------------------------------------------------
// 設定結果

export interface ResolveForm {
	outcome: ReportOutcome;
	/** 給檢舉人的結果說明（選填）。 */
	note: string;
}

export const EMPTY_RESOLVE_FORM: ResolveForm = {
	outcome: ReportOutcome.UNSPECIFIED,
	note: "",
};

/** 設定這個結果會否定這筆通報（不能再當認定的來源）：伺服器隨每個可設定的結果給（OutcomeOption.negates_source），這裡只查表。 */
export function negatesAsSource(
	options: readonly Pick<OutcomeOption, "outcome" | "negatesSource">[],
	outcome: ReportOutcome,
): boolean {
	return options.some(
		(option) => option.outcome === outcome && option.negatesSource,
	);
}

export const NEGATING_OUTCOME_TEXT =
	"結果是未違規、內容不足或濫報的通報，不能當新認定的來源。";

/** 依據這筆通報的措施（含引用的認定）裡，狀態不是已撤銷的。 */
export function livingSourceMeasures(
	measures: readonly Pick<Measure, "id" | "state">[],
): string[] {
	return measures
		.filter((measure) => measure.state !== MeasureState.REVOKED)
		.map((measure) => measure.id);
}

/** 通報已是措施的依據來源，而且選了會讓它不能再當來源的結果時的提示。 */
export function sourceMeasureNoticeText(
	measureId: string,
	outcome: ReportOutcome,
): string {
	return `這筆通報已是措施 ${measureId} 的依據來源。設為${reportOutcomeText(outcome)}不會撤銷那個措施；如果它只依據這筆通報，請到措施詳情撤銷認定。`;
}

/** 結果必須是伺服器說此刻可以設定的（available_outcomes）；結果說明選填，最多 2000 字。 */
export function validateResolve(
	form: ResolveForm,
	availableOutcomes: readonly Pick<OutcomeOption, "outcome">[],
): string | undefined {
	if (!availableOutcomes.some((option) => option.outcome === form.outcome)) {
		return "請選擇結果。";
	}
	if (tooLong(form.note, RESULT_NOTE_MAX_LENGTH)) {
		return `結果說明最多 ${RESULT_NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

export function buildResolve(
	report: Pick<Report, "id" | "etag">,
	form: ResolveForm,
): MessageInitShape<typeof ResolveReportRequestSchema> {
	return {
		id: report.id,
		outcome: form.outcome,
		note: trimGoSpace(form.note),
		etag: report.etag,
	};
}

// ---------------------------------------------------------------------------
// 結案

export interface DecideForm {
	decision: CaseDecision;
	/** 給檢舉人的結果說明（選填），會帶入還沒有結果的通報。 */
	note: string;
}

export const EMPTY_DECIDE_FORM: DecideForm = {
	decision: CaseDecision.UNSPECIFIED,
	note: "",
};

/** 結案的決定必須是伺服器說此刻可以選的（decision_options）；結果說明選填，最多 2000 字。 */
export function validateDecide(
	form: DecideForm,
	options: readonly Pick<DecisionOption, "decision">[],
): string | undefined {
	if (!options.some((option) => option.decision === form.decision)) {
		return "請選擇結案結果。";
	}
	if (tooLong(form.note, RESULT_NOTE_MAX_LENGTH)) {
		return `結果說明最多 ${RESULT_NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

export function buildDecide(
	kase: Pick<Case, "id" | "etag">,
	form: DecideForm,
): MessageInitShape<typeof DecideCaseRequestSchema> {
	return {
		id: kase.id,
		decision: form.decision,
		note: trimGoSpace(form.note),
		etag: kase.etag,
	};
}

/**
 * 結案後還沒有結果的通報會帶入的結果：伺服器隨每個選項給（default_outcome），這裡只組句子。
 * 沒有這樣的通報時只說結案後不能重開。
 */
export function bringInText(
	unresolvedReportCount: number,
	defaultOutcome: ReportOutcome | undefined,
): string {
	const first = "結案後不能重開。";
	if (
		defaultOutcome === undefined ||
		defaultOutcome === ReportOutcome.UNSPECIFIED ||
		unresolvedReportCount <= 0
	) {
		return first;
	}
	return `${first}還沒有結果的 ${unresolvedReportCount} 筆通報會帶入「${reportOutcomeText(defaultOutcome)}」。`;
}

// ---------------------------------------------------------------------------
// 改嚴重度

export interface SeverityForm {
	severity: ViolationSeverity;
	/** 原因，寫成案件的一則內部註記。 */
	note: string;
}

export const EMPTY_SEVERITY_FORM: SeverityForm = {
	severity: ViolationSeverity.UNSPECIFIED,
	note: "",
};

/** 嚴重度必須是有效的值而且和目前不同；原因必填，去掉前後空白後 1 到 2000 字。 */
export function validateSeverity(
	form: SeverityForm,
	current: ViolationSeverity,
): string | undefined {
	if (!SEVERITY_OPTIONS.includes(form.severity)) return "請選擇嚴重度。";
	if (form.severity === current) return "嚴重度和目前相同。";
	const note = trimGoSpace(form.note);
	if (note.length === 0 || tooLong(note, SEVERITY_NOTE_MAX_LENGTH)) {
		return `請填寫原因，最多 ${SEVERITY_NOTE_MAX_LENGTH} 字。`;
	}
	return undefined;
}

export function buildChangeSeverity(
	kase: Pick<Case, "id" | "etag">,
	form: SeverityForm,
): MessageInitShape<typeof ChangeCaseSeverityRequestSchema> {
	return {
		id: kase.id,
		severity: form.severity,
		note: trimGoSpace(form.note),
		etag: kase.etag,
	};
}

// ---------------------------------------------------------------------------
// 案件的內部註記

export function buildCreateCaseNote(
	caseId: string,
	body: string,
): MessageInitShape<typeof CreateCaseNoteRequestSchema> {
	return { caseId, note: { body: trimGoSpace(body) } };
}

// ---------------------------------------------------------------------------
// 從案件建立措施：預先勾選的來源通報（伺服器篩出可以當來源的通報，見 use-case-reports）

/** 預先勾選的來源通報超過一次能選的上限時，建立措施對話框的說明。 */
export const SOURCE_OVERFLOW_TEXT = `這個案件超過 ${SOURCE_REPORTS_MAX} 筆可以當來源的通報，已預先勾選最早的 ${SOURCE_REPORTS_MAX} 筆。`;
