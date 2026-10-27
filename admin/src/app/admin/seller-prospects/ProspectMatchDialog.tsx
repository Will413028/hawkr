"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { useTransport } from "@connectrpc/connect-query";
import { metadataList } from "@hawkr/api/connect/errors";
import {
	ContactKind,
	type PreviewProspectRequestSchema,
	type Prospect,
	type ProspectMatchReason,
	ProspectStatus,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, Dialog } from "@/components/UI";
import type { ActionResult } from "@/lib/connect-action";
import {
	contactKindText,
	matchReasonsText,
	primaryContactValue,
	prospectStatusText,
	vendorTypeText,
} from "@/lib/seller-prospects";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

export type MatchDialogMode = "create" | "update" | "restore";

/** 送出被 `PROSPECT_MATCH_FOUND` 擋下時，相符業者的 ID（metadata 的 `prospectIds`）；不是這個原因或沒有 ID 回 null。 */
export function matchFoundIds(result: ActionResult): string[] | null {
	if (result.ok || result.kind !== "message") return null;
	if (result.reason !== ErrorReason.PROSPECT_MATCH_FOUND) return null;
	const ids = metadataList(result.metadata?.prospectIds);
	return ids.length > 0 ? ids : null;
}

/** 送出被 `PROSPECT_TAX_ID_HELD` 擋下時，統編持有者的 ID（metadata 的 `prospectIds`，只有一筆）；不是這個原因或沒有 ID 回 null。 */
export function taxIdHolderId(result: ActionResult): string | null {
	if (result.ok || result.kind !== "message") return null;
	if (result.reason !== ErrorReason.PROSPECT_TAX_ID_HELD) return null;
	return metadataList(result.metadata?.prospectIds)[0] ?? null;
}

/** 重送又撞到時以新的 ID 集合重開：已勾過而且仍在新集合裡的保留勾選。 */
export function keepConfirmed(
	confirmed: readonly string[],
	nextIds: readonly string[],
): string[] {
	return confirmed.filter((id) => nextIds.includes(id));
}

const TEXT = {
	title: "可能是同一家業者",
	create:
		"下列業者的聯絡方式與這次填寫的相同。是同一家的話請取消：進行中的請改去編輯它，已入駐的不需要再建立；確認都不是同一家，逐一勾選後再送出。",
	update:
		"下列業者的聯絡方式與這次修改後的相同。是同一家的話：進行中的可以與它合併，已入駐的請封存這筆；確認都不是同一家，逐一勾選後再儲存。",
	restore:
		"下列業者的聯絡方式與這筆相同。是同一家的話不需要還原；確認都不是同一家，逐一勾選後再還原。",
	confirm: {
		create: "確認不是同一家並建立",
		update: "確認不是同一家並儲存",
		restore: "確認不是同一家並還原",
	},
	distinct: "不是同一家",
	merge: "與這家合併",
	converted: "已入駐，不能合併",
	loading: "正在讀取相符的業者…",
	loadFailed: "無法讀取相符的業者。",
} as const;

interface MatchRow {
	id: string;
	prospect?: Prospect;
	/** Preview 沒回的（並行寫入才出現的）業者沒有命中原因。 */
	reasons?: readonly ProspectMatchReason[];
}

/**
 * 建立、修改、還原撞到相符的業者（`PROSPECT_MATCH_FOUND`）時，請管理員逐一確認「不是同一家」。
 * `prospectIds`（錯誤帶回的）每一筆都 `GetProspect` 取得名稱、型態、狀態與聯絡方式（Preview 的 matches 不帶聯絡方式）；
 * 以送出的內容呼叫一次 `PreviewProspect` 只取命中原因，與 `prospectIds` 取交集，沒回的列不顯示原因。全部勾選才能送出，送出時帶全部 ID（契約：涵蓋全部相符的業者才寫入）。
 *
 * 不用 `Dialog` 的 `form`：React 的 submit 事件會穿過 portal 冒泡到外層建立對話框的 `<form>`，所以按鈕都是
 * `type="button"`。
 */
export default function ProspectMatchDialog({
	mode,
	prospectIds,
	candidate,
	initiallyConfirmed,
	onConfirm,
	onMerge,
	onCancel,
}: {
	mode: MatchDialogMode;
	prospectIds: readonly string[];
	/** 送出的名稱、統編與聯絡方式（給 Preview）。 */
	candidate: MessageInitShape<typeof PreviewProspectRequestSchema>;
	initiallyConfirmed: readonly string[];
	/** 以全部相符業者的 ID 重送；reject 時對話框留在原地並就地顯示原因。 */
	onConfirm: (ids: string[]) => Promise<void>;
	/** 有傳才在進行中的列顯示「與這家合併」，已入駐的列顯示不能合併；由呼叫端決定這次的重送要不要取消並開合併框。 */
	onMerge?: (other: Prospect) => void;
	onCancel: () => void;
}) {
	const transport = useTransport();
	const [confirmed, setConfirmed] = useState<ReadonlySet<string>>(
		() => new Set(keepConfirmed(initiallyConfirmed, prospectIds)),
	);
	const [submitting, setSubmitting] = useState(false);
	const [error, setError] = useState<string | null>(null);

	const rows = useQuery({
		queryKey: ["seller-prospects", "match-dialog", prospectIds, candidate],
		queryFn: async (): Promise<MatchRow[]> => {
			const client = createClient(SellerProspectAdminService, transport);
			const preview = await client.previewProspect(candidate);
			const reasonsById = new Map(
				preview.matches.flatMap((match) =>
					match.prospect ? [[match.prospect.id, match.matchedOn] as const] : [],
				),
			);
			return Promise.all(
				prospectIds.map(async (id): Promise<MatchRow> => {
					const fetched = await client.getProspect({ id });
					return {
						id,
						prospect: fetched.prospect,
						reasons: reasonsById.get(id),
					};
				}),
			);
		},
		retry: false,
	});

	const loaded = rows.data !== undefined;
	const allConfirmed = prospectIds.every((id) => confirmed.has(id));

	const toggle = (id: string, checked: boolean) => {
		setError(null);
		setConfirmed((prev) => {
			const next = new Set(prev);
			if (checked) next.add(id);
			else next.delete(id);
			return next;
		});
	};

	const submit = async () => {
		if (submitting || !loaded || !allConfirmed) return;
		setError(null);
		setSubmitting(true);
		try {
			await onConfirm([...prospectIds]);
		} catch (caught) {
			setError(
				caught instanceof Error && caught.message ? caught.message : null,
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title={TEXT.title}
			maxWidth={640}
			onClose={onCancel}
			busy={submitting}
			footer={
				<>
					<Button
						variant="outline"
						type="button"
						onClick={onCancel}
						disabled={submitting}
					>
						取消
					</Button>
					<Button
						variant="primary"
						type="button"
						onClick={() => void submit()}
						loading={submitting}
						disabled={!loaded || !allConfirmed}
					>
						{TEXT.confirm[mode]}
					</Button>
				</>
			}
		>
			<p className="adm-operation-field-note">{TEXT[mode]}</p>
			{rows.isLoading ? (
				<p
					aria-live="polite"
					style={{ fontSize: 13, color: "var(--gray-600)" }}
				>
					<MaterialSymbol name="progress_activity" /> {TEXT.loading}
				</p>
			) : rows.isError ? (
				<div>
					<GovernanceNoticeBanner
						notice={{ tone: "error", text: TEXT.loadFailed }}
					/>
					<Button variant="outline" onClick={() => void rows.refetch()}>
						重試
					</Button>
				</div>
			) : (
				<ul style={{ listStyle: "none", margin: 0, padding: 0 }}>
					{(rows.data ?? []).map((row) => (
						<MatchRowItem
							key={row.id}
							row={row}
							checked={confirmed.has(row.id)}
							disabled={submitting}
							onToggle={(checked) => toggle(row.id, checked)}
							onMerge={onMerge}
						/>
					))}
				</ul>
			)}
			{error ? (
				<GovernanceNoticeBanner notice={{ tone: "error", text: error }} />
			) : null}
		</Dialog>
	);
}

function MatchRowItem({
	row,
	checked,
	disabled,
	onToggle,
	onMerge,
}: {
	row: MatchRow;
	checked: boolean;
	disabled: boolean;
	onToggle: (checked: boolean) => void;
	onMerge?: (other: Prospect) => void;
}) {
	const prospect = row.prospect;
	const email = prospect
		? primaryContactValue(prospect.contacts, ContactKind.EMAIL)
		: "";
	const phone = prospect
		? primaryContactValue(prospect.contacts, ContactKind.PHONE)
		: "";
	return (
		<li
			style={{
				padding: "10px 14px",
				marginBottom: 8,
				border: "1px solid var(--gray-300)",
				borderRadius: 8,
			}}
		>
			<a
				href={`/admin/seller-prospects/${row.id}`}
				target="_blank"
				rel="noopener noreferrer"
				style={{ fontWeight: 600 }}
			>
				{prospect?.displayName || "（未命名）"}
			</a>
			<div style={{ fontSize: 13, color: "var(--gray-700)" }}>
				{prospect
					? `${vendorTypeText(prospect.vendorType)} · ${prospectStatusText(prospect.status)}`
					: null}
				{row.reasons ? ` · ${matchReasonsText(row.reasons)}` : null}
			</div>
			{email || phone ? (
				<div style={{ fontSize: 13, color: "var(--gray-700)" }}>
					{email ? `${contactKindText(ContactKind.EMAIL)} · ${email}` : null}
					{email && phone ? " ｜ " : null}
					{phone ? `${contactKindText(ContactKind.PHONE)} · ${phone}` : null}
				</div>
			) : null}
			<div style={{ display: "flex", alignItems: "center", gap: 16 }}>
				<label
					style={{
						display: "inline-flex",
						alignItems: "center",
						gap: 6,
						minHeight: 44,
						cursor: "pointer",
					}}
				>
					<input
						type="checkbox"
						checked={checked}
						disabled={disabled}
						onChange={(e) => onToggle(e.target.checked)}
						style={{ width: "auto", minWidth: 0 }}
					/>
					{TEXT.distinct}
				</label>
				{onMerge && prospect?.status === ProspectStatus.ACTIVE ? (
					<Button
						variant="outline"
						size="sm"
						type="button"
						disabled={disabled}
						onClick={() => onMerge(prospect)}
					>
						{TEXT.merge}
					</Button>
				) : null}
				{onMerge && prospect?.status === ProspectStatus.CONVERTED ? (
					<span style={{ fontSize: 13, color: "var(--gray-700)" }}>
						{TEXT.converted}
					</span>
				) : null}
			</div>
		</li>
	);
}
