"use client";

import { timestampDate, timestampMs } from "@bufbuild/protobuf/wkt";
import { Code } from "@connectrpc/connect";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	CaseDecision,
	CaseState,
	ModerationAdminService,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Field, Section } from "@/app/admin/governance/DetailBits";
import { GovernanceNoticeBanner } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { measureStateBadge } from "@/app/admin/merchants/[id]/measure";
import { Button, Dialog, StatusBadge, useToast } from "@/components/UI";
import { type ConflictText, runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { policyCategoryText } from "@/lib/policy-category";
import {
	CASE_CONFLICT_TEXT,
	canDecide,
	canReverse,
	DECIDE_FAILED_TEXT,
	REASON_MAX_LENGTH,
	REVERSE_FAILED_TEXT,
	REVERSE_PRECONDITION_TEXT,
	validateReverseReason,
} from "../../case-decide";
import { snapshotView } from "../../case-snapshot";
import {
	caseActionText,
	caseDecisionText,
	caseStateText,
	caseStateVariant,
	reporterText,
	reportOutcomeText,
	targetKindText,
} from "../../moderation-text";
import DecideModerationCaseDialog from "./DecideModerationCaseDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";

const conflictText: ConflictText = CASE_CONFLICT_TEXT;

function timeText(value: Parameters<typeof timestampDate>[0] | undefined) {
	return value ? formatDateTimeShort(timestampDate(value)) : "-";
}

/**
 * 社群檢舉案件詳情：案件與它的全部檢舉（舊到新）。進行中的案件可以決定（帶讀到的 etag；版本衝突時重讀）；
 * 已結案是終態，不顯示決定的操作。版本衝突時重讀；重讀後案件已結案（對話框隨之消失），衝突說明改用 toast。
 */
export default function ModerationCaseDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const queryClient = useQueryClient();
	const { showToast } = useToast();
	const [dialog, setDialog] = useState(false);
	const [reverseOpen, setReverseOpen] = useState(false);
	const [reverseReason, setReverseReason] = useState("");
	const [reverseError, setReverseError] = useState<string | null>(null);
	const [reversing, setReversing] = useState(false);
	const detail = useQuery(ModerationAdminService.method.getCase, { id });
	const decide = useMutation(ModerationAdminService.method.decideCase, {
		onSuccess: () => queryClient.invalidateQueries(),
	});
	const reverse = useMutation(ModerationAdminService.method.reverseHide, {
		onSuccess: () => queryClient.invalidateQueries(),
	});
	const kase = detail.data?.case;
	const measureId = kase?.measureId;
	const measure = useQuery(
		SellerGovernanceAdminService.method.getMeasure,
		{ id: measureId ?? "" },
		{ enabled: Boolean(measureId) },
	);
	const measureState = measure.data?.measure
		? measureStateBadge(measure.data.measure.state, measure.data.measure.kind)
		: undefined;
	const reports = [...(detail.data?.reports ?? [])].sort(
		(a, b) =>
			(a.createTime ? timestampMs(a.createTime) : 0) -
			(b.createTime ? timestampMs(b.createTime) : 0),
	);

	const back = (
		<button
			type="button"
			className="btn-back"
			aria-label="返回列表"
			onClick={() => router.push("/admin/reports")}
		>
			<MaterialSymbol name="keyboard_arrow_left" />
		</button>
	);

	if (detail.isLoading) {
		return (
			<div className="certification-detail-page">
				<p role="status">載入案件中...</p>
			</div>
		);
	}
	if (detail.error || !kase) {
		return (
			<div className="certification-detail-page">
				<div className="detail-header">
					<div className="header-left">{back}</div>
				</div>
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{connectErrorText(detail.error, "案件讀取失敗")}</div>
				</div>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const handleDecide = async (
		request: Parameters<typeof decide.mutateAsync>[0],
	) => {
		let stillDecidable = true;
		const result = await runAction(() => decide.mutateAsync(request), {
			reload: async () => {
				const reloaded = await detail.refetch();
				stillDecidable = !reloaded.data?.case || canDecide(reloaded.data.case);
			},
			fallback: DECIDE_FAILED_TEXT,
			conflictText,
		});
		if (!result.ok) {
			if (result.kind === "conflict" && !stillDecidable) {
				setDialog(false);
				showToast(result.text, "warning");
				return;
			}
			throw new Error(result.text);
		}
		setDialog(false);
	};

	const submitReverse = async () => {
		if (reversing) return;
		const invalid = validateReverseReason(reverseReason);
		if (invalid) {
			setReverseError(invalid);
			return;
		}
		setReverseError(null);
		setReversing(true);
		const result = await runAction(
			() => reverse.mutateAsync({ id, reason: reverseReason }),
			{
				reload: async () => {
					await detail.refetch();
				},
				fallback: REVERSE_FAILED_TEXT,
				conflictText,
			},
		);
		setReversing(false);
		if (!result.ok) {
			setReverseError(
				result.code === Code.FailedPrecondition
					? REVERSE_PRECONDITION_TEXT
					: result.text,
			);
			return;
		}
		setReverseOpen(false);
		setReverseReason("");
	};
	const snapshot = detail.data?.snapshot;
	const snapshotRows = snapshotView(snapshot);

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">社群檢舉案件</h1>
							<StatusBadge variant={caseStateVariant(kase.state)}>
								{caseStateText(kase.state)}
							</StatusBadge>
						</div>
						<div className="header-meta">
							案件 {kase.id}・開案於 {timeText(kase.createTime)}
						</div>
					</div>
				</div>
			</div>

			{canDecide(kase) && (
				<div className="header-actions" style={{ display: "flex", gap: 16 }}>
					<Button variant="primary" onClick={() => setDialog(true)}>
						決定
					</Button>
				</div>
			)}

			{canReverse(kase) && (
				<div className="header-actions" style={{ display: "flex", gap: 16 }}>
					<Button variant="outline" onClick={() => setReverseOpen(true)}>
						撤銷隱藏
					</Button>
				</div>
			)}

			<Section id="moderation-case-summary" icon="folder" title="案件">
				<div className="info-grid">
					<Field label="對象種類">
						{targetKindText(kase.target?.kind ?? 0)}
					</Field>
					<Field label="對象">{kase.target?.id ?? "-"}</Field>
					<Field label="檢舉數">{kase.reportCount.toString()}</Field>
					{kase.storeId && <Field label="店家">{kase.storeId}</Field>}
					{kase.state === CaseState.CLOSED && (
						<>
							<Field label="決定">{caseDecisionText(kase.decision)}</Field>
							{kase.decision === CaseDecision.VIOLATION && (
								<>
									<Field label="類別">
										{policyCategoryText(kase.category)}
									</Field>
									<Field label="處置">{caseActionText(kase.action)}</Field>
								</>
							)}
							<Field label="結案的管理員">{kase.decidedBy || "-"}</Field>
							<Field label="結案時間">{timeText(kase.resolveTime)}</Field>
							{kase.reversal && (
								<>
									<Field label="隱藏已撤銷於">
										{timeText(kase.reversal.reverseTime)}
									</Field>
									<Field label="撤銷的管理員">
										{kase.reversal.reversedBy || "-"}
									</Field>
									<Field label="撤銷原因">{kase.reversal.reason || "-"}</Field>
								</>
							)}
						</>
					)}
				</div>
			</Section>

			<Section
				id="moderation-case-snapshot"
				icon="description"
				title="被檢舉內容"
			>
				{snapshot === undefined ? (
					<p>沒有內容快照（結案滿保留期後已清除）。</p>
				) : snapshotRows === undefined ? (
					<p>內容種類無法辨識。</p>
				) : (
					<>
						<p className="adm-operation-field-note">
							{snapshotRows.title}・檢舉當下的版本
						</p>
						<div className="info-grid">
							{snapshotRows.fields.map((field) => (
								<Field key={field.label} label={field.label}>
									{field.value}
								</Field>
							))}
						</div>
						{snapshotRows.context.length > 0 && (
							<>
								<p className="adm-operation-field-note">前後文</p>
								<ul className="adm-operation-list">
									{snapshotRows.context.map((line) => (
										<li key={line.seq} data-tombstone={line.tombstone}>
											#{line.seq}・{line.text}
										</li>
									))}
								</ul>
							</>
						)}
					</>
				)}
			</Section>

			{measureId && (
				<Section id="moderation-case-measure" icon="policy" title="下架措施">
					<div className="info-grid">
						<Field label="措施">
							<Link href={`/admin/measures/${measureId}`}>{measureId}</Link>
						</Field>
						<Field label="目前狀態">
							{measure.isLoading ? (
								"載入中..."
							) : measureState ? (
								<StatusBadge variant={measureState.variant}>
									{measureState.label}
								</StatusBadge>
							) : (
								connectErrorText(measure.error, "措施讀取失敗")
							)}
						</Field>
					</div>
				</Section>
			)}

			<Section
				id="moderation-case-reports"
				icon="flag"
				title={`檢舉（${reports.length}）`}
			>
				{reports.length === 0 ? (
					<p>沒有檢舉。</p>
				) : (
					<ul className="adm-operation-list">
						{reports.map((report) => (
							<li key={report.id}>
								<div>
									{timeText(report.createTime)}・{reporterText(report)}・
									{policyCategoryText(report.category)}・
									{reportOutcomeText(report.outcome)}
								</div>
								<div>{report.description || "（沒有說明）"}</div>
							</li>
						))}
					</ul>
				)}
			</Section>

			{reverseOpen && canReverse(kase) && (
				<Dialog
					title="撤銷隱藏"
					closeLabel="關閉"
					onClose={() => setReverseOpen(false)}
					busy={reversing}
					footer={
						<>
							<Button
								variant="outline"
								onClick={() => setReverseOpen(false)}
								disabled={reversing}
							>
								取消
							</Button>
							<Button
								variant="primary"
								onClick={submitReverse}
								disabled={reversing}
							>
								{reversing ? "送出中…" : "確認撤銷"}
							</Button>
						</>
					}
				>
					<p className="adm-operation-field-note">
						內容會恢復顯示；案件的決定不變，撤銷只能做一次。
					</p>
					<label className="form-label" htmlFor="moderation-reverse-reason">
						撤銷原因 <span className="required">*</span>
					</label>
					<textarea
						id="moderation-reverse-reason"
						className="form-textarea"
						rows={3}
						value={reverseReason}
						disabled={reversing}
						onChange={(e) => {
							setReverseReason(e.target.value);
							setReverseError(null);
						}}
					/>
					<p className="adm-operation-field-note">
						最多 {REASON_MAX_LENGTH} 字。
					</p>
					<GovernanceNoticeBanner
						notice={reverseError ? { tone: "error", text: reverseError } : null}
					/>
				</Dialog>
			)}

			{dialog && canDecide(kase) && (
				<DecideModerationCaseDialog
					kase={kase}
					onClose={() => setDialog(false)}
					onConfirm={handleDecide}
				/>
			)}
		</div>
	);
}
