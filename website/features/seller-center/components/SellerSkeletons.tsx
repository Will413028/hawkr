/**
 * Loading-state building blocks for the seller workspace.
 *
 * Three shapes cover ~all data surfaces under `/seller-center/*`:
 *   <SellerTableSkeleton>   — orders / inventory / customers tables
 *   <SellerCardGridSkeleton>— products / featured items grids
 *   <SellerStatRowSkeleton> — the stat-tile row at the top of overview /
 *                             orders / inventory pages
 *
 * All three consume the seller workspace skeleton palette and honor reduced
 * motion. Render them when the
 * initial fetch is in flight AND the data array is still empty —
 * refreshing existing data should keep the old rows visible (the page's
 * own 重新整理 button disabled-state is the right signal there).
 */

import * as React from "react";

interface SellerTableSkeletonProps {
	/** Column headers — the skeleton renders them as a real <thead> so the
	 * column widths line up with the actual table layout once it loads. */
	columns: string[];
	/** Number of placeholder rows. Default 6 covers a typical above-the-fold
	 * view; bump for tables that need more density. */
	rows?: number;
	/** Optional className passed to the wrapping `<table>` so callers can
	 * apply page-specific layout rules (e.g. `seller-orders__table`). */
	tableClassName?: string;
}

export function SellerTableSkeleton({
	columns,
	rows = 6,
	tableClassName,
}: SellerTableSkeletonProps) {
	return (
		<div role="status" aria-busy="true" aria-label="載入中" className="block">
			<table className={tableClassName}>
				<thead>
					<tr>
						{columns.map((label) => (
							<th key={label}>{label}</th>
						))}
					</tr>
				</thead>
				<tbody>
					<SellerTableSkeletonRows columns={columns.length} rows={rows} />
				</tbody>
			</table>
			<span className="visually-hidden">資料載入中</span>
		</div>
	);
}

interface SellerTableSkeletonRowsProps {
	/** How many `<td>` cells each row gets. Must match the existing table's
	 * column count (including any leading checkbox / icon column). */
	columns: number;
	/** Labels are required when the parent table uses phone work cards. */
	labels?: string[];
	/** Number of placeholder rows. Default 6. */
	rows?: number;
}

/**
 * Renders ONLY `<tr>` rows — for use inside a `<tbody>` that already
 * lives in the page's real table markup. Use this on surfaces that need
 * to preserve a complex `<thead>` (multi-row headers, select-all
 * checkbox, etc.) which the full-table variant would replace.
 */
export function SellerTableSkeletonRows({
	columns,
	rows = 6,
	labels,
}: SellerTableSkeletonRowsProps) {
	const rowKeys = React.useMemo(
		() => Array.from({ length: rows }, (_, i) => `skel-row-${i}`),
		[rows],
	);
	const colIndices = React.useMemo(
		() => Array.from({ length: columns }, (_, i) => i),
		[columns],
	);
	return (
		<>
			{rowKeys.map((rowKey) => (
				// `aria-busy` on the ancestor `<tbody>`/page already announces the
				// loading state — keeping it off the rows themselves so Biome's
				// noAriaHiddenOnFocusable doesn't trip and screen-readers don't
				// see two competing busy regions.
				<tr key={rowKey} className="">
					{colIndices.map((ci) => (
						<td key={`${rowKey}-${ci}`} data-label={labels?.[ci]}>
							<span className="block h-3.5 bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
						</td>
					))}
				</tr>
			))}
		</>
	);
}

interface SellerCardGridSkeletonProps {
	/** How many placeholder cards. Default 8 fills a typical 4-col grid
	 * with 2 rows above the fold. */
	count?: number;
	/** Optional className on the outer grid (defaults to a generic 4-col
	 * responsive grid; pass the page's own grid class to match exactly). */
	className?: string;
	/** Form/status panels have no square product image. */
	variant?: "product" | "panel";
}

export function SellerCardGridSkeleton({
	count = 8,
	className,
	variant = "product",
}: SellerCardGridSkeletonProps) {
	const keys = React.useMemo(
		() => Array.from({ length: count }, (_, i) => `skel-card-${i}`),
		[count],
	);
	return (
		<div
			role="status"
			aria-busy="true"
			aria-label="載入中"
			className={
				className ||
				(variant === "panel"
					? "flex flex-col gap-[var(--hkui-space-4)]"
					: "grid grid-cols-4 gap-[var(--hkui-space-4)] max-[1100px]:grid-cols-3 max-[760px]:grid-cols-2 max-[480px]:grid-cols-1")
			}
		>
			{keys.map((key) => (
				<div
					key={key}
					className={
						variant === "panel"
							? "seller-card flex min-w-0 flex-col gap-4 p-5"
							: "flex min-w-0 flex-col gap-2 p-3 bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)]"
					}
				>
					{variant === "product" && (
						<span className="block w-full aspect-square bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-md)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					)}
					<span className="block h-[12px] w-[90%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					<span className="block h-[12px] w-[40%] bg-[var(--hkui-default-200)] rounded-[var(--hkui-radius-sm)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					{variant === "panel" && (
						<span className="seller-skeleton__bone block h-12 w-full" />
					)}
				</div>
			))}
			<span className="visually-hidden">資料載入中</span>
		</div>
	);
}

interface SellerStatRowSkeletonProps {
	/** Number of stat tiles to fake. Default 3 matches the orders page;
	 * dashboard / inventory currently use 4. */
	count?: number;
}

export function SellerStatRowSkeleton({
	count = 3,
}: SellerStatRowSkeletonProps) {
	const keys = React.useMemo(
		() => Array.from({ length: count }, (_, i) => `skel-stat-${i}`),
		[count],
	);
	return (
		<section
			role="status"
			aria-busy="true"
			aria-label="載入中"
			className="seller-inventory__stats"
		>
			{keys.map((key) => (
				<div key={key} className="seller-inventory__stat">
					<span className="seller-inventory__stat-icon shrink-0 block bg-[var(--hkui-default-200)] animate-pulse motion-reduce:animate-none motion-reduce:opacity-70 dark:bg-[rgba(245,233,195,0.12)]" />
					<strong>
						<span className="seller-skeleton__bone h-[0.8em] w-[4ch]" />
					</strong>
					<small>
						<span className="seller-skeleton__bone h-[0.8em] w-16" />
					</small>
				</div>
			))}
			<span className="visually-hidden">資料載入中</span>
		</section>
	);
}
