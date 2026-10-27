import { PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT } from "@/lib/product/quickViewOrigin";

/**
 * Pre-hydration quick-view origin capture.
 *
 * The home page streams a large client tree. A mouse click can therefore reach
 * a product link while React is still hydrating it. The tiny capture listener
 * records the tapped card geometry so the finished modal can grow from the
 * correct origin. It deliberately paints no pending shell: the complete source
 * page remains visible until the intercepted product route is ready.
 */
export default function ProductQuickViewPendingBridge() {
	return (
		<script
			// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted pre-hydration interaction bridge
			dangerouslySetInnerHTML={{
				__html: PRODUCT_QUICK_VIEW_PENDING_BOOT_SCRIPT,
			}}
		/>
	);
}
