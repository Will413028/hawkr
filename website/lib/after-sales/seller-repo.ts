/**
 * Seller-side after-sale API.
 *
 *   GET  /app/seller/after_sales?page=&page_size=&status= — list cases
 *   GET  /app/seller/after_sales/{caseNo}                 — case detail
 *   POST /app/seller/after_sales/{caseNo}/resolve         — 6 decisions
 *   POST /app/seller/after_sales/{caseNo}/confirm_receipt — receive + inspect
 *
 * The list pulls a single page of 100 — matches the APP `keepAlive` cache and
 * is plenty for the typical seller. Soft-fails to empty on 404 so the order
 * page keeps rendering before the endpoint is live on an env.
 */

import { apiClient } from "../api/client";
import { normalizeArbitration, normalizeDetail } from "./normalize";
import { fetchAfterSaleList } from "./shared";
import type {
	AfterSaleArbitration,
	AfterSaleDetail,
	AfterSaleListResult,
	ConfirmReceiptInput,
	ResolveAfterSaleInput,
} from "./types";

export function fetchSellerAfterSales(
	params: { page?: number; pageSize?: number; status?: string } = {},
): Promise<AfterSaleListResult> {
	return fetchAfterSaleList("/app/seller/after_sales", {
		pageSize: 100,
		...params,
	});
}

/** GET /app/seller/after_sales/{caseNo} — full case detail (seller view). */
export async function fetchSellerAfterSaleDetail(
	caseNo: string,
): Promise<AfterSaleDetail> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/seller/after_sales/${encodeURIComponent(caseNo)}`,
	);
	return normalizeDetail(env.data ?? {});
}

export interface ResolveResult {
	/** 7-11 return code surfaced in the success toast, when present. */
	returnShipmentNo?: string;
	partnerId?: string;
	validationNo?: string;
}

/** POST .../resolve — apply one of the 6 seller decisions. */
export async function resolveAfterSale(
	input: ResolveAfterSaleInput,
): Promise<ResolveResult> {
	const { caseNo, ...body } = input;
	const env = await apiClient.authed.post<Record<string, unknown>>(
		`/app/seller/after_sales/${encodeURIComponent(caseNo)}/resolve`,
		{ json: body },
	);
	const data = env.data ?? {};
	return {
		returnShipmentNo:
			(data.returnShipmentNo as string) ??
			(data.return_shipment_no as string) ??
			undefined,
		partnerId:
			(data.partnerId as string) ?? (data.partner_id as string) ?? undefined,
		validationNo:
			(data.validationNo as string) ??
			(data.validation_no as string) ??
			undefined,
	};
}

/** POST .../confirm_receipt — confirm return received + run refund, with
 * optional inspection deduction. */
export async function confirmReceipt(
	input: ConfirmReceiptInput,
): Promise<boolean> {
	const { caseNo, ...body } = input;
	await apiClient.authed.post(
		`/app/seller/after_sales/${encodeURIComponent(caseNo)}/confirm_receipt`,
		{
			json: {
				...body,
				inspectionPhotoTokens: body.inspectionPhotoTokens ?? [],
			},
		},
	);
	return true;
}

/** GET /app/seller/after_sales/{caseNo}/arbitration — seller arbitration view. */
export async function fetchSellerAfterSaleArbitration(
	caseNo: string,
): Promise<AfterSaleArbitration> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/seller/after_sales/${encodeURIComponent(caseNo)}/arbitration`,
	);
	return normalizeArbitration(env.data ?? {});
}

/** @deprecated Use {@link fetchSellerAfterSaleDetail}. Kept for the old
 * `lib/seller/after-sales` import surface. */
export const fetchAfterSaleDetail = fetchSellerAfterSaleDetail;
