"use client";

/**
 * S08｜發票異常詳情（58A §S08）。端點 33 是本頁唯一的資料源。
 *
 * 權限：`CapInvoiceManage`／停權 **BLOCK**（**不在**白名單）⇒ 停權賣家的回報落點
 * 是 S07 的 ⑦（走端點 62R，`CapInvoiceTaxRemedy`），不是這裡。
 *
 * ── 為什麼是獨立路由而不是 Drawer ──────────────────────────────────────
 * repo 有反例（`shipments/anomalies` 已被 redirect 掉、改成列內 badge），所以這個
 * 決定要寫下來：① 它是 P2 賣家通知**信件**深連結的唯一落點，Drawer 沒有 URL；
 * ② 它同時拿得到 `jobId`（路由 param）與 `version`（端點 33）；③ 發票異常與物流
 * 異常的處置方向相反——物流異常收進訂單列是因為賣家在那裡就能處置（重新預約取件），
 * 而發票異常賣家沒有重試動作，這是一頁「解釋 ＋ 求助」，塞進 badge 會把出路藏起來。
 *
 * ── 本頁沒有「重新嘗試」，這是刻意的、不是暫緩 ──────────────────────────
 * 重試會實際打 provider，而賣家側無法判斷 `WAITING_CONFIRMATION` 下重送是否安全
 * ——結果不明時重送可能開出第二張發票，而發票開錯只能作廢或折讓且作廢有期限。
 * `/app/seller/invoices/jobs/{job_id}/retries` 這支端點**不存在**，也不要為它發明
 * `POST .../assistance-requests` 之類的新端點（會同時撞 `expectedSellerRoutes` 的
 * 雙向斷言與 58 §6 的端點總數）。「聯繫客服」走既有入口。
 *
 * ── ⑤「回報自行開立的發票號碼」已接上（2026-09-02，端點 62）────────────────
 * `ReportExternalIssuance` 在 `internal/grpc/seller_write.go`；先前那句「RPC 尚未
 * 實作、打了必 503」已經過期。⑤a（已回報的唯讀卡）與 ⑤（回報動作）現在並存：
 * 前者看 `RESOLVED_EXTERNALLY`，後者看 `canReportExternal`。
 *
 * 🔴 ⑤ 的唯一判準是後端的 `canReportExternal` 布林，**不得從 `status` 自己推導**
 * （§B-2.0 T23）：今天它只依賴 status，明天加上「解鎖已逾期失效」之類條件時，
 * 前端那份推導會靜默過期——按鈕還在、按下去 9349。
 *
 * ⚠️ 這一頁仍然**沒有「重新嘗試」**（見上一段），兩者不要混為一談：回報是「我已經
 * 自己開好了」，重試是「請平台再打一次 provider」。前者由賣家掌握結果，後者不是。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { INVOICE_OPERATION_TYPE_LABEL } from "@/lib/codemeta/enums";
import { InvoiceOperationStatus } from "@/lib/codemeta/generated";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import { useReportExternalIssuance } from "@/lib/hooks/useSellerInvoiceRemedy";
import { useSellerInvoiceJob } from "@/lib/hooks/useSellerInvoices";
import { manualReasonNotice } from "@/lib/invoices/manualReasonNotice";
import type { InvoiceJobInfo } from "@/lib/invoices/seller-types";
import { DeadlineCountdown } from "../components/DeadlineCountdown";
import {
	invoiceOperationDisplay,
	isInvoiceOperationDone,
	isInvoiceOperationResolvedExternally,
} from "../invoiceOperationDisplay";
import {
	docNoun,
	documentsPageLabel,
	invoiceSectionLabel,
	isReceiptKind,
} from "../receiptFraming";
import { ReportExternalIssuanceDialog } from "./ReportExternalIssuanceDialog";

const DOCUMENTS_HREF = "/seller-center/invoices/documents";
/** 後端未給 `supportContactUrl` 時的既有客服入口（賣家中心訊息）。 */
const SUPPORT_FALLBACK_HREF = "/seller-center/messages";

/** 空值一律 `—`，不猜。 */
function dash(value: string): string {
	return value || "—";
}

export function SellerInvoiceJobPage({ jobId }: { jobId: string }) {
	const { isSuspended } = useSellerIdentity();
	const { job, loading, error, refresh } = useSellerInvoiceJob(jobId);

	// backend#3066 裁決 B：收據與發票在賣家中心是兩個並列的東西，文案各自為政。
	// 🔴 判準來自後端的 documentKind，不從 operationType 推導（那個 enum 沒有 RECEIPT）。
	// job 尚未載入時 noun 退回「發票」——見 receiptFraming.ts 對這個預設的說明。
	const noun = docNoun(job?.documentKind);

	return (
		<SellerPageShell
			title={`${noun}異常詳情`}
			eyebrow={invoiceSectionLabel(job?.documentKind)}
			actions={
				<BackLink fallbackHref={DOCUMENTS_HREF} className="text-sm underline">
					返回紀錄
				</BackLink>
			}
		>
			<Link href={DOCUMENTS_HREF} className="text-sm underline">
				返回{documentsPageLabel(isReceiptKind(job?.documentKind))}
			</Link>

			{isSuspended ? (
				<section role="status" className="seller-card flex flex-col gap-2 p-6">
					<strong className="text-base font-bold">
						帳號停權期間無法查看{noun}異常詳情。
					</strong>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						已開立的{noun}
						仍會正常寄送給買家。若這筆已由你自行開立，請到該筆訂單的詳情頁查看。
					</p>
				</section>
			) : loading ? (
				<JobSkeleton />
			) : error || !job ? (
				// 端點 33 對「越權」與「真的不存在」回同一個 404＋9340（Tier A）⇒
				// 文案必須完全相同，**不得寫「你沒有權限查看」**，那會讓這一頁變成一個
				// 可枚舉的 oracle。503 也走這一支，所以重試鍵恆在（而不是把整頁畫成停用）。
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						找不到這筆{noun}工作。
					</p>
					<div className="flex flex-wrap items-center justify-center gap-[var(--hkui-space-3)]">
						<Button size="lg" variant="bordered" onClick={() => void refresh()}>
							重試
						</Button>
						<Link href={DOCUMENTS_HREF} className="text-sm underline">
							返回紀錄
						</Link>
					</div>
				</section>
			) : (
				<JobBody job={job} />
			)}
		</SellerPageShell>
	);
}

function JobSkeleton() {
	return (
		<div
			role="status"
			aria-busy="true"
			aria-label="載入中"
			className="flex flex-col gap-[var(--hkui-space-4)]"
		>
			{["a", "b", "c", "d"].map((key) => (
				<section
					key={key}
					className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]"
				>
					<span className="block h-3 w-1/3 animate-pulse rounded bg-[var(--hkui-default-200)] motion-reduce:animate-none motion-reduce:opacity-70" />
					<span className="block h-3 w-2/3 animate-pulse rounded bg-[var(--hkui-default-200)] motion-reduce:animate-none motion-reduce:opacity-70" />
				</section>
			))}
			<span className="visually-hidden">資料載入中</span>
		</div>
	);
}

function JobBody({ job }: { job: InvoiceJobInfo }) {
	// 這裡 job 一定已載入 ⇒ documentKind 是真值，不是 fallback（#3066 裁決 B）。
	const noun = docNoun(job.documentKind);
	const display = invoiceOperationDisplay(job.status);
	// 「為什麼卡住」。後端一直有給 manualReasonCode，而畫面層在 2026-09-05 之前
	// 零消費者 —— 而通知文案逐字叫賣家「至賣家中心查看原因」。
	const reason = manualReasonNotice(job.manualReasonCode);
	const done = isInvoiceOperationDone(job.status);
	const resolvedExternally = isInvoiceOperationResolvedExternally(job.status);
	const orderHref = job.orderNo
		? `/seller-center/orders/${encodeURIComponent(job.orderNo)}`
		: "";

	return (
		<div className="flex flex-col gap-[var(--hkui-space-4)]">
			{/* ② 摘要卡。`summary` 由後端保證「使用者可理解、不含 provider code」⇒
			    原樣印出，不重寫、不加工。異常摘要本身**不用 role="alert"**——進頁時
			    就存在的狀態不是即時事件，alert 會打斷螢幕閱讀器。 */}
			<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
				<div className="flex flex-wrap items-center justify-between gap-2">
					<h2 className="m-0 text-base font-bold">{dash(job.summary)}</h2>
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
				</div>
				{done ? (
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						這筆已經處理完成，不需要你做任何事。
					</p>
				) : null}
				{resolvedExternally ? (
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						這筆由你自行開立並回報完成，平台不會再次開立。
					</p>
				) : null}
				{/* 原因 ＋ 行動指引。**只在後端真的給了認得的碼時渲染**，認不得就整段不出現
				    ——把 SELLER_TRACK_INSUFFICIENT 這種字丟給賣家比不說更糟。
				    ⚠️ `!done` 是第二道防線：後端已在 MarkInvoiceDocumentIssued 清掉舊理由，
				    但那條路只要漏一個清除者，這裡就會在「已開立」旁邊顯示上一次的失敗原因。
				    兩層都要——單靠後端，漏掉的那天畫面會自相矛盾而沒有人會發現。
				    字軌不足那一格的處置與其他失敗相反（重試無效、要去財政部配號），
				    看不到它的賣家只會反覆按重試。 */}
				{reason.label && !done ? (
					<div className="m-0 rounded-hkui-md border border-warning-200 bg-warning-50 p-3">
						<p className="m-0 text-sm font-bold text-warning-700">
							原因：{reason.label}
						</p>
						{reason.hint ? (
							<p className="m-0 mt-1 text-[0.8125rem] leading-relaxed text-warning-700">
								{reason.hint}
							</p>
						) : null}
					</div>
				) : null}
				{/* 恆在的 info 行。文案優先用後端給的固定字串，前端不自寫（fallback 只在
				    後端漏給時用）。 */}
				<p
					role="status"
					className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]"
				>
					{job.shipmentUnaffectedNotice ||
						`${noun}異常不影響此訂單的出貨流程。`}
				</p>
			</section>

			{/* ③ 訂單快照 */}
			<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
				<h2 className="m-0 text-sm font-bold">訂單資訊</h2>
				<dl className="seller-order-detail__info">
					<div>
						<dt>訂單編號</dt>
						<dd>
							{orderHref ? (
								<Link href={orderHref} className="underline">
									<code>{job.orderNo}</code>
								</Link>
							) : (
								"—"
							)}
						</dd>
					</div>
					<div>
						<dt>操作類型</dt>
						<dd>
							{INVOICE_OPERATION_TYPE_LABEL[job.operationType] ??
								dash(job.operationType)}
						</dd>
					</div>
				</dl>
			</section>

			<RetryProgressCard job={job} />

			{/* ⑤a 只有 RESOLVED_EXTERNALLY 才有值（三欄皆 omitempty）。 */}
			{resolvedExternally ? <ReportedCard job={job} /> : null}

			{/* ⑤ 回報動作。判準是 `canReportExternal`，不是 status——見檔頭。 */}
			{job.canReportExternal ? <ReportExternalCard job={job} /> : null}

			{/* ⑥ 技術細節：原生 <details>，預設收合（不加 `open`）。 */}
			{job.providerRequestId ? (
				<section className="seller-card p-[var(--hkui-space-4)]">
					<details className="text-sm">
						<summary className="cursor-pointer font-bold">
							提供客服參考的技術資訊
						</summary>
						<div className="pt-3">
							<dl className="seller-order-detail__info">
								<div>
									<dt>追蹤編號</dt>
									<dd>
										<CopyableTrace value={job.providerRequestId} />
									</dd>
								</div>
							</dl>
						</div>
					</details>
				</section>
			) : null}

			{/* ⑦ footer：次要 CTA。`supportContactUrl` 不在 DTO 上（gateway 的
			    `InvoiceJobInfo` 21 欄逐欄查過），所以恆走既有客服入口。已回報。 */}
			<footer className="flex flex-wrap items-center gap-[var(--hkui-space-3)]">
				<Link href={SUPPORT_FALLBACK_HREF} className="text-sm underline">
					聯繫客服
				</Link>
				{orderHref ? (
					<Link href={orderHref} className="text-sm underline">
						查看訂單
					</Link>
				) : null}
			</footer>
		</div>
	);
}

/**
 * ④ 重試進度。
 *
 * 🔴 `retryPolicy` 的四欄是特意寫進 DTO 的，**要真的顯示出來**、不要只放進
 * `<details>`：賣家看到「已嘗試 3／5 次」卻不知道下一次什麼時候、還會不會再試，
 * 就只能去按一顆本頁刻意沒有的重試鍵。
 *
 * ⚠️ 「已嘗試 N／M 次」的 M 取 `job.maxAttempts`（**那一列建立當下寫入的值**），
 * 政策說明句的「最多幾次」取 `retryPolicy.maxAttempts`（**現在的政策值**）。舊列可能
 * 是舊政策，混用會讓「還剩幾次」算錯。
 */
/**
 * AUTO_RETRYING_STATUSES ＝ 這件作業**還在自動線上**（系統之後還會自己再送一次）。
 *
 * 🔴 正面列舉而不是「排除終態」（backend #3089 第 14 項）：`FAILED_PERMANENT`／
 * `FAILED_RETRY_EXHAUSTED`／`SELLER_ACTION_REQUIRED`／`CANCEL_BLOCKED_UNKNOWN`
 * 這幾個在 CodeMeta 上**不是終態**（ADM 或賣家介入後還會動），但系統**不會**
 * 自己再送 —— 對它們說「系統會自動重試」一樣是假的。
 *
 * 值域取自 backend 的 `db/queries/invoice_operations.sql`：
 * `FetchDueInvoiceOperations`（INV.op 的候選）＋ `FetchPendingInvoiceRechecks`
 * （INV.rc 的候選）兩組述詞的聯集 —— 那才是「還有排程器會撿它」的定義。
 */
const AUTO_RETRYING_STATUSES: ReadonlySet<string> = new Set([
	InvoiceOperationStatus.CREATED,
	InvoiceOperationStatus.AWAITING_TRIGGER,
	InvoiceOperationStatus.READY,
	InvoiceOperationStatus.HELD,
	InvoiceOperationStatus.SENDING,
	InvoiceOperationStatus.RETRY_SCHEDULED,
	InvoiceOperationStatus.WAITING_CONFIRMATION,
	InvoiceOperationStatus.PROVIDER_STATUS_RECHECK,
	InvoiceOperationStatus.CANCEL_REQUESTED,
]);

function RetryProgressCard({ job }: { job: InvoiceJobInfo }) {
	const policy = job.retryPolicy;
	const hasPolicy =
		policy.initialIntervalSeconds !== "" && policy.maxAttempts !== "";
	// 🔴 這件作業還會不會被系統自己再送一次。false ⇒ **不要畫「下一次重試」
	// 與政策說明句**：那兩個對已經停下來的作業一定是假的，而賣家會照著它等
	// 一個永遠不會到的時間（backend #3089 第 14 項，dev 實測 NOT_APPLICABLE
	// 的作業仍顯示「系統會自動重試…最多 13 次」＋一個「下一次重試」時間）。
	const stillAutoRetrying = AUTO_RETRYING_STATUSES.has(job.status);

	return (
		<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
			<h2 className="m-0 text-sm font-bold">系統重試進度</h2>
			<dl className="seller-order-detail__info">
				<div>
					{/* 🔴 「已重試」不是「已嘗試」（backend #3089 第 12 項）：
					    attempt_count 由 pkg/outbox 的 bumpAttempt 維護，而那支**只在
					    失敗時**被呼叫 ⇒ 它的語意是「已失敗次數」，0 ＝ 從未失敗。
					    寫「已嘗試 0 次」會讓一張**成功開出來**的發票看起來從沒送過。 */}
					<dt>已重試</dt>
					<dd>{`${dash(job.attemptCount)} / ${dash(job.maxAttempts)} 次`}</dd>
				</div>
				<div>
					<dt>最後嘗試</dt>
					<dd>{dash(formatTaipeiDateTime(job.lastAttemptAt))}</dd>
				</div>
				{stillAutoRetrying ? (
					<div>
						<dt>下一次重試</dt>
						<dd className="flex flex-wrap items-baseline gap-2">
							<span>{dash(formatTaipeiDateTime(job.nextRetryAt))}</span>
							<DeadlineCountdown deadlineAt={job.nextRetryAt} />
						</dd>
					</div>
				) : null}
			</dl>
			{stillAutoRetrying && hasPolicy ? (
				<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
					{`系統會自動重試：首次間隔 ${policy.initialIntervalSeconds} 秒，每次加倍，最長間隔 ${policy.maxIntervalSeconds} 秒，最多 ${policy.maxAttempts} 次。`}
				</p>
			) : (
				<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
					這件作業目前不會由系統自動重送。狀態與後續處理方式見上方「目前狀態」。
				</p>
			)}
		</section>
	);
}

/**
 * ⑤ 回報自行開立（端點 62）。
 *
 * 🔴 `expectedVersion` 取自**本頁剛讀到的那筆作業**（端點 33 的 `job.version`）。
 * 寫死或傳 0 都不是樂觀鎖；對不上時後端回 9343，而那正是「有人在你填表時動過
 * 這筆」的訊號。
 *
 * ⚠️ 成功之後不留 dialog、也不自己改畫面：mutation 會 invalidate 端點 33，
 * 重讀回來的 `status` 變成 `RESOLVED_EXTERNALLY`，⑤a 自然接手、⑤ 自然消失。
 * 自己維護一份「已送出」的本地狀態等於造出第二份真相。
 */
function ReportExternalCard({ job }: { job: InvoiceJobInfo }) {
	const [open, setOpen] = React.useState(false);
	const { report, reporting, error, succeeded, inProgress, reset } =
		useReportExternalIssuance({
			jobId: job.jobId,
			orderNo: job.orderNo,
			expectedVersion: job.version,
		});

	// 成功即關閉。重讀後 `canReportExternal` 會變 false，整張卡跟著不見。
	React.useEffect(() => {
		if (succeeded) setOpen(false);
	}, [succeeded]);

	return (
		<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
			<h2 className="m-0 text-sm font-bold">回報自行開立的發票號碼</h2>
			<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
				{
					"如果你已經用自己的開票系統開好這張發票，把號碼回報進來，平台就不會再開一次。"
				}
			</p>
			<div className="mt-1">
				<Button
					size="lg"
					variant="bordered"
					onClick={() => {
						reset();
						setOpen(true);
					}}
				>
					回報發票號碼
				</Button>
			</div>
			<ReportExternalIssuanceDialog
				open={open}
				onClose={() => setOpen(false)}
				onSubmit={report}
				submitting={reporting}
				inProgress={inProgress}
				error={error}
			/>
		</section>
	);
}

/** ⑤a 已回報唯讀卡。 */
function ReportedCard({ job }: { job: InvoiceJobInfo }) {
	return (
		<section
			role="status"
			className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]"
		>
			<h2 className="m-0 text-sm font-bold">你自行開立的發票</h2>
			<dl className="seller-order-detail__info">
				<div>
					<dt>發票號碼</dt>
					<dd>{dash(job.externalInvoiceNo)}</dd>
				</div>
				<div>
					<dt>發票日期</dt>
					<dd>{dash(formatTaipeiDate(job.externalInvoiceDate))}</dd>
				</div>
				<div>
					<dt>回報時間</dt>
					<dd>{dash(formatTaipeiDateTime(job.reportedAt))}</dd>
				</div>
			</dl>
			<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
				這筆由你自行開立並回報完成，平台不會再次開立。
			</p>
		</section>
	);
}

/** 追蹤編號 ＋ 複製鍵（形狀照 `orders/OrderDetailClient.tsx` 的訂單編號複製）。 */
function CopyableTrace({ value }: { value: string }) {
	const [copied, setCopied] = React.useState(false);

	return (
		<span className="flex flex-wrap items-center gap-2">
			<code className="break-all">{value}</code>
			<Button
				size="sm"
				variant="bordered"
				onClick={() => {
					void navigator.clipboard?.writeText(value).then(() => {
						setCopied(true);
						window.setTimeout(() => setCopied(false), 2000);
					});
				}}
			>
				{copied ? "已複製" : "複製"}
			</Button>
		</span>
	);
}

export default SellerInvoiceJobPage;
