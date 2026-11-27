"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import { INVOICE_OPERATION_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import { JOB_TAB_VALUES, JOB_TABS } from "@/features/invoices/jobs";
import type {
	InvoiceEnvelope,
	InvoiceJobInfo,
	InvoicePagedData,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_MANUAL_REASON_LABEL,
	INVOICE_OPERATION_STATUS_LABEL,
	INVOICE_OPERATION_TYPE_LABEL,
	toOptions,
} from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";


const JOBS_ENDPOINT = "/api/v1/admin/invoices/jobs";
const PAGE_SIZE = 20;

/**
 * 一個分頁一個子元件、一個獨立 `useQuery`。
 *
 * 做成單一元件切 `tab` state 會讓六組篩選／分頁互相污染（切到「需人工」時還帶著
 * 上一頁的訂單編號與第 3 頁）。ADM 既有的 `anomalies` 就是這個結構。
 */
function JobsTable({ tab }: { tab: string }) {
	const router = useRouter();
	const [storeIdInput, setStoreIdInput] = useState("");
	const [storeId, setStoreId] = useState("");
	const [orderNoInput, setOrderNoInput] = useState("");
	const [orderNo, setOrderNo] = useState("");
	const [manualReason, setManualReason] = useState("");
	const [page, setPage] = useState(1);

	const isNeedsManual = tab === "NEEDS_MANUAL";

	const { data, isLoading, isError, error, refetch } = useQuery({
		queryKey: [
			"invoice",
			"jobs",
			{ tab, storeId, orderNo, manualReason, page },
		],
		queryFn: async () => {
			const params = new URLSearchParams({
				tab,
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			if (storeId) params.set("store_id", storeId);
			if (orderNo) params.set("order_no", orderNo);
			if (isNeedsManual && manualReason) {
				params.set("manual_reason", manualReason);
			}
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<InvoiceJobInfo>>
			>(`${JOBS_ENDPOINT}?${params}`);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const hasFilter = storeId !== "" || orderNo !== "" || manualReason !== "";

	const clearFilters = () => {
		setStoreIdInput("");
		setStoreId("");
		setOrderNoInput("");
		setOrderNo("");
		setManualReason("");
		setPage(1);
	};

	const applyText = (setter: (v: string) => void, value: string) => {
		setter(value.trim());
		setPage(1);
	};

	return (
		<>
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
							if (e.key === "Enter") applyText(setStoreId, storeIdInput);
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
							if (e.key === "Enter") applyText(setOrderNo, orderNoInput);
						}}
					/>
				</div>
				{isNeedsManual ? (
					<select
						className="filter-select"
						aria-label="人工處理理由篩選"
						value={manualReason}
						onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
							setManualReason(e.target.value);
							setPage(1);
						}}
					>
						<option value="">全部人工處理理由</option>
						{toOptions(INVOICE_MANUAL_REASON_LABEL).map(([value, label]) => (
							<option key={value} value={value}>
								{label}
							</option>
						))}
					</select>
				) : null}
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>訂單</th>
							<th>賣家</th>
							<th>類型</th>
							<th>狀態</th>
							<th>重試</th>
							<th>最近嘗試</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							error={isError ? error : null}
							empty={records.length === 0}
							emptyText={
								hasFilter ? "沒有符合條件的工作" : "目前沒有需要處理的發票工作"
							}
							emptyAction={
								hasFilter ? (
									<Button variant="outline" size="sm" onClick={clearFilters}>
										清除篩選
									</Button>
								) : null
							}
							colSpan={7}
							onRetry={() => refetch()}
						>
							{records.map((job) => (
								<tr key={job.jobId}>
									<td>
										<div className="cell-main">{job.orderNo || "—"}</div>
										<div className="cell-sub">{job.jobId}</div>
									</td>
									<td>

										<div className="cell-main">{job.storeId}</div>
									</td>
									<td>
										{codeLabel(INVOICE_OPERATION_TYPE_LABEL, job.operationType)}
									</td>
									<td>
										<StatusBadge
											variant={
												INVOICE_OPERATION_STATUS_BADGE_VARIANT[job.status] ??
												"gray"
											}
										>
											{codeLabel(INVOICE_OPERATION_STATUS_LABEL, job.status)}
										</StatusBadge>
										{isNeedsManual && job.manualReasonCode ? (
											<div className="cell-sub">
												{codeLabel(
													INVOICE_MANUAL_REASON_LABEL,
													job.manualReasonCode,
												)}
											</div>
										) : null}
									</td>
									<td>
										<div className="cell-main">
											{job.attemptCount || "0"} / {job.maxAttempts || "—"}
										</div>
										<div className="cell-sub">
											{job.nextRetryAt
												? `下次 ${formatDateTime(job.nextRetryAt)}`
												: "—"}
										</div>
									</td>
									<td>{formatDateTime(job.lastAttemptAt)}</td>
									<td>
										<div className="invoice-table-actions">
											<Button
												variant="outline"
												size="sm"
												onClick={() =>
													router.push(`/admin/invoices/jobs/${job.jobId}`)
												}
											>
												查看詳情
											</Button>
										</div>
									</td>
								</tr>
							))}
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
		</>
	);
}

export default function InvoiceJobsPage() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const initialTab = searchParams.get("tab") ?? "";
	const [tab, setTab] = useState(
		JOB_TAB_VALUES.includes(initialTab) ? initialTab : "PENDING",
	);

	const changeTab = (next: string) => {
		setTab(next);
		router.replace(`/admin/invoices/jobs?tab=${next}`);
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">發票工作與異常</h1>
					<p className="page-subtitle">
						資料為載入當下的快照，請切換分頁或重新整理以取得最新狀態。
					</p>
				</div>
			</div>


			<p className="invoice-page-notice" role="note">
				<MaterialSymbol name="local_shipping" />
				<span>
					發票開立與出貨是兩條獨立流程。這一頁的任何狀態都
					<strong>不會</strong>
					影響賣家出貨，也不需要為了出貨而先處理發票。
				</span>
			</p>

			<div
				className="merchant-filter-tabs"
				role="tablist"
				aria-label="發票工作分頁"
			>
				{JOB_TABS.map((item) => (
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


			<JobsTable key={tab} tab={tab} />
		</div>
	);
}
