"use client";

import * as React from "react";
import { Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { INVOICE_COMPLAINT_TEXT } from "@/features/invoices/i18n";
import { complaintActionFieldRule } from "@/lib/invoices/complaintStatus";

const T = INVOICE_COMPLAINT_TEXT;

const REASON_MAX = 500;

/** 三支動作端點各自的欄位規則——差別只有這一個維度，所以共用一個 Dialog。 */
export type ComplaintActionKind = "reject" | "withdraw" | "reopen";

interface InvoiceComplaintActionDialogProps {
	kind: ComplaintActionKind | null;
	/** `reopenCount` 已用掉的次數，只有 `reopen` 的說明句用得到。 */
	reopenRemaining: number;
	submitting: boolean;
	errorMessage: string;
	onClose: () => void;
	/** `withdraw` 一律送空字串——端點層與前端都不自行加嚴。 */
	onConfirm: (reason: string) => void;
}

/**
 * B07 的三顆動作鍵的二次確認（端點 63／64／65）。
 *
 * 🔴 **三者的必填規則各不相同，而且與建案相反**——照抄任何一個到另一個都會錯：
 *
 * | 動作 | 欄位 | 送出鍵 |
 * |---|---|---|
 * | 63 不接受賣家回覆 | `<Textarea>` **必填** | 空白時 `disabled` |
 * | 64 撤回申訴 | **沒有輸入欄** | 永遠可按 |
 * | 65 重新提出 | `<Textarea>` 選填 | 不因空白而 disable |
 *
 * `kind` 轉為非 null 時重置 state（否則第二次開啟看到上次殘留）。冪等鍵由呼叫端
 * 在**開啟**這個 Dialog 時 mint 一次、重試沿用同一把（§0.4 ②）。
 */
export function InvoiceComplaintActionDialog({
	kind,
	reopenRemaining,
	submitting,
	errorMessage,
	onClose,
	onConfirm,
}: InvoiceComplaintActionDialogProps) {
	const [text, setText] = React.useState("");
	const countId = React.useId();

	React.useEffect(() => {
		if (kind) setText("");
	}, [kind]);

	// 必填與否折在 `complaintActionFieldRule` 一處並有測試守著——三者各不相同，
	// 而且 63 與建案完全相反（見該函式的檔內註解）。
	const rule = complaintActionFieldRule(kind ?? "withdraw");
	const needsReason = rule.required;
	const hasField = rule.hasField;
	const blocked =
		(needsReason && text.trim().length === 0) || text.length > REASON_MAX;

	const title =
		kind === "reject"
			? T.rejectTitle
			: kind === "withdraw"
				? T.withdrawTitle
				: T.reopenTitle;
	const description =
		kind === "reject"
			? T.rejectDescription
			: kind === "withdraw"
				? T.withdrawDescription
				: T.reopenDescription(reopenRemaining);
	const confirmLabel =
		kind === "reject"
			? T.rejectCta
			: kind === "withdraw"
				? T.withdrawConfirm
				: T.reopenConfirm;

	return (
		<Dialog
			open={kind !== null}
			onClose={onClose}
			title={title}
			description={description}
			size="md"
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
						loadingLabel="處理中"
						disabled={blocked}
						onClick={() => onConfirm(text.trim())}
					>
						{confirmLabel}
					</Button>
				</>
			}
		>
			{hasField ? (
				<div>
					<Textarea
						label={kind === "reject" ? T.rejectFieldLabel : T.reopenFieldLabel}
						variant="bordered"
						size="lg"
						isRequired={needsReason}
						value={text}
						onChange={(e) => setText(e.target.value.slice(0, REASON_MAX))}
						minRows={4}
						maxLength={REASON_MAX}
						aria-describedby={countId}
						disabled={submitting}
					/>
					<span
						id={countId}
						className="mt-1 block text-right text-xs tabular-nums text-hk-muted"
					>
						{text.length} / {REASON_MAX}
					</span>
				</div>
			) : null}

			<p className="m-0 mt-3 text-xs text-hk-muted">{T.retryKeepsRequest}</p>

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

export default InvoiceComplaintActionDialog;
