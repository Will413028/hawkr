/**
 * Payment-status classification — single source of truth for "is this
 * payment terminal?" across the web flow.
 *
 * Why this exists: PayUni redirects buyers back to our site with a
 * `status` query parameter and our `/app/payments/{trade_no}` endpoint
 * also returns a status string. The two surfaces use overlapping but
 * not identical vocabularies — PayUni Web uses numeric codes (`1` =
 * success, `2` = failure), while our backend's gateway-synced state
 * uses named strings (`paid`, `failed`, `cancelled`, `expired`, …).
 * Centralising the mapping prevents drift between the redirect landing
 * page, /payment/return, and any future PayUni-aware surface.
 *
 * NEVER trust query-parameter `status` alone — the buyer can edit the
 * URL. Always cross-check with `fetchPayment(tradeNo)` before declaring
 * a payment paid. This module only normalises the vocabulary; the
 * trust check lives in the resolution hook.
 */

export type PaymentOutcome = "paid" | "failed" | "pending";

const PAID_TOKENS = new Set(["paid", "success", "completed", "1", "ok"]);

/**
 * Backend's payment status code table (confirmed against #760 spec):
 *   "1" paid · "2" failed · "0" processing · "3" expired
 * "0" is intentionally NOT here — it must fall through to "pending" so
 * the resolution hook keeps polling. "3" maps to failed because the
 * payment is terminal (expired) and the UI should route to the fail
 * surface instead of looping until our 24s polling cap.
 */
const FAILED_TOKENS = new Set([
	"failed",
	"failure",
	"cancelled",
	"canceled",
	"expired",
	"rejected",
	"2",
	"3",
	"fail",
]);

/**
 * Map a status string from any of: PayUni redirect query, /app/payments
 * GET response, or /app/payments/status_syncs POST response, into one
 * of three terminal outcomes. Unknown / unrecognised values fall through
 * to "pending" so the caller keeps polling rather than failing closed.
 */
export function classifyPaymentStatus(
	raw: string | null | undefined,
): PaymentOutcome {
	const value = (raw ?? "").trim().toLowerCase();
	if (!value) return "pending";
	if (PAID_TOKENS.has(value)) return "paid";
	if (FAILED_TOKENS.has(value)) return "failed";
	return "pending";
}
