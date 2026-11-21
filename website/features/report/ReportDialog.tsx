"use client";

import { Code } from "@connectrpc/connect";
import { fieldMessage, submitError } from "@hawkr/api/connect/submit";
import type { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { trimGoSpace } from "@hawkr/api/text/server-text";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Input, Select } from "@/components/ui";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import { createReport } from "./api";
import {
	DescriptionField,
	EvidenceField,
	useEvidenceUploads,
} from "./ReportFormParts";
import {
	DESCRIPTION_MAX,
	ORDER_NO_MAX,
	type ReportFormError,
	reportCategoryOptions,
	reportSuccessText,
	validateReportDraft,
} from "./report-logic";

export interface ReportDialogProps {
	open: boolean;
	onClose: () => void;
	storeId: string;
	/** 從商品頁開啟時帶入：預填 product_id 並顯示商品名。 */
	product?: { id: string; name: string };
}

const RENDERED_FIELDS = new Set([
	"category",
	"description",
	"order_no",
	"evidence_staged_tokens",
]);

type Outcome =
	| { kind: "idle" }
	| { kind: "error"; message: string }
	| { kind: "done"; message: string };

export default function ReportDialog({
	open,
	onClose,
	storeId,
	product,
}: ReportDialogProps) {
	const t = useTranslations("report");
	const ut = useUiText();
	const defaults = useDefaultErrorText();
	const categories = React.useMemo(() => reportCategoryOptions(ut), [ut]);
	const [category, setCategory] = React.useState<string>("");
	const [description, setDescription] = React.useState("");
	const [orderNo, setOrderNo] = React.useState("");
	const evidence = useEvidenceUploads(open);
	const { uploading } = evidence;
	const [busy, setBusy] = React.useState(false);
	const [showErrors, setShowErrors] = React.useState(false);
	const [serverFields, setServerFields] = React.useState<
		Record<string, string>
	>({});
	const [outcome, setOutcome] = React.useState<Outcome>({ kind: "idle" });

	React.useEffect(() => {
		if (!open) return;
		setCategory("");
		setDescription("");
		setOrderNo("");
		setShowErrors(false);
		setServerFields({});
		setOutcome({ kind: "idle" });
	}, [open]);

	const selected = categories.find((c) => String(c.value) === category);
	const draftErrors = validateReportDraft({
		category: selected?.value,
		description,
		orderNo,
	});
	const errorText = (e: ReportFormError | undefined) =>
		e === undefined
			? undefined
			: t(
					{
						categoryRequired: "errorCategoryRequired",
						descriptionRequired: "errorDescriptionRequired",
						descriptionTooLong: "errorDescriptionTooLong",
						orderNoTooLong: "errorOrderNoTooLong",
					}[e],
					{ max: e === "orderNoTooLong" ? ORDER_NO_MAX : DESCRIPTION_MAX },
				);

	const close = () => {
		if (busy || uploading) return;
		onClose();
	};

	const submit = async () => {
		if (busy) return;
		setShowErrors(true);
		if (Object.keys(draftErrors).length > 0 || !selected || uploading) return;
		setBusy(true);
		setServerFields({});
		setOutcome({ kind: "idle" });
		try {
			const response = await createReport({
				storeId,
				category: selected.value as PolicyCategory,
				description: trimGoSpace(description),
				productId: product?.id,
				orderNo: trimGoSpace(orderNo),
				evidenceStagedTokens: evidence.tokens,
			});
			setOutcome({
				kind: "done",
				message: reportSuccessText(response, t("received")),
			});
		} catch (e) {
			const failure = submitError(e, defaults);
			if (failure.kind === "fields") setServerFields(failure.fields);
			const unmatched =
				failure.kind === "fields" &&
				!Object.keys(failure.fields).some((k) => RENDERED_FIELDS.has(k));
			setOutcome({
				kind: "error",
				message:
					failure.kind === "fields"
						? unmatched
							? Object.values(failure.fields).join("；") ||
								failure.message ||
								defaults(Code.InvalidArgument)
							: (failure.message ?? "")
						: failure.kind === "message"
							? failure.message
							: defaults(Code.Internal),
			});
		} finally {
			setBusy(false);
		}
	};

	const clearServerField = (...keys: string[]) =>
		setServerFields((prev) => {
			if (!keys.some((k) => k in prev)) return prev;
			const next = { ...prev };
			for (const k of keys) delete next[k];
			return next;
		});

	const title = product ? t("titleProduct") : t("titleStore");
	const done = outcome.kind === "done";
	const fieldError = (
		key: "category" | "description" | "orderNo",
		...paths: string[]
	) =>
		(showErrors ? errorText(draftErrors[key]) : undefined) ??
		fieldMessage(serverFields, ...paths);

	return (
		<Dialog
			open={open}
			onClose={close}
			size="md"
			title={title}
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
								disabled={uploading}
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
					{outcome.message}
				</p>
			) : (
				<div className="grid gap-4">
					{product ? (
						<p className="m-0 text-sm text-hk-muted">
							{t("targetProduct", { name: product.name })}
						</p>
					) : null}
					<Select
						label={t("category")}
						size="lg"
						isRequired
						value={category}
						onChange={(e) => {
							setCategory(e.target.value);
							clearServerField("category");
						}}
						disabled={busy}
						isInvalid={fieldError("category", "category") !== undefined}
						errorMessage={fieldError("category", "category")}
					>
						<option value="">{t("categoryPlaceholder")}</option>
						{categories.map((c) => (
							<option key={c.value} value={String(c.value)}>
								{c.label}
							</option>
						))}
					</Select>
					<DescriptionField
						value={description}
						onChange={(next) => {
							setDescription(next);
							clearServerField("description");
						}}
						placeholder={t("descriptionPlaceholder")}
						disabled={busy}
						error={fieldError("description", "description")}
					/>
					<Input
						variant="bordered"
						size="lg"
						label={t("orderNo")}
						value={orderNo}
						onChange={(e) => {
							setOrderNo(e.target.value);
							clearServerField("order_no");
						}}
						disabled={busy}
						isInvalid={fieldError("orderNo", "order_no") !== undefined}
						errorMessage={fieldError("orderNo", "order_no")}
					/>
					<EvidenceField
						evidence={evidence}
						disabled={busy}
						error={fieldMessage(serverFields, "evidence_staged_tokens")}
						onChanged={() => clearServerField("evidence_staged_tokens")}
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
