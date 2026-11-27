"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StatusBadge } from "@/components/UI";
import {
	ACTION_CONFLICT_CODES,
	actionConflictText,
	createInvoiceAdminIdempotencyKey,
	receiptAuthRelevant,
	TAX_APPROVAL_IDENTITY_INCOMPLETE_CODE,
	TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE,
	TAX_DECISION_APPROVALS,
	TAX_DECISION_CONSEQUENCE,
	TAX_DECISION_NEEDS_CONFIRM,
	TAX_DECISION_ORDER,
	TAX_EXTENSION_APPROVE,
	TAX_EXTENSION_DEFAULT_DAYS,
	TAX_EXTENSION_MAX_DAYS,
	TAX_EXTENSION_REJECT,
	TAX_PROFILE_NOT_FOUND_CODE,
	TAX_REVIEW_ENDPOINT,
	taxDecisionAvailable,
	taxDecisionBlockedByReceiptAuth,
	taxDecisionLabel,
	taxExtensionApprovable,
} from "@/features/invoices/admin-actions";
import { TAX_REVIEW_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoiceActionDialog from "@/features/invoices/components/InvoiceActionDialog";
import InvoiceResultBanner, {
	type InvoiceBanner,
} from "@/features/invoices/components/InvoiceResultBanner";
import InvoiceTimeline from "@/features/invoices/components/InvoiceTimeline";
import type {
	DecideTaxReviewForAdminData,
	InvoiceEnvelope,
	SellerTaxProfileData,
} from "@/features/invoices/types";
import {
	codeLabel,
	SELLER_TAX_ENTITY_TYPE_LABEL,
	SELLER_TAX_OBLIGATION_LABEL,
	SELLER_TAX_REVIEW_STATUS_LABEL,
	SellerTaxReviewDecision,
} from "@/lib/codemeta";
import { formatAmount, formatDateTime } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

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

export default function TaxReviewDetailPage() {
	const router = useRouter();
	const params = useParams<{ storeId: string }>();
	const storeId = String(params?.storeId ?? "");

	/** 開著的對話框：`null` ＝ 沒開；決策走碼值、延長走兩個字面值。 */
	const [openDecision, setOpenDecision] = useState<string | null>(null);
	const [openExtension, setOpenExtension] = useState<string | null>(null);
	const [banner, setBanner] = useState<InvoiceBanner | null>(null);
	/** APPROVE_* 專屬：核定效期（選填，空 ＝ 後端預設 365 天）。 */
	const [validUntil, setValidUntil] = useState("");
	/** APPROVE_INVOICE_EXEMPT 專屬：核定依據文件（選填，值是 documentId）。 */
	const [basisDocumentId, setBasisDocumentId] = useState("");
	/** 延長天數（APPROVE 才送）。 */
	const [extendDays, setExtendDays] = useState(
		String(TAX_EXTENSION_DEFAULT_DAYS),
	);

	/**
	 * 冪等鍵。**送出失敗不換**（操作可能已生效而回應沒送到，換鍵會被當成第二次操作）；
	 * **收到回應之後要換**（`uq_invadmaudit_client_request` 走 `ON CONFLICT DO NOTHING`
	 * ⇒ 沿用同一把鍵的第二次操作會靜靜地不寫稽核列）。
	 */
	const [idempotencyKey, setIdempotencyKey] = useState(
		createInvoiceAdminIdempotencyKey,
	);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "taxReview", storeId],
		enabled: storeId !== "",
		queryFn: async () => {
			const res = await request<InvoiceEnvelope<SellerTaxProfileData>>(
				`${TAX_REVIEW_ENDPOINT}/${encodeURIComponent(storeId)}`,
				{ method: "GET" },
			);
			return res?.data ?? null;
		},
	});

	const notFound =
		isError &&
		error instanceof ApiClientError &&
		error.code === TAX_PROFILE_NOT_FOUND_CODE;

	const profile = data ?? null;
	const identity = profile?.taxIdentity;
	const attestation = profile?.selfIssueAttestation;
	const intakeLimit = profile?.intakeLimit;
	const hasIntakeLimit = (intakeLimit?.orderCountLimit ?? "") !== "";
	const attestationAccepted = (attestation?.termsVersion ?? "") !== "";

	const address = [
		identity?.addressCity,
		identity?.addressDistrict,
		identity?.addressDetail,
	]
		.filter((part) => (part ?? "") !== "")
		.join("");

	const reviewStatus = profile?.reviewStatus ?? "";
	/**
	 * 樂觀鎖的值。**取自這一次載入讀到的那筆**——寫死或補 0 會讓兩位管理員同時開著
	 * 這一頁時，後送出的那一位直接覆蓋前一位的決策，而畫面上兩邊都顯示成功。
	 */
	const expectedVersion = profile?.version ?? "";
	const decisionPath = `${TAX_REVIEW_ENDPOINT}/${encodeURIComponent(storeId)}/decisions`;
	const extensionPath = `${TAX_REVIEW_ENDPOINT}/${encodeURIComponent(storeId)}/extensions`;

	/** 目前狀態下真的做得到的決策。條件不成立時**不渲染**（不是 disabled）。 */
	const availableDecisions = TAX_DECISION_ORDER.filter((decision) =>
		taxDecisionAvailable(decision, reviewStatus),
	);
	/**
	 * 「核定應開統一發票」需要統編 ＋ 法定名稱齊備（backend#3049）。
	 *
	 * 🔴 這是**預防**不是授權：後端 `Decide` 有同一道守衛（9353），拿掉這裡只會讓 ADM
	 * 按下去才知道。反過來也一樣——只做這裡而不做後端，任何直接打端點的人照樣撞
	 * `chk_stp_required_needs_taxid` 的 500。
	 *
	 * 用 disabled 而不是不渲染（與 `availableDecisions` 的慣例刻意不同）：狀態機層
	 * 不成立時那個決策**不存在**，而這裡它存在、只是現在按不得——藏起來會讓 ADM
	 * 找不到自己要的那顆鍵，也看不到「為什麼」。
	 */
	const identityIncomplete =
		!(profile?.taxIdentity.taxId ?? "").trim() ||
		!(profile?.taxIdentity.legalName ?? "").trim();
	const canApproveExtension = taxExtensionApprovable(
		reviewStatus,
		profile?.extensionCount ?? "",
	);

	/**
	 * 業務碼分流。回 `true` ＝ 已經在頁面層處理掉（dialog 該關），
	 * 回 `false` ＝ 交給呼叫端就地顯示（錯誤就地顯示、不關 dialog）。
	 */
	const routeBusinessError = (caught: unknown): boolean => {
		if (!(caught instanceof ApiClientError)) return false;
		setIdempotencyKey(createInvoiceAdminIdempotencyKey());

		const code = caught.code ?? 0;
		if (code === TAX_PROFILE_NOT_FOUND_CODE) {
			setOpenDecision(null);
			setOpenExtension(null);
			setBanner({
				tone: "danger",
				text: "這筆稅務身分申請已經不存在，畫面已更新。",
			});
			void refetch();
			return true;
		}
		if (code === TAX_APPROVAL_IDENTITY_INCOMPLETE_CODE) {
			setOpenDecision(null);
			setBanner({
				tone: "danger",
				text: "這位賣家還沒有填寫統一編號與法定名稱，無法核定為應開統一發票。請改按「要求補件」，等賣家補齊後再核定。",
			});
			return true;
		}
		if (code === TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE) {
			setOpenDecision(null);
			setBanner({
				tone: "danger",
				text: "這位賣家尚未完成「收據代製授權」，無法核定為免用統一發票。請通知他到賣家中心簽署授權（ADM 無法代簽），簽好之後再核定。畫面已更新。",
			});
			void refetch();
			return true;
		}
		if (ACTION_CONFLICT_CODES.has(code)) {
			setOpenDecision(null);
			setOpenExtension(null);
			setBanner({ tone: "warning", text: actionConflictText(code) });
			void refetch();
			return true;
		}
		return false;
	};

	/**
	 * 送出包裝。
	 *
	 * 🔴 分流不能寫在 `useMutation` 的 `onError` 裡：那個回呼丟出去的例外**不會**傳到
	 * `mutateAsync` 的呼叫端，dialog 會收到原始例外並把「狀態已變」也顯示成一則就地
	 * 錯誤——而那一類正是要關掉 dialog、refetch、在頁面上說一句話的。
	 */
	const submitting = async (run: () => Promise<unknown>) => {
		try {
			await run();
		} catch (caught) {
			if (routeBusinessError(caught)) return;
			throw caught;
		}
	};

	/**
	 * 端點 40｜審核裁決。
	 *
	 * `confirmed` 只有 `APPROVE_INVOICE_EXEMPT` 與 `REVOKE` 需要，但**一律送 true**：
	 * 那兩個的勾選框由 dialog 強制（`TAX_DECISION_NEEDS_CONFIRM`），走到這裡代表人已經
	 * 勾過；其餘決策後端不看這一欄。分兩條路送反而會在新增「需要確認的決策」時漏掉。
	 *
	 * `approvalBasisKey` 與 `approvalBasisAssetId` **同進同出**（後端 `ErrTaxApprovalBasisIncomplete`），
	 * 而且兩者都要來自這個賣家自己上傳的稅籍文件 ⇒ 值直接取自 `documents[]` 那一列，
	 * 不讓人手打。
	 */
	const decide = useMutation({
		mutationFn: async ({
			decision,
			reason,
		}: {
			decision: string;
			reason: string;
		}) => {
			const basis =
				decision === SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT
					? (profile?.documents ?? []).find(
							(doc) => doc.documentId === basisDocumentId,
						)
					: undefined;
			const res = await request<InvoiceEnvelope<DecideTaxReviewForAdminData>>(
				decisionPath,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({
						decision,
						reason,
						approvalValidUntil:
							TAX_DECISION_APPROVALS.has(decision) && validUntil
								? `${validUntil}T00:00:00Z`
								: "",
						confirmed: true,
						expectedVersion,
						approvalBasisKey: basis?.fileKey ?? "",
						approvalBasisAssetId: basis?.fileAssetId ?? "",
					}),
				},
			);
			return { decision, data: res?.data ?? null };
		},
		onSuccess: ({ decision, data: result }) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenDecision(null);
			setValidUntil("");
			setBasisDocumentId("");
			const revoked = result?.revokedEffectiveAt
				? `已核定資格自 ${formatDateTime(result.revokedEffectiveAt)} 起失效（在此之前已代製的收據不受影響）。`
				: "";
			setBanner({
				tone: "success",
				text: `已送出「${taxDecisionLabel(decision)}」。${revoked}`,
			});
			void refetch();
		},
	});

	/**
	 * 端點 41｜延長裁決。
	 *
	 * 🔴 `decision` 是 `APPROVE`／`REJECT`，**不是** `SellerTaxReviewDecision` 的值域
	 * （送 `EXTEND` 過去會拿到 `decision is outside the defined value set`）。
	 * `extendDays` 只有 APPROVE 會用到，且後端限 1–365。
	 */
	const decideExtension = useMutation({
		mutationFn: async ({
			decision,
			reason,
		}: {
			decision: string;
			reason: string;
		}) => {
			const res = await request<InvoiceEnvelope<DecideTaxReviewForAdminData>>(
				extensionPath,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({
						decision,
						extendDays:
							decision === TAX_EXTENSION_APPROVE ? extendDays.trim() : "",
						reason,
						expectedVersion,
					}),
				},
			);
			return { decision, data: res?.data ?? null };
		},
		onSuccess: ({ decision }) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenExtension(null);
			setBanner({
				tone: "success",
				text:
					decision === TAX_EXTENSION_APPROVE
						? `已核准延長 ${extendDays.trim() || TAX_EXTENSION_DEFAULT_DAYS} 天。新的期限從**原到期日**起算，不是從今天起算。`
						: "已駁回這次的延長申請，期限維持原樣。",
			});
			void refetch();
		},
	});

	if (isError && notFound) {
		return (
			<div>
				<div className="page-header">
					<div className="invoice-header-main">
						<button
							type="button"
							className="btn-back"
							aria-label="返回稅務身分審核列表"
							onClick={() => router.push("/admin/invoices/tax-reviews")}
						>
							<MaterialSymbol name="arrow_back" />
						</button>
						<h1 className="page-title">稅務身分審核</h1>
					</div>
				</div>
				<section className="invoice-section">
					<p className="invoice-empty-hint">找不到這筆稅務身分申請。</p>
					<Button
						variant="primary"
						onClick={() => router.push("/admin/invoices/tax-reviews")}
					>
						返回列表
					</Button>
				</section>
			</div>
		);
	}

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回稅務身分審核列表"
						onClick={() => router.push("/admin/invoices/tax-reviews")}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">
							稅務身分審核
							{profile ? (
								<>
									{" "}
									<StatusBadge
										variant={
											TAX_REVIEW_STATUS_BADGE_VARIANT[profile.reviewStatus] ??
											"gray"
										}
									>
										{codeLabel(
											SELLER_TAX_REVIEW_STATUS_LABEL,
											profile.reviewStatus,
										)}
									</StatusBadge>
								</>
							) : null}
						</h1>
						<div className="invoice-meta">
							<span>store_id：{storeId}</span>
							{profile ? (
								<span>提交時間：{formatDateTime(profile.submittedAt)}</span>
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
			) : profile === null ? (
				<section className="invoice-section">
					<p className="invoice-empty-hint">沒有取得這筆申請的內容。</p>
				</section>
			) : (
				<div className="invoice-detail-grid">
					<div>
						{reviewStatus === "APPROVED_INVOICE_EXEMPT" &&
						!profile.receiptAuthorized ? (
							<section className="invoice-section">
								<p className="invoice-empty-hint" role="alert">
									這位賣家已核定「免用統一發票」，但尚未簽署收據代製授權——在他簽署之前，平台不會替他產出任何收據：每一筆已付款訂單都會落在「待人工處理」，而那些文件不會出現在「發票工作」頁面，買家端會一直顯示「處理中」。請通知賣家到賣家中心完成授權；已經卡住的那些，可到「發票文件」逐筆用「送回自動線」處理。
								</p>
							</section>
						) : null}
						<Section
							title="賣家與申請摘要"
							hint="「賣家自述義務」是賣家自己勾選的，不是平台的判定結果；判定結果是頁首的審核狀態。"
						>
							<div className="invoice-fields">
								<Field label="store_id">{storeId}</Field>
								<Field label="主體類型">
									{codeLabel(
										SELLER_TAX_ENTITY_TYPE_LABEL,
										identity?.entityType,
									)}
								</Field>
								<Field label="賣家自述義務" note="賣家自述，非判定依據。">
									{codeLabel(
										SELLER_TAX_OBLIGATION_LABEL,
										identity?.claimedStatus,
									)}
								</Field>

								<Field label="申請統編">{identity?.taxId || "—"}</Field>
								<Field label="審核期限">
									{formatDateTime(profile.deadlineAt)}
								</Field>

								{receiptAuthRelevant(reviewStatus) ? (
									<Field
										label="收據代製授權"
										note="賣家於賣家中心簽署，ADM 無法代簽。未授權時無法核定免用統一發票，核定後也不會產出收據。"
									>
										{profile.receiptAuthorized
											? `已授權（${formatDateTime(profile.receiptAuthorizedAt)}）`
											: "尚未授權"}
									</Field>
								) : null}
								<Field
									label="已使用延長"
									note={
										profile.extensionCount === "0"
											? "延長：可用 1 次"
											: "延長：已使用 1/1"
									}
								>
									{profile.extensionCount || "0"} 次
								</Field>
								{profile.reapplyCooldownUntil ? (
									<Field label="可重新申請時間">
										{formatDateTime(profile.reapplyCooldownUntil)}
									</Field>
								) : null}
							</div>
						</Section>

						<Section
							title={`稅務文件（${profile.documents.length} 份）`}
							hint="檢視會在新分頁開啟受權限保護的檔案。PDF 沒有內嵌預覽器，一律新分頁開啟。"
						>
							{profile.documents.length === 0 ? (
								<p className="invoice-empty-hint">
									這筆申請沒有檢附任何稅務文件。
								</p>
							) : (
								<div className="table-container">
									<table className="data-table">
										<thead>
											<tr>
												<th>類型</th>
												<th>檔名</th>
												<th>上傳時間</th>
												<th>操作</th>
											</tr>
										</thead>
										<tbody>
											{profile.documents.map((doc) => (
												<tr key={doc.documentId}>
													<td className="invoice-code">{doc.kind || "—"}</td>
													<td>{doc.fileName || "—"}</td>
													<td>{formatDateTime(doc.uploadedAt)}</td>
													<td>
														{doc.fileDownloadPath ? (
															<a
																className="invoice-doc-link"
																href={`/api/bff${doc.fileDownloadPath}`}
																target="_blank"
																rel="noopener noreferrer"
															>
																檢視
															</a>
														) : (
															"—"
														)}
													</td>
												</tr>
											))}
										</tbody>
									</table>
								</div>
							)}
						</Section>

						<Section title="申請內容">
							<div className="invoice-fields">
								<Field label="法定名稱">{identity?.legalName || "—"}</Field>
								<Field label="負責人">{identity?.responsibleName || "—"}</Field>
								<Field label="營業地址" full>
									{address || "—"}
								</Field>
								<Field label="檢附文件數">{profile.documents.length} 份</Field>
								<Field label="自行開票責任條款">
									{attestationAccepted ? "已接受" : "未接受"}
								</Field>
								{attestationAccepted ? (
									<>
										<Field label="條款版本">{attestation?.termsVersion}</Field>
										<Field label="接受時間">
											{formatDateTime(attestation?.acceptedAt)}
										</Field>

										<Field label="接受人 account_id">
											{attestation?.acceptedByAccountId ||
												"見稽核主控台的狀態歷程"}
										</Field>

										<Field label="自行開票責任">
											{attestation?.effective ? "生效中" : "尚未生效"}
										</Field>
									</>
								) : null}
							</div>
						</Section>

						<Section
							title="審核備註"
							hint="只顯示最近一次的備註，而且只有「要求補件／駁回／延長／撤銷／維持審核中」會寫入——核准與重開不寫。完整的決策理由在稽核紀錄。"
						>
							<InvoiceTimeline
								emptyText="尚無審核備註。"
								entries={profile.admNotes.map((note, index) => ({
									id: note.noteId || `note-${index}`,
									time: formatDateTime(note.createdAt),
									title: "審核備註",
									body: note.content,
								}))}
							/>
						</Section>
					</div>

					<aside className="invoice-detail-rail">
						<section className="invoice-section">
							<h2 className="invoice-section__title">目前狀態</h2>
							<div className="invoice-fields">
								<Field label="審核狀態" full>
									<StatusBadge
										variant={
											TAX_REVIEW_STATUS_BADGE_VARIANT[profile.reviewStatus] ??
											"gray"
										}
									>
										{codeLabel(
											SELLER_TAX_REVIEW_STATUS_LABEL,
											profile.reviewStatus,
										)}
									</StatusBadge>
								</Field>
								<Field label="資料版本" full>
									{profile.version || "—"}
								</Field>
							</div>
						</section>

						<section className="invoice-section">
							<h2 className="invoice-section__title">審核決策</h2>
							{availableDecisions.length === 0 ? (
								<p className="invoice-empty-hint">
									目前的狀態沒有可執行的審核決策。
									{reviewStatus === "SUPPLEMENT_REQUIRED"
										? "這筆申請正在等賣家補件，補齊後會自動回到「審核中」，屆時才會出現決策選項。"
										: ""}
								</p>
							) : (
								<>
									<p className="invoice-section__hint">
										每一個決策都要填理由，理由會寫入保存 5
										年的發票稽核紀錄。這裡只列出目前狀態下真的做得到的動作。
									</p>

									{availableDecisions.includes(
										SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT,
									) ? (
										profile.receiptAuthorized ? (
											<p className="invoice-section__hint">
												這位賣家已簽署「收據代製授權」，核定「免用統一發票」之後平台就會替他產出收據。
											</p>
										) : (
											<p className="invoice-section__hint">
												這位賣家還沒有簽署「收據代製授權」，「核定免用統一發票」暫時不能按——請通知他到賣家中心完成授權（ADM
												無法代簽，賣家在審核期間就簽得了）。他簽好之後重新整理這一頁，這顆按鈕就會恢復。若不等授權就核定，他每一筆已付款訂單的收據都會落在「待人工處理」，買家端會一直顯示處理中。
											</p>
										)
									) : null}
									{identityIncomplete &&
									availableDecisions.includes(
										SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED,
									) ? (
										/**
										 * 🔴 指引要指到「這個狀態下真的按得到」的那顆鍵。
										 *
										 * 「要求補件」的 allowed-from 只有 `UNDER_REVIEW`
										 * （backend `taxDecisionAllowedFrom`，另外三個決策都含
										 * `SELF_ISSUE_PENDING`）⇒ 在 `SELF_ISSUE_PENDING` 下它
										 * **不會被渲染**，而這段指引原本無條件叫 ADM 去按它。
										 * dev 第 3 輪 E2E 實測到這個狀態：ADM 看著一句
										 * 「請先按『要求補件』」，而畫面上沒有那顆按鈕。
										 *
										 * 不是死路——`REJECT` 仍可按，且
										 * `ResetSellerTaxProfileToDraft` 的 allowed-from 含
										 * `REJECTED` ⇒ 賣家可以重新填。指引要說的是那條路。
										 */
										availableDecisions.includes(
											SellerTaxReviewDecision.REQUEST_SUPPLEMENT,
										) ? (
											<p className="invoice-section__hint">
												這位賣家的統一編號或法定名稱是空的，「核定應開統一發票」暫時不能按——請先按「要求補件」，等他補齊後再核定。
											</p>
										) : (
											<p className="invoice-section__hint">
												這位賣家的統一編號或法定名稱是空的，「核定應開統一發票」暫時不能按；目前狀態也沒有「要求補件」可用。請改按「駁回」並寫明缺什麼，賣家會回到可編輯狀態、補齊後重新送審。
											</p>
										)
									) : null}
									<div className="invoice-decision-actions">
										{availableDecisions.map((decision) => (
											<Button
												key={decision}
												variant={
													decision === SellerTaxReviewDecision.REVOKE ||
													decision === SellerTaxReviewDecision.REJECT
														? "danger"
														: "outline"
												}
												size="sm"
												disabled={
													(decision ===
														SellerTaxReviewDecision.APPROVE_INVOICE_REQUIRED &&
														identityIncomplete) ||
													taxDecisionBlockedByReceiptAuth(
														decision,
														profile.receiptAuthorized,
													)
												}
												onClick={() => {
													setBanner(null);
													setValidUntil("");
													setBasisDocumentId("");
													setOpenDecision(decision);
												}}
											>
												{taxDecisionLabel(decision)}
											</Button>
										))}
									</div>
								</>
							)}
						</section>

						<section className="invoice-section">
							<h2 className="invoice-section__title">延長申請</h2>
							<p className="invoice-section__hint">
								一筆申請只能延長一次。核准之後新的期限從「原到期日」起算，不是從今天起算——
								所以一個快到期的案子不會因為延長而變成完整的 30 天。
							</p>

							{profile.pendingExtensionRequest ? (
								<div className="invoice-extension-request">
									<strong>
										賣家已於{" "}
										{formatDateTime(
											profile.pendingExtensionRequest.requestedAt,
										)}{" "}
										申請延長，尚未裁決
									</strong>
									<p className="invoice-extension-request__reason">
										理由：{profile.pendingExtensionRequest.reason || "（未填）"}
									</p>
								</div>
							) : null}
							<div className="invoice-decision-actions">
								{canApproveExtension ? (
									<Button
										variant="outline"
										size="sm"
										onClick={() => {
											setBanner(null);
											setExtendDays(String(TAX_EXTENSION_DEFAULT_DAYS));
											setOpenExtension(TAX_EXTENSION_APPROVE);
										}}
									>
										核准延長
									</Button>
								) : null}
								<Button
									variant="ghost"
									size="sm"
									onClick={() => {
										setBanner(null);
										setOpenExtension(TAX_EXTENSION_REJECT);
									}}
								>
									駁回延長申請
								</Button>
							</div>
							{canApproveExtension ? null : (
								<p className="invoice-field__note">
									目前不能核准延長：核准只適用於「已具結待審」且尚未延長過的申請。
								</p>
							)}
						</section>

						{hasIntakeLimit ? (
							<section className="invoice-section">
								<h2 className="invoice-section__title">待審期間接單限制</h2>
								<div className="invoice-fields">
									<Field label="訂單筆數" full>
										{intakeLimit?.orderCountUsed || "0"} /{" "}
										{intakeLimit?.orderCountLimit} 筆
									</Field>
									<Field label="訂單金額" full>
										{formatAmount(intakeLimit?.amountUsed)} /{" "}
										{formatAmount(intakeLimit?.amountLimit)}
									</Field>
								</div>
								{intakeLimit?.blocked ? (
									<p className="invoice-page-notice" role="status">
										<MaterialSymbol name="warning" />
										這個賣家已達待審期間的接單上限，新訂單會在結帳時被擋下。
									</p>
								) : null}
							</section>
						) : null}
					</aside>
				</div>
			)}

			{openDecision ? (
				<InvoiceActionDialog
					idPrefix="adm-tax-decision"
					icon="balance"
					title={taxDecisionLabel(openDecision)}
					description={`store_id ${storeId}　目前狀態：${codeLabel(
						SELLER_TAX_REVIEW_STATUS_LABEL,
						reviewStatus,
					)}`}
					consequence={TAX_DECISION_CONSEQUENCE[openDecision]}
					reasonLabel="審核意見"
					reasonPlaceholder="例：檢附之稅籍登記文件與申請統編一致，核定應開統一發票。"
					variant={
						openDecision === SellerTaxReviewDecision.REVOKE ||
						openDecision === SellerTaxReviewDecision.REJECT
							? "danger"
							: "primary"
					}
					confirmText={
						TAX_DECISION_NEEDS_CONFIRM.has(openDecision)
							? openDecision === SellerTaxReviewDecision.REVOKE
								? "我了解撤銷會讓這個賣家的已核定資格立刻失效。"
								: "我了解核定免用統一發票會改變買家在結帳頁看到的法定告知。"
							: undefined
					}
					submitLabel="送出決策"
					submittingLabel="送出中…"
					failureText="決策沒有送出，這筆申請沒有變動；請確認網路狀態後再送一次。"
					extraFields={(disabled) =>
						TAX_DECISION_APPROVALS.has(openDecision) ? (
							<>
								<div className="adm-operation-field adm-operation-field--wide">
									<label htmlFor="adm-tax-decision-valid-until">
										核定效期到（選填）
									</label>
									<input
										id="adm-tax-decision-valid-until"
										type="date"
										value={validUntil}
										onChange={(event) => setValidUntil(event.target.value)}
										disabled={disabled}
									/>
									<span className="adm-operation-field-note">
										留空 ＝
										由後端從核定時點起算一年。填了就必須晚於今天，否則會被擋下。
									</span>
								</div>
								{openDecision ===
								SellerTaxReviewDecision.APPROVE_INVOICE_EXEMPT ? (
									<div className="adm-operation-field adm-operation-field--wide">
										<label htmlFor="adm-tax-decision-basis">
											核定依據文件（選填）
										</label>
										<select
											id="adm-tax-decision-basis"
											value={basisDocumentId}
											onChange={(event) =>
												setBasisDocumentId(event.target.value)
											}
											disabled={disabled}
										>
											<option value="">不指定</option>
											{(profile?.documents ?? []).map((doc) => (
												<option key={doc.documentId} value={doc.documentId}>
													{doc.kind ? `${doc.kind}｜` : ""}

													{doc.fileName || `文件 #${doc.documentId}`}
												</option>
											))}
										</select>
										<span className="adm-operation-field-note">
											只能從這個賣家自己上傳的稅籍文件裡挑（後端會驗歸屬）。選項的值直接對應該檔的
											objectKey 與 assetId，不是手打的。
										</span>
									</div>
								) : null}
							</>
						) : null
					}
					onClose={() => setOpenDecision(null)}
					onSubmit={async (reason) => {
						await submitting(() =>
							decide.mutateAsync({ decision: openDecision, reason }),
						);
					}}
				/>
			) : null}

			{openExtension ? (
				<InvoiceActionDialog
					idPrefix="adm-tax-extension"
					icon="event_upcoming"
					title={
						openExtension === TAX_EXTENSION_APPROVE
							? "核准延長審核期限"
							: "駁回延長申請"
					}
					description={`store_id ${storeId}　目前期限：${formatDateTime(
						profile?.deadlineAt,
					)}`}
					consequence={
						openExtension === TAX_EXTENSION_APPROVE
							? "新的期限從原到期日起算，一筆申請只能延長一次。狀態不會改變。"
							: "期限維持原樣。駁回本身也會寫入稽核紀錄。"
					}
					reasonLabel="裁決理由"
					reasonPlaceholder="例：賣家已提供國稅局收件回執，同意延長。"
					submitLabel={
						openExtension === TAX_EXTENSION_APPROVE ? "核准延長" : "駁回申請"
					}
					submittingLabel="送出中…"
					variant={
						openExtension === TAX_EXTENSION_APPROVE ? "primary" : "danger"
					}
					failureText="裁決沒有送出，期限沒有變動；請確認網路狀態後再送一次。"
					validateExtra={() => {
						if (openExtension !== TAX_EXTENSION_APPROVE) return null;
						const days = Number(extendDays.trim());
						if (!Number.isInteger(days) || days < 1) {
							return "延長天數請填 1 以上的整數。";
						}
						if (days > TAX_EXTENSION_MAX_DAYS) {
							return `延長天數上限是 ${TAX_EXTENSION_MAX_DAYS} 天。`;
						}
						return null;
					}}
					extraFields={(disabled) =>
						openExtension === TAX_EXTENSION_APPROVE ? (
							<div className="adm-operation-field">
								<label htmlFor="adm-tax-extension-days">延長天數</label>
								<input
									id="adm-tax-extension-days"
									type="text"
									inputMode="numeric"
									value={extendDays}
									onChange={(event) => setExtendDays(event.target.value)}
									disabled={disabled}
								/>
								<span className="adm-operation-field-note">
									1–{TAX_EXTENSION_MAX_DAYS} 天，預設{" "}
									{TAX_EXTENSION_DEFAULT_DAYS} 天。
								</span>
							</div>
						) : null
					}
					onClose={() => setOpenExtension(null)}
					onSubmit={async (reason) => {
						await submitting(() =>
							decideExtension.mutateAsync({
								decision: openExtension,
								reason,
							}),
						);
					}}
				/>
			) : null}
		</div>
	);
}
