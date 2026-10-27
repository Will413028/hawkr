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
// 對話框骨架（overlay、面板、標題列）在共用 Dialog 自己的 Dialog.css；這裡的 DelegatedOperations.css 只放
// `.adm-operation-field*` 等欄位與版面 class（58A 陷阱 C07：少 import 不會報錯也不會轉紅）。
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A03｜稅務身分審核詳情（58A §A03，端點 39 `GET /api/v1/admin/invoices/tax_reviews/{store_id}`）。
//
// ## 審核決策已接上（2026-09-02）
//
// 這一頁原本掛著「這一頁目前只能檢視」的告示，理由寫「invoice-service 沒有實作
// 端點 40／41」。**那是錯的。** 兩支 RPC 在
// `services/invoice-service/internal/grpc/tax_profile.go` 的 receiver 清單裡
// （`DecideTaxReviewForAdmin`／`DecideTaxExtensionForAdmin`），dev 實測
// `POST /admin/invoices/tax_reviews/{id}/decisions` 回的是業務層驗證訊息
// （`decision is outside the defined value set`）而不是 5xx。
//
// 沒有這兩顆鍵的代價：賣家送得出 `SUBMITTED`，ADM 看得到佇列卻既不能核准也不能駁回
// ⇒ 整條稅籍生命週期在 ADM 側斷掉。
//
// ## 七個決策：條件不成立時**不渲染**（不是 disabled）
//
// 「決策 × 允許的來源狀態」逐列住在 `features/invoices/admin-actions.ts` 的
// `TAX_DECISION_ALLOWED_FROM`（＝ `tax_profile.go` 的 `taxDecisionAllowedFrom`），
// 有 gate 逐列比對。放寬它等於造一顆按下去必回 9348 的鍵，而 ADM 不會知道為什麼。
//
// ## `expectedVersion` 取自**這一次載入讀到的那筆**
//
// `profile.version`，不是寫死、不是 0。它是樂觀鎖：兩位管理員同時開著這一頁時，
// 後送出的那一位必須拿到 9322 而不是覆蓋掉前一位的決策。
//
// ## 與 58A 的欄位落差（四處，全部是「規格有、DTO 沒有」）
//
// `storeName` / `kycTaxId` / `riskFlags[]` / `decisionHistory[]` 都不在端點 39 的回應
// 上（詳見 `features/invoices/types.ts` 的 `SellerTaxProfileData`）。憑語意造欄名的
// 症狀是渲染 `undefined`，TS 不擋、runtime 不擋（陷阱 T15）⇒ 一律照 DTO 有什麼畫什麼。

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
	// storeId 是 TSID，全程字串——轉 number 會截斷精度（merchants/page.tsx 已記過）。
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

	// 404 + 9320 ＝ 查無此申請。這一類不顯示技術訊息，只給一句話與返回鍵——
	// 詳情頁的 404 對操作者而言就是「這筆不存在」，貼業務碼沒有幫助。
	const notFound =
		isError &&
		error instanceof ApiClientError &&
		error.code === TAX_PROFILE_NOT_FOUND_CODE;

	const profile = data ?? null;
	const identity = profile?.taxIdentity;
	const attestation = profile?.selfIssueAttestation;
	// intakeLimit 在 proto 上是 message 不是 optional ⇒ JSON 永遠是物件。
	// 「非待審期間」的判準是上限為空字串，不是 `=== null`（那永遠不成立）。
	// 判錯的症狀是每一頁都顯示「0 / 0 筆」，而 0 與「沒有限制」是兩件事。
	const intakeLimit = profile?.intakeLimit;
	const hasIntakeLimit = (intakeLimit?.orderCountLimit ?? "") !== "";
	// 同理：條款未接受時整個物件是空字串，不是 null。
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
		// 收到回應 ⇒ 下一次按鍵是新的操作，換一把新的冪等鍵。
		// （逾時／斷線走上面那個 return，鍵保持不變。）
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
			// 🔴 不 refetch：資料沒有變舊，是本來就缺 ⇒ 重整幾次都一樣。
			// 下一步是「要求補件」，所以文案要指到那顆鍵而不是叫人重新整理。
			setOpenDecision(null);
			setBanner({
				tone: "danger",
				text: "這位賣家還沒有填寫統一編號與法定名稱，無法核定為應開統一發票。請改按「要求補件」，等賣家補齊後再核定。",
			});
			return true;
		}
		if (code === TAX_APPROVAL_RECEIPT_AUTH_MISSING_CODE) {
			// 🔴 **要 refetch**（與 9353 相反）：畫面上那顆按鈕已經照
			// `receiptAuthorized` 擋過一次，還能走到這裡代表讀到的那一筆是舊的
			// ——賣家在載入之後取消了授權。重新載入才會讓按鈕跟著變成不可按。
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
			// 🔴 不透傳後端 message：9322 的定版文案是「稅務資格資料已被更新」，
			// 而 9348 是「決策與狀態不相容」——兩者對操作者的下一步相同（重新整理再看），
			// 但後端那兩句混在一起讀不出來。
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
			// 頁面層接住了（dialog 已關、畫面已更新）⇒ 不要再往上丟。
			if (routeBusinessError(caught)) return;
			// 其餘（網路、格式、5xx）交給 dialog 就地顯示：使用者改完可以直接再送一次，
			// 而冪等鍵沒換 ⇒ 重送不會被當成第二次操作。
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
						// 空字串 ＝ 不指定 ⇒ 後端用 approved_at + 365 天。
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

	// 🔴 **所有 hook 都在這一行之上**：`useMutation` 不能排在下面這個提早 return
	//    之後（`lint/correctness/useHookAtTopLevel`）。查無的分支要放在最後。
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
						{/*
						 * 已核定免用統一發票、但賣家還沒簽收據代製授權。
						 *
						 * 🔴 這不是「資料還沒載完」，是一個**正在扣住收據**的狀態：
						 * 決策樹（58 §7.1.3 的 RA 節點）對 `receipt_authorized_at IS NULL`
						 * 一律落 `AWAITING_MANUAL` / `RECEIPT_AUTH_MISSING`，而那些文件
						 * 不會出現在「發票工作」頁面（工作台是 job-first，那條路徑不建
						 * operation）⇒ 沒有人會發現它們卡住。買家端一直顯示「處理中」。
						 *
						 * 出口有兩條，都寫出來：請賣家補簽（根治），或到 A13 逐筆
						 * 「送回自動線」（補救既有的那些）。
						 */}
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
								{/* 統編屬 O 類公開登記資訊，完整 8 碼不遮。KYC 統編不在這支端點的
								    回應上，所以「兩個統編分兩欄」只畫得出申請統編這一欄。 */}
								<Field label="申請統編">{identity?.taxId || "—"}</Field>
								<Field label="審核期限">
									{formatDateTime(profile.deadlineAt)}
								</Field>
								{/*
								 * 收據代製授權（S05）。畫在「賣家簽得了的每一個狀態」下
								 * （`RECEIPT_AUTH_SIGNABLE_STATUSES` ＝ 端點 29 的 allowed-from）。
								 *
								 * 🔴 **不能只在 `APPROVED_INVOICE_EXEMPT` 畫**：決策按鈕只在
								 * `UNDER_REVIEW`／`SELF_ISSUE_PENDING` 按得到，兩個條件互斥
								 * ⇒ ADM 在按下核准之前永遠看不到這一欄，而它現在正是
								 * 「核定免用統一發票」的前置條件（T08／9354）。
								 *
								 * 反過來，不在那張表裡的狀態（草稿／已駁回／已撤銷／逾期／
								 * 核定應開統一發票）下賣家簽不了，畫出「尚未授權」會被讀成
								 * 缺件而其實不是。
								 */}
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
													{/* kind 的值域（TAX_REGISTRATION / EXEMPTION_CERT /
													    ID_DOCUMENT / OTHER）不是 CodeMeta enum ⇒ 查不到中文。
													    這裡原樣顯示碼值，不自己寫一張四筆的對照表——那會是
													    一份沒有 SSOT 的第二碼表。 */}
													<td className="invoice-code">{doc.kind || "—"}</td>
													<td>{doc.fileName || "—"}</td>
													<td>{formatDateTime(doc.uploadedAt)}</td>
													<td>
														{/* 路徑由 invoice 經 gateway 回傳（fileDownloadPath），不自己組；
														    走同源 BFF（/api/bff 前綴）：Authorization 由 BFF route 在 server 端注入。 */}
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
										{/*
										 * ⚠️ `acceptedByAccountId` 目前恆為空字串：seller_tax_profiles
										 * 只有 attestation_version ＋ attested_at 兩欄，沒有存簽署人。
										 * 簽署人**有**落庫，在 invoice_status_history 的
										 * actor_account_id（端點 19 與 22 都會寫）⇒ 要查是誰簽的
										 * 走稽核主控台，不是這一格。這裡明說出處，不留一個看起來
										 * 像「沒有人簽」的破折號。
										 */}
										<Field label="接受人 account_id">
											{attestation?.acceptedByAccountId ||
												"見稽核主控台的狀態歷程"}
										</Field>
										{/*
										 * 🔴 與上面的「已接受／未接受」是**兩個不同的問題**，不可合併：
										 * 上面問「條款簽過沒」（termsVersion 非空），這裡問「自行開票
										 * 責任現在生效中沒」（後端＝ review_status === SELF_ISSUE_PENDING）。
										 * 裁決 C 之後 SUBMITTED 的賣家已簽但責任未生效，兩列會不同——
										 * 若這兩列永遠一致，就是後端的 effective 又退回讀 attested_at 了。
										 */}
										<Field label="自行開票責任">
											{attestation?.effective ? "生效中" : "尚未生效"}
										</Field>
									</>
								) : null}
							</div>
						</Section>

						{/*
						 * 🔴 這裡**不是**決策歷程（backend#3053 那一輪的稽查）。
						 *
						 * 後端的 `review_note` 是 seller_tax_profiles 上的**單一 text 欄位**，
						 * 只有補件／駁回／延長／撤銷／維持審核中這五種決策會寫它，而且是
						 * **覆蓋**不是追加；核准與重開根本不寫。58A 給 `admNotes[]` 的定位
						 * 也是「補件時的說明清單」（賣家端 S02 讀它），不是審核歷程。
						 *
						 * 舊 hint 寫「追加不覆蓋：每一筆備註都保留」—— 那描述的是一個
						 * 不存在的機制。dev 實測：同一個賣家做過 7 次帶理由的決策，
						 * 這一區顯示 0 筆，而理由全在 invoice_admin_audit_logs 裡。
						 * 完整歷程要等後端把稽核紀錄下發（DTO 目前沒有 decisionHistory[]，
						 * 見 features/invoices/types.ts 的落差清單）。
						 */}
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
									{/*
									 * 核定免用統一發票之前還有一步，而那一步不在 ADM 手上。
									 *
									 * 🔴 這一段 2026-09-05 反過來了（`DEC-20260905-015836`）：
									 * 端點 29 的 allowed-from 放寬到審核期間 ⇒ 賣家在核定之前
									 * 就簽得了 ⇒ 端點 40 新增 T08 守衛，未授權時核定回 9354。
									 * 舊註解寫的「刻意不 disable，否則會死結」已經不成立。
									 *
									 * 用 disabled 而不是不渲染（同 `identityIncomplete` 的慣例）：
									 * 狀態機層不成立的決策**不存在**，而這一顆存在、只是現在按
									 * 不得——藏起來會讓 ADM 找不到自己要的那顆鍵，也看不到原因。
									 *
									 * ⚠️ ADM 的下一步是**通知賣家**，不是補件——這一格 ADM 自己
									 * 解決不了，文案要指到賣家那一步，不能只說「不能按」。
									 */}
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

						{/*
						 * 延長裁決（端點 41）。
						 *
						 * 核准只在「已具結待審 ＋ 還沒延長過」時成立（後端 DecideExtension 的
						 * 兩道守衛，否則 9321／9323）；駁回沒有狀態守衛——「駁回了一個延長申請」
						 * 本身就是一次需要留痕的 ADM 行為，所以它常駐。
						 */}
						<section className="invoice-section">
							<h2 className="invoice-section__title">延長申請</h2>
							<p className="invoice-section__hint">
								一筆申請只能延長一次。核准之後新的期限從「原到期日」起算，不是從今天起算——
								所以一個快到期的案子不會因為延長而變成完整的 30 天。
							</p>
							{/*
							 * 🔴 待裁決的申請。**這一塊是「現在該不該按核准」的唯一線索**：
							 * 端點 20 只寫一列 invoice_status_history、不碰 seller_tax_profiles
							 * 的任何一欄 ⇒ 在 pendingExtensionRequest 出現之前（2026-09-05），
							 * 申請過與沒申請過的賣家在這一頁完全長得一樣，A02 也分不出來
							 * （filter=UNDER_REVIEW 把兩者混在同一頁）。dev 實測有一筆掛兩天沒人看見。
							 *
							 * ⚠️ 它為 null 時**不隱藏下面的按鈕**：核准的判準是
							 * taxExtensionApprovable（狀態 ＋ 次數），與「有沒有人在等」是兩件事。
							 * ADM 主動延長一個沒開口的賣家仍然合法。
							 */}
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
													{/* kind 設計上恆為空（平台從未收集過文件分類，見 backend
													    proto_field_producer_test.go 的白名單）⇒ 直接串接會渲染成
													    前導的「｜檔名」。有值才補分隔線。 */}
													{doc.kind ? `${doc.kind}｜` : ""}
													{/* fileName 可能為空（該 asset 沒有 asset_reference ⇒ media 的
													    BatchGetAssetMetadata 查不到它，見 backend #3149）。同頁表格
													    對空值有 `|| "—"`，這裡沒有的話會渲染出「完全空白但選得下去」
													    的選項——審核員無從分辨，選了之後畫面看起來仍像「不指定」。 */}
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
