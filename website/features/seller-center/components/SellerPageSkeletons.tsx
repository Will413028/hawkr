/**
 * Per-page skeleton rows for the seller workspace. Each component mirrors
 * the column semantics and responsive geometry of its real-data sibling.
 * Content-dependent text wrapping, extra badges and row counts remain data-driven.
 *
 * Naming: <{Page}TableSkeletonRows> — drop straight inside the page's
 * `<tbody>`. They render React fragments of `<tr>` so the page keeps its
 * own `<table>` chrome (which carries column widths from the `<thead>`).
 *
 * For non-table surfaces (posts list, coupon list, dashboard panels):
 * each gets a self-contained skeleton that owns its outer layout.
 *
 * If you need a generic-shape table, use <SellerTableSkeletonRows> from
 * `./SellerSkeletons.tsx` instead — that one fills every cell with a
 * single bar and is fine when the column semantics don't carry visual
 * weight.
 */
import * as React from "react";
import "@/styles/components/seller-mobile-disclosure.css";
import { SellerTableSkeletonRows } from "./SellerSkeletons";

function useRowKeys(count: number, prefix: string): string[] {
	return React.useMemo(
		() => Array.from({ length: count }, (_, i) => `${prefix}-${i}`),
		[count, prefix],
	);
}

/** Shared bars stay inside the real cells so table widths, phone labels and
 * sticky/hidden columns have the same layout during the initial request. */
const bone = "seller-skeleton__bone";

type CellProps = {
	label?: string;
	align?: "left" | "center" | "right";
	className?: string;
};

function cellClass({ align, className }: CellProps) {
	return [
		align === "center" ? "is-center" : align === "right" ? "is-right" : "",
		className,
	]
		.filter(Boolean)
		.join(" ");
}

function CellCheck({ className }: CellProps) {
	return (
		<td className={className} data-label="選取">
			<span className={`${bone} seller-skeleton__check`} />
		</td>
	);
}

function CellBar({ label, align, className }: CellProps) {
	return (
		<td className={cellClass({ align })} data-label={label}>
			<span className={`${bone} seller-skeleton__line ${className ?? ""}`} />
		</td>
	);
}

function CellPriceStack() {
	return (
		<td data-label="價格">
			<div className="seller-products-table__price flex flex-col items-start">
				<span className={`${bone} seller-skeleton__line`} />
				<span
					className={`${bone} seller-skeleton__line seller-skeleton__line--short mt-1`}
				/>
			</div>
		</td>
	);
}

function CellPill({ label, align = "center" }: CellProps) {
	return (
		<td className={cellClass({ align })} data-label={label}>
			<span className={`${bone} seller-skeleton__pill`} />
		</td>
	);
}

function CellButton({
	label = "操作",
	align = "center",
	className,
}: CellProps) {
	return (
		<td className={cellClass({ align })} data-label={label}>
			<span className={`${bone} seller-skeleton__button ${className ?? ""}`} />
		</td>
	);
}

function CellAvatarStack({
	label,
	className = "seller-orders__buyer",
}: CellProps) {
	return (
		<td data-label={label}>
			<div className={className}>
				<span className={`${bone} seller-skeleton__avatar`} />
				<div className="min-w-0 flex-1">
					<span className={`${bone} seller-skeleton__line`} />
					<span
						className={`${bone} seller-skeleton__line seller-skeleton__line--short mt-1.5`}
					/>
				</div>
			</div>
		</td>
	);
}

function CellThumbBar({ product = false }: { product?: boolean }) {
	return (
		<td data-label="商品">
			<div
				className={
					product
						? "seller-products-table__product"
						: "seller-orders__product-cell"
				}
			>
				<span
					className={
						product
							? "seller-products-table__thumb"
							: "seller-orders__product-thumb"
					}
				>
					<span className={`${bone} block size-full`} />
				</span>
				<span className="min-w-0 flex-1">
					<span className={`${bone} seller-skeleton__line`} />
				</span>
			</div>
		</td>
	);
}

interface RowsProps {
	rows?: number;
}

/** Columns and checkbox visibility match the orders table. */
export function SellerOrdersSkeletonRows({ rows = 6 }: RowsProps) {
	return (
		<>
			{useRowKeys(rows, "skel-order").map((key) => (
				<tr key={key} className="seller-skeleton__row">
					<CellCheck className="seller-orders__col-select" />
					<CellBar label="訂單編號" />
					<CellAvatarStack label="顧客資訊" />
					<CellThumbBar />
					<CellBar
						label="數量"
						align="center"
						className="seller-skeleton__line--short"
					/>
					<CellBar label="金額" align="right" />
					<CellBar label="配送方式" align="center" />
					<CellPill label="訂單狀態" />
					<CellButton />
				</tr>
			))}
		</>
	);
}

/** Delegated lists omit social/date columns and may omit the bulk checkbox. */
export function SellerProductsSkeletonRows({
	rows = 6,
	delegated = false,
	canPublish = true,
}: RowsProps & { delegated?: boolean; canPublish?: boolean }) {
	return (
		<>
			{useRowKeys(rows, "skel-product").map((key) => (
				<tr
					key={key}
					className="seller-skeleton__row seller-products-table__row"
				>
					{canPublish && <CellCheck className="seller-products-table__check" />}
					{delegated && <CellBar label="商品 ID" />}
					<CellThumbBar product />
					{!delegated && <CellBar label="評分" />}
					<CellBar label="分類" />
					<CellPriceStack />
					<CellBar label="庫存" className="seller-skeleton__line--short" />
					{!delegated && (
						<CellBar label="收藏" className="seller-skeleton__line--short" />
					)}
					<CellBar label="銷量" className="seller-skeleton__line--short" />
					{!delegated && <CellBar label="上架時間" />}
					<CellPill label="狀態" align="left" />
					<CellButton
						align="left"
						className="seller-skeleton__button--product"
					/>
				</tr>
			))}
		</>
	);
}

export function SellerInventorySkeletonRows({ rows = 6 }: RowsProps) {
	return (
		<>
			{useRowKeys(rows, "skel-inv").map((key) => (
				<tr key={key} className="seller-skeleton__row">
					<CellBar label="商品名稱" />
					<CellBar
						label="當前庫存"
						align="center"
						className="seller-skeleton__line--short"
					/>
					<CellBar label="建議安全庫存" align="center" />
					<CellBar label="週銷趨勢" align="center" />
					<CellBar label="預估售罄" align="center" />
					<CellPill label="狀態" />
					<CellBar label="庫存價值" align="right" />
					<CellButton />
				</tr>
			))}
		</>
	);
}

export function SellerCustomersSkeletonRows({ rows = 5 }: RowsProps) {
	return (
		<>
			{useRowKeys(rows, "skel-cust").map((key) => (
				<tr key={key} className="seller-skeleton__row">
					<CellAvatarStack label="客戶" className="seller-customers__name" />
					<CellBar label="標籤" />
					<CellBar
						label="訂單"
						align="center"
						className="seller-skeleton__line--short"
					/>
					<CellBar label="累計消費" align="right" />
					<CellBar label="最近購買" align="center" />
					<CellButton />
				</tr>
			))}
		</>
	);
}

export function SellerInvoiceDocumentsSkeletonRows({ rows = 8 }: RowsProps) {
	return (
		<>
			{useRowKeys(rows, "skel-invoice-doc").map((key) => (
				<tr key={key} className="seller-skeleton__row">
					<CellBar label="訂單編號" />
					<CellBar label="付款時間" />
					<CellBar label="買家類型" />
					<CellPill label="開票方式" align="left" />
					<CellBar label="金額" align="right" />
					<CellPill label="發票狀態" align="left" />
					<CellPill label="出貨狀態" align="left" />
					<CellBar label="更新時間" />
				</tr>
			))}
		</>
	);
}

/* ------------------------------------------------------------------ */
/* Non-table page skeletons                                            */
/* ------------------------------------------------------------------ */

interface PostsSkeletonProps {
	count?: number;
	delegated?: boolean;
}

/** Uses the production cover, text, chip and action layout. */
export function SellerPostsSkeleton({
	count = 4,
	delegated = false,
}: PostsSkeletonProps) {
	return (
		<ul
			className="seller-posts__list"
			aria-live="polite"
			aria-busy="true"
			aria-label="載入動態中"
		>
			{useRowKeys(count, "skel-post").map((key) => (
				<li key={key} className="seller-posts__row">
					<div className="seller-posts__cover">
						<span className={`${bone} block size-full`} />
					</div>
					<div className="seller-posts__main">
						<div className="seller-posts__excerpt seller-skeleton__excerpt">
							<span className={`${bone} seller-skeleton__line w-[90%]`} />
							<span className={`${bone} seller-skeleton__line w-[60%]`} />
						</div>
						{!delegated && (
							<div className="seller-posts__stats">
								{[0, 1, 2, 3].map((i) => (
									<span key={i} className="seller-posts__stat">
										<span className={`${bone} seller-skeleton__line w-9`} />
									</span>
								))}
							</div>
						)}
					</div>
					{!delegated && (
						<div className="seller-posts__actions">
							{[0, 1].map((i) => (
								<span key={i} className={`${bone} seller-skeleton__button`} />
							))}
						</div>
					)}
				</li>
			))}
		</ul>
	);
}

/** Phone records use the same 56px summary and 32px leading image as the
 * collapsed production disclosure, rather than the desktop feed card. */
export function SellerMobileDisclosureSkeleton({
	count = 4,
}: {
	count?: number;
}) {
	return (
		<ul
			className="seller-mobile-disclosure-list"
			aria-live="polite"
			aria-busy="true"
			aria-label="載入中"
		>
			{useRowKeys(count, "skel-disclosure").map((key) => (
				<li key={key} className="seller-mobile-disclosure-list__item">
					<div className="seller-mobile-disclosure">
						<div className="seller-mobile-disclosure__summary cursor-default">
							<span className="seller-mobile-disclosure__leading">
								<span className={`${bone} block size-full`} />
							</span>
							<span className="seller-mobile-disclosure__title">
								<span className={`${bone} seller-skeleton__line w-full`} />
							</span>
							<span className="seller-mobile-disclosure__meta">
								<span className={`${bone} seller-skeleton__line w-6`} />
							</span>
							<span className="seller-mobile-disclosure__status">
								<span className={`${bone} seller-skeleton__line w-9`} />
							</span>
							<span className="seller-mobile-disclosure__chevron">
								<span className={`${bone} size-4`} />
							</span>
						</div>
					</div>
				</li>
			))}
		</ul>
	);
}

interface CouponsSkeletonProps {
	count?: number;
}

const shimmerBg =
	"bg-[linear-gradient(100deg,var(--hkui-default-200)_0%,var(--hkui-default-200)_35%,rgb(var(--color-neutral-cream-rgb)/0.72)_50%,var(--hkui-default-200)_65%,var(--hkui-default-200)_100%)] dark:bg-[linear-gradient(100deg,rgba(245,233,195,0.1)_0%,rgba(245,233,195,0.1)_35%,rgba(245,233,195,0.32)_50%,rgba(245,233,195,0.1)_65%,rgba(245,233,195,0.1)_100%)] bg-[length:220%_100%] bg-[position:100%_0] animate-sc-skel-shimmer motion-reduce:animate-none";

function getShimmerDelay(index: number): React.CSSProperties {
	const delay =
		index === 0
			? 0
			: index === 1
				? 0.12
				: index === 2
					? 0.24
					: index === 3
						? 0.36
						: 0.48;
	return { animationDelay: `${delay}s` };
}

/** Staggered entrance delay for skeleton rows — each row eases in slightly
 *  after the previous one, giving the list a gentle top-down cascade as it
 *  appears (capped so longer lists don't drag). */
function getEntranceDelay(index: number): React.CSSProperties {
	return { animationDelay: `${Math.min(index, 5) * 0.07}s` };
}

/** Coupons list — reuses the real `.seller-coupons__*` grid (2-up cards,
 *  each `chip · main · state · actions`) so the skeleton lays out exactly
 *  like the loaded list with zero layout shift. Shimmer bars stand in for
 *  the value, title, meta rows, status pill and action buttons. */
export function SellerCouponsSkeleton({ count = 4 }: CouponsSkeletonProps) {
	const keys = useRowKeys(count, "skel-coupon");
	return (
		<ul
			className="seller-coupons__list"
			aria-live="polite"
			aria-busy="true"
			aria-label="載入中"
		>
			{keys.map((key, idx) => (
				<li
					key={key}
					className="seller-coupons__item animate-sc-skel-in motion-reduce:animate-none"
					style={getEntranceDelay(idx)}
				>
					<div className="seller-coupons__chip">
						<span
							className={`${shimmerBg} block h-[22px] w-[68%] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
						<span
							className={`${shimmerBg} block h-[11px] w-[40%] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
					</div>
					<div className="seller-coupons__main">
						<span
							className={`${shimmerBg} block h-[14px] w-[82%] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
						<span
							className={`${shimmerBg} block h-[11px] w-[56%] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
						<span
							className={`${shimmerBg} block h-[11px] w-[38%] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
					</div>
					<div className="seller-coupons__state">
						<span
							className={`${shimmerBg} block h-[22px] w-[56px] rounded-[var(--radius-pill)]`}
							style={getShimmerDelay(idx)}
						/>
					</div>
					<div className="seller-coupons__actions">
						<span
							className={`${shimmerBg} block h-8 w-[60px] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
						<span
							className={`${shimmerBg} block h-8 w-[60px] rounded-[var(--hkui-radius-sm)]`}
							style={getShimmerDelay(idx)}
						/>
					</div>
				</li>
			))}
		</ul>
	);
}

/** Dashboard — hero stat (large) + 3 mini stats + 2 panels (todos +
 *  pending shipments). Composition of smaller pieces. */
export function SellerDashboardSkeleton() {
	return (
		<div
			className="flex flex-col gap-[var(--hkui-space-5)]"
			role="status"
			aria-busy="true"
			aria-label="載入中"
		>
			<div className="flex flex-col gap-[10px] p-[var(--hkui-space-5)] bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] min-h-[120px]">
				<span className="block h-[12px] w-[25%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
				<span className="block h-[12px] w-[90%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
				<span className="block h-[12px] w-[40%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
			</div>
			<div className="seller-stats seller-summary-rail">
				{["a", "b", "c", "d"].map((k) => (
					<div
						key={k}
						className="flex flex-col gap-[10px] p-[var(--hkui-space-4)] bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)]"
					>
						<span className="w-8 h-8 rounded-full shrink-0 block bg-[var(--hkui-default-200)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
						<span className="block h-[12px] w-[25%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
						<span className="block h-[12px] w-[40%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					</div>
				))}
			</div>
			<div className="grid grid-cols-2 gap-[var(--hkui-space-4)] max-[800px]:grid-cols-1">
				{["todos", "shipping"].map((k) => (
					<div
						key={k}
						className="flex flex-col gap-3 p-[var(--hkui-space-5)] bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)] min-h-[200px]"
					>
						<span className="block h-[12px] w-[40%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
						{["a", "b", "c"].map((kk) => (
							<span
								key={`${k}-${kk}`}
								className="block h-[12px] w-[90%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]"
							/>
						))}
					</div>
				))}
			</div>
			<span className="visually-hidden">資料載入中</span>
		</div>
	);
}

/** Invoice overview has a tax banner, three status panels and recent records. */
export function SellerInvoiceOverviewSkeleton() {
	return (
		<div
			className="flex flex-col gap-[var(--hkui-space-4)]"
			role="status"
			aria-busy="true"
			aria-label="載入發票總覽中"
		>
			<section className="flex flex-col gap-[var(--hkui-space-2)] rounded-hkui-lg border border-divider p-[var(--hkui-space-4)]">
				<span className={`${bone} h-6 w-40`} />
				<span className={`${bone} h-5 w-3/4`} />
			</section>
			<div className="grid grid-cols-1 gap-[var(--hkui-space-4)] md:grid-cols-3">
				{[0, 1, 2].map((key) => (
					<section
						key={key}
						className="seller-card flex flex-col gap-1 p-[var(--hkui-space-4)]"
					>
						<span className={`${bone} h-[18px] w-24`} />
						<span className={`${bone} h-6 w-32`} />
						<span className="flex min-h-11 items-center">
							<span className={`${bone} h-3.5 w-16`} />
						</span>
					</section>
				))}
			</div>
			<section className="seller-card">
				<header className="seller-card__head flex min-h-11 items-center justify-between gap-3">
					<span className={`${bone} h-5 w-48`} />
					<span className={`${bone} h-3.5 w-16`} />
				</header>
				<div className="seller-orders__table-wrap seller-mobile-disclosure-desktop">
					<table
						className="seller-table"
						aria-label="最近開立的發票與收據載入中"
					>
						<thead>
							<tr>
								{[
									"開立時間",
									"發票號碼",
									"類型",
									"金額",
									"發票狀態",
									"操作",
								].map((label) => (
									<th key={label} scope="col">
										{label}
									</th>
								))}
							</tr>
						</thead>
						<tbody>
							<SellerTableSkeletonRows rows={5} columns={6} />
						</tbody>
					</table>
				</div>
				<div className="seller-mobile-disclosure-list p-3">
					<SellerMobileDisclosureSkeleton count={5} />
				</div>
			</section>
		</div>
	);
}
