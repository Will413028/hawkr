import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import SellerChatButton from "@/features/seller/components/SellerChatButton";
import type { SellerStats } from "@/lib/api";
import { sellerPath } from "@/lib/seo/urls";

interface ProductSellerIntroProps {
	sellerId: string;
	seller?: unknown;
	/** 評價統計（avgRating/totalCount）— 不含訂閱數，別從這裡撈 followers。 */
	sellerStats?: unknown;
	/** `/public/sellers/{id}/stats` 的公開聚合 — 訂閱數（followers）唯一來源。 */
	followerStats?: SellerStats | null;
	productSeller?: unknown;
	fallbackSeller?: unknown;
	/**
	 * Render the rating / subscriber numbers as a skeleton. Used as the
	 * <Suspense> fallback while the seller + stats fetches stream in — the
	 * identity (avatar / name / location) still renders from the embedded
	 * `productSeller`, so only the numbers shimmer (no zero-flash).
	 */
	pending?: boolean;
	/** Read-only seller storefront preview: show identity without navigation/chat. */
	previewMode?: boolean;
}

const StarIcon = ({ size = 14 }: { size?: number }) => (
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		fill="currentColor"
		aria-hidden="true"
	>
		<path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
	</svg>
);

const UsersIcon = ({ size = 15 }: { size?: number }) => (
	<svg
		width={size}
		height={size}
		viewBox="0 0 24 24"
		fill="currentColor"
		aria-hidden="true"
	>
		<path d="M16 11c1.66 0 2.99-1.34 2.99-3S17.66 5 16 5c-1.66 0-3 1.34-3 3s1.34 3 3 3zm-8 0c1.66 0 2.99-1.34 2.99-3S9.66 5 8 5C6.34 5 5 6.34 5 8s1.34 3 3 3zm0 2c-2.33 0-7 1.17-7 3.5V19h14v-2.5c0-2.33-4.67-3.5-7-3.5zm8 0c-.29 0-.62.02-.97.05 1.16.84 1.97 1.97 1.97 3.45V19h6v-2.5c0-2.33-4.67-3.5-7-3.5z" />
	</svg>
);

function asRecord(value: unknown): Record<string, unknown> {
	return value && typeof value === "object"
		? (value as Record<string, unknown>)
		: {};
}

function textOrEmpty(value: unknown): string {
	return typeof value === "string" ? value.trim() : "";
}

function firstText(...values: unknown[]): string {
	for (const value of values) {
		const text = textOrEmpty(value);
		if (text) return text;
	}
	return "";
}

function numberOrZero(value: unknown): number {
	const n = typeof value === "number" ? value : Number(value);
	return Number.isFinite(n) ? n : 0;
}

function SellerIdentity({
	location,
	name,
	avatarSrc,
	rating,
	totalReviews,
	subscribers,
	pending,
}: {
	location: string;
	name: string;
	avatarSrc: string;
	rating: number;
	totalReviews: number;
	subscribers: number;
	pending: boolean;
}) {
	return (
		<>
			<div className="min-w-0">
				{location ? (
					<span className="block mb-0.5 overflow-hidden text-[#677064] text-xs leading-[1.4] text-ellipsis whitespace-nowrap">
						{location}
					</span>
				) : null}
				<strong className="block overflow-hidden text-hk-ink-950 text-base font-extrabold leading-[1.35] text-ellipsis whitespace-nowrap max-md:text-[0.9375rem] group-hover:underline group-hover:decoration-[1.5px] group-hover:underline-offset-4 transition-[text-decoration-color,transform,box-shadow] duration-[120ms] ease-[var(--hkui-ease,ease)]">
					{name}
				</strong>
				<div className="flex flex-wrap gap-x-2 gap-y-1 justify-end mt-1 text-[#697064] text-xs leading-[1.4]">
					{pending ? (
						<span
							className="inline-flex h-[1lh] w-24 max-w-full items-center"
							role="status"
						>
							<span className="sr-only">商家統計載入中</span>
							<span
								aria-hidden="true"
								className="h-[1em] w-full rounded bg-hk-border-soft/60 animate-pulse motion-reduce:animate-none"
							/>
						</span>
					) : null}
					{!pending && rating > 0 ? (
						<span className="inline-flex items-center gap-0.75">
							<span className="text-[#d1ae41]">
								<StarIcon />
							</span>
							<strong className="text-hk-ink-950 font-extrabold">
								{rating.toFixed(1)}
							</strong>
							<span>({totalReviews})</span>
						</span>
					) : null}
					{!pending ? (
						<span className="inline-flex items-center gap-0.75">
							<span className="text-hk-primary">
								<UsersIcon />
							</span>
							<strong className="text-hk-ink-950 font-extrabold">
								{subscribers}
							</strong>
							<span>位訂閱</span>
						</span>
					) : null}
				</div>
			</div>
			<div className="grid flex-shrink-0 w-16 h-16 overflow-hidden place-items-center bg-hk-cream border-2 border-hk-brand-secondary/40 rounded-full shadow-[0_10px_24px_rgb(var(--color-ink-800-rgb)/0.12)] max-md:w-[58px] max-md:h-[58px] transition-[text-decoration-color,transform,box-shadow] duration-[120ms] ease-[var(--hkui-ease,ease)] group-hover:-translate-y-0.5 group-hover:shadow-[0_6px_18px_rgb(var(--color-ink-950-rgb)/0.18)] dark:group-hover:shadow-[0_6px_18px_rgba(0,0,0,0.55)]">
				<Avatar
					src={avatarSrc}
					name={name || "商"}
					alt={name}
					entity="seller"
					loading="eager"
					fallback="empty"
				/>
			</div>
		</>
	);
}

export default function ProductSellerIntro({
	sellerId,
	seller,
	sellerStats,
	followerStats,
	productSeller,
	fallbackSeller,
	pending = false,
	previewMode = false,
}: ProductSellerIntroProps) {
	const sellerRecord = asRecord(seller);
	const statsRecord = asRecord(sellerStats);
	const productSellerRecord = asRecord(productSeller);
	const fallbackRecord = asRecord(fallbackSeller);
	const cityDistrict = [sellerRecord.city, sellerRecord.district]
		.map(textOrEmpty)
		.filter(Boolean)
		.join(" ");

	// The storefront link is built from the seller's storeCode, not from
	// `sellerId` (the store_id every product payload carries, and what the
	// seller/stats reads are keyed by) — see `sellerPath`.
	//
	// 三個來源**同名**（backend#2235 之後全 camelCase）：`/public/sellers/{id}`
	// 經 fetchSeller 正規化成 `storeCode`（lib/seller/repo.ts:112），商品 wire
	// 內嵌的賣家區塊本來就是 `storeCode`。這裡以前兩種大小寫都讀，註解還寫著
	// 「detail 是 snake_case」——那個前提已不成立。
	const storefrontHref = sellerPath(
		firstText(
			sellerRecord.storeCode,
			productSellerRecord.storeCode,
			fallbackRecord.storeCode,
		),
		sellerId,
	);

	// B-238: seller identity uses the canonical display URL. fetchSeller 只吐
	// `logoUrl`（非顯示用的 logoKey/logo/avatar 已在那裡被刪掉），商品 wire 的
	// 內嵌賣家區塊也是 `logoUrl`。
	const avatarSrc = firstText(
		sellerRecord.logoUrl,
		productSellerRecord.logoUrl,
		fallbackRecord.logoUrl,
	);
	const name =
		firstText(
			sellerRecord.name,
			sellerRecord.storeName,
			sellerRecord.sellerName,
			productSellerRecord.name,
			fallbackRecord.name,
		) || "Hawkr 商家";
	const location = firstText(
		sellerRecord.location,
		cityDistrict,
		sellerRecord.address,
		productSellerRecord.address,
		fallbackRecord.location,
	);
	const rating = numberOrZero(
		statsRecord.avgRating || sellerRecord.rating || fallbackRecord.rating,
	);
	const totalReviews = numberOrZero(
		statsRecord.totalCount ||
			sellerRecord.totalReviews ||
			sellerRecord.reviewCount ||
			fallbackRecord.totalReviews,
	);
	const subscribers = numberOrZero(followerStats?.followers);

	return (
		<aside
			className="flex flex-col items-end gap-3 justify-self-end w-full max-w-[230px] text-right max-md:max-w-none max-md:pt-3.5 max-md:border-t max-md:border-hk-brand-secondary/30"
			aria-label="商家資訊"
		>
			{/* The buyer PDP links identity to the storefront and keeps Chat
			 * separate. Seller preview renders the same identity as plain content
			 * and omits Chat so this public-data view cannot initiate a write flow. */}
			{previewMode ? (
				<div className="flex w-full items-center justify-end gap-3 rounded-[14px] text-inherit">
					<SellerIdentity
						location={location}
						name={name}
						avatarSrc={avatarSrc}
						rating={rating}
						totalReviews={totalReviews}
						subscribers={subscribers}
						pending={pending}
					/>
				</div>
			) : (
				<Link
					href={storefrontHref}
					className="group flex gap-3 items-center justify-end w-full no-underline text-inherit cursor-pointer rounded-[14px] [-webkit-tap-highlight-color:transparent] hover:bg-transparent hover:transform-none focus-visible:bg-transparent focus-visible:transform-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-hk-ink-300 focus-visible:outline-offset-4"
					aria-label={`查看 ${name} 的店家頁`}
				>
					<SellerIdentity
						location={location}
						name={name}
						avatarSrc={avatarSrc}
						rating={rating}
						totalReviews={totalReviews}
						subscribers={subscribers}
						pending={pending}
					/>
				</Link>
			)}
			{previewMode ? null : (
				<SellerChatButton
					sellerId={sellerId}
					className="inline-flex items-center justify-center gap-2 min-w-[142px] min-h-[44px] px-5 text-[#fffdf7] dark:text-[#2d2010] text-[0.9375rem] font-extrabold bg-hk-primary border-0 rounded-full cursor-pointer shadow-[0_10px_22px_rgb(var(--color-ink-600-rgb)/0.2)] transition-[background,box-shadow,transform] duration-200 ease-in-out hover:bg-hk-primary hover:shadow-[0_12px_26px_rgb(var(--color-ink-600-rgb)/0.26)] hover:-translate-y-0.5 max-md:w-full max-md:min-h-[48px] max-md:text-base"
				/>
			)}
		</aside>
	);
}
