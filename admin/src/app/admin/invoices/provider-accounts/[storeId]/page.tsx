"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { useParams, useRouter } from "next/navigation";
import type React from "react";
import { useState } from "react";
import { governanceErrorText } from "@/app/admin/merchants/[id]/GovernanceNotice";
import { Button, StatusBadge } from "@/components/UI";
import { PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import {
	createProviderIdempotencyKey,
	DEACTIVATABLE_STATUSES,
	HEALTH_CHECK_HIDDEN_STATUSES,
	PROVIDER_ACCOUNTS_ENDPOINT,
	PROVIDER_CREDENTIAL_INVALID_CODE,
	PROVIDER_NOT_FOUND_CODE,
	PROVIDER_STATE_CONFLICT_CODES,
	PROVIDER_SYNC_TIMEOUT_MS,
	PROVIDER_UNAVAILABLE_CODE,
	type ProviderVerifyResult,
	providerConflictText,
	providerVerifyResultOf,
	REACTIVATABLE_STATUSES,
	REVOCABLE_STATUSES,
} from "@/features/invoices/provider-actions";
import type {
	DeactivateProviderAccountResultData,
	InvoiceEnvelope,
	RevokeProviderAccountResultData,
	SellerProviderAccountData,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
	InvoiceProviderAccountStatus,
} from "@/lib/codemeta";
import { formatDateTime, formatTaipeiDateTime } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
import ProviderDeactivateDialog from "./ProviderDeactivateDialog";
import ProviderReactivateDialog from "./ProviderReactivateDialog";
import ProviderRevokeDialog from "./ProviderRevokeDialog";
// 對話框骨架（`.adm-operation-dialog*`）在共用 Dialog 自己的 Dialog.css；`.adm-operation-field*`／
// `.adm-operation-error` 住在 `DelegatedOperations.css`（頁面級樣式表），少了這一行**不會報錯也不會轉紅**。
// A10（audit-logs）與 A11（issuances/new）都是同樣兩份一起 import。
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A04（詳情）｜AMEGO 串接監控（58A §A04，端點 43 `GET .../provider_accounts/{store_id}`）。
//
// ## 五顆治理 CTA 都在（2026-08-31 補齊）
//
// 端點 44（重新測試）／45（停用）／66（恢復）／67（撤銷）在 invoice-service 都已實作
// （`internal/grpc/provider_account.go` 的四支 `*ForAdmin` receiver 方法），Gateway 的
// handler 與路由也齊了（`invoice_admin.go` ＋ `routes_invoice.go` 的
// `AdminInvoiceProviderHealth`／`…Deactivate`／`…Reactivations`／`…Revocations`）
// ⇒ 舊版那則「這一頁目前只能檢視」的告示已經過期，連同四顆缺席的按鈕一起補上。
//
// 58A 花了整整一節論證「恢復（66）與撤銷（67）非做不可」：`SUSPENDED` 在它們補上之前
// **沒有任何出邊** —— 按下停用之後該賣家的 AMEGO 帳戶回不來也撤銷不了，之後每一筆訂單
// 只能走人工開立；連帶讓 `REVOKED` 不可達，而 `uq_ipa_fingerprint` 的 partial 述詞正是
// 靠「舊列已 REVOKED」讓賣家關店重開時綁回同一組憑證。
//
// ## 寫入一律走 `request()` 不走 `api.post`
//
// gateway 的 `middleware.RequireIdempotencyKey` 掛在整個 `/admin/invoices` 群組上，
// POST 少了 `Idempotency-Key` header 一律 400，而 `api.post` 不收自訂 header。
// BFF 的轉送白名單已含 `idempotency-key`（`bff-boundary.ts`）。
//
// ## 端點 44／66 的逾時是 130 秒不是 10 秒
//
// 兩支都在請求路徑上同步等 AMEGO（58A 陷阱 T10）。用預設的 10 秒，症狀是「按了說逾時，
// 重新整理卻看到結果已經變了」——後端仍在跑而且結果會落庫。
//
// ## 與 58A 的欄位落差（三塊，全部是「規格有、DTO 沒有」）
//
// `recentHealthChecks[≤10]`／`trackBalance`／`storeName`＋`taxReviewStatus` 都不在端點 43
// 的回應上（見 `features/invoices/types.ts` 的 `SellerProviderAccountData`）。
// ⇒「最近檢查」與「字軌餘量」兩個 Section 本波不畫。**不是「查不到所以顯示 0」**：
// 畫一張永遠空的卡等於告訴管理員「這個賣家沒有檢查紀錄」，而那是一句我們無從得知的話。

type OpenDialog = "deactivate" | "reactivate" | "revoke" | null;

/** 已完成的治理動作（端點 45／67 共用一個形狀，時間欄各自不同）。 */
type Completion =
	| { kind: "deactivate"; data: DeactivateProviderAccountResultData }
	| { kind: "revoke"; data: RevokeProviderAccountResultData };

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

/**
 * 頁面級的結果橫幅。三個 tone 各自沿用既有的 class，**不新增色值**
 * （`tests/admin-color-ratchet.test.ts` 會擋新 hex，而這三個 tone 本來就都有既有樣式）。
 *
 * `danger` 用 `role="alert"`（要打斷），`success`／`warning` 用 `role="status"`
 * ＋ `aria-live="polite"`（不打斷，但兩分鐘的等待結束時要說話）。
 *
 * ⚠️ success 走 `Invoice.css` 自己的 `.invoice-success-notice` 而不是借
 * `.adm-authorization-notice`：後者住在 `DelegatedOperations.css`，一旦哪天這一頁不再
 * import 那張表，借來的 class 會變成零樣式而**不會有任何東西轉紅**（陷阱 C07）。
 */
function ResultBanner({ result }: { result: ProviderVerifyResult | null }) {
	if (!result) return null;
	if (result.tone === "danger") {
		return (
			<div className="invoice-danger-notice" role="alert">
				<MaterialSymbol name="error" />
				<div>{result.text}</div>
			</div>
		);
	}
	if (result.tone === "warning") {
		return (
			<div className="invoice-page-notice" role="status" aria-live="polite">
				<MaterialSymbol name="info" />
				<div>{result.text}</div>
			</div>
		);
	}
	return (
		<div className="invoice-success-notice" role="status" aria-live="polite">
			<MaterialSymbol name="check_circle" />
			<div>{result.text}</div>
		</div>
	);
}

export default function ProviderAccountDetailPage() {
	const router = useRouter();
	const params = useParams<{ storeId: string }>();
	const storeId = String(params?.storeId ?? "");

	const [openDialog, setOpenDialog] = useState<OpenDialog>(null);
	const [banner, setBanner] = useState<ProviderVerifyResult | null>(null);
	const [completion, setCompletion] = useState<Completion | null>(null);

	/**
	 * 冪等鍵。**送出失敗不換**（陷阱 T28），但**收到回應之後要換**：
	 * `uq_invadmaudit_client_request` 是 `(actor, action, request_id)` 的唯一索引而插入走
	 * `ON CONFLICT DO NOTHING` ⇒ 沿用同一把鍵的第二次操作會靜靜地不寫稽核列。
	 */
	const [idempotencyKey, setIdempotencyKey] = useState(
		createProviderIdempotencyKey,
	);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "providerAccount", storeId],
		enabled: storeId !== "",
		queryFn: async () => {
			const res = await request<InvoiceEnvelope<SellerProviderAccountData>>(
				`${PROVIDER_ACCOUNTS_ENDPOINT}/${encodeURIComponent(storeId)}`,
				{ method: "GET" },
			);
			return res?.data ?? null;
		},
	});

	// 404 + 9330 ＝ 這個賣家尚未建立 AMEGO 串接。與列表的 `NOT_STARTED` 不衝突：
	// 列表是全平台掃一遍所以需要一個「還沒開始」的值，單筆查一個不存在的資源就是 404。
	const notFound =
		isError && error instanceof ApiClientError && error.code === 9330;

	const back = () => router.push("/admin/invoices/provider-accounts");
	const account = data ?? null;
	const status = account?.status ?? "";
	const actionPath = (action: string) =>
		`${PROVIDER_ACCOUNTS_ENDPOINT}/${encodeURIComponent(storeId)}/${action}`;

	/**
	 * 業務碼分流。回 `true` ＝ 已經在頁面層處理掉（dialog 該關），
	 * 回 `false` ＝ 交給呼叫端就地顯示（§B-3.1.5「錯誤就地顯示、不關 dialog」）。
	 */
	const routeBusinessError = (caught: unknown): boolean => {
		if (!(caught instanceof ApiClientError)) return false;
		// 收到回應 ⇒ 下一次按鍵是新的操作，換一把新的冪等鍵。
		// （逾時／斷線走上面那個 return，鍵保持不變。）
		setIdempotencyKey(createProviderIdempotencyKey());

		const code = caught.code ?? 0;
		if (code === PROVIDER_NOT_FOUND_CODE) {
			setOpenDialog(null);
			setBanner({
				tone: "danger",
				text: "這個賣家的 AMEGO 串接已經不存在，畫面已更新。",
			});
			void refetch();
			return true;
		}
		if (PROVIDER_STATE_CONFLICT_CODES.has(code)) {
			setOpenDialog(null);
			// 🔴 **不透傳後端 message**：9336 的定版文案是申訴語境、9333 是「啟用或輪替」、
			// 9322 是「稅務資格資料」，三句在這一頁上都是錯的（58A §A04）。
			setBanner({ tone: "warning", text: providerConflictText(code) });
			void refetch();
			return true;
		}
		return false;
	};

	/**
	 * 端點 44｜重新測試連線。
	 *
	 * 唯讀、不產生任何發票；同步等 AMEGO ⇒ `timeoutMs` 130 秒。
	 * 檢查結果的三種落點（OK／憑證錯／AMEGO 連不上）由回來的 `status` 決定 ——
	 * 見 `providerVerifyResultOf` 的說明（端點 44 的 gateway handler 目前吃掉了 9331）。
	 */
	const healthCheck = useMutation({
		mutationFn: async () => {
			const res = await request<InvoiceEnvelope<SellerProviderAccountData>>(
				actionPath("health_checks"),
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({}),
					timeoutMs: PROVIDER_SYNC_TIMEOUT_MS,
				},
			);
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createProviderIdempotencyKey());
			setCompletion(null);
			setBanner(providerVerifyResultOf(result?.status ?? ""));
			void refetch();
		},
		onError: (caught) => {
			setCompletion(null);
			if (routeBusinessError(caught)) return;
			if (caught instanceof ApiClientError) {
				// 9331／9332 是「檢查跑完了，結果是這個」，不是「檢查失敗」。
				if (caught.code === PROVIDER_CREDENTIAL_INVALID_CODE) {
					const requestId = (caught.data as SellerProviderAccountData | null)
						?.lastRequestId;
					setBanner({
						tone: "danger",
						text: `憑證驗證沒有通過：這組統一編號與 AppKey 在 AMEGO 不被接受。請賣家回「AMEGO 串接設定」重新輸入 AppKey。${
							requestId ? `（服務商請求編號 ${requestId}）` : ""
						}`,
					});
					void refetch();
					return;
				}
				if (caught.code === PROVIDER_UNAVAILABLE_CODE) {
					setBanner(
						providerVerifyResultOf(
							InvoiceProviderAccountStatus.PROVIDER_UNAVAILABLE,
						),
					);
					void refetch();
					return;
				}
			}
			setBanner({
				tone: "danger",
				text: governanceErrorText(caught, "檢查失敗，連線狀態沒有變動。"),
			});
		},
	});

	/** 端點 45｜停用連線。成功後不自動離開，切「已完成」畫面。 */
	const deactivate = useMutation({
		mutationFn: async (reason: string) => {
			const res = await request<
				InvoiceEnvelope<DeactivateProviderAccountResultData>
			>(actionPath("deactivations"), {
				method: "POST",
				headers: { "Idempotency-Key": idempotencyKey },
				body: JSON.stringify({
					reason,
					confirmed: true,
					expectedVersion: account?.version ?? "",
				}),
			});
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createProviderIdempotencyKey());
			setOpenDialog(null);
			setBanner(null);
			if (result) setCompletion({ kind: "deactivate", data: result });
			void refetch();
		},
		onError: (caught) => {
			routeBusinessError(caught);
		},
	});

	/**
	 * 端點 66｜恢復連線。同步等 AMEGO ⇒ `timeoutMs` 130 秒。
	 *
	 * ⚠️ `confirmed: true` 固定帶上：58A 逐字「無 confirmed、沒有 checkbox」，但實作端
	 * `usecase.ReactivateForAdmin` 對 `confirmed != true` 一律回 `ErrProviderConfirmRequired`
	 * ⇒ 不帶就是 100% 失敗。畫面維持 58A 的樣子（不長出一個勾選框），值在 wire 上補齊。
	 *
	 * 🔴 成功**不代表回到 `ACTIVE`**：P15 是 `SUSPENDED → CREDENTIAL_VERIFYING → 驗證結果`。
	 * banner 一律由回來的 `status` 決定，文案寫「已送出重新驗證」不寫「已恢復啟用」。
	 */
	const reactivate = useMutation({
		mutationFn: async (reason: string) => {
			const res = await request<InvoiceEnvelope<SellerProviderAccountData>>(
				actionPath("reactivations"),
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({
						reason,
						confirmed: true,
						expectedVersion: account?.version ?? "",
					}),
					timeoutMs: PROVIDER_SYNC_TIMEOUT_MS,
				},
			);
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createProviderIdempotencyKey());
			setOpenDialog(null);
			setCompletion(null);
			const verdict = providerVerifyResultOf(result?.status ?? "");
			setBanner({
				...verdict,
				text: `已送出重新驗證。${verdict.text}`,
			});
			void refetch();
		},
		onError: (caught) => {
			if (routeBusinessError(caught)) return;
			if (caught instanceof ApiClientError) {
				// 恢復動作本身已經生效（狀態已推進），失敗的是重新驗證那一段
				// ⇒ 關 dialog、把結果講清楚，而不是留在框裡叫人再送一次。
				if (caught.code === PROVIDER_CREDENTIAL_INVALID_CODE) {
					const requestId = (caught.data as SellerProviderAccountData | null)
						?.lastRequestId;
					setOpenDialog(null);
					setBanner({
						tone: "danger",
						text: `已送出重新驗證，但憑證沒有通過：這組統一編號與 AppKey 在 AMEGO 不被接受。請賣家回「AMEGO 串接設定」重新輸入 AppKey。${
							requestId ? `（服務商請求編號 ${requestId}）` : ""
						}`,
					});
					void refetch();
					return;
				}
				if (caught.code === PROVIDER_UNAVAILABLE_CODE) {
					setOpenDialog(null);
					setBanner({
						tone: "warning",
						text: "已送出重新驗證，但 AMEGO 目前無法連線，這不是該賣家的設定問題。請稍後用「手動更新」看最新狀態，不要請賣家更換 AppKey。",
					});
					void refetch();
					return;
				}
			}
		},
	});

	/** 端點 67｜撤銷連線。**不可逆**；成功後 `status` 變 `REVOKED`，治理 CTA 全數收起。 */
	const revoke = useMutation({
		mutationFn: async (reason: string) => {
			const res = await request<
				InvoiceEnvelope<RevokeProviderAccountResultData>
			>(actionPath("revocations"), {
				method: "POST",
				headers: { "Idempotency-Key": idempotencyKey },
				body: JSON.stringify({
					reason,
					confirmed: true,
					expectedVersion: account?.version ?? "",
				}),
			});
			return res?.data ?? null;
		},
		onSuccess: (result) => {
			setIdempotencyKey(createProviderIdempotencyKey());
			setOpenDialog(null);
			setBanner(null);
			if (result) setCompletion({ kind: "revoke", data: result });
			void refetch();
		},
		onError: (caught) => {
			routeBusinessError(caught);
		},
	});

	const busy =
		healthCheck.isPending ||
		deactivate.isPending ||
		reactivate.isPending ||
		revoke.isPending;

	// 治理 CTA 一律**不渲染**（不是 disabled）——58A §A04「四態．disabled」。
	const canHealthCheck =
		status !== "" && !HEALTH_CHECK_HIDDEN_STATUSES.has(status);
	const canDeactivate = DEACTIVATABLE_STATUSES.has(status);
	const canReactivate = REACTIVATABLE_STATUSES.has(status);
	const canRevoke = REVOCABLE_STATUSES.has(status);

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回 AMEGO 串接監控列表"
						onClick={back}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">
							AMEGO 串接詳情
							{account ? (
								<>
									{" "}
									<StatusBadge
										variant={
											PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[account.status] ??
											"gray"
										}
									>
										{codeLabel(
											INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
											account.status,
										)}
									</StatusBadge>
								</>
							) : null}
						</h1>
						<div className="invoice-meta">
							<span>store_id：{storeId}</span>
							{account?.provider ? (
								<span>服務商：{account.provider}</span>
							) : null}
						</div>
					</div>
				</div>
				<div className="header-actions">
					{/*
					 * CTA 順序：撤銷**不放第一位**、停用用次要樣式、只有撤銷是 danger
					 * （58A §A04：一顆可逆、一顆不可逆，不得並排成兩顆同權重的紅色鍵）。
					 */}
					{canHealthCheck ? (
						<Button
							variant="outline"
							size="sm"
							onClick={() => healthCheck.mutate()}
							loading={healthCheck.isPending}
							disabled={busy}
						>
							{healthCheck.isPending ? "檢查中…（最長 2 分鐘）" : "重新測試"}
						</Button>
					) : null}
					{canDeactivate ? (
						<Button
							variant="outline"
							size="sm"
							onClick={() => setOpenDialog("deactivate")}
							disabled={busy}
						>
							停用連線
						</Button>
					) : null}
					{canReactivate ? (
						<Button
							variant="primary"
							size="sm"
							onClick={() => setOpenDialog("reactivate")}
							disabled={busy}
						>
							恢復連線
						</Button>
					) : null}
					{canRevoke ? (
						<Button
							variant="danger"
							size="sm"
							onClick={() => setOpenDialog("revoke")}
							disabled={busy}
						>
							撤銷連線
						</Button>
					) : null}
					{/*
					 * A09 的唯一入口（58A §A09「進入點／導覽」）。獨立頁不是 Drawer：
					 * 這一頁的內容是爭議時要指給人看的證據，獨立 URL 才貼得出去。
					 */}
					<Button
						variant="outline"
						size="sm"
						onClick={() =>
							router.push(
								`/admin/invoices/provider-accounts/${encodeURIComponent(storeId)}/cutovers`,
							)
						}
					>
						查看歷程
					</Button>
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
					<p className="invoice-empty-hint">這個賣家尚未建立 AMEGO 串接。</p>
					<Button variant="primary" onClick={back}>
						返回列表
					</Button>
				</section>
			) : (
				<>
					{/*
					 * 「已完成」畫面（58A §A04：成功後不自動離開）。
					 * `pendingOrderCount` 是**停用／撤銷之後**的計數，只在這裡出現；
					 * dialog 內一律不顯示受影響訂單數（前端估算會與後端判定不同）。
					 * 收到不含該欄的回應走「整列不畫」而不是印 undefined。
					 */}
					{completion ? (
						<section className="invoice-scope-notice" role="status">
							<MaterialSymbol name="check_circle" />
							<div>
								<p className="invoice-scope-notice__title">
									{completion.kind === "deactivate"
										? "已停用這個賣家的 AMEGO 連線"
										: "已永久撤銷這個賣家的 AMEGO 連線"}
								</p>
								{completion.data.affectedScopeNotice ? (
									<p className="invoice-scope-notice__text">
										{completion.data.affectedScopeNotice}
									</p>
								) : null}
								{completion.data.pendingOrderCount ? (
									<p className="invoice-scope-notice__text">
										受影響的待處理訂單：{completion.data.pendingOrderCount} 筆
									</p>
								) : null}
								{completion.kind === "deactivate" &&
								completion.data.deactivatedAt ? (
									<p className="invoice-scope-notice__text">
										停用時點：{formatDateTime(completion.data.deactivatedAt)}
									</p>
								) : null}
								{completion.kind === "revoke" && completion.data.revokedAt ? (
									<p className="invoice-scope-notice__text">
										撤銷時點：{formatDateTime(completion.data.revokedAt)}
									</p>
								) : null}
								{completion.data.auditLogId ? (
									<p className="invoice-scope-notice__text">
										稽核紀錄編號：{completion.data.auditLogId}
									</p>
								) : null}
							</div>
						</section>
					) : null}

					<ResultBanner result={banner} />

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
					) : account === null ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint">沒有取得這個賣家的串接資料。</p>
						</section>
					) : (
						<section className="invoice-section">
							<h2 className="invoice-section__title">連線狀態</h2>
							{account.statusNotice ? (
								<p className="invoice-section__hint">{account.statusNotice}</p>
							) : null}
							<div className="invoice-fields">
								<Field label="連線狀態">
									<StatusBadge
										variant={
											PROVIDER_ACCOUNT_STATUS_BADGE_VARIANT[account.status] ??
											"gray"
										}
									>
										{codeLabel(
											INVOICE_PROVIDER_ACCOUNT_STATUS_LABEL,
											account.status,
										)}
									</StatusBadge>
								</Field>
								<Field label="統一編號">{account.sellerTaxId || "—"}</Field>
								<Field
									label="AppKey"
									note="完整值永不回顯：沒有任何介面讀得到明文，這不是遮罩而是根本不存在。"
								>
									{account.credentialLast4 ? (
										<span className="invoice-secret">
											••••••••{account.credentialLast4}
										</span>
									) : (
										"—"
									)}
								</Field>
								{/*
								 * 這三欄標明時區（backend#3053）：切點決定哪些訂單走自動開票、
								 * 驗證時間與憑證更新時間是賣家能不能開票的判斷依據 ——
								 * 讀錯一小時就會把切點前後的訂單判錯。標了之後，就算未來有人
								 * 把格式化改回本機時區也看得出來。
								 */}
								<Field label="憑證更新時間">
									{formatTaipeiDateTime(account.credentialUpdatedAt)}
								</Field>
								<Field label="最近驗證時間">
									{formatTaipeiDateTime(account.lastVerifiedAt)}
								</Field>
								<Field
									label="自動開票啟用切點"
									note="這是「之後付款的訂單才走自動開票」的分界，不是啟用當下就回溯既有訂單。"
								>
									{account.automationCutoverAt
										? formatTaipeiDateTime(account.automationCutoverAt)
										: "尚未啟用"}
								</Field>
								<Field label="同意書版本">
									{account.consentVersion || "—"}
								</Field>
								<Field
									label="最近一次請求編號"
									note="向服務商查問單筆問題時提供這個編號。"
								>
									<span className="invoice-code">
										{account.lastRequestId || "—"}
									</span>
								</Field>
								<Field label="待處理訂單數">
									{account.pendingOrderCount || "0"}
								</Field>
								<Field label="資料版本">{account.version || "—"}</Field>
							</div>
						</section>
					)}
				</>
			)}

			{openDialog === "deactivate" ? (
				<ProviderDeactivateDialog
					storeId={storeId}
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await deactivate.mutateAsync(reason);
					}}
				/>
			) : null}
			{openDialog === "reactivate" ? (
				<ProviderReactivateDialog
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await reactivate.mutateAsync(reason);
					}}
				/>
			) : null}
			{openDialog === "revoke" ? (
				<ProviderRevokeDialog
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await revoke.mutateAsync(reason);
					}}
				/>
			) : null}
		</div>
	);
}
