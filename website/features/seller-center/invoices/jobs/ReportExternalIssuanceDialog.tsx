"use client";

/**
 * 端點 62｜回報自行開立的發票號碼（S07 ⑦／S08 ⑤ 共用）。
 *
 * ── 兩個畫面共用同一顆 dialog，不各寫一份 ──────────────────────────────────
 * S07（訂單詳情的發票卡，停權賣家也看得到）與 S08（發票異常詳情，停權 BLOCK）
 * 回報的是同一件事、送同一支端點。寫兩份的代價不是重複，而是**兩份會分岔**：
 * 號碼格式或必填規則改一邊、另一邊靜默過期。
 *
 * ── 前端驗證只做「後端逐字寫下來的那幾條」 ────────────────────────────────
 * `invoiceNumber` 的 `^[A-Z]{2}[0-9]{8}$` 與 `note` 的 500 字上限都是從
 * `lib/invoices/seller-repo.ts` 的常數來，而那兩個常數抄自 invoice-service 的
 * `externalInvoiceNoPattern`／`adminNoteMaxRunes`。前端先擋是因為後端的 400 會把
 * 一句英文（`invoice: invoice_no must match ...`）直接印到賣家臉上。
 *
 * 🔴 **不自己驗「發票日不得早於付款日」**：那個判定要付款時點，而本畫面沒有它
 * （端點 33／62R 都不給 `issueNotBefore`）。自己算等於猜，猜錯的方向是「擋住一個
 * 其實合法的日期」——賣家會卡在一個前端造出來的死路上。交給後端擋。
 *
 * 🔴 **沒有佐證截圖上傳**：端點 62 只收 media objectKey，而本 repo 的 secure 上傳
 * 產出的是 staged token（`lib/media/secure-upload.ts`）⇒ 放一個上傳元件在這裡，
 * 賣家傳完之後會拿到 400。空陣列是合法的（後端 `validateEvidenceAssetKeys` 只驗
 * 數量與形狀，不驗非空）。
 *
 * ⚠️ 這是**不可逆**的動作：回報成功後該筆作業轉 `RESOLVED_EXTERNALLY`，平台不會
 * 再開立。所以送出鍵前面有一句明說後果的話，而不是只有一個「確定」。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Input, Textarea } from "@/components/ui/Input";
import { invoiceErrorMessage } from "@/lib/invoices/buyer-repo";
import { invoiceErrorCopy } from "@/lib/invoices/errorCopy";
import {
	EXTERNAL_INVOICE_NO_PATTERN,
	EXTERNAL_ISSUANCE_NOTE_MAX,
} from "@/lib/invoices/seller-repo";

export interface ReportExternalIssuanceDialogProps {
	open: boolean;
	onClose: () => void;
	onSubmit: (args: {
		invoiceNumber: string;
		invoiceDate: string;
		note: string;
	}) => void;
	submitting: boolean;
	/** `9351`：上一次操作還在跑。**不是失敗**——顯示進度並鎖住送出鍵。 */
	inProgress: boolean;
	error: unknown;
}

export function ReportExternalIssuanceDialog({
	open,
	onClose,
	onSubmit,
	submitting,
	inProgress,
	error,
}: ReportExternalIssuanceDialogProps) {
	const [invoiceNumber, setInvoiceNumber] = React.useState("");
	const [invoiceDate, setInvoiceDate] = React.useState("");
	const [note, setNote] = React.useState("");

	// 每次開啟都回到空白：上一次放棄的輸入不該在下一次冒出來，而這一次送出去的
	// 東西是不可逆的。
	React.useEffect(() => {
		if (open) {
			setInvoiceNumber("");
			setInvoiceDate("");
			setNote("");
		}
	}, [open]);

	const numberInvalid =
		invoiceNumber !== "" && !EXTERNAL_INVOICE_NO_PATTERN.test(invoiceNumber);
	const canSubmit =
		EXTERNAL_INVOICE_NO_PATTERN.test(invoiceNumber) &&
		invoiceDate !== "" &&
		note.length <= EXTERNAL_ISSUANCE_NOTE_MAX &&
		!inProgress;

	const copy = invoiceErrorCopy(error);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			title="回報自行開立的發票號碼"
			footer={
				<div className="flex justify-end gap-2">
					<Button
						variant="light"
						size="sm"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant="solid"
						size="sm"
						isLoading={submitting}
						loadingLabel="送出中…"
						disabled={!canSubmit}
						onClick={() =>
							onSubmit({
								invoiceNumber,
								invoiceDate,
								note: note.trim(),
							})
						}
					>
						送出回報
					</Button>
				</div>
			}
		>
			<div className="flex flex-col gap-3">
				<p className="m-0 text-[0.6875rem] leading-relaxed text-foreground/70">
					{
						"填你在自家開票系統開出的那張發票。送出後這筆會標記為已由你開立，平台不會再開一次。"
					}
				</p>

				<Input
					label="發票號碼"
					description="兩碼英文字軌 ＋ 八碼數字，例如 AB12345678。"
					placeholder="AB12345678"
					value={invoiceNumber}
					// 字軌一律大寫：後端的 pattern 只收大寫，讓賣家因為打了小寫而被擋
					// 是一個沒有意義的失敗。
					onChange={(e) => setInvoiceNumber(e.target.value.toUpperCase())}
					isInvalid={numberInvalid}
					errorMessage={
						numberInvalid ? "格式是兩碼大寫英文 ＋ 八碼數字。" : undefined
					}
				/>

				<Input
					type="date"
					label="發票日期"
					description="不得早於這筆訂單的付款日。"
					value={invoiceDate}
					onChange={(e) => setInvoiceDate(e.target.value)}
				/>

				<Textarea
					label="備註（選填）"
					description={`${note.length} / ${EXTERNAL_ISSUANCE_NOTE_MAX}`}
					maxLength={EXTERNAL_ISSUANCE_NOTE_MAX}
					rows={3}
					value={note}
					onChange={(e) => setNote(e.target.value)}
				/>

				{inProgress ? (
					<p
						role="status"
						className="m-0 text-[0.6875rem] leading-relaxed text-warning-700"
					>
						{"這張發票有處理中的作業，請稍候幾秒再送出，不要重複點擊。"}
					</p>
				) : null}

				{copy ? (
					<p
						role="alert"
						className="m-0 text-[0.6875rem] leading-relaxed text-danger-700"
					>
						{copy.title}
						{copy.description ? ` ${copy.description}` : ""}
					</p>
				) : error ? (
					// 不是發票業務碼（400 的格式錯誤／5xx／網路）⇒ 把後端那句話原樣顯示，
					// 那是唯一說得出「哪裡不合格」的來源。
					<p
						role="alert"
						className="m-0 text-[0.6875rem] leading-relaxed text-danger-700"
					>
						{invoiceErrorMessage(error) || "送出失敗，請稍後再試一次。"}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}

export default ReportExternalIssuanceDialog;
