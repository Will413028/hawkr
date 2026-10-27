"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import { useDragScroll } from "@/lib/hooks/useDragScroll";
import type { MarketProductItem } from "@/lib/market/types";
import { resolveCdnUrl } from "@/lib/media/cdn";
import type { SellerCardProductStatus } from "./product-state";

interface SellerCardProductRailProps {
	products: MarketProductItem[];
	sellerName: string;
	status: SellerCardProductStatus;
}

export default function SellerCardProductRail({
	products,
	sellerName,
	status,
}: SellerCardProductRailProps) {
	const railRef = React.useRef<HTMLOListElement | null>(null);
	useDragScroll(railRef);

	const scroll = (direction: -1 | 1) => {
		const rail = railRef.current;
		if (!rail) return;
		rail.scrollBy({
			left: direction * Math.min(rail.clientWidth * 0.82, 640),
			behavior: "smooth",
		});
	};

	return (
		<section
			className="seller-card-products"
			aria-labelledby="seller-card-products-title"
			data-seller-card-product-rail
		>
			<header className="seller-card-products__header">
				<div>
					<p className="seller-card-products__eyebrow">本店商品</p>
					<h2 id="seller-card-products-title">看看 {sellerName} 上架的商品</h2>
				</div>
				{products.length > 3 ? (
					<div className="seller-card-products__controls">
						<button
							type="button"
							onClick={() => scroll(-1)}
							aria-label="查看前一批商品"
						>
							<AppIcon name="chevron-left" size={18} />
						</button>
						<button
							type="button"
							onClick={() => scroll(1)}
							aria-label="查看下一批商品"
						>
							<AppIcon name="chevron-right" size={18} />
						</button>
					</div>
				) : null}
			</header>

			{status === "unavailable" ? (
				<div className="seller-card-products__empty">
					<AppIcon name="bag" size={22} />
					<p>商品暫時無法載入。</p>
				</div>
			) : products.length > 0 ? (
				<ol ref={railRef} className="seller-card-products__rail">
					{products.map((product, index) => {
						const soldOut = product.stock !== undefined && product.stock <= 0;
						return (
							<li key={product.id}>
								<Link href={product.href} className="seller-card-product">
									<span className="seller-card-product__media">
										<AppImage
											src={resolveCdnUrl(product.image)}
											alt={product.name}
											fill
											preset="card"
											sizes="(max-width: 640px) 72vw, 240px"
											loading={index < 2 ? "eager" : "lazy"}
											className="seller-card-product__image"
										/>
										{soldOut ? (
											<span className="seller-card-product__status">
												已售完
											</span>
										) : null}
									</span>
									<span className="seller-card-product__body">
										<strong>{product.name}</strong>
										<span className="seller-card-product__price-row">
											{product.price ? <b>{product.price}</b> : <b>查看商品</b>}
											{product.isOnSale && product.originalPrice ? (
												<s>{product.originalPrice}</s>
											) : null}
										</span>
									</span>
								</Link>
							</li>
						);
					})}
				</ol>
			) : (
				<div className="seller-card-products__empty">
					<AppIcon name="bag" size={22} />
					<p>這間店目前還沒有上架商品。</p>
				</div>
			)}
		</section>
	);
}
