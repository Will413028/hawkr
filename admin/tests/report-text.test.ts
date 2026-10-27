import assert from "node:assert/strict";
import test from "node:test";
import {
	CaseCloseReason,
	CaseDecision,
	CaseState,
	ReportState,
	SupplementMatch,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	caseDecisionText,
	caseStatusText,
	reportOutcomeText,
	reportStatusText,
	supplementMatchNotice,
} from "../src/app/admin/violation-reports/report-text";

/**
 * 通報與案件的名稱（admin 的單端表）：契約的每個值都有名稱，UNSPECIFIED 與不認得的值是「未知」；
 * 契約新增一個值而這張表沒補，測試會轉紅，而不是畫面上默默顯示「未知」。
 */

function valuesOf<E extends number>(
	enumObject: Record<string, string | E>,
	unspecified: E,
): E[] {
	return Object.values(enumObject).filter(
		(v): v is E => typeof v === "number" && v !== unspecified,
	);
}

test("結果的名稱：已採取措施｜未違規｜內容不足｜重複｜濫報", () => {
	assert.deepEqual(
		valuesOf(ReportOutcome, ReportOutcome.UNSPECIFIED).map(reportOutcomeText),
		["已採取措施", "未違規", "內容不足", "重複", "濫報"],
	);
	assert.equal(reportOutcomeText(ReportOutcome.UNSPECIFIED), "未知");
	assert.equal(reportOutcomeText(99 as ReportOutcome), "未知");
});

test("決定的名稱：採取措施｜不採取措施", () => {
	assert.deepEqual(
		valuesOf(CaseDecision, CaseDecision.UNSPECIFIED).map(caseDecisionText),
		["採取措施", "不採取措施"],
	);
	assert.equal(caseDecisionText(CaseDecision.UNSPECIFIED), "未知");
});

test("通報的狀態：待分案｜處理中｜{結果}；沒有結果的 RESOLVED 與不認得的值是「未知」", () => {
	assert.equal(
		reportStatusText({
			state: ReportState.AWAITING_TRIAGE,
			outcome: ReportOutcome.UNSPECIFIED,
		}),
		"待分案",
	);
	assert.equal(
		reportStatusText({
			state: ReportState.IN_CASE,
			outcome: ReportOutcome.UNSPECIFIED,
		}),
		"處理中",
	);
	for (const outcome of valuesOf(ReportOutcome, ReportOutcome.UNSPECIFIED)) {
		assert.equal(
			reportStatusText({ state: ReportState.RESOLVED, outcome }),
			reportOutcomeText(outcome),
		);
	}
	assert.equal(
		reportStatusText({
			state: ReportState.RESOLVED,
			outcome: ReportOutcome.UNSPECIFIED,
		}),
		"未知",
	);
	assert.equal(
		reportStatusText({
			state: ReportState.UNSPECIFIED,
			outcome: ReportOutcome.UNSPECIFIED,
		}),
		"未知",
	);
});

test("案件的狀態：進行中｜已結案：採取措施｜已結案：不採取措施｜已結案：沒有通報", () => {
	const none = {
		closeReason: CaseCloseReason.UNSPECIFIED,
		decision: CaseDecision.UNSPECIFIED,
	};
	assert.equal(caseStatusText({ state: CaseState.OPEN, ...none }), "進行中");
	assert.equal(
		caseStatusText({
			state: CaseState.CLOSED,
			closeReason: CaseCloseReason.DECIDED,
			decision: CaseDecision.ACTION_TAKEN,
		}),
		"已結案：採取措施",
	);
	assert.equal(
		caseStatusText({
			state: CaseState.CLOSED,
			closeReason: CaseCloseReason.DECIDED,
			decision: CaseDecision.NO_ACTION,
		}),
		"已結案：不採取措施",
	);
	assert.equal(
		caseStatusText({
			state: CaseState.CLOSED,
			closeReason: CaseCloseReason.NO_REPORTS,
			decision: CaseDecision.UNSPECIFIED,
		}),
		"已結案：沒有通報",
	);
	assert.equal(
		caseStatusText({ state: CaseState.CLOSED, ...none }),
		"已結案：未知",
	);
	assert.equal(
		caseStatusText({ state: CaseState.UNSPECIFIED, ...none }),
		"未知",
	);
});

test("匿名補充的比對提醒：電話或聯絡信箱其中一個相同才提醒；兩者都相同與登入者的補充不提醒", () => {
	assert.equal(
		supplementMatchNotice(SupplementMatch.PHONE),
		"以電話比對到這筆通報的匿名送出，送出者不一定是原檢舉人。",
	);
	assert.equal(
		supplementMatchNotice(SupplementMatch.EMAIL),
		"以聯絡信箱比對到這筆通報的匿名送出，送出者不一定是原檢舉人。",
	);
	assert.equal(supplementMatchNotice(SupplementMatch.BOTH), undefined);
	assert.equal(supplementMatchNotice(SupplementMatch.UNSPECIFIED), undefined);
});
