/**
 * Wire → domain normalization for after-sale shapes. The backend mixes
 * camelCase and snake_case across the buyer/seller views, so we read both and
 * coerce to strings (matching the OpenAPI "amounts/ids are strings" rule).
 */

import { AfterSaleEvidenceType } from "@/lib/codemeta/enums";
import type {
	AfterSaleArbitration,
	AfterSaleCase,
	AfterSaleDetail,
	AfterSaleEvidence,
	AfterSaleItem,
	AfterSaleRefundProgress,
	AfterSaleResolution,
	AfterSaleReturnLogistics,
} from "./types";

/** Loosely-typed wire object — every field optional, snake or camel. */
type Wire = Record<string, unknown>;

function str(...vals: unknown[]): string {
	for (const v of vals) {
		if (v !== undefined && v !== null && v !== "") return String(v);
	}
	return "";
}

function optStr(...vals: unknown[]): string | undefined {
	for (const v of vals) {
		if (v !== undefined && v !== null && v !== "") return String(v);
	}
	return undefined;
}

function num(...vals: unknown[]): number {
	for (const v of vals) {
		if (v !== undefined && v !== null && v !== "") {
			const n = Number(v);
			if (!Number.isNaN(n)) return n;
		}
	}
	return 0;
}

function bool(...vals: unknown[]): boolean | undefined {
	for (const v of vals) {
		if (v === undefined || v === null) continue;
		if (typeof v === "boolean") return v;
		if (v === "true" || v === "1" || v === 1) return true;
		if (v === "false" || v === "0" || v === 0) return false;
	}
	return undefined;
}

function asArray(v: unknown): Wire[] {
	return Array.isArray(v) ? (v as Wire[]) : [];
}

export function normalizeCase(raw: Wire): AfterSaleCase {
	return {
		caseNo: str(raw.caseNo, raw.case_no),
		orderNo: str(raw.orderNo, raw.order_no),
		buyerId: str(raw.buyerId),
		buyerName: str(raw.buyerName, raw.buyer_name),
		// §231: AfterSaleCaseSummaryResponse sends the flat pair `avatarKey`
		// (objectKey SSOT) + `avatarUrl` (Gateway-composed). Read the key;
		// buildImageUrl resolves it.
		avatarKey: optStr(raw.avatarKey),
		sellerName: optStr(raw.sellerName, raw.seller_name),
		status: str(raw.status),
		reason: str(raw.reason, raw.reasonCode, raw.reason_code),
		description: str(
			raw.description,
			raw.note,
			raw.reasonText,
			raw.reason_text,
		),
		refundableAmount: str(raw.refundableAmount, raw.refundable_amount),
		refundAmount: optStr(raw.refundAmount, raw.refund_amount),
		returnShipmentNo: str(raw.returnShipmentNo, raw.return_shipment_no),
		returnShippingMethod: str(
			raw.returnShippingMethod,
			raw.return_shipping_method,
		),
		createdAt: str(raw.createdAt, raw.created_at),
		closedAt: optStr(raw.closedAt, raw.closed_at),
		// 後端列表端點（AfterSaleCaseSummaryResponse）已回傳 appealState；列表 row
		// 需消費它才能在 badge / 下一步 / 操作上反映「申訴仲裁中」。
		appealState: optStr(raw.appealState, raw.appeal_state),
		processDeadline: optStr(
			raw.processDeadline,
			raw.process_deadline,
			raw.deadline,
		),
		storePickupDeadlineAt: optStr(
			raw.storePickupDeadlineAt,
			raw.store_pickup_deadline_at,
		),
		shipBackDeadlineAt: optStr(
			raw.shipBackDeadlineAt,
			raw.ship_back_deadline_at,
		),
		shippingAddress: optStr(raw.shippingAddress, raw.shipping_address),
	};
}

function normalizeItem(raw: Wire): AfterSaleItem {
	// Detail items carry skuName (no product name) + imageKey/imageUrl +
	// originalPrice / actualPayAmount. Fall back across the camel/snake variants.
	// §231: `imageKey` is the objectKey SSOT (aftersales.go:141-142).
	const skuName = optStr(raw.skuName, raw.sku_name, raw.spec);
	return {
		orderItemId: str(raw.orderItemId, raw.order_item_id, raw.id),
		name: str(raw.name, raw.productName, raw.product_name) || skuName || "",
		image: optStr(raw.imageKey),
		skuName,
		quantity: num(raw.quantity, raw.qty),
		price: str(
			raw.actualPayAmount,
			raw.actual_pay_amount,
			raw.originalPrice,
			raw.original_price,
			raw.price,
			raw.unitPrice,
			raw.unit_price,
		),
	};
}

function normalizeResolution(raw: Wire): AfterSaleResolution {
	return {
		resolutionType: str(raw.resolutionType, raw.resolution_type, raw.type),
		refundAmount: optStr(raw.refundAmount, raw.refund_amount),
		couponAmount: optStr(raw.couponAmount, raw.coupon_amount),
		couponMinSpend: optStr(raw.couponMinSpend, raw.coupon_min_spend),
		couponMessage: optStr(raw.couponMessage, raw.coupon_message),
		rejectReason: optStr(raw.rejectReason, raw.reject_reason),
		rejectMessage: optStr(raw.rejectMessage, raw.reject_message),
		deductionReason: optStr(raw.deductionReason, raw.deduction_reason),
		inspectionNote: optStr(raw.inspectionNote, raw.inspection_note),
		createdAt: optStr(raw.createdAt, raw.created_at),
	};
}

function normalizeReturnLogistics(
	raw: Wire | undefined,
): AfterSaleReturnLogistics | undefined {
	if (!raw) return undefined;
	const logistics: AfterSaleReturnLogistics = {
		method: optStr(raw.method, raw.shippingMethod, raw.shipping_method),
		shipmentNo: optStr(raw.shipmentNo, raw.shipment_no, raw.returnShipmentNo),
		partnerId: optStr(raw.partnerId, raw.partner_id),
		odno: optStr(raw.odno, raw.odNo, raw.od_no),
		validationNo: optStr(raw.validationNo, raw.validation_no),
		shipmentStatus: optStr(raw.shipmentStatus, raw.shipment_status),
	};
	const hasAny = Object.values(logistics).some((v) => v != null);
	return hasAny ? logistics : undefined;
}

/** WorkerTaskStatus "5" = AWAITING_BUYER_REFUND_PAGE — the buyer must fill in
 * ATM bank details before the refund runs. */
const AWAITING_BUYER_REFUND_PAGE = "5";

/**
 * Build the refund-progress block. The real detail response carries
 * `refundTaskStatus` / `refundFailureMessage` as FLAT top-level fields (see
 * Apidog 售後詳情), so we read both the flat `raw` and a nested
 * `refundProgress` object. `needRefundPage` is DERIVED from refundTaskStatus,
 * not a separate boolean.
 */
function normalizeRefundProgress(
	raw: Wire | undefined,
	nested: Wire | undefined,
): AfterSaleRefundProgress | undefined {
	const src: Wire = { ...(raw ?? {}), ...(nested ?? {}) };
	const refundTaskStatus = optStr(src.refundTaskStatus, src.refund_task_status);
	const explicitNeed = bool(src.needRefundPage, src.need_refund_page);
	const progress: AfterSaleRefundProgress = {
		refundTaskStatus,
		estimatedRefundDate: optStr(
			src.estimatedRefundDate,
			src.estimated_refund_date,
		),
		bankDate: optStr(src.bankDate, src.bank_date),
		refundFailureMessage: optStr(
			src.refundFailureMessage,
			src.refund_failure_message,
		),
		needRefundPage:
			explicitNeed ?? refundTaskStatus === AWAITING_BUYER_REFUND_PAGE,
	};
	const hasAny = Object.values(progress).some((v) => v != null);
	return hasAny ? progress : undefined;
}

/**
 * The detail's `evidence[]` mixes buyer + seller photos, tagged by
 * `evidenceType` — a CodeMeta `AfterSaleEvidenceType` NUMERIC code on the wire
 * ("8601" 買家舉證 / "8602" 賣家驗貨照片), never a role literal. Split them so
 * the UI can show buyer application photos and seller inspection photos in
 * their own blocks. Only 8602 is the seller's; anything else (including an
 * untagged photo) is buyer evidence, so no image is ever dropped.
 */
function splitEvidence(raw: unknown): {
	buyer: AfterSaleEvidence[];
	seller: AfterSaleEvidence[];
} {
	const buyer: AfterSaleEvidence[] = [];
	const seller: AfterSaleEvidence[] = [];
	for (const item of asArray(raw)) {
		// §231: EvidenceResponse is `assetId / imageKey / imageUrl / evidenceType /
		// createdAt` (aftersales.go:146-152); imageKey is the objectKey SSOT and the
		// grid feeds it through resolveCdnUrl.
		const url = str(item.imageKey);
		if (!url) continue;
		const type = str(item.evidenceType, item.evidence_type);
		const isInspection = type === AfterSaleEvidenceType.INSPECTION_PHOTO;
		(isInspection ? seller : buyer).push({ url });
	}
	return { buyer, seller };
}

/** Build the return-logistics block from the detail's FLAT fields. */
function flatReturnLogistics(raw: Wire): AfterSaleReturnLogistics | undefined {
	const logistics: AfterSaleReturnLogistics = {
		method: optStr(raw.returnShippingMethod, raw.return_shipping_method),
		shipmentNo: optStr(raw.returnShipmentNo, raw.return_shipment_no),
		partnerId: optStr(raw.partnerId, raw.partner_id),
		odno: optStr(raw.odno, raw.od_no),
		validationNo: optStr(raw.validationNo, raw.validation_no),
		shipmentStatus: optStr(
			raw.returnProviderMessage,
			raw.return_provider_message,
			raw.returnProviderStatus,
			raw.return_provider_status,
		),
	};
	const hasAny = Object.values(logistics).some((v) => v != null);
	return hasAny ? logistics : undefined;
}

export function normalizeDetail(raw: Wire): AfterSaleDetail {
	const base = normalizeCase(raw);
	const evidence = splitEvidence(raw.evidence);
	return {
		...base,
		items: asArray(raw.items ?? raw.orderItems ?? raw.order_items).map(
			normalizeItem,
		),
		photos: evidence.buyer,
		sellerPhotos: evidence.seller,
		resolutions: asArray(
			raw.resolutions ?? raw.decisions ?? raw.resolutionHistory,
		).map(normalizeResolution),
		// Detail return-logistics fields are flat; prefer the nested object if
		// the backend ever sends one.
		returnLogistics:
			normalizeReturnLogistics(
				(raw.returnLogistics ?? raw.return_logistics) as Wire | undefined,
			) ?? flatReturnLogistics(raw),
		// Flat top-level refund fields + optional nested object.
		refundProgress: normalizeRefundProgress(
			raw,
			(raw.refundProgress ?? raw.refund_progress) as Wire | undefined,
		),
		canAppeal: bool(raw.canAppeal, raw.can_appeal),
		appealDeadline: optStr(raw.appealDeadline, raw.appeal_deadline),
		appealState: optStr(raw.appealState, raw.appeal_state),
		autoEscalated: bool(raw.autoEscalated, raw.auto_escalated),
		autoConfirmAt: optStr(raw.autoConfirmAt, raw.auto_confirm_at),
		returnDeliveredAt: optStr(raw.returnDeliveredAt, raw.return_delivered_at),
		returnProviderMessage: optStr(
			raw.returnProviderMessage,
			raw.return_provider_message,
		),
		returnProviderStatus: optStr(
			raw.returnProviderStatus,
			raw.return_provider_status,
		),
	};
}

/** Normalize GET .../arbitration. */
export function normalizeArbitration(raw: Wire): AfterSaleArbitration {
	const decisionRaw = (raw.decision ?? null) as Wire | null;
	const decision =
		decisionRaw && Object.keys(decisionRaw).length > 0
			? {
					outcome: str(decisionRaw.outcome),
					reason: str(decisionRaw.reason),
					refundAmount: optStr(
						decisionRaw.refundAmount,
						decisionRaw.refund_amount,
					),
					decidedAt: optStr(decisionRaw.decidedAt, decisionRaw.decided_at),
				}
			: undefined;
	return {
		state: str(raw.state),
		decision,
		timeline: asArray(raw.timeline).map((e) => ({
			eventType: str(e.eventType, e.event_type),
			label: str(e.label),
			description: str(e.description),
			timestamp: str(e.timestamp),
		})),
	};
}
