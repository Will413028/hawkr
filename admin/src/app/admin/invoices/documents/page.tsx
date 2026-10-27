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

// A12｜全平台發票文件查詢（58A §A12，端點 47
// `GET /api/v1/admin/invoices/documents`）。
//
// A01 的下鑽、A13 的父頁。**完全唯讀**：
//   - 沒有任何寫入請求。重寄通知刻意留在 A13 而不放在列尾——逐列出現一顆會發信的鍵
//     是一個誤點就寄出的設計。
//   - **不做跨賣家金額加總**：加總是對帳語意，而月結匯出已移出 Phase 1。給一個沒有
//     匯出、也沒有對帳定義的總和，只會被當成帳用。
//   - **不做表頭排序**（端點 47 無 `sort` query）、**不做自動輪詢**（tierDefault 限流桶
//     與該管理員的其他請求共用額度）。
//   - **不畫下載鍵**，即使 `fileAvailable === true`：列表 DTO 沒有 `downloadPath`
//     （那一欄只在端點 48 的回應上），自行拼接是陷阱 T22——拼出來的是一條死鍵，而前端
//     沒有任何東西會擋。下載一律走 A13。
//
// ## 與 58A §A12 的三處欄位落差（全部是「規格有、契約沒有」，詳見 types.ts）
//
//   1. `storeId` / `storeName`　→ 端點 47 回的是 `[]SellerInvoiceDocumentItem` 本身，
//      沒有 admin 專屬型別 ⇒ 「訂單／賣家」只畫得出訂單。賣家維度只剩 store_id 篩選框。
//   2. `paidAt`　　　　　　　　→ **已修**（backend#3019）：取 `invoice_documents.paid_at`。
//      2026-09-02 之前建的文件該欄是 NULL ⇒ 那些列仍是空字串，那是誠實的「不知道」。
//   3. `shipmentStatus`　　　　→ **已修**（同上），由 invoice-service 逐頁向 shipping-service
//      批次查。58A 要求它「獨立一欄、獨立 badge，禁止與發票狀態合成綜合狀態」⇒ 本頁
//      畫成獨立的第二顆 badge，絕不與發票狀態合成。
//
//      🔴 **空字串只有在 `shipmentStatusDegraded === false` 時才等於「還沒出貨」。**
//      這正是這一欄原本不敢畫的理由：shipping-service 抖動時每一列都會是空字串，
//      而空白會被讀成「還沒出貨」——一句我們無從得知的話。降級時本頁改畫「暫時無法取得」
//      並在表格上方掛一條說明，不讓任何一列說謊。
//
// ## `invoiceNumber` 不遮罩，而且不得由前端補遮
//
// 58A 兩處寫 `invoiceNoMasked`（「例 AB1000****」），而 proto 逐字寫「完整值，**不遮罩**，
// §8.1.3 (b) 判為 O 類」，invoice-service 更註明「§8.1.3(c) 裁定 1 **逐字推翻** img-05
// 的遮罩畫法」。發票號碼是對獎／報帳／申訴的唯一憑據，同時印在發票上、寄在通知信裡、
// 存在載具裡。⇒ 原樣顯示（§0.4 ③：遮罩一律後端做）。

// shipmentStatusCell 只做「三態 → JSX」的對應；**判定本身在
// features/invoices/documents.ts 的 shipmentStatusDisplay**，那裡有單元測試守著
// 「degraded 與 none 不得塌成同一格」——寫在 .tsx 裡就只驗得到字串出現與否，
// 而那種斷言在把 if (degraded) 整段刪掉之後照樣是綠的（實測過）。
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
	// deep-link：A01 頁尾與未來的 KPI 下鑽都可能帶著條件進來。
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
				// 預設 false：舊版後端不回這個 key ⇒ undefined ⇒ 退回「空字串就是沒出貨」，
				// 也就是這一波之前的行為。不預設 true，否則每一頁都會掛降級提示。
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
				{/* type="date" 而非 datetime-local：端點 47 的契約是 YYYY-MM-DD，
				    理由全文見 features/invoices/date-range.ts。 */}
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
										{/* 完整值，前端不還原也不補遮（見檔頭）。 */}
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
									{/* 🔴 獨立的第二顆 badge，**絕不與發票狀態合成綜合狀態**（58A §A12）：
									    合成之後前端就無法履行「發票狀態不影響出貨」這句話。 */}
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
