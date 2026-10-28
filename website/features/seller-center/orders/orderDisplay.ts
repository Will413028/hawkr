/**
 * Shared display formatters for the seller order list + detail pages so both
 * surfaces render the same deadline / label-expiry strings.
 * Mirrors the seller APP's `OrderInfoColumn` formatting helpers.
 */

import { formatTaipeiMonthDayTime } from "@/lib/date/taipei";

/** Format a normalized deadline (unix s or ms) as "MM/DD HH:mm" — mirrors the
 * APP's `_formatShortDate` on the order card. Empty string when absent. */
export const formatDeadline = formatTaipeiMonthDayTime;

/** Format a label-expiry wire value (ISO string or unix s/ms) as "MM/DD HH:mm".
 * Empty string when absent / unparseable so the row is simply not rendered. */
export const formatLabelExpire = formatTaipeiMonthDayTime;
