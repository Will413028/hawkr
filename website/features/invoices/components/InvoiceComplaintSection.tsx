"use client";

import * as React from "react";
import { Chip } from "@/components/ui";
import Button from "@/components/ui/Button";
import { SlaCountdown } from "@/features/invoices/components/SlaCountdown";
import {
	INVOICE_COMPLAINT_ERROR_TEXT,
	INVOICE_COMPLAINT_TEXT,
} from "@/features/invoices/i18n";
import {
	type ComplaintActionKind,
	InvoiceComplaintActionDialog,
} from "@/features/invoices/screens/InvoiceComplaintActionDialog";
import {
	INVOICE_COMPLAINT_ACTION_LABEL,
	INVOICE_COMPLAINT_STATUS_LABEL,
} from "@/lib/codemeta/generated";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useInvoiceComplaint } from "@/lib/hooks/useInvoiceComplaint";
import {
	INVOICE_BIZ_CODE,
	invoiceBizCode,
	invoiceErrorMessage,
	newIdempotencyKey,
	rejectInvoiceComplaintResponse,
	reopenInvoiceComplaint,
	withdrawInvoiceComplaint,
} from "@/lib/invoices/buyer-repo";
import {
	COMPLAINT_REOPEN_LIMIT,
	complaintActionErrorKind,
	complaintActions,
	foldInvoiceComplaintStatus,
	type InvoiceComplaintTone,
} from "@/lib/invoices/complaintStatus";
import type { InvoiceComplaint } from "@/lib/invoices/types";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import {
	COMPLAINT_BANNER_LAYOUT,
	InvoiceComplaintLoadingBody,
} from "./InvoiceSectionLoading";

const T = INVOICE_COMPLAINT_TEXT;
const E = INVOICE_COMPLAINT_ERROR_TEXT;

const TONE_CLASS: Record<InvoiceComplaintTone, string> = {
	info: "border-info-200 bg-info-50 text-info-700",
	warning: "border-warning-200 bg-warning-50 text-warning-700",
	success: "border-success-200 bg-success-50 text-success-700",
	neutral: "border-divider bg-content2 text-foreground-soft",
};

/** 歷程上的「誰」。買家自己的動作不進這一軸，所以只有兩個角色。 */
function actorLabel(actor: string): string {
	if (actor === "SELLER") return T.actorSeller;
	if (actor === "ADMIN") return T.actorAdmin;
	// 後端加了第三種角色時退回原字串，至少看得出是誰做的（§0.6 C11）。
	return actor;
}

/**
 * 動作中文一律走 CodeMeta `InvoiceComplaintAction`（6 值），**不得手抄那六句**。
 *
 * ⚠️ `action` 可以是空字串——系統事件與買家自己的動作不寫這一欄；那時退回
 * `note`／狀態名都不對，直接不渲染標題行的動作段。
 */
function actionLabel(action: string): string {
	if (!action) return "";
	return INVOICE_COMPLAINT_ACTION_LABEL[action] ?? action;
}

/** Chip 用的狀態名走 CodeMeta；橫幅的兩句話走折疊表（兩者職責不同）。 */
function statusChipLabel(status: string): string {
	return INVOICE_COMPLAINT_STATUS_LABEL[status] ?? status;
}

/**
 * 碼 → 哪一種話。**分流表在 `complaintActionErrorKind` 一處並有測試守著**——
 * 9322 與 9336 都是 409 而處置相反（前者禁用後端 message、後者必須用），看 HTTP
 * status 永遠分不出來。
 */
function actionErrorText(err: unknown): string {
	switch (complaintActionErrorKind(invoiceBizCode(err))) {
		case "notFound":
			return E.notFound;
		case "versionConflict":
			return E.versionConflict;
		case "backendMessage":
			return invoiceErrorMessage(err) || E.generic;
		case "idempotencyMismatch":
			return E.idempotencyMismatch;
		default:
			return E.generic;
	}
}

interface InvoiceComplaintSectionProps {
	orderNo: string;
	complaintId: string;
}

/**
 * B07｜案件狀態與歷程（58A §B07），內嵌於 B04 的發票卡底部。
 *
 * 為什麼不開獨立路由：既有先例就是內嵌（訂單詳情的售後摘要）；9310 要「導向既有
 * 案件」，同一張卡上就是零導航；而本畫面的資訊量（狀態＋倒數＋3–5 筆事件）一張
 * 卡裝得下。
 *
 * 🔴 買家能做的動作**只有四個**：提出（13）、不接受（63）、撤回（64）、重新提出
 * （65）。「主動確認已解決」Phase 1 沒有端點（`SELLER_RESPONDED` 沉默滿 72 小時
 * 即自動結案）⇒ **不要自己加一顆「確認已解決」按鈕**，它會打到一支不存在的端點。
 */
export const InvoiceComplaintSection = React.forwardRef<
	HTMLElement,
	InvoiceComplaintSectionProps
>(function InvoiceComplaintSection({ orderNo, complaintId }, ref) {
	const { complaint, loading, notFound, error, refresh, replace } =
		useInvoiceComplaint(orderNo, complaintId);
	const toast = useNotificationToast();

	const [pending, setPending] = React.useState<ComplaintActionKind | null>(
		null,
	);
	const [submitting, setSubmitting] = React.useState(false);
	const [actionError, setActionError] = React.useState("");
	const [copied, setCopied] = React.useState(false);
	// 一次「使用者意圖」一把（§0.4 ②）：開啟確認 Dialog 時 mint，重試沿用同一把，
	// 只有 9350（payload mismatch）才換新的。
	const idemKeyRef = React.useRef("");

	const openAction = (kind: ComplaintActionKind) => {
		idemKeyRef.current = newIdempotencyKey();
		setActionError("");
		setPending(kind);
	};

	const closeAction = () => {
		if (submitting) return;
		setPending(null);
		setActionError("");
	};

	if (notFound) return null;

	if (loading && !complaint) {
		return (
			<section
				ref={ref}
				className="mt-4 border-t border-divider pt-4"
				aria-busy="true"
				role="status"
			>
				<h3 className="m-0 mb-3 text-sm font-semibold">{T.sectionTitle}</h3>
				<InvoiceComplaintLoadingBody />
			</section>
		);
	}

	if (!complaint) {
		// 端點 15 失敗時只有本區塊變成錯誤態——發票卡其餘部分不受影響。
		if (!error) return null;
		return (
			<section ref={ref} className="mt-4 border-t border-divider pt-4">
				<h3 className="m-0 mb-2 text-sm font-semibold">{T.sectionTitle}</h3>
				<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
					{T.loadError}
				</p>
				<button
					type="button"
					className="mt-2 inline-flex min-h-[44px] items-center bg-transparent p-0 text-sm font-semibold text-hk-primary underline underline-offset-2"
					onClick={() => void refresh()}
				>
					{T.retry}
				</button>
			</section>
		);
	}

	const banner = foldInvoiceComplaintStatus(complaint.status);
	const parsedReopen = Number(complaint.reopenCount);
	const reopenCount = Number.isFinite(parsedReopen) ? parsedReopen : 0;
	const actions = complaintActions(
		complaint.status,
		complaint.reopenCount,
		complaint.reopenAllowedAfter,
		Date.now(),
	);
	// 後端上線了一個前端還不知道的狀態時一顆動作鍵都不渲染：對一個我們讀不懂的
	// 狀態送出動作，比暫時少一顆按鈕危險得多。
	const showActions = !banner.unknown;

	const submit = async (reason: string) => {
		if (!pending) return;
		setSubmitting(true);
		setActionError("");
		try {
			let next: InvoiceComplaint;
			if (pending === "reject") {
				next = await rejectInvoiceComplaintResponse(
					orderNo,
					complaint.complaintId,
					reason,
					complaint.version,
					idemKeyRef.current,
				);
			} else if (pending === "withdraw") {
				next = await withdrawInvoiceComplaint(
					orderNo,
					complaint.complaintId,
					complaint.version,
					idemKeyRef.current,
				);
			} else {
				next = await reopenInvoiceComplaint(
					orderNo,
					complaint.complaintId,
					reason,
					complaint.version,
					idemKeyRef.current,
				);
			}
			// 三支都回轉移後的完整單筆 ⇒ 直接覆蓋快取，不再打一次端點 15。
			replace(next);
			toast.show({
				variant: "success",
				title:
					pending === "reject"
						? T.rejectToast
						: pending === "withdraw"
							? T.withdrawToast
							: T.reopenToast,
				dedupeKey: `invoice-complaint-${pending}:${complaint.complaintId}`,
			});
			setPending(null);
		} catch (err) {
			const code = invoiceBizCode(err);
			if (code === INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH) {
				// 只有這個碼才換新鍵——其餘一律沿用，否則後端會把重試當成第二次操作。
				console.error("invoice complaint idempotency mismatch", err);
				idemKeyRef.current = newIdempotencyKey();
			}
			if (
				code === INVOICE_BIZ_CODE.COMPLAINT_ACTION_NOT_ALLOWED ||
				code === INVOICE_BIZ_CODE.VERSION_CONFLICT
			) {
				// 兩者都代表「我手上的版本已經過時」⇒ 關掉確認框並重抓，讓使用者看到
				// 現況再決定。9336 的那句話直接來自後端。
				setPending(null);
				await refresh();
				toast.show({
					variant: "warning",
					title:
						code === INVOICE_BIZ_CODE.VERSION_CONFLICT
							? E.versionConflict
							: actionErrorText(err),
					dedupeKey: `invoice-complaint-stale:${complaint.complaintId}`,
				});
			} else {
				setActionError(actionErrorText(err));
			}
		} finally {
			setSubmitting(false);
		}
	};

	const reopenHint =
		!actions.showReopen || actions.canReopen
			? ""
			: actions.reopenExhausted
				? T.reopenExhausted
				: complaint.reopenAllowedAfter
					? T.reopenCooldown(
							formatTaipeiDateTime(complaint.reopenAllowedAfter) ||
								complaint.reopenAllowedAfter,
						)
					: "";

	const reviewDue = complaint.buyerReviewDueAt
		? formatTaipeiDateTime(complaint.buyerReviewDueAt) ||
			complaint.buyerReviewDueAt
		: "";

	return (
		<section
			ref={ref}
			tabIndex={-1}
			className="mt-4 border-t border-divider pt-4 outline-none"
			aria-label={T.sectionTitle}
		>
			<h3 className="m-0 mb-3 text-sm font-semibold">{T.sectionTitle}</h3>

			<div
				className={`${COMPLAINT_BANNER_LAYOUT} ${TONE_CLASS[banner.tone]}`}
				role="status"
			>
				<div className="flex flex-wrap items-center gap-2">
					{complaint.caseNo ? (
						<span className="inline-flex items-center gap-2 text-xs tabular-nums">
							{T.caseNoLabel(complaint.caseNo)}
							<button
								type="button"
								className="shrink-0 rounded-full border border-current bg-transparent px-2 py-1 text-[0.6875rem] font-bold"
								aria-label={T.copyCaseNo}
								onClick={() => {
									void navigator.clipboard
										.writeText(complaint.caseNo)
										.then(() => {
											setCopied(true);
											window.setTimeout(() => setCopied(false), 1500);
										})
										// 剪貼簿權限被拒時靜默——複製失敗不是使用者要處理的事，
										// 案件編號本來就在畫面上讀得到。
										.catch(() => {});
								}}
							>
								{copied ? T.copied : "複製"}
							</button>
							{copied ? (
								<span role="status" className="sr-only">
									{T.copied}
								</span>
							) : null}
						</span>
					) : null}
					<Chip variant="flat" color="default" size="sm">
						{statusChipLabel(complaint.status)}
					</Chip>
					{/* 「已重啟」不是狀態、是屬性：重開把案件推回 OPEN 並把 reopen_count
					    ＋1，買家在等的事情與第一次完全相同。 */}
					{reopenCount > 0 ? (
						<Chip variant="flat" color="warning" size="sm">
							{T.reopenedChip}
						</Chip>
					) : null}
					{banner.showCountdown ? (
						<SlaCountdown
							remainingBusinessDays={complaint.remainingBusinessDays}
							overdueBusinessDays={complaint.overdueBusinessDays}
						/>
					) : null}
				</div>

				<strong className="mt-2 block text-base">{banner.title}</strong>
				<p className="m-0 mt-1 leading-relaxed">{banner.body}</p>

				{/* 這一句是「這樣沒有解決」那顆鍵的說明——沒有它，買家看得到按鈕卻不
				    知道有期限，過期按下去只會拿到 9336。空字串時整句不渲染。 */}
				{actions.canReject && reviewDue ? (
					<p className="m-0 mt-1 text-xs">{T.buyerReviewDue(reviewDue)}</p>
				) : null}
			</div>

			{complaint.responses.length > 0 || complaint.createdAt ? (
				<>
					<h4 className="m-0 mb-2 mt-4 text-xs font-semibold text-hk-muted">
						{T.historyTitle}
					</h4>
					<ol className="m-0 flex list-none flex-col gap-3 p-0">
						{complaint.createdAt ? (
							<li className="grid grid-cols-[16px_minmax(0,1fr)] gap-3">
								<span
									className="mt-1.5 size-2 rounded-full bg-hk-primary"
									aria-hidden="true"
								/>
								<div className="flex flex-col gap-0.5">
									<strong className="text-sm font-bold text-foreground">
										{T.createdAction}
									</strong>
									<time
										className="text-xs tabular-nums text-foreground-soft"
										dateTime={complaint.createdAt}
									>
										{formatTaipeiDateTime(complaint.createdAt) ||
											complaint.createdAt}
									</time>
									{complaint.description ? (
										<p className="m-0 whitespace-pre-wrap text-xs text-foreground-soft">
											{complaint.description}
										</p>
									) : null}
								</div>
							</li>
						) : null}
						{complaint.responses.map((item, index) => (
							<li
								key={item.responseId || `${item.occurredAt}-${index}`}
								className="grid grid-cols-[16px_minmax(0,1fr)] gap-3"
							>
								<span
									className="mt-1.5 size-2 rounded-full bg-hk-primary"
									aria-hidden="true"
								/>
								<div className="flex flex-col gap-0.5">
									<strong className="text-sm font-bold text-foreground">
										{actorLabel(item.actor)}
										{actionLabel(item.action)
											? ` · ${actionLabel(item.action)}`
											: ""}
									</strong>
									{item.occurredAt ? (
										<time
											className="text-xs tabular-nums text-foreground-soft"
											dateTime={item.occurredAt}
										>
											{formatTaipeiDateTime(item.occurredAt) || item.occurredAt}
										</time>
									) : null}
									{item.note ? (
										<p className="m-0 whitespace-pre-wrap text-xs text-foreground-soft">
											{item.note}
										</p>
									) : null}
								</div>
							</li>
						))}
					</ol>
				</>
			) : null}

			{showActions &&
			(actions.canReject || actions.canWithdraw || actions.showReopen) ? (
				<div className="mt-4 flex flex-wrap items-center gap-2">
					{actions.canReject ? (
						<Button
							variant="bordered"
							color="warning"
							size="lg"
							onClick={() => openAction("reject")}
						>
							{T.rejectCta}
						</Button>
					) : null}
					{actions.canWithdraw ? (
						<Button
							variant="light"
							color="default"
							size="lg"
							onClick={() => openAction("withdraw")}
						>
							{T.withdrawCta}
						</Button>
					) : null}
					{actions.showReopen ? (
						<Button
							variant="bordered"
							color="warning"
							size="lg"
							// 冷卻中／已達上限一律 disabled 但**不隱藏**：案件已在畫面上，
							// 突然沒有出口比看得到、按不動更難理解。
							disabled={!actions.canReopen}
							onClick={() => openAction("reopen")}
						>
							{T.reopenCta}
						</Button>
					) : null}
				</div>
			) : null}

			{/* 兩種 disabled 各有各的說明句。第三種情況（後端沒給冷卻結束時間）
			    自 #2911 起不會走到這裡——`showReopen` 本身就吃 `reopenAllowedAfter`，
			    沒有重開邊時整顆按鈕不渲染。留著這個空字串分支當防線，不亂猜一句話。 */}
			{showActions && reopenHint ? (
				<p className="m-0 mt-2 text-xs text-hk-muted">{reopenHint}</p>
			) : null}

			<InvoiceComplaintActionDialog
				kind={pending}
				reopenRemaining={Math.max(COMPLAINT_REOPEN_LIMIT - reopenCount, 0)}
				submitting={submitting}
				errorMessage={actionError}
				onClose={closeAction}
				onConfirm={(reason) => void submit(reason)}
			/>
		</section>
	);
});

export default InvoiceComplaintSection;
