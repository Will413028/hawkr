"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { Button, StatCard } from "@/components/UI";
import { delegationAuthorizationErrorMessage } from "@/features/delegated-operations/authorization";
import {
	auditConfirmationText,
	formatClockTime,
	localInputToUtcInstant,
	sessionDurationText,
	sessionEndText,
	sessionRevokedByText,
} from "@/features/delegated-operations/format";
import {
	DELEGATED_AUDIT_RESOURCE_TYPE_LABELS,
	DELEGATED_AUDIT_RESULT_LABELS,
	DELEGATED_SESSION_STATUS_LABELS,
	delegatedAuditResourceDisplay,
} from "@/features/delegated-operations/labels";
import { codeLabel, DELEGATION_AUDIT_ACTION_LABEL } from "@/lib/codemeta";
import {
	DelegatedOperationClientError,
	getDelegatedSessionReport,
	listDelegatedAuditLogs,
} from "@/services/delegated-operations";
import {
	DELEGATED_AUDIT_ACTIONS,
	DELEGATED_AUDIT_RESOURCE_TYPES,
	DELEGATED_AUDIT_RESULTS,
	type DelegatedAuditAction,
	type DelegatedAuditLogEntry,
	type DelegatedAuditResourceType,
	type DelegatedAuditResult,
} from "../governance";
import AuditDetailDialog from "./AuditDetailDialog";
import CopyIconButton from "./CopyIconButton";
import SessionRevokeDialog from "./SessionRevokeDialog";

/**
 * ADM 側的代行紀錄治理台（mockup image11 的主區＋右面板，治理視角）。
 *
 * 兩支查詢都掛 `requireAdmin()`：它們可以看**任一**工作階段，與工作區那條
 * credential 路徑是兩種身分。因此本頁不需要、也拿不到 operation credential。
 */

const PAGE_SIZE = 20;

/** 操作類型下拉按資源前綴分組。分組是結構推導（前綴）。
 *
 *  下拉的中文出自兩層，順序不可顛倒：**已載入資料裡的 `actionLabel` 優先**（那是
 *  後端逐列下發的權威值），只有「這個工作階段沒發生過的動作」才退回
 *  `DELEGATION_AUDIT_ACTION_LABEL` 鏡像 —— 篩選器必須在資料到達之前就列滿 19 個選項，
 *  而權威值那時還不存在（原本因此直接印 `PRODUCT_BATCH_UNPUBLISH` 這種機器名，
 *  與同頁另外兩個中文下拉自相矛盾）。稽核**列**永遠只讀 `actionLabel`。 */
const ACTION_GROUPS: {
	key: DelegatedAuditResourceType | "other";
	label: string;
	prefix: string;
}[] = [
	{ key: "storefront", label: "店面", prefix: "STOREFRONT_" },
	{ key: "product", label: "商品", prefix: "PRODUCT_" },
	{ key: "post", label: "內容", prefix: "POST_" },
	{ key: "order", label: "訂單", prefix: "ORDER_" },
	{ key: "other", label: "其他", prefix: "" },
];

/**
 * 變更摘要欄一律渲染後端下發的 label／before／after。
 *
 * 沒有 `changedFields` 時三種結果各說各的話，不能共用一句：`FAILED` 的那一列是
 * **領域寫入根本沒有發生**（稽核列是寫入前落的 intent），說「無欄位差異」會被讀成
 * 「執行了但沒改到東西」——對正在判斷代行者做過什麼的人，這兩件事的差別就是全部。
 * 只有 `SUCCEEDED` 才真的是「寫入成功但沒有欄位改變」（如 PII 揭露這種讀取類紀錄）。
 */
function changeSummaryOf(entry: DelegatedAuditLogEntry): string {
	if (entry.changedFields.length > 0) {
		return entry.changedFields
			.map(
				(field) =>
					`${field.label} ${field.before || "—"} → ${field.after || "—"}`,
			)
			.join("；");
	}
	switch (entry.result) {
		case "FAILED":
			return "未寫入";
		case "PENDING":
			return "結果未確認";
		default:
			return "無欄位差異";
	}
}

/**
 * 一律以業務碼分派，`error.message` 只當 fallback —— 與授權面板同一張表、同一個理由。
 *
 * BFF 的 `safeUpstreamError` 刻意不透傳上游訊息（自由文字可能夾帶憑證或內部拓撲），
 * 它對**每一個**上游錯誤都送同一句「代行營運服務暫時無法使用」。所以只讀 message 的話，
 * 「這個工作階段不存在」（9277）與「上游真的掛了」（9280）在畫面上一字不差，而兩者的
 * 下一步相反：前者要回列表重挑，後者要等或找維運。
 */
function errorMessageOf(error: unknown, fallback: string): string {
	if (error instanceof DelegatedOperationClientError) {
		const byCode =
			typeof error.code === "number"
				? delegationAuthorizationErrorMessage(error.code, error.message)
				: error.message;
		return `${byCode}${error.traceId ? `（追蹤編號 ${error.traceId}）` : ""}`;
	}
	return error instanceof Error ? error.message : fallback;
}

interface SessionAuditConsoleProps {
	sessionId: string;
}

export default function SessionAuditConsole({
	sessionId,
}: SessionAuditConsoleProps) {
	const queryClient = useQueryClient();
	const [page, setPage] = useState(1);
	const [action, setAction] = useState<DelegatedAuditAction | "">("");
	const [resourceType, setResourceType] = useState<
		DelegatedAuditResourceType | ""
	>("");
	const [result, setResult] = useState<DelegatedAuditResult | "">("");
	const [dateFrom, setDateFrom] = useState("");
	const [dateTo, setDateTo] = useState("");
	// 稽核內容走 modal（`AuditDetailDialog`），開啟後焦點與背景隔離由那支自己的
	// a11y 三件套處理，這裡只持有「開哪一列」。
	const [selectedId, setSelectedId] = useState<string | null>(null);
	const [revokeOpen, setRevokeOpen] = useState(false);
	const [revokeNotice, setRevokeNotice] = useState<string | null>(null);

	const reportQuery = useQuery({
		queryKey: ["delegated-governance", sessionId, "summary"],
		queryFn: () => getDelegatedSessionReport(sessionId),
		retry: false,
	});

	// 摘要端點只回成功數（`operation_count` ＝ `count(*) FILTER (WHERE result='SUCCEEDED')`），
	// 而下方列表的「共 N 筆」是三種結果的全部。兩個數字擺在同一頁卻無從對照，讀者只能
	// 把成功數當成「這次總共做了幾件事」。失敗數沒有對應的摘要欄位，就用同一份稽核表以
	// `result=FAILED` 取 `total`（`pageSize=1`，只要那個數）。
	// 不用「全部 − 成功 − 未確認」的減法：那要跨兩次查詢做算術，中間新寫入的列會讓它算出負數。
	const failedCountQuery = useQuery({
		queryKey: ["delegated-governance", sessionId, "failed-count"],
		queryFn: () =>
			listDelegatedAuditLogs(sessionId, {
				page: 1,
				pageSize: 1,
				result: "FAILED",
			}),
		retry: false,
	});

	const auditQuery = useQuery({
		queryKey: [
			"delegated-governance",
			sessionId,
			"audit-logs",
			{ page, action, resourceType, result, dateFrom, dateTo },
		],
		queryFn: () =>
			listDelegatedAuditLogs(sessionId, {
				page,
				pageSize: PAGE_SIZE,
				action,
				resourceType,
				result,
				dateFrom: localInputToUtcInstant(dateFrom),
				dateTo: localInputToUtcInstant(dateTo),
			}),
		retry: false,
	});

	const report = reportQuery.data ?? null;
	const failedCount = failedCountQuery.data?.total ?? null;
	const auditPage = auditQuery.data ?? null;
	const records = auditPage?.records ?? [];
	const selected = useMemo(
		() => records.find((entry) => entry.auditLogId === selectedId) ?? null,
		[records, selectedId],
	);
	const totalPages = auditPage
		? Math.max(1, Math.ceil(auditPage.total / auditPage.pageSize))
		: 1;
	// 鏡像只是引導值；只要這一頁看得到某個動作，就用它自己帶的權威中文覆蓋。
	const actionLabels = useMemo(() => {
		const labels: Record<string, string> = { ...DELEGATION_AUDIT_ACTION_LABEL };
		for (const entry of records) {
			if (entry.actionLabel) labels[entry.action] = entry.actionLabel;
		}
		return labels;
	}, [records]);
	const hasFilters =
		action !== "" ||
		resourceType !== "" ||
		result !== "" ||
		dateFrom !== "" ||
		dateTo !== "";

	const resetFilters = () => {
		setAction("");
		setResourceType("");
		setResult("");
		setDateFrom("");
		setDateTo("");
		setPage(1);
	};

	if (reportQuery.isError) {
		return (
			<div className="adm-audit-console">
				<div className="adm-operation-preflight-blocked" role="alert">
					<MaterialSymbol name="gpp_bad" />
					<div>
						<strong>無法載入這個代行工作階段</strong>
						<p>
							{errorMessageOf(
								reportQuery.error,
								"工作階段不存在，或代行營運服務暫時無法使用。",
							)}
						</p>
					</div>
				</div>
			</div>
		);
	}

	return (
		<div className="adm-audit-console">
			<header className="adm-audit-header">
				<div>
					<h1>代行紀錄</h1>
					<p>本次工作階段的操作、理由與資料差異</p>
				</div>
				<div className="adm-audit-header-actions">
					{report && (
						<span
							className={`adm-audit-status adm-audit-status--${report.status}`}
						>
							{DELEGATED_SESSION_STATUS_LABELS[report.status]}
						</span>
					)}
					<Button
						variant="outline"
						type="button"
						disabled={!report || report.status === "revoked"}
						onClick={() => setRevokeOpen(true)}
					>
						撤銷工作階段
					</Button>
				</div>
			</header>

			{revokeNotice && (
				<div className="adm-audit-notice" role="status">
					<MaterialSymbol name="verified_user" />
					{revokeNotice}
				</div>
			)}

			<section className="adm-audit-summary-cards" aria-label="工作階段摘要">
				{/* 這個代號是要被貼進工單與 log 查詢的東西（設計稿 image11 的
				    `OPS-240803-0187 ⧉`），而同頁的關聯 ID／冪等鍵／操作能力三處都有複製鍵，
				    偏偏最常被複製的這個沒有。複製鍵沿用面板那顆，不另寫一顆。

				    ⚠️ 標籤是「代號」不是「ID」：這張卡顯示的是 `displayCode`，而同一頁的
				    「工作階段 ID」（數字 TSID）是另一個值。兩者都真的會被複製出去用——代號
				    貼工單、數字 ID 貼查詢框與維運 SQL——所以叫錯名字不是措辭問題：照著卡片
				    標籤把 `OPS-…` 貼進要求數字 ID 的地方，會直接被擋下來。數字 ID 在下方
				    「工作階段摘要」有自己的一列。 */}
				<StatCard
					label="工作階段代號"
					value={report?.displayCode ?? "—"}
					icon="fingerprint"
					valueAction={
						report ? (
							<CopyIconButton value={report.displayCode} label="工作階段代號" />
						) : null
					}
				/>
				<StatCard
					label="開始時間"
					value={report ? formatClockTime(report.issuedAt) : "—"}
					icon="schedule"
				/>
				<StatCard
					label="成功操作"
					value={report ? `${report.operationCount} 項` : "—"}
					icon="checklist"
				/>
				<StatCard
					label="風險操作（成功）"
					value={report ? `${report.riskOperationCount} 項` : "—"}
					icon="crisis_alert"
					variant={
						report && report.riskOperationCount > 0 ? "danger" : "default"
					}
				/>
			</section>

			{/* 這兩張卡都只數 `result='SUCCEEDED'` 的列（風險卡的述詞是 `risk_level='high'
			    AND result='SUCCEEDED'`）。不寫明口徑，「風險操作 0」會被讀成「沒有人嘗試過
			    高風險操作」，而實際上可能是嘗試了三次、三次都失敗。 */}
			<p className="adm-audit-summary-note">
				這兩個計數只含<strong>已成功</strong>
				的紀錄；失敗與結果未確認不計入，見下方「操作結果」。
			</p>

			{report && (
				<section className="adm-audit-close-summary" aria-label="結束摘要">
					<h2>工作階段摘要</h2>
					<ul className="adm-audit-close-rows">
						{/* 數字 ID 先前只存在於網址列。它是**查詢框與維運 SQL 認的那一個**
						    （代號走的是列表的篩選軸），而上方那張卡給的是代號 ⇒ 想把這次
						    代行的識別碼交給別人時，畫面上沒有任何地方可以複製。
						    佔滿整行的理由同下方「操作結果」那列：19 位數字塞不進 220px 的
						    自動欄，會被擠成兩行而看不出與標籤的對應。 */}
						<li className="adm-audit-close-rows__wide">
							<MaterialSymbol name="tag" />
							<span>工作階段 ID</span>
							<strong>
								{report.sessionId}
								<CopyIconButton value={report.sessionId} label="工作階段 ID" />
							</strong>
						</li>
						<li>
							<MaterialSymbol name="schedule" />
							<span>工作時長</span>
							{/* 為什麼不能直接印 `durationSeconds`：見 `sessionDurationText`，
							    它與下方「結束時間」是同一個前提的兩半。 */}
							<strong>
								{sessionDurationText(
									report.durationSeconds,
									report.endedAt,
									report.status,
								)}
							</strong>
						</li>
						{/* 三態攤開，相加就是下方列表未篩選時的「共 N 筆」——那兩個數字先前
						    只差在口徑（成功 vs 全部）卻沒有任何交代，讀者無從得知差額是失敗。 */}
						<li className="adm-audit-close-rows__wide">
							<MaterialSymbol name="menu" />
							<span>操作結果</span>
							<strong>
								{failedCount === null
									? `成功 ${report.operationCount} 項（失敗筆數讀取中）`
									: `成功 ${report.operationCount} · 失敗 ${failedCount} · 結果未確認 ${report.pendingAuditCount}（共 ${report.operationCount + failedCount + report.pendingAuditCount} 筆紀錄）`}
							</strong>
						</li>
						<li>
							<MaterialSymbol name="verified_user" />
							<span>稽核狀態</span>
							{/* 三態判定與措辭理由都在 `auditConfirmationText`（純函式、有單元
							    測試）：JSX 只負責把它掛上「未確認」的視覺強調。 */}
							<strong
								className={
									report.pendingAuditCount > 0 ? "is-pending" : undefined
								}
							>
								{auditConfirmationText(report, failedCount)}
							</strong>
						</li>
						<li>
							<MaterialSymbol name="notifications_active" />
							<span>賣家通知</span>
							{/* 語意是「本次至少有一列已發出通知事件」，不是賣家已讀 ——
							    「賣家已通知」會被讀成每一筆操作都通知過。 */}
							<strong>
								{report.notifiedSeller
									? "本次已發送賣家通知"
									: "本次未發送通知"}
							</strong>
						</li>
						<li>
							<MaterialSymbol name="event_available" />
							<span>結束時間</span>
							{/* 為什麼不能直接印 `endedAt ?? "尚未結束"`：見 `sessionEndText`。 */}
							<strong>{sessionEndText(report.endedAt, report.status)}</strong>
						</li>
					</ul>
					{/* 撤銷紀錄。位置緊接在「結束時間」那一列之下，因為它正是那一列的解釋：
					    右上角的「已撤銷」徽章說了結局、「結束時間」說了時刻，而「為什麼中止」
					    在修前**任何畫面都渲染不出來** —— 撤銷理由在 `SessionRevokeDialog` 是
					    必填的，強制收集一個永遠不會被讀回來的欄位，只會訓練操作者亂填，而它
					    正是爭議時唯一能舉證中止原因的字串。

					    自成一塊而不是再加一列 `<li>`：理由是自由文字（最長 200 字），而摘要
					    列表的每一列都是短值，塞進 `grid` 會把整列撐成四五行。

					    ⚠️ 判準是 `revokedAt` 而**不是** `status === "revoked"`：ADM 與 gateway
					    各自 promote，gateway 還沒帶這四欄的窗口裡照 status 渲染會印出一句
					    「未記錄撤銷理由」——那是假的，理由一直都在 DB 裡。沒有資料就整塊不
					    渲染（未撤銷的工作階段也因此不會多出一塊空白區）。 */}
					{report.revokedAt && (
						<div className="adm-audit-revocation">
							<MaterialSymbol name="cancel" />
							<div>
								<strong>撤銷紀錄</strong>
								{/* 後端填的是 `admin_accounts.email`（該表沒有顯示名欄位），所以這裡
								    不掛「姓名」這個標籤。數字 admin id 一併印出：email 會換，而那個
								    id 是維運 SQL 認的那一個。拼接與空白規則見 `sessionRevokedByText`。 */}
								<p className="adm-audit-revocation__meta">
									{sessionRevokedByText(
										report.revokedAt,
										report.revokedByAdminDisplayName,
										report.revokedByAdminId,
									)}
								</p>
								<p className="adm-audit-revocation__reason">
									{report.revokeReason ?? "未記錄撤銷理由"}
								</p>
							</div>
						</div>
					)}
					{/* 這張卡與「稽核狀態」不同源、不得連動：它由兩個結構恆為 0 的計數
					    保證，一旦非零代表三層鎖定被打穿。 */}
					<div
						className={`adm-audit-safety${
							report.fulfillmentActionCount + report.financeActionCount > 0
								? " adm-audit-safety--breached"
								: ""
						}`}
					>
						<MaterialSymbol name="shield_lock" />
						{report.fulfillmentActionCount + report.financeActionCount === 0 ? (
							<span>未執行任何出貨或財務操作</span>
						) : (
							<span>
								偵測到 {report.fulfillmentActionCount} 筆履約與{" "}
								{report.financeActionCount} 筆財務操作 ——
								代行不應具備這些能力，請立即上報。
							</span>
						)}
					</div>
					<p className="adm-audit-retention">
						依平台稽核政策保留 {report.auditRetentionDays} 天
					</p>
				</section>
			)}

			<div className="adm-audit-main">
				<section className="adm-audit-filters" aria-label="稽核紀錄篩選">
					<div className="adm-operation-field">
						<label htmlFor="adm-audit-action">操作類型</label>
						<select
							id="adm-audit-action"
							value={action}
							onChange={(event) => {
								setAction(event.target.value as DelegatedAuditAction | "");
								setPage(1);
							}}
						>
							<option value="">全部</option>
							{ACTION_GROUPS.map((group) => {
								const options = DELEGATED_AUDIT_ACTIONS.filter((value) =>
									group.prefix
										? value.startsWith(group.prefix)
										: !ACTION_GROUPS.some(
												(other) =>
													other.prefix && value.startsWith(other.prefix),
											),
								);
								return options.length === 0 ? null : (
									<optgroup key={group.key} label={group.label}>
										{options.map((value) => (
											<option key={value} value={value}>
												{codeLabel(actionLabels, value)}
											</option>
										))}
									</optgroup>
								);
							})}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-audit-resource">資源</label>
						<select
							id="adm-audit-resource"
							value={resourceType}
							onChange={(event) => {
								setResourceType(
									event.target.value as DelegatedAuditResourceType | "",
								);
								setPage(1);
							}}
						>
							<option value="">全部</option>
							{DELEGATED_AUDIT_RESOURCE_TYPES.map((value) => (
								<option key={value} value={value}>
									{DELEGATED_AUDIT_RESOURCE_TYPE_LABELS[value]}
								</option>
							))}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-audit-result">結果</label>
						<select
							id="adm-audit-result"
							value={result}
							onChange={(event) => {
								setResult(event.target.value as DelegatedAuditResult | "");
								setPage(1);
							}}
						>
							<option value="">全部</option>
							{DELEGATED_AUDIT_RESULTS.map((value) => (
								<option key={value} value={value}>
									{DELEGATED_AUDIT_RESULT_LABELS[value]}
								</option>
							))}
						</select>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-audit-date-from">起始時間</label>
						<input
							id="adm-audit-date-from"
							type="datetime-local"
							value={dateFrom}
							onChange={(event) => {
								setDateFrom(event.target.value);
								setPage(1);
							}}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-audit-date-to">結束時間</label>
						<input
							id="adm-audit-date-to"
							type="datetime-local"
							value={dateTo}
							onChange={(event) => {
								setDateTo(event.target.value);
								setPage(1);
							}}
						/>
					</div>
					<Button
						variant="outline"
						type="button"
						onClick={resetFilters}
						disabled={!hasFilters}
					>
						重設
					</Button>
				</section>

				{auditQuery.isError ? (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>
							{errorMessageOf(
								auditQuery.error,
								"無法載入代行稽核紀錄，請稍後再試。",
							)}
						</div>
					</div>
				) : (
					<div className="adm-audit-table-wrap">
						<table className="adm-audit-table">
							<thead>
								<tr>
									<th scope="col">時間</th>
									<th scope="col">操作</th>
									<th scope="col">目標資源</th>
									{/* 理由是這張表最該被讀的一欄，卻是唯一既沒 nowrap 也沒下限的
										    欄（時間與結果吃得到 td:first-child／td:last-child 的保護）
										    ⇒ auto layout 把 col-summary 讓出來的壓力全丟給它，實測壓成
										    每行一兩個字的窄柱。下限給在 class 上，理由同 col-summary。 */}
									<th scope="col" className="adm-audit-col-reason">
										理由
									</th>
									{/* 這一欄是全表唯一可收縮的（其餘不是 nowrap 就是內容極短），
										    auto layout 於是把剩餘壓力全丟給它。給它一個具名 class 而不是
										    nth-child：欄位增刪時 nth-child 會靜默指到別欄。 */}
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
											{auditQuery.isLoading
												? "正在載入代行紀錄…"
												: hasFilters
													? "目前的篩選條件沒有符合的代行紀錄。"
													: "這個工作階段還沒有任何代行操作。"}
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
											<td>{formatClockTime(entry.createdAt)}</td>
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
											<td className="adm-audit-col-summary">
												{changeSummaryOf(entry)}
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

				{auditPage && auditPage.total > 0 && (
					<div className="adm-audit-pagination">
						{/* 有篩選時 total 是篩選後的數，與上方摘要的口徑不同——不講明，
							    讀者會以為這個工作階段的紀錄總數變少了。 */}
						<span>
							顯示第 {(auditPage.page - 1) * auditPage.pageSize + 1} 至{" "}
							{Math.min(auditPage.page * auditPage.pageSize, auditPage.total)}{" "}
							筆，
							{hasFilters
								? `符合篩選條件的共 ${auditPage.total} 筆`
								: `共 ${auditPage.total} 筆紀錄（含成功、失敗與結果未確認）`}
						</span>
						<div>
							<Button
								variant="outline"
								type="button"
								disabled={auditPage.page <= 1}
								onClick={() => setPage((current) => Math.max(1, current - 1))}
							>
								上一頁
							</Button>
							<span className="adm-audit-page-indicator">
								{auditPage.page} / {totalPages}
							</span>
							<Button
								variant="outline"
								type="button"
								disabled={auditPage.page >= totalPages}
								onClick={() => setPage((current) => current + 1)}
							>
								下一頁
							</Button>
						</div>
					</div>
				)}
			</div>

			{selected && (
				<AuditDetailDialog
					entry={selected}
					auditRetentionDays={report?.auditRetentionDays ?? null}
					onClose={() => setSelectedId(null)}
				/>
			)}

			{revokeOpen && report && (
				<SessionRevokeDialog
					report={report}
					onClose={() => setRevokeOpen(false)}
					onRevoked={() => {
						setRevokeOpen(false);
						setRevokeNotice(
							"已撤銷這個代行工作階段，工作區的後續操作都會被拒絕。",
						);
						void queryClient.invalidateQueries({
							queryKey: ["delegated-governance", sessionId],
						});
					}}
				/>
			)}
		</div>
	);
}
