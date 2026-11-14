"use client";

import { useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "./GovernanceNotice";

/**
 * 「已在金流商處理，刪除對應」的確認（PaymentAdminService.DeletePaymentAccountMapping）。
 *
 * 這個動作不呼叫金流商：它只刪平台這邊的帳戶列，並留下一筆操作者與時間的紀錄。確認前要先在金流商那邊
 * 把外部帳戶處理掉，否則刪掉之後平台就找不回它。沒有理由欄位：契約只有 store_id。
 */

interface PaymentMappingDeleteDialogProps {
	storeId: string;
	provider: string;
	/** 刪除當下列上的外部帳戶 ID；還沒拿到時是空字串。 */
	externalAccountId: string;
	onClose: () => void;
	/** 送出刪除；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: () => Promise<void>;
}

export default function PaymentMappingDeleteDialog({
	storeId,
	provider,
	externalAccountId,
	onClose,
	onConfirm,
}: PaymentMappingDeleteDialogProps) {
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
					"刪除對應失敗，帳戶沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="確定已在金流商處理這個收款帳戶嗎？"
			titleIcon="account_balance"
			description="這個動作不會呼叫金流商：只刪除平台這邊的帳戶對應，並記下是誰、什麼時候刪的。刪除之後平台找不回外部帳戶，請先確認金流商那邊已經關閉或處理。"
			closeLabel="關閉刪除對應視窗"
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
						{submitting ? "正在刪除" : "刪除對應"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>店家 ID · {storeId}</span>
				<span>金流商 · {provider || "-"}</span>
				<span>外部帳戶 · {externalAccountId || "-"}</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
