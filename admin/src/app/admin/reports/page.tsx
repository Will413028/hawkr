"use client";

import { useQuery } from "@connectrpc/connect-query";
import {
	type CaseState,
	ModerationAdminService,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import type { ReportTargetKind } from "@hawkr/api/gen/hawkr/moderation/v1alpha1/moderation_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { useState } from "react";
import { StatusBadge } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import {
	CASE_STATE_FILTERS,
	type CaseFilters,
	caseRow,
	DEFAULT_CASE_FILTERS,
	listCasesRequest,
} from "../moderation/case-list";
import {
	TARGET_KIND_OPTIONS,
	targetKindText,
} from "../moderation/moderation-text";
import "@/styles/pages/MerchantManagement.css";

const COLUMNS = [
	"對象種類",
	"對象",
	"檢舉數",
	"狀態",
	"開案時間",
	"停留時間",
] as const;

/**
 * 社群檢舉案件：一個對象一列，點列進案件詳情處理。篩選是案件狀態（預設進行中）與對象種類，換篩選回第一頁；
 * 翻頁只有上一頁與下一頁（keyset），「上一頁」退回走過的前一個 page token。
 */
export default function ReportsAdminPage() {
	const [filters, setFilters] = useState<CaseFilters>(DEFAULT_CASE_FILTERS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const now = useNow(LIST_REFRESH_MS);
	const cases = useQuery(
		ModerationAdminService.method.listCases,
		listCasesRequest(filters, currentToken(pageTokens)),
	);
	const rows = (cases.data?.cases ?? []).map((kase) => caseRow(kase, now));
	const next = cases.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const change = (patch: Partial<CaseFilters>) => {
		setFilters((f) => ({ ...f, ...patch }));
		setPageTokens(firstPage());
	};

	const renderState = () => {
		if (cases.isLoading) {
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
		if (cases.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>{connectErrorText(cases.error, "案件讀取失敗")}</p>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void cases.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="check_circle" />
				<p>沒有符合條件的案件</p>
			</div>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">社群檢舉</h1>
					<p className="page-subtitle">
						貼文、留言、討論區留言與買家的檢舉，同一個對象的進行中檢舉合成一個案件。
					</p>
				</div>
			</div>

			<div className="merchant-toolbar">
				<select
					aria-label="案件狀態"
					value={filters.state}
					onChange={(e) =>
						change({ state: Number(e.target.value) as CaseState })
					}
				>
					{CASE_STATE_FILTERS.map((option) => (
						<option key={option.state} value={option.state}>
							{option.label}
						</option>
					))}
				</select>
				<select
					aria-label="對象種類"
					value={filters.targetKind}
					onChange={(e) =>
						change({ targetKind: Number(e.target.value) as ReportTargetKind })
					}
				>
					<option value={0}>所有對象</option>
					{TARGET_KIND_OPTIONS.map((kind) => (
						<option key={kind} value={kind}>
							{targetKindText(kind)}
						</option>
					))}
				</select>
			</div>

			<div className="merchant-table-container">
				<table className="merchant-table">
					<thead>
						<tr>
							{COLUMNS.map((column) => (
								<th key={column}>{column}</th>
							))}
						</tr>
					</thead>
					<tbody>
						{cases.isLoading || cases.error || rows.length === 0 ? (
							<tr>
								<td
									colSpan={COLUMNS.length}
									className="merchant-table-state-cell"
								>
									{renderState()}
								</td>
							</tr>
						) : (
							rows.map((row) => (
								<tr key={row.id}>
									<td>
										<Link
											className="merchant-name"
											href={`/admin/moderation/cases/${row.id}`}
										>
											{row.targetKind}
										</Link>
									</td>
									<td>{row.targetId}</td>
									<td>{row.reportCount}</td>
									<td>
										<StatusBadge variant={row.stateVariant}>
											{row.state}
										</StatusBadge>
									</td>
									<td>
										<span className="merchant-date">{row.createTime}</span>
									</td>
									<td>{row.dwell}</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			{(page > 1 || next) && (
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
						onClick={() => setPageTokens((t) => nextPage(t, next))}
						disabled={!next}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}
