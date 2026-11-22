"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { goTrimmedRuneCount, trimGoSpace } from "@hawkr/api/text/server-text";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Select, Textarea } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { newRequestId } from "@/lib/chat/request-id";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { queryKeys } from "@/lib/query/keys";
import { type CommunityReportTarget, submitCommunityReport } from "./api";
import {
	COMMUNITY_DESCRIPTION_MAX,
	communityReportCategoryOptions,
	validateCommunityReport,
} from "./report-logic";

export interface CommunityReportDialogProps {
	open: boolean;
	onClose: () => void;
	target: CommunityReportTarget;
}

type Outcome =
	| { kind: "idle" }
	| { kind: "error"; message: string }
	| { kind: "done" };

/**
 * 評價與聊天訊息的檢舉表單（`ModerationAppService.SubmitReport`）：選原因，選了「其他」才要說明。
 * 內容沒變的重試帶同一個 request_id（伺服器回第一次的結果），內容變了換新的。
 */
export default function CommunityReportDialog({
	open,
	onClose,
	target,
}: CommunityReportDialogProps) {
	const t = useTranslations("report");
	const ut = useUiText();
	const queryClient = useQueryClient();
	const identity = useQueryIdentity();
	const defaults = useDefaultErrorText();
	const categories = React.useMemo(
		() => communityReportCategoryOptions(ut),
		[ut],
	);
	const [category, setCategory] = React.useState("");
	const [description, setDescription] = React.useState("");
	const [busy, setBusy] = React.useState(false);
	const [showErrors, setShowErrors] = React.useState(false);
	const [outcome, setOutcome] = React.useState<Outcome>({ kind: "idle" });
	const attempt = React.useRef<{
		key: string;
		requestId: string;
	} | null>(null);

	React.useEffect(() => {
		if (!open) return;
		setCategory("");
		setDescription("");
		setShowErrors(false);
		setOutcome({ kind: "idle" });
		attempt.current = null;
	}, [open]);

	const selected = categories.find((c) => String(c.value) === category);
	const errors = validateCommunityReport({
		category: selected?.value,
		description,
	});
	const close = () => {
		if (busy) return;
		onClose();
	};

	const submit = async () => {
		if (busy) return;
		setShowErrors(true);
		if (Object.keys(errors).length > 0 || !selected) return;
		const text = trimGoSpace(description);
		const key = `${selected.value}\u0000${text}`;
		if (attempt.current?.key !== key) {
			attempt.current = { key, requestId: newRequestId() };
		}
		setBusy(true);
		setOutcome({ kind: "idle" });
		try {
			await submitCommunityReport({
				target,
				category: selected.value,
				description: text,
				requestId: attempt.current.requestId,
			});
			void queryClient.invalidateQueries({
				queryKey: queryKeys.buyer.myReports(identity),
			});
			setOutcome({ kind: "done" });
		} catch (e) {
			setOutcome({ kind: "error", message: errorText(e, defaults) });
		} finally {
			setBusy(false);
		}
	};

	const errorLabel = (
		e:
			| "categoryRequired"
			| "descriptionRequired"
			| "descriptionTooLong"
			| undefined,
	) =>
		e === undefined
			? undefined
			: t(
					{
						categoryRequired: "errorCategoryRequired",
						descriptionRequired: "errorDescriptionRequired",
						descriptionTooLong: "errorDescriptionTooLong",
					}[e],
					{ max: COMMUNITY_DESCRIPTION_MAX },
				);
	const done = outcome.kind === "done";
	const descriptionRequired = selected?.value === PolicyCategory.OTHER;

	return (
		<Dialog
			open={open}
			onClose={close}
			size="md"
			title={target.kind === "review" ? t("titleReview") : t("titleMessage")}
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
							>
								{t("submit")}
							</Button>
						</>
					)}
				</div>
			}
		>
			{done ? (
				<p className="m-0 text-sm text-hk-heading" role="status">
					{t("received")}
				</p>
			) : (
				<div className="grid gap-4">
					<Select
						label={t("category")}
						size="lg"
						isRequired
						value={category}
						onChange={(e) => setCategory(e.target.value)}
						disabled={busy}
						isInvalid={showErrors && errors.category !== undefined}
						errorMessage={showErrors ? errorLabel(errors.category) : undefined}
					>
						<option value="">{t("categoryPlaceholder")}</option>
						{categories.map((c) => (
							<option key={c.value} value={String(c.value)}>
								{c.label}
							</option>
						))}
					</Select>
					<div className="grid gap-1">
						<Textarea
							label={
								descriptionRequired
									? t("description")
									: t("descriptionOptional")
							}
							variant="bordered"
							size="lg"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder={t("descriptionPlaceholder")}
							minRows={3}
							disabled={busy}
							isInvalid={showErrors && errors.description !== undefined}
							errorMessage={
								showErrors ? errorLabel(errors.description) : undefined
							}
						/>
						<span className="text-right text-xs text-hk-muted tabular-nums">
							{goTrimmedRuneCount(description)}/{COMMUNITY_DESCRIPTION_MAX}
						</span>
					</div>
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
