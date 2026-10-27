"use client";

import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import SellerFollowChip from "@/features/seller/components/SellerFollowChip";
import { resolveCdnUrl } from "@/lib/media/cdn";

export interface MarketSellerItem {
	id: string;
	name: string;
	location: string;
	image: string;
	logo_url: string;
	desc: string;
	rating: number;
	followers: number;
	products: number;
	posts: number;
	href: string;
	/** Real-name verification ("商家認證") — shows the brand-tinted
	 *  check-badge icon next to the seller name when true. */
	certified?: boolean;
	/** 賣家自填獎項（`/public/sellers/{id}` 的 `prize` free-text）；
	 *  空字串 = 未填，卡片不顯示獎章 pill。 */
	prize?: string;
}

interface Props {
	seller: MarketSellerItem;
	/** "list" stretches to a 260px cover; "grid" uses 16/9 aspect. */
	variant?: "list" | "grid";
}

function LocationPin() {
	return (
		<svg
			width="12"
			height="12"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2.4"
			strokeLinecap="round"
			strokeLinejoin="round"
			aria-hidden="true"
			focusable="false"
		>
			<path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 1 1 16 0Z" />
			<circle cx="12" cy="10" r="3" />
		</svg>
	);
}

/**
 * Reusable seller card. The list variant is used on /market/sellers;
 * the grid variant is used on the home page's 推薦商家 row. The DOM is
 * identical so styles in `styles/features/market.css` (the
 * `.market-seller-card` family) drive the visual, including responsive
 * tweaks.
 */
export default function MarketSellerCard({ seller, variant = "grid" }: Props) {
	return (
		<article className={`market-seller-card market-seller-card--${variant}`}>
			<Link
				href={seller.href}
				className="market-seller-card__cover-link"
				aria-label={`查看商家 ${seller.name}`}
			>
				<AppImage
					className="market-seller-card__cover"
					src={resolveCdnUrl(seller.image)}
					alt={seller.name}
					width={400}
					height={250}
					loading="lazy"
					sizes={
						variant === "list"
							? "(max-width: 768px) 100vw, 420px"
							: "(max-width: 768px) 100vw, 360px"
					}
				/>
				{seller.location ? (
					<span className="market-seller-card__location-chip">
						<LocationPin />
						{seller.location}
					</span>
				) : null}
			</Link>
			<div className="market-seller-card__body">
				<Avatar
					src={seller.logo_url}
					name={seller.name}
					alt={seller.name}
					entity="seller"
					className="market-seller-card__avatar"
					fallback="empty"
					loading="lazy"
				/>
				<Link href={seller.href} className="market-seller-card__name-link">
					<h2 className="market-seller-card__name">
						<span className="market-seller-card__name-text">{seller.name}</span>
						{seller.certified ? (
							<AppIcon
								className="market-seller-card__verified"
								name="shield"
								size={18}
								title="已通過商家認證"
							/>
						) : null}
					</h2>
				</Link>
				<ul className="market-seller-card__stats" aria-label="商家統計">
					<li>
						<span>商品</span>
						<b>{seller.products}</b>
					</li>
					<li>
						<span>動態</span>
						<b>{seller.posts}</b>
					</li>
					{seller.rating > 0 ? (
						<li>
							<span>評分</span>
							<b>{seller.rating.toFixed(1)}</b>
						</li>
					) : null}
					<li>
						<span>追蹤</span>
						<b>{seller.followers.toLocaleString()}</b>
					</li>
				</ul>
				<div className="market-seller-card__actions">
					<SellerFollowChip
						sellerId={seller.id}
						className="market-seller-card__follow"
					/>
					<Link href={seller.href} className="market-seller-card__view">
						查看商家
					</Link>
				</div>
			</div>
		</article>
	);
}
