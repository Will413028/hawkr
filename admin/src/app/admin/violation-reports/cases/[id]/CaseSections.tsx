"use client";

import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import type { Report } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	type Case,
	CaseCloseReason,
	CaseState,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { PolicyCategory } from "@hawkr/api/gen/hawkr/common/v1alpha1/policy_category_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { Field, Section } from "@/app/admin/governance/DetailBits";
import NotesSection from "@/app/admin/governance/NotesSection";
import { violationSeverityText } from "@/app/admin/merchants/[id]/measure";
import { Button, DeadlineLine, StatusBadge } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { policyCategoryText } from "@/lib/policy-category";
import {
	buildCreateCaseNote,
	NEW_SUPPLEMENTS_BADGE,
} from "../../report-actions";
import {
	caseStatusBadge,
	reportStatusBadge,
	storeLabel,
} from "../../report-list";
import {
	actorTimeText,
	derivedMeasureText,
	moveRecordText,
	reportSummary,
	supplementSummary,
} from "../../report-view";
import { useAllMeasures } from "../../use-all-measures";

const NONE = "-";

export const NO_REPORTS_TEXT =
	"這個案件的通報都已移到其他案件，案件已自動關閉，不算一次決定。";

/** 案件：店家、類別、嚴重度、狀態、開案與結案的人和時間、給檢舉人的結果說明。 */
export function CaseSummarySection({ kase, now }: { kase: Case; now: Date }) {
	const status = caseStatusBadge(kase);
	const closed = kase.state === CaseState.CLOSED;
	return (
		<Section id="case-summary-title" icon="folder" title="案件">
			<div className="info-grid">
				<Field label="店家">
					<Link href={`/admin/merchants/${kase.storeId}`}>
						{storeLabel(kase)}
					</Link>
				</Field>
				<Field label="類別">{policyCategoryText(kase.category)}</Field>
				<Field label="嚴重度">{violationSeverityText(kase.severity)}</Field>
				<Field label="狀態">
					<StatusBadge variant={status.variant}>{status.label}</StatusBadge>
				</Field>
				<Field label="開案">
					{actorTimeText(kase.openedByAdminId, kase.openTime)}
				</Field>
				<Field label="結案">
					{closed ? actorTimeText(kase.closedByAdminId, kase.closeTime) : NONE}
				</Field>
				<Field label="給檢舉人的結果說明" fullWidth>
					<span style={{ whiteSpace: "pre-wrap" }}>
						{kase.decisionNote || "沒有說明"}
					</span>
				</Field>
			</div>
			{kase.decideDeadline && (
				<p>
					<DeadlineLine
						label="決定期限"
						deadline={kase.decideDeadline}
						now={now}
					/>
				</p>
			)}
			{closed && kase.closeReason === CaseCloseReason.NO_REPORTS && (
				<p>{NO_REPORTS_TEXT}</p>
			)}
		</Section>
	);
}

export interface CaseReportsView {
	status: "loading" | "error" | "ready";
	error: unknown;
	rows: readonly Report[];
	/** 目前是第幾頁（從 1 起）。 */
	page: number;
	hasNext: boolean;
	onPrevious: () => void;
	onNext: () => void;
	onRetry: () => void;
}

/** 案件裡的通報（舊到新）：每一筆有設定結果與移到其他案件的入口（伺服器說可以才顯示），以及到通報詳情的連結。 */
export function CaseReportsSection({
	kase,
	view,
	onResolve,
	onMove,
}: {
	kase: Case;
	view: CaseReportsView;
	onResolve: (report: Report) => void;
	onMove: (report: Report) => void;
}) {
	return (
		<Section
			id="case-reports-title"
			icon="list_alt"
			title={`通報（${kase.reportCount} 筆）`}
		>
			{view.status === "loading" ? (
				<p role="status">載入中...</p>
			) : view.status === "error" ? (
				<div className="tab-empty-state" role="alert">
					<MaterialSymbol name="error" />
					<span>{connectErrorText(view.error, "通報讀取失敗")}</span>
					<Button variant="outline" onClick={view.onRetry}>
						重試
					</Button>
				</div>
			) : (
				<>
					<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{view.rows.map((report) => {
							const status = reportStatusBadge(report);
							const supplements = supplementSummary(report);
							return (
								<li
									key={report.id}
									style={{
										display: "flex",
										flexWrap: "wrap",
										alignItems: "center",
										gap: 12,
										marginBottom: 12,
									}}
								>
									<Link href={`/admin/violation-reports/${report.id}`}>
										{reportSummary(report)}
									</Link>
									<StatusBadge variant={status.variant}>
										{status.label}
									</StatusBadge>
									{report.hasUnseenSupplements && (
										<StatusBadge variant="warning">
											{NEW_SUPPLEMENTS_BADGE}
										</StatusBadge>
									)}
									{supplements && <span>{supplements}</span>}
									{report.canResolve && (
										<Button variant="outline" onClick={() => onResolve(report)}>
											設定結果
										</Button>
									)}
									{report.canMove && (
										<Button variant="outline" onClick={() => onMove(report)}>
											移到其他案件
										</Button>
									)}
								</li>
							);
						})}
					</ul>
					{(view.page > 1 || view.hasNext) && (
						<div style={{ display: "flex", gap: 8, alignItems: "center" }}>
							<Button
								variant="outline"
								disabled={view.page <= 1}
								onClick={view.onPrevious}
							>
								上一頁
							</Button>
							<span>第 {view.page} 頁</span>
							<Button
								variant="outline"
								disabled={!view.hasNext}
								onClick={view.onNext}
							>
								下一頁
							</Button>
						</div>
					)}
				</>
			)}
		</Section>
	);
}

/** 移動紀錄：移入與移出這個案件的移動（舊到新）；沒有移動時不顯示。 */
export function CaseMovesSection({ kase }: { kase: Case }) {
	if (kase.moves.length === 0) return null;
	return (
		<Section id="case-moves-title" icon="swap_horiz" title="移動紀錄">
			<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
				{kase.moves.map((move) => (
					<li
						key={`${move.reportId}-${move.fromCaseId}-${move.toCaseId}-${move.moveTime?.seconds}`}
					>
						{moveRecordText(move, kase.id)}
					</li>
				))}
			</ul>
		</Section>
	);
}

/** 導出的措施：從這個案件建立的措施，以及依據的認定引用了這個案件的通報的措施；各列連到措施詳情。 */
export function DerivedMeasuresSection({ caseId }: { caseId: string }) {
	const list = useAllMeasures({ sourceCaseId: caseId });
	const measures = list.measures;
	return (
		<Section id="case-measures-title" icon="balance" title="導出的措施">
			<p className="adm-operation-field-note">
				包含從這個案件建立的措施，以及依據的認定引用了這個案件的通報的措施。
			</p>
			{list.loading ? (
				<p role="status">載入中...</p>
			) : list.failed ? (
				<div className="tab-empty-state" role="alert">
					<MaterialSymbol name="error" />
					<span>{connectErrorText(list.error, "措施讀取失敗")}</span>
					<Button variant="outline" onClick={() => list.retry()}>
						重試
					</Button>
				</div>
			) : measures.length === 0 ? (
				<p>還沒有導出的措施</p>
			) : (
				<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
					{measures.map((measure) => (
						<li key={measure.id}>
							<Link href={`/admin/measures/${measure.id}`}>
								{derivedMeasureText(measure)}
							</Link>
						</li>
					))}
				</ul>
			)}
		</Section>
	);
}

const NOTES_PAGE_SIZE = 100;

function useCaseNotes(caseId: string, pageToken: string) {
	return useQuery(SellerGovernanceAdminService.method.listCaseNotes, {
		caseId,
		pageSize: NOTES_PAGE_SIZE,
		pageToken,
	});
}

function useCreateCaseNote(caseId: string) {
	const queryClient = useQueryClient();
	const mutation = useMutation(
		SellerGovernanceAdminService.method.createCaseNote,
		{
			onSuccess: () =>
				queryClient.invalidateQueries({
					queryKey: createConnectQueryKey({
						schema: SellerGovernanceAdminService.method.listCaseNotes,
						cardinality: undefined,
					}),
				}),
		},
	);
	return {
		create: (body: string) =>
			mutation.mutateAsync(buildCreateCaseNote(caseId, body)),
		isPending: mutation.isPending,
	};
}

/** 案件的內部註記；改嚴重度寫的註記在標頭列出改前與改後。 */
export function CaseNotesSection({ caseId }: { caseId: string }) {
	return (
		<NotesSection
			titleId="case-notes-title"
			bodyId="case-note-body"
			scopeId={caseId}
			useNotes={useCaseNotes}
			useCreateNote={useCreateCaseNote}
		/>
	);
}
