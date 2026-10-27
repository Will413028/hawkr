"use client";

import { useQuery } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Button } from "@/components/UI";
import {
	formatClockTime,
	localInputToUtcInstant,
} from "@/features/delegated-operations/format";
import {
	DELEGATED_AUDIT_RESOURCE_TYPE_LABELS,
	DELEGATED_AUDIT_RESULT_LABELS,
	delegatedAuditResourceDisplay,
} from "@/features/delegated-operations/labels";
import {
	DelegatedOperationClientError,
	listAllDelegatedAuditLogs,
} from "@/services/delegated-operations";
import {
	DELEGATED_AUDIT_RESOURCE_TYPES,
	DELEGATED_AUDIT_RESULTS,
	type DelegatedAuditLogEntry,
	type DelegatedAuditResourceType,
	type DelegatedAuditResult,
	delegatedAllAuditLogQueryKey,
	isDelegatedNumericId,
} from "../governance";
import AuditDetailDialog from "./AuditDetailDialog";

/**
 * 跨工作階段的代行稽核紀錄（`GET /api/v1/admin/delegated-operations/audit-logs`，
 * 56 §11.3 P1；接線見 backend#2740）。
 *
 * 與 `SessionAuditConsole` 是**兩種視角、同一種列**：那支答「這一場代行做了什麼」，
 * 這支答「這家店／這位管理員**跨多場代行**被改過什麼」。回應共用同一份 parser。
 *
 * 這支端點在 backend PR #2723 就交付了，但到 #2740 之前零前端消費者 —— 管理員只能
 * 一場一場點進去看，而兩支專為它建的索引（`idx_aoal_target_created`／
 * `idx_aoal_actor_created`）一直在付寫入成本卻沒有任何查詢在用。
 *
 * ⚠️ **這裡沒有「跳到該工作階段」的連結，那不是漏做**：`AuditLogEntry` 這個 proto
 * message 不帶 `session_id`（實查 `api/proto/account/delegation.proto:276-315`），
 * 跨階段的列拿不到它。給一個假的連結比沒有更糟；要補得先改 proto。
 */

const PAGE_SIZE = 20;

interface FilterDraft {
	actorAdminId: string;
	storeId: string;
	resourceType: DelegatedAuditResourceType | "";
	result: DelegatedAuditResult | "";
	dateFrom: string;
	dateTo: string;
}

const EMPTY_FILTERS: FilterDraft = {
	actorAdminId: "",
	storeId: "",
	resourceType: "",
	result: "",
	dateFrom: "",
	dateTo: "",
};

/**
 * 草稿與已套用分開存：邊打字邊查會在使用者還沒輸入完的時候送出一個半截的 id，
 * 而那個查詢**會成功**（只是查錯人），畫面上分不出來。與 SessionListConsole 同源。
 */
export default function AllAuditLogConsole() {
	const [draft, setDraft] = useState<FilterDraft>(EMPTY_FILTERS);
	const [applied, setApplied] = useState<FilterDraft>(EMPTY_FILTERS);
	const [page, setPage] = useState(1);
	const [formError, setFormError] = useState("");
	const [selected, setSelected] = useState<DelegatedAuditLogEntry | null>(null);

	const query = {
		page,
		pageSize: PAGE_SIZE,
		actorAdminId: applied.actorAdminId,
		storeId: applied.storeId,
		action: "" as const,
		resourceType: applied.resourceType,
		result: applied.result,
		dateFrom: localInputToUtcInstant(applied.dateFrom),
		dateTo: localInputToUtcInstant(applied.dateTo),
	};

	const auditQuery = useQuery({
		queryKey: delegatedAllAuditLogQueryKey(query),
		queryFn: () => listAllDelegatedAuditLogs(query),
	});

	function apply(event: FormEvent) {
		event.preventDefault();
		// 空字串＝不篩選是合法的；只有「填了但不是數字 id」才擋。靜默降級成不篩選會
		// 把「你打錯了店家 id」變成「這是全平台的紀錄」，而畫面上只差在筆數。
		if (draft.actorAdminId && !isDelegatedNumericId(draft.actorAdminId)) {
			setFormError("管理員 ID 必須是數字。");
			return;
		}
		if (draft.storeId && !isDelegatedNumericId(draft.storeId)) {
			setFormError("店家 ID 必須是數字。");
			return;
		}
		if (draft.dateFrom && draft.dateTo && draft.dateFrom > draft.dateTo) {
			setFormError("起始時間不能晚於結束時間。");
			return;
		}
		setFormError("");
		setApplied(draft);
		setPage(1);
	}

	function reset() {
		setDraft(EMPTY_FILTERS);
		setApplied(EMPTY_FILTERS);
		setFormError("");
		setPage(1);
	}

	const hasFilters = Object.values(applied).some((v) => v !== "");
	const records = auditQuery.data?.records ?? [];
	const total = auditQuery.data?.total ?? 0;
	const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

	return (
		<div className="adm-audit-console">
			<header className="adm-audit-header">
				<div>
					<h1>跨工作階段代行紀錄</h1>
					<p>
						依店家或操作者回看歷來的代行動作。要看「某一場代行做了什麼」請從
						代行紀錄列表點進該工作階段。
					</p>
				</div>
			</header>

			<div className="adm-audit-main">
				<form className="adm-audit-filters" onSubmit={apply}>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-actor">操作者管理員 ID</label>
						<input
							id="adm-all-audit-actor"
							value={draft.actorAdminId}
							onChange={(e) =>
								setDraft({ ...draft, actorAdminId: e.target.value })
							}
							placeholder="留空＝不限"
							inputMode="numeric"
							autoComplete="off"
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-store">店家 ID</label>
						<input
							id="adm-all-audit-store"
							value={draft.storeId}
							onChange={(e) => setDraft({ ...draft, storeId: e.target.value })}
							placeholder="留空＝不限"
							inputMode="numeric"
							autoComplete="off"
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-resource">資源</label>
						<select
							id="adm-all-audit-resource"
							value={draft.resourceType}
							onChange={(e) =>
								setDraft({
									...draft,
									resourceType: e.target.value as
										| DelegatedAuditResourceType
										| "",
								})
							}
						>
							<option value="">全部</option>
							{DELEGATED_AUDIT_RESOURCE_TYPES.map((v) => (
								<option key={v} value={v}>
									{DELEGATED_AUDIT_RESOURCE_TYPE_LABELS[v]}
								</option>
							))}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-result">結果</label>
						<select
							id="adm-all-audit-result"
							value={draft.result}
							onChange={(e) =>
								setDraft({
									...draft,
									result: e.target.value as DelegatedAuditResult | "",
								})
							}
						>
							<option value="">全部</option>
							{DELEGATED_AUDIT_RESULTS.map((v) => (
								<option key={v} value={v}>
									{DELEGATED_AUDIT_RESULT_LABELS[v]}
								</option>
							))}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-from">起始時間</label>
						<input
							id="adm-all-audit-from"
							type="datetime-local"
							value={draft.dateFrom}
							onChange={(e) => setDraft({ ...draft, dateFrom: e.target.value })}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-all-audit-to">結束時間</label>
						<input
							id="adm-all-audit-to"
							type="datetime-local"
							value={draft.dateTo}
							onChange={(e) => setDraft({ ...draft, dateTo: e.target.value })}
						/>
					</div>
					<div className="adm-audit-filter-actions">
						<Button type="submit">套用</Button>
						<Button type="button" variant="outline" onClick={reset}>
							清除
						</Button>
					</div>
					{formError && (
						<p className="adm-operation-error" role="alert">
							{formError}
						</p>
					)}
				</form>

				{auditQuery.isError ? (
					<div className="adm-audit-notice" role="alert">
						{auditQuery.error instanceof DelegatedOperationClientError
							? auditQuery.error.message
							: "代行稽核紀錄讀取失敗，請稍後再試。"}
					</div>
				) : (
					<div className="adm-audit-table-wrap">
						<table className="adm-audit-table">
							<thead>
								<tr>
									<th scope="col">時間</th>
									<th scope="col">操作者</th>
									<th scope="col">店家</th>
									<th scope="col">操作</th>
									<th scope="col">目標資源</th>
									<th scope="col" className="adm-audit-col-reason">
										理由
									</th>
									<th scope="col">結果</th>
								</tr>
							</thead>
							<tbody>
								{records.length === 0 ? (
									<tr>
										<td colSpan={7} className="adm-audit-empty">
											{/* 三態必須分得開：載入中、篩選無結果、真的沒有任何代行
											    動作。第三種正是治理最想確認的結論，把它跟前兩種
											    混在一起等於讓人無法確認。 */}
											{auditQuery.isLoading
												? "正在載入代行紀錄…"
												: hasFilters
													? "目前的篩選條件沒有符合的代行紀錄。"
													: "平台上還沒有任何代行操作紀錄。"}
										</td>
									</tr>
								) : (
									records.map((entry) => (
										<tr key={entry.auditLogId}>
											<td>{formatClockTime(entry.createdAt)}</td>
											<td>
												<strong>{entry.actor.displayName}</strong>
												<span>{entry.actor.adminId}</span>
											</td>
											<td>
												<strong>{entry.target.storeName}</strong>
												<span>{entry.target.storeId}</span>
											</td>
											<td>
												<strong>{entry.actionLabel}</strong>
												{entry.riskLevel === "high" && (
													<span className="adm-audit-risk adm-audit-risk--high">
														風險操作
													</span>
												)}
											</td>
											<td>
												<strong>{delegatedAuditResourceDisplay(entry)}</strong>
												<span>
													{
														DELEGATED_AUDIT_RESOURCE_TYPE_LABELS[
															entry.resourceType
														]
													}
												</span>
											</td>
											<td className="adm-audit-col-reason">
												{entry.operationReason || "—"}
											</td>
											<td>
												<span
													className={`adm-audit-result adm-audit-result--${entry.result.toLowerCase()}`}
												>
													{DELEGATED_AUDIT_RESULT_LABELS[entry.result]}
												</span>
												<button
													type="button"
													className="adm-audit-detail-trigger"
													onClick={() => setSelected(entry)}
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

				{total > 0 && (
					<nav className="adm-audit-pager" aria-label="分頁">
						<Button
							type="button"
							variant="outline"
							disabled={page <= 1}
							onClick={() => setPage((p) => Math.max(1, p - 1))}
						>
							上一頁
						</Button>
						<span>
							第 {page} / {totalPages} 頁（共 {total} 筆）
						</span>
						<Button
							type="button"
							variant="outline"
							disabled={page >= totalPages}
							onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
						>
							下一頁
						</Button>
					</nav>
				)}
			</div>

			{selected && (
				<AuditDetailDialog
					entry={selected}
					auditRetentionDays={null}
					onClose={() => setSelected(null)}
				/>
			)}
		</div>
	);
}
