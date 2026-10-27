"use client";

import * as React from "react";
import { Drawer } from "@/components/shared/Drawer";
import Button from "@/components/ui/Button";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";
import ProductSellerIntro from "@/features/product/components/ProductSellerIntro";
import { deriveDetailLayoutProps } from "@/features/product/deriveDetailLayoutProps";
import ProductDetailLayout from "@/features/product/ProductDetailLayout";
import {
	fetchPublicProduct,
	fetchSeller,
	fetchSellerReviewStats,
	fetchSellerStats,
} from "@/lib/api";
import { fetchProductReviews } from "@/lib/reviews/repo";
import { sellerIdFromPreviewProduct } from "./storefrontProductPreview";

interface StorefrontProductPreviewDrawerProps {
	productId: string | null;
	productName?: string;
	sellerId: string;
	onClose: () => void;
}

interface LoadedProductPreview {
	product: Record<string, unknown>;
	ownerSellerId: string;
	seller: unknown;
	sellerReviewStats: unknown;
	followerStats: Awaited<ReturnType<typeof fetchSellerStats>>;
	reviewsResult: Awaited<ReturnType<typeof fetchProductReviews>> | null;
}

type PreviewLoadState =
	| { status: "idle" | "loading" }
	| { status: "ready"; data: LoadedProductPreview }
	| { status: "unavailable" }
	| { status: "error" };

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function ProductPreviewDetail({ data }: { data: LoadedProductPreview }) {
	const embeddedSeller = asRecord(data.product.seller);
	// Public detail payloads may carry storeId as a number. Normalise it into the
	// layout's string-based seller seam after the ownership check has succeeded.
	const product = {
		...data.product,
		storeId: data.ownerSellerId,
		seller: { ...embeddedSeller, storeId: data.ownerSellerId },
	};
	const derived = deriveDetailLayoutProps({
		product,
		seller: data.seller,
		sellerStats: data.sellerReviewStats,
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
						sellerId={data.ownerSellerId}
						seller={data.seller}
						sellerStats={data.sellerReviewStats}
						followerStats={data.followerStats}
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
	sellerId,
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
				const product = await fetchPublicProduct(productId);
				if (!active) return;
				const ownerSellerId = sellerIdFromPreviewProduct(product);
				if (!product || ownerSellerId !== sellerId) {
					setLoadState({ status: "unavailable" });
					return;
				}

				const [seller, sellerReviewStats, followerStats, reviewsResult] =
					await Promise.all([
						fetchSeller(ownerSellerId).catch(() => null),
						fetchSellerReviewStats(ownerSellerId).catch(() => null),
						fetchSellerStats(ownerSellerId).catch(() => null),
						fetchProductReviews(productId, { page: 1, pageSize: 6 }).catch(
							() => null,
						),
					]);
				if (!active) return;
				setLoadState({
					status: "ready",
					data: {
						product: product as Record<string, unknown>,
						ownerSellerId,
						seller,
						sellerReviewStats,
						followerStats,
						reviewsResult,
					},
				});
			} catch {
				if (active) setLoadState({ status: "error" });
			}
		})();

		return () => {
			active = false;
		};
	}, [productId, sellerId, reloadKey]);

	const description =
		loadState.status === "ready"
			? String(loadState.data.product.name ?? productName ?? "")
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
			{loadState.status === "ready" ? (
				<ProductPreviewDetail data={loadState.data} />
			) : null}
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
