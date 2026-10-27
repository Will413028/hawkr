"use client";

import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	REVOKE_BUTTON_TEXT,
	REVOKE_DIALOG_TEXT,
	REVOKE_REASON_EMPTY_TEXT,
	REVOKE_REASON_LABEL,
} from "@/lib/food-registration";
import { GovernanceNoticeBanner } from "../../merchants/[id]/GovernanceNotice";
import {
	REASON_MAX_LENGTH,
	validateReason,
} from "../../merchants/[id]/store-detail";

interface RevokeFoodRegistrationDialogProps {
	storeName: string;
	registrationNumber: string;
	onClose: () => void;
	onConfirm: (reason: string) => Promise<void>;
}

export default function RevokeFoodRegistrationDialog({
	storeName,
	registrationNumber,
	onClose,
	onConfirm,
}: RevokeFoodRegistrationDialogProps) {
	const [reason, setReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = validateReason(reason, REVOKE_REASON_EMPTY_TEXT);
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
				connectErrorText(caught, "撤銷失敗，登錄沒有變動；請稍後再試一次。"),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="revoke"
			title={REVOKE_BUTTON_TEXT}
			titleIcon="cancel"
			description={REVOKE_DIALOG_TEXT}
			closeLabel="關閉撤銷視窗"
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
						{submitting ? "正在撤銷" : REVOKE_BUTTON_TEXT}
					</Button>
				</>
			}
		>
			<div className="adm-audit-revoke-target">
				<span>{storeName || "店家"}</span>
				<strong>{registrationNumber}</strong>
			</div>

			<div className="adm-operation-field adm-operation-field--wide">
				<label htmlFor="adm-revoke-food-reason">{REVOKE_REASON_LABEL}</label>
				<textarea
					id="adm-revoke-food-reason"
					value={reason}
					onChange={(event) => {
						setReason(event.target.value);
						setError(null);
					}}
					rows={3}
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
