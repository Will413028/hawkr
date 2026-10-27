"use client";

import { type FormEvent, useState } from "react";
import {
	GovernanceNoticeBanner,
	governanceErrorText,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";

/**
 * A04 ③｜撤銷 AMEGO 連線（58A §A04，端點 67 ＝ 58 §7.3.4 P16）。**不可逆。**
 *
 * ## `REVOKED` 是這個狀態機唯一的終態
 *
 * 撤銷之後這組憑證永久作廢，沒有任何一條路回得來（`RevokeProviderAccount` 的
 * allowed-from 只有 `ACTIVE`／`DEGRADED`／`SUSPENDED`，撤不了第二次也回不去）。
 * 所以這一支要的保護是三件：必填理由 ＋ 明文 checkbox ＋ `variant="danger"`，
 * 而且**送出中 Escape 失效**。
 *
 * ## 兩件「不會發生」的事要寫在畫面上
 *
 * ① 已經開立的發票不受影響 ② `automation_cutover_at` **保留不清空**（歷史訂單的
 * 開票方式判定要用它）——後端的破壞性防護⑦就是「在 SQL 裡不寫那一欄」。不講的話，
 * ADM 會以為撤銷等於把這個賣家的發票歷史一起抹掉，於是不敢按該按的鍵。
 *
 * ## UI 不得暗示雙人把關
 *
 * 58 §7.3.4 P16 逐字「**不是雙人確認**」——全案唯一的雙人機制是端點 54／55 的人工解鎖。
 * 這裡寫成「送出後由第二位管理員確認」會讓人以為還有一道關卡，然後放心地按下去。
 */
export default function ProviderRevokeDialog({
	onClose,
	onSubmit,
}: {
	onClose: () => void;
	/** 送出端點 67。reject 時對話框留在原地並就地顯示原因。 */
	onSubmit: (reason: string) => Promise<void>;
}) {
	const [reason, setReason] = useState("");
	const [acknowledged, setAcknowledged] = useState(false);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!reason.trim()) {
			setError("請填寫撤銷理由，這一欄會寫入發票稽核紀錄。");
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
					"撤銷失敗，這條連線沒有變動；請確認網路狀態後再送一次。",
				),
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="撤銷 AMEGO 連線"
			titleIcon="block"
			description="撤銷之後這組憑證永久作廢，無法恢復。這個賣家之後付款的訂單一律回到人工開立。"
			closeLabel="關閉撤銷 AMEGO 連線視窗"
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
						{submitting ? "撤銷中…" : "確認永久撤銷"}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">
				已經開立的發票不受影響，啟用切點也會保留——歷史訂單的開票方式維持原本的判定。
			</p>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-provider-revoke-reason">撤銷理由</label>
				<textarea
					id="adm-provider-revoke-reason"
					maxLength={500}
					rows={3}
					required
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					disabled={submitting}
					placeholder="例：賣家已終止與 AMEGO 的合約，憑證不再有效"
				/>
				<span className="adm-operation-field-note">
					理由會寫入發票稽核紀錄（保存 5 年），請寫得讓之後查帳的人看得懂。
				</span>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label
					className="invoice-confirm-check"
					htmlFor="adm-provider-revoke-ack"
				>
					<input
						id="adm-provider-revoke-ack"
						type="checkbox"
						checked={acknowledged}
						onChange={(event) => {
							setAcknowledged(event.target.checked);
							setError(null);
						}}
						disabled={submitting}
					/>
					<span>我了解撤銷不可逆，且這個賣家之後的訂單將回到人工開立。</span>
				</label>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
