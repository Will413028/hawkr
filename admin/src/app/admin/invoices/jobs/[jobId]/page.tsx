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
	EXTERNAL_INVOICE_NO_PATTERN,
	FUNDING_DECISION_OPTIONS,
	JOB_NOT_FOUND_CODE,
	JOBS_ENDPOINT,
	MANUAL_UNLOCK_AWAITING,
	MANUAL_UNLOCK_NEEDS_SECOND_APPROVAL_CODE,
	MANUAL_UNLOCK_STATUS_LABEL,
	MAX_ADMIN_NOTE_RUNES,
	MAX_UNLOCK_EVIDENCE_RUNES,
} from "@/features/invoices/admin-actions";
import { INVOICE_OPERATION_STATUS_BADGE_VARIANT } from "@/features/invoices/badge-variants";
import InvoiceActionDialog from "@/features/invoices/components/InvoiceActionDialog";
import InvoiceResultBanner, {
	type InvoiceBanner,
} from "@/features/invoices/components/InvoiceResultBanner";
import InvoiceTimeline from "@/features/invoices/components/InvoiceTimeline";
import type {
	AdminInvoiceJobDetail,
	InvoiceEnvelope,
	InvoiceJobMutationData,
	ManualUnlockInfo,
} from "@/features/invoices/types";
import {
	codeLabel,
	INVOICE_CANCEL_EVIDENCE_KIND_LABEL,
	INVOICE_MANUAL_REASON_LABEL,
	INVOICE_OPERATION_STATUS_LABEL,
	INVOICE_OPERATION_TYPE_LABEL,
	INVOICE_TAX_CATEGORY_LABEL,
	InvoiceCancelEvidenceKind,
	InvoiceOperationStatus,
} from "@/lib/codemeta";
import { formatAmount, formatDateTime } from "@/lib/format";
import { request } from "@/services/api";
import { ApiClientError } from "@/services/api-error";
import { uploadImageToMedia } from "@/services/media-upload";
// 對話框骨架（overlay、面板、標題列）在共用 Dialog 自己的 Dialog.css；這裡的 DelegatedOperations.css 只放
// `.adm-operation-field*` 等欄位與版面 class（58A 陷阱 C07：少 import 不會報錯也不會轉紅）。
import "@/styles/pages/DelegatedOperations.css";
import "@/styles/pages/Invoice.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

// A06｜發票異常詳情（58A §A06，端點 51 `GET /api/v1/admin/invoices/jobs/{job_id}`）。
//
// ## 五個操作已接上（2026-09-02）
//
// 52 安全重試／53 受控取消／54 發起人工解鎖／55 覆核人工解鎖／56 回報賣家自行開立結果，
// 五支 RPC 在 invoice-service 都已實作（`internal/grpc/admin_write.go` 與
// `internal/grpc/manual_unlock.go` 的 receiver 清單），dev 實測全部回業務層驗證訊息
// （`expected_version: must not be empty`）而不是 5xx。
//
// 舊版這裡寫「七個操作全部不渲染，因為 RPC 都沒有實作」——那個判斷是錯的，來源是拿
// 猜的路徑名去打 dev（`jobs/{id}/external_issuances` 回 404，真名是 `.../manual_results`）。
//
// **代價不是「少一顆按鈕」**：端點 52 是 R09 的唯一觸發者。訂單一旦落進 R08（原發票
// 已作廢、新發票開不出來），後端救得了，而 ADM 按不到那顆鍵 ⇒ 那張稅務憑證永遠停在
// 「作廢了但沒有新票」。
//
// ## 「重新寄送通知」不在這一頁，而且不是漏做
//
// 端點 49 的路徑是 `documents/{document_id}/notification_resends`（gateway
// `AdminInvoiceDocumentResend`），**掛在 document 上不是 job 上**；而端點 51 的回應
// （`AdminInvoiceJobDetail`）沒有任何 `documentId` 欄位 ⇒ 這一頁組不出那條路徑。
// 憑語意猜一個 id 去打會寄到別人的發票。⇒ 那顆鍵的正確位置是 A13（已接上）。
//
// ## 按鈕的渲染條件一律取自**後端下發的布林**，不從 status 推
//
// `canRetry`／`canCancel`／`canReportExternal` 由後端算，其中 `canRetry` 包含一個前端
// 看不到的事實（該冪等鍵在 AMEGO 已確認未產生文件）。自己從 status 推 ⇒ 推錯的後果是
// 開出第二張發票。條件不成立時**不渲染**（不是 disabled）。
//
// ## 仍然照 58A 做的事
//
//   - 固定警示常駐頁頂（「請勿以手動開票取代系統工作；此操作不會變更出貨狀態」）。
//   - `orderSnapshotJson` 當**字串**呈現，不解析（解析等於讓前端綁死一個歷史結構）。
//   - `providerCode` 與 `providerMessage` **成對**顯示：AMEGO 的 `53` 在官方錯誤表出現
//     兩次且語意不同，只看 code 會導向錯誤的下一步。
//   - `masked*Body` 原樣印出：遮罩在 invoice-service **寫入時**就完成了，前端零遮罩
//     邏輯、零「顯示完整」切換（沒有完整值可顯示）。
//   - 技術內容用原生 `<details>`：ADM 查無 Accordion primitive，原生元素最小新增且
//     天然可鍵盤操作，不為此引入折疊元件庫。
//   - **副標不寫「開啟本頁會記錄一筆存取紀錄」**（陷阱 T30）：本頁主資料是端點 51，
//     它不落 `INVOICE_PII_REVEAL`。宣告與事實不符會把嚇阻變成假保證。那句話只屬於
//     A08（端點 59）與 A13（端點 48）。

/**
 * 這一頁六個對話框的識別。
 *
 * `approveUnlock` 與 `rejectUnlock` 打同一支端點（55），差別只在 `approved` ——
 * 分成兩個值而不是一個 boolean state，是為了讓「哪一顆鍵開了哪一個框」在 JSX 上
 * 讀得出來，而不是靠兩個 state 的組合去推。
 */
type JobDialog =
	| "retry"
	| "cancel"
	| "requestUnlock"
	| "approveUnlock"
	| "rejectUnlock"
	| "reportManual"
	| null;

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

export default function InvoiceJobDetailPage() {
	const router = useRouter();
	const params = useParams<{ jobId: string }>();
	const jobId = String(params?.jobId ?? "");

	/** 開著的對話框。`approveUnlock`／`rejectUnlock` 打同一支端點、`approved` 不同。 */
	const [openDialog, setOpenDialog] = useState<JobDialog>(null);
	const [banner, setBanner] = useState<InvoiceBanner | null>(null);
	/** 端點 52 專屬：折扣資金來源裁決（只有 needsFundingDecision 時才要）。 */
	const [fundingDecision, setFundingDecision] = useState("");
	/** 端點 53 專屬：取消後是否允許人工開立。 */
	const [allowManualAfterCancel, setAllowManualAfterCancel] = useState(false);
	/** 端點 54 專屬：向服務商查證的說明與時點。 */
	const [checkEvidence, setCheckEvidence] = useState("");
	const [checkedAt, setCheckedAt] = useState("");
	/** J28 的證據種類／檔案鍵（只有 `CANCEL_BLOCKED_UNKNOWN` 那條路徑要填）。 */
	const [evidenceKind, setEvidenceKind] = useState("");
	const [evidenceObjectKey, setEvidenceObjectKey] = useState("");
	/** 證據檔上傳（backend #3108）：拿到 stagedToken 後由後端換發成 objectKey。 */
	const [evidenceToken, setEvidenceToken] = useState("");
	const [evidenceFileName, setEvidenceFileName] = useState("");
	const [evidenceUploading, setEvidenceUploading] = useState(false);
	/** 端點 56 專屬：賣家自行開立的發票內容。 */
	const [externalNo, setExternalNo] = useState("");
	const [externalDate, setExternalDate] = useState("");
	const [externalAmount, setExternalAmount] = useState("");

	/**
	 * 冪等鍵。送出失敗不換（可能已生效而回應沒送到），收到回應之後要換
	 * （`uq_invadmaudit_client_request` 走 `ON CONFLICT DO NOTHING`，沿用同一把鍵的
	 * 第二次操作會靜靜地不寫稽核列）。
	 */
	const [idempotencyKey, setIdempotencyKey] = useState(
		createInvoiceAdminIdempotencyKey,
	);

	const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
		queryKey: ["invoice", "job", jobId],
		enabled: jobId !== "",
		queryFn: async () => {
			const res = await request<InvoiceEnvelope<AdminInvoiceJobDetail>>(
				`${JOBS_ENDPOINT}/${encodeURIComponent(jobId)}`,
				{ method: "GET" },
			);
			return res?.data ?? null;
		},
	});

	const notFound =
		isError &&
		error instanceof ApiClientError &&
		error.code === JOB_NOT_FOUND_CODE;

	// 返回用 router.back()：A05 的分頁與篩選狀態留在上一個 history entry 裡，
	// router.push 會把管理員丟回預設分頁的第 1 頁。
	const detail = data ?? null;
	const job = detail?.job;
	const amount = detail?.amountBreakdown;
	// manualUnlock 在 proto 上是 message 不是 optional ⇒ JSON 永遠是物件；
	// 「沒有解鎖請求」的判準是 unlockId 為空字串。
	const unlock = detail?.manualUnlock;
	const hasUnlock = (unlock?.unlockId ?? "") !== "";
	/**
	 * 樂觀鎖的值。取自**這一次載入讀到的那筆** job（端點 54／55 用的也是 job 的版本，
	 * 不是 unlock 的版本——後端 `ApproveManualUnlock` 比對的是 `op.Version`）。
	 */
	const expectedVersion = job?.version ?? "";
	const actionPath = (action: string) =>
		`${JOBS_ENDPOINT}/${encodeURIComponent(jobId)}/${action}`;

	/**
	 * 端點 54 的 allowed-from 有**兩個**狀態（後端 `RequestManualUnlock` 的守衛）：
	 *
	 * - `CANCELLED_BY_ADMIN`      受控取消已落地，只差解鎖 → 覆核時只走 J29
	 * - `CANCEL_BLOCKED_UNKNOWN`  取消受阻、服務商狀態不明 → 發起時要附證據，
	 *   覆核時同一交易內走 **J28 → J29**
	 *
	 * 🔴 第二個是 2026-09-04 補的。在那之前這顆按鈕只認第一個狀態，
	 * 而 `CANCEL_BLOCKED_UNKNOWN` 的**唯一出邊**就是這條路徑
	 * ⇒ 落到那個狀態的工作在 ADM 上沒有任何可執行的動作，會一直卡著。
	 */
	const isCancelBlockedUnknown =
		job?.status === InvoiceOperationStatus.CANCEL_BLOCKED_UNKNOWN;
	const canRequestUnlock =
		(job?.status === InvoiceOperationStatus.CANCELLED_BY_ADMIN ||
			isCancelBlockedUnknown) &&
		(!hasUnlock || unlock?.status !== MANUAL_UNLOCK_AWAITING);
	/** 端點 55 只在有一筆等待覆核的請求時成立。 */
	const canApproveUnlock =
		hasUnlock && unlock?.status === MANUAL_UNLOCK_AWAITING;

	const routeBusinessError = (caught: unknown): boolean => {
		if (!(caught instanceof ApiClientError)) return false;
		setIdempotencyKey(createInvoiceAdminIdempotencyKey());

		const code = caught.code ?? 0;
		if (code === JOB_NOT_FOUND_CODE) {
			setOpenDialog(null);
			setBanner({
				tone: "danger",
				text: "這件發票工作已經不存在，畫面已更新。",
			});
			void refetch();
			return true;
		}
		if (code === MANUAL_UNLOCK_NEEDS_SECOND_APPROVAL_CODE) {
			// 🔴 9344 不是失敗：它是「已經有一位管理員發起過了」，data 帶著既有那一列的
			// unlockId。回應是 HTTP 200 而 code 非零（preserveData 家族），所以只會走到
			// 這裡——把畫面切到「覆核」而不是顯示紅字。
			setOpenDialog(null);
			const existing = caught.data as ManualUnlockInfo | null;
			setBanner({
				tone: "warning",
				text: `已經有一筆等待覆核的解鎖請求${
					existing?.unlockId ? `（編號 ${existing.unlockId}）` : ""
				}，請改為覆核。畫面已更新。`,
			});
			void refetch();
			return true;
		}
		if (ACTION_CONFLICT_CODES.has(code)) {
			setOpenDialog(null);
			setBanner({ tone: "warning", text: actionConflictText(code) });
			void refetch();
			return true;
		}
		return false;
	};

	/**
	 * 送出包裝。分流不能放在 `useMutation` 的 `onError`：那個回呼丟出去的例外不會傳到
	 * `mutateAsync` 的呼叫端，dialog 會把「狀態已變」也顯示成一則就地錯誤。
	 */
	const submitting = async (run: () => Promise<unknown>) => {
		try {
			await run();
		} catch (caught) {
			if (routeBusinessError(caught)) return;
			throw caught;
		}
	};

	/** 五支寫入共用的送出（回應形狀分兩種，各自的 onSuccess 再處理）。 */
	const postAction = <T,>(action: string, body: Record<string, unknown>) =>
		request<InvoiceEnvelope<T>>(actionPath(action), {
			method: "POST",
			headers: { "Idempotency-Key": idempotencyKey },
			body: JSON.stringify(body),
		});

	const finishJobMutation =
		(text: string) =>
		(result: InvoiceEnvelope<InvoiceJobMutationData> | null) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenDialog(null);
			// `nextStep` 是後端下發的一句話，原樣接在後面不改寫：同一個 status 在不同
			// operationType 下的下一步不同，前端推不出來。
			const nextStep = result?.data?.nextStep
				? `　${result.data.nextStep}`
				: "";
			setBanner({ tone: "success", text: `${text}${nextStep}` });
			void refetch();
		};

	/** 端點 52｜安全重試。R09 的唯一觸發者。 */
	const retry = useMutation({
		mutationFn: (reason: string) =>
			postAction<InvoiceJobMutationData>("retries", {
				reason,
				expectedVersion,
				// 空字串合法（多數 job 沒有這一格要裁決）；needsFundingDecision 為真時
				// 送空字串會被後端擋下並回一句看得懂的話，所以這裡不自己補預設值。
				fundingDecision: job?.needsFundingDecision ? fundingDecision : "",
			}),
		onSuccess: finishJobMutation("已送出安全重試。"),
	});

	/** 端點 53｜受控取消。 */
	const cancel = useMutation({
		mutationFn: (reason: string) =>
			postAction<InvoiceJobMutationData>("cancellations", {
				// `reasonCode` 刻意不送：後端 `normalizeAdminCancelReason` 把空字串正規化成
				// `ADM_MANUAL`，而 ADM 手動取消**只准**這一個值（送別的一律 400）。
				// 在前端寫死一個 CodeMeta 碼值只會多一份會漂移的拷貝。
				reasonDetail: reason,
				allowManualAfterCancellation: allowManualAfterCancel,
				expectedVersion,
			}),
		onSuccess: finishJobMutation("已取消這件發票工作。"),
	});

	/** 端點 54｜發起人工出口解鎖（雙人第一步）。 */
	const requestUnlock = useMutation({
		mutationFn: (reason: string) =>
			postAction<ManualUnlockInfo>("manual_unlocks", {
				providerCheckEvidence: checkEvidence.trim(),
				// `<input type="datetime-local">` 給的是本地時間且沒有時區 ⇒ 交給 Date
				// 轉成 UTC 的 ISO 8601。直接把那個字串送出去會被後端當成 UTC，於是
				// 台北時間會整整早八小時，而「查證時點」是一列保存 5 年的證據。
				providerCheckedAt: new Date(checkedAt).toISOString(),
				reason,
				expectedVersion,
				// 只有 CANCEL_BLOCKED_UNKNOWN 那條路徑帶；另一條帶了會被後端擋成 400
				// （那份證據不會被任何一句 SQL 讀到，收下它就是靜默失效）。
				cancelEvidenceKind: isCancelBlockedUnknown ? evidenceKind : "",
				// objectKey 與 stagedToken 二擇一（後端兩者並存回 400）。
				// 有上傳檔案就送 token，否則送手貼的 key。
				cancelEvidenceObjectKey:
					isCancelBlockedUnknown && !evidenceToken
						? evidenceObjectKey.trim()
						: "",
				cancelEvidenceStagedToken: isCancelBlockedUnknown ? evidenceToken : "",
			}),
		onSuccess: () => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenDialog(null);
			setCheckEvidence("");
			setCheckedAt("");
			setEvidenceKind("");
			setEvidenceObjectKey("");
			setEvidenceToken("");
			setEvidenceFileName("");
			setBanner({
				tone: "success",
				text: "已發起人工出口解鎖，等待第二位管理員覆核（24 小時內未覆核會自動失效）。",
			});
			void refetch();
		},
	});

	/** 端點 55｜覆核人工出口解鎖（雙人第二步）。駁回也是成功回應。 */
	const approveUnlock = useMutation({
		mutationFn: ({ approved, reason }: { approved: boolean; reason: string }) =>
			request<InvoiceEnvelope<ManualUnlockInfo>>(
				`${actionPath("manual_unlocks")}/${encodeURIComponent(
					unlock?.unlockId ?? "",
				)}/approvals`,
				{
					method: "POST",
					headers: { "Idempotency-Key": idempotencyKey },
					body: JSON.stringify({ approved, reason, expectedVersion }),
				},
			).then((res) => ({ approved, res })),
		onSuccess: ({ approved }) => {
			setIdempotencyKey(createInvoiceAdminIdempotencyKey());
			setOpenDialog(null);
			setBanner({
				tone: "success",
				text: approved
					? "已覆核放行，這件工作已解鎖為可人工處理。"
					: "已駁回這次的解鎖請求，工作狀態維持原樣。",
			});
			void refetch();
		},
	});

	/** 端點 56｜回報賣家自行開立的結果。 */
	const reportManual = useMutation({
		mutationFn: (note: string) =>
			postAction<InvoiceJobMutationData>("manual_results", {
				invoiceNo: externalNo.trim().toUpperCase(),
				invoiceDate: externalDate,
				totalAmount: externalAmount.trim(),
				note,
				expectedVersion,
			}),
		onSuccess: finishJobMutation("已回報賣家自行開立的結果。"),
	});

	return (
		<div>
			<div className="page-header">
				<div className="invoice-header-main">
					<button
						type="button"
						className="btn-back"
						aria-label="返回發票工作佇列"
						onClick={() => router.back()}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="invoice-header-titles">
						<h1 className="page-title">
							發票工作詳情
							{job ? (
								<>
									{" "}
									<StatusBadge
										variant={
											INVOICE_OPERATION_STATUS_BADGE_VARIANT[job.status] ??
											"gray"
										}
									>
										{codeLabel(INVOICE_OPERATION_STATUS_LABEL, job.status)}
									</StatusBadge>
								</>
							) : null}
							{job?.manualReasonCode ? (
								<>
									{" "}
									<StatusBadge variant="warning" size="sm">
										{codeLabel(
											INVOICE_MANUAL_REASON_LABEL,
											job.manualReasonCode,
										)}
									</StatusBadge>
								</>
							) : null}
						</h1>
						<div className="invoice-meta">
							{job?.orderNo ? <span>訂單：{job.orderNo}</span> : null}
							{job?.storeId ? <span>賣家：{job.storeId}</span> : null}
							<span>job_id：{jobId}</span>
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
					<p className="invoice-empty-hint">找不到這筆發票工作。</p>
					<Button
						variant="primary"
						onClick={() => router.push("/admin/invoices/jobs")}
					>
						返回佇列
					</Button>
				</section>
			) : (
				<>
					<p className="invoice-page-notice" role="note">
						<MaterialSymbol name="warning" />
						<span>請勿以手動開票取代系統工作；此操作不會變更出貨狀態。</span>
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
					) : detail === null || job === undefined ? (
						<section className="invoice-section">
							<p className="invoice-empty-hint">沒有取得這筆工作的內容。</p>
						</section>
					) : (
						<>
							<Section
								title="可執行的操作"
								hint="條件不成立的動作不會出現在這裡——一顆按不下去的按鈕會讓人一直找「為什麼不能按」，而正確的訊息是「這個狀態沒有這個動作」。每個動作都要填理由，理由會寫入保存 5 年的稽核紀錄。"
							>
								{!job.canRetry &&
								!job.canCancel &&
								!canRequestUnlock &&
								!canApproveUnlock &&
								!job.canReportExternal ? (
									<p className="invoice-empty-hint">
										這件工作目前沒有可執行的管理員操作。
									</p>
								) : (
									<div className="invoice-decision-actions">
										{job.canRetry ? (
											<Button
												variant="primary"
												size="sm"
												onClick={() => {
													setBanner(null);
													setFundingDecision("");
													setOpenDialog("retry");
												}}
											>
												安全重試
											</Button>
										) : null}
										{job.canCancel ? (
											<Button
												variant="danger"
												size="sm"
												onClick={() => {
													setBanner(null);
													setAllowManualAfterCancel(false);
													setOpenDialog("cancel");
												}}
											>
												受控取消
											</Button>
										) : null}
										{canRequestUnlock ? (
											<Button
												variant="outline"
												size="sm"
												onClick={() => {
													setBanner(null);
													setCheckEvidence("");
													setCheckedAt("");
													setOpenDialog("requestUnlock");
												}}
											>
												發起人工解鎖
											</Button>
										) : null}
										{canApproveUnlock ? (
											<>
												<Button
													variant="primary"
													size="sm"
													onClick={() => {
														setBanner(null);
														setOpenDialog("approveUnlock");
													}}
												>
													覆核放行解鎖
												</Button>
												<Button
													variant="ghost"
													size="sm"
													onClick={() => {
														setBanner(null);
														setOpenDialog("rejectUnlock");
													}}
												>
													駁回解鎖請求
												</Button>
											</>
										) : null}
										{job.canReportExternal ? (
											<Button
												variant="outline"
												size="sm"
												onClick={() => {
													setBanner(null);
													setExternalNo("");
													setExternalDate("");
													setExternalAmount("");
													setOpenDialog("reportManual");
												}}
											>
												回報賣家自行開立結果
											</Button>
										) : null}
									</div>
								)}
								{/*
								 * 「重新寄送通知」不在這一頁。端點 49 掛在 document 上
								 * （`documents/{document_id}/notification_resends`），而端點 51 的回應
								 * 沒有 documentId ⇒ 這一頁組不出那條路徑。入口在 A13。
								 */}
								<p className="invoice-field__note">
									要重新寄送發票通知，請到該張發票的「發票文件詳情」——那個動作掛在文件上，這一頁沒有文件編號可用。
								</p>
							</Section>

							<Section title="異常摘要">
								<p className="invoice-field__value">{job.summary || "—"}</p>
								<div className="invoice-fields invoice-fields--spaced">
									<Field label="操作類型">
										{codeLabel(INVOICE_OPERATION_TYPE_LABEL, job.operationType)}
									</Field>
									<Field
										label="後端判定：可否安全重試"
										note="這個值由後端算，包含一個前端看不到的事實（該冪等鍵在服務商端已確認未產生文件）。"
									>
										{job.canRetry ? "可重試" : "不可重試"}
									</Field>
									<Field label="後端判定：可否取消">
										{job.canCancel ? "可取消" : "不可取消"}
									</Field>
									<Field label="是否需要出資裁決">
										{job.needsFundingDecision ? "需要" : "不需要"}
									</Field>
								</div>
							</Section>

							<Section
								title="訂單與付款快照"
								hint="這是當時那份快照的原文，不是即時資料；前端不解析它，因為解析等於讓畫面綁死一個歷史結構。"
							>
								<details className="invoice-details">
									<summary>展開訂單快照 JSON</summary>
									<pre className="invoice-pre">
										{detail.orderSnapshotJson || "（沒有快照內容）"}
									</pre>
								</details>
							</Section>

							<Section title="發票計算明細">
								<div className="invoice-fields">
									<Field label="商品金額">
										{formatAmount(amount?.productAmount)}
									</Field>
									<Field label="運費">
										{formatAmount(amount?.shippingAmount)}
									</Field>
									<Field label="折扣">
										{formatAmount(amount?.discountAmount)}
									</Field>
									<Field label="開立金額">
										{formatAmount(amount?.invoiceAmount)}
									</Field>
									<Field label="稅別">
										{codeLabel(INVOICE_TAX_CATEGORY_LABEL, amount?.taxCategory)}
									</Field>
								</div>
							</Section>

							<Section
								title={`服務商送件歷程（${detail.providerExchanges.length} 筆）`}
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
												{/* code 與 message 成對：同一個 code 在官方錯誤表可能出現
												    多次且語意不同，只看 code 會導向錯誤的下一步。 */}
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

							<Section title="冪等鍵與重試">
								<div className="invoice-fields">
									<Field label="冪等鍵" full>
										<span className="invoice-code">
											{detail.idempotencyKey || "—"}
										</span>
									</Field>
									<Field label="已嘗試次數">
										{job.attemptCount || "0"} / {job.maxAttempts || "—"}
									</Field>
									<Field label="最近一次嘗試">
										{formatDateTime(job.lastAttemptAt)}
									</Field>
									<Field label="下次重試時間">
										{formatDateTime(job.nextRetryAt)}
									</Field>
									<Field label="重試間隔">
										{job.retryPolicy?.initialIntervalSeconds || "—"} 秒起，倍率{" "}
										{job.retryPolicy?.backoffMultiplier || "—"}，上限{" "}
										{job.retryPolicy?.maxIntervalSeconds || "—"} 秒
									</Field>
									<Field label="服務商請求編號">
										<span className="invoice-code">
											{job.providerRequestId || "—"}
										</span>
									</Field>
									<Field label="資料版本">{job.version || "—"}</Field>
								</div>
							</Section>

							<Section title="出貨狀態">
								<div className="invoice-fields">
									<Field
										label="目前出貨狀態"
										full
										note={
											job.shipmentUnaffectedNotice ||
											"發票狀態不影響出貨；這一欄只是讓你知道這張訂單走到哪裡了。"
										}
									>
										{detail.shipmentStatus || "—"}
									</Field>
								</div>
							</Section>

							{hasUnlock ? (
								<Section
									title="人工解鎖現況"
									hint="雙人把關：發起與覆核必須是兩位不同的管理員，而且覆核的人不能是當初提出取消的那一位。請求發起後 24 小時未覆核就自動失效。"
								>
									<div className="invoice-fields">
										<Field label="解鎖編號">
											<span className="invoice-code">{unlock?.unlockId}</span>
										</Field>
										<Field label="狀態">
											{/* 這四個值不是 CodeMeta enum（`domain/manual_unlock.go` 的 Go
											    常數）⇒ 中文查 admin-actions 的對照表，查不到就原樣顯示碼值。 */}
											{MANUAL_UNLOCK_STATUS_LABEL[unlock?.status ?? ""] ??
												unlock?.status ??
												"—"}
										</Field>
										<Field label="發起人 account_id">
											{unlock?.requestedByAccountId || "—"}
										</Field>
										<Field label="覆核人 account_id">
											{unlock?.approvedByAccountId || "—"}
										</Field>
										<Field label="發起時間">
											{formatDateTime(unlock?.requestedAt)}
										</Field>
										<Field label="到期時間">
											{formatDateTime(unlock?.expiresAt)}
										</Field>
										<Field label="向服務商查證的時間">
											{formatDateTime(unlock?.providerCheckedAt)}
										</Field>
										{/*
										 * J28 的證據只在 CANCEL_BLOCKED_UNKNOWN 那條路徑上有值。
										 * **覆核者要看得到它**——覆核放行會在同一筆交易裡順帶確認
										 * 「這張票確實沒開出去」，看不到證據就等於對看不見的東西按同意。
										 */}
										{unlock?.cancelEvidenceKind ? (
											<Field label="取消證據來源">
												{codeLabel(
													INVOICE_CANCEL_EVIDENCE_KIND_LABEL,
													unlock.cancelEvidenceKind,
												)}
											</Field>
										) : null}
										{unlock?.cancelEvidenceObjectKey ? (
											<Field label="證據檔案">
												{/*
												 * 🔴 覆核者要**打得開**才算看得到證據——只顯示 objectKey
												 * 等於要他相信一串他打不開的字。J28 的後端註解逐字：
												 * 「沒有它，第二個人是在對一份看不見證據的請求按同意」。
												 */}
												<a
													href={`/api/v1${JOBS_ENDPOINT}/${encodeURIComponent(jobId)}/manual_unlocks/${encodeURIComponent(unlock.unlockId)}/evidence/content?disposition=inline`}
													target="_blank"
													rel="noreferrer"
												>
													開啟證據檔
												</a>
											</Field>
										) : null}
										<Field label="查證方式與結果" full>
											{unlock?.providerCheckEvidence || "—"}
										</Field>
									</div>
								</Section>
							) : null}

							<Section
								title="操作稽核"
								hint="追加不覆蓋：每一筆操作都保留，這一頁不提供編輯。"
							>
								<InvoiceTimeline
									emptyText="尚無操作紀錄。"
									entries={detail.auditLogs.map((log, index) => ({
										id: log.auditLogId || `audit-${index}`,
										time: formatDateTime(log.occurredAt),
										title: codeLabel(
											INVOICE_OPERATION_TYPE_LABEL,
											log.operationType,
										),
										actor:
											log.actorDisplayName || log.actorAccountId
												? `操作者：${log.actorDisplayName || log.actorAccountId}`
												: undefined,
										body: [log.reason, log.result && `結果：${log.result}`]
											.filter(Boolean)
											.join("\n"),
									}))}
								/>
							</Section>
						</>
					)}
				</>
			)}

			{openDialog === "retry" ? (
				<InvoiceActionDialog
					idPrefix="adm-job-retry"
					icon="refresh"
					title="安全重試"
					description={`job_id ${jobId}　${
						job?.orderNo ? `訂單 ${job.orderNo}` : ""
					}`}
					consequence="重新把這件工作送給服務商。後端已確認這個冪等鍵在服務商端沒有產生文件，所以不會開出第二張發票。"
					reasonLabel="重試理由"
					reasonPlaceholder="例：服務商端暫時性錯誤已排除，重新送出。"
					submitLabel="送出重試"
					submittingLabel="送出中…"
					failureText="重試沒有送出，這件工作沒有變動；請確認網路狀態後再送一次。"
					validateExtra={() =>
						job?.needsFundingDecision && !fundingDecision
							? "這件工作需要先裁決折扣的資金來源，請選一項。"
							: null
					}
					extraFields={(disabled) =>
						job?.needsFundingDecision ? (
							<div className="adm-operation-field adm-operation-field--wide">
								<label htmlFor="adm-job-retry-funding">折扣資金來源</label>
								<select
									id="adm-job-retry-funding"
									value={fundingDecision}
									onChange={(event) => setFundingDecision(event.target.value)}
									disabled={disabled}
								>
									<option value="">請選擇</option>
									{FUNDING_DECISION_OPTIONS.map((option) => (
										<option key={option.value} value={option.value}>
											{option.label}
										</option>
									))}
								</select>
								<span className="adm-operation-field-note">
									{FUNDING_DECISION_OPTIONS.find(
										(option) => option.value === fundingDecision,
									)?.hint ??
										"兩者的開立金額不同，不是「隨便選一個就好」：由平台吸收時開立金額是折扣前金額。"}
								</span>
							</div>
						) : null
					}
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await submitting(() => retry.mutateAsync(reason));
					}}
				/>
			) : null}

			{openDialog === "cancel" ? (
				<InvoiceActionDialog
					idPrefix="adm-job-cancel"
					icon="cancel"
					title="受控取消"
					description={`job_id ${jobId}　${
						job?.orderNo ? `訂單 ${job.orderNo}` : ""
					}`}
					consequence="取消之後這件工作不會再自動送出。取消原因固定記為「管理員手動取消」，你填的說明會一併存進稽核紀錄。"
					reasonLabel="取消說明"
					reasonPlaceholder="例：買家已改為索取收據，本張發票不需開立。"
					variant="danger"
					confirmText="我了解取消之後這件工作不會再自動送出。"
					submitLabel="確認取消"
					submittingLabel="取消中…"
					failureText="取消沒有送出，這件工作沒有變動；請確認網路狀態後再送一次。"
					extraFields={(disabled) => (
						<div className="adm-operation-field adm-operation-field--wide">
							<label
								className="invoice-confirm-check"
								htmlFor="adm-job-cancel-allow-manual"
							>
								<input
									id="adm-job-cancel-allow-manual"
									type="checkbox"
									checked={allowManualAfterCancel}
									onChange={(event) =>
										setAllowManualAfterCancel(event.target.checked)
									}
									disabled={disabled}
								/>
								<span>
									取消後允許改由人工開立（不勾 ＝ 這張訂單不再開立任何發票）
								</span>
							</label>
						</div>
					)}
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await submitting(() => cancel.mutateAsync(reason));
					}}
				/>
			) : null}

			{openDialog === "requestUnlock" ? (
				<InvoiceActionDialog
					idPrefix="adm-job-unlock-request"
					icon="lock_open"
					title="發起人工出口解鎖"
					description={`job_id ${jobId}　需要第二位管理員覆核才會生效`}
					consequence="解鎖之後這件工作才能改由人工開立。發起後 24 小時內沒有人覆核就自動失效，覆核的人不能是你，也不能是當初提出取消的那一位。"
					reasonLabel="申請理由"
					reasonPlaceholder="例：訂單需補開發票，已確認服務商端沒有產生任何文件。"
					submitLabel="送出申請"
					submittingLabel="送出中…"
					failureText="申請沒有送出，這件工作沒有變動；請確認網路狀態後再送一次。"
					validateExtra={() => {
						if (!checkEvidence.trim()) {
							return "請填寫向服務商查證的方式與結果，這是「確實沒開出去」的唯一證據。";
						}
						if (
							Array.from(checkEvidence.trim()).length >
							MAX_UNLOCK_EVIDENCE_RUNES
						) {
							return `查證說明請控制在 ${MAX_UNLOCK_EVIDENCE_RUNES} 字以內。`;
						}
						if (!checkedAt) return "請填寫向服務商查證的時間。";
						if (new Date(checkedAt).getTime() > Date.now()) {
							// 後端也擋（未來時點不可能發生過），這裡先擋是為了不浪費一次往返。
							return "查證時間不能是未來時間。";
						}
						if (isCancelBlockedUnknown && !evidenceKind) {
							return "取消受阻（狀態不明）的工作必須指定取消證據來源。";
						}
						return null;
					}}
					extraFields={(disabled) => (
						<>
							<div className="adm-operation-field adm-operation-field--wide">
								<label htmlFor="adm-job-unlock-evidence">
									向服務商查證的方式與結果
								</label>
								<textarea
									id="adm-job-unlock-evidence"
									rows={3}
									maxLength={MAX_UNLOCK_EVIDENCE_RUNES}
									value={checkEvidence}
									onChange={(event) => setCheckEvidence(event.target.value)}
									disabled={disabled}
									placeholder="例：致電 AMEGO 客服，以 requestId O123456-1-1 查詢，客服確認該筆未產生任何發票號碼。"
								/>
								<span className="adm-operation-field-note">
									這一段會存進解鎖紀錄並保存 5
									年，請寫得讓之後查帳的人看得出你是怎麼確認的。
								</span>
							</div>
							{isCancelBlockedUnknown ? (
								<>
									<div className="adm-operation-field">
										<label htmlFor="adm-job-unlock-evidence-kind">
											取消證據來源
										</label>
										<select
											id="adm-job-unlock-evidence-kind"
											value={evidenceKind}
											onChange={(event) => setEvidenceKind(event.target.value)}
											disabled={disabled}
										>
											<option value="">請選擇</option>
											{/*
											 * 只列**需人判讀**的兩個。另外兩個
											 * （PROVIDER_QUERY_71／PROVIDER_LIST_ABSENT）是服務商
											 * 直接作證、由系統自動落地，人不該替系統蓋章
											 * ——後端也會擋（400）。
											 */}
											<option
												value={
													InvoiceCancelEvidenceKind.MOF_PLATFORM_SCREENSHOT
												}
											>
												{
													INVOICE_CANCEL_EVIDENCE_KIND_LABEL[
														InvoiceCancelEvidenceKind.MOF_PLATFORM_SCREENSHOT
													]
												}
											</option>
											<option
												value={InvoiceCancelEvidenceKind.AMEGO_RECONCILE_FILE}
											>
												{
													INVOICE_CANCEL_EVIDENCE_KIND_LABEL[
														InvoiceCancelEvidenceKind.AMEGO_RECONCILE_FILE
													]
												}
											</option>
										</select>
										<span className="adm-operation-field-note">
											這件工作的取消卡在「服務商狀態不明」，覆核放行會同時確認取消並解鎖，
											所以必須先說明憑什麼相信它沒有開出去。
										</span>
									</div>
									<div className="adm-operation-field">
										<label htmlFor="adm-job-unlock-evidence-file">
											證據檔案（選填）
										</label>
										<input
											id="adm-job-unlock-evidence-file"
											type="file"
											accept="image/*,.pdf,.csv,.xlsx"
											disabled={disabled || evidenceUploading}
											onChange={async (event) => {
												const file = event.target.files?.[0];
												if (!file) return;
												setEvidenceUploading(true);
												try {
													// 🔴 isSecure: true —— 財政部截圖／對帳檔含賣家稅籍資訊，
													// 走公開資產等於放到任何人拿到網址就看得到的地方。
													const token = await uploadImageToMedia(file, {
														isSecure: true,
													});
													setEvidenceToken(token);
													setEvidenceFileName(file.name);
													setEvidenceObjectKey("");
												} catch (caught) {
													setEvidenceToken("");
													setEvidenceFileName("");
													setBanner({
														tone: "danger",
														text:
															caught instanceof Error
																? `證據檔上傳失敗：${caught.message}`
																: "證據檔上傳失敗，請再試一次。",
													});
												} finally {
													setEvidenceUploading(false);
												}
											}}
										/>
										<span className="adm-operation-field-note">
											{evidenceUploading
												? "上傳中…"
												: evidenceFileName
													? `已上傳：${evidenceFileName}`
													: "財政部平台截圖或光貿對帳檔。存成機敏資產，只有覆核者看得到。"}
										</span>
									</div>
								</>
							) : null}
							<div className="adm-operation-field">
								<label htmlFor="adm-job-unlock-checked-at">查證時間</label>
								<input
									id="adm-job-unlock-checked-at"
									type="datetime-local"
									value={checkedAt}
									onChange={(event) => setCheckedAt(event.target.value)}
									disabled={disabled}
								/>
								<span className="adm-operation-field-note">
									你實際向服務商確認的時點（不能填未來時間）。
								</span>
							</div>
						</>
					)}
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						await submitting(() => requestUnlock.mutateAsync(reason));
					}}
				/>
			) : null}

			{openDialog === "approveUnlock" || openDialog === "rejectUnlock" ? (
				<InvoiceActionDialog
					idPrefix="adm-job-unlock-approval"
					icon="person_add"
					title={
						openDialog === "approveUnlock" ? "覆核放行解鎖" : "駁回解鎖請求"
					}
					description={`解鎖編號 ${unlock?.unlockId ?? "—"}　發起人 account_id ${
						unlock?.requestedByAccountId || "—"
					}`}
					consequence={
						openDialog === "approveUnlock"
							? "放行之後這件工作變成「已解鎖人工處理」，接著才能回報賣家自行開立的結果。"
							: "駁回之後這筆解鎖請求結束，工作狀態維持原樣；需要的話可以重新發起。"
					}
					reasonLabel="覆核意見"
					reasonPlaceholder="例：已複核查證紀錄，服務商端確實沒有產生文件，同意解鎖。"
					variant={openDialog === "approveUnlock" ? "primary" : "danger"}
					submitLabel={openDialog === "approveUnlock" ? "確認放行" : "確認駁回"}
					submittingLabel="送出中…"
					failureText="覆核沒有送出，這筆請求沒有變動；請確認網路狀態後再送一次。"
					onClose={() => setOpenDialog(null)}
					onSubmit={async (reason) => {
						const approved = openDialog === "approveUnlock";
						await submitting(() =>
							approveUnlock.mutateAsync({ approved, reason }),
						);
					}}
				/>
			) : null}

			{openDialog === "reportManual" ? (
				<InvoiceActionDialog
					idPrefix="adm-job-manual-result"
					icon="list_alt"
					title="回報賣家自行開立結果"
					description={`job_id ${jobId}　${
						job?.orderNo ? `訂單 ${job.orderNo}` : ""
					}`}
					consequence="回報之後這件工作結案為「賣家自行開立並回報」，平台不會再為這張訂單開票。發票號碼與日期會成為這筆訂單的稅務憑據，請照賣家提供的實體發票逐字填寫。"
					reasonLabel="處理說明"
					reasonNote="說明會寫入發票稽核紀錄（保存 5 年）。"
					reasonPlaceholder="例：賣家提供實體發票影本，號碼與金額均已核對一致。"
					maxReasonRunes={MAX_ADMIN_NOTE_RUNES}
					submitLabel="送出回報"
					submittingLabel="送出中…"
					failureText="回報沒有送出，這件工作沒有變動；請確認網路狀態後再送一次。"
					validateExtra={() => {
						if (
							!EXTERNAL_INVOICE_NO_PATTERN.test(externalNo.trim().toUpperCase())
						) {
							return "發票號碼格式是 2 個大寫英文字母加 8 位數字，例如 AB12345678。";
						}
						if (!/^\d{4}-\d{2}-\d{2}$/u.test(externalDate)) {
							return "請填寫發票日期。";
						}
						if (!/^\d+$/u.test(externalAmount.trim())) {
							return "發票金額請填非負整數（新臺幣元，不含小數）。";
						}
						return null;
					}}
					extraFields={(disabled) => (
						<>
							<div className="adm-operation-field">
								<label htmlFor="adm-job-manual-no">發票號碼</label>
								<input
									id="adm-job-manual-no"
									type="text"
									value={externalNo}
									onChange={(event) => setExternalNo(event.target.value)}
									disabled={disabled}
									placeholder="AB12345678"
								/>
								<span className="adm-operation-field-note">
									2 個大寫英文字母 ＋ 8 位數字。
								</span>
							</div>
							<div className="adm-operation-field">
								<label htmlFor="adm-job-manual-date">發票日期</label>
								<input
									id="adm-job-manual-date"
									type="date"
									value={externalDate}
									onChange={(event) => setExternalDate(event.target.value)}
									disabled={disabled}
								/>
								<span className="adm-operation-field-note">
									不得早於這張訂單的付款日（後端會擋）。
								</span>
							</div>
							<div className="adm-operation-field">
								<label htmlFor="adm-job-manual-amount">發票金額（元）</label>
								<input
									id="adm-job-manual-amount"
									type="text"
									inputMode="numeric"
									value={externalAmount}
									onChange={(event) => setExternalAmount(event.target.value)}
									disabled={disabled}
									placeholder="1280"
								/>
								<span className="adm-operation-field-note">
									新臺幣元的整數，不含小數與千分位。
								</span>
							</div>
						</>
					)}
					onClose={() => setOpenDialog(null)}
					onSubmit={async (note) => {
						await submitting(() => reportManual.mutateAsync(note));
					}}
				/>
			) : null}
		</div>
	);
}
