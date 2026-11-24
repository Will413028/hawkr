import { timestampDate } from "@bufbuild/protobuf/wkt";
import type {
	Payment,
	PaymentRefund,
	PaymentTransfer,
} from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentRefundStatus,
	PaymentStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
import {
	paymentPaymentRefundStatus,
	paymentPaymentReversalStatus,
	paymentPaymentStatus,
	paymentPaymentTopupStatus,
	paymentPaymentTransferStatus,
} from "@hawkr/ui-text/keys";
import type { BadgeVariant } from "@/components/UI/StatusBadge";
import { formatDateTime } from "@/lib/format";
import { staffTextOrUnknown } from "@/lib/ui-text";

// 異常監控的付款、退款、轉帳三個分頁（PaymentAdminService.ListPayments、ListPaymentRefunds、
// ListPaymentTransfers）的請求與列顯示。page token 綁定篩選條件，換條件時呼叫端回到第一頁。

export const PAYMENT_LIST_PAGE_SIZE = 20;

export interface PaymentFilters {
	status: PaymentStatus;
	orderNo: string;
}

export interface RefundFilters {
	status: PaymentRefundStatus;
	orderNo: string;
	anomalyOnly: boolean;
}

export interface TransferFilters {
	status: PaymentTransferStatus;
	orderNo: string;
}

// 預設沿用舊頁「只看異常」的語意；狀態選「全部」才看所有。
export const DEFAULT_PAYMENT_FILTERS: PaymentFilters = {
	status: PaymentStatus.ANOMALY,
	orderNo: "",
};
export const DEFAULT_REFUND_FILTERS: RefundFilters = {
	status: PaymentRefundStatus.UNSPECIFIED,
	orderNo: "",
	anomalyOnly: true,
};
export const DEFAULT_TRANSFER_FILTERS: TransferFilters = {
	status: PaymentTransferStatus.ANOMALY,
	orderNo: "",
};

/** UNSPECIFIED 是「全部」：請求不帶 status（optional 欄位不設）。 */
function statusOf<E extends number>(status: E): E | undefined {
	return status === 0 ? undefined : status;
}

export function listPaymentsRequest(
	filters: PaymentFilters,
	pageToken: string,
) {
	return {
		status: statusOf(filters.status),
		orderNo: filters.orderNo,
		pageSize: PAYMENT_LIST_PAGE_SIZE,
		pageToken,
	};
}

export function listPaymentRefundsRequest(
	filters: RefundFilters,
	pageToken: string,
) {
	return {
		status: statusOf(filters.status),
		orderNo: filters.orderNo,
		anomalyOnly: filters.anomalyOnly,
		pageSize: PAYMENT_LIST_PAGE_SIZE,
		pageToken,
	};
}

export function listPaymentTransfersRequest(
	filters: TransferFilters,
	pageToken: string,
) {
	return {
		status: statusOf(filters.status),
		orderNo: filters.orderNo,
		pageSize: PAYMENT_LIST_PAGE_SIZE,
		pageToken,
	};
}

/** 付款狀態下拉的選項順序：ANOMALY、CREATE_FAILED 在最前面，其餘依 enum 順序，不含 UNSPECIFIED。 */
export const PAYMENT_STATUS_OPTIONS: readonly PaymentStatus[] = [
	PaymentStatus.ANOMALY,
	PaymentStatus.CREATE_FAILED,
	PaymentStatus.CREATING,
	PaymentStatus.AWAITING_PAYMENT,
	PaymentStatus.PROCESSING,
	PaymentStatus.SUCCEEDED,
	PaymentStatus.CANCELED,
];

export const REFUND_STATUS_OPTIONS: readonly PaymentRefundStatus[] = [
	PaymentRefundStatus.ANOMALY,
	PaymentRefundStatus.FAILED,
	PaymentRefundStatus.PENDING,
	PaymentRefundStatus.REFUNDING,
	PaymentRefundStatus.SUCCEEDED,
];

export const TRANSFER_STATUS_OPTIONS: readonly PaymentTransferStatus[] = [
	PaymentTransferStatus.ANOMALY,
	PaymentTransferStatus.FAILED,
	PaymentTransferStatus.PENDING,
	PaymentTransferStatus.SUCCEEDED,
];

export function paymentStatusText(status: PaymentStatus): string {
	return staffTextOrUnknown(paymentPaymentStatus(status));
}

export function refundStatusText(status: PaymentRefundStatus): string {
	return staffTextOrUnknown(paymentPaymentRefundStatus(status));
}

export function transferStatusText(status: PaymentTransferStatus): string {
	return staffTextOrUnknown(paymentPaymentTransferStatus(status));
}

export function paymentStatusVariant(status: PaymentStatus): BadgeVariant {
	switch (status) {
		case PaymentStatus.SUCCEEDED:
			return "success";
		case PaymentStatus.ANOMALY:
		case PaymentStatus.CREATE_FAILED:
			return "danger";
		case PaymentStatus.CANCELED:
			return "gray";
		default:
			return "warning";
	}
}

export function refundStatusVariant(status: PaymentRefundStatus): BadgeVariant {
	switch (status) {
		case PaymentRefundStatus.SUCCEEDED:
			return "success";
		case PaymentRefundStatus.ANOMALY:
		case PaymentRefundStatus.FAILED:
			return "danger";
		default:
			return "warning";
	}
}

export function transferStatusVariant(
	status: PaymentTransferStatus,
): BadgeVariant {
	switch (status) {
		case PaymentTransferStatus.SUCCEEDED:
			return "success";
		case PaymentTransferStatus.ANOMALY:
		case PaymentTransferStatus.FAILED:
			return "danger";
		default:
			return "warning";
	}
}

/** 金額是 int64（bigint），顯示千分位。 */
export function amountText(amount: bigint): string {
	return `NT$ ${amount.toLocaleString("en-US")}`;
}

function timeText(ts: Payment["createTime"]): string {
	return ts ? formatDateTime(timestampDate(ts)) : "—";
}

function orDash(value: string): string {
	return value === "" ? "—" : value;
}

export interface PaymentRow {
	id: string;
	orderNo: string;
	externalPaymentId: string;
	buyerId: string;
	storeId: string;
	amount: string;
	provider: string;
	status: string;
	statusVariant: BadgeVariant;
	/** last_error 與 failure_reason，有才顯示。 */
	lastError: string;
	failureReason: string;
	createTime: string;
}

export function paymentRow(p: Payment): PaymentRow {
	return {
		id: p.id,
		orderNo: orDash(p.orderNo),
		externalPaymentId: p.externalPaymentId,
		buyerId: orDash(p.buyerId),
		storeId: orDash(p.storeId),
		amount: amountText(p.amount),
		provider: orDash(p.provider),
		status: paymentStatusText(p.status),
		statusVariant: paymentStatusVariant(p.status),
		lastError: p.lastError,
		failureReason: p.failureReason,
		createTime: timeText(p.createTime),
	};
}

export interface RefundRow {
	id: string;
	orderNo: string;
	amount: string;
	auto: string;
	status: string;
	statusVariant: BadgeVariant;
	reversalStatus: string;
	topupStatus: string;
	externalRefundId: string;
	failureReason: string;
	createTime: string;
}

export function refundRow(r: PaymentRefund): RefundRow {
	return {
		id: r.id,
		orderNo: orDash(r.orderNo),
		amount: amountText(r.amount),
		auto: r.auto ? "自動" : "人工",
		status: refundStatusText(r.status),
		statusVariant: refundStatusVariant(r.status),
		reversalStatus: staffTextOrUnknown(
			paymentPaymentReversalStatus(r.reversalStatus),
		),
		topupStatus: staffTextOrUnknown(paymentPaymentTopupStatus(r.topupStatus)),
		externalRefundId: r.externalRefundId,
		failureReason: r.failureReason,
		createTime: timeText(r.createTime),
	};
}

export interface TransferRow {
	id: string;
	orderNo: string;
	storeId: string;
	amount: string;
	reversedAmount: string;
	status: string;
	statusVariant: BadgeVariant;
	externalTransferId: string;
	failureReason: string;
	createTime: string;
}

export function transferRow(t: PaymentTransfer): TransferRow {
	return {
		id: t.id,
		orderNo: orDash(t.orderNo),
		storeId: orDash(t.storeId),
		amount: amountText(t.amount),
		reversedAmount: amountText(t.reversedAmount),
		status: transferStatusText(t.status),
		statusVariant: transferStatusVariant(t.status),
		externalTransferId: t.externalTransferId,
		failureReason: t.failureReason,
		createTime: timeText(t.createTime),
	};
}
