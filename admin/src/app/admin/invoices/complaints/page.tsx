"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { Button, SlaCountdown, StateRow, StatusBadge } from "@/components/UI";
import { COMPLAINT_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import {
	COMPLAINT_SORT_VALUES,
	COMPLAINT_SORTS,
	COMPLAINT_TAB_VALUES,
	COMPLAINT_TABS,
} from "@/features/invoices/complaints";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import type {
	InvoiceComplaintInfo,
	InvoiceEnvelope,
	InvoicePagedData,
} from "@/features/invoices/types";
import { codeLabel, INVOICE_COMPLAINT_STATUS_LABEL } from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const COMPLAINTS_ENDPOINT = "/api/v1/admin/invoices/complaints";
const PAGE_SIZE = 20;

export default function InvoiceComplaintsPage() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const initialTab = searchParams.get("tab") ?? "";
	const [tab, setTab] = useState(
		COMPLAINT_TAB_VALUES.includes(initialTab) ? initialTab : "AWAITING_SELLER",
	);
	const [sort, setSort] = useState(COMPLAINT_SORT_VALUES[0]);
	const [storeIdInput, setStoreIdInput] = useState("");
	const [storeId, setStoreId] = useState("");
	const [orderNoInput, setOrderNoInput] = useState("");
	const [orderNo, setOrderNo] = useState("");
	const [page, setPage] = useState(1);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "complaints", { tab, storeId, orderNo, sort, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				tab,
				sort,
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			if (storeId) params.set("store_id", storeId);
			if (orderNo) params.set("order_no", orderNo);
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<InvoiceComplaintInfo>>
			>(`${COMPLAINTS_ENDPOINT}?${params}`);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const overdueCount = useQuery({
		queryKey: ["invoice", "complaints", "overdueCount"],
		queryFn: async () => {
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<InvoiceComplaintInfo>>
			>(`${COMPLAINTS_ENDPOINT}?tab=OVERDUE&page=1&page_size=1`);
			return Number(res?.data?.total) || 0;
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const hasFilter = storeId !== "" || orderNo !== "";
	const activeTab = COMPLAINT_TABS.find((t) => t.value === tab);

	const changeTab = (next: string) => {
		setTab(next);
		setPage(1);
		router.replace(`/admin/invoices/complaints?tab=${next}`);
	};

	const clearFilters = () => {
		setStoreIdInput("");
		setStoreId("");
		setOrderNoInput("");
		setOrderNo("");
		setPage(1);
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">未收到發票申訴</h1>
					<p className="page-subtitle">
						資料為載入當下的快照，請按「手動更新」重新取得。五個分頁互相包含，筆數不可相加。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						size="sm"
						onClick={() => {
							refetch();
							overdueCount.refetch();
						}}
						loading={isFetching}
					>
						手動更新
					</Button>
				</div>
			</div>

			{(overdueCount.data ?? 0) > 0 ? (
				<p className="invoice-page-notice" role="status">
					<MaterialSymbol name="crisis_alert" />
					<span>
						目前有 {overdueCount.data} 件申訴已逾期。
						<button
							type="button"
							className="invoice-inline-link"
							onClick={() => changeTab("OVERDUE")}
						>
							查看逾期案件
						</button>
					</span>
				</p>
			) : null}

			<div
				className="merchant-filter-tabs"
				role="tablist"
				aria-label="申訴案件分頁"
			>
				{COMPLAINT_TABS.map((item) => (
					<button
						key={item.value}
						type="button"
						role="tab"
						aria-selected={tab === item.value}
						className={`filter-tab ${tab === item.value ? "active" : ""}`}
						onClick={() => changeTab(item.value)}
					>
						{item.label}
					</button>
				))}
			</div>

			{activeTab?.note ? (
				<p className="invoice-tab-note">{activeTab.note}</p>
			) : null}

			<div className="invoice-filter-bar">
				<div className="filter-search">
					<MaterialSymbol name="storefront" />
					<input
						type="text"
						aria-label="依 store_id 篩選"
						placeholder="store_id（按 Enter 套用）"
						value={storeIdInput}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							setStoreIdInput(e.target.value)
						}
						onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => {
							if (e.key === "Enter") {
								setStoreId(storeIdInput.trim());
								setPage(1);
							}
						}}
					/>
				</div>
				<div className="filter-search">
					<MaterialSymbol name="list_alt" />
					<input
						type="text"
						aria-label="依訂單編號篩選"
						placeholder="訂單編號（按 Enter 套用）"
						value={orderNoInput}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							setOrderNoInput(e.target.value)
						}
						onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => {
							if (e.key === "Enter") {
								setOrderNo(orderNoInput.trim());
								setPage(1);
							}
						}}
					/>
				</div>

				<select
					className="filter-select"
					aria-label="排序方式"
					value={sort}
					onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
						setSort(e.target.value);
						setPage(1);
					}}
				>
					{COMPLAINT_SORTS.map((option) => (
						<option key={option.value} value={option.value}>
							{option.label}
						</option>
					))}
				</select>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>案號</th>
							<th>買家／訂單</th>
							<th>賣家</th>
							<th>狀態</th>
							<th>處理期限</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							error={isError ? error : null}
							empty={records.length === 0}
							emptyText={
								hasFilter
									? "沒有符合條件的案件"
									: "目前沒有未收到發票的申訴案件"
							}
							emptyAction={
								hasFilter ? (
									<Button variant="outline" size="sm" onClick={clearFilters}>
										清除篩選
									</Button>
								) : null
							}
							colSpan={6}
							onRetry={() => refetch()}
						>
							{records.map((item) => {
								const overdue = Number(item.overdueBusinessDays);
								const isOverdue =
									item.overdueBusinessDays !== "" &&
									Number.isFinite(overdue) &&
									overdue > 0;
								return (
									<tr
										key={item.complaintId}
										className={isOverdue ? "invoice-row--danger" : ""}
									>
										<td>
											<div className="cell-main">
												{item.caseNo || item.complaintId}
											</div>
											<div className="cell-sub">
												{formatDateTime(item.createdAt)}
											</div>

											{Number(item.reopenCount) > 0 ? (
												<StatusBadge variant="info" size="sm">
													重開第 {item.reopenCount} 次
												</StatusBadge>
											) : null}
										</td>
										<td>
											<div className="cell-main">
												{item.buyerMaskedName || "—"}
											</div>
											<div className="cell-sub">{item.orderNo || "—"}</div>
										</td>
										<td>
											<div className="cell-main">{item.storeId}</div>
										</td>
										<td>
											<StatusBadge
												variant={
													COMPLAINT_STATUS_BADGE_VARIANT[item.status] ?? "gray"
												}
											>
												{codeLabel(INVOICE_COMPLAINT_STATUS_LABEL, item.status)}
											</StatusBadge>
										</td>
										<td>
											<div className="cell-main">
												{formatDateTime(item.dueAt)}
											</div>

											<SlaCountdown
												remainingBusinessDays={item.remainingBusinessDays}
												overdueBusinessDays={item.overdueBusinessDays}
											/>
										</td>
										<td>
											<div className="invoice-table-actions">
												<Button
													variant="outline"
													size="sm"
													onClick={() =>
														router.push(
															`/admin/invoices/complaints/${item.complaintId}`,
														)
													}
												>
													查看案件
												</Button>
											</div>
										</td>
									</tr>
								);
							})}
						</StateRow>
					</tbody>
				</table>
			</div>

			<InvoicePagination
				page={page}
				total={total}
				pageSize={PAGE_SIZE}
				onPage={setPage}
			/>
		</div>
	);
}
