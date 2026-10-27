"use client";

/**
 * S10｜買家申訴詳情（58A §S10）。端點 35 是本頁唯一的資料源。
 *
 * 權限：`CapOrderFulfill`，在停權白名單內（S09 的前綴 `/seller-center/invoices/appeals`
 * 已涵蓋本頁）。理由同 S09 的死結——對應的 hook 在 `useSellerInvoiceRemedy.ts`，
 * **不含 `!isSuspended`**。
 *
 * ── ⑥「回覆申訴」已於 2026-09-01 接上 ───────────────────────────────
 * 端點 36 `RespondSellerInvoiceComplaint` 的 RPC 在 `internal/grpc/seller_write.go`，
 * 解除條件（本註解原本寫的那條 grep）已滿足。
 *
 * ⚠️ 原註解說它「尚未實作」是**用錯名字查出來的結論**：查的是
 * `RespondSellerInvoiceComplaint`，這次是對的；但同一輪我用自己編的
 * `AuthorizeSellerReceipt` 查 S05，把一個做好的端點誤判成缺。
 * ⇒ 判準改成從 gateway handler 的呼叫點反推 RPC 名，不憑記憶猜。
 *
 * 已帶的東西（58A §S10 定版）：三選一 radio（`RESENT`／`REISSUED`／
 * `NEED_PLATFORM_HELP`，選最後一項時說明欄由前端加嚴為必填）、`expectedVersion`
 * **取自 `complaint.version`**（不得用 `responses.length` 頂替——平台側也會追加
 * 事件，兩個數字會分岔）。
 *
 * ⚠️ 回覆是**追加**語意：同一件案可回覆多次，所以送出成功後只收 dialog，
 * 整個區塊不收起來；按鈕文案在已有回覆時改成「補充說明」。
 *
 * ── ④「目前發票狀態卡」做不出來 ────────────────────────────────────────
 * 58A §S10 ④ 假定端點 35 內嵌了該訂單的發票文件摘要（§B-2.4 A4 把它列為「開工時
 * 要實打確認」的四項之一）。實查 gateway `InvoiceComplaintInfo` 的 17 欄與
 * invoice-service 的 `toComplaintInfo`：**沒有任何文件欄位**。依 A4 的指示「以回應
 * 的實際形狀為準」⇒ ④ 不渲染，改在 ③ 給一條「查看這張訂單的發票狀態」的連結
 * （導向 S07，那裡才有文件資料）。已回報衝突，未自行改規格。
 * 「重新寄送通知」（端點 32）連帶不做——它本來就掛在 ④ 內，而且該 RPC 同樣未實作。
 */

import * as React from "react";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { Chip } from "@/components/ui/Chip";
import { SlaCountdown } from "@/features/invoices/components/SlaCountdown";
import { INVOICE_COMPLAINT_ACTION_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiDate, formatTaipeiDateTime } from "@/lib/date/taipei";
import {
	useRespondSellerInvoiceComplaint,
	useSellerInvoiceComplaint,
} from "@/lib/hooks/useSellerInvoiceRemedy";
import type {
	SellerInvoiceComplaint,
	SellerInvoiceComplaintResponse,
} from "@/lib/invoices/seller-types";
import {
	invoiceComplaintDisplay,
	isComplaintClosed,
	isComplaintOverdue,
} from "../invoiceComplaintDisplay";
import { InvoiceComplaintRespondDialog } from "./InvoiceComplaintRespondDialog";

const APPEALS_HREF = "/seller-center/invoices/appeals";

function dash(value: string): string {
	return value || "—";
}

/** `actor` 是英文碼（`SELLER`／`ADMIN`／`BUYER`），CodeMeta 沒有這一組的碼表。 */
const ACTOR_LABEL: Record<string, string> = {
	SELLER: "你",
	ADMIN: "平台",
	BUYER: "買家",
	SYSTEM: "系統",
};

export function SellerInvoiceAppealDetailPage({
	complaintId,
}: {
	complaintId: string;
}) {
	const { complaint, loading, error, refresh } =
		useSellerInvoiceComplaint(complaintId);

	return (
		<SellerPageShell
			title="申訴詳情"
			eyebrow="買家發票申訴"
			// `caseNo` 自 backend#3054 起有值，且**逐字等於** complaintId
			// ——58 §5.1.12 的 DDL 寫著「id bigint … TSID（對外＝案件編號）」，
			// 案件編號的來源設計上就是那一列的 id，本表沒有第二個案號欄。
			// fallback 留著是為了舊回應（部署窗口內），兩者同值 ⇒ 顯示結果不變。
			subtitle={
				complaint ? (
					<CopyableCaseNo value={complaint.caseNo || complaint.complaintId} />
				) : undefined
			}
			actions={
				<BackLink fallbackHref={APPEALS_HREF} className="text-sm underline">
					返回列表
				</BackLink>
			}
		>
			<Link href={APPEALS_HREF} className="text-sm underline">
				返回買家發票申訴
			</Link>

			{loading ? (
				<DetailSkeleton />
			) : !complaint ? (
				// 端點 35 對「越權」與「真的不存在」回同一個 404＋9309（Tier A）⇒
				// 文案必須完全相同，不得寫「沒有權限」。
				<section
					role="alert"
					className="seller-card flex flex-col items-center gap-[var(--hkui-space-3)] p-6 text-center"
				>
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						找不到這件申訴。
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
						<Link href={APPEALS_HREF} className="text-sm underline">
							返回列表
						</Link>
					</div>
				</section>
			) : (
				<ComplaintBody complaint={complaint} />
			)}
		</SellerPageShell>
	);
}

function DetailSkeleton() {
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

function ComplaintBody({ complaint }: { complaint: SellerInvoiceComplaint }) {
	const display = invoiceComplaintDisplay(complaint.status);
	const [respondOpen, setRespondOpen] = React.useState(false);
	// 🔴 expectedVersion 取自 complaint.version，不得用 responses.length 頂替
	// ——平台側也會追加事件，兩個數字會分岔。
	const respond = useRespondSellerInvoiceComplaint(
		complaint.complaintId,
		complaint.version,
	);

	// 送出成功後把 dialog 收掉。回覆是追加語意，整個區塊不收起來。
	React.useEffect(() => {
		if (respond.succeeded) {
			setRespondOpen(false);
			respond.reset();
		}
	}, [respond.succeeded, respond.reset]);
	const closed = isComplaintClosed(complaint.status);
	const overdue = isComplaintOverdue(
		complaint.status,
		complaint.remainingBusinessDays,
	);
	const reopened =
		complaint.reopenCount !== "" && Number(complaint.reopenCount) > 0;
	const orderHref = complaint.orderNo
		? `/seller-center/orders/${encodeURIComponent(complaint.orderNo)}`
		: "";

	return (
		<div className="flex flex-col gap-[var(--hkui-space-4)]">
			{/* ② SLA 橫幅。逾期對賣家仍是 `status` 不是 `alert`——進頁時就存在的狀態
			    不是即時事件。 */}
			<section
				role="status"
				className={[
					"flex flex-wrap items-center justify-between gap-2 rounded-hkui-lg border p-[var(--hkui-space-4)]",
					closed
						? "border-divider bg-content2 text-[var(--hkui-foreground-soft)]"
						: overdue
							? "border-[var(--hkui-danger-200)] bg-danger-50 text-danger-700"
							: "border-[var(--hkui-warning-200)] bg-warning-50 text-warning-700",
				].join(" ")}
			>
				<strong className="text-sm font-bold">
					{closed
						? "這件申訴已結案。"
						: overdue
							? "已逾期，請儘快回覆。"
							: "請在期限內回覆這件申訴。"}
				</strong>
				{closed ? null : (
					<SlaCountdown
						remainingBusinessDays={complaint.remainingBusinessDays}
						overdueBusinessDays={complaint.overdueBusinessDays}
					/>
				)}
			</section>

			{/*
			 * ⑥ 回覆申訴。已結案的申訴不渲染入口。
			 *
			 * 按鈕文案依狀態換：已回覆過的案子再按是**補充說明**（追加語意），
			 * 寫成「回覆申訴」會讓賣家以為前一次沒送出去。
			 */}
			{closed ? null : (
				<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
					<strong className="text-sm font-bold">回覆這件申訴</strong>
					<p className="m-0 text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
						{"處理完成後在這裡回報，買家會收到通知。逾期未回覆會累積違規記錄。"}
					</p>
					<div className="mt-1">
						<Button
							size="lg"
							variant="solid"
							onClick={() => setRespondOpen(true)}
						>
							{complaint.responses.length > 0 ? "補充說明" : "回覆申訴"}
						</Button>
					</div>
				</section>
			)}

			<InvoiceComplaintRespondDialog
				open={respondOpen}
				onClose={() => {
					setRespondOpen(false);
					respond.reset();
				}}
				submitting={respond.responding}
				error={respond.error}
				onSubmit={(args) => respond.respond(args)}
			/>

			{/* ③ 申訴內容。⚠️「買家」與「買家期待的文件」兩列整列不渲染——`buyerMaskedName`
			    與 `expectedDocKind` 後端恆為空字串（見 seller-types.ts 檔頭）。 */}
			<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
				<div className="flex flex-wrap items-center justify-between gap-2">
					<h2 className="m-0 text-sm font-bold">申訴內容</h2>
					<span className="flex flex-wrap items-center gap-2">
						<Chip variant="flat" color={display.color} size="sm">
							{display.label}
						</Chip>
						{/* `reopenCount` 是屬性不是狀態。 */}
						{reopened ? (
							<Chip variant="flat" color="warning" size="sm">
								{`已重開 ${complaint.reopenCount} 次`}
							</Chip>
						) : null}
					</span>
				</div>
				<dl className="seller-order-detail__info">
					<div>
						<dt>訂單編號</dt>
						<dd>
							{orderHref ? (
								<Link href={orderHref} className="underline">
									<code>{complaint.orderNo}</code>
								</Link>
							) : (
								"—"
							)}
						</dd>
					</div>
					<div>
						<dt>申訴時間</dt>
						<dd>{dash(formatTaipeiDateTime(complaint.createdAt))}</dd>
					</div>
					{complaint.buyerMaskedName ? (
						<div>
							<dt>買家</dt>
							<dd>{complaint.buyerMaskedName}</dd>
						</div>
					) : null}
					{complaint.buyerReviewDueAt ? (
						<div>
							<dt>買家確認期限</dt>
							<dd>{dash(formatTaipeiDate(complaint.buyerReviewDueAt))}</dd>
						</div>
					) : null}
				</dl>
				{complaint.description ? (
					<p className="m-0 whitespace-pre-wrap text-sm">
						{complaint.description}
					</p>
				) : null}
				{orderHref ? (
					// ④ 的替代：端點 35 沒有內嵌任何文件摘要（見檔頭），發票狀態只有
					// S07 看得到。給連結而不是渲染一張空卡。
					<Link href={orderHref} className="text-sm underline">
						查看這張訂單的發票狀態
					</Link>
				) : null}
			</section>

			{/* ⑤ 處理紀錄。追加語意 ⇒ 依 occurredAt 遞增；每則帶明確的時間文字，
			    不用相對時間（「3 天前」對稽核沒用）。 */}
			<section className="seller-card flex flex-col gap-2 p-[var(--hkui-space-4)]">
				<h2 className="m-0 text-sm font-bold">處理紀錄</h2>
				{complaint.responses.length === 0 ? (
					<p className="m-0 text-sm text-[var(--hkui-foreground-soft)]">
						尚未有任何處理紀錄。
					</p>
				) : (
					<ol className="m-0 flex list-none flex-col gap-3 p-0">
						{[...complaint.responses]
							.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt))
							.map((item) => (
								<ResponseRow key={item.responseId} item={item} />
							))}
					</ol>
				)}
			</section>
		</div>
	);
}

function ResponseRow({ item }: { item: SellerInvoiceComplaintResponse }) {
	return (
		<li className="flex flex-col gap-1 border-b border-divider pb-3 last:border-b-0 last:pb-0">
			<span className="flex flex-wrap items-baseline gap-2">
				<strong className="text-sm font-bold">
					{ACTOR_LABEL[item.actor] ?? dash(item.actor)}
				</strong>
				<span className="text-sm">
					{/* 中文一律取自 CodeMeta；查不到就印原始碼（賣家截圖給客服時那串碼
					    是唯一線索），不顯示空白。 */}
					{INVOICE_COMPLAINT_ACTION_LABEL[item.action] ?? dash(item.action)}
				</span>
				{item.occurredAt ? (
					<time
						dateTime={item.occurredAt}
						className="text-[0.75rem] text-[var(--hkui-foreground-soft)]"
					>
						{formatTaipeiDateTime(item.occurredAt)}
					</time>
				) : null}
			</span>
			{item.note ? (
				<p className="m-0 whitespace-pre-wrap text-[0.8125rem] text-[var(--hkui-foreground-soft)]">
					{item.note}
				</p>
			) : null}
		</li>
	);
}

function CopyableCaseNo({ value }: { value: string }) {
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

export default SellerInvoiceAppealDetailPage;
