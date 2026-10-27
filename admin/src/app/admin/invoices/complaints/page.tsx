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

// A07｜未收到發票申訴佇列（58A §A07，端點 58 `GET /api/v1/admin/invoices/complaints`）。
//
// 唯讀頁。**佇列頁不放停權按鈕**——不因單一申訴自動停權，停權是 A08 的明確操作，而且
// 走的是 ADM 既有的違規模組。不做批次操作、不做表頭排序（排序走後端 query）、不輪詢。
//
// ⚠️ **五個分頁不互斥，筆數不可相加**（`AWAITING_SELLER` 含 `OVERDUE`、`OVERDUE` 是
// 橫切視圖）。這與 A05 的六分頁**規則相反**，兩處不可互相套用。因此本頁：
//   - tab 上不掛筆數 badge（也因為五支查詢會搶同一個限流桶）
//   - 不顯示合計、不用減法算任何一頁
//
// ⚠️ 逾期橫幅的 N 來自**同一支端點的獨立查詢**（`tab=OVERDUE&page_size=1`，只讀 total），
// 不是從當前分頁的 records 數出來的——人不會先切到「已逾期」才知道有逾期。這是本頁
// 唯一一支額外請求；**不為此改打 A01 的總覽端點**（那是另一個口徑的聚合）。
//
// ## 與 58A 的欄位落差
//
// 58A 的 8 欄裡有兩欄畫不出來：`violationCountIn90Days`（90 天同類申訴）與由它推導的
// 「是否達停權門檻」。端點 58／59 共用的 DTO（Gateway `InvoiceComplaintInfo`）沒有這個
// 欄位——它只出現在**端點 60 的回應**（`SuspensionRecommendation`）。
// ⇒ 本波 6 欄。這是**契約缺口不是取捨**：憑語意造欄名只會渲染 `undefined`（陷阱 T15）。
//
// ⚠️ 2026-09-02 更正：這裡原本寫「而端點 60 未實作」——**端點 60 一直都在**
// （`internal/grpc/complaint.go` 的 `ActOnInvoiceComplaintForAdmin`，路徑
// `POST /admin/invoices/complaints/{id}/actions`），A08 已接上五個處理動作。
// 那兩欄仍然畫不出來，但理由是「它們只在寫入端點的回應裡」而不是「後端沒做」——
// 兩者對讀的人的下一步完全不同：前者去 A08 送出動作就看得到，後者會讓人去等後端。

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

	// 逾期件數：獨立查詢，只讀 total（page_size=1 是為了不把 20 筆資料也撈回來）。
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
				{/* 排序走後端 query param，不做表頭排序：前端排序只排得動當前這一頁，
				    而管理員會以為那是全域排序。 */}
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
											{/* 重開案沒有獨立狀態：狀態回到 OPEN，靠 reopenCount 判別。 */}
											{Number(item.reopenCount) > 0 ? (
												<StatusBadge variant="info" size="sm">
													重開第 {item.reopenCount} 次
												</StatusBadge>
											) : null}
										</td>
										<td>
											{/* buyerMaskedName 是後端遮好的，前端不還原。 */}
											<div className="cell-main">
												{item.buyerMaskedName || "—"}
											</div>
											<div className="cell-sub">{item.orderNo || "—"}</div>
										</td>
										<td>
											{/* 這支端點的 DTO 沒有 storeName，只有 storeId。 */}
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
											{/* 工作天由後端給，前端零計算（只扣週末不扣國定假日會與後端分歧）。 */}
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
