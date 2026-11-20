"use client";

import { trimGoSpace } from "@hawkr/api/text/server-text";
import { useTranslations } from "next-intl";
import * as React from "react";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type { StagedPhoto } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSalePhotoUploader } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { toast } from "@/lib/feedback/toast";
import {
	useAdoptInquiry,
	useSubmitInquiryReply,
} from "@/lib/hooks/useSellerInquiries";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import {
	INQUIRY_REPLY_MAX_LENGTH,
	type SellerInquiry,
	settleReplyFailure,
} from "@/lib/seller/inquiries";
import { uploadInquiryEvidence } from "./inquiryEvidenceUpload";
import { itemErrorsByToken, replyDraftState } from "./inquiryReply";

/**
 * 詢問的回覆表單：一則內文加最多 `replyAttachmentMax` 張機敏附件。一則詢問只能回覆一次（送出後表單消失，改顯示已回覆）；
 * 送出失敗若是已被回覆或撤回，重讀後依伺服器的 `canReply` 關閉表單。
 */
export default function InquiryReplyForm({
	inquiry,
}: {
	inquiry: SellerInquiry;
}) {
	const t = useTranslations("seller.inquiries");
	const defaults = useDefaultErrorText();
	const submitReply = useSubmitInquiryReply();
	const adoptInquiry = useAdoptInquiry();
	const [body, setBody] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);
	const [busy, setBusy] = React.useState(false);
	const [notice, setNotice] = React.useState<string | null>(null);
	const [bodyError, setBodyError] = React.useState<string | null>(null);
	const [evidenceError, setEvidenceError] = React.useState<string | null>(null);
	const [itemErrors, setItemErrors] = React.useState<Record<string, string>>(
		{},
	);

	const draft = replyDraftState({
		body,
		hasFailedPhoto: photos.some((photo) => itemErrors[photo.token]),
		busy,
	});

	const submit = async () => {
		if (!draft.canSubmit) return;
		const submittedTokens = photos.map((photo) => photo.token);
		setBusy(true);
		setNotice(null);
		setBodyError(null);
		setEvidenceError(null);
		setItemErrors({});
		try {
			await submitReply(inquiry.id, {
				body: trimGoSpace(body),
				evidenceStagedTokens: submittedTokens,
			});
			toast.success(t("reply.submitted"));
			return;
		} catch (e) {
			const outcome = await settleReplyFailure(e, defaults, inquiry.id);
			switch (outcome.kind) {
				case "closed":
					await adoptInquiry(outcome.latest);
					toast.error(t("reply.closed"));
					return;
				case "fields":
					setBodyError(outcome.fields.body ?? null);
					setEvidenceError(outcome.fields.evidence ?? null);
					setItemErrors(
						itemErrorsByToken(outcome.fields.evidenceByIndex, submittedTokens),
					);
					setNotice(outcome.fields.other ?? null);
					break;
				case "message":
					setNotice(outcome.text);
					break;
			}
		} finally {
			setBusy(false);
		}
	};

	const bodyMessage = bodyError ?? (draft.tooLong ? t("reply.tooLong") : null);

	return (
		<Card className="flex flex-col gap-4 p-5">
			<label className="flex flex-col gap-1.5">
				<span className="text-sm font-semibold text-[var(--hkui-foreground)]">
					{t("reply.bodyLabel")}{" "}
					<span className="text-[var(--hkui-danger)]">*</span>
				</span>
				<textarea
					value={body}
					onChange={(e) => {
						setBody(e.target.value);
						setBodyError(null);
					}}
					rows={6}
					placeholder={t("reply.bodyPlaceholder")}
					disabled={busy}
					aria-invalid={bodyMessage !== null}
					className="w-full resize-none rounded-[var(--hkui-radius-md)] border border-[var(--hkui-border)] bg-[var(--hkui-surface)] px-3 py-2 text-sm text-[var(--hkui-foreground)] outline-none placeholder:text-[var(--hkui-foreground-muted)] focus-visible:![box-shadow:var(--hkui-focus-ring)]"
				/>
				<span
					className={`self-end text-xs ${draft.tooLong ? "text-[var(--hkui-danger)]" : "text-[var(--hkui-foreground-muted)]"}`}
				>
					{draft.length} / {INQUIRY_REPLY_MAX_LENGTH}
				</span>
				{bodyMessage && (
					<p
						className="m-0 text-xs font-semibold text-[var(--hkui-danger)]"
						role="alert"
					>
						{bodyMessage}
					</p>
				)}
			</label>

			<AfterSalePhotoUploader
				value={photos}
				onChange={(next) => {
					setPhotos(next);
					setEvidenceError(null);
				}}
				max={inquiry.replyAttachmentMax}
				disabled={busy}
				label={t("reply.evidenceLabel")}
				hint={t("reply.evidenceHint")}
				upload={uploadInquiryEvidence}
				itemErrors={itemErrors}
				error={evidenceError ?? undefined}
			/>

			{notice && (
				<p
					className="m-0 text-xs font-semibold text-[var(--hkui-danger)]"
					role="alert"
				>
					{notice}
				</p>
			)}

			<div className="flex justify-end">
				<Button
					color="primary"
					onClick={() => void submit()}
					disabled={!draft.canSubmit}
					isLoading={busy}
					loadingLabel={t("reply.submitting")}
				>
					{t("reply.submit")}
				</Button>
			</div>
		</Card>
	);
}
