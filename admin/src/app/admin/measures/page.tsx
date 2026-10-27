"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import Link from "next/link";
import { useState } from "react";
import { DeadlineLine, StatusBadge } from "@/components/UI";
import { errorMessage } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import {
	measureBasisLabel,
	measureKindLabel,
	measureStateBadge,
} from "../merchants/[id]/measure";
import {
	MEASURE_LIST_TABS,
	type MeasureListTab,
	measureListRequest,
	pendingRequestOf,
	requestKindLabel,
	rowWaitingText,
	storeColumnText,
} from "./measure-inbox";
import "@/styles/pages/MerchantManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const PAGE_SIZE = 20;
const COLUMN_COUNT = 7;

/** 措施收件匣：待裁定｜複審到期｜生效中｜全部。篩選與排序都由伺服器決定，這裡不重排。 */
export default function MeasureInbox() {
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [tab, setTab] = useState<MeasureListTab>(MEASURE_LIST_TABS[0]);

	const list = useQuery(
		SellerGovernanceAdminService.method.listMeasures,
		measureListRequest(tab, currentToken(pageTokens), PAGE_SIZE),
	);
	const measures = list.data?.measures ?? [];
	const nextPageToken = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);
	const now = useNow(LIST_REFRESH_MS);

	const changeTab = (next: MeasureListTab) => {
		setTab(next);
		setPageTokens(firstPage());
	};

	const renderTableState = () => {
		if (list.isLoading) {
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
		if (list.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>{errorMessage(list.error)}</p>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void list.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="verified_user" />
				<p>{tab.emptyText}</p>
			</div>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">措施與申覆</h1>
				</div>
			</div>

			<div className="merchant-toolbar">
				<div className="merchant-filter-tabs">
					{MEASURE_LIST_TABS.map((item) => (
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
			</div>

			<div className="merchant-table-container">
				<table className="merchant-table">
					<thead>
						<tr>
							<th>店家</th>
							<th>種類</th>
							<th>依據</th>
							<th>請求</th>
							<th>已等候</th>
							<th>狀態</th>
							<th>建立時間</th>
						</tr>
					</thead>
					<tbody>
						{list.isLoading || list.error || measures.length === 0 ? (
							<tr>
								<td
									colSpan={COLUMN_COUNT}
									className="merchant-table-state-cell"
								>
									{renderTableState()}
								</td>
							</tr>
						) : (
							measures.map((m) => {
								const badge = measureStateBadge(m.state, m.kind);
								const pending = pendingRequestOf(m);
								return (
									<tr key={m.id}>
										<td>
											<Link
												className="merchant-name"
												href={`/admin/measures/${m.id}`}
												title={m.storeName}
											>
												{storeColumnText(m)}
											</Link>
											{tab.key === "review-due" && (
												<div className="merchant-date">
													<DeadlineLine
														label="複審期限"
														deadline={m.reviewDeadline}
														now={now}
													/>
												</div>
											)}
										</td>
										<td>{measureKindLabel(m.kind)}</td>
										<td>{measureBasisLabel(m.basis)}</td>
										<td>{pending ? requestKindLabel(pending.kind) : "-"}</td>
										<td>{rowWaitingText(m, now) ?? "-"}</td>
										<td>
											<StatusBadge variant={badge.variant}>
												{badge.label}
											</StatusBadge>
										</td>
										<td>
											<span className="merchant-date">
												{m.createTime
													? formatDateTimeShort(timestampDate(m.createTime))
													: "-"}
											</span>
										</td>
									</tr>
								);
							})
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
