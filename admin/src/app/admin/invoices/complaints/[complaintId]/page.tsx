"use client";

import type { MessageInitShape } from "@bufbuild/protobuf";
import { useMutation as useConnectMutation } from "@connectrpc/connect-query";
import {
	type CreateMeasureRequestSchema,
	SellerGovernanceAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import type React from "react";
import { useState } from "react";
import CreateMeasureDialog from "@/app/admin/merchants/[id]/CreateMeasureDialog";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { VIOLATION_MEASURE_CREATED_TEXT } from "@/app/admin/merchants/[id]/store-detail";
import {
	type CreateMeasureInitial,
	measureInitialFromSuggestion,
} from "@/app/admin/merchants/[id]/violation-measure";
import { Button, SlaCountdown, StatusBadge } from "@/components/UI";
import {
	ACTION_CONFLICT_CODES,
	actionConflictText,
	COMPLAINT_ACTION_CONSEQUENCE,
	COMPLAINT_ACTION_DANGER,
	COMPLAINT_ACTION_DECIDE_VIOLATION,
	COMPLAINT_ACTION_LABEL,
	COMPLAINT_ACTION_NEEDS_CONFIRM,
	COMPLAINT_ACTION_ORDER,
	COMPLAINT_NOT_FOUND_CODE,
	COMPLAINTS_ENDPOINT,
	complaintActionAvailable,
	createInvoiceAdminIdempotencyKey,
} from "@/features/invoices/admin-actions";
import { COMPLAINT_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoiceActionDialog from "@/features/invoices/components/InvoiceActionDialog";
import InvoiceResultBanner, {
	type InvoiceBanner,
} from "@/features/invoices/components/InvoiceResultBanner";
import InvoiceTimeline from "@/features/invoices/components/InvoiceTimeline";
import type {
	ActOnInvoiceComplaintForAdminData,
	InvoiceComplaintInfo,
	InvoiceEnvelope,
	SuspensionRecommendation,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_COMPLAINT_STATUS_LABEL,
	INVOICE_DOCUMENT_KIND_LABEL,
} from "@/lib/codemeta";
import { formatDateTime } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
// 對話框骨架（overlay、面板、標題列）在共用 Dialog 自己的 Dialog.css；這裡的 DelegatedOperations.css 只放
// `.adm-operation-field*` 等欄位與版面 class（58A 陷阱 C07：少 import 不會報錯也不會轉紅）。
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A08｜申訴詳情（58A §A08，端點 59 `GET /api/v1/admin/invoices/complaints/{complaint_id}`）。
//
// ## 五個決策動作已接上（2026-09-02）
//
// 端點 60 的真實路徑是 `POST /admin/invoices/complaints/{complaint_id}/actions`
// （gateway `AdminInvoiceComplaintActions`），RPC 在
// `services/invoice-service/internal/grpc/complaint.go` 的
// `ActOnInvoiceComplaintForAdmin`。舊版這裡寫「沒有實作」，那個結論來自拿猜的路徑名
// （`.../decisions`）去打 dev 拿到 404 —— **路徑名猜錯不等於後端沒做**。
//
// 五個 action（`REMIND_SELLER` / `REQUEST_REWORK` / `DECIDE_RESOLVED` /
// `DECIDE_VIOLATION` / `RECOMMEND_SUSPENSION`）的值域與 allowed-from 住在
// `features/invoices/admin-actions.ts`，逐列對應 `admin_complaint.go` 的常數與
// `invoice_complaints.sql` 的 CAS 述詞。
//
// ## 停權路線：本頁不呼叫發票模組或舊的停權端點，措施只經建立措施對話框的 CreateMeasure 建立
//
// 58A 的「停權三步」第一步是送出 `DECIDE_VIOLATION` 或 `RECOMMEND_SUSPENSION` 拿回應
// 裡的 `recommendation`（90 天違規件數 ＋ 是否達門檻 ＋ 建議天數），第二步才在本頁開
// 「建立措施」對話框。**發票模組只提建議、不執行處分**（`admin_complaint.go` 的 ③
// 「沒有第三件」：不蓋 suspended_at、不發停權事件、不排恢復 job）——所以建議卡上那顆
// 鍵只是開對話框，真正的措施要在對話框填完並送出，由賣家治理模組
// （`SellerGovernanceAdminService.CreateMeasure`）建立，期間與自動解除也由它處理。
//
// ## 與 58A 的落差（兩個 Section 畫不出來）
//
// 端點 59 的回應（Gateway `InvoiceComplaintInfo`，與列表同一型別）沒有：
//   - `invoiceSnapshot`　　　→「發票現況」Section 無資料來源
//   - `notificationHistory[]`→「通知寄送歷程」Section 無資料來源
//   - `violationCountIn90Days`→ 右欄「賣家風險」無資料來源
// 第二項最要緊：58A 稱它是「A08 能不能裁決的關鍵」——申訴內容是「我沒收到發票」，而
// 「有沒有開」與「有沒有寄到」是兩件事。沒有它，ADM 判不出是賣家沒開還是信被退回。
// ⇒ 本頁明寫這件事，不畫一張空表格假裝有資料。
//
// ## 仍然照 58A 做的事
//
//   - **副標必寫「開啟本頁會記錄一筆存取紀錄」**：端點 59 會寫 `INVOICE_PII_REVEAL`
//     稽核並有異常尖峰告警。讓人知道自己被記錄，是這套稽核能發揮嚇阻作用的前提。
//     （對照 A06：那一頁的主資料端點不落這筆稽核，所以那裡**不能**寫這句話。）
//   - **往來紀錄追加不覆蓋**，UI 不提供編輯任何一筆。
//
// ## 建立措施鍵的兩個落點
//
// 58A 把建立處分的入口掛在 `RECOMMEND_SUSPENSION` 的回應之後——現在那個掛點
// 走得到了，所以**建議卡底下有一顆**：開啟時帶入剛拿到的建議（種類＝停權、結束時間＝
// 現在加建議天數、理由＝建議理由的原文，`measureInitialFromSuggestion`），類別不預填，
// 由管理員選，每一欄都可以改。右欄那顆常駐鍵仍然保留、不帶預填：沒有它的話，一個還沒
// 送出任何動作的管理員從這一頁到處分之間沒有任何路徑。
// 兩顆都只是開對話框，不寫入任何東西；寫入發生在對話框送出的 `CreateMeasure`。
function Section({
	title,
	hint,
	children,
}: {
	title: string;
	hint?: string;
	children: React.ReactNode;
}) {
	return (
		<section className="invoice-section">
			<h2 className="invoice-section__title">{title}</h2>
			{hint ? <p className="invoice-section__hint">{hint}</p> : null}
			{children}
		</section>
	);
}

function Field({
	label,
	note,
	full,
	children,
}: {
	label: string;
	note?: string;
	full?: boolean;
	children: React.ReactNode;
}) {
	return (
		<div className={full ? "invoice-field--full" : undefined}>
			<div className="invoice-field__label">{label}</div>
			<div className="invoice-field__value">{children}</div>
			{note ? <div className="invoice-field__note">{note}</div> : null}
		</div>
	);
}

export default function InvoiceComplaintDetailPage() {
	const router = useRouter();
	const params = useParams<{ complaintId: string }>();
	const complaintId = String(params?.complaintId ?? "");

	const [openAction, setOpenAction] = useState<string | null>(null);
	// 「建立措施」對話框要建立在哪家店、開啟時帶什麼初始值（只有「依建議建立措施」帶）；null 是沒開。
	const [measureTarget, setMeasureTarget] = useState<{
		storeId: string;
		initial?: CreateMeasureInitial;
	} | null>(null);
	const createMeasureMutation = useConnectMutation(
		SellerGovernanceAdminService.method.createMeasure,
	);
	const handleCreateMeasure = async (
		request: MessageInitShape<typeof CreateMeasureRequestSchema>,
	) => {
		await createMeasureMutation.mutateAsync(request);
		setMeasureTarget(null);
		setBanner({ tone: "success", text: VIOLATION_MEASURE_CREATED_TEXT });
	};
	const [banner, setBanner] = useState<InvoiceBanner | null>(null);
	/**
	 * 上一次動作回來的停權建議。
	 *
	 * ⚠️ `recommendation` 在 proto 上是 message 不是 optional ⇒ JSON 上永遠是物件；
	 * 「這次沒有建議」的判準是 `storeId` 為空字串（只有兩個 action 會真的算它），
	 * 不是 `=== null`——判成 null 會讓建議卡在每一次動作之後都出現，而且全是 0。
	 */
	const [recommendation, setRecommendation] =
		useState<SuspensionRecommendation | null>(null);

	const [idempotencyKey, setIdempotencyKey] = useState(
		createInvoiceAdminIdempotencyKey,
	);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "complaint", complaintId],
		enabled: complaintId !== "",
		queryFn: async () => {
			const res = await request<InvoiceEnvelope<InvoiceComplaintInfo>>(
				`${COMPLAINTS_ENDPOINT}/${encodeURIComponent(complaintId)}`,
				{ method: "GET" },
			);
			return res?.data ?? null;
		},
	});

	const notFound =
		isError &&
		error instanceof ApiClientError &&
		error.code === COMPLAINT_NOT_FOUND_CODE;

	const complaint = data ?? null;
	const status = complaint?.status ?? "";
	/** 樂觀鎖：取自這一次載入讀到的那筆。 */
	const expectedVersion = complaint?.version ?? "";
	const availableActions = COMPLAINT_ACTION_ORDER.filter((action) =>
		complaintActionAvailable(action, status),
	);

	const routeBusinessError = (caught: unknown): boolean => {
		if (!(caught instanceof ApiClientError)) return false;
		setIdempotencyKey(createInvoiceAdminIdempotencyKey());

		const code = caught.code ?? 0;
		if (code === COMPLAINT_NOT_FOUND_CODE) {
			setOpenAction(null);
			setBanner({
				tone: "danger",
				text: "這件申訴案件已經不存在，畫面已更新。",
			});
			void refetch();
			return true;
		}
		if (ACTION_CONFLICT_CODES.has(code)) {
			setOpenAction(null);
			// 不透傳後端 message：9336 的定版文案講的是申訴狀態、9322 講的是稅務資料，
			// 兩者在這一頁上要說的下一步是同一句。
			setBanner({ tone: "warning", text: actionConflictText(code) });
			void refetch();
			return true;
		}
		return false;
	};

	const submitting = async (run: () => Promise<unknown>) => {
		try {
			await run();
		} catch (caught) {
			if (routeBusinessError(caught)) return;
			throw caught;
		}
	};

	/**
	 * 端點 60｜申訴處理動作。
	 *
	 * `confirmed` 一律送 true：只有 `RECOMMEND_SUSPENSION` 需要它（後端的那一道守衛），
	 * 而它的勾選框由 dialog 依 `COMPLAINT_ACTION_NEEDS_CONFIRM` 強制 ⇒ 走到這裡代表人
	 * 已經勾過。分兩條路送反而會在新增「需要確認的動作」時漏掉。
	 */
	const act = useMutation({
		mutationFn: async ({
			action,
			reason,
		}: {
			action: string;
			reason: string;
		}) => {
			const res = await request<
				InvoiceEnvelope<ActOnInvoiceComplaintForAdminData>
			>(`${COMPLAINTS_ENDPOINT}/${encodeURIComponent(complaintId)}/actions`, {
				method: "POST",
				headers: { "Idempotency-Key": idempotencyKey },
				body: JSON.stringify({
					action,
					reason,
					confirmed: true,
					expectedVersion,
				}),
			});
			return { action, data: res?.data ?? null };
		},
		onSuccess: ({ action, data: result }) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenAction(null);
			const rec = result?.recommendation ?? null;
			setRecommendation(rec?.storeId ? rec : null);
			setBanner({
				tone: "success",
				text: `已送出「${COMPLAINT_ACTION_LABEL[action] ?? action}」。${
					rec?.storeId
						? "下方已帶出停權建議——這只是建議，沒有任何人因此被停權。"
						: ""
				}`,
			});
			void refetch();
		},
	});

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回申訴佇列"
						onClick={() => router.back()}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">
							申訴案件 {complaint?.caseNo || complaintId}
							{complaint ? (
								<>
									{" "}
									<StatusBadge
										variant={
											COMPLAINT_STATUS_BADGE_VARIANT[complaint.status] ?? "gray"
										}
									>
										{codeLabel(
											INVOICE_COMPLAINT_STATUS_LABEL,
											complaint.status,
										)}
									</StatusBadge>
								</>
							) : null}
						</h1>
						{/* 這句話不是裝飾：端點 59 會寫一筆 PII 存取稽核並有尖峰告警。 */}
						<p className="page-subtitle">開啟本頁會記錄一筆存取紀錄。</p>
						<div className="invoice-meta">
							{complaint?.orderNo ? (
								<span>訂單：{complaint.orderNo}</span>
							) : null}
							{complaint?.storeId ? (
								<span>賣家：{complaint.storeId}</span>
							) : null}
							{complaint ? (
								<SlaCountdown
									remainingBusinessDays={complaint.remainingBusinessDays}
									overdueBusinessDays={complaint.overdueBusinessDays}
								/>
							) : null}
						</div>
					</div>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						size="sm"
						onClick={() => refetch()}
						loading={isFetching}
					>
						手動更新
					</Button>
				</div>
			</div>

			{notFound ? (
				<section className="invoice-section">
					<p className="invoice-empty-hint">找不到這件申訴案件。</p>
					<Button
						variant="primary"
						onClick={() => router.push("/admin/invoices/complaints")}
					>
						返回佇列
					</Button>
				</section>
			) : (
				<>
					<InvoiceResultBanner banner={banner} />

					{isLoading ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint">載入中...</p>
						</section>
					) : isError ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint" role="alert">
								{governanceErrorText(error, "載入失敗，請稍後重試。")}
							</p>
							<Button variant="outline" onClick={() => refetch()}>
								重試
							</Button>
						</section>
					) : complaint === null ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint">沒有取得這件案件的內容。</p>
						</section>
					) : (
						<div className="invoice-detail-grid">
							<div>
								<Section title="訂單與買家陳述">
									<div className="invoice-fields">
										<Field label="案號">
											{complaint.caseNo || complaint.complaintId}
										</Field>
										<Field label="訂單編號">{complaint.orderNo || "—"}</Field>
										<Field
											label="買家"
											note="姓名由後端遮罩後下發，本頁不還原。"
										>
											{complaint.buyerMaskedName || "—"}
										</Field>
										<Field label="應提供的文件類型">
											{codeLabel(
												INVOICE_DOCUMENT_KIND_LABEL,
												complaint.expectedDocKind,
											)}
										</Field>
										<Field label="申訴時間">
											{formatDateTime(complaint.createdAt)}
										</Field>
										<Field label="重開次數">
											{complaint.reopenCount || "0"} 次
										</Field>
										<Field label="買家陳述" full>
											{complaint.description || "—"}
										</Field>
									</div>
								</Section>

								<Section
									title="發票現況與通知寄送歷程"
									hint="判斷一件「沒收到發票」的申訴要同時看兩件事：有沒有開，以及有沒有寄到。"
								>
									<p className="invoice-empty-hint">
										這兩塊資料目前不在申訴詳情的回應內容裡，因此本頁無法呈現。
										要確認發票是否已開立，請到「發票文件查詢」以訂單編號
										{complaint.orderNo ? ` ${complaint.orderNo} ` : " "}
										查詢；寄送結果請洽通知紀錄。
									</p>
								</Section>

								<Section
									title="往來紀錄"
									hint="買家申訴、賣家回覆、平台動作依時間排列。追加不覆蓋，本頁不提供編輯。"
								>
									<InvoiceTimeline
										emptyText="尚無往來紀錄。"
										entries={complaint.responses.map((item, index) => ({
											id: item.responseId || `response-${index}`,
											time: formatDateTime(item.occurredAt),
											// actor／action 是碼值而且**沒有對應的 CodeMeta 碼表**：
											// 58A 規劃由後端逐列下發 `eventLabel`，但 DTO 上沒有那個欄位。
											// 兩條路都沒有中文 ⇒ 原樣顯示碼值，不自己補一張碼表。
											title: item.action || "—",
											actor: item.actor ? `來源：${item.actor}` : undefined,
											body: item.note,
										}))}
									/>
								</Section>
							</div>

							<aside className="invoice-detail-rail">
								<section className="invoice-section">
									<h2 className="invoice-section__title">案件狀態</h2>
									<div className="invoice-fields">
										<Field label="目前狀態" full>
											<StatusBadge
												variant={
													COMPLAINT_STATUS_BADGE_VARIANT[complaint.status] ??
													"gray"
												}
											>
												{codeLabel(
													INVOICE_COMPLAINT_STATUS_LABEL,
													complaint.status,
												)}
											</StatusBadge>
										</Field>
										<Field label="處理期限" full>
											<div>{formatDateTime(complaint.dueAt)}</div>
											<SlaCountdown
												remainingBusinessDays={complaint.remainingBusinessDays}
												overdueBusinessDays={complaint.overdueBusinessDays}
											/>
										</Field>
										{complaint.buyerReviewDueAt ? (
											<Field label="買家確認期限" full>
												{formatDateTime(complaint.buyerReviewDueAt)}
											</Field>
										) : null}
										{complaint.reopenAllowedAfter ? (
											<Field label="可重開時間" full>
												{formatDateTime(complaint.reopenAllowedAfter)}
											</Field>
										) : null}
										<Field label="資料版本" full>
											{complaint.version || "—"}
										</Field>
									</div>
								</section>

								<section className="invoice-section">
									<h2 className="invoice-section__title">處理動作</h2>
									{availableActions.length === 0 ? (
										<p className="invoice-empty-hint">
											目前的狀態沒有可執行的處理動作。
										</p>
									) : (
										<>
											<p className="invoice-section__hint">
												每一個動作都要填理由，理由會寫入保存 5
												年的稽核紀錄。這裡只列出目前狀態下真的做得到的動作。
											</p>
											<div className="invoice-decision-actions">
												{availableActions.map((action) => (
													<Button
														key={action}
														variant={
															COMPLAINT_ACTION_DANGER.has(action)
																? "danger"
																: "outline"
														}
														size="sm"
														onClick={() => {
															setBanner(null);
															setOpenAction(action);
														}}
													>
														{COMPLAINT_ACTION_LABEL[action]}
													</Button>
												))}
											</div>
										</>
									)}
								</section>

								{recommendation ? (
									<section className="invoice-section">
										<h2 className="invoice-section__title">停權建議</h2>
										{/*
										 * 🔴 這一段必須讓人看得出「沒有人因此被停權」。發票模組只提建議：
										 * 不蓋 suspended_at、不發停權事件、不排恢復 job。寫成「已停權」
										 * 會讓 ADM 以為處分生效而不去治理頁，於是誰都沒有被處分。
										 */}
										<p className="invoice-section__hint">
											這是一份建議，**還沒有任何人被停權**。建立措施之後，處分才會生效。
										</p>
										<div className="invoice-fields">
											<Field label="90 天內判定違規的訂單數" full>
												{recommendation.violationCountIn90Days || "0"} 件
											</Field>
											<Field label="是否達到建議停權門檻" full>
												{recommendation.thresholdReached
													? "已達門檻（90 天內 3 件）"
													: "尚未達門檻"}
											</Field>
											<Field label="建議停權天數" full>
												{recommendation.suggestedSuspendDays || "—"} 天
											</Field>
											<Field label="建議理由" full>
												{recommendation.suggestedReason || "—"}
											</Field>
										</div>
										<div className="invoice-decision-actions">
											<Button
												variant="danger"
												size="sm"
												onClick={() =>
													setMeasureTarget({
														storeId:
															recommendation.storeId || complaint.storeId,
														initial: measureInitialFromSuggestion(
															recommendation,
															new Date(),
														),
													})
												}
											>
												依建議建立措施
											</Button>
										</div>
									</section>
								) : null}

								<section className="invoice-section">
									<h2 className="invoice-section__title">賣家治理</h2>
									<p className="invoice-section__hint">
										發票模組不寫入任何停權狀態——停權狀態的權威在賣家治理模組。
										在這裡建立措施，期間與自動解除由賣家治理處理。
									</p>
									<Button
										variant="outline"
										size="sm"
										onClick={() =>
											setMeasureTarget({ storeId: complaint.storeId })
										}
									>
										建立措施
									</Button>
								</section>
							</aside>
						</div>
					)}
				</>
			)}

			{measureTarget !== null ? (
				<CreateMeasureDialog
					storeId={measureTarget.storeId}
					initial={measureTarget.initial}
					onClose={() => setMeasureTarget(null)}
					onConfirm={handleCreateMeasure}
				/>
			) : null}

			{openAction ? (
				<InvoiceActionDialog
					idPrefix="adm-complaint-action"
					icon={
						openAction === COMPLAINT_ACTION_DECIDE_VIOLATION
							? "crisis_alert"
							: "support_agent"
					}
					title={COMPLAINT_ACTION_LABEL[openAction] ?? openAction}
					description={`案號 ${complaint?.caseNo || complaintId}　目前狀態：${codeLabel(
						INVOICE_COMPLAINT_STATUS_LABEL,
						status,
					)}`}
					consequence={COMPLAINT_ACTION_CONSEQUENCE[openAction]}
					reasonLabel="處理說明"
					reasonPlaceholder="例：已與賣家確認發票開立進度，請其於期限內補寄。"
					variant={
						COMPLAINT_ACTION_DANGER.has(openAction) ? "danger" : "primary"
					}
					confirmText={
						COMPLAINT_ACTION_NEEDS_CONFIRM.has(openAction)
							? "我了解這只會產生一份停權建議，不會停權任何人；實際處分要另外建立措施。"
							: undefined
					}
					submitLabel="送出"
					submittingLabel="送出中…"
					failureText="動作沒有送出，這件案件沒有變動；請確認網路狀態後再送一次。"
					onClose={() => setOpenAction(null)}
					onSubmit={async (reason) => {
						await submitting(() =>
							act.mutateAsync({ action: openAction, reason }),
						);
					}}
				/>
			) : null}
		</div>
	);
}
