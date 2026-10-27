"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import type React from "react";
import { useState } from "react";
import { Button } from "@/components/UI";
import { auditActionOptionGroups } from "@/features/invoices/audit-logs";
import { checkDateRange } from "@/features/invoices/date-range";
import type {
	InvoiceAuditLogItem,
	InvoiceEnvelope,
	InvoicePagedData,
} from "@/features/invoices/types";
import { codeLabel, INVOICE_OPERATION_TYPE_LABEL } from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { api } from "@/services/api";
import { auditChangeSummary } from "../audit-logs";
import InvoiceAuditDetailDialog from "./InvoiceAuditDetailDialog";

/**
 * A10｜發票稽核紀錄的本體（58A §A10，端點 61
 * `GET /api/v1/admin/invoices/audit_logs`）。
 *
 * ## 為什麼放 `features/` 而不是直接寫在 `page.tsx`
 *
 * ADM 的主流慣例是「一頁一個 `page.tsx`」，發票另外 12 個畫面都照做。這一頁是例外：
 * 篩選（7 軸）＋ 六欄表 ＋ 明細 dialog ＋ 分頁的體量與唯一的 feature 模組先例
 * （`features/delegated-operations`）同級。
 *
 * ## 唯讀，而且是「刻意不畫」而不是「畫成 disabled」
 *
 *   - **不畫下載／匯出鍵**：端點 61 沒有 exports 端點。ADM 既有的
 *     `merchants/page.tsx` 就是一顆 `alert("匯出功能開發中...")` 的假按鈕——它存在很久、
 *     每個看到的人都會按一次，而它從來沒做過任何事。不要再造第二顆。
 *   - **不畫保留天數那一列**：端點 61 的回應**沒有任何 retention 欄位**。寫死「5 年」
 *     等於把法遵文字繞過版本控管（日後改期限時畫面不會跟著改，也不會有東西轉紅）；
 *     退而寫「長期保存」更糟——沒有來源的形容詞比不寫還不如。整列不畫。
 *   - **不做表頭排序**：端點 61 沒有 `sort` query。排序固定 `occurredAt` 遞減，
 *     這件事寫在副標上。
 *   - **不做自動輪詢**：發票端點走 `tierDefault` 限流桶，與該管理員的其他請求共用額度。
 */

const AUDIT_LOGS_ENDPOINT = "/api/v1/admin/invoices/audit_logs";
const PAGE_SIZE = 20;

interface FilterDraft {
	actorAccountId: string;
	storeId: string;
	orderNo: string;
	documentId: string;
	jobId: string;
	operationType: string;
	dateFrom: string;
	dateTo: string;
}

const EMPTY_FILTERS: FilterDraft = {
	actorAccountId: "",
	storeId: "",
	orderNo: "",
	documentId: "",
	jobId: "",
	operationType: "",
	dateFrom: "",
	dateTo: "",
};

const isBlank = (f: FilterDraft): boolean =>
	Object.values(f).every((value) => value === "");

export default function InvoiceAuditConsole({
	initialJobId = "",
}: {
	/** A06 的「查看完整稽核紀錄」帶 `?job_id=` 進來。 */
	initialJobId?: string;
}) {
	const seeded: FilterDraft = { ...EMPTY_FILTERS, jobId: initialJobId };
	// draft / applied 兩層：按「套用篩選」才生效。少了這一層，每打一個字就是一次
	// 查詢，而這些端點共用一個限流桶。
	const [draft, setDraft] = useState<FilterDraft>(seeded);
	const [applied, setApplied] = useState<FilterDraft>(seeded);
	const [filterError, setFilterError] = useState("");
	const [page, setPage] = useState(1);
	const [selectedId, setSelectedId] = useState<string | null>(null);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "auditLogs", { ...applied, page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			const wire: Array<[string, string]> = [
				["actor_account_id", applied.actorAccountId],
				["store_id", applied.storeId],
				["order_no", applied.orderNo],
				["document_id", applied.documentId],
				["job_id", applied.jobId],
				["operation_type", applied.operationType],
				["date_from", applied.dateFrom],
				["date_to", applied.dateTo],
			];
			for (const [key, value] of wire) {
				if (value !== "") params.set(key, value);
			}
			const res = await api.get<
				InvoiceEnvelope<InvoicePagedData<InvoiceAuditLogItem>>
			>(`${AUDIT_LOGS_ENDPOINT}?${params}`);
			return {
				records: res?.data?.records ?? [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
	const hasFilters = !isBlank(applied);
	// 「重設」要連**還沒套用的草稿**一起清掉：否則按完之後篩選框還留著字而列表已是全集，
	// 兩者互相矛盾。
	const canReset = hasFilters || !isBlank(draft);
	const selected = records.find((row) => row.auditLogId === selectedId) ?? null;

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
		<div className="adm-audit-console">
			<div className="page-header">
				<div>
					<h1 className="page-title">發票稽核紀錄</h1>
					<p className="page-subtitle">
						依<strong>操作發生時間</strong>由新到舊排列。資料為載入當下的快照。
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

			<div className="adm-audit-main">
				<form className="adm-audit-filters" onSubmit={applyFilters}>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-operation">操作類型</label>
						<select
							id="invoice-audit-operation"
							value={draft.operationType}
							onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
								field("operationType")(e.target.value)
							}
						>
							<option value="">全部</option>
							{auditActionOptionGroups().map(([label, options]) => (
								<optgroup key={label} label={label}>
									{options.map(([value, text]) => (
										<option key={value} value={value}>
											{text}
										</option>
									))}
								</optgroup>
							))}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-actor">操作者 account_id</label>
						<input
							id="invoice-audit-actor"
							value={draft.actorAccountId}
							inputMode="numeric"
							autoComplete="off"
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("actorAccountId")(e.target.value)
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-store">賣家 store_id</label>
						<input
							id="invoice-audit-store"
							value={draft.storeId}
							inputMode="numeric"
							autoComplete="off"
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("storeId")(e.target.value)
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-order">訂單編號</label>
						<input
							id="invoice-audit-order"
							value={draft.orderNo}
							autoComplete="off"
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("orderNo")(e.target.value)
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-document">發票文件 ID</label>
						<input
							id="invoice-audit-document"
							value={draft.documentId}
							autoComplete="off"
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("documentId")(e.target.value)
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-job">發票工作 ID</label>
						<input
							id="invoice-audit-job"
							value={draft.jobId}
							autoComplete="off"
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("jobId")(e.target.value)
							}
						/>
					</div>
					{/*
					 * 這兩個是 `type="date"` 不是 `datetime-local`：端點 61 的 date_from／date_to
					 * 契約就是 `YYYY-MM-DD`。給時間選擇器卻把時間那半丟掉，是一個不會發出
					 * 任何訊號的靜默失效（理由全文見 features/invoices/date-range.ts）。
					 */}
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-from">起始日期</label>
						<input
							id="invoice-audit-from"
							type="date"
							value={draft.dateFrom}
							onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
								field("dateFrom")(e.target.value)
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="invoice-audit-to">結束日期</label>
						<input
							id="invoice-audit-to"
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
					時間區間比對的是<strong>操作發生時間</strong>（`occurredAt`）， 依
					UTC+8 日界計算。
				</p>

				{isError ? (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>
							{error instanceof Error
								? error.message
								: "無法載入發票稽核紀錄，請稍後再試。"}
						</div>
						<Button variant="outline" type="button" onClick={() => refetch()}>
							重試
						</Button>
					</div>
				) : (
					<div className="adm-audit-table-wrap">
						<table className="adm-audit-table">
							{/*
							 * ADM 既有表格沒有 caption，這是刻意補強（58A §A10 要求，並寫進 PR body
							 * 免得被當成不一致）：螢幕閱讀器在進入表格時需要一句「這是什麼表、多少筆」。
							 */}
							<caption className="sr-only">
								發票操作稽核紀錄，共 {total} 筆，依操作發生時間由新到舊排列
							</caption>
							<thead>
								<tr>
									<th scope="col">時間</th>
									<th scope="col">操作</th>
									<th scope="col">目標資源</th>
									<th scope="col" className="adm-audit-col-reason">
										理由
									</th>
									<th scope="col" className="adm-audit-col-summary">
										變更摘要
									</th>
									<th scope="col">結果</th>
								</tr>
							</thead>
							<tbody>
								{records.length === 0 ? (
									<tr>
										<td colSpan={6} className="adm-audit-empty">
											{/*
											 * 三句話刻意不同：載入中／有篩選但無結果／完全沒有紀錄。
											 * 共用一句會讓管理員以為今天沒有任何操作，而實際上是他把
											 * 條件設窄了。
											 */}
											{isLoading
												? "正在載入發票稽核紀錄…"
												: hasFilters
													? "目前篩選條件沒有操作紀錄"
													: "這個範圍內還沒有任何發票操作紀錄"}
										</td>
									</tr>
								) : (
									records.map((entry) => (
										<tr
											key={entry.auditLogId}
											className={
												entry.auditLogId === selectedId ? "is-selected" : ""
											}
										>
											<td>{formatDateTime(entry.occurredAt)}</td>
											<td>
												{/*
												 * 中文來源優先序：**已載入資料的欄位優先**，只有「本期間沒
												 * 發生過的動作」才退回 CodeMeta 鏡像。Phase 1 的 DTO 還沒有
												 * `operationTypeLabel` ⇒ 兩處都走 CodeMeta，但結構留成這個
												 * 形狀，日後後端補欄位不用重寫。
												 */}
												<strong>
													{codeLabel(
														INVOICE_OPERATION_TYPE_LABEL,
														entry.operationType,
													)}
												</strong>
											</td>
											<td>
												<strong>{entry.targetId || "—"}</strong>
												<span>{entry.targetKind || "—"}</span>
											</td>
											<td className="adm-audit-col-reason">
												{entry.reason || "—"}
											</td>
											<td className="adm-audit-col-summary">
												{auditChangeSummary(
													entry.result,
													entry.beforeJson,
													entry.afterJson,
												)}
											</td>
											<td>
												<span
													className={`adm-audit-result adm-audit-result--${entry.result === "FAILED" ? "failed" : "succeeded"}`}
												>
													{entry.result || "—"}
												</span>
												<button
													type="button"
													className="adm-audit-detail-trigger"
													onClick={() => setSelectedId(entry.auditLogId)}
												>
													查看前後差異
												</button>
											</td>
										</tr>
									))
								)}
							</tbody>
						</table>
					</div>
				)}

				{total > 0 ? (
					<div className="adm-audit-pagination">
						<span>
							顯示第 {(page - 1) * PAGE_SIZE + 1} 至{" "}
							{Math.min(page * PAGE_SIZE, total)} 筆，
							{hasFilters
								? `符合篩選條件的共 ${total} 筆`
								: `共 ${total} 筆紀錄`}
						</span>
						<div>
							<Button
								variant="outline"
								type="button"
								disabled={page <= 1}
								onClick={() => setPage((current) => Math.max(1, current - 1))}
							>
								上一頁
							</Button>
							<span className="adm-audit-page-indicator">
								{page} / {totalPages}
							</span>
							<Button
								variant="outline"
								type="button"
								disabled={page >= totalPages}
								onClick={() => setPage((current) => current + 1)}
							>
								下一頁
							</Button>
						</div>
					</div>
				) : null}
			</div>

			{selected ? (
				<InvoiceAuditDetailDialog
					entry={selected}
					onClose={() => setSelectedId(null)}
				/>
			) : null}
		</div>
	);
}
