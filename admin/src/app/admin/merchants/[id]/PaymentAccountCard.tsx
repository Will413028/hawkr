"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	createConnectQueryKey,
	useMutation as useConnectMutation,
	useTransport,
} from "@connectrpc/connect-query";
import {
	type PaymentAccount,
	PaymentAdminService,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import { ReviewQueueAdminService } from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { PaymentAccountStatus } from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	paymentPaymentAccountStatus,
	paymentRequirementCollection,
} from "@hawkr/ui-text/keys";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type BadgeVariant, Button } from "@/components/UI";
import CopyIconButton from "@/features/delegated-operations/components/CopyIconButton";
import { connectErrorText, errorMessage } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { staffTextOrUnknown } from "@/lib/ui-text";
import { DetailCardShell } from "./DetailCardShell";
import type { GovernanceNotice } from "./GovernanceNotice";
import PaymentMappingDeleteDialog from "./PaymentMappingDeleteDialog";
import PaymentRetryCreateConfirmDialog from "./PaymentRetryCreateConfirmDialog";
import {
	canDeleteMapping,
	canRetryPurge,
	type PaymentAccountState,
	paymentAccountCreated,
	paymentAccountQueryOptions,
} from "./payment-account";
import type { usePaymentAccount } from "./use-store-detail";

/** 錨點：審核待辦 PAYMENT_ACCOUNT_CREATE_FAILED 與 PAYMENT_ACCOUNT_PURGE_STUCK 的連結帶這個 hash（queue-item.ts）。 */
export const PAYMENT_ACCOUNT_ANCHOR = "payment-account";

export const REFRESHED_TEXT = "已重新整理";
export const RETRY_QUEUED_TEXT = "已重新排入建立";
export const PURGE_RETRY_QUEUED_TEXT = "已重新排入關閉";
export const MAPPING_DELETED_TEXT = "已刪除對應";

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
	purging,
	onRefresh,
	onRetryCreate,
	onRetryPurge,
	onDeleteMapping,
	onReload,
}: {
	state: PaymentAccountState;
	refreshing: boolean;
	retrying: boolean;
	/** 重試關閉進行中。 */
	purging: boolean;
	onRefresh: () => void;
	onRetryCreate: () => void;
	onRetryPurge: () => void;
	/** 「已在金流商處理，刪除對應」：由呼叫端先確認再送出。 */
	onDeleteMapping: () => void;
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
	const blockedTime = account.purgeBlockedTime;
	const closingTime = blockedTime ? undefined : account.purgePendingTime;
	const failedTime =
		blockedTime || closingTime ? undefined : account.createFailedTime;
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
			{closingTime && (
				<p className="payment-desc payment-account-failed" role="alert">
					收款帳戶關閉中（{formatDateTimeShort(timestampDate(closingTime))} 起）
				</p>
			)}
			{blockedTime && (
				<p className="payment-desc payment-account-failed" role="alert">
					收款帳戶無法關閉：{account.purgeBlockedReason || "-"}（
					{formatDateTimeShort(timestampDate(blockedTime))} 起）
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
				{paymentAccountCreated(account) && !blockedTime && !closingTime && (
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
				{blockedTime && (
					<>
						{canRetryPurge(account) && (
							<Button
								variant="primary"
								onClick={onRetryPurge}
								loading={purging}
								disabled={purging}
							>
								重試關閉
							</Button>
						)}
						{canDeleteMapping(account) && (
							<Button
								variant="danger"
								onClick={onDeleteMapping}
								disabled={purging}
							>
								已在金流商處理，刪除對應
							</Button>
						)}
					</>
				)}
			</div>
		</DetailCardShell>
	);
}

/**
 * 商家詳情放的收款帳戶區塊：資料來自頁面的 usePaymentAccount，動作的結果走頁面的 notice（onNotice，與重新綁定同一個結果區）。
 * 重新整理與重試建立的回應都直接寫進快取（重新整理是同步重讀，回應帶最新帳戶）。
 */
export function PaymentAccountSection({
	storeId,
	payment,
	onNotice,
}: {
	storeId: string;
	payment: ReturnType<typeof usePaymentAccount>;
	onNotice: (notice: GovernanceNotice | null) => void;
}) {
	const queryClient = useQueryClient();
	const transport = useTransport();
	const refreshMutation = useConnectMutation(
		PaymentAdminService.method.refreshPaymentAccount,
	);
	const retryMutation = useConnectMutation(
		PaymentAdminService.method.retryPaymentAccountCreate,
	);
	const purgeMutation = useConnectMutation(
		PaymentAdminService.method.retryPaymentAccountPurge,
	);
	const mappingMutation = useConnectMutation(
		PaymentAdminService.method.deletePaymentAccountMapping,
	);
	const [confirmingDelete, setConfirmingDelete] = useState(false);
	const [confirmingRetryCreate, setConfirmingRetryCreate] = useState(false);
	const invalidateQueue = () => {
		void queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: ReviewQueueAdminService.method.listReviewQueue,
				cardinality: undefined,
			}),
		});
	};
	const invalidateOnboarding = () => {
		void queryClient.invalidateQueries({
			queryKey: createConnectQueryKey({
				schema: SellerAdminService.method.getStoreOnboarding,
				cardinality: undefined,
			}),
		});
	};

	const writeAccount = (account: PaymentAccount | undefined) => {
		queryClient.setQueryData(
			paymentAccountQueryOptions(transport, storeId).queryKey,
			{ account },
		);
	};

	const handleRefresh = async () => {
		onNotice(null);
		try {
			const res = await refreshMutation.mutateAsync({ storeId });
			writeAccount(res.account);
			onNotice({ tone: "success", text: REFRESHED_TEXT });
			invalidateQueue();
			invalidateOnboarding();
		} catch (caught) {
			onNotice({
				tone: "error",
				text: connectErrorText(caught, "重新整理失敗，請稍後再試一次。"),
			});
		}
	};
	const handleRetryCreate = async (acknowledgeNoProviderAccount = false) => {
		onNotice(null);
		try {
			const res = await retryMutation.mutateAsync({
				storeId,
				acknowledgeNoProviderAccount,
			});
			writeAccount(res.account);
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

	const handleConfirmRetryCreate = async () => {
		const res = await retryMutation.mutateAsync({
			storeId,
			acknowledgeNoProviderAccount: true,
		});
		setConfirmingRetryCreate(false);
		writeAccount(res.account);
		onNotice({ tone: "success", text: RETRY_QUEUED_TEXT });
		invalidateQueue();
		invalidateOnboarding();
	};

	const handleRetryPurge = async () => {
		onNotice(null);
		try {
			const res = await purgeMutation.mutateAsync({ storeId });
			writeAccount(res.account);
			onNotice({ tone: "success", text: PURGE_RETRY_QUEUED_TEXT });
			invalidateQueue();
		} catch (caught) {
			onNotice({
				tone: "error",
				text: connectErrorText(caught, "重試關閉失敗，請稍後再試一次。"),
			});
		}
	};
	const handleDeleteMapping = async () => {
		await mappingMutation.mutateAsync({ storeId });
		setConfirmingDelete(false);
		onNotice({ tone: "success", text: MAPPING_DELETED_TEXT });
		void queryClient.invalidateQueries({
			queryKey: paymentAccountQueryOptions(transport, storeId).queryKey,
		});
		invalidateQueue();
		invalidateOnboarding();
	};

	const account =
		payment.value.state === "ready" ? payment.value.account : null;
	return (
		<>
			<PaymentAccountCard
				state={payment.value}
				refreshing={refreshMutation.isPending}
				retrying={retryMutation.isPending}
				purging={purgeMutation.isPending}
				onRefresh={() => void handleRefresh()}
				onRetryCreate={() =>
					account?.createUnresolved
						? setConfirmingRetryCreate(true)
						: void handleRetryCreate()
				}
				onRetryPurge={() => void handleRetryPurge()}
				onDeleteMapping={() => setConfirmingDelete(true)}
				onReload={() => void payment.query.refetch()}
			/>
			{confirmingRetryCreate && (
				<PaymentRetryCreateConfirmDialog
					storeId={storeId}
					provider={account?.provider ?? ""}
					onClose={() => setConfirmingRetryCreate(false)}
					onConfirm={handleConfirmRetryCreate}
				/>
			)}
			{confirmingDelete && (
				<PaymentMappingDeleteDialog
					storeId={storeId}
					provider={account?.provider ?? ""}
					externalAccountId={account?.externalAccountId ?? ""}
					onClose={() => setConfirmingDelete(false)}
					onConfirm={handleDeleteMapping}
				/>
			)}
		</>
	);
}
