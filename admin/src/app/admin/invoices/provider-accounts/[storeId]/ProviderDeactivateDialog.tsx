"use client";

import { type FormEvent, useState } from "react";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";

/**
 * A04 ①｜停用 AMEGO 連線（58A §A04、§B-3.1.5，端點 45 ＝ 58 §7.3.4 P14）。
 *
 * ## 可逆，但不是「沒有後果」
 *
 * 停用會在**同一個交易內**把這個賣家所有未終態的開票工作打成 `FAILED_PERMANENT`
 * （`usecase.ProviderAccount.DeactivateForAdmin`）。那些工作**不會**在恢復連線之後
 * 自動復活，要由 ADM 到「發票工作與異常佇列」逐筆安全重試。所以 checkbox 那一句
 * 講的是範圍（「只影響停用時點之後付款的訂單」），而內文要把「在途工作會被中止」
 * 講出來——少了這句，按下去的人不知道自己剛剛停掉了什麼。
 *
 * ## 為什麼這裡不顯示受影響訂單數
 *
 * 端點 43（開這個框之前已經載入的那一支）的 `pendingOrderCount` 是**當下**的計數，
 * 而端點 45 回的是**停用之後**的計數。在框裡先顯示一個數字，等於承諾一個後端沒有
 * 承諾的值——中間還隔著一個 TOCTOU 窗口。數字只出現在成功畫面上。
 *
 * ## 冪等鍵
 *
 * 一次「開框 → 送出」用同一把，**失敗不換**（58A 陷阱 T28）：停用可能已經生效而回應
 * 沒送到，換鍵會被後端當成第二次操作。
 */
export default function ProviderDeactivateDialog({
	storeId,
	onClose,
	onSubmit,
}: {
	storeId: string;
	onClose: () => void;
	/**
	 * 送出端點 45。reject 時對話框留在原地並就地顯示原因；
	 * 版本衝突／狀態不合法那一類由呼叫端關框並在頁面上說明（58A §A04「業務碼」）。
	 */
	onSubmit: (reason: string) => Promise<void>;
}) {
	const [reason, setReason] = useState("");
	const [acknowledged, setAcknowledged] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!reason.trim()) {
			setError("請填寫停用理由，這一欄會寫入發票稽核紀錄。");
			return;
		}
		if (!acknowledged) {
			setError("請先勾選確認，這一步是送出前的最後一道保護。");
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
					"停用失敗，這條連線沒有變動；請確認網路狀態後再送一次。",
				),
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			title="確定要停用這個賣家的 AMEGO 連線嗎？"
			titleIcon="pause_circle"
			description={
				<>
					停用只影響<strong>停用時點之後付款</strong>
					的訂單，已經開立的發票與啟用切點都不受影響。
				</>
			}
			closeLabel="關閉停用 AMEGO 連線視窗"
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
						variant="danger"
						type="submit"
						loading={submitting}
						disabled={!acknowledged}
					>
						{submitting ? "停用中" : "確認停用連線"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				停用後，這個賣家<strong>在途的開票工作會一併中止</strong>
				，而且不會在恢復連線時自動重跑——要到「發票工作與異常佇列」逐筆安全重試。
				停用可以恢復（恢復時會重新驗證憑證）。
			</p>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-provider-deactivate-reason">停用理由</label>
				<textarea
					id="adm-provider-deactivate-reason"
					maxLength={500}
					rows={3}
					required
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					disabled={submitting}
					placeholder={`例：賣家 ${storeId} 回報 AppKey 已在 AMEGO 後台停用，先停用避免持續失敗`}
				/>
				<span className="adm-operation-field-note">
					理由會寫入發票稽核紀錄（保存 5 年），請寫得讓之後查帳的人看得懂。
				</span>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label
					className="invoice-confirm-check"
					htmlFor="adm-provider-deactivate-ack"
				>
					<input
						id="adm-provider-deactivate-ack"
						type="checkbox"
						checked={acknowledged}
						onChange={(event) => {
							setAcknowledged(event.target.checked);
							setError(null);
						}}
						disabled={submitting}
					/>
					<span>我了解此操作只影響停用時點之後付款的訂單</span>
				</label>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
