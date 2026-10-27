"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { useState } from "react";
import { delegationAuthorizationErrorMessage } from "@/features/delegated-operations/authorization";
import {
	DelegatedOperationClientError,
	getDelegatedOperationPreflight,
} from "@/services/delegated-operations";
import OperationPreflightDialog from "./OperationPreflightDialog";

/**
 * 建立工作階段走頂層表單導覽（跨 host 的 303 只有瀏覽器跟得動），失敗因此
 * 是一次導回本頁的 303 而不是一個 fetch 的例外 —— 原因碼由 query 帶回，在這裡
 * 譯回人看得懂的句子。
 */
const START_ERROR_MESSAGES: Record<string, string> = {
	DELEGATED_OPERATION_UNAVAILABLE: "此環境尚未啟用代行營運服務。",
	DELEGATED_OPERATION_DENIED: "此店家目前不可建立代行工作階段。",
	DELEGATED_OPERATION_REASON_DENIED:
		"代行原因不在目前核准範圍內，請重新確認資格後選擇。",
	CASE_REFERENCE_REQUIRED: "此代行原因需要可追溯的案件編號。",
	INVALID_DELEGATED_OPERATION_CONTRACT:
		"代行營運回應無法驗證，已停止建立工作階段。",
	DELEGATED_OPERATION_ALREADY_REDEEMED:
		"這份代行申請已進入工作區，無法再次使用同一份申請。請先結束現有工作階段。",
	DELEGATED_OPERATION_RECONCILIATION_FAILED:
		"無法確認剛建立的代行工作階段，請直接重試（會沿用同一份申請）。",
	DELEGATED_OPERATION_RECONCILIATION_MISMATCH:
		"剛建立的代行工作階段無法安全核對，已停止進入工作區。",
	WEB_ORIGIN_NOT_CONFIGURED:
		"這個環境沒有設定代行工作區位址，請聯絡維運人員確認 WEB_ORIGIN。",
	UPSTREAM_TIMEOUT: "建立代行營運工作階段逾時，請直接重試。",
	UPSTREAM_UNREACHABLE: "無法連線至代行營運服務。",
	UPSTREAM_FAILURE: "代行營運服務暫時無法使用，請稍後再試。",
};

function startErrorMessage(code: string): string {
	const fallback = `代行營運服務拒絕了這次申請（代碼 ${code}）。`;
	const known = START_ERROR_MESSAGES[code];
	if (known) return known;
	// 純數字 ⇒ 上游的業務碼（BFF 的 `safeUpstreamError` 會原樣帶過來）。文案走
	// `authorization.ts` 那份既有的「業務碼 → 下一步」對照，不在這裡另建第二份：
	// 兩份對照必然各自漂移，而漂移的方向剛好是同一個碼在兩個畫面上講不同的話。
	if (/^\d+$/u.test(code)) {
		return delegationAuthorizationErrorMessage(Number(code), fallback);
	}
	return fallback;
}

interface MerchantOperationEntryProps {
	storeId: string;
	/**
	 * 操作者自己能不能建立代行工作階段。false ⇒ 整個入口不渲染 —— 設計書 6.1
	 * 驗收明文：「沒有 session-create capability 時，代行入口不顯示；不能只用
	 * disabled button 掩飾權限。」
	 */
	enabled: boolean;
	/**
	 * 目標店家不合格的原因。有值 ⇒ 按鈕照樣渲染，但停用並以原因當 label（設計書
	 * 6.1 圖 3 的停權列）。與 `enabled` 是兩回事：那個講操作者的權限、這個講這一
	 * 家店的狀態。混成同一個判斷，管理員看到的就是「有些列有按鈕、有些沒有」而
	 * 無從得知為什麼。
	 */
	blockedReason?: string;
	/**
	 * 進場就向伺服器問一次資格，並讓按鈕的可用性反映答案（設計書 6.1 S-1：preflight
	 * 就是「入口按鈕的可用性」，觸發點含詳情頁）。
	 *
	 * 只給單店畫面用。列表頁一頁十列，開了等於每次 render 批量核發十次 preflight，
	 * 所以那一側維持 lazy，改用 `blockedReason` 的本地判準先擋。
	 *
	 * 與 `blockedReason` 的分工：那個是前端已知的店家狀態（列表頁手上就有
	 * `lifecycleStatus`，零請求），這個是伺服器的權威判定 —— 詳情頁的 profile 根本
	 * 沒有 `lifecycleStatus`，硬補一個本地判準就是造第二份判準。
	 */
	resolveEligibility?: boolean;
	label?: string;
	compact?: boolean;
}

export default function MerchantOperationEntry({
	storeId,
	enabled,
	blockedReason,
	resolveEligibility = false,
	label = "開始代行營運",
	compact = false,
}: MerchantOperationEntryProps) {
	const [dialogOpen, setDialogOpen] = useState(false);
	const searchParams = useSearchParams();
	const startErrorCode =
		searchParams.get("delegation_store") === storeId
			? searchParams.get("delegation_error")
			: null;
	const resumedIdempotencyKey = startErrorCode
		? (searchParams.get("delegation_key") ?? undefined)
		: undefined;
	const startErrorTraceId = startErrorCode
		? searchParams.get("delegation_trace")
		: null;
	// 建立被拒的那一次 303 只帶得回機器碼，帶不回原因（原因是自由文字，塞進
	// query 等於把可反射的文字放進網址）。改為重問一次 preflight —— 它是唯讀、
	// 明文標示可安全重複呼叫的端點，且回的是**當下**的權威原因。
	const startDenied = startErrorCode === "DELEGATED_OPERATION_DENIED";
	const preflightQuery = useQuery({
		queryKey: ["delegated-operation", "preflight", storeId],
		queryFn: () => getDelegatedOperationPreflight(storeId),
		// 商家列表可能同時顯示多個合格店家。只有管理員主動開啟對話框後
		// 才向伺服器確認資格，避免每次列表 render 都批量核發 preflight。
		// `resolveEligibility` 是單店畫面的例外（見 props 註解）。
		enabled: enabled && (resolveEligibility || dialogOpen || startDenied),
		retry: false,
		staleTime: 30_000,
		// 資格會在別的分頁被改掉（自己在另一家店開了工作階段、同事撤了授權）。
		// 詳情頁的按鈕既然宣稱反映當下資格，回到這個分頁時就得重問一次。
		refetchOnWindowFocus: dialogOpen || resolveEligibility,
	});

	if (!enabled) return null;

	// 不合格的店家留一顆停用按鈕在原位：操作欄空白只說得出「這一列不能按」，說不出
	// 為什麼，而原因就在該列自己的狀態裡（不必再問伺服器一次）。
	if (blockedReason) {
		return (
			<div className="adm-operation-entry">
				<button
					type="button"
					className={`adm-operation-trigger${compact ? " adm-operation-trigger--compact" : ""}`}
					disabled
				>
					{blockedReason}
				</button>
			</div>
		);
	}

	const error = preflightQuery.error;
	const targetMismatch =
		preflightQuery.data !== undefined &&
		preflightQuery.data.target.storeId !== storeId;
	const loadError = targetMismatch
		? {
				title: "目標店家不相符",
				message:
					"伺服器回傳的目標店家與目前選擇不相符，已阻擋建立工作階段。請關閉後重新選擇店家。",
			}
		: error
			? {
					title:
						error instanceof DelegatedOperationClientError &&
						error.status === 403
							? // 中性標題：403 的成因不只「這家店不行」——9268 是「你自己
								// 已有進行中的工作階段」，店家其實沒問題。真正的原因在
								// message（後端下發的 deniedLabel）。
								"目前無法開始代行"
							: error instanceof DelegatedOperationClientError &&
									(error.status === 404 || error.status === 503)
								? "代行營運目前未啟用"
								: "無法確認代行資格",
					message:
						error instanceof Error
							? error.message
							: "目前無法確認代行資格，請稍後再試。",
					traceId:
						error instanceof DelegatedOperationClientError
							? error.traceId
							: undefined,
					deniedCode:
						error instanceof DelegatedOperationClientError
							? error.deniedCode
							: undefined,
				}
			: undefined;

	// preflight 回 403 ⇒ 這一刻不能開始。原因是後端下發的 deniedLabel：9286「從未
	// 開通」、9264「已撤回或已到期」、9272「已停權或關閉」、9269「別人正在代行」
	// 的下一步各不相同，壓成同一句就把後端已經講清楚的事丟掉了。
	const preflightDenial =
		error instanceof DelegatedOperationClientError && error.status === 403
			? error
			: null;

	// 建立失敗導回本頁時，優先顯示重問 preflight 拿到的當下原因；拒絕已解除
	// （preflight 這次通過）就退回通用句 —— 那是一件已經發生過的事，不能宣稱
	// 它現在還被拒。
	const currentDenial = startDenied ? preflightDenial : null;

	// 解析資格的畫面上，按鈕本身就是答案（設計書 6.1 的拒絕分支表：「入口按鈕不可用、
	// 顯示原因」）。這與 entry-gating ①「不得用 disabled 掩飾權限」不衝突：那條守的是
	// 前端已知的 session-create capability（`enabled`，已在上面早退），而 deniedLabel 是
	// 伺服器逐次下發的原因，把它畫出來正是「不要掩飾」。
	//
	// 只有 403 進這裡。404／503／502 是「問不到」不是「不行」，那時保留可按的入口讓對話
	// 框顯示錯誤與重試 —— 把暫時性故障畫成永久停用，管理員連重試的入口都沒有；伺服器端
	// 本來就 fail-closed，前端不必替它再擋一次。
	const eligibilityGateLabel = !resolveEligibility
		? undefined
		: preflightDenial
			? preflightDenial.message
			: preflightQuery.isLoading
				? "確認代行資格中…"
				: undefined;

	return (
		<div className="adm-operation-entry">
			{eligibilityGateLabel ? (
				<button
					type="button"
					className={`adm-operation-trigger${compact ? " adm-operation-trigger--compact" : ""}`}
					disabled
				>
					{eligibilityGateLabel}
				</button>
			) : (
				<button
					type="button"
					className={`adm-operation-trigger${compact ? " adm-operation-trigger--compact" : ""}`}
					onClick={() => setDialogOpen(true)}
					aria-haspopup="dialog"
					aria-expanded={dialogOpen}
					aria-label={`${label}（需再次確認資格與範圍）`}
				>
					<MaterialSymbol name="manage_accounts" />
					{label}
				</button>
			)}
			{startErrorCode && (
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>
						{currentDenial
							? currentDenial.message
							: startErrorMessage(startErrorCode)}
						{currentDenial?.deniedCode ? (
							<span className="adm-operation-trace">
								原因代碼：{currentDenial.deniedCode}
							</span>
						) : null}
						{startErrorTraceId && (
							<span className="adm-operation-trace">
								追蹤編號：{startErrorTraceId}
							</span>
						)}
					</div>
				</div>
			)}
			{dialogOpen && (
				<OperationPreflightDialog
					preflight={targetMismatch ? undefined : preflightQuery.data}
					requestedStoreId={storeId}
					loading={preflightQuery.isLoading || preflightQuery.isFetching}
					loadError={loadError}
					resumedIdempotencyKey={resumedIdempotencyKey}
					onRetry={() => void preflightQuery.refetch()}
					onClose={() => setDialogOpen(false)}
				/>
			)}
		</div>
	);
}
