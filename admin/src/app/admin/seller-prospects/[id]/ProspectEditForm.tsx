"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	type OutreachStatus,
	type Prospect,
	ProspectSchema,
	ProspectStatus,
	SellerProspectAdminService,
	type VendorType,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { useRouter } from "next/navigation";
import { type FormEvent, useRef, useState } from "react";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import {
	canReuseEtag,
	EDITABLE_OUTREACH_STATUSES,
	mergeDraftOntoIncoming,
	outreachStatusText,
	type ProspectForm,
	parsePriorityStars,
	previewRequest,
	prospectFormError,
	prospectInit,
	prospectToForm,
	sortIdsNumerically,
	taxIdFormatError,
	updateMaskPaths,
	VENDOR_TYPE_OPTIONS,
	vendorTypeText,
} from "@/lib/seller-prospects";
import ContactsEditor from "../ContactsEditor";
import MergeDialog from "../MergeDialog";
import { Field, type FieldSpec, fs } from "../ProspectFormFields";
import ProspectMatchDialog, { taxIdHolderId } from "../ProspectMatchDialog";
import { useProspectMatchRetry } from "../use-prospect-match-retry";
import "@/styles/pages/DelegatedOperations.css";

/**
 * 站外業者欄位編輯。草稿與它開啟時的基準分開存：送出時 update_mask 只列和基準不同的欄位。
 * 頁面持有唯一的 `prospect`（含 etag），文字表單與圖片區塊都以它的 etag 寫入；別人（或換圖）寫入使 etag 換新時，
 * 圖片以外的可寫欄位都等於基準才沿用新 etag，否則當作衝突：草稿保留、管理員沒改過的欄位換成新版本的值。
 */

const UPDATE_FAILED_TEXT = "更新失敗，請確認欄位後再試一次。";
const UPDATE_CONFLICT_TEXT =
	"資料已被其他操作更新。畫面已重新讀取，你輸入的內容還在，請確認後再送出。";

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

interface MergeTarget {
	leftId: string;
	rightId: string;
	defaultWinnerId?: string;
}

export default function ProspectEditForm({
	prospect,
	onUpdated,
	reload,
	readOnly = false,
}: {
	prospect: Prospect;
	/** 寫入成功後以回應換掉頁面持有的業者。 */
	onUpdated: (prospect: Prospect) => void;
	reload: () => Promise<unknown>;
	/** 非進行中的業者：欄位只供檢視，沒有儲存鍵。 */
	readOnly?: boolean;
}) {
	const router = useRouter();
	const { showToast } = useToast();
	const update = useMutation(SellerProspectAdminService.method.updateProspect);
	const [base, setBase] = useState<ProspectForm>(() =>
		prospectToForm(prospect),
	);
	const [seenEtag, setSeenEtag] = useState(prospect.etag);
	const [draft, setDraft] = useState<ProspectForm>(base);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [mergeTarget, setMergeTarget] = useState<MergeTarget | null>(null);
	const [holderId, setHolderId] = useState<string | null>(null);
	const ownEtag = useRef<string>("");

	if (prospect.etag !== seenEtag) {
		setSeenEtag(prospect.etag);
		if (prospect.etag !== ownEtag.current) {
			const incoming = prospectToForm(prospect);
			if (!canReuseEtag(base, incoming)) {
				setDraft(mergeDraftOntoIncoming(base, draft, incoming));
				setNotice({ tone: "error", text: UPDATE_CONFLICT_TEXT });
			}
			setBase(incoming);
		}
	}

	const setField = <K extends keyof ProspectForm>(
		key: K,
		value: ProspectForm[K],
	) => {
		setDraft((f) => ({ ...f, [key]: value }));
		setNotice(null);
		setHolderId(null);
	};

	const holder = useQuery(
		SellerProspectAdminService.method.getProspect,
		{ id: holderId ?? "" },
		{ enabled: holderId !== null },
	).data?.prospect;

	const openMerge = (other: Prospect, defaultWinnerId?: string) => {
		const [leftId, rightId] = sortIdsNumerically(prospect.id, other.id);
		setMergeTarget({ leftId, rightId, defaultWinnerId });
	};

	const handleMerged = (winner: Prospect) => {
		setMergeTarget(null);
		if (winner.id !== prospect.id) {
			router.push(`/admin/seller-prospects/${winner.id}`);
			return;
		}
		const next = prospectToForm(winner);
		ownEtag.current = winner.etag;
		setBase(next);
		setDraft(next);
		setNotice(null);
		setHolderId(null);
	};

	const paths = updateMaskPaths(base, draft);
	const taxIdError = taxIdFormatError(draft.taxId);
	const starsInvalid = parsePriorityStars(draft.priorityStars) === null;

	const send = async (confirmedIds: string[]) => {
		const result = await runAction(
			() =>
				update.mutateAsync({
					prospect: {
						...prospectInit(draft, { includeOutreach: true }),
						id: prospect.id,
						etag: prospect.etag,
					},
					updateMask: fieldMask(ProspectSchema, paths),
					confirmedDistinctProspectIds: confirmedIds,
				}),
			{
				reload,
				fallback: UPDATE_FAILED_TEXT,
				conflictText: UPDATE_CONFLICT_TEXT,
			},
		);
		const saved = result.ok ? result.value.prospect : undefined;
		if (saved) {
			const next = prospectToForm(saved);
			ownEtag.current = saved.etag;
			setBase(next);
			setDraft(next);
			setNotice(null);
			onUpdated(saved);
			showToast("已儲存", "success");
		}
		return result;
	};
	const match = useProspectMatchRetry({
		send,
		onFailure: (failure) => {
			setNotice({ tone: "error", text: failure.text });
			setHolderId(taxIdHolderId(failure));
		},
	});

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting || paths.length === 0) return;
		const invalid = prospectFormError(draft);
		if (invalid) {
			setNotice({ tone: "error", text: invalid });
			return;
		}
		if (starsInvalid) return;
		setNotice(null);
		setHolderId(null);
		setSubmitting(true);
		try {
			await match.submit();
		} finally {
			setSubmitting(false);
		}
	};

	const rows: FieldSpec<TextFields>[][] = [
		[
			fs("prospect-edit-display-name", "displayName", "業者名稱", {
				required: true,
			}),
		],
	];
	const taxIdRow: FieldSpec<TextFields>[] = [
		fs("prospect-edit-tax-id", "taxId", "統一編號", {
			invalid: !!taxIdError,
			hint: taxIdError ?? "選填",
		}),
	];
	const lowerRows: FieldSpec<TextFields>[][] = [
		[
			fs("prospect-edit-category", "category", "分類"),
			fs("prospect-edit-region", "region", "地區"),
		],
		[
			fs("prospect-edit-product-summary", "productSummary", "商品摘要", {
				as: "textarea",
			}),
		],
		[
			fs("prospect-edit-positioning", "positioning", "市場定位", {
				as: "textarea",
			}),
		],
		[
			fs("prospect-edit-research-summary", "researchSummary", "調研備註", {
				as: "textarea",
			}),
		],
		[
			fs(
				"prospect-edit-priority-letter",
				"priorityLetter",
				"業務優先順位（字母）",
				{ hint: "例如 A／B／C，選填" },
			),
			fs("prospect-edit-priority-stars", "priorityStars", "業務優先星等", {
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
					value={draft[spec.field]}
					onChange={(v) => setField(spec.field, v)}
					disabled={submitting || readOnly}
				/>
			))}
		</div>
	);

	const outreachOptions = [
		...EDITABLE_OUTREACH_STATUSES.map((value) => ({
			value: String(value),
			label: outreachStatusText(value),
		})),
		...(EDITABLE_OUTREACH_STATUSES.includes(draft.outreachStatus)
			? []
			: [
					{
						value: String(draft.outreachStatus),
						label: outreachStatusText(draft.outreachStatus),
						disabled: true,
					},
				]),
	];

	return (
		<>
			<form onSubmit={handleSubmit} noValidate>
				{rows.map(renderRow)}
				<div className="info-grid">
					<Field
						id="prospect-edit-vendor-type"
						label="業者型態"
						required
						as="select"
						options={VENDOR_TYPE_OPTIONS.map((value) => ({
							value: String(value),
							label: vendorTypeText(value),
						}))}
						value={String(draft.vendorType)}
						onChange={(v) => setField("vendorType", Number(v) as VendorType)}
						disabled={submitting || readOnly}
					/>
					<Field
						id="prospect-edit-outreach-status"
						label="招商狀態"
						required
						as="select"
						options={outreachOptions}
						value={String(draft.outreachStatus)}
						onChange={(v) =>
							setField("outreachStatus", Number(v) as OutreachStatus)
						}
						disabled={submitting || readOnly}
					/>
				</div>
				{renderRow(taxIdRow)}
				<ContactsEditor
					idPrefix="prospect-edit-contact"
					rows={draft.contacts}
					onChange={(contacts) => setField("contacts", contacts)}
					disabled={submitting || readOnly}
				/>
				{lowerRows.map(renderRow)}

				<GovernanceNoticeBanner notice={notice} />
				{notice && holder?.status === ProspectStatus.ACTIVE ? (
					<Button
						variant="outline"
						type="button"
						onClick={() => openMerge(holder, holder.id)}
					>
						{`與「${holder.displayName || "（未命名）"}」合併`}
					</Button>
				) : null}

				{readOnly ? null : (
					<div
						style={{
							display: "flex",
							justifyContent: "flex-end",
							marginTop: 16,
						}}
					>
						<Button
							variant="primary"
							type="submit"
							disabled={paths.length === 0}
							loading={submitting}
						>
							{submitting ? "儲存中…" : "儲存變更"}
						</Button>
					</div>
				)}

				{match.pending ? (
					<ProspectMatchDialog
						key={match.pending.ids.join(",")}
						mode="update"
						prospectIds={match.pending.ids}
						candidate={previewRequest(draft) ?? {}}
						initiallyConfirmed={match.pending.confirmed}
						onConfirm={match.confirm}
						onMerge={(other) => {
							match.cancel();
							openMerge(other);
						}}
						onCancel={match.cancel}
					/>
				) : null}
			</form>
			{mergeTarget ? (
				<MergeDialog
					leftId={mergeTarget.leftId}
					rightId={mergeTarget.rightId}
					defaultWinnerId={mergeTarget.defaultWinnerId}
					draftWillBeLost
					onMerged={handleMerged}
					onClose={() => setMergeTarget(null)}
				/>
			) : null}
		</>
	);
}
