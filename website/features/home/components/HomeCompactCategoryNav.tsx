"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import FoodCategoryIcon from "@/components/shared/FoodCategoryIcon";
import Link from "@/components/shared/Link";
import type { HawkrAgentCategoryId } from "@/lib/animation/hawkr-agent-season";
import { useDragScroll } from "@/lib/hooks/useDragScroll";

type CompactCategoryItem = {
	label: string;
	href: string;
	highlight?: boolean;
	separatorBefore?: boolean;
	/** Registry glyph inside a circular frame. */
	icon?: AppIconName;
	/** Tailwind class for the icon's circular frame — lets each icon chip
	 * carry a distinct accent ring. */
	frameClass?: string;
	/** One of the nine food categories; never used by promo/cert chips. */
	categoryId?: HawkrAgentCategoryId;
};

const compactCategories: CompactCategoryItem[] = [
	{
		label: "本週新品",
		href: "/market/products?sort=最新上架",
		icon: "sparkle",
		frameClass: "home-chip-frame--gold",
	},
	{
		label: "特惠商品",
		href: "/market/products",
		icon: "discount",
		frameClass: "home-chip-frame--primary",
	},
	{
		label: "蔬菜",
		href: "/market/products?categories=蔬菜",
		categoryId: "vegetables",
		separatorBefore: true,
	},
	{
		label: "水果",
		href: "/market/products?categories=水果",
		categoryId: "fruits",
	},
	{
		label: "海鮮",
		href: "/market/products?categories=海鮮",
		categoryId: "seafood",
	},
	{
		label: "肉類",
		href: "/market/products?categories=肉類",
		categoryId: "meat",
	},
	{
		label: "蛋奶",
		href: "/market/products?categories=蛋奶",
		categoryId: "egg-dairy",
	},
	{
		label: "加工食品",
		href: "/market/products?categories=加工食品",
		categoryId: "processed",
	},
	{
		label: "香料",
		href: "/market/products?categories=香料",
		categoryId: "spices",
	},
	{
		label: "穀物",
		href: "/market/products?categories=穀物",
		categoryId: "grains",
	},
	{
		label: "花卉",
		href: "/market/products?categories=花卉",
		categoryId: "flowers",
	},
	// Right-side cert block — separated visually from the categories so
	// it reads as a "trust filter" group rather than another taxonomy.
	// Routed via ?certification_types=<CertificationType value> so it filters
	// on product certifications (有機=3701 / 產銷履歷=3702), matching what
	// sellers store in certifications[].name.
	{
		label: "有機",
		href: "/market/products?certification_types=3701",
		icon: "leaf",
		frameClass: "home-chip-frame--primary",
		highlight: true,
		separatorBefore: true,
	},
	{
		label: "產銷履歷",
		href: "/market/products?certification_types=3702",
		icon: "verified",
		frameClass: "home-chip-frame--primary",
		highlight: true,
	},
];

export default function HomeCompactCategoryNav() {
	const railRef = React.useRef<HTMLDivElement>(null);
	// Mouse drag-to-scroll. Native overflow-x already gives touch inertia;
	// this adds the missing pointer-drag so desktop / narrow-viewport users
	// can reach the categories that overflow off-screen. A drag past ~6px
	// suppresses the trailing click so dragging never fires a chip's link.
	useDragScroll(railRef);

	return (
		<section
			aria-label="分類捷徑"
			className="home-fixed-section relative z-[2]"
		>
			<div
				ref={railRef}
				className="home-category-rail home-section-surface flex cursor-grab select-none items-center gap-1 overflow-x-auto rounded-[18px] border border-white/60 bg-hk-surface-card/80 p-3 shadow-hk-card backdrop-blur-md [overscroll-behavior-x:contain] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&_img]:[-webkit-user-drag:none]"
			>
				{compactCategories.map((item) => (
					<CompactCategoryChip key={item.label} item={item} />
				))}
			</div>
		</section>
	);
}

const CHIP_CLASS =
	"home-chip group flex shrink-0 flex-col items-center gap-1 rounded-hk-lg px-3 py-1.5 text-inherit no-underline transition-[transform,background] duration-200 hover:-translate-y-0.5 hover:bg-hk-primary-subtle focus-visible:-translate-y-0.5 focus-visible:bg-hk-primary-subtle focus-visible:outline-none";

interface CompactCategoryChipProps {
	item: CompactCategoryItem;
}

function CompactCategoryChip({ item }: CompactCategoryChipProps) {
	const body = (
		<>
			{/* `home-chip-icon` is the breathing/pop animation target — keyframes
			    live in styles/features/home.css so they cascade from one source. */}
			<span className="home-chip-icon grid size-9 place-items-center overflow-hidden rounded-hk-lg text-sm font-black text-hk-primary">
				{item.categoryId ? (
					<FoodCategoryIcon categoryId={item.categoryId} />
				) : item.icon ? (
					<span
						className={`home-chip-frame grid size-8 place-items-center rounded-full ${
							item.frameClass ?? ""
						}`}
					>
						<AppIcon name={item.icon} size={22} />
					</span>
				) : null}
			</span>
			<span
				className={
					item.highlight
						? "whitespace-nowrap text-xs font-black text-hk-primary"
						: "whitespace-nowrap text-xs font-bold text-hk-heading"
				}
			>
				{item.label}
			</span>
		</>
	);

	return (
		<div className="flex shrink-0 items-center gap-1">
			{item.separatorBefore ? (
				<span className="mx-1 h-9 w-px bg-hk-home-ink/10" aria-hidden="true" />
			) : null}
			<Link
				href={item.href}
				className={CHIP_CLASS}
				data-seasonal-category={item.categoryId}
			>
				{body}
			</Link>
		</div>
	);
}
