"use client";

/**
 * S11｜賣家發票詳情（58A §S11，document-scoped，本設計新增）。
 * 端點 31 `GET /app/seller/invoices/documents/{document_id}` 是本頁唯一的資料源。
 *
 * ── 為什麼需要這一頁 ────────────────────────────────────────────────────
 * 舊版逐字寫「Phase 1 不做 S06 列內詳情 Drawer，『更多』一律導向訂單詳情」⇒ 賣家的
 * 發票詳情只存在於 S07（訂單詳情內的卡）。但**方向二（平台開給賣家的服務費發票）
 * 沒有訂單** ⇒ 那條路走不通，必須有獨立的 document-scoped 詳情頁。S12 的進項分頁
 * 每一列也只能導到這裡。
 *
 * ── 銷項／進項是同一頁的兩種語意 ────────────────────────────────────────
 * **銷項**（我開給買家的）可以作廢／折讓／重寄；**進項**（平台開給我的）唯讀，
 * 只能查看與下載。進項不得顯示任何作廢／折讓按鈕——那張票不是我開的，我沒有權限
 * 也沒有法律地位去作廢它。
 *
 * **重寄已於 2026-09-01 接上**（端點 32 `ResendInvoiceNotification`，
 * `internal/grpc/notification_resend.go`）；作廢與折讓仍不渲染——把 gateway
 * `invoice_seller.go` 呼叫的 24 支 RPC 逐一列出來對過，**賣家面沒有作廢／折讓的端點**，
 * 那兩個動作是 ADM 的權限。
 *
 * ⚠️ 原註解把重寄也算進「還沒有」，是憑記憶猜 RPC 名查出來的錯誤結論。判準改成
 * 從呼叫端反推，命令寫在 `lib/invoices/seller-repo.ts` 檔頭。
 *
 * 銷項／進項的分流用「入口帶進來的 `?kind=received`」——**端點 31 的回應沒有
 * `issuerType`／`recipientType` 兩欄**（gateway `SellerInvoiceDocumentDetail` 逐欄
 * 查過）⇒ 不自行從別的欄位推導出一個會靜默過期的判斷。已回報衝突。
 *
 * ── PDF ────────────────────────────────────────────────────────────────
 * `fileAvailable === true` 時才給連結；`false` 顯示後端給的 `fileUnavailableReason`
 * （9312 載具未中獎／9313 逾查詢期限）。**不先打一次探測**——那與 `fileAvailable`
 * 是同一個判定，只是多燒一次限流額度。
 * ⚠️ 目前 `GetInvoiceDocumentFile` 在 invoice-service 沒有實作、`pdf_object_key`
 * 也沒有任何寫入路徑 ⇒ 實務上 `fileAvailable` 恆為 false，這條連結不會出現。
 * 邏輯照契約寫，後端補上時不必再動版面。
 */

import { useSearchParams } from "next/navigation";
import * as React from "react";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { carrierRowKindOf } from "@/features/invoices/carrierDisplay";
import { INVOICE_DOCUMENT_KIND_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useSellerInvoiceDocument } from "@/lib/hooks/useSellerInvoices";
import { formatInvoiceAmount, hasRecipientTaxId } from "@/lib/invoices/display";
import { isInvoiceBizCode } from "@/lib/invoices/errorCopy";
import { invoiceFileUnavailableHint } from "@/lib/invoices/orderInvoiceState";
import {
	resendSellerInvoiceNotification,
	SELLER_INVOICE_BIZ_CODE,
	sellerInvoiceFileUrl,
} from "@/lib/invoices/seller-repo";
import type { SellerInvoiceDocumentDetail } from "@/lib/invoices/seller-types";
import { InvoiceSnapshotBreakdown } from "./components/InvoiceSnapshotBreakdown";
import { invoiceDocumentDisplay } from "./invoiceDocumentDisplay";

const DOCUMENTS_HREF = "/seller-center/invoices/documents";
const RECEIVED_HREF = "/seller-center/invoices/documents?tab=received";

function dash(value: string): string {
	return value || "—";
}

export function SellerInvoiceDetailPage({
	documentId,
}: {
	documentId: string;
}) {
	const searchParams = useSearchParams();
	const { isSuspended } = useSellerIdentity();
	const { detail, loading, error, refresh } =
		useSellerInvoiceDocument(documentId);
	/**
	 * 入口帶進來的語意提示（S12 的進項列連結帶 `?kind=received`）。
	 *
	 * 🔴 端點 31 的回應**沒有** `issuerType`／`recipientType`（gateway
	 * `SellerInvoiceDocumentDetail` 逐欄查過）⇒ 本頁不從別的欄位自行推導方向。
	 * 推導出來的東西會在後端補上真欄位時靜默過期，而錯的那一面（把進項標成銷項）
	 * 會讓賣家以為自己可以作廢平台開給他的發票。沒有提示時一律當銷項（S06 的入口）。
	 */
	const received = searchParams?.get("kind") === "received";
	const backHref = received ? RECEIVED_HREF : DOCUMENTS_HREF;

	return (
		<SellerPageShell
			title="發票詳情"
			eyebrow="發票與稅籍"
			subtitle={received ? "平台開立給你的憑證，唯讀。" : undefined}
			actions={
				<Link href={backHref} className="text-sm underline">
					返回紀錄
				</Link>
			}
		>
			{isSuspended ? (
				<section role="status" className="seller-card flex flex-col gap-2 p-6">
					<strong className="text-base font-bold">
						帳號停權期間無法查看發票明細。
					</strong>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						已開立的發票仍會正常寄送給買家。
					</p>
				</section>
			) : loading && !detail ? (
				<DocumentSkeleton />
			) : !detail ? (
				// 端點 31 對「越權」與「真的不存在」回同一個 404＋9301（Tier A）⇒
				// 文案必須完全相同，不得寫「沒有權限」。
				// 致命錯誤態判 `!detail` 而不是 `error || !detail`：背景 refetch 失敗時
				// 保留最後一份好資料，只有真的沒東西可顯示才換頁。
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						找不到這份發票文件。
					</p>
					<div className="flex flex-wrap items-center justify-center gap-[var(--hkui-space-3)]">
						{error ? (
							<Button
								size="lg"
								variant="bordered"
								onClick={() => void refresh()}
							>
								重試
							</Button>
						) : null}
						<Link href={backHref} className="text-sm underline">
							返回紀錄
						</Link>
					</div>
				</section>
			) : (
				<DocumentBody
					detail={detail}
					documentId={documentId}
					received={received}
				/>
			)}
		</SellerPageShell>
	);
}

function DocumentSkeleton() {
	return (
		<div
			role="status"
			aria-busy="true"
			aria-label="載入中"
			className="flex flex-col gap-[var(--hkui-space-4)]"
		>
			{["a", "b", "c"].map((key) => (
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

function DocumentBody({
	detail,
	documentId,
	received,
}: {
	detail: SellerInvoiceDocumentDetail;
	documentId: string;
	received: boolean;
}) {
	const { summary } = detail;
	const display = invoiceDocumentDisplay(summary.status);

	return (
		<div className="flex flex-col gap-[var(--hkui-space-4)]">
			<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
				<div className="flex flex-wrap items-center justify-between gap-2">
					<h2 className="m-0 text-sm font-bold">文件資訊</h2>
					<span className="flex flex-wrap items-center gap-2">
						<Chip variant="flat" size="sm">
							{received ? "進項" : "銷項"}
						</Chip>
						<Chip variant="flat" color={display.color} size="sm">
							{display.label}
						</Chip>
					</span>
				</div>
				<dl className="seller-order-detail__info">
					<div>
						<dt>文件種類</dt>
						<dd>
							{INVOICE_DOCUMENT_KIND_LABEL[summary.documentKind] ??
								dash(summary.documentKind)}
						</dd>
					</div>
					<div>
						{/* 發票號碼三端皆完整不遮（58 §8.1.3(c) 裁定 1）：它是對獎／報帳／
						    申訴的唯一憑據，而且同時印在發票上、寄在信裡、存在載具裡。 */}
						<dt>發票號碼</dt>
						<dd className="tabular-nums">{dash(summary.invoiceNumber)}</dd>
					</div>
					<div>
						<dt>開立時間</dt>
						<dd>{dash(formatTaipeiDateTime(summary.issuedAt))}</dd>
					</div>
					{/* 「開立人」兩種語意都要顯示，不得因為「反正是自己」就省略：
					    進項時它是平台的抬頭與統編，銷項時是自己的稅籍抬頭。 */}
					<div>
						<dt>開立人</dt>
						<dd>{dash(detail.issuerName)}</dd>
					</div>
					{detail.issuerTaxId ? (
						<div>
							<dt>開立人統編</dt>
							<dd className="tabular-nums">{detail.issuerTaxId}</dd>
						</div>
					) : null}
					{detail.recipientName ? (
						<div>
							<dt>買受人</dt>
							<dd>{detail.recipientName}</dd>
						</div>
					) : null}
					{/*
					 * 🔴 與 B05 共用同一個判準（#3012）：B2C 的 `recipientTaxId` 是 DB
					 * CHECK 強制的哨兵值 `"0000000000"`，純真值判斷會把它當資料印出來。
					 * 賣家面與買家面印的是同一份資料的同一個欄位，判準寫兩份就會分岔。
					 */}
					{hasRecipientTaxId(summary.documentKind, detail.recipientTaxId) ? (
						<div>
							<dt>買受人統編</dt>
							<dd className="tabular-nums">{detail.recipientTaxId}</dd>
						</div>
					) : null}
					{/* backend#3084：沒有載具時後端**刻意**把買受人統編放進 carrierSummary
					    （B2B 列表要看得到開給哪個統編，那是裁決 B 保留的行為）⇒ 只看
					    carrierSummary 分不出兩者，判別欄是 carrierType。上面已經有專屬的
					    「買受人統編」列，這裡再渲染一次就是同一頁兩列同值、而且下面那列
					    標成「載具」——那是在法定憑證上標錯欄位。
					    買家側（InvoiceDocumentDetailClient）已經這樣做，這裡是同一份判準的
					    第二個消費者，**複用 carrierRowKindOf 而不是再寫一份**。
					    dev 實測（2026-09-05，document 7501682805689024512）：DB 的 carrier
					    欄位全 NULL，而畫面顯示「載具 10000004」＝上一行的買受人統編。 */}
					{carrierRowKindOf(summary.carrierSummary, summary.carrierType) ===
					"carrier" ? (
						<div>
							<dt>載具</dt>
							{/* 遮罩值用 <span> 而非 <code>，避免螢幕閱讀器逐字讀 `*`。 */}
							<dd>
								<span>{summary.carrierSummary}</span>
							</dd>
						</div>
					) : null}
					<div>
						<dt>金額</dt>
						<dd className="tabular-nums">
							{formatInvoiceAmount(summary.totalAmount) || "—"}
						</dd>
					</div>
				</dl>
				{summary.relatedDocumentId ? (
					<Link
						href={`/seller-center/invoices/${encodeURIComponent(summary.relatedDocumentId)}`}
						className="text-sm underline"
					>
						查看關聯文件
					</Link>
				) : null}
			</section>

			<section className="seller-card flex flex-col gap-3 p-[var(--hkui-space-4)]">
				<h2 className="m-0 text-sm font-bold">金額與稅額明細</h2>
				<InvoiceSnapshotBreakdown detail={detail} />
			</section>

			{/*
			 * 重寄通知（端點 32）。**只在銷項渲染**：進項是平台開給我的，
			 * 收件人不是我的買家，重寄沒有意義也不該由我發動。
			 * 銷項／進項由入口帶進來的 `?kind=received` 決定——端點 31 的回應沒有
			 * issuerType／recipientType 兩欄（見檔頭）。
			 */}
			{received ? null : <ResendNotice documentId={documentId} />}

			{detail.timeline.length > 0 ? (
				<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
					<h2 className="m-0 text-sm font-bold">處理歷程</h2>
					<ol className="m-0 flex list-none flex-col gap-2 p-0">
						{detail.timeline.map((item) => (
							<li
								key={`${item.occurredAt}-${item.event}`}
								className="flex flex-wrap items-baseline gap-2 border-b border-divider pb-2 text-sm last:border-b-0 last:pb-0"
							>
								<time
									dateTime={item.occurredAt}
									className="text-[0.75rem] text-[var(--hkui-foreground-soft)]"
								>
									{formatTaipeiDateTime(item.occurredAt)}
								</time>
								<span>{dash(item.note || item.event)}</span>
							</li>
						))}
					</ol>
				</section>
			) : null}

			<FileRow detail={detail} documentId={documentId} />
		</div>
	);
}

/**
 * PDF 連結。條件是 `fileAvailable === true` —— 它與 PDF 子路由會回的 `9312`／`9313`
 * 是同一個判定，先打一次探測只是多燒一次限流額度。
 */
function FileRow({
	detail,
	documentId,
}: {
	detail: SellerInvoiceDocumentDetail;
	documentId: string;
}) {
	if (!detail.summary.fileAvailable) {
		return detail.summary.fileUnavailableReason ? (
			<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
				{invoiceFileUnavailableHint(detail.summary.fileUnavailableReason)}
			</p>
		) : null;
	}

	return (
		<div className="flex flex-col gap-1">
			<a
				href={sellerInvoiceFileUrl(documentId)}
				target="_blank"
				rel="noopener noreferrer"
				className="text-sm underline"
			>
				查看文件
			</a>
			<small className="text-[var(--hkui-foreground-soft)]">
				請允許瀏覽器彈出新分頁。
			</small>
		</div>
	);
}

export default SellerInvoiceDetailPage;

/**
 * 重寄發票通知（端點 32）。
 *
 * 🔴 **節流走的是 `9352` 例外，不是 `resent: false`**（2026-09-01 dev 實測）。
 * 這裡曾經只認 `resent` 布林，於是「剛剛才寄過」整個掉進 `catch` 顯示成「寄送
 * 失敗」——賣家會以為系統壞了，然後去催一封其實已經寄出的信。兩條都留著：
 * 布林分支是防後端哪天改回 200＋false 的安全網，`9352` 分支才是現在走的那條。
 */
function ResendNotice({ documentId }: { documentId: string }) {
	const [state, setState] = React.useState<
		| { kind: "idle" }
		| { kind: "sending" }
		| { kind: "sent"; toMasked: string }
		| { kind: "throttled" }
		| { kind: "failed" }
	>({ kind: "idle" });

	const send = async () => {
		setState({ kind: "sending" });
		try {
			const r = await resendSellerInvoiceNotification(documentId);
			setState(
				r.resent
					? { kind: "sent", toMasked: r.toMasked }
					: { kind: "throttled" },
			);
		} catch (e) {
			setState(
				isInvoiceBizCode(
					e,
					SELLER_INVOICE_BIZ_CODE.NOTIFICATION_RESEND_THROTTLED,
				)
					? { kind: "throttled" }
					: { kind: "failed" },
			);
		}
	};

	return (
		<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
			<h2 className="m-0 text-sm font-bold">重寄通知</h2>
			<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
				{"買家說沒收到發票通知時，可以在這裡重寄一次。"}
			</p>
			<div className="mt-1">
				<Button
					size="lg"
					variant="bordered"
					isLoading={state.kind === "sending"}
					loadingLabel="寄送中…"
					onClick={() => void send()}
				>
					重寄通知
				</Button>
			</div>
			{state.kind === "sent" ? (
				<p role="status" className="m-0 text-[0.75rem] text-success-700">
					{`已重新寄出到 ${state.toMasked || "買家的信箱"}。`}
				</p>
			) : null}
			{state.kind === "throttled" ? (
				<p role="status" className="m-0 text-[0.75rem] text-warning-700">
					{"剛剛才寄過，暫時不會重複寄送。請稍後再試。"}
				</p>
			) : null}
			{state.kind === "failed" ? (
				<p role="alert" className="m-0 text-[0.75rem] text-danger-700">
					{"寄送失敗，請稍後再試。"}
				</p>
			) : null}
		</section>
	);
}
