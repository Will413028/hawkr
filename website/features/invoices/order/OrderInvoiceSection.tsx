"use client";

import * as React from "react";
import Button from "@/components/ui/Button";
import { InvoiceComplaintSection } from "@/features/invoices/components/InvoiceComplaintSection";
import { InvoiceDocumentCard } from "@/features/invoices/components/InvoiceDocumentCard";
import { InvoiceModeBadge } from "@/features/invoices/components/InvoiceModeBadge";
import { InvoiceStateBanner } from "@/features/invoices/components/InvoiceStateBanner";
import {
	INVOICE_COMPLAINT_ERROR_TEXT,
	INVOICE_COMPLAINT_TEXT,
	INVOICE_ORDER_TEXT,
} from "@/features/invoices/i18n";
import { InvoiceComplaintDialog } from "@/features/invoices/screens/InvoiceComplaintDialog";
import {
	INVOICE_DOCUMENT_KIND_LABEL,
	InvoiceDocumentStatus,
} from "@/lib/codemeta/generated";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { useOrderInvoice } from "@/lib/hooks/useOrderInvoice";
import {
	complaintIdFromError,
	createInvoiceComplaint,
	createInvoiceCorrection,
	fetchInvoiceCorrection,
	INVOICE_BIZ_CODE,
	INVOICE_COMPLAINT_REASON_NOT_RECEIVED,
	invoiceBizCode,
	invoiceErrorMessage,
	listOrderInvoiceComplaints,
	newIdempotencyKey,
} from "@/lib/invoices/buyer-repo";
import { correctionEntry } from "@/lib/invoices/correctionEntry";
import { correctionResultNotice } from "@/lib/invoices/correctionRejectNotice";
import { orderInvoiceEmptyHint } from "@/lib/invoices/orderInvoiceState";
import type {
	InvoiceCorrection,
	InvoiceCorrectionInput,
	InvoiceDocumentSummary,
} from "@/lib/invoices/types";
import { useNotificationToast } from "@/lib/notifications/toast-context";
import {
	InvoiceLoadingLines,
	ORDER_INVOICE_HEADER_CLASS,
} from "../components/InvoiceSectionLoading";
import { InvoiceCorrectionDialog } from "./InvoiceCorrectionDialog";

const T = INVOICE_ORDER_TEXT;
const C = INVOICE_COMPLAINT_TEXT;

const SECTION_CLASS =
	"bg-content1 border border-divider rounded-hkui-lg p-5 mb-0";

/**
 * Phase 1 的 `reasonCode` 只有一個值 ⇒ Dialog 渲染唯讀說明列而不是只有一個選項的
 * `<Select>`。這個陣列長度就是那個開關，Phase 2 增加原因只要多一筆。
 */
const COMPLAINT_REASONS = [
	{
		value: INVOICE_COMPLAINT_REASON_NOT_RECEIVED,
		label: INVOICE_COMPLAINT_TEXT.reasonNotReceived,
	},
];

/**
 * 當前有效的那一份 ＝ **最後一筆 `ISSUED`**。
 *
 * 順序由後端保證（依 `issuedAt` 遞增），前端不再排序。找不到就回 `null`：
 * 下載鍵與通知信箱列都掛在它身上，寧可不渲染也不要掛到一張已作廢的文件上。
 */
function currentDocument(
	documents: InvoiceDocumentSummary[],
): InvoiceDocumentSummary | null {
	for (let i = documents.length - 1; i >= 0; i -= 1) {
		const candidate = documents[i];
		if (candidate && candidate.status === InvoiceDocumentStatus.ISSUED) {
			return candidate;
		}
	}
	return null;
}

interface OrderInvoiceSectionProps {
	orderNo: string;
	/** 申訴 Dialog 的唯讀「賣家」列；取不到時那一列不渲染。 */
	sellerName?: string;
}

/**
 * B04｜訂單詳情—發票與收據區（58A §B04）＋ B07 的申訴入口與案件區塊（§B07）。
 *
 * 四件事值得先讀：
 *
 *   1. **404 ＋ 9301 時整張卡不渲染**，且不顯示任何「無權限」字樣。端點 8 是
 *      Tier A：越權與不存在回同一個碼，前端分得出來就等於把那個旗標補回去。
 *      零元訂單不開票，走的也是這一條——顯示一張「沒有發票」的卡只是噪音。
 *   2. **`documents` 全部渲染**。作廢重開之後常態是「舊的已作廢 ＋ 新的已開立
 *      ＋ 一張折讓」三筆並存，取最後一筆會讓買家看到已經不算數的那張。
 *   3. **零輪詢**（§0.4 ④）。刷新來源在 `useOrderInvoice` 的檔頭。
 *   4. **申訴入口有四種呈現**（依 `canFileComplaint`／`existingComplaintId`），
 *      其中一種是「什麼都不渲染」——已開立且買家已收到，本來就沒有申訴餘地。
 *      入口只長在這張卡上，**前端不提供任何繞過入口的方式**（例如深連結直接開
 *      Dialog）：24 小時時間閘（9311）就是防「申訴變催單管道」。
 *
 *   5. **「修改發票資料」（B06）已於 2026-09-01 接上**：更正執行面 R02–R04
 *      （`Correction.startVoidReissue`／`Voiding.SubmitVoid`／`ConfirmVoid`）已在
 *      `origin/dev`，backend#2909 已 CLOSED。
 *      🔴 入口**只看後端的 `canRequestCorrection`**，不自己算——它涵蓋開票狀態、
 *      法定作廢窗口、是否已有進行中的申請三件事，前端複製一份必然漂移。
 *      `handlingMode`（開票前直接改／開票後作廢重開）同樣由後端決定，前端不得
 *      依「有沒有發票號碼」推導；送出後把 `statusNotice` 原樣顯示。
 */
export function OrderInvoiceSection({
	orderNo,
	sellerName = "",
}: OrderInvoiceSectionProps) {
	const { invoice, loading, notFound, error, refresh } =
		useOrderInvoice(orderNo);
	const toast = useNotificationToast();

	const [dialogOpen, setDialogOpen] = React.useState(false);

	const correction = correctionEntry(invoice);

	const [correctionOpen, setCorrectionOpen] = React.useState(false);
	const [correctionSubmitting, setCorrectionSubmitting] = React.useState(false);
	const [correctionError, setCorrectionError] = React.useState("");
	const [correctionNotice, setCorrectionNotice] = React.useState("");
	const correctionIdem = React.useRef("");

	/**
	 * 最近一次修改申請的進度（端點 12）。
	 *
	 * 🔴 少了這一段，B06 送出之後就進黑箱：`correctionNotice` 只活在 Dialog 的
	 * state 裡，**重新載入頁面就沒了**，而婉拒理由（`rejectReasonCode`）決定買家
	 * 該改走折讓還是申訴。2026-09-05 第 6 輪稽核：`fetchInvoiceCorrection` 當時
	 * 零呼叫端，因為端點 8 沒有給出任何 correction_id。
	 *
	 * 失敗時不顯示、也不擋整個區塊：這是補充資訊，發票本體照舊要看得到。
	 */
	const latestCorrectionId = invoice?.latestCorrectionId ?? "";
	const [latestCorrection, setLatestCorrection] =
		React.useState<InvoiceCorrection | null>(null);
	React.useEffect(() => {
		if (!latestCorrectionId) {
			setLatestCorrection(null);
			return;
		}
		let alive = true;
		void fetchInvoiceCorrection(orderNo, latestCorrectionId)
			.then((c) => {
				if (alive) setLatestCorrection(c);
			})
			.catch(() => {
				if (alive) setLatestCorrection(null);
			});
		return () => {
			alive = false;
		};
	}, [orderNo, latestCorrectionId]);

	const submitCorrection = async (
		input: InvoiceCorrectionInput,
		reason: string,
	) => {
		if (!invoice) return;
		setCorrectionSubmitting(true);
		setCorrectionError("");
		try {
			const result = await createInvoiceCorrection(
				orderNo,
				input,
				correction.expectedSnapshotVersion,
				reason,
				correctionIdem.current,
			);
			setCorrectionNotice(correctionResultNotice(result));
			await refresh();
		} catch (e) {
			setCorrectionError(invoiceErrorMessage(e));
		} finally {
			setCorrectionSubmitting(false);
		}
	};
	const [submitting, setSubmitting] = React.useState(false);
	const [dialogError, setDialogError] = React.useState("");
	/**
	 * 9310 的補救路徑找回來的案件 id。正常情況下 `existingComplaintId` 會在下一次
	 * 查詢就補上，但那要等一個 round trip；先放這裡，案件區塊當場就出得來。
	 */
	const [foundComplaintId, setFoundComplaintId] = React.useState("");
	const idemKeyRef = React.useRef("");
	const complaintRef = React.useRef<HTMLElement | null>(null);
	const [focusComplaint, setFocusComplaint] = React.useState(false);

	React.useEffect(() => {
		if (!focusComplaint) return;
		const node = complaintRef.current;
		if (node) {
			node.scrollIntoView({ block: "center", behavior: "smooth" });
			node.focus({ preventScroll: true });
		}
		setFocusComplaint(false);
	}, [focusComplaint]);

	if (notFound) return null;

	if (loading && !invoice) {
		return (
			<section className={SECTION_CLASS} role="status" aria-busy="true">
				<div className={ORDER_INVOICE_HEADER_CLASS}>
					<h2 className="m-0 text-base font-semibold">{T.cardTitle}</h2>
					<span
						aria-hidden="true"
						className="h-6 w-20 animate-pulse rounded-full bg-content2 motion-reduce:animate-none"
					/>
				</div>
				<InvoiceLoadingLines />
			</section>
		);
	}

	if (!invoice) {
		if (!error) return null;
		return (
			<section className={SECTION_CLASS}>
				<h2 className="text-base font-semibold mb-3">{T.cardTitle}</h2>
				<p className="m-0 text-sm font-semibold text-danger-700" role="alert">
					{error}
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

	const current = currentDocument(invoice.documents);
	const complaintId = invoice.existingComplaintId || foundComplaintId;
	const showEntry = invoice.canFileComplaint && !complaintId;
	const showOpensAt =
		!invoice.canFileComplaint &&
		!complaintId &&
		invoice.complaintOpensAt !== "";

	const openDialog = () => {
		idemKeyRef.current = newIdempotencyKey();
		setDialogError("");
		setDialogOpen(true);
	};

	const submitComplaint = async (reasonCode: string, description: string) => {
		setSubmitting(true);
		setDialogError("");
		try {
			const created = await createInvoiceComplaint(
				orderNo,
				{ reasonCode, description },
				idemKeyRef.current,
			);
			setFoundComplaintId(created.complaintId);
			setDialogOpen(false);
			await refresh();
			toast.show({
				variant: "success",
				title: C.createdToastTitle,
				body: C.createdToastBody(created.caseNo),
				dedupeKey: `invoice-complaint-created:${orderNo}`,
			});
			setFocusComplaint(true);
		} catch (err) {
			const code = invoiceBizCode(err);
			if (code === INVOICE_BIZ_CODE.COMPLAINT_DUPLICATE) {
				let existing = complaintIdFromError(err);
				if (!existing) {
					try {
						const records = await listOrderInvoiceComplaints(orderNo);
						existing = records[0]?.complaintId ?? "";
					} catch {
					}
				}
				if (existing) setFoundComplaintId(existing);
				setDialogOpen(false);
				await refresh();
				setFocusComplaint(true);
				return;
			}
			if (code === INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH) {
				console.error("invoice complaint idempotency mismatch", err);
				idemKeyRef.current = newIdempotencyKey();
				setDialogError(INVOICE_COMPLAINT_ERROR_TEXT.idempotencyMismatch);
				return;
			}
			if (code === INVOICE_BIZ_CODE.COMPLAINT_NOT_OPEN_YET) {
				setDialogError(INVOICE_COMPLAINT_ERROR_TEXT.notOpenYet);
				await refresh();
				return;
			}
			if (code === INVOICE_BIZ_CODE.DOCUMENT_NOT_FOUND) {
				setDialogError(INVOICE_COMPLAINT_ERROR_TEXT.notFound);
				return;
			}
			setDialogError(INVOICE_COMPLAINT_ERROR_TEXT.generic);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<section className={SECTION_CLASS}>
			<div className={ORDER_INVOICE_HEADER_CLASS}>
				<h2 className="m-0 text-base font-semibold">{T.cardTitle}</h2>
				<InvoiceModeBadge mode={invoice.invoiceMode} />
			</div>

			<InvoiceStateBanner
				state={invoice.state}
				notice={invoice.stateNotice}
				onRefresh={() => void refresh()}
			/>

			{invoice.documents.length > 0 ? (
				<ul className="mt-3 flex list-none flex-col gap-3 p-0">
					{invoice.documents.map((document) => (
						<InvoiceDocumentCard
							key={document.documentId}
							orderNo={invoice.orderNo}
							document={document}
							siblings={invoice.documents}
							isCurrent={current?.documentId === document.documentId}
							notifiedEmailMasked={invoice.notifiedEmailMasked}
						/>
					))}
				</ul>
			) : (
				<p className="m-0 mt-3 text-sm text-hk-muted">
					{orderInvoiceEmptyHint(invoice.state)}
				</p>
			)}


			{invoice.documents.length > 0 ? (
				<p
					className="m-0 mt-4 rounded-hkui-md border border-info-200 bg-info-50 p-3 text-xs leading-relaxed text-info-700"
					role="status"
				>
					{invoice.shippingFeeNotice || T.shippingFeeNotice}
				</p>
			) : null}


			{latestCorrection ? (
				<p
					className="m-0 mt-4 rounded-hkui-md border border-info-200 bg-info-50 p-3 text-xs leading-relaxed text-info-700"
					data-testid="invoice-correction-status"
					role="status"
				>
					{correctionResultNotice(latestCorrection)}
				</p>
			) : null}

			{showEntry || correction.visible ? (
				<div className="mt-4 flex flex-wrap justify-end gap-2">

					{correction.visible ? (
						<Button
							variant="light"
							size="sm"
							onClick={() => {
								correctionIdem.current = newIdempotencyKey();
								setCorrectionNotice("");
								setCorrectionError("");
								setCorrectionOpen(true);
							}}
						>
							修改發票資料
						</Button>
					) : null}
					{showEntry ? (
						<Button
							variant="light"
							color="warning"
							size="sm"
							onClick={openDialog}
						>
							{C.entryCta}
						</Button>
					) : null}
				</div>
			) : null}

			<InvoiceCorrectionDialog
				open={correctionOpen}
				onClose={() => setCorrectionOpen(false)}
				submitting={correctionSubmitting}
				errorMessage={correctionError}
				resultNotice={correctionNotice}
				onSubmit={(input, reason) => {
					void submitCorrection(input, reason);
				}}
			/>

			{showOpensAt ? (
				<p className="m-0 mt-4 text-xs text-hk-muted">
					{C.notOpenYet(
						formatTaipeiDateTime(invoice.complaintOpensAt) ||
							invoice.complaintOpensAt,
					)}
				</p>
			) : null}

			{complaintId ? (
				<InvoiceComplaintSection
					ref={complaintRef}
					orderNo={invoice.orderNo}
					complaintId={complaintId}
				/>
			) : null}

			<InvoiceComplaintDialog
				open={dialogOpen}
				orderNo={invoice.orderNo}
				sellerName={sellerName}
				expectedDocLabel={
					current
						? (INVOICE_DOCUMENT_KIND_LABEL[current.documentKind] ?? "")
						: (INVOICE_DOCUMENT_KIND_LABEL[
								invoice.documents[0]?.documentKind ?? ""
							] ?? "")
				}
				reasonOptions={COMPLAINT_REASONS}
				submitting={submitting}
				errorMessage={dialogError}
				onClose={() => {
					if (submitting) return;
					setDialogOpen(false);
				}}
				onSubmit={(reasonCode, description) =>
					void submitComplaint(reasonCode, description)
				}
			/>
		</section>
	);
}

export default OrderInvoiceSection;
