import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	type Case,
	CaseCloseReason,
	CaseDecision,
	CaseState,
	type ListCasesRequestSchema,
	type ListReportsRequestSchema,
	type Report,
	ReportState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import { type DeadlineView, deadlineView } from "@/lib/deadline";
import {
	ALL_CATEGORIES,
	ASSESSABLE_CATEGORIES,
	REPORTER_CATEGORIES,
} from "@/lib/policy-category";
import { caseStatusText, reportStatusText } from "./report-text";

// 商家通報清單（/admin/violation-reports）的純邏輯：分頁籤對應的伺服器請求與篩選選項、空狀態文字、欄位文字。
// 篩選與排序都由伺服器決定（待分案與案件內的通報舊到新，其餘新到舊），這裡與畫面都不重排。時間一律由呼叫端傳入（now）。

export type ReportTabKey = "awaiting-triage" | "cases" | "all-reports";

export interface ReportListTab {
	key: ReportTabKey;
	label: string;
	/** 這個分頁的清單來自哪支 RPC。 */
	source: "reports" | "cases";
	/** 類別篩選的選項（第一個選項「所有類別」由畫面加）。 */
	categories: readonly PolicyCategory[];
	/** 沒有任何篩選時的空狀態。 */
	emptyText: string;
	/** 有篩選時的空狀態。 */
	emptyFilteredText: string;
}

/**
 * 清單的分頁：待分案｜案件｜所有通報。類別篩選的選項依那一欄的意思不同：
 * 待分案只有檢舉人選的類別；案件是平台判定類別；所有通報的類別是生效的類別（在案件裡的是平台判定類別，其他是檢舉人選的），可以是全部 13 個。
 */
export const REPORT_LIST_TABS: readonly ReportListTab[] = [
	{
		key: "awaiting-triage",
		label: "待分案",
		source: "reports",
		categories: REPORTER_CATEGORIES,
		emptyText: "沒有待分案的通報",
		emptyFilteredText: "沒有待分案的通報",
	},
	{
		key: "cases",
		label: "案件",
		source: "cases",
		categories: ASSESSABLE_CATEGORIES,
		emptyText: "尚無案件",
		emptyFilteredText: "沒有符合條件的案件",
	},
	{
		key: "all-reports",
		label: "所有通報",
		source: "reports",
		categories: ALL_CATEGORIES,
		emptyText: "尚無通報",
		emptyFilteredText: "沒有符合條件的通報",
	},
];

export interface ReportListFilters {
	/** UNSPECIFIED 是所有類別。 */
	category: PolicyCategory;
	/** 只有案件分頁用；UNSPECIFIED 是全部。 */
	caseState: CaseState;
	/** 只有案件分頁用：只列有還沒結束的詢問的進行中案件（等賣家）；與已結案互斥。 */
	awaitingSeller: boolean;
}

export const NO_FILTERS: ReportListFilters = {
	category: PolicyCategory.UNSPECIFIED,
	caseState: CaseState.UNSPECIFIED,
	awaitingSeller: false,
};

/** 案件的狀態篩選：全部｜進行中｜等賣家｜已結案（3cd D′2 經 rb §3 改寫）。等賣家不是 CaseState 的值，和進行中同時成立。 */
export const CASE_STATE_FILTERS: readonly {
	key: string;
	state: CaseState;
	awaitingSeller: boolean;
	label: string;
}[] = [
	{
		key: "all",
		state: CaseState.UNSPECIFIED,
		awaitingSeller: false,
		label: "全部",
	},
	{
		key: "open",
		state: CaseState.OPEN,
		awaitingSeller: false,
		label: "進行中",
	},
	{
		key: "awaiting-seller",
		state: CaseState.OPEN,
		awaitingSeller: true,
		label: "等賣家",
	},
	{
		key: "closed",
		state: CaseState.CLOSED,
		awaitingSeller: false,
		label: "已結案",
	},
];

/** 目前篩選對應的選項鍵。 */
export function caseStateFilterKey(filters: ReportListFilters): string {
	return (
		CASE_STATE_FILTERS.find(
			(option) =>
				option.state === filters.caseState &&
				option.awaitingSeller === filters.awaitingSeller,
		)?.key ?? "all"
	);
}

/** 這個分頁此刻有沒有篩選（決定空狀態說「尚無」還是「沒有符合條件的」）。 */
export function isFiltered(
	tab: ReportListTab,
	filters: ReportListFilters,
): boolean {
	return (
		filters.category !== PolicyCategory.UNSPECIFIED ||
		(tab.source === "cases" &&
			(filters.caseState !== CaseState.UNSPECIFIED || filters.awaitingSeller))
	);
}

export function emptyTextOf(
	tab: ReportListTab,
	filters: ReportListFilters,
): string {
	return isFiltered(tab, filters) ? tab.emptyFilteredText : tab.emptyText;
}

/** `ListReports` 的請求：待分案分頁只列待分案，所有通報分頁不篩狀態。 */
export function reportListRequest(
	tab: ReportListTab,
	filters: ReportListFilters,
	pageToken: string,
	pageSize: number,
): MessageInitShape<typeof ListReportsRequestSchema> {
	return {
		category: filters.category,
		state:
			tab.key === "awaiting-triage"
				? ReportState.AWAITING_TRIAGE
				: ReportState.UNSPECIFIED,
		pageToken,
		pageSize,
	};
}

/** `ListCases` 的請求。 */
export function caseListRequest(
	filters: ReportListFilters,
	pageToken: string,
	pageSize: number,
): MessageInitShape<typeof ListCasesRequestSchema> {
	return {
		category: filters.category,
		state: filters.caseState,
		awaitingSeller: filters.awaitingSeller,
		pageToken,
		pageSize,
	};
}

/** 店家欄：店名；個資清理之後沒有店名時是店家 ID。 */
export function storeLabel(row: {
	storeName: string;
	storeId: string;
}): string {
	return row.storeName || row.storeId;
}

/** 通報所在的案件欄：「案件 {id}」，沒有案件是「-」。 */
export function caseColumnText(report: Pick<Report, "caseId">): string {
	return report.caseId ? `案件 ${report.caseId}` : "-";
}

/**
 * 分案期限（只在待分案時有；伺服器沒有輸出就是沒有期限或義務已結束，顯示「-」）。開始與期限都是伺服器給的
 * `triageDeadline`，剩下的時間不到整段期限的四分之一時轉 warning。
 */
export function triageDeadline(
	report: Pick<Report, "triageDeadline">,
	now: Date,
): DeadlineView {
	return deadlineView(report.triageDeadline, now);
}

/** 案件列上的標記：有成員通報的補充還沒被看過（伺服器的 unseen_supplement_report_count 大於 0）。 */
export const NEW_SUPPLEMENTS_MARK = "有新的補充";

export type StatusVariant = "success" | "danger" | "warning" | "gray" | "info";

/** 通報的狀態徽章：文字是 A15 的名稱，顏色只是輔助（待分案要注意，採取措施是成功，其他結果是灰色）。 */
export function reportStatusBadge(report: Pick<Report, "state" | "outcome">): {
	label: string;
	variant: StatusVariant;
} {
	const label = reportStatusText(report);
	switch (report.state) {
		case ReportState.AWAITING_TRIAGE:
			return { label, variant: "warning" };
		case ReportState.IN_CASE:
			return { label, variant: "info" };
		case ReportState.RESOLVED:
			return {
				label,
				variant:
					report.outcome === ReportOutcome.ACTION_TAKEN ? "success" : "gray",
			};
		default:
			return { label, variant: "gray" };
	}
}

/** 案件的狀態徽章：進行中要處理，已結案是灰色（決定採取措施的是成功）。 */
export function caseStatusBadge(
	kase: Pick<Case, "state" | "closeReason" | "decision">,
): { label: string; variant: StatusVariant } {
	const label = caseStatusText(kase);
	if (kase.state === CaseState.OPEN) return { label, variant: "info" };
	if (
		kase.state === CaseState.CLOSED &&
		kase.closeReason === CaseCloseReason.DECIDED &&
		kase.decision === CaseDecision.ACTION_TAKEN
	) {
		return { label, variant: "success" };
	}
	return { label, variant: "gray" };
}
