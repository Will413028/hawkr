"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	type Case,
	type Report,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { skipToken } from "@tanstack/react-query";
import Link from "next/link";
import { useState } from "react";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import { DeadlineText, StatusBadge } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { policyCategoryText } from "@/lib/policy-category";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import {
	CASE_STATE_FILTERS,
	caseColumnText,
	caseListRequest,
	caseStateFilterKey,
	caseStatusBadge,
	emptyTextOf,
	NEW_SUPPLEMENTS_MARK,
	NO_FILTERS,
	REPORT_LIST_TABS,
	type ReportListFilters,
	type ReportListTab,
	reportListRequest,
	reportStatusBadge,
	storeLabel,
	triageDeadline,
} from "./report-list";
import "@/styles/pages/MerchantManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const PAGE_SIZE = 20;

function timeCell(
	value: Parameters<typeof timestampDate>[0] | undefined,
): string {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

/** 商家通報：待分案｜案件｜所有通報。篩選與排序都由伺服器決定，這裡不重排；每 30 秒自動更新。 */
export default function ViolationReportsPage() {
	const [tab, setTab] = useState<ReportListTab>(REPORT_LIST_TABS[0]);
	const [filters, setFilters] = useState<ReportListFilters>(NO_FILTERS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const now = useNow(LIST_REFRESH_MS);
	const token = currentToken(pageTokens);

	const reports = useQuery(
		SellerGovernanceAdminService.method.listReports,
		tab.source === "reports"
			? reportListRequest(tab, filters, token, PAGE_SIZE)
			: skipToken,
		{ refetchInterval: LIST_REFRESH_MS },
	);
	const cases = useQuery(
		SellerGovernanceAdminService.method.listCases,
		tab.source === "cases"
			? caseListRequest(filters, token, PAGE_SIZE)
			: skipToken,
		{ refetchInterval: LIST_REFRESH_MS },
	);

	const active = tab.source === "cases" ? cases : reports;
	const reportRows: Report[] = reports.data?.reports ?? [];
	const caseRows: Case[] = cases.data?.cases ?? [];
	const rowCount = tab.source === "cases" ? caseRows.length : reportRows.length;
	const nextPageToken =
		(tab.source === "cases" ? cases.data : reports.data)?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const changeTab = (next: ReportListTab) => {
		setTab(next);
		setFilters(NO_FILTERS);
		setPageTokens(firstPage());
	};
	const changeFilters = (patch: Partial<ReportListFilters>) => {
		setFilters((f) => ({ ...f, ...patch }));
		setPageTokens(firstPage());
	};

	const columns =
		tab.key === "awaiting-triage"
			? ["店家", "檢舉人選的類別", "分案期限", "建立時間"]
			: tab.key === "cases"
				? ["店家", "類別", "嚴重度", "狀態", "通報數", "開案時間"]
				: ["店家", "類別", "狀態", "案件", "建立時間"];

	const renderTableState = () => {
		if (active.isLoading) {
			return (
				<div className="merchant-table-state">
					<MaterialSymbol
						name="progress_activity"
						className="merchant-table-state-spinner"
					/>
					<p>載入中...</p>
				</div>
			);
		}
		if (active.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>
						{connectErrorText(
							active.error,
							tab.source === "cases" ? "案件讀取失敗" : "通報讀取失敗",
						)}
					</p>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void active.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="verified_user" />
				<p>{emptyTextOf(tab, filters)}</p>
			</div>
		);
	};

	const renderReportRow = (report: Report) => {
		const href = `/admin/violation-reports/${report.id}`;
		const store = (
			<Link className="merchant-name" href={href} title={storeLabel(report)}>
				{storeLabel(report)}
			</Link>
		);
		if (tab.key === "awaiting-triage") {
			const triage = triageDeadline(report, now);
			return (
				<tr key={report.id}>
					<td>{store}</td>
					<td>{policyCategoryText(report.category)}</td>
					<td>
						<DeadlineText display={triage} title={triage.dueText} />
					</td>
					<td>
						<span className="merchant-date">{timeCell(report.createTime)}</span>
					</td>
				</tr>
			);
		}
		const badge = reportStatusBadge(report);
		return (
			<tr key={report.id}>
				<td>{store}</td>
				<td>{policyCategoryText(report.effectiveCategory)}</td>
				<td>
					<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
				</td>
				<td>
					{report.caseId ? (
						<Link href={`/admin/violation-reports/cases/${report.caseId}`}>
							{caseColumnText(report)}
						</Link>
					) : (
						caseColumnText(report)
					)}
				</td>
				<td>
					<span className="merchant-date">{timeCell(report.createTime)}</span>
				</td>
			</tr>
		);
	};

	const renderCaseRow = (kase: Case) => {
		const badge = caseStatusBadge(kase);
		return (
			<tr key={kase.id}>
				<td>
					<Link
						className="merchant-name"
						href={`/admin/violation-reports/cases/${kase.id}`}
						title={storeLabel(kase)}
					>
						{storeLabel(kase)}
					</Link>
				</td>
				<td>{policyCategoryText(kase.category)}</td>
				<td>{violationSeverityText(kase.severity)}</td>
				<td>
					<StatusBadge variant={badge.variant}>{badge.label}</StatusBadge>
					{kase.awaitingSeller && (
						<StatusBadge variant="warning">等賣家</StatusBadge>
					)}
					{kase.unseenSupplementReportCount > 0 && (
						<div className="merchant-date">{NEW_SUPPLEMENTS_MARK}</div>
					)}
				</td>
				<td>{kase.reportCount}</td>
				<td>
					<span className="merchant-date">{timeCell(kase.openTime)}</span>
				</td>
			</tr>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">商家通報</h1>
					<p className="page-subtitle">
						買家與公眾對店家的通報。先分案：同一家店、同一個類別的通報歸到同一個案件；再在案件裡決定是否採取措施。每
						30 秒自動更新。
					</p>
				</div>
			</div>

			<div className="merchant-toolbar">
				<div className="merchant-filter-tabs">
					{REPORT_LIST_TABS.map((item) => (
						<button
							key={item.key}
							type="button"
							className={`filter-tab ${tab.key === item.key ? "active" : ""}`}
							onClick={() => changeTab(item)}
						>
							{item.label}
						</button>
					))}
				</div>
				<select
					className="filter-select"
					aria-label="類別"
					value={filters.category}
					onChange={(e) =>
						changeFilters({
							category: Number(e.target.value) as PolicyCategory,
						})
					}
				>
					<option value={PolicyCategory.UNSPECIFIED}>所有類別</option>
					{tab.categories.map((option) => (
						<option key={option} value={option}>
							{policyCategoryText(option)}
						</option>
					))}
				</select>
				{tab.source === "cases" && (
					<select
						className="filter-select"
						aria-label="狀態"
						value={caseStateFilterKey(filters)}
						onChange={(e) => {
							const option = CASE_STATE_FILTERS.find(
								(item) => item.key === e.target.value,
							);
							if (option)
								changeFilters({
									caseState: option.state,
									awaitingSeller: option.awaitingSeller,
								});
						}}
					>
						{CASE_STATE_FILTERS.map((option) => (
							<option key={option.key} value={option.key}>
								{option.label}
							</option>
						))}
					</select>
				)}
			</div>

			<div className="merchant-table-container">
				<table className="merchant-table">
					<thead>
						<tr>
							{columns.map((column) => (
								<th key={column}>{column}</th>
							))}
						</tr>
					</thead>
					<tbody>
						{active.isLoading || active.error || rowCount === 0 ? (
							<tr>
								<td
									colSpan={columns.length}
									className="merchant-table-state-cell"
								>
									{renderTableState()}
								</td>
							</tr>
						) : tab.source === "cases" ? (
							caseRows.map(renderCaseRow)
						) : (
							reportRows.map(renderReportRow)
						)}
					</tbody>
				</table>
			</div>

			{(page > 1 || nextPageToken) && (
				<div className="merchant-pagination">
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => setPageTokens(previousPage)}
						disabled={page <= 1}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span className="page-info">第 {page} 頁</span>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => setPageTokens((t) => nextPage(t, nextPageToken))}
						disabled={!nextPageToken}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}
