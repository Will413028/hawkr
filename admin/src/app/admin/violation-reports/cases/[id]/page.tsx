"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { createClient } from "@connectrpc/connect";
import { useMutation, useQuery, useTransport } from "@connectrpc/connect-query";
import type { CaseInquiry } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/case_inquiry_pb";
import {
	type CreateMeasureRequestSchema,
	type Report,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import CreateMeasureDialog from "@/app/admin/merchants/[id]/CreateMeasureDialog";
import CreateRiskMeasureDialog from "@/app/admin/merchants/[id]/CreateRiskMeasureDialog";
import CreateTerminationDialog from "@/app/admin/merchants/[id]/CreateTerminationDialog";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "@/app/admin/merchants/[id]/GovernanceNotice";
import {
	terminationAvailability,
	terminationCreatedText,
} from "@/app/admin/merchants/[id]/measure";
import {
	MEASURE_CREATED_TEXT,
	VIOLATION_MEASURE_CREATED_TEXT,
} from "@/app/admin/merchants/[id]/store-detail";
import { useStoreDetail } from "@/app/admin/merchants/[id]/use-store-detail";
import { Button, StatusBadge } from "@/components/UI";
import { type ConflictText, runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { useScrollToHash } from "@/lib/use-scroll-to-hash";
import MoveReportDialog from "../../[id]/MoveReportDialog";
import ResolveReportDialog from "../../[id]/ResolveReportDialog";
import { useInvalidateGovernance } from "../../governance-queries";
import {
	INQUIRY_ALREADY_REPLIED_TEXT,
	INQUIRY_ALREADY_WITHDRAWN_TEXT,
	INQUIRY_FAILED_TEXT,
} from "../../inquiry-actions";
import {
	CASE_CONFLICT_TEXT,
	DECIDE_FAILED_TEXT,
	MOVE_FAILED_TEXT,
	RESOLVE_FAILED_TEXT,
	reportConflictText,
	SEVERITY_FAILED_TEXT,
	SOURCE_CASE_CLOSED_TEXT,
	SOURCE_OVERFLOW_TEXT,
	unseenSupplementsText,
} from "../../report-actions";
import { caseStatusBadge } from "../../report-list";
import { timeText } from "../../report-view";
import { useCaseInquiries } from "../../use-case-inquiries";
import { CaseInquiriesSection } from "./CaseInquiries";
import {
	CaseMovesSection,
	CaseNotesSection,
	CaseReportsSection,
	CaseSummarySection,
	DerivedMeasuresSection,
} from "./CaseSections";
import ChangeCaseSeverityDialog from "./ChangeCaseSeverityDialog";
import CreateCaseInquiryDialog from "./CreateCaseInquiryDialog";
import DecideCaseDialog from "./DecideCaseDialog";
import RecordInquiryReplyDialog from "./RecordInquiryReplyDialog";
import { useCaseReports } from "./use-case-reports";
import WithdrawInquiryDialog from "./WithdrawInquiryDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

type DialogState =
	| { name: "decide" }
	| { name: "severity" }
	| { name: "inquiry" }
	| { name: "inquiryReply"; inquiry: CaseInquiry }
	| { name: "inquiryWithdraw"; inquiry: CaseInquiry }
	| { name: "measure" }
	| { name: "risk" }
	| { name: "termination" }
	| { name: "resolve"; reportId: string }
	| { name: "move"; report: Report }
	| null;

/**
 * 案件詳情：案件、案件裡的通報、移動紀錄、導出的措施與內部註記。按鈕只看伺服器的 can_*
 * （不能操作時隱藏，不顯示原因）；從案件建立的措施都記為這個案件導出的措施（source_case_id）。
 */
export default function CaseDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const now = useNow(LIST_REFRESH_MS);
	const [dialog, setDialog] = useState<DialogState>(null);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const [openingMove, setOpeningMove] = useState(false);
	const invalidate = useInvalidateGovernance();

	const detail = useQuery(SellerGovernanceAdminService.method.getCase, { id });
	const kase = detail.data?.case;
	const inquiries = useCaseInquiries(id);
	useScrollToHash(!detail.isLoading && !!kase && !inquiries.loading);
	const reports = useCaseReports(id, kase?.canCreateMeasure === true);
	const store = useStoreDetail(kase?.storeId ?? "");
	const termination = terminationAvailability(store.data);

	const transport = useTransport();
	const client = useMemo(
		() => createClient(SellerGovernanceAdminService, transport),
		[transport],
	);

	const G = SellerGovernanceAdminService.method;
	const decide = useMutation(G.decideCase, { onSuccess: invalidate });
	const changeSeverity = useMutation(G.changeCaseSeverity, {
		onSuccess: invalidate,
	});
	const createInquiry = useMutation(G.createCaseInquiry, {
		onSuccess: invalidate,
	});
	const inquiryReply = useMutation(G.recordCaseInquiryReply, {
		onSuccess: invalidate,
	});
	const withdrawInquiry = useMutation(G.withdrawCaseInquiry, {
		onSuccess: invalidate,
	});
	const resolve = useMutation(G.resolveReport, { onSuccess: invalidate });
	const move = useMutation(G.moveReport, { onSuccess: invalidate });
	const createMeasure = useMutation(G.createMeasure, { onSuccess: invalidate });

	/**
	 * 送出案件的操作。VERSION_CONFLICT（別人先改過這個案件）時先重讀再回 conflict：這裡丟出錯誤讓對話框留在原地、
	 * 保留輸入並顯示說明；其他失敗丟出伺服器的訊息。成功才關閉對話框；成功時不另外提示。
	 */
	const run = async (
		call: () => Promise<unknown>,
		fallback: string,
		reload: () => Promise<unknown> = () => detail.refetch(),
		conflictText: ConflictText = CASE_CONFLICT_TEXT,
	): Promise<void> => {
		const result = await runAction(call, { reload, fallback, conflictText });
		if (!result.ok) throw new Error(result.text);
		setDialog(null);
		setNotice(null);
	};
	/**
	 * 案件裡一筆通報的操作：版本衝突時重讀的是案件裡的通報（reload 可以多讀對話框用的那一筆），說的是通報或案件（看伺服器說哪一個 etag 過時）。
	 */
	const runReport = (
		call: () => Promise<unknown>,
		fallback: string,
		reload: () => Promise<unknown> = () => reports.refetch(),
	) => run(call, fallback, reload, reportConflictText);

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

	const status = caseStatusBadge(kase);
	const dialogReport = (reportId: string): Report | undefined =>
		reports.loaded.find((report) => report.id === reportId);

	const handleDecide = (request: Parameters<typeof decide.mutateAsync>[0]) =>
		run(() => decide.mutateAsync(request), DECIDE_FAILED_TEXT);
	const handleSeverity = (
		request: Parameters<typeof changeSeverity.mutateAsync>[0],
	) => run(() => changeSeverity.mutateAsync(request), SEVERITY_FAILED_TEXT);
	const handleCreateInquiry = (
		request: Parameters<typeof createInquiry.mutateAsync>[0],
	) => run(() => createInquiry.mutateAsync(request), INQUIRY_FAILED_TEXT);
	const handleInquiryReply = async (
		request: Parameters<typeof inquiryReply.mutateAsync>[0],
	) => {
		let alreadyRecorded = false;
		await run(async () => {
			const res = await inquiryReply.mutateAsync(request);
			alreadyRecorded = res.alreadyRecorded;
		}, INQUIRY_FAILED_TEXT);
		if (alreadyRecorded) {
			setNotice({ tone: "success", text: INQUIRY_ALREADY_REPLIED_TEXT });
		}
	};
	const handleWithdrawInquiry = async (inquiryId: string) => {
		let alreadyWithdrawn = false;
		await run(async () => {
			const res = await withdrawInquiry.mutateAsync({ id: inquiryId });
			alreadyWithdrawn = res.alreadyWithdrawn;
		}, INQUIRY_FAILED_TEXT);
		if (alreadyWithdrawn) {
			setNotice({ tone: "success", text: INQUIRY_ALREADY_WITHDRAWN_TEXT });
		}
	};
	const handleResolve = (request: Parameters<typeof resolve.mutateAsync>[0]) =>
		runReport(() => resolve.mutateAsync(request), RESOLVE_FAILED_TEXT);
	const handleMove = async (
		request: Parameters<typeof move.mutateAsync>[0],
	) => {
		let sourceCaseClosed = false;
		await runReport(
			async () => {
				const res = await move.mutateAsync(request);
				sourceCaseClosed = res.sourceCaseClosed;
			},
			MOVE_FAILED_TEXT,
			// 對話框拿的是開啟時讀的那一筆（etag 與去處的案件版本都在裡面）：衝突時連它一起重讀，重送才不會帶過時的意圖。
			() =>
				Promise.all([reports.refetch(), rereadMoveReport(request.id ?? "")]),
		);
		if (sourceCaseClosed) {
			setNotice({ tone: "success", text: SOURCE_CASE_CLOSED_TEXT });
		}
	};
	/** 建立措施的對話框自己處理失敗與重送（request_id）；成功才進來。 */
	const created = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
		text: (measure: Measure | undefined) => string,
	) => {
		const res = await createMeasure.mutateAsync(request);
		setDialog(null);
		setNotice({ tone: "success", text: text(res.measure) });
	};

	const storeFailed = store.isError && kase.canLinkMeasure;
	const targetReport =
		dialog?.name === "resolve" ? dialogReport(dialog.reportId) : undefined;

	/** 移動的去處（triage_targets）只有單筆讀取才帶：點「移到其他案件」時讀這筆通報，讀到了才開對話框。 */
	const openMove = async (row: Report) => {
		if (openingMove) return;
		setOpeningMove(true);
		try {
			const res = await client.getReport({ id: row.id });
			if (res.report) setDialog({ name: "move", report: res.report });
		} catch (caught) {
			setNotice({
				tone: "error",
				text: connectErrorText(caught, "通報讀取失敗"),
			});
		} finally {
			setOpeningMove(false);
		}
	};

	/** 版本衝突之後重讀移動對話框的那一筆通報（新的 etag 與去處）；讀不到就丟出，由 runAction 改用不宣稱已重新載入的句子。 */
	const rereadMoveReport = async (reportId: string) => {
		const res = await client.getReport({ id: reportId });
		const fresh = res.report;
		if (!fresh) throw new Error("report missing");
		setDialog((current) =>
			current?.name === "move" && current.report.id === reportId
				? { name: "move", report: fresh }
				: current,
		);
	};

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">案件詳情</h1>
							<StatusBadge variant={status.variant}>{status.label}</StatusBadge>
							{kase.awaitingSeller && (
								<StatusBadge variant="warning">等賣家</StatusBadge>
							)}
						</div>
						<div className="header-meta">
							<Link href={`/admin/merchants/${kase.storeId}`}>查看店家</Link>
							<span>
								・案件 {kase.id}・開案於 {timeText(kase.openTime)}
							</span>
						</div>
					</div>
				</div>
			</div>

			{kase.unseenSupplementReportCount > 0 && (
				<div className="adm-operation-error" role="status">
					<MaterialSymbol name="error" />
					<div>{unseenSupplementsText(kase.unseenSupplementReportCount)}</div>
				</div>
			)}
			<GovernanceNoticeBanner notice={notice} />

			<div
				className="header-actions"
				style={{ display: "flex", flexWrap: "wrap", gap: 16 }}
			>
				{kase.canDecide && (
					<Button
						variant="primary"
						onClick={() => setDialog({ name: "decide" })}
					>
						結案
					</Button>
				)}
				{kase.canCreateMeasure && (
					<Button
						variant="outline"
						loading={!reports.sourcesReady}
						onClick={() => setDialog({ name: "measure" })}
					>
						建立措施
					</Button>
				)}
				{kase.canCreateMeasure && (
					<Button variant="outline" onClick={() => setDialog({ name: "risk" })}>
						建立風險措施
					</Button>
				)}
				{kase.canLinkMeasure && termination.canCreate && (
					<Button
						variant="outline"
						onClick={() => setDialog({ name: "termination" })}
					>
						終止合作
					</Button>
				)}
				{storeFailed && (
					<div className="adm-operation-error" role="alert">
						<MaterialSymbol name="error" />
						<div>{connectErrorText(store.error, "店家資料讀取失敗")}</div>
						<Button variant="outline" onClick={() => void store.refetch()}>
							重試
						</Button>
					</div>
				)}
				{kase.canCreateInquiry && (
					<Button
						variant="outline"
						onClick={() => setDialog({ name: "inquiry" })}
					>
						詢問賣家
					</Button>
				)}
				{kase.canChangeSeverity && (
					<Button
						variant="outline"
						onClick={() => setDialog({ name: "severity" })}
					>
						改嚴重度
					</Button>
				)}
			</div>

			<CaseSummarySection kase={kase} now={now} />
			<CaseReportsSection
				kase={kase}
				view={reports.view}
				onResolve={(report) =>
					setDialog({ name: "resolve", reportId: report.id })
				}
				onMove={(report) => void openMove(report)}
			/>
			<CaseInquiriesSection
				read={inquiries}
				onReply={(inquiry) => setDialog({ name: "inquiryReply", inquiry })}
				onWithdraw={(inquiry) =>
					setDialog({ name: "inquiryWithdraw", inquiry })
				}
			/>
			<CaseMovesSection kase={kase} />
			<DerivedMeasuresSection caseId={kase.id} />
			<CaseNotesSection caseId={kase.id} />

			{dialog?.name === "decide" && (
				<DecideCaseDialog
					caseId={kase.id}
					onClose={() => setDialog(null)}
					onConfirm={handleDecide}
				/>
			)}
			{dialog?.name === "severity" && (
				<ChangeCaseSeverityDialog
					kase={kase}
					onClose={() => setDialog(null)}
					onConfirm={handleSeverity}
				/>
			)}
			{dialog?.name === "inquiry" && (
				<CreateCaseInquiryDialog
					caseId={kase.id}
					onClose={() => setDialog(null)}
					onConfirm={handleCreateInquiry}
				/>
			)}
			{dialog?.name === "inquiryReply" && (
				<RecordInquiryReplyDialog
					inquiry={dialog.inquiry}
					onClose={() => setDialog(null)}
					onConfirm={handleInquiryReply}
				/>
			)}
			{dialog?.name === "inquiryWithdraw" && (
				<WithdrawInquiryDialog
					inquiry={dialog.inquiry}
					onClose={() => setDialog(null)}
					onConfirm={handleWithdrawInquiry}
				/>
			)}
			{dialog?.name === "resolve" && targetReport && (
				<ResolveReportDialog
					report={targetReport}
					onClose={() => setDialog(null)}
					onConfirm={handleResolve}
				/>
			)}
			{dialog?.name === "move" && (
				<MoveReportDialog
					report={dialog.report}
					onClose={() => setDialog(null)}
					onConfirm={handleMove}
				/>
			)}
			{dialog?.name === "measure" && (
				<CreateMeasureDialog
					storeId={kase.storeId}
					sourceCaseId={kase.id}
					initial={{
						category: kase.category,
						sourceReportIds: reports.sourceReportIds,
					}}
					sourceReportsNote={
						reports.sourceOverflow ? SOURCE_OVERFLOW_TEXT : undefined
					}
					onClose={() => setDialog(null)}
					onConfirm={(request) =>
						created(request, () => VIOLATION_MEASURE_CREATED_TEXT)
					}
				/>
			)}
			{dialog?.name === "risk" && (
				<CreateRiskMeasureDialog
					storeId={kase.storeId}
					sourceCaseId={kase.id}
					onClose={() => setDialog(null)}
					onConfirm={(request) => created(request, () => MEASURE_CREATED_TEXT)}
				/>
			)}
			{dialog?.name === "termination" && (
				<CreateTerminationDialog
					storeId={kase.storeId}
					accountId={store.data?.store?.accountId ?? ""}
					sourceCaseId={kase.id}
					options={termination.options}
					onClose={() => setDialog(null)}
					onConfirm={(request) => created(request, terminationCreatedText)}
				/>
			)}
		</div>
	);
}
