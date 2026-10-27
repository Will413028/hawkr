"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import {
	createConnectQueryKey,
	useMutation,
	useQuery,
} from "@connectrpc/connect-query";
import {
	type CreateMeasureRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { LinkedAccount } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Button, StatusBadge } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { connectErrorText } from "@/lib/error-text";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import CreateTerminationDialog from "../../merchants/[id]/CreateTerminationDialog";
import EndMeasureDialog from "../../merchants/[id]/EndMeasureDialog";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
} from "../../merchants/[id]/GovernanceNotice";
import {
	MEASURE_CONFLICT_TEXT,
	measureStateBadge,
	terminationCreatedText,
} from "../../merchants/[id]/measure";
import {
	actionBar,
	buildDecideAppeal,
	buildDecideRemediation,
	buildRevoke,
	buildScheduleReview,
	buildUpdateMeasure,
} from "../measure-actions";
import DecideMeasureRequestDialog from "./DecideMeasureRequestDialog";
import LinkedAccountsSection from "./LinkedAccountsSection";
import MeasureNotesSection from "./MeasureNotesSection";
import {
	CitedFindingsSection,
	MeasureContentSection,
	PendingRequestSection,
	RevisionsSection,
} from "./MeasureSections";
import RevokeMeasureDialog from "./RevokeMeasureDialog";
import ScheduleReviewDialog from "./ScheduleReviewDialog";
import UpdateMeasureDialog from "./UpdateMeasureDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

type DialogName =
	| "update"
	| "end"
	| "revoke"
	| "schedule"
	| "decide-appeal"
	| "decide-remediation";

const actionRowStyle = {
	display: "inline-flex",
	flexWrap: "wrap",
	alignItems: "center",
	gap: 8,
} as const;

/** 措施詳情：待處理的請求、措施內容、依據的認定、版本紀錄、相同身分的其他帳號、內部註記，以及依伺服器 can_* 顯示的操作列。 */
export default function MeasureDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const queryClient = useQueryClient();
	const now = useNow(LIST_REFRESH_MS);
	const [dialog, setDialog] = useState<DialogName | null>(null);
	const [linkedTarget, setLinkedTarget] = useState<{
		sourceMeasureId: string;
		account: LinkedAccount;
	} | null>(null);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);

	const detail = useQuery(SellerGovernanceAdminService.method.getMeasure, {
		id,
	});
	const measure = detail.data?.measure;

	// 寫入成功後措施（詳情、清單、註記）與店家（active_measures）整組失效。
	const invalidate = () => {
		const keys = [
			createConnectQueryKey({
				schema: SellerGovernanceAdminService.method.getMeasure,
				cardinality: undefined,
			}),
			createConnectQueryKey({
				schema: SellerGovernanceAdminService.method.listMeasures,
				cardinality: undefined,
			}),
			createConnectQueryKey({
				schema: SellerGovernanceAdminService.method.listMeasureNotes,
				cardinality: undefined,
			}),
			createConnectQueryKey({
				schema: SellerAdminService.method.getStore,
				cardinality: undefined,
			}),
		];
		for (const queryKey of keys) queryClient.invalidateQueries({ queryKey });
	};
	const G = SellerGovernanceAdminService.method;
	const updateMeasure = useMutation(G.updateMeasure, { onSuccess: invalidate });
	const endMeasure = useMutation(G.endMeasure, { onSuccess: invalidate });
	const revokeMeasure = useMutation(G.revokeMeasure, { onSuccess: invalidate });
	const scheduleReview = useMutation(G.scheduleMeasureReview, {
		onSuccess: invalidate,
	});
	const decideAppeal = useMutation(G.decideAppeal, { onSuccess: invalidate });
	const decideRemediation = useMutation(G.decideRemediation, {
		onSuccess: invalidate,
	});
	const createMeasure = useMutation(G.createMeasure, { onSuccess: invalidate });

	/**
	 * 送出措施的操作。VERSION_CONFLICT（別人先改過這個措施）時 runAction 先重讀措施再回 conflict：這裡丟出錯誤讓對話框
	 * 留在原地、保留輸入並顯示說明，之後再送帶的是重讀後的 etag；其他失敗丟出伺服器的訊息。成功才關閉對話框。
	 */
	const run = async (
		call: () => Promise<unknown>,
		fallback: string,
	): Promise<void> => {
		const result = await runAction(call, {
			reload: () => detail.refetch(),
			fallback,
			conflictText: MEASURE_CONFLICT_TEXT,
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
			onClick={() => router.push("/admin/measures")}
		>
			<MaterialSymbol name="keyboard_arrow_left" />
		</button>
	);

	if (detail.isLoading) {
		return (
			<div className="certification-detail-page">
				<p role="status">載入措施中...</p>
			</div>
		);
	}
	if (detail.error || !measure) {
		return (
			<div className="certification-detail-page">
				<div className="detail-header">
					<div className="header-left">{back}</div>
				</div>
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{connectErrorText(detail.error, "措施讀取失敗")}</div>
				</div>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const state = measureStateBadge(measure.state, measure.kind);
	const bar = actionBar(measure);

	const handleEnd = async () => {
		const result = await runAction(
			() => endMeasure.mutateAsync({ id: measure.id, etag: measure.etag }),
			{
				reload: () => detail.refetch(),
				fallback: "結束措施失敗，賣家仍在停權中；請確認網路狀態後再試一次。",
				conflictText: MEASURE_CONFLICT_TEXT,
			},
		);
		if (!result.ok) {
			if (result.kind === "conflict") {
				setDialog(null);
				setNotice({ tone: "error", text: result.text });
				return;
			}
			throw new Error(result.text);
		}
		setDialog(null);
		setNotice(null);
	};

	const handleCreateLinked = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		const res = await createMeasure.mutateAsync(request);
		setLinkedTarget(null);
		setNotice({ tone: "success", text: terminationCreatedText(res.measure) });
	};

	const reasonList = (reasons: string[], id: string) =>
		reasons.length > 0 && (
			<ul
				id={id}
				className="adm-operation-field-note"
				style={{ margin: 0, paddingLeft: 16 }}
			>
				{reasons.map((text) => (
					<li key={text}>{text}</li>
				))}
			</ul>
		);

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name">措施詳情</h1>
							<StatusBadge variant={state.variant}>{state.label}</StatusBadge>
						</div>
						<div className="header-meta">
							<span>{measure.storeName || "-"}</span>
							<span className="meta-separator">·</span>
							<Link href={`/admin/merchants/${measure.storeId}`}>查看店家</Link>
							<span className="meta-separator">·</span>
							<span>措施 {measure.id}</span>
						</div>
					</div>
				</div>
			</div>

			<GovernanceNoticeBanner notice={notice} />

			<div
				className="header-actions"
				style={{ display: "flex", flexWrap: "wrap", gap: 16 }}
			>
				{bar.decideAppeal.visible && (
					<Button variant="primary" onClick={() => setDialog("decide-appeal")}>
						裁定申覆
					</Button>
				)}
				{bar.decideRemediation.visible && (
					<Button
						variant="primary"
						onClick={() => setDialog("decide-remediation")}
					>
						裁定解除請求
					</Button>
				)}
				<div style={actionRowStyle}>
					<Button
						variant="outline"
						disabled={!bar.edit.enabled}
						aria-describedby={
							bar.edit.reasons.length > 0 ? "measure-edit-reasons" : undefined
						}
						onClick={() => setDialog("update")}
					>
						修改
					</Button>
					{reasonList(bar.edit.reasons, "measure-edit-reasons")}
				</div>
				{bar.end.visible && (
					<Button variant="outline" onClick={() => setDialog("end")}>
						結束
					</Button>
				)}
				<div style={actionRowStyle}>
					<Button
						variant="danger"
						disabled={!bar.revoke.enabled}
						aria-describedby={
							bar.revoke.reasons.length > 0
								? "measure-revoke-reasons"
								: undefined
						}
						onClick={() => setDialog("revoke")}
					>
						撤銷
					</Button>
					{reasonList(bar.revoke.reasons, "measure-revoke-reasons")}
				</div>
				{bar.scheduleReview.visible && (
					<Button variant="outline" onClick={() => setDialog("schedule")}>
						延後複審
					</Button>
				)}
			</div>

			<PendingRequestSection measure={measure} now={now} />
			<MeasureContentSection measure={measure} />
			<CitedFindingsSection measure={measure} />
			<RevisionsSection measure={measure} />
			<LinkedAccountsSection
				measure={measure}
				onCreateLinked={(sourceMeasureId, account) =>
					setLinkedTarget({ sourceMeasureId, account })
				}
			/>
			<MeasureNotesSection measureId={measure.id} />

			{dialog === "update" && (
				<UpdateMeasureDialog
					measure={measure}
					onClose={() => setDialog(null)}
					onConfirm={(form, baseline) =>
						run(
							() =>
								updateMeasure.mutateAsync(
									buildUpdateMeasure(measure, form, baseline),
								),
							"修改失敗，措施沒有變動；請確認網路狀態後再試一次。",
						)
					}
				/>
			)}
			{dialog === "end" && (
				<EndMeasureDialog
					measure={measure}
					onClose={() => setDialog(null)}
					onConfirm={handleEnd}
				/>
			)}
			{dialog === "revoke" && (
				<RevokeMeasureDialog
					measure={measure}
					onClose={() => setDialog(null)}
					onConfirm={(reason, revokeFindings) =>
						run(
							() =>
								revokeMeasure.mutateAsync(
									buildRevoke(measure, reason, revokeFindings),
								),
							"撤銷失敗，措施沒有變動；請確認網路狀態後再試一次。",
						)
					}
				/>
			)}
			{dialog === "schedule" && (
				<ScheduleReviewDialog
					onClose={() => setDialog(null)}
					onConfirm={(form) =>
						run(
							() =>
								scheduleReview.mutateAsync(buildScheduleReview(measure, form)),
							"延後失敗，措施沒有變動；請確認網路狀態後再試一次。",
						)
					}
				/>
			)}
			{dialog === "decide-appeal" && (
				<DecideMeasureRequestDialog
					kind="appeal"
					measure={measure}
					onClose={() => setDialog(null)}
					onConfirm={(form) =>
						run(
							() => decideAppeal.mutateAsync(buildDecideAppeal(measure, form)),
							"裁定失敗，措施沒有變動；請確認網路狀態後再試一次。",
						)
					}
				/>
			)}
			{dialog === "decide-remediation" && (
				<DecideMeasureRequestDialog
					kind="remediation"
					measure={measure}
					onClose={() => setDialog(null)}
					onConfirm={(form) =>
						run(
							() =>
								decideRemediation.mutateAsync(
									buildDecideRemediation(measure, form),
								),
							"裁定失敗，措施沒有變動；請確認網路狀態後再試一次。",
						)
					}
				/>
			)}
			{linkedTarget && (
				<CreateTerminationDialog
					storeId={linkedTarget.account.storeId}
					accountId={linkedTarget.account.accountId}
					sourceMeasureId={linkedTarget.sourceMeasureId}
					options={
						linkedTarget.account.terminationOption
							? [linkedTarget.account.terminationOption]
							: []
					}
					onClose={() => setLinkedTarget(null)}
					onConfirm={handleCreateLinked}
				/>
			)}
		</div>
	);
}
