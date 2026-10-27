"use client";

import ErrorState from "@/components/shared/ErrorState";

/**
 * Commerce-zone error boundary (cart / checkout / payments). Money-adjacent
 * copy: reassure that nothing was charged twice, and point back at the cart
 * instead of the home page — the buyer was mid-purchase.
 */
export default function CommerceError({
	error,
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	return (
		<ErrorState
			error={error}
			reset={reset}
			title="結帳路上出了點狀況"
			description={
				<>
					剛剛的頁面沒有載入成功——放心，不會因此重複扣款。
					<br />
					再試一次，或回購物車確認商品後重新結帳。
				</>
			}
			homeHref="/cart"
			homeLabel="回購物車"
		/>
	);
}
