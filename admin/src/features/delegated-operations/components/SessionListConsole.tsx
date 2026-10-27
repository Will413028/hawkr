"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { type FormEvent, useState } from "react";
import { Button } from "@/components/UI";
import { delegationAuthorizationErrorMessage } from "@/features/delegated-operations/authorization";
import {
	formatClockTime,
	localInputToUtcInstant,
} from "@/features/delegated-operations/format";
import {
	DELEGATED_SESSION_STATUS_LABELS,
	delegationCapabilityLabel,
} from "@/features/delegated-operations/labels";
import {
	DelegatedOperationClientError,
	listDelegatedOperationSessions,
} from "@/services/delegated-operations";
import {
	delegatedSessionListQueryKey,
	isDelegatedNumericId,
	resolveDelegatedLookup,
} from "../governance";

/**
 * 代行紀錄的入口：**跨工作階段的工作階段列表**（gateway 的
 * `GET /api/v1/admin/delegated-operations`，56 §11.3 P1）。
 *
 * 這裡曾經只是一個查詢框，理由寫著「本期沒有工作階段列表端點」——那句話在該端點
 * 交付之後就不再成立，而它造成的實際後果是：營運要回看代行紀錄，得先從別的地方
 * （建立當下的回應，或請後端工程師代跑 SQL）拿到工作階段 ID。§11.3 P1 的觸發條件
 * 之一正是「營運開始自己手寫 SQL 查 session_id 並且填錯」。
 *
 * ⚠️ 這一支列得出**全平台**的代行活動（回應含其他管理員的 actor 與其他店家的
 * target），而 `requireAdmin()` 之下任一 admin 都看得到。那與既有的單一工作階段查詢
 * （`/{sessionId}/summary`、`/{sessionId}/audit-logs`）是**同一個暴露面**——那兩支從
 * 來就沒有「只能看自己開的」這道限制，猜得到 ID 就讀得到。列表把它從「猜得到」變成
 * 「列得出來」，是可用性的差別，不是權限邊界的差別。要分級是另一件事，屬於後端的
 * capability 層（56 §7），不由前端隱藏入口來假裝。
 *
 * 直查與列表兩者並存、互不取代：已經知道數字 ID 時直接跳詳情頁比在列表裡翻快得多。
 */

const PAGE_SIZE = 20;

interface FilterDraft {
	actorAdminId: string;
	storeId: string;
	dateFrom: string;
	dateTo: string;
}

const EMPTY_FILTERS: FilterDraft = {
	actorAdminId: "",
	storeId: "",
	dateFrom: "",
	dateTo: "",
};

/**
 * 同 `SessionAuditConsole`：一律以業務碼分派，`error.message` 只當 fallback。
 *
 * BFF 的 `safeUpstreamError` 對每一個上游錯誤都送同一句「代行營運服務暫時無法使用」，
 * 只讀 message 的話「查詢條件被拒」與「上游掛了」在畫面上一字不差。
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

export default function SessionListConsole() {
	const router = useRouter();
	const [lookup, setLookup] = useState("");
	const [lookupError, setLookupError] = useState<string | null>(null);
	const [displayCode, setDisplayCode] = useState("");
	const [draft, setDraft] = useState<FilterDraft>(EMPTY_FILTERS);
	const [applied, setApplied] = useState<FilterDraft>(EMPTY_FILTERS);
	const [filterError, setFilterError] = useState<string | null>(null);
	const [page, setPage] = useState(1);

	const query = {
		page,
		pageSize: PAGE_SIZE,
		actorAdminId: applied.actorAdminId,
		storeId: applied.storeId,
		displayCode,
		dateFrom: localInputToUtcInstant(applied.dateFrom),
		dateTo: localInputToUtcInstant(applied.dateTo),
	};

	const sessionsQuery = useQuery({
		queryKey: delegatedSessionListQueryKey(query),
		queryFn: () => listDelegatedOperationSessions(query),
		retry: false,
	});

	const sessionPage = sessionsQuery.data ?? null;
	const records = sessionPage?.records ?? [];
	const totalPages = sessionPage
		? Math.max(1, Math.ceil(sessionPage.total / sessionPage.pageSize))
		: 1;
	const isBlank = (filters: FilterDraft): boolean =>
		filters.actorAdminId === "" &&
		filters.storeId === "" &&
		filters.dateFrom === "" &&
		filters.dateTo === "";
	// 「重設」要連**還沒套用的草稿**一起清掉，否則按完之後篩選框裡還留著字，而列表
	// 已經是全集——兩者互相矛盾，正是這一頁最該避免的狀態。
	const hasFilters = displayCode !== "" || !isBlank(applied);
	const canReset = hasFilters || !isBlank(draft);

	/**
	 * 查詢框收兩種識別碼（判定在 `resolveDelegatedLookup`，是純函式所以測得到）：
	 * 數字 ID 直接跳詳情頁，`OPS-` 代號套成列表的篩選。
	 */
	const handleLookup = (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		const intent = resolveDelegatedLookup(lookup);
		if (intent.kind === "session") {
			router.push(`/admin/delegated-operations/${intent.sessionId}`);
			return;
		}
		if (intent.kind === "displayCode") {
			setDisplayCode(intent.displayCode);
			setLookupError(null);
			setPage(1);
			return;
		}
		setLookupError(
			"請輸入工作階段代號（OPS- 開頭）或工作階段 ID（一組數字）後再查詢。",
		);
	};

	/**
	 * 篩選要按下「套用」才生效，不隨打字即時查。
	 *
	 * 兩個識別碼欄是自由輸入，而半截的值（打到一半的 store_id）既不合法也不是空的：
	 * 即時查的話，那個值會被丟掉而畫面回全平台的列表，操作者卻看著一個有內容的篩選框
	 * 以為自己已經縮到一家店了。這裡改成當場擋下並說出原因。
	 */
	const applyFilters = (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		for (const [value, name] of [
			[draft.actorAdminId, "操作者 ID"],
			[draft.storeId, "店家 ID"],
		] as const) {
			if (value !== "" && !isDelegatedNumericId(value)) {
				setFilterError(`${name}是一組數字，請確認後再套用。`);
				return;
			}
		}
		if (draft.dateFrom && draft.dateTo && draft.dateFrom > draft.dateTo) {
			// 後端對顛倒的區間回 9260。先擋下來，錯誤訊息才說得出是哪裡錯。
			setFilterError("起始時間不得晚於結束時間。");
			return;
		}
		setFilterError(null);
		setApplied(draft);
		setPage(1);
	};

	const resetFilters = () => {
		setDraft(EMPTY_FILTERS);
		setApplied(EMPTY_FILTERS);
		setDisplayCode("");
		setFilterError(null);
		setPage(1);
	};

	return (
		<div className="adm-audit-console">
			<header className="adm-audit-header">
				<div>
					<h1>代行紀錄</h1>
					<p>
						查詢平台上的 ADM
						代行工作階段，點進任一筆可看該次代行的操作、理由與資料差異
					</p>
				</div>
				<div className="adm-audit-header-actions">
					{/* 跨工作階段視角的入口（backend#2740）。兩者不互相取代：這一頁答
					    「誰代行了哪些店」，那一頁答「這家店／這位管理員歷來被改過什麼」。
					    端點在 PR #2723 就交付了，缺的一直是這顆按鈕。 */}
					<Link
						className="adm-audit-detail-trigger"
						href="/admin/delegated-operations/audit-logs"
					>
						跨工作階段代行紀錄
					</Link>
				</div>
			</header>

			<form className="adm-audit-lookup" onSubmit={handleLookup}>
				<div className="adm-operation-field adm-operation-field--wide">
					<label htmlFor="adm-audit-lookup">工作階段代號或 ID</label>
					<input
						id="adm-audit-lookup"
						value={lookup}
						onChange={(event) => {
							setLookup(event.target.value);
							setLookupError(null);
						}}
						autoComplete="off"
						placeholder="例如 OPS-1KXP05J5SRQPS 或 7494004863848677376"
					/>
					<span className="adm-operation-field-note">
						代號（OPS- 開頭）會套用成下方列表的篩選；數字 ID
						會直接開啟那次代行的紀錄。
					</span>
				</div>
				{lookupError && (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>{lookupError}</div>
					</div>
				)}
				<Button type="submit">查詢</Button>
			</form>

			{displayCode && (
				<div className="adm-audit-notice" role="status">
					<MaterialSymbol name="filter_list" />
					只顯示工作階段代號 {displayCode} 的紀錄
					<Button
						variant="outline"
						type="button"
						onClick={() => {
							setDisplayCode("");
							setPage(1);
						}}
					>
						清除代號篩選
					</Button>
				</div>
			)}

			<div className="adm-audit-main">
				<form className="adm-audit-filters" onSubmit={applyFilters}>
					<div className="adm-operation-field">
						<label htmlFor="adm-session-actor">操作者 ID</label>
						<input
							id="adm-session-actor"
							value={draft.actorAdminId}
							onChange={(event) =>
								setDraft((current) => ({
									...current,
									actorAdminId: event.target.value,
								}))
							}
							inputMode="numeric"
							autoComplete="off"
							placeholder="管理員 ID"
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-session-store">店家 ID</label>
						<input
							id="adm-session-store"
							value={draft.storeId}
							onChange={(event) =>
								setDraft((current) => ({
									...current,
									storeId: event.target.value,
								}))
							}
							inputMode="numeric"
							autoComplete="off"
							placeholder="store_id"
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-session-date-from">起始時間</label>
						<input
							id="adm-session-date-from"
							type="datetime-local"
							value={draft.dateFrom}
							onChange={(event) =>
								setDraft((current) => ({
									...current,
									dateFrom: event.target.value,
								}))
							}
						/>
					</div>
					<div className="adm-operation-field">
						<label htmlFor="adm-session-date-to">結束時間</label>
						<input
							id="adm-session-date-to"
							type="datetime-local"
							value={draft.dateTo}
							onChange={(event) =>
								setDraft((current) => ({
									...current,
									dateTo: event.target.value,
								}))
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

				{filterError && (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>{filterError}</div>
					</div>
				)}

				{/* 時間篩選比對的是 `issuedAt`（建立時刻），不是結束時刻——不講明的話，
				    「昨天結束的那一場」用昨天的區間會查不到。 */}
				<p className="adm-audit-summary-note">
					時間區間比對工作階段的<strong>開始時間</strong>。
				</p>

				{sessionsQuery.isError ? (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>
							{errorMessageOf(
								sessionsQuery.error,
								"無法載入代行工作階段列表，請稍後再試。",
							)}
						</div>
					</div>
				) : (
					<div className="adm-audit-table-wrap">
						<table className="adm-audit-table">
							<thead>
								<tr>
									<th scope="col">工作階段</th>
									<th scope="col">目標店家</th>
									<th scope="col">操作者</th>
									<th scope="col" className="adm-audit-col-reason">
										代行原因
									</th>
									<th scope="col" className="adm-audit-col-summary">
										授權範圍
									</th>
									<th scope="col">開始時間</th>
									<th scope="col">狀態</th>
								</tr>
							</thead>
							<tbody>
								{records.length === 0 ? (
									<tr>
										<td colSpan={7} className="adm-audit-empty">
											{sessionsQuery.isLoading
												? "正在載入代行工作階段…"
												: hasFilters
													? "目前的篩選條件沒有符合的代行工作階段。"
													: "平台上還沒有任何代行工作階段。"}
										</td>
									</tr>
								) : (
									records.map((record) => (
										<tr key={record.sessionId}>
											<td>
												<strong>{record.displayCode}</strong>
												<span>ID {record.sessionId}</span>
											</td>
											<td>
												<strong>{record.target.storeName || "—"}</strong>
												<span>#{record.target.storeId}</span>
											</td>
											<td>
												<strong>{record.actor.displayName || "—"}</strong>
												<span>#{record.actor.adminId}</span>
											</td>
											<td className="adm-audit-col-reason">
												<strong>{record.reason.label}</strong>
												{record.caseReference && (
													<span>案件編號 {record.caseReference}</span>
												)}
											</td>
											{/* 授權範圍是「這次代行拿到什麼權限」，而它正是回看時第一個
											    要問的問題。逐項列出而不是只給數量：「6 項能力」讀不出
											    到底包不包含上架。 */}
											<td className="adm-audit-col-summary">
												{record.capabilities.length === 0
													? "—"
													: record.capabilities
															.map(delegationCapabilityLabel)
															.join("、")}
											</td>
											<td>{formatClockTime(record.issuedAt)}</td>
											<td>
												<span
													className={`adm-audit-status adm-audit-status--${record.status}`}
												>
													{DELEGATED_SESSION_STATUS_LABELS[record.status]}
												</span>
												<Link
													className="adm-audit-detail-trigger"
													href={`/admin/delegated-operations/${record.sessionId}`}
												>
													查看代行紀錄
												</Link>
											</td>
										</tr>
									))
								)}
							</tbody>
						</table>
					</div>
				)}

				{sessionPage && sessionPage.total > 0 && (
					<div className="adm-audit-pagination">
						<span>
							顯示第 {(sessionPage.page - 1) * sessionPage.pageSize + 1} 至{" "}
							{Math.min(
								sessionPage.page * sessionPage.pageSize,
								sessionPage.total,
							)}{" "}
							筆，
							{hasFilters
								? `符合篩選條件的共 ${sessionPage.total} 筆`
								: `共 ${sessionPage.total} 個代行工作階段`}
						</span>
						<div>
							<Button
								variant="outline"
								type="button"
								disabled={sessionPage.page <= 1}
								onClick={() => setPage((current) => Math.max(1, current - 1))}
							>
								上一頁
							</Button>
							<span className="adm-audit-page-indicator">
								{sessionPage.page} / {totalPages}
							</span>
							<Button
								variant="outline"
								type="button"
								disabled={sessionPage.page >= totalPages}
								onClick={() => setPage((current) => current + 1)}
							>
								下一頁
							</Button>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}
