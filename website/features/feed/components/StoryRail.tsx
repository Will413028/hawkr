"use client";

import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import { useDragScroll } from "@/lib/hooks/useDragScroll";
import { sellerPath } from "@/lib/seo/urls";

export interface StoryItem {
	id: number | string;
	/**
	 * Canonical storefront segment (`store_code`) when the payload that built
	 * this item carried one — see `sellerPath` (#365). Producers reading a
	 * recommended-seller payload take it from snake_case `store_code`; the rail
	 * itself only ever sees this camelCase property. Omitted producers keep the
	 * id path, which middleware answers with a 308.
	 */
	storeCode?: string;
	name: string;
	avatar: string;
	hasUnread?: boolean;
	city?: string;
	district?: string;
	latitude?: number;
	longitude?: number;
}

export interface StoryTab {
	key: string;
	label: string;
}

interface StoryRailProps {
	items: StoryItem[];
	/**
	 * Optional tabs rendered next to the "今日活躍商家" heading. The
	 * component is purely presentational — actual filter wiring stays
	 * with the parent page.
	 */
	tabs?: StoryTab[];
	activeTab?: string;
	onTabChange?: (key: string) => void;
	isLoading?: boolean;
	emptyMessage?: React.ReactNode;
	/**
	 * Optional click hook (used by carousel/modal flows). When provided,
	 * the rail renders <button>s; otherwise it renders <Link>s straight
	 * to the seller's storefront so the avatar is always navigable.
	 */
	onSelect?: (item: StoryItem) => void;
	/** Market mobile groups these tabs inside its filter drawer. */
	hideTabsOnMobile?: boolean;
	/**
	 * Reserve the region sub-line's height even before any item carries a
	 * region. The feed rail first renders server post-derived stories (no
	 * city/district) and then swaps to client seller-derived stories that DO
	 * carry regions; without this, the sub-line appears on the swap and grows
	 * every item by 16px, pushing the post grid down. Since #338 keeps the app
	 * tree visible under the splash, that shift is now recorded by the Layout
	 * Instability API and can leak into the visible fade — so the rail must be
	 * height-stable from first paint, not merely once its data arrives.
	 */
	reserveRegion?: boolean;
}

const COLLAPSE_KEY = "feed-stories-collapsed";
const COLLAPSE_MEDIA = "(max-width: 768px)";

/**
 * 圓形頭像的「奶油裱框」：3px `surface-card` 內框把商家 logo 從頁面底色
 * 切開（很多 logo 是白底，沒有這層會糊在奶油背景上），外面再壓一條
 * `border-soft` 髮絲線收邊。常態刻意不上彩色環——彩色只留給 `hasUnread`，
 * 環才有語意；全部都亮的環等於沒有環。
 */
const AVATAR_FRAME =
	"relative block size-[72px] shrink-0 overflow-hidden rounded-full border-[3px] border-hk-surface-card bg-hk-surface-muted shadow-hk-sm transition-[box-shadow] duration-200 ease-out max-[768px]:size-[62px]";
const AVATAR_FRAME_DEFAULT =
	"ring-1 ring-hk-border-soft group-hover:ring-hk-primary-border";
const AVATAR_FRAME_UNREAD = "ring-[1.5px] ring-hk-brand-secondary";

/** 店名下方的第二行 — 有地區才有值，補上被 ellipsis 截斷的店名資訊。 */
function regionLabel(item: StoryItem): string {
	return [item.city, item.district]
		.map((part) => part?.trim())
		.filter(Boolean)
		.join(" · ");
}

export default function StoryRail({
	items,
	tabs,
	activeTab,
	onTabChange,
	isLoading = false,
	emptyMessage,
	onSelect,
	hideTabsOnMobile = false,
	reserveRegion = false,
}: StoryRailProps) {
	// Collapse is a compact-screen affordance. If a mobile collapse state is
	// restored on desktop, the rail becomes hidden while the toggle is hidden too.
	const [collapsed, setCollapsed] = React.useState(false);
	const [canCollapse, setCanCollapse] = React.useState(false);
	const listId = React.useId();
	const listRef = React.useRef<HTMLUListElement | null>(null);

	useDragScroll(listRef);

	React.useEffect(() => {
		const query = window.matchMedia(COLLAPSE_MEDIA);
		const syncCollapseMode = () => {
			const compact = query.matches;
			setCanCollapse(compact);
			if (!compact) {
				setCollapsed(false);
				return;
			}
			try {
				setCollapsed(localStorage.getItem(COLLAPSE_KEY) === "1");
			} catch {
				setCollapsed(false);
			}
		};

		syncCollapseMode();
		query.addEventListener?.("change", syncCollapseMode);
		return () => query.removeEventListener?.("change", syncCollapseMode);
	}, []);

	const toggle = React.useCallback(() => {
		if (!canCollapse) return;
		setCollapsed((prev) => {
			const next = !prev;
			try {
				localStorage.setItem(COLLAPSE_KEY, next ? "1" : "0");
			} catch {}
			return next;
		});
	}, [canCollapse]);

	const isCollapsed = canCollapse && collapsed;

	// 只要有任何一家帶得出地區，整排都保留第二行的高度，否則同一排會出現
	// 兩行 / 一行混排、名稱基線參差。`reserveRegion` 讓會被 client 補上地區的
	// rail（feed 推薦商家）從 server 首屏就先保留這行高度，避免資料換手時整排
	// 長高 16px 把下方貼文網格往下推（見 reserveRegion prop 註解）。
	const showRegion = React.useMemo(
		() => reserveRegion || items.some((item) => regionLabel(item).length > 0),
		[items, reserveRegion],
	);

	if (!items.length && !tabs?.length && !isLoading && !emptyMessage)
		return null;
	return (
		<section
			className={
				isCollapsed ? "mb-6 max-[768px]:mb-2" : "mb-6 max-[768px]:mb-4"
			}
			aria-label="推薦商家"
			aria-busy={isLoading}
		>
			<header
				className={`flex flex-wrap items-center justify-between gap-3 ${
					isCollapsed ? "mb-0" : "mb-3"
				}`}
			>
				<div className="inline-flex min-w-0 items-center gap-1">
					<h2 className="m-0 text-[1.125rem] font-bold leading-tight tracking-tight text-hk-heading">
						推薦商家
					</h2>
					<button
						type="button"
						className="-m-1 hidden rounded-full p-1 leading-none text-hk-muted transition-colors hover:text-hk-heading max-[768px]:inline-flex max-[768px]:items-center max-[768px]:justify-center"
						onClick={toggle}
						aria-expanded={!isCollapsed}
						aria-controls={listId}
						aria-label={isCollapsed ? "展開推薦商家" : "收起推薦商家"}
					>
						<svg
							viewBox="0 0 20 20"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
							aria-hidden="true"
							className={`size-5 transition-transform duration-200 ease-out ${
								isCollapsed ? "-rotate-90" : ""
							}`}
						>
							<polyline points="6 8 10 12 14 8" />
						</svg>
					</button>
				</div>
				{tabs && tabs.length > 0 ? (
					<div
						className={`inline-flex items-center gap-1 rounded-hk-pill border border-hk-border-soft bg-hk-surface-card p-1 ${
							hideTabsOnMobile ? "market-story-tabs" : ""
						}`}
						role="tablist"
						aria-label="動態篩選"
					>
						{tabs.map((t) => {
							const active = t.key === activeTab;
							return (
								<button
									key={t.key}
									type="button"
									role="tab"
									aria-selected={active}
									className={`whitespace-nowrap rounded-hk-pill px-3.5 py-1.5 text-xs transition-colors duration-150 ${
										active
											? "bg-hk-primary-subtle font-bold text-hk-heading"
											: "font-semibold text-hk-muted hover:text-hk-heading"
									}`}
									onClick={() => onTabChange?.(t.key)}
								>
									{t.label}
								</button>
							);
						})}
					</div>
				) : (
					<Link
						href="/market/products"
						className="text-xs font-semibold text-hk-muted no-underline transition-colors hover:text-hk-accent"
					>
						看全部 →
					</Link>
				)}
			</header>
			{items.length > 0 ? (
				/* Instagram-style horizontal scroller — single row, swipe sideways
				 * through merchants. The list breaks out of the page's 24px
				 * (desktop) / 14px (mobile) gutter via negative side margins so
				 * cards scroll edge-to-edge; the matching padding keeps the first
				 * card visually flush with the content column. */
				<ul
					id={listId}
					ref={listRef}
					className="m-0 -mx-6 flex cursor-grab list-none flex-nowrap gap-3 overflow-x-auto overflow-y-hidden px-6 pt-1 pb-2 select-none [-webkit-overflow-scrolling:touch] [overscroll-behavior-x:contain] [scroll-behavior:auto] [scroll-padding-inline:24px] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden [&[hidden]]:hidden [&_a]:[-webkit-user-drag:none] [&_img]:[-webkit-user-drag:none] max-[768px]:-mx-3.5 max-[768px]:px-3.5 max-[768px]:[scroll-padding-inline:14px]"
					hidden={isCollapsed}
				>
					{items.map((s) => {
						const region = regionLabel(s);
						const frameClass = `${AVATAR_FRAME} ${
							s.hasUnread ? AVATAR_FRAME_UNREAD : AVATAR_FRAME_DEFAULT
						}`;
						/**
						 * Per the 2026-05 buyer-spec: the small store-badge icon at
						 * the avatar's bottom-right is removed, and clicking the
						 * avatar takes the buyer straight to the storefront
						 * (`/sellers/:id`), not the seller's posts list. The segment is
						 * the seller's store_code whenever the producer carried one, so
						 * the tap lands on the canonical URL instead of paying for the
						 * middleware 308 (#365).
						 */
						const inner = (
							<>
								<span className={frameClass}>
									<Avatar
										src={s.avatar}
										name={s.name}
										alt=""
										entity="seller"
										fallback="empty"
										loading="eager"
										className="block size-full object-cover"
									/>
								</span>
								<span className="mt-2.5 flex w-full flex-col items-center gap-0.5">
									<span className="w-full truncate text-center text-[0.8125rem] font-semibold leading-tight text-hk-heading max-[768px]:text-xs">
										{s.name}
									</span>
									{showRegion ? (
										<span className="block min-h-[14px] w-full truncate text-center text-[0.6875rem] leading-tight text-hk-muted">
											{region}
										</span>
									) : null}
								</span>
							</>
						);
						const btnClass =
							"group flex w-full cursor-pointer flex-col items-center border-none bg-transparent px-0 py-1 font-sans text-inherit no-underline transition-transform duration-200 ease-out hover:-translate-y-0.5 active:scale-[0.97]";
						return (
							<li
								key={s.id}
								className="flex flex-[0_0_96px] justify-center max-[768px]:flex-[0_0_84px]"
							>
								{onSelect ? (
									<button
										type="button"
										className={btnClass}
										onClick={() => onSelect(s)}
										aria-label={`查看 ${s.name} 的店家頁`}
									>
										{inner}
									</button>
								) : (
									<Link
										href={sellerPath(s.storeCode, s.id)}
										className={btnClass}
										aria-label={`進入 ${s.name} 的店家頁`}
									>
										{inner}
									</Link>
								)}
							</li>
						);
					})}
				</ul>
			) : (
				<div
					id={listId}
					className="flex min-h-[72px] items-center rounded-hk-xl border border-dashed border-hk-border-soft bg-hk-surface-card px-[18px] py-3.5 text-sm leading-relaxed text-hk-muted [&[hidden]]:hidden"
					role="status"
					hidden={collapsed}
				>
					{isLoading ? "正在整理商家..." : emptyMessage}
				</div>
			)}
		</section>
	);
}
