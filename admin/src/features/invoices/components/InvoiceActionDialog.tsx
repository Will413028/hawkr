"use client";

import type { MaterialSymbolName } from "@hawkr/icons/material-symbol";
import { type FormEvent, type ReactNode, useState } from "react";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import { MAX_ADMIN_REASON_RUNES } from "@/features/invoices/admin-actions";

/**
 * A03／A06／A08／A13 十一個寫入動作共用的確認對話框。
 *
 * ## 為什麼是一支共用元件而不是十一支檔案
 *
 * A04 的三支 dialog 各自成檔是對的：它們三個的**內容**不同（停用要講影響範圍、
 * 恢復要講「不代表回到啟用」、撤銷要講不可逆與切點保留）。而這一批十一個動作的
 * 骨架完全一樣——標題、後果說明、必填理由、（部分動作的）二次確認、就地錯誤、
 * 送出中鎖定。差異全部落在「多幾個欄位」，那正好是 `children` 的形狀。
 *
 * 複製十一份的實際代價是十一份各自會漂移的骨架與 a11y 掛法；對話框骨架現在由共用的
 * `Dialog` 提供，這支只剩內容。
 *
 * ## 三件事寫死在這裡，呼叫端關不掉
 *
 * 1. **理由必填**：後端九支端點無一例外（`requireAdminReason`／`requireReason`／
 *    `ErrComplaintAdminReasonRequired`），而且理由會落進保存 5 年的
 *    `invoice_admin_audit_logs.reason_detail`。前端先擋只是為了不浪費一次往返。
 * 2. **送出中 Escape 失效**：這些動作有一半不可逆（撤銷資格、判定違規、取消工作），
 *    請求已經在路上時關掉視窗會讓人以為自己取消掉了。
 * 3. **錯誤就地顯示、不關 dialog**：頁面層只處理「狀態已變」那一類（關 dialog ＋
 *    refetch ＋ 頁面橫幅），其餘一律留在原地讓人改完再送。
 *
 * ⚠️ **不做的事**：不管冪等鍵、不管 refetch、不決定成功要顯示什麼。那些是頁面的
 * 責任——鍵要跨 dialog 開關保留（送出失敗時不能換），而這個元件會被卸載。
 */
export default function InvoiceActionDialog({
	idPrefix,
	title,
	icon,
	description,
	consequence,
	reasonLabel,
	reasonPlaceholder,
	reasonNote,
	maxReasonRunes = MAX_ADMIN_REASON_RUNES,
	confirmText,
	variant = "primary",
	submitLabel,
	submittingLabel,
	extraFields,
	validateExtra,
	failureText,
	onClose,
	onSubmit,
}: {
	/** DOM id 前綴。同一頁同時只會開一個 dialog，但 id 仍要唯一才對得上 aria。 */
	idPrefix: string;
	title: string;
	/** 標題前的圖示（例：`verified_user`）。 */
	icon: MaterialSymbolName;
	/** 標題底下那一句：這個動作是什麼。 */
	description: string;
	/** 後果說明：按下去之後會發生什麼、哪一部分收不回來。 */
	consequence?: string;
	reasonLabel: string;
	reasonPlaceholder?: string;
	reasonNote?: string;
	maxReasonRunes?: number;
	/** 有值才渲染二次確認勾選框；文字就是勾選框旁邊那句話。 */
	confirmText?: string;
	variant?: "primary" | "danger";
	submitLabel: string;
	submittingLabel: string;
	/** 額外欄位。`disabled` 在送出中為 true，呼叫端要把它接到自己的 input 上。 */
	extraFields?: (disabled: boolean) => ReactNode;
	/** 額外欄位的檢查。回字串 ＝ 擋下並顯示它；回 null ＝ 放行。 */
	validateExtra?: () => string | null;
	/** 送出失敗但頁面層沒有接手時的預設訊息（要說清楚「有沒有生效」）。 */
	failureText: string;
	onClose: () => void;
	/** reject 時對話框留在原地並就地顯示原因。 */
	onSubmit: (reason: string) => Promise<void>;
}) {
	const [reason, setReason] = useState("");
	const [acknowledged, setAcknowledged] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!reason.trim()) {
			setError("請填寫理由，這一欄會寫入發票稽核紀錄。");
			return;
		}
		// 用 Array.from 數字元而不是 `.length`：後端擋的是**符文數**
		// （`utf8.RuneCountInString`），而 JS 的 `.length` 是 UTF-16 code unit
		// ⇒ 表情符號與部分罕用字會讓兩邊對同一段文字得到不同的數字。
		if (Array.from(reason.trim()).length > maxReasonRunes) {
			setError(`理由請控制在 ${maxReasonRunes} 字以內。`);
			return;
		}
		if (confirmText && !acknowledged) {
			setError("請先勾選確認，這一步是送出前的最後一道保護。");
			return;
		}
		const extraError = validateExtra?.() ?? null;
		if (extraError) {
			setError(extraError);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onSubmit(reason.trim());
		} catch (caught) {
			setError(governanceErrorText(caught, failureText));
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			title={title}
			titleIcon={icon}
			description={description}
			closeLabel={`關閉${title}視窗`}
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="ghost"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant={variant}
						type="submit"
						loading={submitting}
						disabled={confirmText ? !acknowledged : false}
					>
						{submitting ? submittingLabel : submitLabel}
					</Button>
				</>
			}
		>
			{consequence ? (
				<p className="adm-operation-field-note">{consequence}</p>
			) : null}

			{extraFields?.(submitting)}

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor={`${idPrefix}-reason`}>{reasonLabel}</label>
				<textarea
					id={`${idPrefix}-reason`}
					maxLength={maxReasonRunes}
					rows={3}
					required
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					disabled={submitting}
					placeholder={reasonPlaceholder}
				/>
				<span className="adm-operation-field-note">
					{reasonNote ??
						"理由會寫入發票稽核紀錄（保存 5 年），請寫得讓之後查帳的人看得懂。"}
				</span>
			</div>

			{confirmText ? (
				<div className="adm-operation-field adm-operation-field--wide">
					<label className="invoice-confirm-check" htmlFor={`${idPrefix}-ack`}>
						<input
							id={`${idPrefix}-ack`}
							type="checkbox"
							checked={acknowledged}
							onChange={(event) => {
								setAcknowledged(event.target.checked);
								setError(null);
							}}
							disabled={submitting}
						/>
						<span>{confirmText}</span>
					</label>
				</div>
			) : null}

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
