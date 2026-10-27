import { Suspense } from "react";
import RouteLoadingSkeleton from "@/components/shared/RouteLoadingSkeleton";
import { ProductDetailPage } from "@/features/product";
import ProductModal from "@/features/product/components/ProductModal";

interface Props {
	params: Promise<{ id: string }>;
}

/**
 * Intercepted product view: a soft navigation to /products/[id] from anywhere
 * under (public) renders here, in the @modal slot, OVER the page you were on
 * (which stays mounted). Open the modal shell immediately, then stream the
 * product tree into it. SEO / canonical / JSON-LD stay on the real page
 * (app/[locale]/(public)/products/[id]/page.tsx), which serves hard loads + refreshes.
 */
export default function InterceptedProductModalPage({ params }: Props) {
	return (
		<ProductModal>
			<Suspense
				fallback={<RouteLoadingSkeleton page variant="product-detail" />}
			>
				<InterceptedProductModalContent params={params} />
			</Suspense>
		</ProductModal>
	);
}

async function InterceptedProductModalContent({ params }: Props) {
	const { id } = await params;
	return <ProductDetailPage productId={id} />;
}
