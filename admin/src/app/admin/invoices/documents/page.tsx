"use client";

import { useQuery } from "@tanstack/react-query";
import { useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import {
	DOCUMENT_STATUS_BADGE_VARIANT,
	FULFILLMENT_MODE_BADGE_VARIANT,
} from "@/features/invoices/badge-variants";
import InvoicePagination from "@/features/invoices/components/InvoicePagination";
import { checkDateRange } from "@/features/invoices/date-range";
import {
	fileUnavailableText,
	shipmentStatusDisplay,
} from "@/features/invoices/documents";
import type {
	InvoiceDocumentsPagedData,
	InvoiceEnvelope,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_DOCUMENT_STATUS_LABEL,
	INVOICE_FULFILLMENT_MODE_LABEL,
	toOptions,
} from "@/lib/codemeta";
import { formatAmount, formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";


function shipmentStatusCell(code: string, degraded: boolean) {
	const view = shipmentStatusDisplay(code, degraded);
	if (view.kind === "degraded") {
		return (
			<span title="物流服務暫時不可用，本欄非「未出貨」">暫時無法取得</span>
		);
	}
	if (view.kind === "none") {
		return "—";
	}
	return <StatusBadge variant="gray">{view.label}</StatusBadge>;
}

const DOCUMENTS_ENDPOINT = "/api/v1/admin/invoices/documents";
const PAGE_SIZE = 20;

interface FilterDraft {
	storeId: string;
	orderNo: string;
	status: string;
	mode: string;
	dateFrom: string;
	dateTo: string;
}

const EMPTY_FILTERS: FilterDraft = {
	storeId: "",
	orderNo: "",
	status: "",
	mode: "",
	dateFrom: "",
	dateTo: "",
};

const isBlank = (f: FilterDraft): boolean =>
	Object.values(f).every((value) => value === "");

export default function InvoiceDocumentsPage() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const seeded: FilterDraft = {
		storeId: searchParams.get("store_id") ?? "",
		orderNo: searchParams.get("order_no") ?? "",
		status: searchParams.get("status") ?? "",
		mode: searchParams.get("mode") ?? "",
		dateFrom: searchParams.get("date_from") ?? "",
		dateTo: searchParams.get("date_to") ?? "",
	};
	const [draft, setDraft] = useState<FilterDraft>(seeded);
	const [applied, setApplied] = useState<FilterDraft>(seeded);
	const [filterError, setFilterError] = useState("");
	const [page, setPage] = useState(1);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "documents", { ...applied, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			const wire: Array<[string, string]> = [
				["store_id", applied.storeId],
				["order_no", applied.orderNo],
				["status", applied.status],
				["mode", applied.mode],
				["date_from", applied.dateFrom],
				["date_to", applied.dateTo],
			];
			for (const [key, value] of wire) {
				if (value !== "") params.set(key, value);
			}
			const res = await api.get<InvoiceEnvelope<InvoiceDocumentsPagedData>>(
				`${DOCUMENTS_ENDPOINT}?${params}`,
			);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
				shipmentStatusDegraded: res?.data?.shipmentStatusDegraded === true,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const shipmentDegraded = data?.shipmentStatusDegraded ?? false;
	const hasFilters = !isBlank(applied);
	const canReset = hasFilters || !isBlank(draft);

	const applyFilters = (event: React.FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		const check = checkDateRange(draft.dateFrom, draft.dateTo);
		if (!check.ok) {
			setFilterError(check.message);
			return;
		}
		setFilterError("");
		setApplied(draft);
		setPage(1);
	};

	const resetFilters = () => {
		setDraft(EMPTY_FILTERS);
		setApplied(EMPTY_FILTERS);
		setFilterError("");
		setPage(1);
	};

	const field = (key: keyof FilterDraft) => (value: string) =>
		setDraft((current) => ({ ...current, [key]: value }));

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">發票文件查詢</h1>
					<p className="page-subtitle">
						依<strong>開立時間</strong>由新到舊排列。資料為載入當下的快照。
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

			<form className="adm-audit-filters" onSubmit={applyFilters}>
				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-store">賣家 store_id</label>
					<input
						id="invoice-doc-store"
						value={draft.storeId}
						inputMode="numeric"
						autoComplete="off"
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							field("storeId")(e.target.value)
						}
					/>
				</div>
				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-order">訂單編號（精確比對）</label>
					<input
						id="invoice-doc-order"
						value={draft.orderNo}
						autoComplete="off"
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							field("orderNo")(e.target.value)
						}
					/>
				</div>
				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-status">狀態</label>
					<select
						id="invoice-doc-status"
						value={draft.status}
						onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
							field("status")(e.target.value)
						}
					>
						<option value="">全部狀態</option>
						{toOptions(INVOICE_DOCUMENT_STATUS_LABEL).map(([value, label]) => (
							<option key={value} value={value}>
								{label}
							</option>
						))}
					</select>
				</div>
				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-mode">開票模式</label>
					<select
						id="invoice-doc-mode"
						value={draft.mode}
						onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
							field("mode")(e.target.value)
						}
					>
						<option value="">全部模式</option>
						{toOptions(INVOICE_FULFILLMENT_MODE_LABEL).map(([value, label]) => (
							<option key={value} value={value}>
								{label}
							</option>
						))}
					</select>
				</div>

				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-from">開立起始日期</label>
					<input
						id="invoice-doc-from"
						type="date"
						value={draft.dateFrom}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							field("dateFrom")(e.target.value)
						}
					/>
				</div>
				<div className="adm-operation-field">
					<label htmlFor="invoice-doc-to">開立結束日期</label>
					<input
						id="invoice-doc-to"
						type="date"
						value={draft.dateTo}
						onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
							field("dateTo")(e.target.value)
						}
					/>
				</div>
				<Button type="submit">套用篩選</Button>
				<Button
					variant="outline"
					type="button"
					onClick={resetFilters}
					disabled={!canReset}
				>
					重設
				</Button>
			</form>

			{filterError ? (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{filterError}</div>
				</div>
			) : null}

			<p className="adm-audit-summary-note">
				日期區間比對的是<strong>開立時間</strong>（`issuedAt`），依 UTC+8
				日界計算。
			</p>

			{shipmentDegraded && (
				<p className="adm-audit-summary-note" role="status">
					⚠️
					出貨狀態暫時無法取得（物流服務不可用或逾時），本頁「出貨狀態」欄一律顯示「暫時無法取得」。
					<strong>這不代表這些訂單還沒出貨</strong>——發票資料本身不受影響。
				</p>
			)}

			<div className="table-container">
				<table className="data-table">
					<caption className="sr-only">
						全平台發票文件，共 {total} 筆，依開立時間由新到舊排列
					</caption>
					<thead>
						<tr>
							<th scope="col">訂單</th>
							<th scope="col">發票號碼</th>
							<th scope="col">類型</th>
							<th scope="col">模式</th>
							<th scope="col">狀態</th>
							<th scope="col">出貨狀態</th>
							<th scope="col">付款時間</th>
							<th scope="col">開立時間</th>
							<th scope="col">金額</th>
							<th scope="col">操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							error={isError ? error : null}
							empty={records.length === 0}
							emptyText={
								hasFilters
									? "沒有符合條件的發票文件"
									: "目前平台上還沒有任何發票文件"
							}
							emptyAction={
								hasFilters ? (
									<Button variant="outline" size="sm" onClick={resetFilters}>
										清除篩選
									</Button>
								) : null
							}
							colSpan={10}
							onRetry={() => refetch()}
						>
							{records.map((row) => (
								<tr key={row.summary.documentId}>
									<td>
										<div className="cell-main">{row.orderNo || "—"}</div>
										<div className="cell-sub">{row.summary.documentId}</div>
									</td>
									<td>

										<div className="cell-main">
											{row.summary.invoiceNumber || "—"}
										</div>
										{row.summary.fileAvailable ? null : (
											<div className="cell-sub">
												{fileUnavailableText(row.summary.fileUnavailableReason)}
											</div>
										)}
									</td>
									<td>
										{codeLabel(
											INVOICE_DOCUMENT_KIND_LABEL,
											row.summary.documentKind,
										)}
									</td>
									<td>
										<StatusBadge
											variant={
												FULFILLMENT_MODE_BADGE_VARIANT[row.invoiceMode] ??
												"gray"
											}
										>
											{codeLabel(
												INVOICE_FULFILLMENT_MODE_LABEL,
												row.invoiceMode,
											)}
										</StatusBadge>
									</td>
									<td>
										<StatusBadge
											variant={
												DOCUMENT_STATUS_BADGE_VARIANT[row.summary.status] ??
												"gray"
											}
										>
											{codeLabel(
												INVOICE_DOCUMENT_STATUS_LABEL,
												row.summary.status,
											)}
										</StatusBadge>
									</td>

									<td>
										{shipmentStatusCell(row.shipmentStatus, shipmentDegraded)}
									</td>
									<td>{row.paidAt ? formatDateTime(row.paidAt) : "—"}</td>
									<td>{formatDateTime(row.summary.issuedAt)}</td>
									<td>{formatAmount(row.summary.totalAmount)}</td>
									<td>
										<div className="invoice-table-actions">
											<Button
												variant="outline"
												size="sm"
												onClick={() =>
													router.push(
														`/admin/invoices/documents/${encodeURIComponent(row.summary.documentId)}`,
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
