"use client";

import {
	COMPACT_BACK_CONTROL_CLASSNAME,
	CompactBackIcon,
} from "@/components/shared/CompactBackControl";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * Returns to the previous page (usually the market list the user zoomed in
 * from). PageTransition detects "leaving a product detail" and shrinks the page
 * back into the originating card; this button just triggers the navigation.
 * Falls back to the market list for direct visits / deep links with no history.
 */
export default function ProductBackButton() {
	const router = useObservedRouter("product_back_button");

	const handleBack = () => {
		// `history.length > 1` ≈ arrived via an in-app navigation, so `back()`
		// returns to the originating list (and runs the reverse zoom). Otherwise
		// send the user to the market list explicitly.
		if (typeof window !== "undefined" && window.history.length > 1) {
			router.back();
		} else {
			router.push("/market/products");
		}
	};

	return (
		<button
			type="button"
			onClick={handleBack}
			// PageTransition's capture-phase click listener keys off this to snapshot
			// the PDP for the reverse zoom before `router.back()` swaps the route.
			data-zoom-back=""
			aria-label="返回"
			className={COMPACT_BACK_CONTROL_CLASSNAME}
		>
			<CompactBackIcon />
			返回
		</button>
	);
}
