"use client";

/**
 * S10 ⑥｜賣家回覆買家申訴（端點 36）。
 *
 * 🔴 **三個 action 正面列舉**，不是「`InvoiceComplaintAction` 減掉 ADM 的三個」。
 * 那份碼表是 6 值共用表，減法寫法會在它長胖時把新的 ADM 動作靜默收進賣家選單。
 *
 * 🔴 **沒有佐證檔上傳**：端點 36 的 request 沒有 attachment 欄，那是刻意的
 * （58 §6.1 端點 36 定版③）。放一個上傳元件在這裡只會讓賣家傳完之後發現沒送出去。
 *
 * 選 `NEED_PLATFORM_HELP` 時說明欄由前端加嚴為必填：那條路徑會把案子丟給平台，
 * 沒有說明的話客服拿到的是一件不知道卡在哪的案子。
 */

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { Textarea } from "@/components/ui/Input";
import type { SellerComplaintAction } from "@/lib/invoices/seller-repo";

const NOTE_MAX = 500;

const OPTIONS: ReadonlyArray<{
	value: SellerComplaintAction;
	label: string;
	hint: string;
}> = [
	{
		value: "RESENT",
		label: "已重新寄送",
		hint: "發票或收據已重新寄到買家的信箱。",
	},
	{
		value: "REISSUED",
		label: "已補開或更正",
		hint: "已重新開立或更正發票內容。",
	},
	{
		value: "NEED_PLATFORM_HELP",
		label: "需要平台協助",
		hint: "我無法自行處理，需要平台介入。",
	},
];

export function InvoiceComplaintRespondDialog({
	open,
	onClose,
	onSubmit,
	submitting,
	error,
}: {
	open: boolean;
	onClose: () => void;
	onSubmit: (args: { action: SellerComplaintAction; note: string }) => void;
	submitting: boolean;
	error: unknown;
}) {
	const [action, setAction] = React.useState<SellerComplaintAction | "">("");
	const [note, setNote] = React.useState("");

	// 每次開啟都回到空白：上一次放棄的輸入不該在下一次冒出來。
	React.useEffect(() => {
		if (open) {
			setAction("");
			setNote("");
		}
	}, [open]);

	const noteRequired = action === "NEED_PLATFORM_HELP";
	const noteMissing = noteRequired && !note.trim();
	const canSubmit = Boolean(action) && !noteMissing && note.length <= NOTE_MAX;

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			title="回覆買家申訴"
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
						onClick={() => action && onSubmit({ action, note: note.trim() })}
					>
						送出回覆
					</Button>
				</div>
			}
		>
			<div className="flex flex-col gap-3">
				<div
					className="flex flex-col gap-2"
					role="radiogroup"
					aria-label="處理方式"
				>
					{OPTIONS.map((opt) => (
						<label
							key={opt.value}
							className="flex items-start gap-3 p-[12px_14px] bg-content2 border border-divider rounded-hkui-lg cursor-pointer"
						>
							<input
								type="radio"
								name="complaint-action"
								className="shrink-0 mt-[2px]"
								checked={action === opt.value}
								onChange={() => setAction(opt.value)}
							/>
							<span className="flex flex-col gap-[2px] min-w-0">
								<strong className="text-xs font-extrabold text-foreground">
									{opt.label}
								</strong>
								<small className="text-[0.625rem] leading-relaxed text-foreground/60">
									{opt.hint}
								</small>
							</span>
						</label>
					))}
				</div>

				<Textarea
					label={noteRequired ? "處理說明（必填）" : "處理說明（選填）"}
					description={`${note.length} / ${NOTE_MAX}`}
					maxLength={NOTE_MAX}
					rows={4}
					value={note}
					isInvalid={noteMissing}
					errorMessage={
						noteMissing ? "選擇「需要平台協助」時請說明卡在哪裡。" : undefined
					}
					onChange={(e) => setNote(e.target.value)}
				/>

				{error ? (
					<p
						role="alert"
						className="text-[0.6875rem] leading-relaxed text-danger-700"
					>
						{"送出失敗。這件申訴可能已被更新，請關閉後重新整理再試一次。"}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}
