"use client";

import { Drawer } from "@/components/shared/Drawer";
import ProductReviewsPanel from "./ProductReviewsPanel";
import type { ProductReviewsTarget } from "./useProductReviewsDrawer";

/**
 * Right-side drawer that shows one product's buyer reviews — same shared
 * {@link Drawer} primitive as every other drawer. Read-only, so it omits
 * `onMutated` (no parent list to re-sync). Opened from the 評分 cell in the
 * 商品管理 list; `target` carries the product whose reviews to load (`null`
 * keeps it closed). Keyed by product id so each open mounts a fresh panel.
 */
export default function ProductReviewsDrawer({
	target,
	onClose,
}: {
	target: ProductReviewsTarget | null;
	onClose: () => void;
}) {
	return (
		<Drawer
			open={target !== null}
			onClose={onClose}
			side="right"
			title={target ? `「${target.name}」的買家評論` : "買家評論"}
			closeLabel="關閉評論"
			className="max-w-xl"
		>
			{target ? (
				<ProductReviewsPanel
					key={target.id}
					productId={target.id}
					productImage={target.image}
				/>
			) : null}
		</Drawer>
	);
}
