/**
 * Product detail feature module — buyer-facing PDP at /products/:id.
 *
 * The route layer (app/products/[id]/page.tsx) handles only the [id]
 * extraction + Next metadata; the actual SSR composition lives here.
 */

export { default as ProductDetailPage } from "./ProductDetailPage";
export { ProductViewTracker } from "./ProductViewTracker";
