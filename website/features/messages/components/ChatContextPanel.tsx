"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import ChatPeerAvatar from "@/features/messages/components/ChatPeerAvatar";
import { fetchSeller, fetchSellerProducts } from "@/lib/api";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { sellerPath } from "@/lib/seo/urls";

/**
 * Third-column context panel for buyer↔seller threads — the Airbnb
 * "Reservation" slot. A marketplace conversation is almost always
 * about a product or an order, so the panel keeps that commerce
 * context (shop card + a few products + storefront CTA) on screen
 * next to the thread instead of leaving the right edge a void.
 *
 * Only rendered on the buyer side (the peer is a seller with a public
 * profile) and only displayed at >=1440px via `.chat-context` CSS —
 * below that the thread keeps the full width. Renders nothing until
 * the seller profile resolves so the grid never shows an empty rail.
 */

interface ProductLite {
	id: string;
	name: string;
	price: string;
	imageUrl: string;
}

function pickString(raw: Record<string, unknown>, keys: string[]): string {
	for (const key of keys) {
		const value = raw[key];
		if (typeof value === "string" && value.trim()) return value.trim();
		if (typeof value === "number" && Number.isFinite(value)) {
			return String(value);
		}
	}
	return "";
}

function normalizeProduct(raw: Record<string, unknown>): ProductLite {
	return {
		id: pickString(raw, ["id", "productId", "product_id"]),
		name: pickString(raw, ["name", "productName", "product_name"]),
		price: pickString(raw, [
			"priceMin",
			"price_min",
			"price",
			"unitPrice",
			"unit_price",
		]),
		imageUrl: pickString(raw, ["imageUrl", "image", "cover", "thumb"]),
	};
}

export default function ChatContextPanel({ sellerId }: { sellerId: string }) {
	const [seller, setSeller] = React.useState<{
		name: string;
		logo_url: string;
		/** Wire name (gateway `PublicSellerInfo.storeCode`, backend#2235). `fetchSeller`
		 *  passes it through unrenamed — only `logo_url` is a display alias. */
		storeCode: string;
	} | null>(null);
	const [products, setProducts] = React.useState<ProductLite[]>([]);

	React.useEffect(() => {
		if (!sellerId) return;
		let cancelled = false;
		void (async () => {
			const [profile, catalog] = await Promise.all([
				fetchSeller(sellerId).catch(() => null),
				fetchSellerProducts(sellerId, 1, 3).catch(() => null),
			]);
			if (cancelled) return;
			if (profile) {
				setSeller({
					name: profile.name || "",
					logo_url:
						typeof profile.logo_url === "string" ? profile.logo_url : "",
					storeCode:
						typeof profile.storeCode === "string" ? profile.storeCode : "",
				});
			}
			const records = (catalog?.records ?? []) as Record<string, unknown>[];
			setProducts(records.map(normalizeProduct).filter((p) => p.id && p.name));
		})();
		return () => {
			cancelled = true;
		};
	}, [sellerId]);

	if (!seller) return null;

	// The panel already holds the seller's public profile, so the CTA can point
	// straight at the canonical storefront rather than eat a 308 (#365).
	const shopHref = sellerPath(seller.storeCode, sellerId);

	return (
		<aside className="chat-context" aria-label="賣家資訊">
			<div className="chat-context__seller">
				<ChatPeerAvatar
					src={seller.logo_url}
					alt={seller.name}
					className="chat-context__avatar"
					fallback="empty"
				/>
				<strong className="chat-context__name">{seller.name || "賣家"}</strong>
				<Link
					href={shopHref}
					// `.chat-context__cta` is a filled pill with a
					// `--hkui-primary-foreground` label (0-1-0); without this marker
					// `[data-theme="dark"] body a:not([data-hkui-button])` (0-2-2)
					// repaints it olive `--color-ink-200`.
					data-hkui-button=""
					className="chat-context__cta"
				>
					前往賣場
				</Link>
			</div>
			{products.length > 0 ? (
				<section className="chat-context__section" aria-label="店內商品">
					<h3>店內商品</h3>
					<ul className="chat-context__products">
						{products.map((p) => (
							<li key={p.id}>
								<Link
									href={`/products/${encodeURIComponent(p.id)}`}
									className="chat-context__product"
								>
									<AppImage
										src={resolveCdnUrl(p.imageUrl)}
										alt=""
										width={52}
										height={52}
										sizes="52px"
										loading="lazy"
										className="chat-context__thumb"
									/>
									<span className="chat-context__product-meta">
										<span className="chat-context__product-name">{p.name}</span>
										{p.price ? (
											<span className="chat-context__product-price">
												NT$ {p.price}
											</span>
										) : null}
									</span>
								</Link>
							</li>
						))}
					</ul>
				</section>
			) : null}
		</aside>
	);
}
