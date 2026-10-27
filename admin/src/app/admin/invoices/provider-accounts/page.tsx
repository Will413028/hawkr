"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import { PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import type {
	InvoiceEnvelope,
	InvoicePagedData,
	ProviderAccountItem,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
	toOptions,
} from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A04（列表）｜AMEGO 串接監控（58A §A04，端點 42 `GET /api/v1/admin/invoices/provider_accounts`）。
//
// 唯讀頁。三件刻意不做的事：
//   - **不做自動輪詢**：發票端點與該管理員其他請求共用同一個限流桶，`refetchInterval`
//     會吃掉額度並連帶打壞同頁其他請求。一律「手動更新」鍵 ＋ 副標明寫快照語意。
//   - **不做表頭排序**：端點 42 沒有 sort query，做一個只排當前頁的前端排序會讓人
//     以為看到的是全域排序。
//   - **不顯示 AppKey 或憑證指紋**：完整 AppKey 沒有任何 RPC 回傳（型別上不存在），
//     `credentialFingerprint` 也不在 DTO 上——外露 HMAC 指紋是 58 §8 沒有裁決過的
//     暴露面，畫面規格不能單方面決定。列表只印後四碼。

const PROVIDER_ACCOUNTS_ENDPOINT = "/api/v1/admin/invoices/provider_accounts";
const PAGE_SIZE = 20;

export default function ProviderAccountsPage() {
	const router = useRouter();
	const [status, setStatus] = useState("");
	const [keywordInput, setKeywordInput] = useState("");
	const [keyword, setKeyword] = useState("");
	const [page, setPage] = useState(1);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "providerAccounts", { status, keyword, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			if (status) params.set("status", status);
			if (keyword) params.set("keyword", keyword);
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<ProviderAccountItem>>
			>(`${PROVIDER_ACCOUNTS_ENDPOINT}?${params}`);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const hasFilter = status !== "" || keyword !== "";

	const clearFilters = () => {
		setStatus("");
		setKeywordInput("");
		setKeyword("");
		setPage(1);
	};

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">AMEGO 串接監控</h1>
					<p className="page-subtitle">
						資料為載入當下的快照，請按「手動更新」重新取得。統一編號是完整 8
						碼（公開營業資訊，非遮罩值）。
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
					<input
						type="text"
						aria-label="搜尋賣家店名或 store_id"
						placeholder="搜尋店名 / store_id（按 Enter 套用）"
						value={keywordInput}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							setKeywordInput(e.target.value)
						}
						onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => {
							if (e.key === "Enter") {
								setKeyword(keywordInput.trim());
								setPage(1);
							}
						}}
					/>
				</div>
				<select
					className="filter-select"
					aria-label="連線狀態篩選"
					value={status}
					onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
						setStatus(e.target.value);
						setPage(1);
					}}
				>
					<option value="">全部連線狀態</option>
					{/* 中文一律來自 CodeMeta 生成物，前端不手抄。 */}
					{toOptions(INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL).map(
						([value, label]) => (
							<option key={value} value={value}>
								{label}
							</option>
						),
					)}
				</select>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>賣家</th>
							<th>統一編號</th>
							<th>連線狀態</th>
							<th>AppKey</th>
							<th>最近驗證</th>
							<th>啟用切點</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							error={isError ? error : null}
							empty={records.length === 0}
							emptyText={
								hasFilter ? "沒有符合條件的賣家" : "目前沒有賣家設定 AMEGO 串接"
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
							{records.map((item) => (
								<tr key={item.storeId}>
									<td>
										<div className="cell-main">
											{item.storeName || item.storeId}
										</div>
										<div className="cell-sub">{item.storeId}</div>
									</td>
									{/* 後端遮好的統編，前端不遮也不還原。要完整 8 碼看 A03。 */}
									<td>{item.sellerTaxId || "—"}</td>
									<td>
										{/*
										 * 沒有 `status || "NOT_STARTED"` 這種 fallback（陷阱 T16）：
										 * 後端刻意區分 NOT_STARTED（上線當天的全體賣家）／
										 * NOT_CONFIGURED（已建立未填憑證）／CONFIG_ERROR（真正的配置錯誤），
										 * 壓成兩態會讓既有賣家在上線當天全被標成配置錯誤。
										 */}
										<StatusBadge
											variant={
												PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[item.status] ??
												"gray"
											}
										>
											{codeLabel(
												INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
												item.status,
											)}
										</StatusBadge>
									</td>
									<td>
										{item.credentialLast4 ? (
											<span className="invoice-secret">
												••••••••{item.credentialLast4}
											</span>
										) : (
											"—"
										)}
									</td>
									<td>{formatDateTime(item.lastVerifiedAt)}</td>
									<td>
										{item.automationCutoverAt
											? formatDateTime(item.automationCutoverAt)
											: "尚未啟用"}
									</td>
									<td>
										<div className="invoice-table-actions">
											<Button
												variant="outline"
												size="sm"
												onClick={() =>
													router.push(
														`/admin/invoices/provider-accounts/${item.storeId}`,
													)
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
		</div>
	);
}
