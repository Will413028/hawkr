"use client";

import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "./GovernanceNotice";

/**
 * 建立結果不明、超過可向金流商查回的期限之後的「重試建立」確認（PaymentAdminService.RetryPaymentAccountCreate 的 acknowledge_no_provider_account，Stripe 計畫 D63）。
 *
 * 重試會換新的冪等鍵並建立新的收款帳戶：管理員按確認等於聲明金流商那邊沒有這家店的帳戶。只有帳戶的 create_unresolved 為真時才出現；
 * 其他建立失敗的重試不需要確認。
 */

interface PaymentRetryCreateConfirmDialogProps {
	storeId: string;
	provider: string;
	onClose: () => void;
	/** 送出帶確認的重試；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: () => Promise<void>;
}

export default function PaymentRetryCreateConfirmDialog({
	storeId,
	provider,
	onClose,
	onConfirm,
}: PaymentRetryCreateConfirmDialogProps) {
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleConfirm = async () => {
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm();
		} catch (caught) {
			setError(
				governanceErrorText(
					caught,
					"重試建立失敗，帳戶沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="確定金流商那邊沒有這家店的收款帳戶嗎？"
			titleIcon="account_balance"
			description="上一次建立的結果不明，而且已經超過可以向金流商查回的期限。重試會建立新的收款帳戶；如果金流商那邊其實已經有一個，平台不會知道，請先到金流商後台確認。"
			closeLabel="關閉重試建立視窗"
			onClose={onClose}
			busy={submitting}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						返回
					</Button>
					<Button variant="danger" onClick={handleConfirm} loading={submitting}>
						{submitting ? "正在重試" : "確認，重試建立"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>店家 ID · {storeId}</span>
				<span>金流商 · {provider || "-"}</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
