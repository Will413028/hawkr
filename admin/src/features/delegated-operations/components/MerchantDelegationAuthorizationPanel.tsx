"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Button } from "@/components/UI";
import {
	delegationScopeDelta,
	formatClockTime,
} from "@/features/delegated-operations/format";
import {
	DELEGATION_AUTHORIZATION_SOURCE_LABELS,
	delegationCapabilityLabel,
} from "@/features/delegated-operations/labels";
import {
	delegationAuthorizationErrorText,
	getDelegationAuthorization,
	getDelegationCapabilityCatalog,
	listDelegationAuthorizationHistory,
} from "@/services/delegated-operations";
import type { DelegationAuthorization } from "../authorization";
import DelegationAuthorizationDialog from "./DelegationAuthorizationDialog";
import DelegationAuthorizationRevokeDialog from "./DelegationAuthorizationRevokeDialog";

/**
 * 商家詳情頁「代行授權」分頁（backend#2444）。
 *
 * 這一頁答的是「這家店可不可以被代行、範圍多大」，與同一頁其他分頁旁的「開始代行
 * 營運」入口是兩件事：那顆按鈕建立**一次**工作階段，這裡決定它能不能建得起來。
 *
 * ⚠️ 兩種狀態都要服務，而且不能長得像錯誤：後端對「尚未開通」回 200 ＋
 * `exists=false`，不是 404。把它畫成錯誤畫面，操作者會去找維運而不是按下開通。
 *
 * ⚠️ 分頁的 `storeId` 是 `sellers.store_id` surrogate（URL 上的 `params.id`），
 * 不是 `account_id`：授權列以 store_id 為鍵。
 */

const HISTORY_PAGE_SIZE = 10;

interface MerchantDelegationAuthorizationPanelProps {
	storeId: string;
	storeName: string;
}

function capabilityList(codes: readonly string[]): string {
	return codes.map(delegationCapabilityLabel).join("、") || "—";
}

/**
 * 「範圍變更」欄的內容。
 *
 * 只畫差異，完整清單收進可展開的面板 —— 兩份完整清單並排會把這一格撐成四五行，而
 * 「這一次動了什麼」正是集合減法的結果，讓讀的人自己算等於這張表沒做完它的工作。
 *
 * 「範圍未變更」不寫成「—」：那與「首次開通」（before 為空）在畫面上長得一樣，而兩者
 * 是相反的事。首次開通也不寫成「＋ 九項全部」了事 —— 標出「首次開通」才看得出這是開通
 * 而不是擴權。
 */
function ScopeChangeCell({
	before,
	after,
	expanded,
	onToggle,
}: {
	before: readonly string[];
	after: readonly string[];
	expanded: boolean;
	onToggle: () => void;
}) {
	const { added, removed } = delegationScopeDelta(before, after);
	const unchanged = added.length === 0 && removed.length === 0;
	// 差異已經等於全貌時就不掛展開鈕：首次開通（before 為空 ⇒ added ≡ after）與撤回
	// （after 為空 ⇒ removed ≡ before）沒有任何東西被收起來，那顆鈕只會白佔一行。
	const hidesAnything = before.length > 0 && after.length > 0;
	return (
		<>
			{unchanged ? (
				<div className="adm-scope-delta adm-scope-delta--none">範圍未變更</div>
			) : (
				<>
					{added.length > 0 && (
						<div className="adm-scope-delta adm-scope-delta--added">
							{before.length === 0 ? "首次開通：" : null}＋{" "}
							{capabilityList(added)}
						</div>
					)}
					{removed.length > 0 && (
						<div className="adm-scope-delta adm-scope-delta--removed">
							－ {capabilityList(removed)}
						</div>
					)}
				</>
			)}
			{hidesAnything && (
				<button
					type="button"
					className="adm-audit-detail-trigger adm-scope-full-toggle"
					aria-expanded={expanded}
					onClick={onToggle}
				>
					{expanded ? "收合完整範圍" : "查看完整範圍"}
				</button>
			)}
			{expanded && (
				<dl className="adm-scope-full">
					<div>
						<dt>變更前</dt>
						<dd>{capabilityList(before)}</dd>
					</div>
					<div>
						<dt>變更後</dt>
						<dd>{capabilityList(after)}</dd>
					</div>
				</dl>
			)}
		</>
	);
}

/**
 * 「為什麼無效」。`effective` 是伺服端以 now() 逐列評估的結果，**這裡只負責解釋它**
 * ——不從三個時間欄自行推導 effective 與否，客戶端重算一次就會有兩套答案，而其中
 * 一套會在邊界時刻說謊。
 */
function ineffectiveReason(authorization: DelegationAuthorization): string {
	if (authorization.revokedAt) {
		return `已於 ${formatClockTime(authorization.revokedAt)} 撤回`;
	}
	if (authorization.effectiveUntil) {
		return `期限至 ${formatClockTime(authorization.effectiveUntil)}`;
	}
	return `生效時間為 ${formatClockTime(authorization.effectiveFrom)}`;
}

export default function MerchantDelegationAuthorizationPanel({
	storeId,
	storeName,
}: MerchantDelegationAuthorizationPanelProps) {
	const queryClient = useQueryClient();
	const [dialogMode, setDialogMode] = useState<"grant" | "update" | null>(null);
	const [revokeOpen, setRevokeOpen] = useState(false);
	const [notice, setNotice] = useState<string | null>(null);
	const [historyPage, setHistoryPage] = useState(1);
	// 一次只展開一列完整範圍（同 SessionAuditConsole 的 selectedId）：多列同時展開
	// 就把剛剛收掉的那面高度牆又蓋回來。
	const [expandedScopeId, setExpandedScopeId] = useState<string | null>(null);

	const authorizationQuery = useQuery({
		queryKey: ["delegation-authorization", storeId],
		queryFn: () => getDelegationAuthorization(storeId),
		enabled: !!storeId,
		retry: false,
	});
	const lookup = authorizationQuery.data;
	const authorization = lookup?.authorization ?? null;

	// 歷程只在確定有授權列時才查：從未開通過的店回 9286／404，照打會在畫面上生出
	// 一則與「尚未開通」互相矛盾的錯誤。
	const historyQuery = useQuery({
		queryKey: ["delegation-authorization", storeId, "history", historyPage],
		queryFn: () =>
			listDelegationAuthorizationHistory(storeId, {
				page: historyPage,
				pageSize: HISTORY_PAGE_SIZE,
			}),
		enabled: !!storeId && lookup?.exists === true,
		retry: false,
	});

	/**
	 * 能力目錄只在對話框開啟時才取（`enabled`），而不是進分頁就抓：它是全域常數型的
	 * 資料，每開一次商家頁就打一次是白費的往返。取不到就 fail-closed —— 對話框保持
	 * 在 blocked 狀態，不退回本地依賴表。
	 */
	const catalogQuery = useQuery({
		queryKey: ["delegation-capability-catalog"],
		queryFn: getDelegationCapabilityCatalog,
		enabled: dialogMode !== null,
		retry: false,
		staleTime: 5 * 60_000,
	});

	const refresh = () => {
		void queryClient.invalidateQueries({
			queryKey: ["delegation-authorization", storeId],
		});
	};

	if (authorizationQuery.isLoading) {
		return (
			<div className="adm-authorization-panel">
				<div className="adm-operation-preflight-loading" role="status">
					<MaterialSymbol name="progress_activity" />
					<div>
						<strong>正在讀取代行授權設定</strong>
						<span>授權狀態由伺服器判定，畫面不自行推算。</span>
					</div>
				</div>
			</div>
		);
	}

	if (authorizationQuery.isError || !lookup) {
		return (
			<div className="adm-authorization-panel">
				<div className="adm-operation-preflight-blocked" role="alert">
					<MaterialSymbol name="gpp_bad" />
					<div>
						<strong>無法讀取代行授權設定</strong>
						<p>
							{delegationAuthorizationErrorText(
								authorizationQuery.error,
								"目前無法確認這家店的代行授權，請稍後再試。",
							)}
						</p>
					</div>
				</div>
				<div className="adm-authorization-actions">
					<Button
						variant="outline"
						onClick={() => void authorizationQuery.refetch()}
					>
						重新讀取
					</Button>
				</div>
			</div>
		);
	}

	const history = historyQuery.data;
	const historyTotalPages = history
		? Math.max(1, Math.ceil(history.total / HISTORY_PAGE_SIZE))
		: 1;
	const readCapabilities =
		authorization?.grantedCapabilities.filter((capability) =>
			capability.endsWith(".read"),
		) ?? [];
	const writeCapabilities =
		authorization?.grantedCapabilities.filter(
			(capability) => !capability.endsWith(".read"),
		) ?? [];

	return (
		<div className="adm-authorization-panel">
			{notice && (
				<div className="adm-authorization-notice" role="status">
					<MaterialSymbol name="check_circle" />
					<div>{notice}</div>
				</div>
			)}

			{!lookup.exists || !authorization ? (
				<section className="adm-authorization-empty">
					<MaterialSymbol name="shield_person" />
					<div>
						<strong>尚未開通代行授權</strong>
						<p>
							這家店目前不能被 ADM
							代行。開通後仍需逐次建立工作階段，且每一次操作都會留下稽核紀錄。
						</p>
					</div>
					<Button
						onClick={() => {
							setNotice(null);
							setDialogMode("grant");
						}}
					>
						開通授權
					</Button>
				</section>
			) : (
				<>
					<section className="adm-authorization-summary">
						<div className="adm-authorization-summary-head">
							<span
								className={`adm-authorization-state${authorization.effective ? " is-effective" : ""}`}
							>
								<MaterialSymbol
									name={authorization.effective ? "verified_user" : "gpp_bad"}
								/>
								{authorization.effective ? "授權中" : "目前無效"}
							</span>
							{!authorization.effective && (
								<span className="adm-authorization-state-reason">
									{ineffectiveReason(authorization)}
								</span>
							)}
						</div>

						<dl className="adm-authorization-facts">
							<div>
								<dt>授權依據</dt>
								<dd>
									{DELEGATION_AUTHORIZATION_SOURCE_LABELS[authorization.source]}
									｜{authorization.sourceReference}
								</dd>
							</div>
							<div>
								<dt>生效時間</dt>
								<dd>{formatClockTime(authorization.effectiveFrom)}</dd>
							</div>
							<div>
								<dt>到期時間</dt>
								<dd>
									{authorization.effectiveUntil
										? formatClockTime(authorization.effectiveUntil)
										: "無期限"}
								</dd>
							</div>
							<div>
								<dt>核准者</dt>
								<dd>
									{authorization.approvedByAdminName}（
									{authorization.approvedByAdminId}）·{" "}
									{formatClockTime(authorization.approvedAt)}
								</dd>
							</div>
							<div>
								<dt>進行中的代行工作階段</dt>
								<dd>{lookup.activeSessionCount} 個</dd>
							</div>
							{authorization.revokedAt && (
								<div className="adm-authorization-facts__wide">
									<dt>撤回紀錄</dt>
									<dd>
										{formatClockTime(authorization.revokedAt)} 由{" "}
										{authorization.revokedByAdminName ?? "—"} 撤回：
										{authorization.revokeReason ?? "—"}
									</dd>
								</div>
							)}
						</dl>

						<div className="adm-operation-scope-groups">
							<div className="adm-operation-scope-group">
								<strong>
									<MaterialSymbol name="visibility" />
									可查看
								</strong>
								{readCapabilities.length > 0 ? (
									<div className="adm-operation-scope-list">
										{readCapabilities.map((capability) => (
											<span className="adm-operation-scope" key={capability}>
												{delegationCapabilityLabel(capability)}
											</span>
										))}
									</div>
								) : (
									<span className="adm-operation-scope-empty">
										此授權沒有任何檢視權限
									</span>
								)}
							</div>
							<div className="adm-operation-scope-group adm-operation-scope-group--write">
								<strong>
									<MaterialSymbol name="edit" />
									可變更
								</strong>
								{writeCapabilities.length > 0 ? (
									<div className="adm-operation-scope-list">
										{writeCapabilities.map((capability) => (
											<span className="adm-operation-scope" key={capability}>
												{delegationCapabilityLabel(capability)}
											</span>
										))}
									</div>
								) : (
									<span className="adm-operation-scope-empty">
										此授權沒有內容寫入權限
									</span>
								)}
							</div>
						</div>

						<div className="adm-authorization-actions">
							{authorization.revokedAt ? (
								<Button
									onClick={() => {
										setNotice(null);
										setDialogMode("grant");
									}}
								>
									重新開通
								</Button>
							) : (
								<>
									<Button
										variant="outline"
										onClick={() => {
											setNotice(null);
											setDialogMode("update");
										}}
									>
										調整授權範圍
									</Button>
									<Button
										variant="danger"
										onClick={() => {
											setNotice(null);
											setRevokeOpen(true);
										}}
									>
										撤回授權
									</Button>
								</>
							)}
						</div>
					</section>

					<section
						className="adm-authorization-history"
						aria-labelledby="adm-authorization-history-title"
					>
						<h3 id="adm-authorization-history-title">授權變更歷程</h3>
						<p>
							每店終生一列授權，復權會就地覆寫前一次的撤回欄位——「曾經被撤回過」只在這張表上留得下來。
						</p>
						{historyQuery.isError ? (
							<div className="adm-operation-error" role="alert">
								<MaterialSymbol name="error" />
								<div>
									{delegationAuthorizationErrorText(
										historyQuery.error,
										"目前無法讀取授權變更歷程。",
									)}
								</div>
							</div>
						) : (
							<div className="adm-audit-table-wrap">
								<table className="adm-audit-table">
									<thead>
										<tr>
											<th scope="col">時間</th>
											<th scope="col" className="adm-audit-col-action">
												動作
											</th>
											<th scope="col">操作者</th>
											{/* 全表唯一可收縮的那一欄（CSS 註解同此）。⚠️ 這個 class 只能掛在
											    「一」欄上：掛兩欄會讓它們各自宣告 width:100%，auto layout 把
											    剩餘壓力全丟給沒宣告的欄，「動作」就被壓成一字一行。 */}
											<th scope="col" className="adm-audit-col-summary">
												範圍變更
											</th>
											{/* 這一張的理由欄是 td:last-child ⇒ 吃到那條 nowrap，而
											    operationReason 上限 200 字，一則長理由會被拉成一整行、
											    把整張表推進橫捲。同一個 class 一併把它換回可換行。 */}
											<th scope="col" className="adm-audit-col-reason">
												理由
											</th>
										</tr>
									</thead>
									<tbody>
										{historyQuery.isLoading ? (
											<tr>
												<td colSpan={5} className="adm-audit-empty">
													讀取中…
												</td>
											</tr>
										) : history && history.records.length > 0 ? (
											history.records.map((entry) => (
												<tr key={entry.id}>
													<td>{formatClockTime(entry.createdAt)}</td>
													<td className="adm-audit-col-action">
														<strong>{entry.actionLabel}</strong>
													</td>
													<td>
														<strong>{entry.actorDisplayName}</strong>
														<span>{entry.actorAdminId}</span>
													</td>
													<td className="adm-audit-col-summary">
														<ScopeChangeCell
															before={entry.capabilitiesBefore}
															after={entry.capabilitiesAfter}
															expanded={expandedScopeId === entry.id}
															onToggle={() =>
																setExpandedScopeId((current) =>
																	current === entry.id ? null : entry.id,
																)
															}
														/>
													</td>
													<td className="adm-audit-col-reason">
														{entry.operationReason || "—"}
													</td>
												</tr>
											))
										) : (
											<tr>
												<td colSpan={5} className="adm-audit-empty">
													尚無授權變更紀錄
												</td>
											</tr>
										)}
									</tbody>
								</table>
							</div>
						)}
						{history && history.total > HISTORY_PAGE_SIZE && (
							<div className="adm-audit-pagination">
								<span className="adm-audit-page-indicator">
									第 {history.page} / {historyTotalPages} 頁 · 共{" "}
									{history.total} 筆
								</span>
								<div>
									<Button
										variant="outline"
										size="sm"
										disabled={historyPage <= 1}
										onClick={() =>
											setHistoryPage((page) => Math.max(1, page - 1))
										}
									>
										上一頁
									</Button>
									<Button
										variant="outline"
										size="sm"
										disabled={historyPage >= historyTotalPages}
										onClick={() =>
											setHistoryPage((page) =>
												Math.min(historyTotalPages, page + 1),
											)
										}
									>
										下一頁
									</Button>
								</div>
							</div>
						)}
					</section>
				</>
			)}

			{dialogMode && (
				<DelegationAuthorizationDialog
					mode={dialogMode}
					storeId={storeId}
					storeName={storeName}
					current={authorization}
					catalog={catalogQuery.data}
					catalogLoading={catalogQuery.isLoading || catalogQuery.isFetching}
					catalogError={
						catalogQuery.error
							? {
									title: "目前無法設定代行授權",
									message: delegationAuthorizationErrorText(
										catalogQuery.error,
										"尚未取得代行能力目錄，已停止開啟設定表單。",
									),
								}
							: undefined
					}
					onRetryCatalog={() => void catalogQuery.refetch()}
					onClose={() => setDialogMode(null)}
					onSaved={(message) => {
						setDialogMode(null);
						setNotice(message);
						setHistoryPage(1);
						refresh();
					}}
				/>
			)}

			{revokeOpen && authorization && (
				<DelegationAuthorizationRevokeDialog
					storeId={storeId}
					storeName={storeName}
					authorization={authorization}
					activeSessionCount={lookup.activeSessionCount}
					onClose={() => setRevokeOpen(false)}
					onRevoked={(message) => {
						setRevokeOpen(false);
						setNotice(message);
						setHistoryPage(1);
						refresh();
					}}
				/>
			)}
		</div>
	);
}
