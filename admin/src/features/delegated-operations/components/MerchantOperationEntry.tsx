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
	const startDenied = startErrorCode === "DELEGATED_OPERATION_DENIED";
	const preflightQuery = useQuery({
		queryKey: ["delegated-operation", "preflight", storeId],
		queryFn: () => getDelegatedOperationPreflight(storeId),
		enabled: enabled && (resolveEligibility || dialogOpen || startDenied),
		retry: false,
		staleTime: 30_000,
		refetchOnWindowFocus: dialogOpen || resolveEligibility,
	});

	if (!enabled) return null;

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
							? "目前無法開始代行"
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

	const preflightDenial =
		error instanceof DelegatedOperationClientError && error.status === 403
			? error
			: null;

	const currentDenial = startDenied ? preflightDenial : null;

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
