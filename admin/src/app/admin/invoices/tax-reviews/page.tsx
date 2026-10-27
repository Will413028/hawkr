"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import {
	deadlineDisplay,
	PENDING_REVIEW_STATUSES,
	TAX_REVIEW_FILTER_VALUES,
	TAX_REVIEW_FILTERS,
} from "@/features/invoices/tax-reviews";
import type {
	InvoiceEnvelope,
	InvoicePagedData,
	TaxReviewItem,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_TAX_RISK_FLAG_LABEL,
	SELLER_TAX_ENTITY_TYPE_LABEL,
	SELLER_TAX_OBLIGATION_LABEL,
} from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A02｜稅務身分審核佇列（58A §A02，端點 38 `GET /api/v1/admin/invoices/tax_reviews`）。
//
// 唯讀頁：只導頁、不寫入。三件刻意不做的事：
//   - **不做批次核准**：58 需求明令「不預設大量核准」。稅務資格是逐案判斷。
//   - **不做表頭排序**：端點 38 沒有 sort query，後端固定依 deadlineAt 遞增。
//     做一個只排當前頁的前端排序，會讓管理員以為看到的是全域排序。
//   - **不做自動輪詢**：一律「手動更新」鍵。
//
// `entityType` 只是顯示欄位，**不得**拿來做任何自動判斷或「建議核准」提示。

const TAX_REVIEWS_ENDPOINT = "/api/v1/admin/invoices/tax_reviews";
const PAGE_SIZE = 20;

export default function TaxReviewQueuePage() {
	const router = useRouter();
	const searchParams = useSearchParams();

	// 篩選存在 URL query 而不是 localStorage：連結可以貼給同事、可以加書籤、重整不
	// 失效。localStorage 會讓兩個管理員點同一條連結卻看到不同清單。
	const initialFilter = searchParams.get("filter") ?? "";
	const [filter, setFilter] = useState(
		TAX_REVIEW_FILTER_VALUES.includes(initialFilter) ? initialFilter : "",
	);
	// 兩層 keyword：輸入框即時更新，實際查詢只在 Enter／按搜尋時才換，避免逐鍵 refetch。
	const [keywordInput, setKeywordInput] = useState("");
	const [keyword, setKeyword] = useState("");
	const [page, setPage] = useState(1);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "taxReviews", { filter, keyword, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			if (filter) params.set("filter", filter);
			if (keyword) params.set("keyword", keyword);
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<TaxReviewItem>>
			>(`${TAX_REVIEWS_ENDPOINT}?${params}`);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const hasFilter = filter !== "" || keyword !== "";

	const changeFilter = (next: string) => {
		setFilter(next);
		setPage(1);
		const query = next ? `?filter=${next}` : "";
		router.replace(`/admin/invoices/tax-reviews${query}`);
	};

	const applyKeyword = () => {
		setKeyword(keywordInput.trim());
		setPage(1);
	};

	const clearFilters = () => {
		setKeywordInput("");
		setKeyword("");
		changeFilter("");
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">稅務身分審核</h1>
					<p className="page-subtitle">
						資料為載入當下的快照，請按「手動更新」重新取得。六個篩選為交集視圖，筆數不可相加。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						size="sm"
						onClick={() => refetch()}
						loading={isFetching}
					>
						手動更新
					</Button>
				</div>
			</div>

			<div className="invoice-filter-bar">
				<div className="filter-search">
					<MaterialSymbol name="search" />
					{/* 文案逐字對齊後端述詞（backend #3089 第 16 項）：
					    ListTaxReviewsForAdmin 比對 legal_name／tax_id／issuer_ref_id(store_id)。
					    原文案寫「店名」而後端比的是**法定名稱** —— 店名（store_name）
					    在 seller-service，本服務搜不到，照螢幕上的字去搜必然 0 筆。
					    store_id 那一格是同一批補上的（在那之前述詞裡根本沒有它）。 */}
					<input
						type="text"
						aria-label="搜尋法定名稱、統一編號或 store_id"
						placeholder="搜尋法定名稱 / 統編 / store_id（按 Enter 套用）"
						value={keywordInput}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							setKeywordInput(e.target.value)
						}
						onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => {
							if (e.key === "Enter") applyKeyword();
						}}
					/>
				</div>
				{/* ADM 既有的 filter tab 沒有掛 tablist 語意，這裡是刻意補強。 */}
				<div
					className="merchant-filter-tabs"
					role="tablist"
					aria-label="稅務身分審核篩選"
				>
					{TAX_REVIEW_FILTERS.map((tab) => (
						<button
							key={tab.value || "ALL"}
							type="button"
							role="tab"
							aria-selected={filter === tab.value}
							className={`filter-tab ${filter === tab.value ? "active" : ""}`}
							onClick={() => changeFilter(tab.value)}
						>
							{tab.label}
						</button>
					))}
				</div>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>賣家</th>
							<th>KYC 類型</th>
							<th>申請稅務狀態</th>
							<th>期限</th>
							<th>已延長</th>
							<th>自行開票條款</th>
							<th>收據代製授權</th>
							<th>風險標記</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							error={isError ? error : null}
							empty={records.length === 0}
							emptyText={
								hasFilter ? "沒有符合條件的申請" : "目前沒有待審的稅務身分申請"
							}
							emptyAction={
								hasFilter ? (
									<Button variant="outline" size="sm" onClick={clearFilters}>
										清除篩選
									</Button>
								) : null
							}
							colSpan={9}
							onRetry={() => refetch()}
						>
							{records.map((item) => {
								const deadline = deadlineDisplay(item.remainingDays);
								const isPending = PENDING_REVIEW_STATUSES.includes(
									item.reviewStatus,
								);
								return (
									<tr
										key={item.storeId}
										className={
											deadline.tone ? `invoice-row--${deadline.tone}` : ""
										}
									>
										<td>
											<div className="cell-main">
												{item.storeName || item.storeId}
											</div>
											{/* storeId 是 TSID，永遠以字串處理——轉 number 會截斷精度。 */}
											<div className="cell-sub">{item.storeId}</div>
										</td>
										<td>
											{codeLabel(SELLER_TAX_ENTITY_TYPE_LABEL, item.entityType)}
										</td>
										<td>
											{/* 賣家自述，不是判定結果（判定結果是 reviewStatus）。 */}
											{codeLabel(
												SELLER_TAX_OBLIGATION_LABEL,
												item.claimedStatus,
											)}
										</td>
										<td>
											<div className="cell-main">
												{formatDateTime(item.deadlineAt)}
											</div>
											<div
												className={
													deadline.tone
														? `invoice-deadline--${deadline.tone}`
														: "cell-sub"
												}
											>
												{deadline.text}
											</div>
										</td>
										<td>
											{item.extensionCount === "1" ? (
												<StatusBadge variant="info">已延長</StatusBadge>
											) : null}
										</td>
										<td>
											<StatusBadge
												variant={item.selfIssueAccepted ? "success" : "gray"}
											>
												{item.selfIssueAccepted ? "已接受" : "未接受"}
											</StatusBadge>
										</td>
										{/*
										 * 收據代製授權：T08「核定免用統一發票」的前置條件。
										 * 未授權時點進 A03 只會看到一顆 disabled 的按鈕，
										 * 所以要在佇列上就看得出來。
										 */}
										<td>
											<StatusBadge
												variant={item.receiptAuthorized ? "success" : "warning"}
											>
												{item.receiptAuthorized ? "已授權" : "尚未授權"}
											</StatusBadge>
										</td>
										<td>
											<div className="invoice-risk-flags">
												{item.riskFlags.map((flag) => (
													<StatusBadge key={flag} variant="danger" size="sm">
														{codeLabel(INVOICE_TAX_RISK_FLAG_LABEL, flag)}
													</StatusBadge>
												))}
											</div>
										</td>
										<td>
											<div className="invoice-table-actions">
												<Button
													variant="outline"
													size="sm"
													onClick={() =>
														router.push(
															`/admin/invoices/tax-reviews/${item.storeId}`,
														)
													}
												>
													{isPending ? "開始審核" : "查看詳情"}
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
