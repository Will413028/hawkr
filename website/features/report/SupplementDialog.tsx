"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import { fieldMessage } from "@hawkr/api/connect/submit";
import { trimGoSpace } from "@hawkr/api/text/server-text";
import { useTranslations } from "next-intl";
import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { addReportSupplement } from "./api";
import type { StoreReportDetail } from "./hooks";
import {
	DescriptionField,
	EvidenceField,
	submitFailureView,
	useEvidenceUploads,
} from "./ReportFormParts";
import {
	DESCRIPTION_MAX,
	type SupplementFormError,
	validateSupplementDescription,
} from "./report-logic";

export interface SupplementDialogProps {
	open: boolean;
	onClose: () => void;
	reportId: string;
	/** 補充成功：用回應的完整詳情取代畫面資料。 */
	onAdded: (latest: StoreReportDetail) => void;
	/** 伺服器以前置條件拒絕（已有結果、已滿 10 則）：呼叫端重讀詳情並顯示伺服器訊息，對話框隨即關閉。 */
	onStale: (message: string) => void;
}

const RENDERED_FIELDS = new Set(["description", "evidence_staged_tokens"]);

type Outcome =
	| { kind: "idle" }
	| { kind: "error"; message: string }
	| { kind: "done" };

export default function SupplementDialog({
	open,
	onClose,
	reportId,
	onAdded,
	onStale,
}: SupplementDialogProps) {
	const t = useTranslations("report");
	const defaults = useDefaultErrorText();
	const evidence = useEvidenceUploads(open);
	const [description, setDescription] = React.useState("");
	const [busy, setBusy] = React.useState(false);
	const [showErrors, setShowErrors] = React.useState(false);
	const [outcome, setOutcome] = React.useState<Outcome>({ kind: "idle" });
	const [serverFields, setServerFields] = React.useState<
		Record<string, string>
	>({});

	React.useEffect(() => {
		if (!open) return;
		setDescription("");
		setShowErrors(false);
		setOutcome({ kind: "idle" });
		setServerFields({});
	}, [open]);

	const draftError: SupplementFormError | undefined =
		validateSupplementDescription(description);
	const localError = showErrors
		? draftError === "descriptionRequired"
			? t("errorDescriptionRequired")
			: draftError === "descriptionTooLong"
				? t("errorDescriptionTooLong", { max: DESCRIPTION_MAX })
				: undefined
		: undefined;
	const fieldError = localError ?? fieldMessage(serverFields, "description");

	const close = () => {
		if (busy || evidence.uploading) return;
		onClose();
	};

	const submit = async () => {
		if (busy) return;
		setShowErrors(true);
		if (draftError !== undefined || evidence.uploading) return;
		setBusy(true);
		setServerFields({});
		setOutcome({ kind: "idle" });
		try {
			const response = await addReportSupplement({
				reportId,
				description: trimGoSpace(description),
				evidenceStagedTokens: evidence.tokens,
			});
			onAdded(response);
			setOutcome({ kind: "done" });
		} catch (e) {
			if (ConnectError.from(e).code === Code.FailedPrecondition) {
				onStale(errorText(e, defaults));
				onClose();
				return;
			}
			const failure = submitFailureView(e, defaults, RENDERED_FIELDS);
			setServerFields(failure.fields);
			setOutcome({ kind: "error", message: failure.message });
		} finally {
			setBusy(false);
		}
	};

	const done = outcome.kind === "done";
	return (
		<Dialog
			open={open}
			onClose={close}
			size="md"
			title={t("supplementTitle")}
			footer={
				<div className="flex w-full justify-end gap-2">
					{done ? (
						<Button onClick={onClose}>{t("close")}</Button>
					) : (
						<>
							<Button
								variant="light"
								color="default"
								onClick={close}
								disabled={busy}
							>
								{t("cancel")}
							</Button>
							<Button
								onClick={() => void submit()}
								isLoading={busy}
								loadingLabel={t("submitting")}
								disabled={evidence.uploading}
							>
								{t("supplementSubmit")}
							</Button>
						</>
					)}
				</div>
			}
		>
			{done ? (
				<p className="m-0 text-sm text-hk-heading" role="status">
					{t("supplementReceived")}
				</p>
			) : (
				<div className="grid gap-4">
					<DescriptionField
						value={description}
						onChange={(next) => {
							setDescription(next);
							setServerFields((prev) => {
								if (!("description" in prev)) return prev;
								const { description: _drop, ...rest } = prev;
								return rest;
							});
						}}
						placeholder={t("supplementPlaceholder")}
						disabled={busy}
						error={fieldError}
					/>
					<EvidenceField
						evidence={evidence}
						disabled={busy}
						error={fieldMessage(serverFields, "evidence_staged_tokens")}
					/>
					{outcome.kind === "error" && outcome.message ? (
						<p
							className="m-0 text-sm font-semibold text-danger-700"
							role="alert"
						>
							{outcome.message}
						</p>
					) : null}
				</div>
			)}
		</Dialog>
	);
}
