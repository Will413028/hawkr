"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import { type ReactNode, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	CERTIFICATION_TYPE_LABEL,
	codeLabel,
	PRODUCT_CERTIFICATION_ACTOR_ROLE_LABEL,
	PRODUCT_CERTIFICATION_REJECT_REASON_LABEL,
	PRODUCT_CERTIFICATION_REVIEW_ACTION_LABEL,
	PRODUCT_CERTIFICATION_SOURCE_LABEL,
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	PRODUCT_CERTIFICATION_VALIDITY_LABEL,
	ProductCertificationStatus,
	ProductCertificationValidity,
	toOptions,
} from "@/lib/codemeta";
import {
	type CertificationApplicationWire,
	factoryScopeHint,
	formatDateTime,
	type GovVerifyResultWire,
	govPayloadMatchesApplication,
	isFactoryScopedCertification,
	isProductGone,
	productGoneLabel,
	REVERIFY_HINT,
	reverifyResultNoticeText,
	reviewerDisplay,
	reviewIdempotencyKey,
	reviewResultNoticeText,
} from "@/lib/product-certifications";
import {
	type CertificationReviewAction,
	getCertificationApplication,
	listCertificationAuditLogs,
	newIdempotencyKey,
	reverifyCertificationApplication,
	reviewCertificationApplication,
} from "@/services/product-certifications";
import {
	type GovernanceNotice,
	GovernanceNoticeBanner,
	governanceErrorText,
} from "../../merchants/[id]/GovernanceNotice";
import {
	CertificationStatusPill,
	CertificationValidityPill,
	GovVerifyStatePill,
} from "../badges";
import "@/styles/pages/ProductCertifications.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 商品農產品認證申請詳情與裁決（backend-payuni 19 號設計書 §9.2 / §9.3）。
 *
 * 形狀借 `/admin/merchants/[id]`（申請詳情 ＋ approve/reject 帶原因 ＋ 讀 audit-logs），
 * 治理提示元件直接沿用該頁的 `GovernanceNotice`——同一種「操作結果就地承載」的責任，
 * 不再複製第二份。
 *
 * ## 這一頁存在的理由（不是列表的展開版）
 *
 * 🔴 **政府查驗結果 `govVerifyPayload` 是核准的唯一依據**。少了它，ADM 只看得到
 * 「賣家自己填的號碼」與一個綠色徽章，那不是審核。三個特別要命的細節：
 *
 * - **3703 CAS 的編號是工廠級的**：實測一個 `Emblem_ID` 對到 173 個產品（19 §8.5）。
 *   查得到只證明該工廠有 CAS 資格，不證明**這個商品**是 CAS 產品 ⇒ 一律人工審，
 *   而且必須把「共 N 項」與品項清單攤在審核者眼前。
 * - **`UNAVAILABLE` 不是 `NOT_FOUND`**：政府 API 金鑰失效時回 HTTP 200 但無資料，
 *   舊實作把它讀成「查無此證書」（19 §8.4 防線 1）。把兩者混為一談，金鑰一出事
 *   全站認證都會被駁回。
 * - **編號自我校驗**：政府 API 會忽略未知查詢參數並回傳整個資料集（實測 1000 筆），
 *   所以回應裡的編號必須等於送查的編號。後端擋了一層，這裡再顯示一次給人看。
 *
 * 🔴 **「重新查驗」是 migration 產生的 legacy 申請唯一的查驗觸發方式**：那些列沒有
 * MQ 事件，而 19 §8.3 明令禁止用排程去掃 PENDING。沒有這顆按鈕，它們永遠停在待查驗。
 */

/** 裁決動作 → 它在畫面上的說明（不是碼→標籤對照，所以不走生成物）。 */
const ACTION_DESCRIPTIONS: Record<CertificationReviewAction, string> = {
	approve:
		"標章立即出現在商品頁；同類型現行有效的那一筆會被標記為「已被新號碼取代」。",
	reject: "賣家會看到「未通過」與你填的公開原因，標章不會出現。必填原因代碼。",
	revoke: "已核准的標章立即從商品頁消失，用於事後發現證書失效。必填原因代碼。",
};

/** 需要原因代碼的動作（缺少時後端回業務碼 3028）。 */
const REASON_REQUIRED: Record<CertificationReviewAction, boolean> = {
	approve: false,
	reject: true,
	revoke: true,
};

/**
 * 目前狀態下允許的裁決動作。
 *
 * 對齊 19 §5.1 的合法轉移表：只有 `SUBMITTED` 可以核准／駁回，只有 `APPROVED` 可以
 * 撤銷，四個終態（REJECTED / WITHDRAWN / SUPERSEDED / REVOKED）不再轉出。
 * ⚠️ 這是**呈現層的收斂**，不是授權——後端仍以 3027 擋非法轉移。
 */
function allowedActions(status: string): CertificationReviewAction[] {
	if (status === ProductCertificationStatus.SUBMITTED) {
		return ["approve", "reject"];
	}
	if (status === ProductCertificationStatus.APPROVED) {
		return ["revoke"];
	}
	return [];
}

function KeyValue({ label, children }: { label: string; children: ReactNode }) {
	return (
		<div className="cert-kv">
			<span className="cert-kv-label">{label}</span>
			<span className="cert-kv-value">{children}</span>
		</div>
	);
}

/**
 * 政府查驗結果面板。
 *
 * `rawExcerpt` 用 `JSON.stringify` 塞進 `<pre>` 的**文字節點**——3702 的
 * `ParentTraceCode` 內含未跳脫的 HTML（`<a href="…">`，19 §8.4 附帶陷阱），
 * 走文字節點由 React 自動 escape，絕不可改成 dangerouslySetInnerHTML。
 */
function GovVerifyPanel({
	application,
}: {
	application: CertificationApplicationWire;
}) {
	const payload: GovVerifyResultWire | null = application.govVerifyPayload;
	const factoryScoped = isFactoryScopedCertification(
		application.certificationType,
	);
	const mismatched = !govPayloadMatchesApplication(application, payload);

	return (
		<div className="detail-card">
			<div className="detail-card-header">
				<MaterialSymbol name="account_balance" />
				政府查驗結果
				<span style={{ marginLeft: "auto" }}>
					<GovVerifyStatePill state={application.govVerifyState} />
				</span>
			</div>
			<div className="detail-card-body cert-detail-stack">
				{factoryScoped && (
					<div className="cert-note cert-note--warning">
						<MaterialSymbol name="warning" />
						<div>
							{factoryScopeHint(payload?.matchedCount)}
							{payload?.productSummary ? (
								<div style={{ marginTop: 6 }}>
									涵蓋品項：{payload.productSummary}
								</div>
							) : null}
						</div>
					</div>
				)}

				{mismatched && (
					<div className="cert-note cert-note--danger" role="alert">
						<MaterialSymbol name="error" />
						<div>
							查驗結果的編號／類型與這筆申請不一致（申請：
							{application.certificationType} / {application.certificateNo}
							；查驗回應：{payload?.certType} / {payload?.certNo}）。
							這份資料不能當作本筆申請的依據，請重新查驗後再裁決。
						</div>
					</div>
				)}

				{payload ? (
					<>
						<div className="cert-kv-grid">
							<KeyValue label="業者名稱">{payload.holderName || "-"}</KeyValue>
							<KeyValue label="業者地址">
								{payload.holderAddress || "-"}
							</KeyValue>
							<KeyValue label="驗證機構">{payload.issuerName || "-"}</KeyValue>
							<KeyValue label="涵蓋品項">
								{payload.productSummary || "-"}
							</KeyValue>
							{/* statusRaw 是政府資料庫的原文（3701 才有：通過／結束／終止／暫終），
							    原樣顯示不做翻譯——它本來就是中文，翻譯只會失真。 */}
							<KeyValue label="狀態原文">{payload.statusRaw || "-"}</KeyValue>
							<KeyValue label="效期">{payload.expiresOn || "-"}</KeyValue>
							<KeyValue label="查驗時間">
								{formatDateTime(payload.queriedAt)}
							</KeyValue>
							<KeyValue label="命中列數">{payload.matchedCount}</KeyValue>
							<KeyValue label="查得到編號">
								{payload.found ? "是" : "否"}
							</KeyValue>
							{/* 🔴 這一欄是「能不能核准」的主要依據，不是上方那顆徽章：
							    govVerifyState 把 EXPIRED／TERMINATED／SUSPENDED 三值塌成
							    一個 EXPIRED（有損），只看徽章分不出「已終止」（不可逆，
							    要求換一張）與「暫時終止」（日後可能恢復，不必換號）。 */}
							<KeyValue label="查驗結論">
								{payload.validity ? (
									<CertificationValidityPill validity={payload.validity} />
								) : (
									"-"
								)}
							</KeyValue>
							<KeyValue label="送查編號">
								{payload.certType} / {payload.certNo}
							</KeyValue>
						</div>

						{/* 🔴「無法判讀」是故障不是結論，處置與其他失效值完全相反：
						    其他值是「查證後認定不能核准」，這個值是「我們根本沒查出結論」。
						    不特別說明的話，審核者會把它讀成查證失敗而直接駁回一張可能
						    完全正常的證書——與 UNAVAILABLE 曾被當成 NOT_FOUND
						    （19 §8.4 防線 1）是同一個坑，只是換了一個欄位重演。 */}
						{payload.validity === ProductCertificationValidity.UNKNOWN && (
							<div className="cert-note cert-note--warning" role="status">
								<MaterialSymbol name="help" />
								<div>
									{/* 標籤文字也從生成物取：這句提示點名的就是上面那顆徽章，
									    把「無法判讀」抄成字面值，SSOT 改字時兩邊會對不起來。 */}
									<strong>
										「
										{codeLabel(
											PRODUCT_CERTIFICATION_VALIDITY_LABEL,
											ProductCertificationValidity.UNKNOWN,
										)}
										」不是查驗結論。
									</strong>
									政府回應裡沒有狀態欄，或狀態值不在我們已知的值域，代表
									<strong>我們沒能判讀出這張證書現在還算不算數</strong>
									，並不代表這張證書有問題。
									請先按上方「重新查驗」；若重查後仍然無法判讀，請以其他方式
									人工查證並在內部註記寫明依據，
									<strong>不要據此駁回</strong>。
								</div>
							</div>
						)}

						{payload.rawExcerpt != null && (
							<details>
								<summary className="cert-raw-summary">
									政府原始回應（已由後端修剪）
								</summary>
								<pre className="cert-raw-excerpt">
									{JSON.stringify(payload.rawExcerpt, null, 2)}
								</pre>
							</details>
						)}
					</>
				) : (
					<div className="cert-note cert-note--muted">
						<MaterialSymbol name="info" />
						<div>
							尚無政府查驗結果。可能是背景查驗還沒回來（待查驗）、這個類型不支援
							線上查驗，或政府 API 當時不可用。
							<strong>沒有查驗結果就核准等於沒有依據</strong>
							——請先按上方「重新查驗」，或以其他方式人工查證後在內部註記寫明依據。
						</div>
					</div>
				)}

				<div className="cert-note cert-note--info">
					<MaterialSymbol name="lightbulb" />
					<div>
						查驗通過不等於可以核准：政府 API 只驗「這個號碼存在且有效」，不驗
						「這張證書屬於這個賣家、涵蓋這個商品」。業者名稱與涵蓋品項要自己比對。
					</div>
				</div>
			</div>
		</div>
	);
}

export default function ProductCertificationDetailPage() {
	const params = useParams<{ id: string }>();
	const applicationId = params.id;
	const router = useRouter();
	const queryClient = useQueryClient();

	const [notice, setNotice] = useState<GovernanceNotice | null>(null);
	const [dialogOpen, setDialogOpen] = useState(false);
	const [action, setAction] = useState<CertificationReviewAction>("approve");
	const [reasonCode, setReasonCode] = useState("");
	const [publicReason, setPublicReason] = useState("");
	const [internalNote, setInternalNote] = useState("");

	const {
		data: application = null,
		isLoading,
		isError,
		error,
	} = useQuery({
		queryKey: ["product-certifications", "detail", applicationId],
		queryFn: () => getCertificationApplication(applicationId),
		enabled: !!applicationId,
	});

	const { data: auditLogs, isLoading: auditLoading } = useQuery({
		queryKey: ["product-certifications", "audit-logs", applicationId],
		queryFn: () => listCertificationAuditLogs(applicationId),
		enabled: !!applicationId,
	});

	const closeDialog = () => setDialogOpen(false);

	const applyUpdated = (updated: CertificationApplicationWire | null) => {
		if (updated) {
			queryClient.setQueryData(
				["product-certifications", "detail", applicationId],
				updated,
			);
		}
		void queryClient.invalidateQueries({
			queryKey: ["product-certifications", "audit-logs", applicationId],
		});
		void queryClient.invalidateQueries({
			queryKey: ["product-certifications", "queue"],
		});
	};

	const reviewMutation = useMutation({
		mutationFn: () => {
			if (!application) throw new Error("尚未載入申請資料");
			const trimmedPublicReason = publicReason.trim();
			const trimmedInternalNote = internalNote.trim();
			return reviewCertificationApplication(applicationId, action, {
				// 冪等鍵由送出內容決定（不是亂數）：同一個決定重送安全，改任何一個欄位
				// 就自動換一把——後端會比對 request hash，同鑰匙配不同內容是直接拒絕。
				idempotencyKey: reviewIdempotencyKey({
					applicationId,
					action,
					expectedVersion: application.version,
					reasonCode,
					publicReason: trimmedPublicReason,
					internalNote: trimmedInternalNote,
				}),
				expectedVersion: application.version,
				reasonCode: reasonCode || undefined,
				publicReason: trimmedPublicReason || undefined,
				internalNote: trimmedInternalNote || undefined,
			});
		},
		onSuccess: (updated) => {
			applyUpdated(updated);
			setDialogOpen(false);
			setReasonCode("");
			setPublicReason("");
			setInternalNote("");
			setNotice({
				tone: "success",
				text: reviewResultNoticeText(updated?.status),
			});
		},
		onError: (caught) => {
			setNotice({
				tone: "error",
				text: governanceErrorText(caught, "裁決失敗，請稍後再試"),
			});
		},
	});

	const reverifyMutation = useMutation({
		// 冪等鍵每次點擊都換新的：它擋的是「連點兩次打兩次政府 API」，不是「這輩子只能
		// 查一次」。證書可能在申請之後才登記上政府資料庫，同一筆本來就會被查很多次。
		mutationFn: () =>
			reverifyCertificationApplication(
				applicationId,
				newIdempotencyKey("cert-reverify"),
			),
		onSuccess: (updated) => {
			applyUpdated(updated);
			setNotice({
				tone: "success",
				text: reverifyResultNoticeText(
					updated?.govVerifyState,
					updated?.status,
				),
			});
		},
		onError: (caught) => {
			setNotice({
				tone: "error",
				text: governanceErrorText(caught, "重新查驗失敗，請稍後再試"),
			});
		},
	});

	const openDialog = (next: CertificationReviewAction) => {
		setNotice(null);
		setAction(next);
		setReasonCode("");
		setPublicReason("");
		setInternalNote("");
		setDialogOpen(true);
	};

	if (isLoading) {
		return <div style={{ padding: 40, textAlign: "center" }}>載入中…</div>;
	}

	if (isError || !application) {
		return (
			<div style={{ padding: 40, textAlign: "center" }}>
				<p style={{ marginBottom: 16 }}>
					{isError && error instanceof Error
						? `讀取失敗：${error.message}`
						: "找不到這筆認證申請"}
				</p>
				<Button
					variant="primary"
					onClick={() => router.push("/admin/product-certifications")}
				>
					返回佇列
				</Button>
			</div>
		);
	}

	const actions = allowedActions(application.status);
	const reasonRequired = REASON_REQUIRED[action];
	const submitBlocked =
		reviewMutation.isPending || (reasonRequired && !reasonCode);

	return (
		<div>
			<div className="detail-header">
				<div className="detail-header-left">
					<button
						type="button"
						className="btn-back"
						aria-label="返回佇列"
						onClick={() => router.push("/admin/product-certifications")}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="detail-title-section">
						<div className="detail-title-row">
							<h1 className="detail-title">
								{application.productName || "（未命名商品）"}
							</h1>
							{isProductGone(application.productPublicationState) ? (
								<span className="cert-gone-pill">
									{productGoneLabel(application.productPublicationState)}
								</span>
							) : null}
							<CertificationStatusPill status={application.status} />
						</div>
						<div className="detail-meta">
							{codeLabel(
								CERTIFICATION_TYPE_LABEL,
								application.certificationType,
							)}
							　·　證號 {application.certificateNo}　·　申請 #{application.id}
						</div>
					</div>
				</div>
				<div className="detail-header-actions">
					<Button
						variant="outline"
						disabled={reverifyMutation.isPending}
						onClick={() => reverifyMutation.mutate()}
					>
						{reverifyMutation.isPending ? "查驗中…" : "重新查驗"}
					</Button>
					{actions.map((a) => (
						<Button
							key={a}
							variant={a === "approve" ? "primary" : "danger"}
							onClick={() => openDialog(a)}
						>
							{codeLabel(
								PRODUCT_CERTIFICATION_REVIEW_ACTION_LABEL,
								a.toUpperCase(),
							)}
						</Button>
					))}
				</div>
			</div>

			{/*
			 * 配額是「按下去那一刻」花掉的，講在事後的 toast 就太晚了 ——
			 * 所以事前說明必須在按鈕旁邊常駐（Button 元件不吃 title，也不該把
			 * 唯一的說明藏進 hover）。
			 */}
			<div className="cert-note cert-note--info">
				<MaterialSymbol name="info" />
				<div>{REVERIFY_HINT}</div>
			</div>

			{!dialogOpen && <GovernanceNoticeBanner notice={notice} />}

			{actions.length === 0 && (
				<div className="cert-note cert-note--muted">
					<MaterialSymbol name="lock" />
					<div>
						這筆申請已是終態（
						{codeLabel(PRODUCT_CERTIFICATION_STATUS_LABEL, application.status)}
						），不再接受裁決。仍可按「重新查驗」更新政府查驗資料以供存查。
					</div>
				</div>
			)}

			<div className="cert-detail-stack">
				<div className="detail-card">
					<div className="detail-card-header">
						<MaterialSymbol name="list_alt" />
						申請資訊
					</div>
					<div className="detail-card-body">
						<div className="cert-kv-grid">
							<KeyValue label="商品">
								{application.productName || "-"}（#{application.productId}）
								{isProductGone(application.productPublicationState) ? (
									<span className="cert-cell-warn">
										（{productGoneLabel(application.productPublicationState)}）
									</span>
								) : null}
							</KeyValue>
							<KeyValue label="店家 store_id">{application.storeId}</KeyValue>
							<KeyValue label="認證類型">
								{codeLabel(
									CERTIFICATION_TYPE_LABEL,
									application.certificationType,
								)}
							</KeyValue>
							<KeyValue label="證書編號">{application.certificateNo}</KeyValue>
							<KeyValue label="來源">
								{codeLabel(
									PRODUCT_CERTIFICATION_SOURCE_LABEL,
									application.source,
								)}
							</KeyValue>
							<KeyValue label="須人工審核">
								{application.requiresManualReview ? "是" : "否"}
							</KeyValue>
							<KeyValue label="送件時間">
								{formatDateTime(application.submittedAt)}
							</KeyValue>
							{/* 送件者是登入 principal（accounts PK），賣家換手後 storeId 不變而
							    這個值會變——稽核「當時是誰送的」只能靠它。 */}
							<KeyValue label="送件者 account_id">
								{application.submittedByAccountId || "-"}
							</KeyValue>
							<KeyValue label="裁決時間">
								{formatDateTime(application.reviewedAt)}
							</KeyValue>
							<KeyValue label="裁決者 account_id">
								{reviewerDisplay(
									application.reviewedAt,
									application.reviewedByAccountId,
								)}
							</KeyValue>
							<KeyValue label="原因代碼">
								{application.reasonCode
									? codeLabel(
											PRODUCT_CERTIFICATION_REJECT_REASON_LABEL,
											application.reasonCode,
										)
									: "-"}
							</KeyValue>
							<KeyValue label="樂觀鎖版本">{application.version}</KeyValue>
							<KeyValue label="公開原因（賣家看得到）">
								{application.publicReason || "-"}
							</KeyValue>
							<KeyValue label="內部註記（賣家看不到）">
								{application.internalNote || "-"}
							</KeyValue>
						</div>
					</div>
				</div>

				<GovVerifyPanel application={application} />

				<div className="detail-card">
					<div className="detail-card-header">
						<MaterialSymbol name="workspace_premium" />
						同類型現行有效的標章
					</div>
					<div className="detail-card-body">
						{application.currentApproved ? (
							<>
								{/* 🔴 核准這一筆會讓下面那一筆被 SUPERSEDE、標章當場換掉。
								    審核者必須先看得到「我核准之後，哪一張標章會消失」。 */}
								<div className="cert-note cert-note--warning">
									<MaterialSymbol name="swap_vert" />
									<div>
										核准這筆申請會把下面這張現行標章標記為「已被新號碼取代」，
										商品頁上的標章會立即換成新的證號。
									</div>
								</div>
								<div className="cert-kv-grid" style={{ marginTop: 16 }}>
									<KeyValue label="現行證號">
										{application.currentApproved.certificateNo}
									</KeyValue>
									<KeyValue label="認證類型">
										{codeLabel(
											CERTIFICATION_TYPE_LABEL,
											application.currentApproved.certificationType,
										)}
									</KeyValue>
									<KeyValue label="查驗狀態">
										<GovVerifyStatePill
											state={application.currentApproved.govVerifyState}
										/>
									</KeyValue>
									<KeyValue label="核准時間">
										{formatDateTime(application.currentApproved.reviewedAt)}
									</KeyValue>
								</div>
							</>
						) : (
							<div className="cert-note cert-note--muted">
								<MaterialSymbol name="info" />
								<div>
									這個類型目前沒有已核准的標章，核准這筆不會取代任何既有標章。
								</div>
							</div>
						)}
					</div>
				</div>

				<div className="detail-card">
					<div className="detail-card-header">
						<MaterialSymbol name="history" />
						審核紀錄
					</div>
					<div className="detail-card-body">
						{auditLoading ? (
							<div className="cert-audit-empty">載入中…</div>
						) : (auditLogs?.records.length ?? 0) === 0 ? (
							<div className="cert-audit-empty">尚無審核事件</div>
						) : (
							<div className="table-container">
								<table className="data-table">
									<thead>
										<tr>
											<th>時間</th>
											<th>動作</th>
											<th>狀態變化</th>
											<th>操作者</th>
											<th>原因</th>
											<th>版本</th>
										</tr>
									</thead>
									<tbody>
										{auditLogs?.records.map((log) => (
											<tr key={log.id}>
												<td>{formatDateTime(log.createdAt)}</td>
												<td>
													{codeLabel(
														PRODUCT_CERTIFICATION_REVIEW_ACTION_LABEL,
														log.action,
													)}
												</td>
												<td>
													{codeLabel(
														PRODUCT_CERTIFICATION_STATUS_LABEL,
														log.fromStatus,
													)}
													{" → "}
													{codeLabel(
														PRODUCT_CERTIFICATION_STATUS_LABEL,
														log.toStatus,
													)}
												</td>
												<td>
													{/* 帳號 id 只在有值時顯示。原本 null 會補一個
													    「（系統）」，那是碼值時期用來解釋
													    「為什麼沒有 id」的補丁；role 一旦顯示成
													    中文標籤，SYSTEM 那列就成了「系統（系統）」，
													    而 SELLER／ADMIN 若真的缺 id，補上「（系統）」
													    更是直接說錯了是誰做的。 */}
													{codeLabel(
														PRODUCT_CERTIFICATION_ACTOR_ROLE_LABEL,
														log.actorRole,
													)}
													{log.actorAccountId ? (
														<span className="cert-actor-system">
															#{log.actorAccountId}
														</span>
													) : null}
												</td>
												<td>
													{log.reasonCode
														? codeLabel(
																PRODUCT_CERTIFICATION_REJECT_REASON_LABEL,
																log.reasonCode,
															)
														: "-"}
													{log.publicReason ? (
														<span className="cert-cell-sub">
															{log.publicReason}
														</span>
													) : null}
													{log.internalNote ? (
														<span className="cert-cell-sub">
															內部：{log.internalNote}
														</span>
													) : null}
												</td>
												<td>{log.applicationVersion}</td>
											</tr>
										))}
									</tbody>
								</table>
							</div>
						)}
					</div>
				</div>
			</div>

			{dialogOpen && (
				<Dialog
					title={`裁決認證申請 #${application.id}`}
					onClose={closeDialog}
					busy={reviewMutation.isPending}
					footer={
						<>
							<Button
								variant="outline"
								onClick={closeDialog}
								disabled={reviewMutation.isPending}
							>
								取消
							</Button>
							<Button
								variant={action === "approve" ? "primary" : "danger"}
								onClick={() => reviewMutation.mutate()}
								disabled={submitBlocked}
							>
								{reviewMutation.isPending
									? "送出中…"
									: `確認${codeLabel(
											PRODUCT_CERTIFICATION_REVIEW_ACTION_LABEL,
											action.toUpperCase(),
										)}`}
							</Button>
						</>
					}
				>
					<div className="cert-kv-grid">
						<KeyValue label="商品">
							{application.productName || "-"}
							{isProductGone(application.productPublicationState) ? (
								<span className="cert-cell-warn">
									（{productGoneLabel(application.productPublicationState)}）
								</span>
							) : null}
						</KeyValue>
						<KeyValue label="證號">
							{codeLabel(
								CERTIFICATION_TYPE_LABEL,
								application.certificationType,
							)}
							｜{application.certificateNo}
						</KeyValue>
					</div>

					{actions.map((a) => (
						<label key={a} className="cert-decision-option">
							<input
								type="radio"
								name="cert-decision"
								value={a}
								checked={action === a}
								onChange={() => setAction(a)}
							/>
							<span>
								<strong>
									{codeLabel(
										PRODUCT_CERTIFICATION_REVIEW_ACTION_LABEL,
										a.toUpperCase(),
									)}
								</strong>
								{ACTION_DESCRIPTIONS[a]}
							</span>
						</label>
					))}

					<label className="cert-field" htmlFor="cert-reason-code">
						<span>
							原因代碼{" "}
							{reasonRequired ? <span className="required">*</span> : null}
						</span>
						<select
							id="cert-reason-code"
							value={reasonCode}
							onChange={(e) => setReasonCode(e.target.value)}
						>
							<option value="">
								{reasonRequired ? "請選擇原因" : "（不填）"}
							</option>
							{toOptions(PRODUCT_CERTIFICATION_REJECT_REASON_LABEL).map(
								([k, v]) => (
									<option key={k} value={k}>
										{v}
									</option>
								),
							)}
						</select>
						{reasonRequired && (
							<div className="cert-field-hint">
								駁回與撤銷一定要有原因代碼，後端沒收到會回 3028。
							</div>
						)}
					</label>

					<label className="cert-field" htmlFor="cert-public-reason">
						<span>公開原因（賣家看得到）</span>
						<textarea
							id="cert-public-reason"
							value={publicReason}
							onChange={(e) => setPublicReason(e.target.value)}
							placeholder="例：政府資料庫查無此編號，請確認後重新提交"
						/>
					</label>

					<label className="cert-field" htmlFor="cert-internal-note">
						<span>內部註記（只有管理員看得到）</span>
						<textarea
							id="cert-internal-note"
							value={internalNote}
							onChange={(e) => setInternalNote(e.target.value)}
							placeholder="例：已致電賣家確認工廠與商品對應關係"
						/>
					</label>

					<div className="cert-field-hint">
						送出時會帶樂觀鎖版本 {application.version}
						；若這筆在你審核期間被別人改過，後端會以版本衝突擋下來。
					</div>

					<GovernanceNoticeBanner notice={notice} />
				</Dialog>
			)}
		</div>
	);
}
