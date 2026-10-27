"use client";

/**
 * S07｜賣家訂單詳情的發票區（58A §S07）。**不是新路由**——掛進既有的
 * `/seller-center/orders/[orderNo]`，插在右側 `<aside>` 的「付款」卡之後、售後卡之前。
 *
 * ── 兩件事必須並存且互不影響 ────────────────────────────────────────────
 * 發票卡處在紅色異常態時，左欄「出貨資訊」的按鈕**不得被 disable、不得加鎖 icon、
 * 不得灰化**。實作上的保證不是紀律而是結構：本元件是**純葉節點**，只吃 `orderNo`
 * 一個 prop、不回傳任何 state 給 `OrderDetailClient` ⇒「發票異常 disable 出貨鍵」
 * 在結構上就做不出來。改動本檔時不要新增 callback prop。
 *
 * ── 資料源：三支查詢，兩種權限 ──────────────────────────────────────────
 * 賣家側**沒有 order-scoped 的發票端點**（買家側有端點 8，賣家側逐條核對沒有），
 * 所以走兩段：
 *   段 1  端點 30 `?order_no=` → 拿 `documentId`、狀態、`invoiceMode`
 *   段 2  端點 31 `documents/{id}` → 明細（**沒有 `invoiceMode`**，只能從段 1 拿）
 * 兩段都是 `CapInvoiceManage`／停權 BLOCK ⇒ 查詢前就 gate。
 *
 * 🔴 獨立第三支：端點 62R `orders/{order_no}/external_issuances` 是
 * `CapInvoiceTaxRemedy`／**停權放行**，它的 `enabled` 只看 `orderNo`，**不得**掛在
 * 段 1／段 2 的成功狀態上——掛上去等於「停權賣家永遠看不到 ⑦」，而那正是端點 62
 * 被放行的目的。未停權時兩種寫法看起來一模一樣（§B-2.0 T05）。
 *
 * ── ⑦ 回報自行開立：唯讀 ＋ 動作並存（端點 62，2026-09-02 接上）───────────
 * `ReportExternalIssuance` 在 `internal/grpc/seller_write.go`；先前那句「寫入 RPC
 * 尚未實作、打了必 503」已經過期。62R 回來的每一列各有自己的閘門：
 *   `reportedAt !== ""`      → ⑦a 已回報的唯讀卡
 *   `canReportExternal`      → ⑦b 回報動作（**判準是這個布林，不是 status**）
 * 兩者不互斥地寫在一起：一張訂單可能同時有一筆已回報、一筆待回報。
 *
 * ⚠️ 「重新寄送通知」（端點 32）仍**不在本卡**：`nextResendAvailableAt` 後端恆為
 * 空字串，而空字串在前端語意是「可以重寄」⇒ 會變成一顆永遠可按、按了才知道被
 * 節流的鍵。重寄鍵在 S11（發票明細頁）——那裡拿得到端點 31 的完整欄位。
 *
 * 也沒有「重新開立／重新嘗試」（賣家端沒有 retries 端點，安全重試只在 ADM 側）、
 * 沒有「轉人工開票」捷徑（只能由 ADM 雙人解鎖打開）、沒有任何可修改金額的欄位。
 *
 * ⚠️ 異常態的「查看異常」連到 S08（`/seller-center/invoices/jobs/[jobId]`）。它是
 * `CapInvoiceManage`／停權 BLOCK ⇒ 停權時本卡整段不渲染，那條連結自然也不在——
 * 停權賣家的回報落點是本卡的 ⑦（端點 62R），不是 S08。
 */

import * as React from "react";
import Link from "@/components/shared/Link";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import {
	INVOICE_FULFILLMENT_MODE_LABEL,
	INVOICE_MANUAL_REASON_LABEL,
	InvoiceFulfillmentMode,
} from "@/lib/codemeta/enums";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useOrderExternalIssuances,
	useReportExternalIssuance,
} from "@/lib/hooks/useSellerInvoiceRemedy";
import {
	useSellerInvoiceDocument,
	useSellerInvoiceDocuments,
} from "@/lib/hooks/useSellerInvoices";
import { invoiceFileUnavailableHint } from "@/lib/invoices/orderInvoiceState";
import { sellerInvoiceFileUrl } from "@/lib/invoices/seller-repo";
import type {
	InvoiceJobInfo,
	SellerInvoiceDocumentDetail,
	SellerInvoiceDocumentItem,
} from "@/lib/invoices/seller-types";
import { InvoiceSnapshotBreakdown } from "./components/InvoiceSnapshotBreakdown";
import {
	invoiceDocumentDisplay,
	isInvoiceDocumentAbnormal,
	isInvoiceDocumentProcessing,
} from "./invoiceDocumentDisplay";
import { ReportExternalIssuanceDialog } from "./jobs/ReportExternalIssuanceDialog";
import { pickPrimaryInvoiceDocument } from "./pickPrimaryInvoiceDocument";

interface Props {
	orderNo: string;
}

export function SellerOrderInvoiceCard({ orderNo }: Props) {
	const { isSuspended } = useSellerIdentity();

	// 段 1。`page_size: 10` —— 一張訂單的發票文件數量遠小於此（作廢重開會有兩筆）。
	const listParams = React.useMemo(
		() => ({ orderNo, page: 1, pageSize: 10 }),
		[orderNo],
	);
	const { page: list, loading: listLoading } =
		useSellerInvoiceDocuments(listParams);

	// 挑主卡的規則抽成純函式（含跨 repo 排序契約的說明與 dev 實測證據），
	// 這樣它才測得到——原本的 inline 取值沒有任何測試守著，於是
	// 「後端依 issuedAt 遞增」這個錯的假前提一路活到 dev。
	const items = list?.records ?? [];
	const primary: SellerInvoiceDocumentItem | null =
		pickPrimaryInvoiceDocument(items);

	// 段 2。
	const { detail, loading: detailLoading } = useSellerInvoiceDocument(
		primary?.summary.documentId ?? "",
	);

	// 獨立第三支——見檔頭。
	const { records: issuances } = useOrderExternalIssuances(orderNo);
	const reported = issuances.find((job) => job.reportedAt !== "");
	// 🔴 閘門是後端的 `canReportExternal`，不是 `reportedAt === ""`——後者對「還沒
	// 解鎖人工處理」的作業也成立，會渲染一顆必回 9349 的鍵（§B-2.0 T23）。
	const reportable = issuances.find((job) => job.canReportExternal);

	const mode = primary?.invoiceMode ?? "";
	const isReceipt = mode === InvoiceFulfillmentMode.PLATFORM_RECEIPT;
	const isSelfIssue = mode === InvoiceFulfillmentMode.SELLER_SELF_ISSUE;
	const status = primary?.summary.status ?? "";
	const display = invoiceDocumentDisplay(status);
	const jobId = detail?.latestJobId || primary?.latestJobId || "";

	return (
		<section className="seller-card">
			<header className="seller-card__head flex items-center justify-between gap-3">
				<h2>{isReceipt ? "收據" : "發票與收據"}</h2>
				{primary ? (
					<Chip variant="flat" color={display.color} size="sm">
						{display.label}
					</Chip>
				) : null}
			</header>

			{isSuspended ? (
				<p role="status" className="m-0 p-4 text-sm">
					帳號停權期間無法查看發票明細。已開立的發票仍會正常寄送給買家。
				</p>
			) : listLoading ? (
				<p className="m-0 p-4 text-sm text-[var(--hkui-foreground-soft)]">
					載入中…
				</p>
			) : !primary ? (
				<p className="m-0 p-4 text-sm text-[var(--hkui-foreground-soft)]">
					這張訂單尚未付款；發票會在付款成功後開立。
				</p>
			) : (
				<div className="flex flex-col gap-[var(--hkui-space-3)] p-4 pt-0">
					{isInvoiceDocumentAbnormal(status) ? (
						<section
							role="alert"
							className="flex flex-col gap-1 rounded-hkui-lg border border-[var(--hkui-warning-200)] bg-warning-50 p-3 text-warning-700"
						>
							<strong className="text-sm font-bold">
								{display.label}
								{detail?.manualReasonCode
									? `：${INVOICE_MANUAL_REASON_LABEL[detail.manualReasonCode] ?? detail.manualReasonCode}`
									: ""}
							</strong>
							<p className="m-0 text-[0.8125rem]">
								平台會繼續處理這筆發票，你不需要重新開立。
							</p>
							{/*
							 * 🔴 `latestJobId` 為空字串時**整顆不渲染**，不要渲染 disabled 鍵，
							 * 也不得拿 `documentId` 去組 `/jobs/{jobId}`——打進去會 404＋9340，
							 * 而那個 404 與「不是你的」長得完全一樣（Tier A，§B-2.0 T06）。
							 * 這裡優先取端點 31 的 `latestJobId`（明細比列表新），列表那份當備援。
							 */}
							{jobId ? (
								<Link
									href={`/seller-center/invoices/jobs/${encodeURIComponent(jobId)}`}
									className="text-sm underline"
								>
									查看異常
								</Link>
							) : null}
						</section>
					) : null}

					{isInvoiceDocumentProcessing(status) ? (
						<p
							role="status"
							className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]"
						>
							系統正在開立發票，完成後會通知買家。
						</p>
					) : null}

					{isSelfIssue ? (
						<p
							role="status"
							className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]"
						>
							這張訂單由你自行開立發票。平台不會替你開立或交付。
						</p>
					) : null}

					{/* 恆在的那一行——出貨與發票是兩條獨立的線。 */}
					<p
						role="status"
						className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]"
					>
						此狀態不影響這張訂單的出貨流程。
					</p>

					<dl className="seller-order-detail__info">
						<Row
							label="開票方式"
							value={INVOICE_FULFILLMENT_MODE_LABEL[mode] ?? mode}
						/>
						{/*
						 * 遮罩值用 <span> 而非 <code>，避免螢幕閱讀器逐字讀 `*`。
						 * 端點 31 **沒有 `notifiedEmailMasked`**（那一欄在買家端點 8）⇒
						 * 「通知寄送對象」那一列整列不渲染，不得用賣家自己的 email 頂替
						 * （§B-2.0 T21）。
						 */}
						<Row label="發票號碼" value={primary.summary.invoiceNumber} />
						<Row
							label="開立時間"
							value={formatTaipeiDateTime(primary.summary.issuedAt)}
						/>
						{detailLoading ? (
							<div>
								<dt>連線版本</dt>
								<dd>
									<span className="block h-3 w-16 animate-pulse rounded bg-[var(--hkui-default-200)] motion-reduce:animate-none motion-reduce:opacity-70" />
								</dd>
							</div>
						) : (
							<Row label="連線版本" value={detail?.cutoverVersion ?? ""} />
						)}
					</dl>

					{detail ? (
						<details className="text-sm">
							<summary className="cursor-pointer font-bold">
								金額與稅額明細
							</summary>
							<div className="pt-3">
								<InvoiceSnapshotBreakdown detail={detail} />
							</div>
						</details>
					) : null}

					{reported ? <ReportedIssuance job={reported} /> : null}

					{reportable ? (
						<ReportExternalIssuance job={reportable} orderNo={orderNo} />
					) : null}

					<ViewDocumentLink detail={detail} primary={primary} />
				</div>
			)}
		</section>
	);
}

/**
 * ⑦a 的唯讀部分：這筆已由賣家自行開立並回報完成。
 *
 * 62R 對停權賣家也放行 ⇒ 停權期間仍看得到自己回報過什麼；否則賣家自行開立之後
 * 在訂單頁看不到任何痕跡。回報**動作**是下面的 `ReportExternalIssuance`。
 */
function ReportedIssuance({ job }: { job: InvoiceJobInfo }) {
	return (
		<section
			role="status"
			className="flex flex-col gap-2 rounded-hkui-lg border border-divider bg-content2 p-3"
		>
			<strong className="text-sm font-bold">自行開立的發票</strong>
			<dl className="m-0 flex flex-col gap-1 text-[0.8125rem]">
				<Row label="發票號碼" value={job.externalInvoiceNo} inline />
				<Row
					label="發票日期"
					value={formatTaipeiDate(job.externalInvoiceDate)}
					inline
				/>
				<Row
					label="回報時間"
					value={formatTaipeiDateTime(job.reportedAt)}
					inline
				/>
			</dl>
			<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
				這筆已由你自行開立並回報完成，平台不會再次開立。
			</p>
		</section>
	);
}

/**
 * ⑦b 回報動作（端點 62）。
 *
 * 🔴 `expectedVersion` 取自 **62R 剛回來的那一列**的 `version`，不是端點 30／31 的
 * 任何欄位——那兩支查的是 document，這裡的樂觀鎖鎖的是 operation，兩個版本號各自
 * 獨立。拿錯一個，賣家會在一個沒人動過的作業上收到 9343。
 *
 * 🔴 本元件**只依賴 62R**（`CapInvoiceTaxRemedy`／停權放行）。它是停權賣家唯一還
 * 走得通的回報路徑（S08 對停權 BLOCK），所以不得掛上任何 `!isSuspended` 或段 1／
 * 段 2 的成功條件。
 */
function ReportExternalIssuance({
	job,
	orderNo,
}: {
	job: InvoiceJobInfo;
	orderNo: string;
}) {
	const [open, setOpen] = React.useState(false);
	const { report, reporting, error, succeeded, inProgress, reset } =
		useReportExternalIssuance({
			jobId: job.jobId,
			orderNo,
			expectedVersion: job.version,
		});

	// 成功即關閉；62R 重讀後 `canReportExternal` 變 false，整段自然換成 ⑦a。
	React.useEffect(() => {
		if (succeeded) setOpen(false);
	}, [succeeded]);

	return (
		<section className="flex flex-col gap-2 rounded-hkui-lg border border-divider bg-content2 p-3">
			<strong className="text-sm font-bold">回報自行開立的發票</strong>
			<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
				{"如果你已經自己開好這張發票，把號碼回報進來，平台就不會再開一次。"}
			</p>
			<div>
				<Button
					size="sm"
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

/**
 * 「查看文件」。條件是 `fileAvailable === true`——它與 PDF 端點會回的 `9312`
 * （載具未中獎）／`9313`（逾查詢期限）是同一個判定，先打一次探測只是多燒一次
 * 限流額度。
 */
function ViewDocumentLink({
	detail,
	primary,
}: {
	detail: SellerInvoiceDocumentDetail | null;
	primary: SellerInvoiceDocumentItem;
}) {
	const available =
		detail?.summary.fileAvailable ?? primary.summary.fileAvailable;
	const reason =
		detail?.summary.fileUnavailableReason ??
		primary.summary.fileUnavailableReason;

	if (!available) {
		return reason ? (
			<p className="m-0 text-[0.75rem] text-[var(--hkui-foreground-soft)]">
				{invoiceFileUnavailableHint(reason)}
			</p>
		) : null;
	}

	return (
		<div className="flex flex-col gap-1">
			<a
				href={sellerInvoiceFileUrl(primary.summary.documentId)}
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

function Row({
	label,
	value,
	inline,
}: {
	label: string;
	value: string;
	inline?: boolean;
}) {
	if (!value) return null;
	if (inline) {
		return (
			<div className="flex items-center justify-between gap-3">
				<dt className="text-[var(--hkui-foreground-soft)]">{label}</dt>
				<dd className="m-0">{value}</dd>
			</div>
		);
	}
	return (
		<div>
			<dt>{label}</dt>
			<dd>{value}</dd>
		</div>
	);
}

export default SellerOrderInvoiceCard;
