"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import {
	type BadgeVariant,
	Button,
	Dialog,
	StatusBadge,
} from "@/components/UI";
import {
	codeLabel,
	REPORT_REASON_CODE_LABEL,
	REPORT_RESOLVE_ACTION_LABEL,
	REPORT_STATUS_LABEL,
	REPORT_TARGET_TYPE_LABEL,
	type ReportReasonCodeValue,
	ReportResolveAction,
	type ReportResolveActionValue,
	ReportStatus,
	type ReportStatusValue,
	type ReportTargetTypeValue,
	toOptions,
} from "@/lib/codemeta";
import { api } from "@/services/api";

// B-135: 檢舉處理頁（Apple Guideline 1.2 / Issue #645）

interface Report {
	// id 系列為 64-bit TSID（> Number.MAX_SAFE_INTEGER），必須用 string，否則原生
	// JSON.parse 會失精導致查不到 / resolve 打錯紀錄。後端 gateway 已對 reportData
	// 加 ,string（PR #966）。*_ms 是毫秒 timestamp（< 2^53）維持 number。
	id: string;
	reporterId: string;
	targetType: string;
	targetId: string;
	reasonCode: string;
	description: string;
	status: string;
	resolvedAction: string;
	resolvedBy: string;
	resolvedAtMs: number;
	createTimeMs: number;
}

interface ReportsResponse {
	code: number;
	data: { records: Report[] };
}

// 值域一律從 CodeMeta 生成物導出，不在這裡重列一次。這頁曾經手抄七份，其中
// `STATUS_LABELS` 漏掉 `APPEALED`（申訴重開）——申訴中的檢舉在列表顯示原始碼字串，
// 而且狀態篩選下拉根本沒有那個選項，等於篩不出來也處理不了。
type StatusFilter = "" | ReportStatusValue;
type TargetTypeFilter = "" | ReportTargetTypeValue;
type ReasonFilter = "" | ReportReasonCodeValue;
type ResolveAction = ReportResolveActionValue;

/**
 * 狀態徽章的顏色 token —— 後端不下發顏色，所以這張表留在前端；顯示文字則一律走
 * `codeLabel(REPORT_STATUS_LABEL, …)`。鍵直接引用生成的 `ReportStatus`，值域長出
 * 新成員時這裡跟著壞（型別錯），而不是靜靜落進 fallback。
 */
const STATUS_BADGE_VARIANT: Record<string, BadgeVariant> = {
	[ReportStatus.PENDING]: "warning",
	[ReportStatus.RESOLVED_HIDDEN]: "danger",
	[ReportStatus.RESOLVED_WARNED]: "success",
	[ReportStatus.RESOLVED_BANNED]: "danger",
	[ReportStatus.DISMISSED]: "gray",
	// 申訴中＝處置過但被質疑，仍待管理員裁決。用 info 與「待處理」(warning) 區隔，
	// 但同樣不是終態灰。
	[ReportStatus.APPEALED]: "info",
};

/** 可裁決的起始狀態，對齊 social-service `report_repository.go` 的 allowed-from。 */
const ACTIONABLE_STATUSES: readonly string[] = [
	ReportStatus.PENDING,
	ReportStatus.APPEALED,
];

/**
 * 每個裁決動作的**操作說明**，不是碼→標籤對照，所以不走生成物：CodeMeta 的
 * Description 欄寫的是「這個碼在系統裡代表什麼」，這裡要講的是「按下去會發生什麼副作用」
 * （寫哪個欄位、登入會不會失效、Phase 1 還缺什麼）。標籤本身仍從
 * `REPORT_RESOLVE_ACTION_LABEL` 查。
 */
const ACTION_DESCRIPTIONS: Record<ResolveAction, string> = {
	HIDE_TARGET: "將被檢舉內容下架（hidden_at 寫入），使用者無法再看到",
	WARN_USER: "記錄警告，目前 Phase 1 不發站內信（待 Phase 2）",
	BAN_USER: "停權帳號，登入立即失效",
	DISMISS: "駁回檢舉，內容保留",
};

function formatHoursAgo(ms: number): string {
	if (!ms) return "-";
	const hours = Math.floor((Date.now() - ms) / 3600000);
	if (hours < 1) return "< 1h";
	if (hours < 24) return `${hours}h`;
	return `${Math.floor(hours / 24)}d`;
}

function ageHours(ms: number): number {
	return (Date.now() - ms) / 3600000;
}

function StatusPill({ status }: { status: string }) {
	// 值域外的碼：灰底 ＋ `codeLabel` 的原碼回退，不會顯示空白徽章。
	return (
		<StatusBadge variant={STATUS_BADGE_VARIANT[status] ?? "gray"}>
			{codeLabel(REPORT_STATUS_LABEL, status)}
		</StatusBadge>
	);
}

export default function ReportsAdminPage() {
	const queryClient = useQueryClient();
	const [page, setPage] = useState(1);
	const [statusFilter, setStatusFilter] = useState<StatusFilter>("PENDING");
	const [targetFilter, setTargetFilter] = useState<TargetTypeFilter>("");
	const [reasonFilter, setReasonFilter] = useState<ReasonFilter>("");

	const [resolving, setResolving] = useState<Report | null>(null);
	const [action, setAction] = useState<ResolveAction>(
		ReportResolveAction.HIDE_TARGET,
	);
	const [note, setNote] = useState("");

	const pageSize = 20;

	const { data: reports = [], isLoading: loading } = useQuery({
		queryKey: [
			"reports",
			"list",
			{
				page,
				status: statusFilter,
				target: targetFilter,
				reason: reasonFilter,
			},
		],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(pageSize),
			});
			if (statusFilter) params.set("status", statusFilter);
			if (targetFilter) params.set("target_type", targetFilter);
			if (reasonFilter) params.set("reason_code", reasonFilter);
			const res = await api.get<ReportsResponse>(
				`/api/v1/admin/reports?${params}`,
			);
			return res?.code === 0 && res?.data ? res.data.records || [] : [];
		},
	});

	// 處理檢舉 → 成功後 invalidate 自動重抓
	const resolveMutation = useMutation({
		mutationFn: (vars: { id: string; action: ResolveAction; note: string }) =>
			api.post(`/api/v1/admin/reports/${vars.id}/resolve`, {
				action: vars.action,
				note: vars.note,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["reports"] });
		},
	});
	const submitting = resolveMutation.isPending;

	// deep-link：URL ?target_type= 進來時預設過濾（貼文管理「檢舉數」帶過來）
	useEffect(() => {
		const tt = new URLSearchParams(window.location.search).get("target_type");
		if (tt) setTargetFilter(tt as TargetTypeFilter);
	}, []);

	const openResolve = (r: Report) => {
		setResolving(r);
		setAction(ReportResolveAction.HIDE_TARGET);
		setNote("");
	};

	const closeResolve = () => {
		if (submitting) return;
		setResolving(null);
	};

	const handleResolve = async () => {
		if (!resolving) return;
		// HIDE_TARGET 的 note 會寫入 hidden_reason、賣家在自己頁面看得到 →
		// 對齊主動下架（posts 頁）強制填原因，避免賣家只看到 fallback 文案。
		if (action === ReportResolveAction.HIDE_TARGET && !note.trim()) {
			alert("請輸入下架原因");
			return;
		}
		try {
			await resolveMutation.mutateAsync({
				id: resolving.id,
				action,
				note: note.trim(),
			});
			closeResolve();
		} catch (e) {
			alert(`處理失敗：${e instanceof Error ? e.message : "請稍後再試"}`);
		}
	};

	return (
		<div className="post-mgmt-page">
			<div className="page-header">
				<div>
					<h1 className="page-title">內容審核佇列</h1>
					<p className="page-subtitle">
						內容檢舉與使用者封鎖（Apple Guideline 1.2 合規 24h SLA）
					</p>
				</div>
			</div>

			<div className="post-mgmt-tabs" style={{ flexWrap: "wrap", gap: 8 }}>
				<select
					value={statusFilter}
					onChange={(e) => {
						setStatusFilter(e.target.value as StatusFilter);
						setPage(1);
					}}
				>
					<option value="">全部狀態</option>
					{toOptions(REPORT_STATUS_LABEL).map(([k, v]) => (
						<option key={k} value={k}>
							{v}
						</option>
					))}
				</select>
				<select
					value={targetFilter}
					onChange={(e) => {
						setTargetFilter(e.target.value as TargetTypeFilter);
						setPage(1);
					}}
				>
					<option value="">全部類型</option>
					{toOptions(REPORT_TARGET_TYPE_LABEL).map(([k, v]) => (
						<option key={k} value={k}>
							{v}
						</option>
					))}
				</select>
				<select
					value={reasonFilter}
					onChange={(e) => {
						setReasonFilter(e.target.value as ReasonFilter);
						setPage(1);
					}}
				>
					<option value="">全部原因</option>
					{toOptions(REPORT_REASON_CODE_LABEL).map(([k, v]) => (
						<option key={k} value={k}>
							{v}
						</option>
					))}
				</select>
			</div>

			<div className="table-container">
				<table className="data-table post-mgmt-table">
					<thead>
						<tr>
							<th>檢舉 ID</th>
							<th>類型</th>
							<th>Target ID</th>
							<th>原因</th>
							<th>說明</th>
							<th>檢舉人</th>
							<th>狀態</th>
							<th>停留時間</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td
									colSpan={9}
									style={{ textAlign: "center", padding: "40px" }}
								>
									載入中...
								</td>
							</tr>
						) : reports.length === 0 ? (
							<tr>
								<td
									colSpan={9}
									style={{ textAlign: "center", padding: "40px" }}
								>
									暫無資料
								</td>
							</tr>
						) : (
							reports.map((r) => {
								const isOverSLA =
									r.status === ReportStatus.PENDING &&
									ageHours(r.createTimeMs) > 24;
								return (
									<tr
										key={r.id}
										style={isOverSLA ? { background: "#fef2f2" } : undefined}
									>
										<td className="post-id-cell">#{r.id}</td>
										<td>{codeLabel(REPORT_TARGET_TYPE_LABEL, r.targetType)}</td>
										<td>{r.targetId}</td>
										<td>{codeLabel(REPORT_REASON_CODE_LABEL, r.reasonCode)}</td>
										<td className="report-reason-cell" title={r.description}>
											{r.description || "-"}
										</td>
										<td>{r.reporterId}</td>
										<td>
											<StatusPill status={r.status} />
										</td>
										<td
											style={{
												color: isOverSLA ? "#dc2626" : undefined,
												fontWeight: isOverSLA ? 600 : undefined,
											}}
										>
											{formatHoursAgo(r.createTimeMs)}
											{isOverSLA && " ⚠"}
										</td>
										<td>
											{ACTIONABLE_STATUSES.includes(r.status) ? (
												<Button
													size="sm"
													variant="primary"
													onClick={() => openResolve(r)}
												>
													處理
												</Button>
											) : (
												<span style={{ color: "var(--gray-400)" }}>已處理</span>
											)}
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>

			<div className="post-mgmt-pagination">
				<button
					type="button"
					className="btn"
					disabled={page === 1}
					onClick={() => setPage((p) => p - 1)}
				>
					<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
				</button>
				<span className="page-info">第 {page} 頁</span>
				<button
					type="button"
					className="btn"
					disabled={reports.length < pageSize}
					onClick={() => setPage((p) => p + 1)}
				>
					下一頁 <MaterialSymbol name="keyboard_arrow_right" />
				</button>
			</div>

			{resolving && (
				<Dialog
					title={`處理檢舉 #${resolving.id}`}
					className="post-moderate-modal"
					onClose={closeResolve}
					footer={
						<>
							<Button
								variant="outline"
								onClick={closeResolve}
								disabled={submitting}
							>
								取消
							</Button>
							<Button
								variant={
									action === ReportResolveAction.BAN_USER ||
									action === ReportResolveAction.HIDE_TARGET
										? "danger"
										: "primary"
								}
								onClick={handleResolve}
								disabled={submitting}
							>
								{submitting
									? "處理中..."
									: `確認 ${REPORT_RESOLVE_ACTION_LABEL[action]}`}
							</Button>
						</>
					}
				>
					<div className="moderate-post-preview">
						<div className="moderate-info-row">
							<span className="moderate-label">類型 / Target</span>
							<span>
								{codeLabel(REPORT_TARGET_TYPE_LABEL, resolving.targetType)} #
								{resolving.targetId}
							</span>
						</div>
						<div className="moderate-info-row">
							<span className="moderate-label">原因</span>
							<span>
								{codeLabel(REPORT_REASON_CODE_LABEL, resolving.reasonCode)}
							</span>
						</div>
						{resolving.description && (
							<div className="moderate-info-row">
								<span className="moderate-label">補充</span>
								<span className="moderate-content">
									{resolving.description}
								</span>
							</div>
						)}
						<div className="moderate-info-row">
							<span className="moderate-label">檢舉人 ID</span>
							<span>{resolving.reporterId}</span>
						</div>
					</div>

					<div
						style={{
							display: "block",
							marginTop: 16,
							marginBottom: 8,
							fontWeight: 600,
						}}
					>
						採取動作
					</div>
					{(Object.keys(REPORT_RESOLVE_ACTION_LABEL) as ResolveAction[]).map(
						(a) => (
							<label
								key={a}
								style={{ display: "block", padding: 8, cursor: "pointer" }}
							>
								<input
									type="radio"
									name="action"
									value={a}
									checked={action === a}
									onChange={() => setAction(a)}
									style={{ marginRight: 8 }}
								/>
								<strong>{REPORT_RESOLVE_ACTION_LABEL[a]}</strong>
								<span
									style={{
										color: "var(--gray-500)",
										marginLeft: 8,
										fontSize: 13,
									}}
								>
									{ACTION_DESCRIPTIONS[a]}
								</span>
							</label>
						),
					)}

					<label
						htmlFor="resolve-note-input"
						style={{ display: "block", marginTop: 12, marginBottom: 4 }}
					>
						{action === ReportResolveAction.HIDE_TARGET ? (
							<>
								下架原因 <span className="required">*</span>
								（寫入 hidden_reason，賣家會看到）
							</>
						) : (
							"備註（選填、寫入 hidden_reason 與 audit）"
						)}
					</label>
					<textarea
						id="resolve-note-input"
						value={note}
						onChange={(e) => setNote(e.target.value)}
						rows={3}
						style={{ width: "100%", padding: 8 }}
						placeholder="例：違反社群守則、騷擾性留言..."
					/>
				</Dialog>
			)}
		</div>
	);
}
