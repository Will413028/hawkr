"use client";

import { type FormEvent, useState } from "react";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";

/**
 * A04 ②｜恢復 AMEGO 連線（58A §A04，端點 66 ＝ 58 §7.3.4 P15）。
 *
 * ## 這一顆是 `SUSPENDED` 的唯一出邊之一
 *
 * 端點 66／67 補上之前，被停用的賣家**沒有任何 API、排程或 MQ consumer 走得回來**
 * ——每筆訂單只能走人工開立。所以這一支不是「補齊」而是把一條死路接上。
 *
 * ## 恢復 ≠ 回到已啟用
 *
 * `ReactivateForAdmin` 走的是 `SUSPENDED → CREDENTIAL_VERIFYING → （重新驗證結果）`。
 * UI **不得**寫成「已恢復啟用」：憑證可能早就被 AMEGO 撤銷了，宣告恢復正常之後
 * 每一筆開票都會失敗，而畫面上看起來一切正常。成功文案一律是「已送出重新驗證」。
 *
 * ## 沒有 checkbox，但 wire 上仍然帶 `confirmed: true`
 *
 * 58A 逐字「**無 `confirmed`**、沒有 checkbox」——恢復是可逆的，再多一個勾選只會
 * 讓真正不可逆的那一顆（撤銷）失去份量。但實作端 `usecase.ReactivateForAdmin` 對
 * `confirmed != true` 一律回 `ErrProviderConfirmRequired`（400）⇒ 不帶就是 100% 失敗。
 * ⇒ 值由呼叫端固定帶 true，**畫面維持 58A 的樣子**。這個落差登記在交付回報。
 *
 * ## 送出中的等待要有語音回饋
 *
 * 這一支同步等 AMEGO，最長兩分鐘。沒有 `role="status"` ＋ `aria-live="polite"` 的話，
 * 讀螢幕的人會以為當掉了（58A §A04 A11y 逐字）。
 */
export default function ProviderReactivateDialog({
	onClose,
	onSubmit,
}: {
	onClose: () => void;
	/** 送出端點 66。reject 時對話框留在原地並就地顯示原因。 */
	onSubmit: (reason: string) => Promise<void>;
}) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!reason.trim()) {
			setError("請填寫恢復理由，這一欄會寫入發票稽核紀錄。");
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onSubmit(reason.trim());
		} catch (caught) {
			setError(
				governanceErrorText(
					caught,
					"恢復失敗，這條連線沒有變動；請確認網路狀態後再送一次。",
				),
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			title="恢復 AMEGO 連線"
			titleIcon="play_circle"
			description="恢復後系統會重新驗證這組憑證，不會直接回到「已啟用」；驗證通過才會恢復自動開票。"
			closeLabel="關閉恢復 AMEGO 連線視窗"
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
					<Button variant="primary" type="submit" loading={submitting}>
						{submitting ? "恢復中…（最長 2 分鐘）" : "送出重新驗證"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				停用期間被中止的開票作業不會自動重跑，請到「發票工作與異常佇列」逐筆安全重試。
			</p>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-provider-reactivate-reason">恢復理由</label>
				<textarea
					id="adm-provider-reactivate-reason"
					maxLength={500}
					rows={3}
					required
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					disabled={submitting}
					placeholder="例：賣家已於 AMEGO 後台重新開通商戶，確認可恢復"
				/>
				<span className="adm-operation-field-note">
					理由會寫入發票稽核紀錄（保存 5 年）。
				</span>
			</div>

			{submitting ? (
				<p
					className="adm-operation-field-note"
					role="status"
					aria-live="polite"
				>
					恢復中…（最長 2 分鐘）系統正在向 AMEGO
					重新驗證這組憑證，請不要關閉這個視窗。
				</p>
			) : null}

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
