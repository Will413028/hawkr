"use client";

import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import type { StagedPhoto } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { AfterSalePhotoUploader } from "@/features/after-sales/components/AfterSalePhotoUploader";
import { SellerCardGridSkeleton } from "@/features/seller-center/components/SellerSkeletons";
import { toast } from "@/lib/feedback/toast";
import {
	useAdoptMeasure,
	useSellerMeasure,
	useSubmitSellerAppeal,
	useSubmitSellerRemediation,
} from "@/lib/hooks/useSellerMeasures";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import {
	APPEAL_MAX_EVIDENCE,
	APPEAL_REASON_MAX_LENGTH,
	appealClosedNotice,
	fetchMyMeasure,
	REMEDIATION_SUBMITTED_NOTICE,
	remediationClosedNotice,
	type SellerMeasure,
	settleAppealFailure,
	settleRemediationFailure,
} from "@/lib/seller/measures";
import { uploadAppealEvidence } from "./appealEvidenceUpload";
import {
	MEASURE_LOAD_FAILED_DESCRIPTION,
	MEASURE_LOAD_FAILED_TITLE,
	MEASURE_NOT_FOUND_ACTION,
	MEASURE_NOT_FOUND_DESCRIPTION,
	MEASURE_NOT_FOUND_TITLE,
	MEASURES_PATH,
	measureHref,
	measureTitle,
	rejectionCooldownWarning,
} from "./measureDisplay";

export type MeasureRequestKind = "appeal" | "remediation";

const APPEAL_SUBMITTED_NOTICE = "申覆已送出，請等候審核";
const CLOSED_DESCRIPTION = "請回到措施與申覆查看目前狀態。";

/** 兩種請求的文字；欄位、字數與佐證規則相同。 */
const KIND_TEXT: Record<
	MeasureRequestKind,
	{
		title: string;
		reasonLabel: string;
		placeholder: string;
		submit: string;
		submitting: string;
		tooLong: string;
	}
> = {
	appeal: {
		title: "提出申覆",
		reasonLabel: "申覆理由",
		placeholder: "請說明此次措施有誤的理由，平台會盡快協助複核。",
		submit: "送出申覆",
		submitting: "申覆送出中…",
		tooLong: `申覆理由最多 ${APPEAL_REASON_MAX_LENGTH} 字`,
	},
	remediation: {
		title: "請求解除措施",
		reasonLabel: "改善說明",
		placeholder: "請說明已完成的改善，平台會盡快複核。",
		submit: "送出請求",
		submitting: "請求送出中…",
		tooLong: `改善說明最多 ${APPEAL_REASON_MAX_LENGTH} 字`,
	},
};

/** 兩種請求的差異：伺服器判斷能不能提出、不能提出時的說明、失敗的處理與成功的提示。 */
const KIND_RULES = {
	appeal: {
		canSubmit: (m: SellerMeasure) => m.canAppeal,
		closedNotice: appealClosedNotice,
		settle: settleAppealFailure,
		submittedNotice: APPEAL_SUBMITTED_NOTICE,
	},
	remediation: {
		canSubmit: (m: SellerMeasure) => m.canRequestRemediation,
		closedNotice: remediationClosedNotice,
		settle: settleRemediationFailure,
		submittedNotice: REMEDIATION_SUBMITTED_NOTICE,
	},
} as const;

/**
 * 提出申覆（`/seller/measures/[measureId]/appeal`）與請求解除（`/seller/measures/[measureId]/remediation`）的獨立頁，
 * 同一個元件、不同的請求種類。措施先用列表快取裡的那筆，沒有才 `GetMyMeasure`，所以網址可以直接開啟或重新整理。
 * 打開時伺服器已說不能提出（`canAppeal`／`canRequestRemediation`）就顯示原因，不畫表單；表單一旦顯示就不會因為背景
 * 重讀而卸載（否則已輸入的理由與已上傳的照片會消失）。
 */
export default function SellerMeasureRequestPage({
	kind,
	measureId,
}: {
	kind: MeasureRequestKind;
	measureId: string;
}) {
	const { measure, loading, error, notFound, refetch } =
		useSellerMeasure(measureId);
	const title = KIND_TEXT[kind].title;
	const rules = KIND_RULES[kind];
	const [formShown, setFormShown] = React.useState(false);
	if (measure && rules.canSubmit(measure) && !formShown) setFormShown(true);

	if (!measure) {
		if (notFound) {
			return (
				<RequestPanel
					title={title}
					heading={MEASURE_NOT_FOUND_TITLE}
					description={MEASURE_NOT_FOUND_DESCRIPTION}
					action={
						<Button href={MEASURES_PATH} variant="bordered" color="default">
							{MEASURE_NOT_FOUND_ACTION}
						</Button>
					}
				/>
			);
		}
		if (error) {
			return (
				<RequestPanel
					title={title}
					heading={MEASURE_LOAD_FAILED_TITLE}
					description={MEASURE_LOAD_FAILED_DESCRIPTION}
					action={
						<Button
							variant="bordered"
							color="default"
							onClick={() => void refetch()}
						>
							重試
						</Button>
					}
				/>
			);
		}
		if (loading) {
			return (
				<SellerPageShell title={title}>
					<SellerCardGridSkeleton variant="panel" count={1} />
				</SellerPageShell>
			);
		}
		return null;
	}

	if (!formShown) {
		return (
			<RequestPanel
				title={title}
				heading={rules.closedNotice(measure)}
				description={CLOSED_DESCRIPTION}
				action={
					<Button href={MEASURES_PATH} variant="bordered" color="default">
						{MEASURE_NOT_FOUND_ACTION}
					</Button>
				}
			/>
		);
	}
	return <RequestForm kind={kind} latest={measure} />;
}

function RequestPanel({
	title,
	heading,
	description,
	action,
}: {
	title: string;
	heading: string;
	description: string;
	action: React.ReactNode;
}) {
	return (
		<SellerPageShell title={title}>
			<Card className="flex flex-col items-center gap-3 p-6 text-center">
				<strong className="text-[0.9375rem] text-[var(--hkui-foreground)]">
					{heading}
				</strong>
				<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
					{description}
				</p>
				{action}
			</Card>
		</SellerPageShell>
	);
}

function RequestForm({
	kind,
	latest,
}: {
	kind: MeasureRequestKind;
	latest: SellerMeasure;
}) {
	const router = useObservedRouter("seller_measure_request");
	const defaults = useDefaultErrorText();
	const ut = useUiText();
	const submitAppeal = useSubmitSellerAppeal();
	const submitRemediation = useSubmitSellerRemediation();
	const adoptMeasure = useAdoptMeasure();
	const text = KIND_TEXT[kind];
	const rules = KIND_RULES[kind];
	const submitRequest =
		kind === "remediation" ? submitRemediation : submitAppeal;
	const [measure, setMeasure] = React.useState(latest);
	const [reason, setReason] = React.useState("");
	const [photos, setPhotos] = React.useState<StagedPhoto[]>([]);
	const [busy, setBusy] = React.useState(false);
	const [notice, setNotice] = React.useState<string | null>(null);
	const [reasonError, setReasonError] = React.useState<string | null>(null);
	const [evidenceError, setEvidenceError] = React.useState<string | null>(null);
	const [itemErrors, setItemErrors] = React.useState<Record<string, string>>(
		{},
	);

	React.useEffect(() => {
		setMeasure(latest);
	}, [latest]);

	const leave = () => router.replace(measureHref(latest));
	const cooldownWarning = rejectionCooldownWarning(ut, measure);

	const reasonLength = goTrimmedRuneCount(reason);
	const reasonTooLong = reasonLength > APPEAL_REASON_MAX_LENGTH;
	const hasFailedPhoto = photos.some((photo) => itemErrors[photo.token]);
	const canSubmit =
		reasonLength > 0 && !reasonTooLong && !hasFailedPhoto && !busy;

	const submit = async () => {
		if (!canSubmit) return;
		const submittedTokens = photos.map((photo) => photo.token);
		setBusy(true);
		setNotice(null);
		setReasonError(null);
		setEvidenceError(null);
		setItemErrors({});
		try {
			await submitRequest(measure, {
				reason: trimGoSpace(reason),
				evidenceStagedTokens: submittedTokens,
			});
			toast.success(rules.submittedNotice);
			leave();
			return;
		} catch (e) {
			// 重讀這一筆用本檔匯入的 `fetchMyMeasure`，由伺服器判斷的欄位決定留下還是離開。
			const outcome = await rules.settle(
				e,
				defaults,
				measure.id,
				fetchMyMeasure,
			);
			switch (outcome.kind) {
				case "closed":
					adoptMeasure(outcome.latest);
					toast.error(outcome.notice);
					leave();
					return;
				case "refreshed":
					adoptMeasure(outcome.latest);
					setMeasure(outcome.latest);
					setNotice(outcome.notice);
					break;
				case "fields":
					setReasonError(outcome.fields.reason ?? null);
					setEvidenceError(outcome.fields.evidence ?? null);
					// `evidence_staged_tokens[i]` 指的是送出的第 i 個 token，用 token 記錄錯誤才不會因為之後
					// 增刪照片而標錯張。
					setItemErrors(
						Object.fromEntries(
							Object.entries(outcome.fields.evidenceByIndex).flatMap(
								([index, message]) => {
									const token = submittedTokens[Number(index)];
									return token === undefined ? [] : [[token, message]];
								},
							),
						),
					);
					setNotice(outcome.fields.other ?? null);
					break;
				case "message":
					setNotice(outcome.text);
					break;
			}
		}
		setBusy(false);
	};

	const reasonMessage = reasonError ?? (reasonTooLong ? text.tooLong : null);

	return (
		<SellerPageShell title={text.title}>
			<Card className="p-5 text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
				{measure.statement || measureTitle(ut, measure)}
			</Card>
			<Card className="flex flex-col gap-4 p-5">
				<label className="flex flex-col gap-1.5">
					<span className="text-sm font-semibold text-[var(--hkui-foreground)]">
						{text.reasonLabel}{" "}
						<span className="text-[var(--hkui-danger)]">*</span>
					</span>
					<textarea
						value={reason}
						onChange={(e) => {
							setReason(e.target.value);
							setReasonError(null);
						}}
						rows={5}
						placeholder={text.placeholder}
						disabled={busy}
						aria-invalid={reasonMessage !== null}
						className="w-full resize-none rounded-[var(--hkui-radius-md)] border border-[var(--hkui-border)] bg-[var(--hkui-surface)] px-3 py-2 text-sm text-[var(--hkui-foreground)] outline-none placeholder:text-[var(--hkui-foreground-muted)] focus-visible:![box-shadow:var(--hkui-focus-ring)]"
					/>
					<span
						className={`self-end text-xs ${reasonTooLong ? "text-[var(--hkui-danger)]" : "text-[var(--hkui-foreground-muted)]"}`}
					>
						{reasonLength} / {APPEAL_REASON_MAX_LENGTH}
					</span>
					{reasonMessage && (
						<p
							className="m-0 text-xs font-semibold text-[var(--hkui-danger)]"
							role="alert"
						>
							{reasonMessage}
						</p>
					)}
				</label>

				<AfterSalePhotoUploader
					value={photos}
					onChange={(next) => {
						setPhotos(next);
						setEvidenceError(null);
					}}
					max={APPEAL_MAX_EVIDENCE}
					disabled={busy}
					label="佐證圖片（選填）"
					upload={uploadAppealEvidence}
					itemErrors={itemErrors}
					error={evidenceError ?? undefined}
				/>

				{cooldownWarning && (
					<p
						className="m-0 text-xs font-semibold text-[var(--hkui-warning)]"
						role="note"
					>
						{cooldownWarning}
					</p>
				)}

				{notice && (
					<p
						className="m-0 text-xs font-semibold text-[var(--hkui-danger)]"
						role="alert"
					>
						{notice}
					</p>
				)}

				<div className="flex justify-end gap-3">
					<Button
						variant="bordered"
						color="default"
						onClick={leave}
						disabled={busy}
					>
						返回
					</Button>
					<Button
						color="primary"
						onClick={() => void submit()}
						disabled={!canSubmit}
						isLoading={busy}
						loadingLabel={text.submitting}
					>
						{text.submit}
					</Button>
				</div>
			</Card>
		</SellerPageShell>
	);
}
