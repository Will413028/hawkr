"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";

/**
 * 「查看完整地址」的代行理由輸入（mockup 沒有畫這一步，但沒有它就打不出請求）。
 *
 * `operation_reason` 是揭露端點的**必填 query**（trim 後 1–100 字）。少了它後端回
 * 9260／400；前端先擋是為了讓操作者當場看到字數，而不是送出後拿到一個沒有訊息的
 * 錯誤碼（代行 BFF 一路不透傳上游 message）。
 *
 * ⚠️ 這個 modal 存在的第二個理由與 UI 無關：揭露是 `risk_level=high` 的稽核事實，
 * 而**同一筆揭露兩次刻意不去重**（後端 `idempotency_key` 為 `NULL`）。要求先寫下理由
 * 才送出，讓「多按一次」不是零成本的。
 */

export const OPERATION_REASON_MAX = 100;

export default function RevealRecipientDialog({
	open,
	orderNo,
	submitting,
	errorMessage,
	onClose,
	onSubmit,
}: {
	open: boolean;
	orderNo: string;
	submitting: boolean;
	errorMessage: string | null;
	onClose: () => void;
	onSubmit: (operationReason: string) => void;
}) {
	const [reason, setReason] = React.useState("");
	const trimmed = reason.trim();
	const tooLong = trimmed.length > OPERATION_REASON_MAX;
	const canSubmit = trimmed.length > 0 && !tooLong && !submitting;

	React.useEffect(() => {
		if (open) setReason("");
	}, [open]);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			title="查看完整收件資訊"
			description={`訂單 #${orderNo}`}
			footer={
				<>
					<Button
						variant="bordered"
						className="adm-quiet-button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						color="primary"
						onClick={() => onSubmit(trimmed)}
						isLoading={submitting}
						loadingLabel="揭露中"
						disabled={!canSubmit}
					>
						確認查看
					</Button>
				</>
			}
		>
			<div className="adm-reveal-form">
				<p className="adm-reveal-form__hint">
					{
						"這是本次代行唯一會取得未遮罩收件資訊的動作，會以高風險等級寫入代行紀錄，並計入結束摘要的風險操作次數。請寫下需要查看的理由。"
					}
				</p>
				<textarea
					className="adm-reveal-form__input"
					value={reason}
					onChange={(event) => setReason(event.target.value)}
					placeholder="例：協助賣家確認地址是否可配送"
					aria-label="代行原因"
				/>
				<span
					className={`adm-reveal-form__counter${tooLong ? " adm-reveal-form__counter--over" : ""}`}
				>
					{trimmed.length} / {OPERATION_REASON_MAX}
				</span>
				{errorMessage ? (
					<p className="adm-reveal-form__error" role="alert">
						{errorMessage}
					</p>
				) : null}
			</div>
		</Dialog>
	);
}
