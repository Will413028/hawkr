"use client";

import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { GovernanceNoticeBanner } from "./GovernanceNotice";
import { REVEAL_DIALOG_TEXT } from "./kyc-application";
import { REASON_MAX_LENGTH, validateReason } from "./store-detail";

interface RevealKycDialogProps {
	storeName: string;
	onClose: () => void;
	/** 送出顯示全文；reject 時對話框留在原地並就地顯示原因，輸入的理由保留。 */
	onConfirm: (reason: string) => Promise<void>;
}

/**
 * 顯示全文（RevealKYCSensitive）：理由必填，會寫進稽核紀錄並授權這位管理員 15 分鐘內檢視全文與證件影像。
 * 文案照決策簡報 D4 第 5 點。版面沿用關閉店家確認框（同一組需要理由的操作）。
 */
export default function RevealKycDialog({
	storeName,
	onClose,
	onConfirm,
}: RevealKycDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = validateReason(
			reason,
			"請填寫顯示全文的理由，它會記在稽核紀錄。",
		);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm(reason.trim());
		} catch (caught) {
			setError(
				connectErrorText(
					caught,
					"顯示全文失敗，沒有留下紀錄；請確認網路狀態後再試一次。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			title="顯示全文"
			titleIcon="lock_open"
			description={REVEAL_DIALOG_TEXT}
			closeLabel="關閉顯示全文視窗"
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
						返回
					</Button>
					<Button variant="primary" type="submit" loading={submitting}>
						{submitting ? "正在顯示" : "顯示全文"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>申請的店家</span>
				<strong>{storeName}</strong>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-reveal-kyc-reason">顯示全文的理由（必填）</label>
				<textarea
					id="adm-reveal-kyc-reason"
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					rows={3}
					placeholder="例如：核對身分證字號與證件影像是否一致"
					disabled={submitting}
				/>
				<span className="adm-operation-field-note">
					理由會記在稽核紀錄，最多 {REASON_MAX_LENGTH} 字。
				</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
