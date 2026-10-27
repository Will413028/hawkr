import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import {
	CaseCloseReason,
	CaseDecision,
	CaseState,
	ListCasesRequestSchema,
	ListReportsRequestSchema,
	ReportState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import {
	CASE_STATE_FILTERS,
	caseColumnText,
	caseListRequest,
	caseStateFilterKey,
	caseStatusBadge,
	emptyTextOf,
	isFiltered,
	NEW_SUPPLEMENTS_MARK,
	NO_FILTERS,
	REPORT_LIST_TABS,
	reportListRequest,
	reportStatusBadge,
	storeLabel,
	triageDeadline,
} from "../src/app/admin/violation-reports/report-list";
import {
	ALL_CATEGORIES,
	ASSESSABLE_CATEGORIES,
	REPORTER_CATEGORIES,
} from "../src/lib/policy-category";
import { report, T0, ts } from "./fixtures/report-fixtures";

/**
 * 商家通報清單的純邏輯：三個分頁各自送給伺服器的請求與篩選、類別選項、空狀態文字、欄位文字。
 */

const [AWAITING, CASES, ALL] = REPORT_LIST_TABS;

test("三個分頁：待分案｜案件｜所有通報，各自的來源與類別選項（檢舉人可選的 12 類、平台判定的 12 類、全部 13 類）", () => {
	assert.deepEqual(
		REPORT_LIST_TABS.map((tab) => [tab.key, tab.label, tab.source]),
		[
			["awaiting-triage", "待分案", "reports"],
			["cases", "案件", "cases"],
			["all-reports", "所有通報", "reports"],
		],
	);
	assert.equal(AWAITING.categories, REPORTER_CATEGORIES);
	assert.equal(CASES.categories, ASSESSABLE_CATEGORIES);
	assert.equal(ALL.categories, ALL_CATEGORIES);
	assert.equal(AWAITING.categories.length, 12);
	assert.equal(CASES.categories.length, 12);
	assert.equal(ALL.categories.length, 13);
});

test("待分案分頁：ListReports 只列 AWAITING_TRIAGE；沒有篩選時不帶類別", () => {
	const req = reportListRequest(AWAITING, NO_FILTERS, "", 20);
	assert.deepEqual(req, {
		category: PolicyCategory.UNSPECIFIED,
		state: ReportState.AWAITING_TRIAGE,
		pageToken: "",
		pageSize: 20,
	});
	// 請求要是契約接得住的形狀。
	assert.doesNotThrow(() => create(ListReportsRequestSchema, req));
});

test("所有通報分頁：ListReports 不篩狀態；類別是生效的類別，帶什麼就篩什麼", () => {
	const req = reportListRequest(
		ALL,
		{ ...NO_FILTERS, category: PolicyCategory.PLATFORM_ABUSE },
		"token-2",
		20,
	);
	assert.deepEqual(req, {
		category: PolicyCategory.PLATFORM_ABUSE,
		state: ReportState.UNSPECIFIED,
		pageToken: "token-2",
		pageSize: 20,
	});
});

test("案件分頁：ListCases 帶狀態、等賣家與類別篩選；狀態篩選是 全部｜進行中｜等賣家｜已結案", () => {
	assert.deepEqual(
		CASE_STATE_FILTERS.map((f) => [f.state, f.awaitingSeller, f.label]),
		[
			[CaseState.UNSPECIFIED, false, "全部"],
			[CaseState.OPEN, false, "進行中"],
			[CaseState.OPEN, true, "等賣家"],
			[CaseState.CLOSED, false, "已結案"],
		],
	);
	const req = caseListRequest(
		{
			category: PolicyCategory.FRAUD,
			caseState: CaseState.OPEN,
			awaitingSeller: true,
		},
		"t",
		20,
	);
	assert.deepEqual(req, {
		category: PolicyCategory.FRAUD,
		state: CaseState.OPEN,
		awaitingSeller: true,
		pageToken: "t",
		pageSize: 20,
	});
	assert.doesNotThrow(() => create(ListCasesRequestSchema, req));
});

test("空狀態：沒有篩選說「尚無」，有篩選說「沒有符合條件的」；待分案只有一句", () => {
	assert.equal(emptyTextOf(AWAITING, NO_FILTERS), "沒有待分案的通報");
	assert.equal(
		emptyTextOf(AWAITING, { ...NO_FILTERS, category: PolicyCategory.FRAUD }),
		"沒有待分案的通報",
	);
	assert.equal(emptyTextOf(CASES, NO_FILTERS), "尚無案件");
	assert.equal(
		emptyTextOf(CASES, { ...NO_FILTERS, caseState: CaseState.CLOSED }),
		"沒有符合條件的案件",
	);
	assert.equal(emptyTextOf(ALL, NO_FILTERS), "尚無通報");
	assert.equal(
		emptyTextOf(ALL, { ...NO_FILTERS, category: PolicyCategory.OTHER }),
		"沒有符合條件的通報",
	);
});

test("有沒有篩選：案件的狀態篩選只算在案件分頁", () => {
	assert.equal(isFiltered(ALL, NO_FILTERS), false);
	assert.equal(
		isFiltered(ALL, { ...NO_FILTERS, caseState: CaseState.OPEN }),
		false,
	);
	assert.equal(
		isFiltered(CASES, { ...NO_FILTERS, caseState: CaseState.OPEN }),
		true,
	);
});

test("等賣家的篩選：選項鍵由（狀態, 等賣家）決定，只有等賣家時也算有篩選", () => {
	const awaiting = {
		...NO_FILTERS,
		caseState: CaseState.OPEN,
		awaitingSeller: true,
	};
	assert.equal(caseStateFilterKey(NO_FILTERS), "all");
	assert.equal(
		caseStateFilterKey({ ...NO_FILTERS, caseState: CaseState.OPEN }),
		"open",
	);
	assert.equal(caseStateFilterKey(awaiting), "awaiting-seller");
	assert.equal(
		caseStateFilterKey({ ...NO_FILTERS, caseState: CaseState.CLOSED }),
		"closed",
	);
	assert.equal(isFiltered(CASES, awaiting), true);
	assert.equal(isFiltered(ALL, awaiting), false);
});

test("店家欄：店名；個資清理之後沒有店名是店家 ID", () => {
	assert.equal(
		storeLabel({ storeName: "阿里山農場", storeId: "100" }),
		"阿里山農場",
	);
	assert.equal(storeLabel({ storeName: "", storeId: "100" }), "100");
});

test("案件欄：「案件 {id}」，沒有案件是「-」", () => {
	assert.equal(caseColumnText({ caseId: "c1" }), "案件 c1");
	assert.equal(caseColumnText({ caseId: "" }), "-");
});

test("通報的狀態徽章：待分案 warning、處理中 info、已採取措施 success、其他結果 gray；文字是名稱", () => {
	const badge = (state: ReportState, outcome = ReportOutcome.UNSPECIFIED) =>
		reportStatusBadge({ state, outcome });
	assert.deepEqual(badge(ReportState.AWAITING_TRIAGE), {
		label: "待分案",
		variant: "warning",
	});
	assert.deepEqual(badge(ReportState.IN_CASE), {
		label: "處理中",
		variant: "info",
	});
	assert.deepEqual(badge(ReportState.RESOLVED, ReportOutcome.ACTION_TAKEN), {
		label: "已採取措施",
		variant: "success",
	});
	assert.deepEqual(badge(ReportState.RESOLVED, ReportOutcome.ABUSIVE), {
		label: "濫報",
		variant: "gray",
	});
});

test("案件的狀態徽章：進行中 info、決定採取措施 success、其他已結案 gray", () => {
	assert.deepEqual(
		caseStatusBadge({
			state: CaseState.OPEN,
			closeReason: CaseCloseReason.UNSPECIFIED,
			decision: CaseDecision.UNSPECIFIED,
		}),
		{ label: "進行中", variant: "info" },
	);
	assert.deepEqual(
		caseStatusBadge({
			state: CaseState.CLOSED,
			closeReason: CaseCloseReason.DECIDED,
			decision: CaseDecision.ACTION_TAKEN,
		}),
		{ label: "已結案：採取措施", variant: "success" },
	);
	assert.deepEqual(
		caseStatusBadge({
			state: CaseState.CLOSED,
			closeReason: CaseCloseReason.NO_REPORTS,
			decision: CaseDecision.UNSPECIFIED,
		}),
		{ label: "已結案：沒有通報", variant: "gray" },
	);
});

test("分案期限：伺服器沒有輸出就是「-」；有期限時從伺服器給的開始時間起算，剩下不到四分之一轉 warning，逾期 danger", () => {
	const now = (ms: number) => new Date(T0.getTime() + ms);
	const HOUR = 60 * 60 * 1000;
	const due = ts(now(4 * HOUR));
	const shown = (r: ReturnType<typeof report>, at: Date) => {
		const { text, tone } = triageDeadline(r, at);
		return { text, tone };
	};
	assert.deepEqual(shown(report({}), now(0)), { text: "-", tone: "" });
	// 開始時間用 triageDeadline.startTime，不是通報的 createTime（createTime 在這裡故意設得很早）。
	const r = report({
		createTime: ts(now(-100 * HOUR)),
		triageDeadline: { startTime: ts(now(0)), dueTime: due },
	});
	assert.deepEqual(shown(r, now(0)), { text: "剩 4 小時", tone: "" });
	assert.deepEqual(shown(r, now(3 * HOUR + 1)), {
		text: "剩 59 分",
		tone: "warning",
	});
	assert.deepEqual(shown(r, now(5 * HOUR)), {
		text: "已逾期 1 小時",
		tone: "danger",
	});
	assert.equal(triageDeadline(r, now(0)).dueText, "2026/10/18 14:00");
});

test("案件列的標記文字是「有新的補充」（顯示與否看伺服器的 has_unseen_supplements，頁面測試涵蓋）", () => {
	assert.equal(NEW_SUPPLEMENTS_MARK, "有新的補充");
});
