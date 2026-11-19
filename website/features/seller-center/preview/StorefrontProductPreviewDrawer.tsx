"use client";

import * as React from "react";
import { Drawer } from "@/components/shared/Drawer";
import Button from "@/components/ui/Button";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";
import ProductSellerIntro from "@/features/product/components/ProductSellerIntro";
import { deriveDetailLayoutProps } from "@/features/product/deriveDetailLayoutProps";
import ProductDetailLayout from "@/features/product/ProductDetailLayout";
import { fetchSellerProductEdit } from "@/lib/products/seller-repo";
import { fetchProductReviews } from "@/lib/reviews/repo";
import type { PublicStoreView } from "@/lib/seller/public-store";
import {
	previewDetailProduct,
	previewStoreStats,
} from "@/lib/seller/storefront-preview";

interface StorefrontProductPreviewDrawerProps {
	productId: string | null;
	productName?: string;
	store: PublicStoreView;
	onClose: () => void;
}

type ProductReviewsResult = Awaited<
	ReturnType<typeof fetchProductReviews>
> | null;

interface LoadedProductPreview {
	detail: NonNullable<Awaited<ReturnType<typeof fetchSellerProductEdit>>>;
	reviewsResult: ProductReviewsResult;
}

interface ProductPreviewView {
	product: Record<string, unknown>;
	store: PublicStoreView;
	reviewsResult: ProductReviewsResult;
}

type PreviewLoadState =
	| { status: "idle" | "loading" }
	| { status: "ready"; data: LoadedProductPreview }
	| { status: "unavailable" }
	| { status: "error" };

function ProductPreviewDetail({ data }: { data: ProductPreviewView }) {
	const product = data.product;
	const { sellerStats, followerStats } = previewStoreStats(data.store);
	const derived = deriveDetailLayoutProps({
		product,
		seller: product.seller,
		sellerStats,
		reviewsResult: data.reviewsResult,
		descriptionImageDimEntries: [],
	});

	return (
		<div
			className="storefront-product-preview-drawer__content"
			data-storefront-product-preview-ready={String(data.product.id ?? "")}
			onClickCapture={(event) => {
				const target = event.target;
				if (!(target instanceof Element) || !target.closest("a[href]")) return;
				event.preventDefault();
				event.stopPropagation();
			}}
			onSubmitCapture={(event) => {
				event.preventDefault();
				event.stopPropagation();
			}}
		>
			<ProductDetailLayout
				previewMode
				productId={String(data.product.id ?? "")}
				{...derived}
				sellerIntroSlot={
					<ProductSellerIntro
						previewMode
						sellerId={data.store.storeId}
						seller={product.seller}
						sellerStats={sellerStats}
						followerStats={followerStats}
						productSeller={product.seller}
					/>
				}
			/>
		</div>
	);
}

export default function StorefrontProductPreviewDrawer({
	productId,
	productName,
	store,
	onClose,
}: StorefrontProductPreviewDrawerProps) {
	const [reloadKey, setReloadKey] = React.useState(0);
	const [loadState, setLoadState] = React.useState<PreviewLoadState>({
		status: "idle",
	});

	React.useEffect(() => {
		void reloadKey;
		if (!productId) {
			setLoadState({ status: "idle" });
			return;
		}

		let active = true;
		setLoadState({ status: "loading" });
		(async () => {
			try {
				const detail = await fetchSellerProductEdit(productId);
				if (!active) return;
				if (!detail) {
					setLoadState({ status: "unavailable" });
					return;
				}
				const reviewsResult = await fetchProductReviews(productId, {
					page: 1,
					pageSize: 6,
				}).catch(() => null);
				if (!active) return;
				setLoadState({
					status: "ready",
					data: { detail, reviewsResult },
				});
			} catch {
				if (active) setLoadState({ status: "error" });
			}
		})();

		return () => {
			active = false;
		};
	}, [productId, reloadKey]);

	const preview = React.useMemo<ProductPreviewView | null>(
		() =>
			loadState.status === "ready"
				? {
						product: previewDetailProduct(loadState.data.detail, store),
						store,
						reviewsResult: loadState.data.reviewsResult,
					}
				: null,
		[loadState, store],
	);

	const description =
		preview !== null
			? String(preview.product.name ?? productName ?? "")
			: productName || "以買家看到的公開資料呈現";

	return (
		<Drawer
			open={productId !== null}
			onClose={onClose}
			side="right"
			title="商品詳情預覽"
			description={description}
			closeLabel="關閉商品詳情預覽"
			className="storefront-product-preview-drawer max-w-[min(960px,72vw)] max-md:max-w-none max-md:rounded-none"
			overlayClassName="storefront-product-preview-drawer__overlay max-md:p-0"
			bodyClassName="storefront-product-preview-drawer__body p-0 [overscroll-behavior:contain]"
			data-storefront-product-preview-drawer="true"
		>
			{loadState.status === "loading" || loadState.status === "idle" ? (
				<DrawerSkeleton rows={5} className="p-6" />
			) : null}
			{preview ? <ProductPreviewDetail data={preview} /> : null}
			{loadState.status === "unavailable" ? (
				<div
					className="grid min-h-64 place-items-center px-6 py-12 text-center"
					role="status"
				>
					<div className="max-w-sm">
						<h3 className="m-0 text-lg font-bold text-hk-heading">
							目前無法預覽這項商品
						</h3>
						<p className="mb-0 mt-2 text-sm leading-6 text-hk-muted">
							商品可能已下架，或已不屬於目前預覽的商店。
						</p>
					</div>
				</div>
			) : null}
			{loadState.status === "error" ? (
				<div
					className="grid min-h-64 place-items-center px-6 py-12 text-center"
					role="alert"
				>
					<div className="max-w-sm">
						<h3 className="m-0 text-lg font-bold text-hk-heading">
							商品資料載入失敗
						</h3>
						<p className="mb-5 mt-2 text-sm leading-6 text-hk-muted">
							請確認網路連線後再試一次。
						</p>
						<Button
							type="button"
							color="primary"
							onClick={() => setReloadKey((value) => value + 1)}
						>
							重新載入
						</Button>
					</div>
				</div>
			) : null}
		</Drawer>
	);
}
