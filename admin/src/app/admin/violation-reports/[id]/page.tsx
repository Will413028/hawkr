"use client";

import { useMutation, useQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { ReportOutcome } from "@hawkr/api/gen/hawkr/seller/v1alpha1/report_pb";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Field, Section } from "@/app/admin/governance/DetailBits";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, DeadlineLine, StatusBadge } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { policyCategoryText } from "@/lib/policy-category";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { useInvalidateGovernance } from "../governance-queries";
import {
	MOVE_FAILED_TEXT,
	RESOLVE_FAILED_TEXT,
	reportConflictText,
	SOURCE_CASE_CLOSED_TEXT,
	TRIAGE_FAILED_TEXT,
} from "../report-actions";
import { caseColumnText, reportStatusBadge, storeLabel } from "../report-list";
import { reportOutcomeText, supplementMatchNotice } from "../report-text";
import {
	actorTimeText,
	evidenceItems,
	reporterView,
	supplementTitle,
	timeText,
} from "../report-view";
import MoveReportDialog from "./MoveReportDialog";
import ResolveReportDialog from "./ResolveReportDialog";
import TriageReportDialog from "./TriageReportDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

type DialogName = "triage" | "resolve" | "move";

const NONE = "-";

/** 通報詳情：通報內容、補充、檢舉人、佐證與處理；按鈕只看伺服器的 can_*（不能操作時隱藏，不顯示原因）。 */
export default function ReportDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const [dialog, setDialog] = useState<DialogName | null>(null);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const now = useNow(LIST_REFRESH_MS);
	const invalidate = useInvalidateGovernance();

	const detail = useQuery(SellerGovernanceAdminService.method.getReport, {
		id,
	});
	const report = detail.data?.report;

	const G = SellerGovernanceAdminService.method;
	const triage = useMutation(G.triageReport, { onSuccess: invalidate });
	const resolve = useMutation(G.resolveReport, { onSuccess: invalidate });
	const move = useMutation(G.moveReport, { onSuccess: invalidate });
	// 明確的已讀：頁面顯示補充時記下這筆通報的補充看過了（GetReport 本身不改狀態）。帶讀到的 etag；etag 過時（讀到之後又有新的補充）就重讀，
	// 重讀到的新 etag 會讓下面的 effect 再記一次。標記不讓 etag 前進，所以不會讓分案、設定結果等對話框裡的 etag 過時。其他失敗不重試：標記留著，
	// 案件頁的「新的補充」仍然如實顯示還沒看過。
	const markSeen = useMutation(G.markReportSupplementsSeen, {
		onSuccess: invalidate,
		onError: () => void detail.refetch(),
	});
	const unseenId = report?.hasUnseenSupplements ? report.id : undefined;
	const unseenEtag = report?.etag;
	const markSeenMutate = markSeen.mutate;
	useEffect(() => {
		if (unseenId && unseenEtag) {
			markSeenMutate({ id: unseenId, etag: unseenEtag });
		}
	}, [unseenId, unseenEtag, markSeenMutate]);

	/**
	 * 送出通報的操作。VERSION_CONFLICT（別人先改過這筆通報）時 runAction 先重讀再回 conflict：這裡丟出錯誤讓對話框
	 * 留在原地、保留輸入並顯示說明，之後再送帶的是重讀後的 etag；其他失敗丟出伺服器的訊息。成功才關閉對話框；成功時不另外提示。
	 */
	const run = async (
		call: () => Promise<unknown>,
		fallback: string,
	): Promise<void> => {
		const result = await runAction(call, {
			reload: () => detail.refetch(),
			fallback,
			conflictText: reportConflictText,
		});
		if (!result.ok) throw new Error(result.text);
		setDialog(null);
		setNotice(null);
	};

	const back = (
		<button
			type="button"
			className="btn-back"
			aria-label="返回列表"
			onClick={() => router.push("/admin/violation-reports")}
		>
			<MaterialSymbol name="keyboard_arrow_left" />
		</button>
	);

	if (detail.isLoading) {
		return (
			<div className="certification-detail-page">
				<p role="status">載入通報中...</p>
			</div>
		);
	}
	if (detail.error || !report) {
		return (
			<div className="certification-detail-page">
				<div className="detail-header">
					<div className="header-left">{back}</div>
				</div>
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{connectErrorText(detail.error, "通報讀取失敗")}</div>
				</div>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const status = reportStatusBadge(report);
	const reporter = reporterView(report);
	const evidence = evidenceItems(report);

	const handleMove = async (
		request: Parameters<typeof move.mutateAsync>[0],
	) => {
		let sourceCaseClosed = false;
		await run(async () => {
			const res = await move.mutateAsync(request);
			sourceCaseClosed = res.sourceCaseClosed;
		}, MOVE_FAILED_TEXT);
		if (sourceCaseClosed) {
			setNotice({ tone: "success", text: SOURCE_CASE_CLOSED_TEXT });
		}
	};

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">通報詳情</h1>
							<StatusBadge variant={status.variant}>{status.label}</StatusBadge>
						</div>
						<div className="header-meta">
							<Link href={`/admin/merchants/${report.storeId}`}>查看店家</Link>
							<span>
								・通報 {report.id}・建立於 {timeText(report.createTime)}
							</span>
						</div>
					</div>
				</div>
			</div>

			<GovernanceNoticeBanner notice={notice} />

			<div
				className="header-actions"
				style={{ display: "flex", flexWrap: "wrap", gap: 16 }}
			>
				{report.canTriage && (
					<Button variant="primary" onClick={() => setDialog("triage")}>
						分案
					</Button>
				)}
				{report.canResolve && (
					<Button variant="primary" onClick={() => setDialog("resolve")}>
						設定結果
					</Button>
				)}
				{report.canMove && (
					<Button variant="outline" onClick={() => setDialog("move")}>
						移到其他案件
					</Button>
				)}
				{report.caseId && (
					<Button
						variant="outline"
						onClick={() =>
							router.push(`/admin/violation-reports/cases/${report.caseId}`)
						}
					>
						查看案件
					</Button>
				)}
			</div>

			<Section id="report-content-title" icon="list_alt" title="通報內容">
				<div className="info-grid">
					<Field label="店家">
						<Link href={`/admin/merchants/${report.storeId}`}>
							{storeLabel(report)}
						</Link>
					</Field>
					<Field label="檢舉人選的類別">
						{policyCategoryText(report.category)}
					</Field>
					<Field label="商品 ID">{report.productId || NONE}</Field>
					<Field label="訂單編號">{report.orderNo || NONE}</Field>
					<Field label="說明" fullWidth>
						<span style={{ whiteSpace: "pre-wrap" }}>
							{report.description || NONE}
						</span>
					</Field>
				</div>
			</Section>

			{(report.supplements.length > 0 ||
				report.discardedSubmissionCount > 0) && (
				<Section id="report-supplements-title" icon="add_comment" title="補充">
					<ol style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{report.supplements.map((supplement, i) => {
							const matched = supplementMatchNotice(supplement.matchedBy);
							return (
								<li key={supplement.id} style={{ marginBottom: 12 }}>
									<div className="adm-operation-field-note">
										{supplementTitle(i, supplement.createTime)}
									</div>
									{matched && (
										<div className="adm-operation-field-note">{matched}</div>
									)}
									<div style={{ whiteSpace: "pre-wrap" }}>
										{supplement.description}
									</div>
								</li>
							);
						})}
					</ol>
					{report.discardedSubmissionCount > 0 && (
						<p className="adm-operation-field-note">
							另有 {report.discardedSubmissionCount}{" "}
							次重複送出因補充已達上限，內容沒有存下。
						</p>
					)}
				</Section>
			)}

			<Section id="report-reporter-title" icon="person" title="檢舉人">
				{reporter.kind === "buyer" ? (
					<p>買家 ID {reporter.buyerId}</p>
				) : (
					<>
						<p>匿名</p>
						{reporter.contact.length > 0 && (
							<div className="info-grid">
								{reporter.contact.map((row) => (
									<Field key={row.label} label={row.label}>
										{row.value}
									</Field>
								))}
							</div>
						)}
						{reporter.erasedText && <p>{reporter.erasedText}</p>}
					</>
				)}
			</Section>

			<Section id="report-evidence-title" icon="attach_file" title="佐證">
				{evidence.length === 0 ? (
					<p>沒有佐證</p>
				) : (
					<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{evidence.map((item) => (
							<li key={item.label}>
								{item.href ? (
									<a href={item.href} target="_blank" rel="noopener noreferrer">
										{item.label}
									</a>
								) : (
									item.label
								)}
							</li>
						))}
					</ul>
				)}
			</Section>

			<Section id="report-handling-title" icon="account_tree" title="處理">
				<div className="info-grid">
					<Field label="狀態">
						<StatusBadge variant={status.variant}>{status.label}</StatusBadge>
					</Field>
					<Field label="平台判定類別">
						{report.assessedCategory === PolicyCategory.UNSPECIFIED
							? NONE
							: policyCategoryText(report.assessedCategory)}
					</Field>
					<Field label="案件">
						{report.caseId ? (
							<Link href={`/admin/violation-reports/cases/${report.caseId}`}>
								{caseColumnText(report)}
							</Link>
						) : (
							NONE
						)}
					</Field>
					<Field label="分案">
						{actorTimeText(report.triagedByAdminId, report.triageTime)}
					</Field>
					<Field label="結果">
						{report.outcome === ReportOutcome.UNSPECIFIED
							? NONE
							: reportOutcomeText(report.outcome)}
					</Field>
					<Field label="結案">
						{actorTimeText(report.resolvedByAdminId, report.resolveTime)}
					</Field>
					<Field label="給檢舉人的結果說明" fullWidth>
						<span style={{ whiteSpace: "pre-wrap" }}>
							{report.resolutionNote || "沒有說明"}
						</span>
					</Field>
				</div>
				{report.triageDeadline && (
					<p>
						<DeadlineLine
							label="分案期限"
							deadline={report.triageDeadline}
							now={now}
						/>
					</p>
				)}
			</Section>

			{dialog === "triage" && (
				<TriageReportDialog
					report={report}
					onClose={() => setDialog(null)}
					onConfirm={(request) =>
						run(() => triage.mutateAsync(request), TRIAGE_FAILED_TEXT)
					}
				/>
			)}
			{dialog === "resolve" && (
				<ResolveReportDialog
					report={report}
					onClose={() => setDialog(null)}
					onConfirm={(request) =>
						run(() => resolve.mutateAsync(request), RESOLVE_FAILED_TEXT)
					}
				/>
			)}
			{dialog === "move" && (
				<MoveReportDialog
					report={report}
					onClose={() => setDialog(null)}
					onConfirm={handleMove}
				/>
			)}
		</div>
	);
}
