"use client";

import { timestampDate, timestampMs } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	CaseDecision,
	CaseState,
	ModerationAdminService,
} from "@hawkr/api/gen/hawkr/admin/moderation/v1alpha1/moderation_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Field, Section } from "@/app/admin/governance/DetailBits";
import { Button, StatusBadge } from "@/components/UI";
import { type ConflictText, runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import { policyCategoryText } from "@/lib/policy-category";
import {
	CASE_CONFLICT_TEXT,
	canDecide,
	DECIDE_FAILED_TEXT,
} from "../../case-decide";
import {
	caseActionText,
	caseDecisionText,
	caseStateText,
	caseStateVariant,
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
 * 已結案是終態，不顯示決定的操作。
 */
export default function ModerationCaseDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const queryClient = useQueryClient();
	const [dialog, setDialog] = useState(false);
	const detail = useQuery(ModerationAdminService.method.getCase, { id });
	const decide = useMutation(ModerationAdminService.method.decideCase, {
		onSuccess: () => queryClient.invalidateQueries(),
	});
	const kase = detail.data?.case;
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
		const result = await runAction(() => decide.mutateAsync(request), {
			reload: () => detail.refetch(),
			fallback: DECIDE_FAILED_TEXT,
			conflictText,
		});
		if (!result.ok) throw new Error(result.text);
		setDialog(false);
	};

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

			<Section id="moderation-case-summary" icon="folder" title="案件">
				<div className="info-grid">
					<Field label="對象種類">
						{targetKindText(kase.target?.kind ?? 0)}
					</Field>
					<Field label="對象">{kase.target?.id ?? "-"}</Field>
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
						</>
					)}
				</div>
			</Section>

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
									{timeText(report.createTime)}・檢舉人 {report.reporterId}・
									{policyCategoryText(report.category)}・
									{reportOutcomeText(report.outcome)}
								</div>
								<div>{report.description || "（沒有說明）"}</div>
							</li>
						))}
					</ul>
				)}
			</Section>

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
