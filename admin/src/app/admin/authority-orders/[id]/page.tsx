"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import {
	AuthorityOrderState,
	AuthorityTargetNoticeKind,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/authority_order_pb";
import {
	type CreateMeasureRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import type { Measure } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { Field, Section } from "@/app/admin/governance/DetailBits";
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
import { useInvalidateGovernance } from "@/app/admin/violation-reports/governance-queries";
import {
	calendarDateText,
	derivedMeasureText,
	timeText,
} from "@/app/admin/violation-reports/report-view";
import { useAllMeasures } from "@/app/admin/violation-reports/use-all-measures";
import { Button, DeadlineText, StatusBadge } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import { deadlineView } from "@/lib/deadline";
import { connectErrorText } from "@/lib/error-text";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import { useScrollToHash } from "@/lib/use-scroll-to-hash";
import {
	AUTHORITY_CONFLICT_TEXT,
	AUTHORITY_FAILED_TEXT,
} from "../authority-actions";
import {
	AUTHORITY_REVOKE_TEXT,
	authorityCoverageText,
	authorityKindText,
	authorityNoticeKindText,
	authorityStateText,
	holdExtensionText,
	holdUntilText,
	measureStoreOptions,
	NO_SELLER_DATA_TEXT,
	revokedText,
	targetLabel,
	WITHHELD_FOLLOW_UP_TEXT,
} from "../authority-text";
import CompleteAuthorityOrderDialog from "../CompleteAuthorityOrderDialog";
import ExtendHoldDialog from "../ExtendHoldDialog";
import RecordTargetNoticeDialog from "../RecordTargetNoticeDialog";
import RevokeAuthorityOrderDialog from "../RevokeAuthorityOrderDialog";
import "@/styles/pages/MerchantCertificationDetail.css";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

type DialogState =
	| { name: "complete" }
	| { name: "extend" }
	| { name: "notice"; position: number }
	| { name: "revoke" }
	| { name: "measure" }
	| { name: "risk" }
	| { name: "termination" }
	| null;

const NONE = "-";

/**
 * 來函詳情：來函、指名的對象（逐家的涵蓋狀態）、處理（完成與每個對象的告知）、保存（延長歷史）與導出的措施。
 * 按鈕只看伺服器的 can_*（來函的 can_complete／can_extend_hold／can_revoke、對象的 can_record_notice、店家的 can_create_measure／
 * can_create_termination），畫面不重算可用性；建立措施的三個入口帶 source_authority_order_id（違規依據另帶新認定的來源來函）。
 * 入口對應的店家有多家時不預設，先選再開（選項只列該動作的旗標為真的店家）。
 * 來函沒有內部註記。成功不另外提示；版本衝突重讀並顯示衝突句、對話框保留輸入。
 */
export default function AuthorityOrderDetailPage() {
	const router = useRouter();
	const params = useParams<{ id: string }>();
	const id = params.id;
	const [dialog, setDialog] = useState<DialogState>(null);
	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const [pickedStore, setPickedStore] = useState("");
	const invalidate = useInvalidateGovernance();
	const now = useNow(LIST_REFRESH_MS);

	const detail = useQuery(
		SellerGovernanceAdminService.method.getAuthorityOrder,
		{
			id,
		},
	);
	const order = detail.data?.order;
	useScrollToHash(!detail.isLoading && !!order);
	const derived = useAllMeasures({ sourceAuthorityOrderId: id });

	// 建立措施的店家：只列伺服器旗標為真的店（措施：can_create_measure；終止合作：can_create_termination）。
	// 只有一家可選就直接用；多家時不預設，先選（L20）。
	const measureOptions = order
		? measureStoreOptions(order, "canCreateMeasure")
		: [];
	const terminationOptions = order
		? measureStoreOptions(order, "canCreateTermination")
		: [];
	const allOptions = [
		...measureOptions,
		...terminationOptions.filter(
			(t) => !measureOptions.some((m) => m.storeId === t.storeId),
		),
	];
	const storeId =
		allOptions.length === 1
			? allOptions[0].storeId
			: (allOptions.find((o) => o.storeId === pickedStore)?.storeId ?? "");
	const measureReady = measureOptions.some((o) => o.storeId === storeId);
	const terminationReady = terminationOptions.some(
		(o) => o.storeId === storeId,
	);
	// 終止合作的事由與生效時機讀選定店家的 GetStore。
	const store = useStoreDetail(terminationReady ? storeId : "");
	const termination = terminationAvailability(store.data);

	const G = SellerGovernanceAdminService.method;
	const complete = useMutation(G.completeAuthorityOrder, {
		onSuccess: invalidate,
	});
	const extend = useMutation(G.extendAuthorityOrderHold, {
		onSuccess: invalidate,
	});
	const recordNotice = useMutation(G.recordAuthorityTargetNotice, {
		onSuccess: invalidate,
	});
	const revoke = useMutation(G.revokeAuthorityOrder, { onSuccess: invalidate });
	const createMeasure = useMutation(G.createMeasure, { onSuccess: invalidate });

	/** 送出來函的操作。版本衝突時先重讀再丟出衝突句（對話框留在原地、保留輸入）；其他失敗丟出伺服器的句子；成功才關閉對話框。 */
	const run = async (call: () => Promise<unknown>): Promise<void> => {
		const result = await runAction(call, {
			reload: () => detail.refetch(),
			fallback: AUTHORITY_FAILED_TEXT,
			conflictText: AUTHORITY_CONFLICT_TEXT,
		});
		if (!result.ok) throw new Error(result.text);
		setDialog(null);
		setNotice(null);
	};

	const handleRevoke = async (
		request: Parameters<typeof revoke.mutateAsync>[0],
	) => {
		let alreadyRevoked = false;
		await run(async () => {
			const res = await revoke.mutateAsync(request);
			alreadyRevoked = res.alreadyRevoked;
		});
		if (alreadyRevoked) {
			setNotice({ tone: "success", text: AUTHORITY_REVOKE_TEXT.already });
		}
	};

	const created = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
		text: (measure: Measure | undefined) => string,
	) => {
		const res = await createMeasure.mutateAsync(request);
		setDialog(null);
		setNotice({ tone: "success", text: text(res.measure) });
	};

	const back = (
		<button
			type="button"
			className="btn-back"
			aria-label="返回列表"
			onClick={() => router.push("/admin/authority-orders")}
		>
			<MaterialSymbol name="keyboard_arrow_left" />
		</button>
	);

	if (detail.isLoading) {
		return (
			<div className="certification-detail-page">
				<p role="status">載入中...</p>
			</div>
		);
	}
	if (detail.error || !order) {
		return (
			<div className="certification-detail-page">
				<div className="detail-header">
					<div className="header-left">{back}</div>
				</div>
				<div className="adm-operation-error" role="alert">
					<MaterialSymbol name="error" />
					<div>{connectErrorText(detail.error, "來函讀取失敗")}</div>
				</div>
				<Button variant="outline" onClick={() => void detail.refetch()}>
					重試
				</Button>
			</div>
		);
	}

	const deadline = deadlineView(order.deadline, now);
	const open = order.state === AuthorityOrderState.OPEN;
	const completed = order.state === AuthorityOrderState.COMPLETED;
	const revoked = order.state === AuthorityOrderState.REVOKED;
	const sourceOrder = { id: order.id, documentNo: order.documentNo };
	const storeFailed = terminationReady && store.isError;

	return (
		<div className="certification-detail-page">
			<div className="detail-header">
				<div className="header-left">
					{back}
					<div className="header-info">
						<div className="header-title-row">
							<h1 className="merchant-name" title={order.documentNo}>
								來函 {order.documentNo}
							</h1>
							<StatusBadge variant={open ? "warning" : "gray"}>
								{authorityStateText(order.state)}
							</StatusBadge>
						</div>
						<div className="header-meta">
							<span>
								{order.agency}・收文 {timeText(order.receiveTime)}
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
				{order.canComplete && (
					<Button
						variant="primary"
						onClick={() => setDialog({ name: "complete" })}
					>
						完成
					</Button>
				)}
				{order.canExtendHold && (
					<Button
						variant="outline"
						onClick={() => setDialog({ name: "extend" })}
					>
						延長保存
					</Button>
				)}
				{order.canRevoke && (
					<Button
						variant="outline"
						onClick={() => setDialog({ name: "revoke" })}
					>
						{AUTHORITY_REVOKE_TEXT.action}
					</Button>
				)}
				{allOptions.length > 1 && (
					<label
						htmlFor="authority-measure-store"
						style={{
							display: "flex",
							alignItems: "center",
							gap: 8,
							whiteSpace: "nowrap",
						}}
					>
						先選帳號
						<select
							id="authority-measure-store"
							className="form-select"
							value={storeId}
							onChange={(e) => setPickedStore(e.target.value)}
						>
							<option value="">請選擇</option>
							{allOptions.map((option) => (
								<option key={option.storeId} value={option.storeId}>
									{option.label}
								</option>
							))}
						</select>
					</label>
				)}
				{measureOptions.length > 0 && (
					<>
						<Button
							variant="outline"
							disabled={!measureReady}
							onClick={() => setDialog({ name: "measure" })}
						>
							建立措施
						</Button>
						<Button
							variant="outline"
							disabled={!measureReady}
							onClick={() => setDialog({ name: "risk" })}
						>
							建立風險措施
						</Button>
					</>
				)}
				{terminationOptions.length > 0 && (
					<Button
						variant="outline"
						disabled={!terminationReady || !termination.canCreate}
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
			</div>

			<Section id="authority-order-title" icon="account_balance" title="來函">
				<div className="info-grid">
					<Field label="機關">{order.agency}</Field>
					<Field label="文號">{order.documentNo}</Field>
					<Field label="收文時間">{timeText(order.receiveTime)}</Field>
					<Field label="種類">{authorityKindText(order.kind)}</Field>
					<Field label="期限">
						<DeadlineText display={deadline} title={deadline.dueText} />
					</Field>
					<Field label="內容摘要" fullWidth>
						<span style={{ whiteSpace: "pre-wrap" }}>{order.summary}</span>
					</Field>
				</div>
			</Section>

			<Section
				id="authority-targets-title"
				icon="storefront"
				title="指名的對象"
			>
				<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
					{order.targets.map((target, index) => (
						<li
							key={index}
							id={`target-${index}`}
							style={{
								borderTop:
									index === 0 ? undefined : "1px solid var(--admin-border)",
								padding: "12px 0",
							}}
						>
							<div>
								<strong>帳號 {target.accountId}</strong>
								{target.productIds.length > 0 && (
									<span>・商品 {target.productIds.join("、")}</span>
								)}
							</div>
							{target.noSellerData ? (
								<p>{NO_SELLER_DATA_TEXT}</p>
							) : (
								<ul
									style={{ listStyle: "none", padding: 0, margin: "4px 0 0" }}
								>
									{target.coveredStores.map((covered) => (
										<li key={covered.storeId}>
											<Link href={`/admin/merchants/${covered.storeId}`}>
												{covered.storeName || covered.storeId}
											</Link>
											・{authorityCoverageText(covered.coverage)}
										</li>
									))}
								</ul>
							)}
						</li>
					))}
				</ul>
			</Section>

			<Section id="authority-handling-title" icon="account_tree" title="處理">
				<div className="info-grid">
					<Field label="狀態">
						<StatusBadge variant={open ? "warning" : "gray"}>
							{authorityStateText(order.state)}
						</StatusBadge>
					</Field>
					{!revoked && (
						<Field label="完成">
							{completed
								? `${order.completedByAdminId || NONE}・${timeText(order.completeTime)}`
								: NONE}
						</Field>
					)}
					{completed && (
						<Field label="處理摘要" fullWidth>
							<span style={{ whiteSpace: "pre-wrap" }}>
								{order.resolutionSummary}
							</span>
						</Field>
					)}
					{revoked && (
						<Field label="撤銷登記" fullWidth>
							<span style={{ whiteSpace: "pre-wrap" }}>
								{revokedText(timeText(order.revokeTime), order.revokeReason)}
							</span>
						</Field>
					)}
				</div>
				{completed && (
					<ul style={{ listStyle: "none", padding: 0, margin: "12px 0 0" }}>
						{order.targets.map((target, index) => {
							const n = target.notice;
							if (!n) return null;
							const withheld = n.kind === AuthorityTargetNoticeKind.WITHHELD;
							return (
								<li key={index} style={{ padding: "8px 0" }}>
									<div>
										{targetLabel(target)}・告知被指名者：
										{authorityNoticeKindText(n.kind)}
										{n.kind === AuthorityTargetNoticeKind.NOTIFIED &&
											`（${timeText(n.notifyTime)}${n.note ? `・${n.note}` : ""}）`}
										{withheld && `（${calendarDateText(n.recheckDate)}）`}
									</div>
									{withheld && (
										<p className="adm-operation-field-note">
											{WITHHELD_FOLLOW_UP_TEXT}
										</p>
									)}
									{target.canRecordNotice && (
										<Button
											variant="outline"
											onClick={() =>
												setDialog({ name: "notice", position: index })
											}
										>
											記下告知
										</Button>
									)}
								</li>
							);
						})}
					</ul>
				)}
			</Section>

			<Section id="authority-hold-title" icon="archive" title="保存">
				<p>
					{holdUntilText(
						timeText(order.holdUntil),
						order.holdActive,
						order.state === AuthorityOrderState.REVOKED,
					)}
				</p>
				{order.holdExtensions.length > 0 && (
					<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{order.holdExtensions.map((extension) => (
							<li
								key={`${extension.createTime?.seconds}-${extension.toTime?.seconds}`}
							>
								{holdExtensionText({
									createTime: timeText(extension.createTime),
									adminId: extension.adminId || NONE,
									fromTime: timeText(extension.fromTime),
									toTime: timeText(extension.toTime),
									cause: extension.cause,
									reason: extension.reason,
								})}
							</li>
						))}
					</ul>
				)}
			</Section>

			<Section id="authority-measures-title" icon="balance" title="導出的措施">
				{derived.loading ? (
					<p role="status">載入中...</p>
				) : derived.failed ? (
					<div className="tab-empty-state" role="alert">
						<MaterialSymbol name="error" />
						<span>{connectErrorText(derived.error, "措施讀取失敗")}</span>
						<Button variant="outline" onClick={() => derived.retry()}>
							重試
						</Button>
					</div>
				) : derived.measures.length === 0 ? (
					<p>還沒有導出的措施</p>
				) : (
					<ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
						{derived.measures.map((measure) => (
							<li key={measure.id}>
								<Link href={`/admin/measures/${measure.id}`}>
									{derivedMeasureText(measure)}
								</Link>
							</li>
						))}
					</ul>
				)}
			</Section>

			{dialog?.name === "complete" && (
				<CompleteAuthorityOrderDialog
					order={order}
					onClose={() => setDialog(null)}
					onConfirm={(request) => run(() => complete.mutateAsync(request))}
				/>
			)}
			{dialog?.name === "extend" && (
				<ExtendHoldDialog
					order={order}
					onClose={() => setDialog(null)}
					onConfirm={(request) => run(() => extend.mutateAsync(request))}
				/>
			)}
			{dialog?.name === "notice" && (
				<RecordTargetNoticeDialog
					order={order}
					position={dialog.position}
					onClose={() => setDialog(null)}
					onConfirm={(request) => run(() => recordNotice.mutateAsync(request))}
				/>
			)}
			{dialog?.name === "revoke" && (
				<RevokeAuthorityOrderDialog
					order={order}
					onClose={() => setDialog(null)}
					onConfirm={handleRevoke}
				/>
			)}
			{dialog?.name === "measure" && (
				<CreateMeasureDialog
					storeId={storeId}
					sourceAuthorityOrder={sourceOrder}
					initial={{ sourceAuthorityOrderIds: [order.id] }}
					onClose={() => setDialog(null)}
					onConfirm={(request) =>
						created(request, () => VIOLATION_MEASURE_CREATED_TEXT)
					}
				/>
			)}
			{dialog?.name === "risk" && (
				<CreateRiskMeasureDialog
					storeId={storeId}
					sourceAuthorityOrder={sourceOrder}
					onClose={() => setDialog(null)}
					onConfirm={(request) => created(request, () => MEASURE_CREATED_TEXT)}
				/>
			)}
			{dialog?.name === "termination" && (
				<CreateTerminationDialog
					storeId={storeId}
					accountId={store.data?.store?.accountId ?? ""}
					sourceAuthorityOrder={sourceOrder}
					options={termination.options}
					onClose={() => setDialog(null)}
					onConfirm={(request) => created(request, terminationCreatedText)}
				/>
			)}
		</div>
	);
}
