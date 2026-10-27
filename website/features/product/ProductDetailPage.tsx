import { Suspense } from "react";
import ProductUnavailableView from "@/features/product/ProductUnavailableView";
import { getCachedProduct } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import ProductReviewsSkeleton from "./components/ProductReviewsSkeleton";
import ProductSellerIntro from "./components/ProductSellerIntro";
import {
	deriveDetailLayoutProps,
	getProductSellerStoreId,
} from "./deriveDetailLayoutProps";
import ProductDetailLayout from "./ProductDetailLayout";
import ReviewsAsync from "./ReviewsAsync";
import SellerIntroAsync from "./SellerIntroAsync";

interface Props {
	productId: string;
}

export default async function ProductDetailPage({ productId: id }: Props) {
	const product = await getCachedProduct(id).catch(prerenderSafe(null));

	if (!product) {
		return <ProductUnavailableView productId={id} />;
	}

	const sellerStoreId = getProductSellerStoreId(product);
	const productSeller = (product as Record<string, unknown>).seller;
	const sellerIntroSlot = sellerStoreId ? (
		<Suspense
			key="seller-intro"
			fallback={
				<ProductSellerIntro
					sellerId={sellerStoreId}
					productSeller={productSeller}
					pending
				/>
			}
		>
			<SellerIntroAsync
				sellerId={sellerStoreId}
				productSeller={productSeller}
			/>
		</Suspense>
	) : undefined;
	const reviewsSlot = (
		<Suspense key="reviews" fallback={<ProductReviewsSkeleton />}>
			<ReviewsAsync productId={id} />
		</Suspense>
	);

	const derived = deriveDetailLayoutProps({
		product: product as Record<string, unknown>,
		seller: null,
		sellerStats: null,
		reviewsResult: null,
		// Media gateway short-circuits the OSS dimension probe (recedes to no-op
		// once MEDIA_BASE_URL is set), so image blocks fall back to a raw <img>,
		// matching the products-import preview page.
		descriptionImageDimEntries: [],
	});

	return (
		<ProductDetailLayout
			productId={id}
			{...derived}
			sellerIntroSlot={sellerIntroSlot}
			reviewsSlot={reviewsSlot}
		/>
	);
}
