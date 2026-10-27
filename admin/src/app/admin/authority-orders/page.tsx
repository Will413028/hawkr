"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	type AuthorityOrder,
	AuthorityOrderState,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import Link from "next/link";
import { useState } from "react";
import { DeadlineText } from "@/components/UI";
import { deadlineView } from "@/lib/deadline";
import { connectErrorText } from "@/lib/error-text";
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
	AUTHORITY_REVOKE_TEXT,
	authorityKindText,
	namedStoresText,
} from "./authority-text";
import "@/styles/pages/MerchantManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const PAGE_SIZE = 20;

interface Tab {
	key: "open" | "completed" | "revoked";
	label: string;
	state: AuthorityOrderState;
	emptyText: string;
	columns: readonly string[];
}

/** 分頁籤（L3）、欄位（L4、L5）與空狀態（L8）。 */
const TABS: readonly Tab[] = [
	{
		key: "open",
		label: "處理中",
		state: AuthorityOrderState.OPEN,
		emptyText: "沒有處理中的來函",
		columns: ["機關", "文號", "種類", "指名的店家", "期限", "收文時間"],
	},
	{
		key: "completed",
		label: "已完成",
		state: AuthorityOrderState.COMPLETED,
		emptyText: "尚無來函",
		columns: ["機關", "文號", "種類", "指名的店家", "完成時間", "保存到"],
	},
	{
		key: "revoked",
		label: AUTHORITY_REVOKE_TEXT.tab,
		state: AuthorityOrderState.REVOKED,
		emptyText: "尚無來函",
		columns: [
			"機關",
			"文號",
			"種類",
			"指名的店家",
			AUTHORITY_REVOKE_TEXT.timeColumn,
		],
	},
];

function timeCell(
	value: Parameters<typeof timestampDate>[0] | undefined,
): string {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

/**
 * 主管機關來函（清單）：處理中（期限舊到新）｜已完成｜已撤銷登記（收文時間新到舊），排序由伺服器決定，這裡不重排；每 30 秒自動更新。
 */
export default function AuthorityOrdersPage() {
	const [tab, setTab] = useState<Tab>(TABS[0]);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const now = useNow(LIST_REFRESH_MS);
	const orders = useQuery(
		SellerGovernanceAdminService.method.listAuthorityOrders,
		{
			state: tab.state,
			pageSize: PAGE_SIZE,
			pageToken: currentToken(pageTokens),
		},
		{ refetchInterval: LIST_REFRESH_MS },
	);
	const rows: AuthorityOrder[] = orders.data?.orders ?? [];
	const next = orders.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const changeTab = (item: Tab) => {
		setTab(item);
		setPageTokens(firstPage());
	};

	const renderState = () => {
		if (orders.isLoading) {
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
		if (orders.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>{connectErrorText(orders.error, "來函讀取失敗")}</p>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void orders.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="account_balance" />
				<p>{tab.emptyText}</p>
			</div>
		);
	};

	const renderRow = (order: AuthorityOrder) => {
		const deadline = deadlineView(order.deadline, now);
		return (
			<tr key={order.id}>
				<td>{order.agency}</td>
				<td>
					<Link
						className="merchant-name"
						href={`/admin/authority-orders/${order.id}`}
						title={order.documentNo}
					>
						{order.documentNo}
					</Link>
				</td>
				<td>{authorityKindText(order.kind)}</td>
				<td>{namedStoresText(order)}</td>
				{tab.key === "open" ? (
					<>
						<td>
							<DeadlineText display={deadline} title={deadline.dueText} />
						</td>
						<td>
							<span className="merchant-date">
								{timeCell(order.receiveTime)}
							</span>
						</td>
					</>
				) : tab.key === "revoked" ? (
					<td>
						<span className="merchant-date">{timeCell(order.revokeTime)}</span>
					</td>
				) : (
					<>
						<td>
							<span className="merchant-date">
								{timeCell(order.completeTime)}
							</span>
						</td>
						<td>
							<span className="merchant-date">{timeCell(order.holdUntil)}</span>
						</td>
					</>
				)}
			</tr>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">主管機關來函</h1>
					<p className="page-subtitle">
						主管機關的來函：限制或移除商品、調取資料、暫停帳號等。來函指名的帳號或店家，資料會保存到保存期限，不會被關店後的清理刪除。每
						30 秒自動更新。
					</p>
				</div>
				<Link className="btn btn-primary" href="/admin/authority-orders/new">
					登記來函
				</Link>
			</div>

			<div className="merchant-toolbar">
				<div className="merchant-filter-tabs">
					{TABS.map((item) => (
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
							{tab.columns.map((column) => (
								<th key={column}>{column}</th>
							))}
						</tr>
					</thead>
					<tbody>
						{orders.isLoading || orders.error || rows.length === 0 ? (
							<tr>
								<td
									colSpan={tab.columns.length}
									className="merchant-table-state-cell"
								>
									{renderState()}
								</td>
							</tr>
						) : (
							rows.map(renderRow)
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
