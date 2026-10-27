"use client";

import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { REJECT_REASON_EMPTY_TEXT } from "@/lib/food-registration";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import {
	REASON_MAX_LENGTH,
	validateReason,
} from "../../merchants/[id]/store-detail";

interface RejectFoodRegistrationDialogProps {
	storeName: string;
	registrationNumber: string;
	onClose: () => void;
	onConfirm: (reason: string) => Promise<void>;
}

export default function RejectFoodRegistrationDialog({
	storeName,
	registrationNumber,
	onClose,
	onConfirm,
}: RejectFoodRegistrationDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = validateReason(reason, REJECT_REASON_EMPTY_TEXT);
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
				connectErrorText(caught, "退回失敗，登錄沒有變動；請稍後再試一次。"),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title="退回食品業者登錄"
			titleIcon="cancel"
			description="退回理由賣家看得到；店家目前公開的字號不變。"
			closeLabel="關閉退回視窗"
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
					<Button variant="danger" type="submit" loading={submitting}>
						{submitting ? "正在退回" : "退回"}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>{storeName || "店家"}</span>
				<strong>{registrationNumber}</strong>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-reject-food-reason">退回理由（必填）</label>
				<textarea
					id="adm-reject-food-reason"
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					rows={3}
					placeholder="例如：食藥署查無這個字號"
					disabled={submitting}
				/>
				<span className="adm-operation-field-note">
					最多 {REASON_MAX_LENGTH} 字。
				</span>
			</div>

			<GovernanceNoticeBanner
				notice={error ? { tone: "error", text: error } : null}
			/>
		</Dialog>
	);
}
