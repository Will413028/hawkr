/**
 * @deprecated Moved to `lib/after-sales/`. This module is now a thin
 * re-export so existing seller imports (`@/lib/seller/after-sales`) keep
 * compiling. Prefer importing from `@/lib/after-sales` directly.
 */

export {
	fetchAfterSaleDetail,
	fetchSellerAfterSales,
} from "../after-sales/seller-repo";
export {
	AFTER_SALE_STATUS_LABEL,
	afterSaleGroupOf,
	afterSaleIsTerminal,
	afterSaleNeedsSeller,
	buildAfterSaleLookup,
} from "../after-sales/status";
export type {
	AfterSaleCase,
	AfterSaleGroup,
	AfterSaleListResult,
} from "../after-sales/types";
