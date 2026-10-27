"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	type ProspectMatch,
	ProspectMatchTier,
	SellerProspectAdminService,
	type VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { useRouter } from "next/navigation";
import { type FormEvent, useEffect, useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import {
	emptyProspectForm,
	type ProspectForm,
	parsePriorityStars,
	previewRequest,
	prospectFormError,
	prospectInit,
	taxIdFormatError,
	VENDOR_TYPE_OPTIONS,
	vendorTypeText,
} from "@/lib/seller-prospects";
import ContactsEditor from "./ContactsEditor";
import { MatchCard } from "./MatchCard";
import { Field, type FieldSpec, fs } from "./ProspectFormFields";
import ProspectMatchDialog from "./ProspectMatchDialog";
import { useProspectCache } from "./prospect-queries";
import { useProspectMatchRetry } from "./use-prospect-match-retry";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 站外業者建立對話框，含即時查重：名稱、統編與聯絡方式停打後送 `PreviewProspect`（不寫入），命中列在表單下方。
 * 送出時伺服器還會再比對一次；撞到相符的業者（`PROSPECT_MATCH_FOUND`）就開 `ProspectMatchDialog`，確認都不是同一家後
 * 帶全部 ID 重送。
 */

const DEBOUNCE_MS = 300;
const CREATE_FAILED_TEXT = "建檔失敗，請確認欄位後再試一次。";
const PREVIEW_FAILED_TEXT =
	"即時查重暫時失敗，不影響建立；送出時仍會再比對一次。";

type TextFields = Pick<
	ProspectForm,
	| "displayName"
	| "taxId"
	| "category"
	| "region"
	| "productSummary"
	| "positioning"
	| "researchSummary"
	| "priorityLetter"
	| "priorityStars"
>;

export default function CreateProspectDialog({
	onClose,
}: {
	onClose: () => void;
}) {
	const router = useRouter();
	const { showToast } = useToast();
	const cache = useProspectCache();
	const create = useMutation(SellerProspectAdminService.method.createProspect);

	const [form, setForm] = useState<ProspectForm>(emptyProspectForm);
	const [fieldError, setFieldError] = useState<string | null>(null);
	const [submitError, setSubmitError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const setField = <K extends keyof ProspectForm>(
		key: K,
		value: ProspectForm[K],
	) => {
		setForm((f) => ({ ...f, [key]: value }));
		setFieldError(null);
		setSubmitError(null);
	};

	const taxIdError = taxIdFormatError(form.taxId);
	const starsInvalid = parsePriorityStars(form.priorityStars) === null;

	const liveRequest = previewRequest(form);
	const liveKey = JSON.stringify(liveRequest);
	const [debouncedRequest, setDebouncedRequest] = useState(liveRequest);
	useEffect(() => {
		const timer = setTimeout(
			() => setDebouncedRequest(liveRequest),
			DEBOUNCE_MS,
		);
		return () => clearTimeout(timer);
	}, [liveKey]);

	const preview = useQuery(
		SellerProspectAdminService.method.previewProspect,
		debouncedRequest ?? undefined,
		{ enabled: debouncedRequest !== null },
	);
	const matches: ProspectMatch[] =
		debouncedRequest !== null ? (preview.data?.matches ?? []) : [];
	// 統編相同（T1）幾乎確定是同一家，排最前面。
	const sortedMatches = [...matches].sort(
		(a, b) =>
			Number(b.tier === ProspectMatchTier.T1_TAX_ID) -
			Number(a.tier === ProspectMatchTier.T1_TAX_ID),
	);

	const send = async (confirmedIds: string[]) => {
		const result = await runAction(
			() =>
				create.mutateAsync({
					prospect: prospectInit(form, { includeOutreach: false }),
					confirmedDistinctProspectIds: confirmedIds,
				}),
			{ fallback: CREATE_FAILED_TEXT, conflictText: CREATE_FAILED_TEXT },
		);
		if (result.ok) {
			const created = result.value.prospect;
			cache.written(created);
			showToast("已建立業者", "success");
			if (created?.id) router.push(`/admin/seller-prospects/${created.id}`);
			else onClose();
		}
		return result;
	};
	const match = useProspectMatchRetry({
		send,
		onFailure: (failure) => setSubmitError(failure.text),
	});

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = prospectFormError(form);
		if (invalid) {
			setFieldError(invalid);
			return;
		}
		if (starsInvalid) return;
		setFieldError(null);
		setSubmitError(null);
		setSubmitting(true);
		try {
			await match.submit();
		} finally {
			setSubmitting(false);
		}
	};

	let notice: GovernanceNotice | null = null;
	if (fieldError) notice = { tone: "error", text: fieldError };
	else if (submitError) notice = { tone: "error", text: submitError };
	else if (preview.isError) {
		notice = {
			tone: "error",
			text: connectErrorText(preview.error, PREVIEW_FAILED_TEXT),
		};
	}

	const rows: FieldSpec<TextFields>[][] = [
		[
			fs("prospect-display-name", "displayName", "業者名稱", {
				required: true,
			}),
		],
	];
	const taxIdField = fs<TextFields>("prospect-tax-id", "taxId", "統一編號", {
		invalid: !!taxIdError,
		hint: taxIdError ?? "選填；業者名稱、統編與聯絡方式都會用於下方的即時查重",
	});
	const lowerRows: FieldSpec<TextFields>[][] = [
		[
			fs("prospect-category", "category", "分類"),
			fs("prospect-region", "region", "地區"),
		],
		[
			fs("prospect-product-summary", "productSummary", "商品摘要", {
				as: "textarea",
			}),
		],
		[fs("prospect-positioning", "positioning", "市場定位", { as: "textarea" })],
		[
			fs("prospect-research-summary", "researchSummary", "調研備註", {
				as: "textarea",
			}),
		],
		[
			fs("prospect-priority-letter", "priorityLetter", "業務優先順位（字母）", {
				hint: "例如 A／B／C，選填",
			}),
			fs("prospect-priority-stars", "priorityStars", "業務優先星等", {
				hint: "例如 5，選填",
				invalid: starsInvalid,
				numeric: true,
			}),
		],
	];

	const renderRow = (row: FieldSpec<TextFields>[]) => (
		<div key={row[0].id} className={row.length > 1 ? "info-grid" : undefined}>
			{row.map((spec) => (
				<Field
					key={spec.id}
					{...spec}
					value={form[spec.field]}
					onChange={(v) => setField(spec.field, v)}
					disabled={submitting}
				/>
			))}
		</div>
	);

	return (
		<Dialog
			title="新增站外業者"
			closeLabel="關閉新增站外業者視窗"
			maxWidth={640}
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="outline"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button variant="primary" type="submit" loading={submitting}>
						{submitting ? "建立中…" : "建立"}
					</Button>
				</>
			}
		>
			{rows.map(renderRow)}
			<Field
				id="prospect-vendor-type"
				label="業者型態"
				required
				as="select"
				options={VENDOR_TYPE_OPTIONS.map((value) => ({
					value: String(value),
					label: vendorTypeText(value),
				}))}
				value={String(form.vendorType)}
				onChange={(v) => setField("vendorType", Number(v) as VendorType)}
				disabled={submitting}
			/>
			{renderRow([taxIdField])}
			<ContactsEditor
				idPrefix="prospect-contact"
				rows={form.contacts}
				onChange={(contacts) => setField("contacts", contacts)}
				disabled={submitting}
			/>
			{lowerRows.map(renderRow)}

			{preview.isFetching ? (
				<p
					aria-live="polite"
					style={{ fontSize: 13, color: "var(--gray-600)" }}
				>
					<MaterialSymbol name="progress_activity" />{" "}
					正在確認是否有可能重複的業者…
				</p>
			) : sortedMatches.length > 0 ? (
				<div className="form-group" aria-live="polite">
					<div className="form-label">
						可能重複（{sortedMatches.length} 筆）
					</div>
					{sortedMatches.map((m) => (
						<MatchCard key={m.prospect?.id ?? m.tier} match={m} />
					))}
				</div>
			) : null}
			<GovernanceNoticeBanner notice={notice} />

			{match.pending ? (
				<ProspectMatchDialog
					key={match.pending.ids.join(",")}
					mode="create"
					prospectIds={match.pending.ids}
					candidate={previewRequest(form) ?? {}}
					initiallyConfirmed={match.pending.confirmed}
					onConfirm={match.confirm}
					onCancel={match.cancel}
				/>
			) : null}
		</Dialog>
	);
}
