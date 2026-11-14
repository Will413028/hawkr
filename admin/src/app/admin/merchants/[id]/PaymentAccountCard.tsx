"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	createConnectQueryKey,
	useMutation as useConnectMutation,
	useTransport,
} from "@connectrpc/connect-query";
import { PaymentAdminService } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { ReviewQueueAdminService } from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	paymentPaymentAccountStatus,
	paymentRequirementCollection,
} from "@hawkr/ui-text/keys";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { type BadgeVariant, Button } from "@/components/UI";
import CopyIconButton from "@/features/delegated-operations/components/CopyIconButton";
import { connectErrorText, errorMessage } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { staffTextOrUnknown } from "@/lib/ui-text";
import { DetailCardShell } from "./DetailCardShell";
import type { GovernanceNotice } from "./GovernanceNotice";
import {
	fetchTimeMs,
	type PaymentAccountState,
	paymentAccountQueryOptions,
	REFRESH_REREAD_MAX_ATTEMPTS,
	REFRESH_REREAD_MS,
	rereadFinished,
} from "./payment-account";
import type { usePaymentAccount } from "./use-store-detail";

// 商家詳情的收款帳戶卡（PaymentAdminService 的 GetPaymentAccount、RefreshPaymentAccount、RetryPaymentAccountCreate；附件 step7-screen-drafts.md 第 3 節）。
// 徽章文字與賣家共用目錄（D24 Q6），細節看原始的 status_reason 與待補條件。

/** 錨點：審核待辦 PAYMENT_ACCOUNT_CREATE_FAILED 的連結帶這個 hash（queue-item.ts）。 */
export const PAYMENT_ACCOUNT_ANCHOR = "payment-account";

export const REFRESH_QUEUED_TEXT = "已排入重新讀取，稍後重新載入";
export const RETRY_QUEUED_TEXT = "已重新排入建立";

const STATUS_VARIANT: Readonly<
	Partial<Record<PaymentAccountStatus, BadgeVariant>>
> = {
	[PaymentAccountStatus.NOT_CREATED]: "gray",
	[PaymentAccountStatus.UNDER_REVIEW]: "warning",
	[PaymentAccountStatus.ACTION_REQUIRED]: "warning",
	[PaymentAccountStatus.ENABLED]: "success",
	[PaymentAccountStatus.DISABLED]: "danger",
};

const SHELL = {
	id: PAYMENT_ACCOUNT_ANCHOR,
	titleId: "payment-account-title",
	icon: "account_balance",
	title: "收款帳戶",
	className: "payment-account-card",
} as const;

function capabilityText(active: boolean): string {
	return active ? "已啟用" : "未啟用";
}

function Field({
	label,
	children,
}: {
	label: string;
	children: React.ReactNode;
}) {
	return (
		<div className="info-field">
			<div className="field-label">{label}</div>
			<div className="field-value">{children}</div>
		</div>
	);
}

/** 收款帳戶卡（純呈現）：資料與動作由 PaymentAccountSection 接上。 */
export function PaymentAccountCard({
	state,
	refreshing,
	retrying,
	onRefresh,
	onRetryCreate,
	onReload,
}: {
	state: PaymentAccountState;
	refreshing: boolean;
	retrying: boolean;
	onRefresh: () => void;
	onRetryCreate: () => void;
	onReload: () => void;
}) {
	if (state.state === "loading") {
		return (
			<DetailCardShell {...SHELL}>
				<p className="payment-desc" role="status">
					收款帳戶載入中…
				</p>
			</DetailCardShell>
		);
	}
	if (state.state === "error") {
		return (
			<DetailCardShell {...SHELL}>
				<p className="payment-desc" role="alert">
					收款帳戶讀取失敗
					{state.error !== undefined && `：${errorMessage(state.error)}`}
				</p>
				<div>
					<Button variant="outline" onClick={onReload}>
						重試
					</Button>
				</div>
			</DetailCardShell>
		);
	}
	if (state.state === "notFound") {
		return (
			<DetailCardShell {...SHELL}>
				<p className="payment-desc">尚未建立（平台審核核准後自動建立）</p>
			</DetailCardShell>
		);
	}
	const account = state.account;
	const badge = {
		label: staffTextOrUnknown(paymentPaymentAccountStatus(account.status)),
		variant: STATUS_VARIANT[account.status] ?? "gray",
	};
	const failedTime = account.createFailedTime;
	const requirements = account.requirements;
	return (
		<DetailCardShell {...SHELL} badge={badge}>
			<div className="info-grid">
				<Field label="金流商">{account.provider || "-"}</Field>
				<Field label="外部帳戶">
					{account.externalAccountId ? (
						<span className="payment-account-id">
							<code>{account.externalAccountId}</code>
							<CopyIconButton
								value={account.externalAccountId}
								label="複製外部帳戶 ID"
							/>
						</span>
					) : (
						"-"
					)}
				</Field>
				<Field label="收件方式">
					{staffTextOrUnknown(
						paymentRequirementCollection(account.requirementCollection),
					)}
				</Field>
				<Field label="轉帳能力">
					{capabilityText(account.transfersActive)}
				</Field>
				<Field label="撥款能力">{capabilityText(account.payoutsActive)}</Field>
				<Field label="狀態原因">{account.statusReason || "-"}</Field>
				<Field label="上次讀取">
					{account.fetchTime
						? formatDateTimeShort(timestampDate(account.fetchTime))
						: "-"}
				</Field>
			</div>
			{failedTime && (
				<p className="payment-desc payment-account-failed" role="alert">
					建立失敗於 {formatDateTimeShort(timestampDate(failedTime))}
				</p>
			)}
			{requirements && Object.keys(requirements).length > 0 && (
				<details className="payment-account-requirements">
					<summary>待補條件（原始內容）</summary>
					<pre className="adm-json-pre">
						{JSON.stringify(requirements, null, 2)}
					</pre>
				</details>
			)}
			<div className="payment-account-actions">
				{/* RefreshPaymentAccount 在外部帳戶還沒建立時回 failed_precondition：只在有外部帳戶時給這個動作。 */}
				{account.externalAccountId && (
					<Button
						variant="outline"
						onClick={onRefresh}
						loading={refreshing}
						disabled={refreshing}
					>
						重新整理
					</Button>
				)}
				{failedTime && (
					<Button
						variant="primary"
						onClick={onRetryCreate}
						loading={retrying}
						disabled={retrying}
					>
						重試建立
					</Button>
				)}
			</div>
		</DetailCardShell>
	);
}

/**
 * 商家詳情放的收款帳戶區塊：資料來自頁面的 usePaymentAccount，動作的結果走頁面的 notice（onNotice，與重新綁定同一個結果區）。
 * 重新整理後每隔 rereadMs 重讀，直到 fetch_time 變了或到次數上限；重試建立的回應直接寫進快取。
 */
export function PaymentAccountSection({
	storeId,
	payment,
	onNotice,
	rereadMs = REFRESH_REREAD_MS,
}: {
	storeId: string;
	payment: ReturnType<typeof usePaymentAccount>;
	onNotice: (notice: GovernanceNotice | null) => void;
	rereadMs?: number;
}) {
	const queryClient = useQueryClient();
	const transport = useTransport();
	const refreshMutation = useConnectMutation(
		PaymentAdminService.method.refreshPaymentAccount,
	);
	const retryMutation = useConnectMutation(
		PaymentAdminService.method.retryPaymentAccountCreate,
	);
	const rereadTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);
	// 每次重新排程或卸載就換一代，上一代還在等的重讀結果不再排下一輪。
	const generation = useRef(0);
	const stopReread = () => {
		generation.current += 1;
		clearTimeout(rereadTimer.current);
	};
	// 卸載後才回來的重新整理回應不可再排計時器（cleanup 的 stopReread 已經跑過了）。
	const mounted = useRef(false);
	useEffect(() => {
		mounted.current = true;
		return () => {
			mounted.current = false;
			stopReread();
		};
	}, []);

	// 處理過的待辦（建立失敗）依帳戶狀態出入佇列：重新整理排入、重讀完成、重試建立成功都重讀。
	const invalidateQueue = () => {
		void queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: ReviewQueueAdminService.method.listReviewQueue,
				cardinality: undefined,
			}),
		});
	};
	// 開通任務的收款帳戶項跟著帳戶結果走：只在重讀完成與重試建立成功時重讀（重新整理剛排入時帳戶還沒變）。
	const invalidateOnboarding = () => {
		void queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStoreOnboarding,
				cardinality: undefined,
			}),
		});
	};

	const startReread = (baselineMs: number | undefined) => {
		stopReread();
		const mine = generation.current;
		let attempt = 0;
		const schedule = () => {
			rereadTimer.current = setTimeout(async () => {
				attempt += 1;
				const result = await payment.query.refetch();
				// 帳戶快取已更新：即使已卸載或被新一輪取代，頁面層的開通任務查詢也要重讀；世代只管要不要排下一輪。
				if (rereadFinished(baselineMs, result.data?.account)) {
					invalidateQueue();
					invalidateOnboarding();
					return;
				}
				if (generation.current !== mine) return;
				if (attempt < REFRESH_REREAD_MAX_ATTEMPTS) schedule();
			}, rereadMs);
		};
		schedule();
	};

	const handleRefresh = async () => {
		onNotice(null);
		const baselineMs = fetchTimeMs(payment.query.data?.account);
		try {
			await refreshMutation.mutateAsync({ storeId });
			onNotice({ tone: "success", text: REFRESH_QUEUED_TEXT });
			invalidateQueue();
			if (mounted.current) startReread(baselineMs);
		} catch (caught) {
			onNotice({
				tone: "error",
				text: connectErrorText(caught, "重新整理失敗，請稍後再試一次。"),
			});
		}
	};
	const handleRetryCreate = async () => {
		onNotice(null);
		try {
			const res = await retryMutation.mutateAsync({ storeId });
			queryClient.setQueryData(
				paymentAccountQueryOptions(transport, storeId).queryKey,
				{ account: res.account },
			);
			onNotice({ tone: "success", text: RETRY_QUEUED_TEXT });
			invalidateQueue();
			invalidateOnboarding();
		} catch (caught) {
			onNotice({
				tone: "error",
				text: connectErrorText(caught, "重試建立失敗，請稍後再試一次。"),
			});
		}
	};

	return (
		<PaymentAccountCard
			state={payment.value}
			refreshing={refreshMutation.isPending}
			retrying={retryMutation.isPending}
			onRefresh={() => void handleRefresh()}
			onRetryCreate={() => void handleRetryCreate()}
			onReload={() => void payment.query.refetch()}
		/>
	);
}
