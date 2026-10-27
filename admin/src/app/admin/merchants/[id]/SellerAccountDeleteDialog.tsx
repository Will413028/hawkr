"use client";

import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import {
	type AccountEmail,
	DELETE_ACCOUNT_WARNING_TEXT,
	deletionOutcome,
	REASON_MAX_LENGTH,
	validateReason,
} from "./store-detail";

/**
 * 刪除整個帳號（買家與店家，IdentityAdminService.DeleteAccount）。理由寫入刪除申請紀錄，送出前必填。
 *
 * 結果有兩種（契約）：DELETED 是立即刪除，畫面講完再由使用者按「返回商家列表」；PENDING_ORDERS 是帳號還有
 * 未完成的訂單，帳號留著、等最後一筆訂單完成後自動刪除，畫面留在商家詳情（onDone 會重讀店家）。
 * 只讓店家退出（帳號留著）請建立終止合作的措施（CreateTerminationDialog）。
 */

interface SellerAccountDeleteDialogProps {
	storeName: string;
	/** 登入信箱（ListAccounts 找到的那一筆）；沒有信箱時不顯示該列。 */
	accountEmail: AccountEmail;
	accountId: string;
	onClose: () => void;
	/** 送出刪除；回傳契約的結果。reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (reason: string) => Promise<DeleteAccountResponse_DeletionState>;
	/** 使用者在結果畫面按下完成；DELETED 時離開本頁，PENDING_ORDERS 時留在本頁。 */
	onDone: (state: DeleteAccountResponse_DeletionState) => void;
}

export default function SellerAccountDeleteDialog({
	storeName,
	accountEmail,
	accountId,
	onClose,
	onConfirm,
	onDone,
}: SellerAccountDeleteDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [outcome, setOutcome] = useState<{
		state: DeleteAccountResponse_DeletionState;
		title: string;
		text: string;
	} | null>(null);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = validateReason(
			reason,
			"請填寫刪除理由，它會寫入帳號刪除紀錄。",
		);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			const state = await onConfirm(reason.trim());
			const result = deletionOutcome(state);
			if (!result) {
				// 契約的結果沒有 UNSPECIFIED；收到它就不能宣告成功。
				setError("伺服器沒有回報刪除的結果，請重新整理確認帳號的狀態。");
				setSubmitting(false);
				return;
			}
			setOutcome({ state, ...result });
			setSubmitting(false);
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"刪除失敗，帳號沒有變動；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	const emailRow =
		accountEmail.state === "loading" ? (
			<span>登入信箱 · 讀取中…</span>
		) : accountEmail.state === "error" ? (
			<span>登入信箱 · 無法讀取</span>
		) : accountEmail.state === "notFound" ? (
			<span>登入信箱 · 找不到登入帳號</span>
		) : accountEmail.email ? (
			<span>登入信箱 · {accountEmail.email}</span>
		) : null;

	const target = (
		<div className="adm-audit-revoke-target">
			<span>刪除對象</span>
			<strong>{storeName}</strong>
			{emailRow}
			<span>Account ID · {accountId}</span>
		</div>
	);

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title={outcome ? outcome.title : "確定要刪除這個賣家帳號嗎？"}
			titleIcon="delete"
			description={outcome ? outcome.text : DELETE_ACCOUNT_WARNING_TEXT}
			closeLabel="關閉刪除賣家帳號視窗"
			hideClose={Boolean(outcome)}
			onClose={onClose}
			// 已有結果之後 Escape 不再是「取消」，只剩下完成按鈕一條路（關閉鍵本來就已隱藏）。
			busy={submitting || Boolean(outcome)}
			ariaBusy={submitting}
			// `noValidate`（Dialog 固定開啟）：必填由 validateReason 統一擋，與提示文字一致（原生氣泡會搶先出現）。
			form={outcome ? undefined : { onSubmit: handleSubmit }}
			footer={
				outcome ? (
					<Button onClick={() => onDone(outcome.state)}>
						{outcome.state === DeleteAccountResponse_DeletionState.DELETED
							? "返回商家列表"
							: "完成"}
					</Button>
				) : (
					<>
						<Button
							variant="ghost"
							type="button"
							onClick={onClose}
							disabled={submitting}
						>
							返回
						</Button>
						<Button variant="danger" type="submit" loading={submitting}>
							{submitting ? "正在刪除" : "刪除賣家帳號"}
						</Button>
					</>
				)
			}
		>
			{target}
			{outcome ? (
				<GovernanceNoticeBanner
					notice={{
						tone: "success",
						text: "刪除理由已寫入帳號刪除紀錄，可在後端稽核資料中查得。",
					}}
				/>
			) : (
				<>
					<div className="adm-operation-field adm-operation-field--wide">
						<label htmlFor="adm-seller-delete-reason">刪除理由（必填）</label>
						<textarea
							id="adm-seller-delete-reason"
							value={reason}
							onChange={(event) => {
								setReason(event.target.value);
								setError(null);
							}}
							rows={3}
							placeholder="例如：賣家來信要求終止合作並刪除帳號"
							disabled={submitting}
						/>
						<span className="adm-operation-field-note">
							理由會寫入帳號刪除紀錄，最多 {REASON_MAX_LENGTH} 字。
						</span>
					</div>

					<GovernanceNoticeBanner
						notice={error ? { tone: "error", text: error } : null}
					/>
				</>
			)}
		</Dialog>
	);
}
