/**
 * Buyer-side after-sale API.
 *
 *   GET  /app/after_sales                 — buyer's own cases
 *   GET  /app/after_sales/{caseNo}        — case detail
 *   POST /app/after_sales                 — create case
 *   POST /app/after_sales/{caseNo}/cancel — withdraw a pending case
 *   POST /app/after_sales/{caseNo}/supplement — add evidence/notes
 *   POST /app/after_sales/{caseNo}/appeal     — escalate to platform arbitration
 */

import { apiClient } from "../api/client";
import { normalizeArbitration, normalizeDetail } from "./normalize";
import { fetchAfterSaleList } from "./shared";
import type {
	AfterSaleArbitration,
	AfterSaleDetail,
	AfterSaleListResult,
	CreateAfterSaleInput,
} from "./types";

/** GET /app/after_sales — buyer's own after-sale cases. Soft-fails to an
 * empty list on 404 so the history page renders before the endpoint is live. */
export function fetchMyAfterSales(
	params: { page?: number; pageSize?: number; status?: string } = {},
): Promise<AfterSaleListResult> {
	return fetchAfterSaleList("/app/after_sales", params);
}

/** GET /app/after_sales/{caseNo} — full case detail (buyer view). */
export async function fetchMyAfterSaleDetail(
	caseNo: string,
): Promise<AfterSaleDetail> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/after_sales/${encodeURIComponent(caseNo)}`,
	);
	return normalizeDetail(env.data ?? {});
}

/** POST /app/after_sales — create an after-sale case for an order. */
export async function createAfterSale(
	input: CreateAfterSaleInput,
): Promise<{ caseNo: string }> {
	const env = await apiClient.authed.post<Record<string, unknown>>(
		"/app/after_sales",
		{
			json: {
				orderNo: input.orderNo,
				reason: input.reason,
				description: input.description,
				photoTokens: input.photoTokens ?? [],
			},
		},
	);
	const data = env.data ?? {};
	return { caseNo: String(data.caseNo ?? data.case_no ?? "") };
}

async function caseAction(
	caseNo: string,
	action: "cancel" | "supplement" | "appeal",
	json?: Record<string, unknown>,
): Promise<boolean> {
	await apiClient.authed.post(
		`/app/after_sales/${encodeURIComponent(caseNo)}/${action}`,
		json ? { json } : {},
	);
	return true;
}

/** Withdraw a still-pending case. POST .../cancel (no body). */
export function cancelAfterSale(caseNo: string): Promise<boolean> {
	return caseAction(caseNo, "cancel");
}

/** Supplement a case. Body: `{ additionalDescription, photoTokens }`. */
export function supplementAfterSale(
	caseNo: string,
	body: { additionalDescription?: string; photoTokens?: string[] },
): Promise<boolean> {
	return caseAction(caseNo, "supplement", {
		additionalDescription: body.additionalDescription ?? "",
		photoTokens: body.photoTokens ?? [],
	});
}

/** Escalate to platform arbitration.
 * Body: `{ reason, description, evidencePhotoTokens }` (up to 5 photos). */
export function appealAfterSale(
	caseNo: string,
	body: {
		reason: string;
		description?: string;
		evidencePhotoTokens?: string[];
	},
): Promise<boolean> {
	return caseAction(caseNo, "appeal", {
		reason: body.reason,
		description: body.description ?? "",
		evidencePhotoTokens: body.evidencePhotoTokens ?? [],
	});
}

/**
 * ATM refunds need the buyer to supply a bank account on PayUni's hosted
 * refund page. Keyed by ORDER NO (`POST /app/orders/{orderNo}/refund_page`),
 * not the case — there is no after-sale-scoped refund-page endpoint. Returns
 * the redirect URL to bounce to. Soft-fails to an empty result so the banner
 * can show a fallback message rather than throwing.
 *
 * MUST send `clientType: "WEB_BROWSER"` (design doc 09 §5.4.2): the backend
 * persists it on the refund task and uses it to build PayUni's return URL.
 * Omitting it falls back to MOBILE_APP, which returns a
 * `hawkrbuyer://refund/result` deep link the browser can't open.
 *
 * NOTE: the gateway binds the body field as camelCase `clientType`
 * (`GetRefundPageUrlRequest`, json:"clientType"). A snake_case `client_type`
 * is silently dropped by `ShouldBindJSON` → empty → MOBILE_APP fallback.
 * `appType` is NOT sent here — the gateway derives it from the JWT account type.
 */
export async function fetchRefundPage(
	orderNo: string,
): Promise<{ redirectUrl?: string }> {
	try {
		const env = await apiClient.authed.post<Record<string, unknown>>(
			`/app/orders/${encodeURIComponent(orderNo)}/refund_page`,
			{ json: { clientType: "WEB_BROWSER" }, throwOnHttpError: false },
		);
		const data = env.data ?? {};
		return {
			redirectUrl:
				(data.redirectUrl as string) ??
				(data.redirect_url as string) ??
				(data.url as string) ??
				undefined,
		};
	} catch {
		return {};
	}
}

/** GET /app/after_sales/{caseNo}/arbitration — buyer arbitration view. */
export async function fetchAfterSaleArbitration(
	caseNo: string,
): Promise<AfterSaleArbitration> {
	const env = await apiClient.authed.get<Record<string, unknown>>(
		`/app/after_sales/${encodeURIComponent(caseNo)}/arbitration`,
	);
	return normalizeArbitration(env.data ?? {});
}
