"use client";

import * as React from "react";
import { Select, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { INVOICE_COMPLAINT_TEXT } from "@/features/invoices/i18n";

const T = INVOICE_COMPLAINT_TEXT;

/** 端點 13 的 `description` 上限。 */
const DESCRIPTION_MAX = 500;

export interface ComplaintReasonOption {
	value: string;
	label: string;
}

interface InvoiceComplaintDialogProps {
	open: boolean;
	orderNo: string;
	/** 賣家名稱；取不到時整列不渲染（不填「—」，那只是佔位噪音）。 */
	sellerName: string;
	/** 應收到的文件類型（CodeMeta label）；取不到時整列不渲染。 */
	expectedDocLabel: string;
	/**
	 * Phase 1 的 `reasonCode` 只有 `NOT_RECEIVED` 一個值 ⇒ `length === 1` 時渲染
	 * **唯讀說明列**，不渲染只有一個選項的 `<Select>`。保留這個 prop 是為了讓
	 * Phase 2 增加原因只是資料變更、不是改版。
	 */
	reasonOptions: ComplaintReasonOption[];
	submitting: boolean;
	/** 送出失敗時的表單級錯誤（`role="alert"`）。 */
	errorMessage: string;
	onClose: () => void;
	onSubmit: (reasonCode: string, description: string) => void;
}

/**
 * B07｜提出申訴 Dialog（58A §B07，端點 13）。
 *
 * 三個「照抄既有 `AppealDialog` 會做錯」的地方：
 *
 *   1. **送出鍵不因說明空白而 disable**。建案的 `description` 是**選填**，
 *      而既有的 `AppealDialog` 寫 `disabled={text.trim().length === 0}` ⇒ 照抄
 *      會讓買家送不出最常見的那個情況（「就是沒收到，沒別的好說」），而畫面上
 *      按鈕只是灰的、沒有任何錯誤訊息（§B-1.0.9 B18t）。
 *   2. **不放 `AfterSalePhotoUploader`**：Phase 1 不要求買家附件，多留一個沒人
 *      看的上傳器等於製造一批沒人審的機敏檔。
 *   3. **`Textarea` 沒有 `aria-describedby`**（既有 divergence）⇒ 手動把字數
 *      `<span>` 的 id 接上去，否則讀屏使用者聽不到「還剩幾字」（§B-1.0.8）。
 *
 * `open` 轉 true 時重置 state：否則第二次開啟會看到上次殘留的文字。
 */
export function InvoiceComplaintDialog({
	open,
	orderNo,
	sellerName,
	expectedDocLabel,
	reasonOptions,
	submitting,
	errorMessage,
	onClose,
	onSubmit,
}: InvoiceComplaintDialogProps) {
	const firstReason = reasonOptions[0]?.value ?? "";
	const [reasonCode, setReasonCode] = React.useState(firstReason);
	const [text, setText] = React.useState("");
	const countId = React.useId();

	React.useEffect(() => {
		if (open) {
			setReasonCode(firstReason);
			setText("");
		}
	}, [open, firstReason]);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title={T.createTitle}
			description={T.createDescription}
			size="md"
			// 送出中不讓背景點擊與關閉鍵把 Dialog 關掉——請求已經在路上，關掉只會
			// 讓使用者以為沒送出而再按一次。
			isDismissable={!submitting}
			showCloseButton={!submitting}
			footer={
				<>
					<Button
						variant="bordered"
						color="default"
						size="lg"
						disabled={submitting}
						onClick={onClose}
					>
						{T.cancel}
					</Button>
					<Button
						color="warning"
						size="lg"
						isLoading={submitting}
						loadingLabel={T.submitting}
						// 說明是選填 ⇒ 只在超過上限時 disable。
						disabled={text.length > DESCRIPTION_MAX}
						onClick={() => onSubmit(reasonCode, text.trim())}
					>
						{T.submitCta}
					</Button>
				</>
			}
		>
			<dl className="m-0 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 rounded-hkui-lg bg-content2 p-4 text-sm">
				<dt className="m-0 text-hk-muted">{T.fieldOrderNo}</dt>
				<dd className="m-0 tabular-nums text-hk-body">{orderNo}</dd>
				{sellerName ? (
					<>
						<dt className="m-0 text-hk-muted">{T.fieldSeller}</dt>
						<dd className="m-0 text-hk-body">{sellerName}</dd>
					</>
				) : null}
				{expectedDocLabel ? (
					<>
						<dt className="m-0 text-hk-muted">{T.fieldExpectedDoc}</dt>
						<dd className="m-0 text-hk-body">{expectedDocLabel}</dd>
					</>
				) : null}
			</dl>

			{reasonOptions.length > 1 ? (
				<div className="mt-3">
					<Select
						label={T.fieldReason}
						size="lg"
						value={reasonCode}
						onChange={(e) => setReasonCode(e.target.value)}
					>
						{reasonOptions.map((option) => (
							<option key={option.value} value={option.value}>
								{option.label}
							</option>
						))}
					</Select>
				</div>
			) : (
				<dl className="m-0 mt-3 grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 text-sm">
					<dt className="m-0 text-hk-muted">{T.fieldReason}</dt>
					<dd className="m-0 text-hk-body">
						{reasonOptions[0]?.label ?? T.reasonNotReceived}
					</dd>
				</dl>
			)}

			<div className="mt-3">
				<Textarea
					label={T.descriptionLabel}
					variant="bordered"
					size="lg"
					value={text}
					onChange={(e) => setText(e.target.value.slice(0, DESCRIPTION_MAX))}
					placeholder={T.descriptionPlaceholder}
					minRows={4}
					maxLength={DESCRIPTION_MAX}
					aria-describedby={countId}
					disabled={submitting}
				/>
				<span
					id={countId}
					className="mt-1 block text-right text-xs tabular-nums text-hk-muted"
				>
					{text.length} / {DESCRIPTION_MAX}
				</span>
			</div>

			<p className="m-0 mt-3 text-xs text-info-700">{T.slaHint}</p>
			<p className="m-0 mt-1 text-xs text-hk-muted">{T.retryKeepsRequest}</p>

			{errorMessage ? (
				<p
					className="m-0 mt-3 text-sm font-semibold text-danger-700"
					role="alert"
				>
					{errorMessage}
				</p>
			) : null}
		</Dialog>
	);
}

export default InvoiceComplaintDialog;
