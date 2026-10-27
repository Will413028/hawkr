"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StatusBadge } from "@/components/UI";
import {
	ACTION_CONFLICT_CODES,
	actionConflictText,
	createInvoiceAdminIdempotencyKey,
	DOCUMENT_NOT_FOUND_CODE,
	DOCUMENT_RESUME_NOT_ALLOWED_CODE,
	DOCUMENTS_ENDPOINT,
	RESEND_TOO_FREQUENT_CODE,
	RESEND_WINDOW_MINUTES,
} from "@/features/invoices/admin-actions";
import { DOCUMENT_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoiceActionDialog from "@/features/invoices/components/InvoiceActionDialog";
import InvoiceResultBanner, {
	type InvoiceBanner,
} from "@/features/invoices/components/InvoiceResultBanner";
import InvoiceTimeline from "@/features/invoices/components/InvoiceTimeline";
import {
	documentTimelineLabel,
	fileUnavailableText,
} from "@/features/invoices/documents";
import type {
	AdminInvoiceDocumentDetail,
	InvoiceEnvelope,
	InvoiceJobMutationData,
	ResendInvoiceNotificationData,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_DOCUMENT_KIND_LABEL,
	INVOICE_DOCUMENT_STATUS_LABEL,
	INVOICE_LINE_TYPE_LABEL,
	INVOICE_MANUAL_REASON_LABEL,
	INVOICE_OPERATION_STATUS_LABEL,
	INVOICE_TAX_CATEGORY_LABEL,
} from "@/lib/codemeta";
import { formatAmount, formatDateTime } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
// 對話框骨架（overlay、面板、標題列）在共用 Dialog 自己的 Dialog.css；這裡的 DelegatedOperations.css 只放
// `.adm-operation-field*` 等欄位與版面 class（58A 陷阱 C07：少 import 不會報錯也不會轉紅）。
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A13｜發票文件詳情（58A §A13，端點 48
// `GET /api/v1/admin/invoices/documents/{document_id}`）。
//
// ## 這一頁的存在理由是「四顆按鈕的共同目的地」
//
// A12 列尾「查看詳情」／A11 成功後的「查看這張發票」／A11 收到 9347 的導頁
// （`?from=duplicate`）／A08 與 A06 的「查看文件詳情」。四條裡接得上三條：
//   - A12 與 A11 的兩條都通（A11 已建，端點 57 `CreateManualIssuanceForAdmin` 在
//     `internal/grpc/admin_write.go`；`?from=duplicate` 的橫幅就在下面）
//   - A06 的 `AdminInvoiceJobDetail`（端點 51）與 A08 的 `InvoiceComplaintInfo`（端點 59）
//     **都沒有 `documentId` 欄位** ⇒ 那兩顆鍵組不出 href。這不是本頁的缺，是那兩支 DTO
//     的缺；憑語意猜一個 id 去導頁只會導到別人的發票。已列入回報。
//   反方向（A13 → A06）走得通：`relatedJobs[].jobId` 在契約上有。
//
// ## 存取會留痕，而且這句話是真的
//
// 副標寫「開啟本頁會記錄一筆存取紀錄」——這不是嚇阻用的場面話：invoice-service 的
// `GetInvoiceDocumentForAdmin` 逐字寫「usecase 在回資料之前先寫一列 INVOICE_PII_REVEAL
// 稽核，寫不進去就不回（fail-closed）」，並有 `InvoicePiiRevealSpike` 告警。
// ⚠️ **A06 不得寫這句**（陷阱 T30）：那一頁的主資料是端點 51。
//
// ## 唯一的寫入是「重新寄送通知」（端點 49，2026-09-02 接上）
//
// 作廢／折讓／重開／編輯是 job 層的操作，入口在 A06（各有雙人與樂觀鎖保護）。
//
// 端點 49 的 RPC **在** `internal/grpc/notification_resend.go`
// （`ResendInvoiceNotificationForAdmin`），路徑是
// `POST /admin/invoices/documents/{document_id}/notification_resends`
// （gateway `AdminInvoiceDocumentResend`）。舊版寫「沒有實作」，那個結論來自把它當成
// 掛在 job 上（`jobs/{id}/notification_resends` 回 404）——**它掛在 document 上**，
// 所以 A13 是它唯一組得出路徑的地方。
//
// ## 冷卻窗口只能靠後端的 9352
//
// 同一份 document 每 10 分鐘只能寄一次，而且**三個入口（賣家／ADM／買家）共用同一個
// 計數器**——ADM 幫忙點一次、賣家再點一次，買家會在十分鐘內收到兩封。
// ⚠️ 不能用 `nextResendAvailableAt` 判斷：那一欄的 mapper 從不填 ⇒ 恆為空字串，
// 讀它等於「隨時都可以寄」。守衛在 SQL 的 WHERE，唯一可靠的訊號是回來的 9352，
// 而 9352 是「剛剛才寄過」不是「寄失敗」，文案不得寫成失敗。
//
// ## 三塊 58A 指定但畫不出來的東西（詳見 types.ts）
//
//   - **Section 2「憑證預覽」**：端點 48 的回應沒有 `downloadPath`，只有 `invoiceFileKey`
//     （objectKey，58A 自己明令不顯示）。自行拼接下載路徑是陷阱 T22。且 `InvoicePdfViewer`
//     是 WEB 側的新元件（N4），ADM 沒有、也不該為單一頁面造一個。⇒ 整區不畫，
//     `fileAvailable === false` 時顯示 `fileUnavailableReason`。
//   - **`idempotencyKey`**：`toAdminDocumentDetail` 從不填 ⇒ 恆為空字串。畫出
//     「冪等鍵：—」等於宣稱這份文件沒有冪等鍵。
//   - **`fulfillmentMode`**：詳情 DTO 上不存在（列表才有 `invoiceMode`）。

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

export default function InvoiceDocumentDetailPage() {
	const router = useRouter();
	const params = useParams<{ documentId: string }>();
	const searchParams = useSearchParams();
	const documentId = String(params?.documentId ?? "");
	// 9347（冪等鍵命中、發票已經開好了）導過來時帶的旗標。**是 info 不是 error**：
	// 畫成錯誤會讓管理員以為沒開成功而再送一次。
	const fromDuplicate = searchParams.get("from") === "duplicate";

	const [resendOpen, setResendOpen] = useState(false);
	const [resumeOpen, setResumeOpen] = useState(false);
	const [banner, setBanner] = useState<InvoiceBanner | null>(null);
	const [idempotencyKey, setIdempotencyKey] = useState(
		createInvoiceAdminIdempotencyKey,
	);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "document", documentId],
		enabled: documentId !== "",
		queryFn: async () => {
			const res = await request<InvoiceEnvelope<AdminInvoiceDocumentDetail>>(
				`${DOCUMENTS_ENDPOINT}/${encodeURIComponent(documentId)}`,
				{
					method: "GET",
				},
			);
			return res?.data ?? null;
		},
	});

	const notFound =
		isError &&
		error instanceof ApiClientError &&
		error.code === DOCUMENT_NOT_FOUND_CODE;

	/**
	 * 端點 49｜重新寄送發票通知。
	 *
	 * 沒有 `expectedVersion`：這一支不改任何狀態，只是再送一次通知（request body 只有
	 * `reason`）。樂觀鎖對它沒有意義——沒有「被別人改掉」這回事。
	 */
	const resend = useMutation({
		mutationFn: async (reason: string) => {
			const res = await request<InvoiceEnvelope<ResendInvoiceNotificationData>>(
				`${DOCUMENTS_ENDPOINT}/${encodeURIComponent(documentId)}/notification_resends`,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({ reason }),
				},
			);
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setResendOpen(false);
			setBanner({
				tone: "success",
				text: `已重新寄出發票通知${
					result?.toMasked ? `給 ${result.toMasked}` : ""
				}${result?.sentAt ? `（${formatDateTime(result.sentAt)}）` : ""}。`,
			});
			void refetch();
		},
	});

	/**
	 * 端點 73｜把待人工處理、且尚無任何開票作業的文件送回自動線。
	 *
	 * ## 為什麼這一顆非有不可
	 *
	 * 自動線判定要人工介入時（賣家未完成收據代製授權、平台出資折扣、合併訂單、混合稅別）
	 * 文件落 `AWAITING_MANUAL`，而 **operation 根本不建**（58 §7.1.3 的 AM 節點明文）。
	 * A05 工作台是 job-first（後端 SQL 是 `invoice_operations` INNER JOIN `invoice_documents`）
	 * ⇒ 那些文件在工作台上**看不到**；A06 的三個動作（端點 52 重試／53 取消／56 代錄）
	 * 全部以 `job_id` 為鍵 ⇒ 對它們**組不出請求**。賣家補完授權也不會回頭處理既有文件。
	 * 在這一顆之前，那些文件沒有任何角色推得動，買家端永遠停在「處理中」。
	 *
	 * ## 三個不自己判的地方
	 *
	 * 1. **可見性看 `canResumeFromManual`**，不從 `status` 推導：後端的述詞是
	 *    「AWAITING_MANUAL **且** 零 operation」，已經有作業的走 A06。
	 * 2. **`expectedVersion` 取 `documentVersion`**，不是 `relatedJobs[].version`
	 *    ——這條路徑上還沒有 job。
	 * 3. **9341 的文案讀後端 message**：它與端點 52 共用碼值但下一步不同
	 *    （見 `DOCUMENT_RESUME_NOT_ALLOWED_CODE` 的紅字）。
	 */
	const resume = useMutation({
		mutationFn: async (reason: string) => {
			const res = await request<InvoiceEnvelope<InvoiceJobMutationData>>(
				`${DOCUMENTS_ENDPOINT}/${encodeURIComponent(documentId)}/manual_resolutions`,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({
						reason,
						// 取 `data`（useQuery 的結果）而不是下面的 `detail`：後者宣告在
						// 本 mutation 之後，讀它只是靠閉包在執行期才求值——能動，但把
						// 「這一行何時可用」變成需要推理的事。
						expectedVersion: data?.documentVersion ?? "",
					}),
				},
			);
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setResumeOpen(false);
			setBanner({
				tone: "success",
				text: `已送回自動線，並建立開票作業${
					result?.jobId ? ` ${result.jobId}` : ""
				}。系統會自動送出，可在「發票工作」頁面追蹤進度。`,
			});
			void refetch();
		},
	});

	const submitting = async (run: () => Promise<unknown>) => {
		try {
			await run();
		} catch (caught) {
			if (!(caught instanceof ApiClientError)) throw caught;
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			const code = caught.code ?? 0;
			if (code === RESEND_TOO_FREQUENT_CODE) {
				// 🔴 這**不是失敗**：它是「剛剛才寄過」。三個入口（賣家／ADM／買家）共用
				// 同一個 10 分鐘窗口 ⇒ 很可能是賣家剛按過。寫成紅字會讓 ADM 一直重按。
				setResendOpen(false);
				setBanner({
					tone: "warning",
					text: `這份文件在 ${RESEND_WINDOW_MINUTES} 分鐘內已經寄送過通知了（賣家與買家端的重寄共用同一個間隔）。請稍後再試，不要重複按。`,
				});
				return;
			}
			if (code === DOCUMENT_NOT_FOUND_CODE) {
				setResendOpen(false);
				setBanner({
					tone: "danger",
					text: "這份發票文件已經不存在，畫面已更新。",
				});
				void refetch();
				return;
			}
			if (ACTION_CONFLICT_CODES.has(code)) {
				setResendOpen(false);
				setBanner({ tone: "warning", text: actionConflictText(code) });
				void refetch();
				return;
			}
			throw caught;
		}
	};

	/**
	 * 端點 73 的錯誤分流。**不共用 `submitting`**：那一支對
	 * `ACTION_CONFLICT_CODES`（含 9341）套 `actionConflictText`，而 9341 在這裡的兩種
	 * 成因（狀態不符／已有作業）與端點 52 的「無法確認服務商端」完全無關 ——
	 * 套過去會給出一句與現場對不上的話，而後端的 message 本來就分得出來。
	 */
	const submittingResume = async (run: () => Promise<unknown>) => {
		try {
			await run();
		} catch (caught) {
			if (!(caught instanceof ApiClientError)) throw caught;
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			const code = caught.code ?? 0;
			if (code === DOCUMENT_NOT_FOUND_CODE) {
				setResumeOpen(false);
				setBanner({
					tone: "danger",
					text: "這份發票文件已經不存在，畫面已更新。",
				});
				void refetch();
				return;
			}
			if (code === DOCUMENT_RESUME_NOT_ALLOWED_CODE) {
				setResumeOpen(false);
				// 後端 message 是唯一分得出「狀態不符」與「已有作業」的東西。
				setBanner({
					tone: "warning",
					text: `${caught.message}（畫面已更新）`,
				});
				void refetch();
				return;
			}
			if (ACTION_CONFLICT_CODES.has(code)) {
				setResumeOpen(false);
				setBanner({ tone: "warning", text: actionConflictText(code) });
				void refetch();
				return;
			}
			throw caught;
		}
	};

	const detail = data ?? null;
	const sellerView = detail?.sellerView;
	const doc = sellerView?.detail;
	const summary = doc?.summary;
	// 品項行在 DTO 上**沒有任何 id**（`InvoiceLine` 只有 lineType/name/quantity/
	// unitPrice/netAmount/taxCategory），而同一張發票可以有兩行完全一樣的內容 ⇒
	// 用內容當 key 會撞。這裡把序號在 JSX 之外先綁成穩定 key：這張表是唯讀的、
	// 不排序也不增刪，序號就是它真正的身分。
	const lineRows = (doc?.lines ?? []).map((line, index) => ({
		key: `line-${index}`,
		line,
	}));

	// 返回用 router.back()：四個來源各自回原處。寫死回 A12 會讓從 A08 點進來的人
	// 掉到一個他沒去過的列表。
	const back = () => router.back();

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回上一頁"
						onClick={back}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">
							發票文件詳情
							{summary ? (
								<>
									{" "}
									<StatusBadge
										variant={
											DOCUMENT_STATUS_BADGE_VARIANT[summary.status] ?? "gray"
										}
									>
										{codeLabel(INVOICE_DOCUMENT_STATUS_LABEL, summary.status)}
									</StatusBadge>
								</>
							) : null}
						</h1>
						<div className="invoice-meta">
							<span>document_id：{documentId}</span>
							{summary?.invoiceNumber ? (
								<span>發票號碼：{summary.invoiceNumber}</span>
							) : null}
						</div>
					</div>
				</div>
				<div className="header-actions">
					{/*
					 * 端點 49。渲染條件只有「這份文件真的存在」——**不從
					 * `nextResendAvailableAt` 推冷卻**（那一欄的 mapper 從不填、恆為空字串，
					 * 讀它等於「隨時都可以寄」）。窗口守衛在後端的 SQL WHERE，撞到就回 9352。
					 */}
					{detail ? (
						<Button
							variant="outline"
							size="sm"
							onClick={() => {
								setBanner(null);
								setResendOpen(true);
							}}
						>
							重新寄送通知
						</Button>
					) : null}
					{/*
					 * 端點 73。渲染條件**只看後端的 `canResumeFromManual`**——它是
					 * 「AWAITING_MANUAL 且零 operation」那一支述詞的讀端，與寫端同一份。
					 * 自己從 status 推會讓按鈕亮在一個 CAS 必定 rows=0 的狀態上。
					 */}
					{detail?.canResumeFromManual ? (
						<Button
							variant="primary"
							size="sm"
							onClick={() => {
								setBanner(null);
								setResumeOpen(true);
							}}
						>
							送回自動線
						</Button>
					) : null}
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
					<p className="invoice-empty-hint">找不到這份發票文件。</p>
					<Button variant="primary" onClick={back}>
						返回
					</Button>
				</section>
			) : (
				<>
					{fromDuplicate ? (
						<div className="invoice-duplicate-notice" role="status">
							<MaterialSymbol name="info" />
							<div>
								這筆開票先前已經送出過，以下是那一次的結果。不需要、也不應該再送一次。
							</div>
						</div>
					) : null}

					{/*
					 * 這句話與事實相符（端點 48 fail-closed 落一列 INVOICE_PII_REVEAL），
					 * 所以它是真的嚇阻而不是假保證。
					 */}
					<p className="invoice-page-notice" role="note">
						<MaterialSymbol name="shield_lock" />
						<span>
							開啟本頁會<strong>記錄一筆存取紀錄</strong>
							（含操作者與時間），可在「發票稽核紀錄」查到。
						</span>
					</p>

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
					) : !detail || !doc || !summary || !sellerView ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint">沒有取得這份文件的資料。</p>
						</section>
					) : (
						<>
							<Section title="文件摘要">
								<div className="invoice-fields">
									<Field
										label="發票號碼"
										note="完整值：它是對獎、報帳與申訴的唯一憑據，平台不遮罩（每次開啟本頁都會留下稽核）。"
									>
										{summary.invoiceNumber || "—"}
									</Field>
									<Field label="文件類型">
										{codeLabel(
											INVOICE_DOCUMENT_KIND_LABEL,
											summary.documentKind,
										)}
									</Field>
									<Field label="開立時間">
										{formatDateTime(summary.issuedAt)}
									</Field>
									<Field label="總金額">
										{formatAmount(summary.totalAmount)}
									</Field>
									<Field label="載具">{summary.carrierSummary || "—"}</Field>
									<Field
										label="關聯文件"
										note="折讓單指回原發票；重開的新發票指回舊發票。"
									>
										{summary.relatedDocumentId ? (
											<button
												type="button"
												className="invoice-inline-link"
												onClick={() =>
													router.push(
														`/admin/invoices/documents/${encodeURIComponent(summary.relatedDocumentId)}`,
													)
												}
											>
												{summary.relatedDocumentId}
											</button>
										) : (
											"—"
										)}
									</Field>
									<Field label="開票人">{doc.issuerName || "—"}</Field>
									<Field label="開票人統編">{doc.issuerTaxId || "—"}</Field>
									<Field label="買受人">{doc.recipientName || "—"}</Field>
									<Field label="買受人統編">{doc.recipientTaxId || "—"}</Field>
									<Field label="啟用切點版本">
										{sellerView.cutoverVersion || "—"}
									</Field>
									<Field label="人工處理理由">
										{sellerView.manualReasonCode
											? codeLabel(
													INVOICE_MANUAL_REASON_LABEL,
													sellerView.manualReasonCode,
												)
											: "—"}
									</Field>
									<Field
										label="服務商請求編號"
										note="向服務商查問單筆問題時提供這個編號。"
									>
										<span className="invoice-code">
											{sellerView.providerRequestId || "—"}
										</span>
									</Field>
									<Field
										label="憑證檔案"
										full
										note="本頁不提供下載：端點 48 的回應沒有下載路徑欄位，自行拼接出來的會是一條死鍵。"
									>
										{summary.fileAvailable
											? "服務商端已有檔案"
											: fileUnavailableText(summary.fileUnavailableReason)}
									</Field>
								</div>
							</Section>

							<Section
								title="明細與稅額"
								hint="銷售額與稅額由後端依法定拆分算出，前端不重算——算一份就是製造第二個真相。"
							>
								<div className="table-container">
									<table className="data-table">
										<caption className="sr-only">
											發票品項明細，共 {doc.lines.length} 行
										</caption>
										<thead>
											<tr>
												<th scope="col">種類</th>
												<th scope="col">品名</th>
												<th scope="col">數量</th>
												<th scope="col">單價</th>
												<th scope="col">小計</th>
												<th scope="col">稅別</th>
											</tr>
										</thead>
										<tbody>
											{doc.lines.length === 0 ? (
												<tr>
													<td colSpan={6} className="invoice-empty-cell">
														這份文件沒有品項明細。
													</td>
												</tr>
											) : (
												lineRows.map(({ key, line }) => (
													<tr key={key}>
														<td>
															{codeLabel(
																INVOICE_LINE_TYPE_LABEL,
																line.lineType,
															)}
														</td>
														<td>{line.name || "—"}</td>
														<td>{line.quantity || "—"}</td>
														<td>{formatAmount(line.unitPrice)}</td>
														{/*
														 * DISCOUNT 行的 netAmount 是負數（AMEGO 官方範例即以負數
														 * 明細表示折抵）⇒ 顯示為 -NT$ 120，**不取絕對值**：
														 * 取了之後這一行看起來就像又加了一筆錢。
														 */}
														<td>{formatAmount(line.netAmount)}</td>
														<td>
															{codeLabel(
																INVOICE_TAX_CATEGORY_LABEL,
																line.taxCategory,
															)}
														</td>
													</tr>
												))
											)}
										</tbody>
									</table>
								</div>
								<div className="invoice-fields invoice-fields--spaced">
									<Field label="銷售額">{formatAmount(doc.salesAmount)}</Field>
									<Field label="稅額">{formatAmount(doc.taxAmount)}</Field>
									<Field label="總計">{formatAmount(doc.totalAmount)}</Field>
									<Field label="稅別">
										{codeLabel(INVOICE_TAX_CATEGORY_LABEL, doc.taxCategory)}
									</Field>
								</div>
							</Section>

							<Section
								title={`狀態歷程（${doc.timeline.length} 筆）`}
								hint="追加不覆蓋：每一次狀態變更都保留，這一頁不提供編輯。"
							>
								<InvoiceTimeline
									emptyText="尚無狀態變更紀錄。"
									entries={doc.timeline.map((item, index) => ({
										id: `${item.occurredAt}-${item.event}-${index}`,
										time: formatDateTime(item.occurredAt),
										title: documentTimelineLabel(item.event),
										body: item.note || undefined,
									}))}
								/>
							</Section>

							<Section
								title={`AMEGO 送件歷程（${detail.providerExchanges.length} 筆）`}
								hint="請求與回應內容在寫入時就已遮罩；這裡沒有「顯示完整」的切換，因為系統本身沒有保存明文。"
							>
								{detail.providerExchanges.length === 0 ? (
									<p className="invoice-empty-hint">尚未送出任何請求。</p>
								) : (
									detail.providerExchanges.map((ex, index) => (
										<div
											key={ex.exchangeId || `exchange-${index}`}
											className="invoice-exchange"
										>
											<div className="invoice-exchange__head">
												<span>{formatDateTime(ex.occurredAt)}</span>
												<span className="invoice-exchange__endpoint">
													{ex.endpoint || "—"}
												</span>
												{/* code 與 message 成對：AMEGO 的 53 在官方錯誤表出現兩次
												    且語意不同，只看 code 會導向錯誤的下一步。 */}
												<span>
													代碼 {ex.providerCode || "—"}：
													{ex.providerMessage || "—"}
												</span>
												<span className="invoice-code">
													requestId {ex.requestId || "—"}
												</span>
											</div>
											<details className="invoice-details">
												<summary>展開請求與回應內容（已遮罩）</summary>
												<pre className="invoice-pre">
													{`── 請求 ──\n${ex.maskedRequestBody || "（無）"}\n\n── 回應 ──\n${ex.maskedResponseBody || "（無）"}`}
												</pre>
											</details>
										</div>
									))
								)}
							</Section>

							<Section
								title={`關聯的發票工作（${detail.relatedJobs.length} 筆）`}
								hint="一份文件可能同時有多個工作（例如開票成功 ＋ 作廢中），所以工作狀態與文件狀態分開列。"
							>
								{detail.relatedJobs.length === 0 ? (
									<p className="invoice-empty-hint">
										沒有關聯的發票工作。
										{detail.canResumeFromManual
											? "　這份文件停在「待人工處理」而且還沒有任何開票作業——它不會出現在「發票工作」頁面，也不會有人自動處理。確認人工判定完成後，用右上角的「送回自動線」交回系統自動處理。"
											: ""}
									</p>
								) : (
									<div className="table-container">
										<table className="data-table">
											<caption className="sr-only">
												與這份文件關聯的發票工作，共 {detail.relatedJobs.length}{" "}
												筆
											</caption>
											<thead>
												<tr>
													<th scope="col">工作 ID</th>
													<th scope="col">狀態</th>
													<th scope="col">最近嘗試</th>
													<th scope="col">操作</th>
												</tr>
											</thead>
											<tbody>
												{detail.relatedJobs.map((job) => (
													<tr key={job.jobId}>
														<td>
															<div className="cell-main">{job.jobId}</div>
														</td>
														<td>
															{codeLabel(
																INVOICE_OPERATION_STATUS_LABEL,
																job.status,
															)}
														</td>
														<td>{formatDateTime(job.lastAttemptAt)}</td>
														<td>
															<div className="invoice-table-actions">
																<Button
																	variant="outline"
																	size="sm"
																	onClick={() =>
																		router.push(
																			`/admin/invoices/jobs/${encodeURIComponent(job.jobId)}`,
																		)
																	}
																>
																	查看工作
																</Button>
															</div>
														</td>
													</tr>
												))}
											</tbody>
										</table>
									</div>
								)}
							</Section>
						</>
					)}
				</>
			)}

			{resumeOpen ? (
				<InvoiceActionDialog
					idPrefix="adm-document-resume"
					icon="refresh"
					title="送回自動線"
					description={`文件編號 ${documentId}　目前狀態：${codeLabel(
						INVOICE_DOCUMENT_STATUS_LABEL,
						summary?.status ?? "",
					)}${
						sellerView?.manualReasonCode
							? `　人工處理原因：${codeLabel(
									INVOICE_MANUAL_REASON_LABEL,
									sellerView.manualReasonCode,
								)}`
							: ""
					}`}
					consequence="會把這份文件送回自動開立流程，並建立一件開票作業。系統接著會真的向服務商送出開立請求——按之前請先確認人工判定已完成（例如賣家已補簽收據代製授權、折扣資金來源已釐清）。送出後可在「發票工作」頁面追蹤。"
					reasonLabel="送回自動線的理由"
					reasonPlaceholder="例：賣家已於今日完成收據代製授權，確認可由平台代製，送回自動線。"
					submitLabel="確認送回"
					submittingLabel="處理中…"
					failureText="沒有送回；請確認網路狀態後再送一次。"
					onClose={() => setResumeOpen(false)}
					onSubmit={async (reason) => {
						await submittingResume(() => resume.mutateAsync(reason));
					}}
				/>
			) : null}

			{resendOpen ? (
				<InvoiceActionDialog
					idPrefix="adm-document-resend"
					icon="forward_to_inbox"
					title="重新寄送發票通知"
					description={`發票號碼 ${summary?.invoiceNumber || "—"}　文件編號 ${documentId}`}
					consequence={`會再寄一封發票通知給原收件人。同一份文件每 ${RESEND_WINDOW_MINUTES} 分鐘只能寄一次，而且這個間隔與賣家端、買家端的重寄共用——按之前先確認不是剛剛已經有人寄過。`}
					reasonLabel="重寄理由"
					reasonPlaceholder="例：買家反映沒有收到發票通知信，已確認信箱正確，重新寄送。"
					submitLabel="確認重寄"
					submittingLabel="寄送中…"
					failureText="通知沒有寄出；請確認網路狀態後再送一次。"
					onClose={() => setResendOpen(false)}
					onSubmit={async (reason) => {
						await submitting(() => resend.mutateAsync(reason));
					}}
				/>
			) : null}
		</div>
	);
}
