"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import Button from "@/components/ui/Button";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import {
	type CustomerTag,
	type FetchSellerCustomersParams,
	fetchSellerCustomersPage,
	fetchSellerTags,
	type SellerCustomer,
	type SellerCustomerType,
} from "@/lib/seller/repo";

interface Props {
	open: boolean;
	/** Initially-selected buyer account IDs (as strings to match wire). */
	initialSelected: string[];
	/** Confirm — receives the selected `SellerCustomer` rows so the parent
	 * can cache the id → name mapping for the chip display. */
	onConfirm: (selected: SellerCustomer[]) => void;
	/** Backdrop / × / Esc — leaves the form's existing selection untouched. */
	onClose: () => void;
}

type CustomerTypeFilter = "" | SellerCustomerType;

const PAGE_SIZE = 50;
const BULK_PAGE_SIZE = 200;
const CUSTOMER_TYPE_OPTIONS: Array<{
	value: CustomerTypeFilter;
	label: string;
}> = [
	{ value: "", label: "全部客戶" },
	{ value: "vip", label: "VIP" },
	{ value: "normal", label: "一般" },
	{ value: "blocklist", label: "黑名單" },
];
const CUSTOMER_TYPE_LABEL: Record<SellerCustomerType, string> = {
	normal: "一般",
	vip: "VIP",
	blocklist: "黑名單",
};

function fallbackCustomer(id: string): SellerCustomer {
	return {
		id,
		name: `買家 #${id}`,
	};
}

function mergeRowsIntoMap(
	prev: Map<string, SellerCustomer>,
	rows: SellerCustomer[],
) {
	const next = new Map(prev);
	for (const row of rows) next.set(String(row.id), row);
	return next;
}

function rowsFromSelection(
	selected: Set<string>,
	cache: Map<string, SellerCustomer>,
) {
	return Array.from(selected).map(
		(id) => cache.get(id) ?? fallbackCustomer(id),
	);
}

/**
 * Buyer picker dialog for the create-coupon "指定買家" flow.
 *
 * Backed by `GET /app/sellers/me/customers` with debounced keyword search and
 * CRM segmentation filters. Segment actions still resolve to buyer account IDs
 * and reuse the coupon service's existing `targetType=3 + buyerAccountIds`
 * contract, so no new coupon-target enum is required.
 */
export default function CouponBuyerPicker({
	open,
	initialSelected,
	onConfirm,
	onClose,
}: Props) {
	const [keyword, setKeyword] = React.useState("");
	const [debounced, setDebounced] = React.useState("");
	const [customerType, setCustomerType] =
		React.useState<CustomerTypeFilter>("");
	const [tagId, setTagId] = React.useState<string | null>(null);
	const [tagPool, setTagPool] = React.useState<CustomerTag[]>([]);
	const [list, setList] = React.useState<SellerCustomer[]>([]);
	const [total, setTotal] = React.useState(0);
	const [loading, setLoading] = React.useState(false);
	const [bulkSelecting, setBulkSelecting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const [selected, setSelected] = React.useState<Set<string>>(
		() => new Set(initialSelected),
	);
	const [selectedRows, setSelectedRows] = React.useState<
		Map<string, SellerCustomer>
	>(() => new Map(initialSelected.map((id) => [id, fallbackCustomer(id)])));

	const activeTag = tagPool.find((tag) => tag.id === tagId) ?? null;
	const activeFilterLabel = [
		customerType ? CUSTOMER_TYPE_LABEL[customerType] : "全部客戶",
		activeTag ? `#${activeTag.name}` : null,
		debounced ? `搜尋「${debounced}」` : null,
	]
		.filter(Boolean)
		.join(" · ");

	const currentParams = React.useMemo<FetchSellerCustomersParams>(
		() => ({
			keyword: debounced || undefined,
			pageSize: PAGE_SIZE,
			customerType: customerType || undefined,
			tagId: tagId ?? undefined,
			sortBy: "order_count",
			sortOrder: "desc",
		}),
		[customerType, debounced, tagId],
	);

	// Reset internal state whenever the dialog re-opens so a stale
	// selection from a previous session doesn't bleed into the new one.
	React.useEffect(() => {
		if (!open) return;
		setSelected(new Set(initialSelected));
		setSelectedRows(
			new Map(initialSelected.map((id) => [id, fallbackCustomer(id)])),
		);
		setKeyword("");
		setDebounced("");
		setCustomerType("");
		setTagId(null);
		setError(null);
		// `initialSelected` intentionally a snapshot per open
	}, [open, initialSelected]);

	React.useEffect(() => {
		if (!open) return;
		let alive = true;
		fetchSellerTags()
			.then((tags) => {
				if (alive) setTagPool(tags);
			})
			.catch(() => {
				if (alive) setTagPool([]);
			});
		return () => {
			alive = false;
		};
	}, [open]);

	// Debounce the keyword → backend call. 240ms feels responsive while
	// still avoiding a request per keystroke.
	React.useEffect(() => {
		const t = window.setTimeout(() => setDebounced(keyword), 240);
		return () => window.clearTimeout(t);
	}, [keyword]);

	React.useEffect(() => {
		if (!open) return;
		let alive = true;
		setLoading(true);
		setError(null);
		fetchSellerCustomersPage({ ...currentParams, page: 1 })
			.then((page) => {
				if (!alive) return;
				setList(page.records);
				setTotal(page.total);
				setSelectedRows((prev) => mergeRowsIntoMap(prev, page.records));
			})
			.catch((e: unknown) => {
				if (alive)
					setError(e instanceof Error ? e.message : "載入客戶清單失敗");
			})
			.finally(() => {
				if (alive) setLoading(false);
			});
		return () => {
			alive = false;
		};
	}, [open, currentParams]);

	// Esc to close, lock body scroll while open — same pattern as
	// CertGateDialog so the workspace feels coherent.
	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		const releaseScrollLock = acquireBodyScrollLock();
		document.addEventListener("keydown", onKey);
		return () => {
			releaseScrollLock();
			document.removeEventListener("keydown", onKey);
		};
	}, [open, onClose]);

	if (!open) return null;

	const toggle = (id: string, row: SellerCustomer) => {
		setSelectedRows((prev) => mergeRowsIntoMap(prev, [row]));
		setSelected((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	};

	const clearAll = () => setSelected(new Set());

	const selectRows = (rows: SellerCustomer[]) => {
		setSelectedRows((prev) => mergeRowsIntoMap(prev, rows));
		setSelected((prev) => {
			const next = new Set(prev);
			for (const row of rows) next.add(String(row.id));
			return next;
		});
	};

	const selectCurrentFilter = async () => {
		if (bulkSelecting || total === 0) return;
		setBulkSelecting(true);
		setError(null);
		try {
			const first = await fetchSellerCustomersPage({
				...currentParams,
				page: 1,
				pageSize: BULK_PAGE_SIZE,
			});
			const rows = [...first.records];
			const expected = first.total;
			const pageCount = Math.ceil(expected / BULK_PAGE_SIZE);
			for (
				let page = 2;
				page <= pageCount && rows.length < expected;
				page += 1
			) {
				const next = await fetchSellerCustomersPage({
					...currentParams,
					page,
					pageSize: BULK_PAGE_SIZE,
				});
				if (next.records.length === 0) break;
				rows.push(...next.records);
			}
			selectRows(rows);
		} catch (e) {
			setError(e instanceof Error ? e.message : "套用分群失敗");
		} finally {
			setBulkSelecting(false);
		}
	};

	const confirm = () => {
		onConfirm(rowsFromSelection(selected, selectedRows));
	};

	return (
		<div
			className="fixed inset-0 z-[300] grid place-items-stretch p-0 sm:place-items-center sm:p-4"
			role="dialog"
			aria-modal="true"
			aria-labelledby="buyer-picker-title"
		>
			<button
				type="button"
				className="absolute inset-0 bg-hk-ink-950/48 border-0 p-0 cursor-pointer backdrop-blur-[4px]"
				aria-label="關閉"
				onClick={onClose}
			/>
			<div className="relative flex h-[100dvh] max-h-[100dvh] w-full max-w-none flex-col overflow-hidden rounded-none bg-hk-surface-card pb-[env(safe-area-inset-bottom,0px)] pt-[env(safe-area-inset-top,0px)] shadow-[0_24px_64px_rgba(0,0,0,0.18)] dark:border dark:border-[#f5dea5]/14 dark:bg-hk-ink-900 sm:h-full sm:max-h-[720px] sm:max-w-[640px] sm:rounded-2xl sm:py-0 md:max-h-[90vh]">
				<header className="flex min-h-14 shrink-0 items-center justify-between border-b border-hk-border-soft px-3 py-1.5 sm:min-h-0 sm:px-5 sm:py-4">
					<div className="min-w-0">
						<h2
							id="buyer-picker-title"
							className="m-0 text-lg font-bold text-hk-heading"
						>
							選擇買家
						</h2>
						<p className="m-0 mt-1 hidden text-xs text-hk-muted sm:block">
							可依客戶等級或 CRM tag 套用分群，再微調個別買家。
						</p>
					</div>
					<button
						type="button"
						className="grid h-11 w-11 shrink-0 place-items-center rounded border-0 bg-transparent p-0 text-2xl leading-none text-hk-muted cursor-pointer hover:bg-hk-surface-muted hover:text-hk-heading sm:h-auto sm:w-auto sm:px-2 sm:py-1"
						aria-label="關閉"
						onClick={onClose}
					>
						<AppIcon name="close" size={20} />
					</button>
				</header>

				<div className="flex shrink-0 flex-col gap-3 border-b border-hk-border-soft px-3 py-3 sm:px-5">
					<input
						type="search"
						value={keyword}
						onChange={(e) => setKeyword(e.target.value)}
						placeholder="搜尋客戶名稱 / 電話"
						aria-label="搜尋客戶"
						className="h-11 w-full rounded-md border border-hk-border-soft bg-hk-surface-card px-3 font-inherit focus:border-hk-primary focus:outline-none focus:ring-3 focus:ring-hk-primary/20 sm:h-10"
					/>

					<fieldset className="border-0 p-0 m-0 min-w-0">
						<legend className="mb-2 text-xs font-semibold text-hk-muted">
							客戶等級
						</legend>
						<div
							data-mobile-rail="customer-type"
							className="flex min-w-0 flex-nowrap gap-2 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:flex-wrap sm:overflow-visible"
						>
							{CUSTOMER_TYPE_OPTIONS.map((opt) => {
								const active = customerType === opt.value;
								return (
									<button
										key={opt.value || "all"}
										type="button"
										className={`h-11 shrink-0 whitespace-nowrap px-3 rounded-full border text-xs font-semibold cursor-pointer transition-colors sm:h-8 ${active ? "bg-hk-primary text-white border-hk-primary dark:bg-hk-ink-200 dark:text-hk-ink-900" : "bg-hk-surface-card text-hk-muted border-hk-border-soft hover:border-hk-primary/50 hover:text-hk-primary"}`}
										aria-pressed={active}
										onClick={() => setCustomerType(opt.value)}
									>
										{opt.label}
									</button>
								);
							})}
						</div>
					</fieldset>

					{tagPool.length > 0 && (
						<fieldset className="border-0 p-0 m-0 min-w-0">
							<legend className="mb-2 text-xs font-semibold text-hk-muted">
								CRM tag
							</legend>
							<div
								data-mobile-rail="crm-tag"
								className="flex min-w-0 flex-nowrap gap-2 overflow-x-auto overscroll-x-contain [scrollbar-width:none] [&::-webkit-scrollbar]:hidden sm:flex-wrap sm:overflow-visible"
							>
								<button
									type="button"
									className={`h-11 shrink-0 whitespace-nowrap px-3 rounded-full border text-xs font-semibold cursor-pointer transition-colors sm:h-8 ${tagId === null ? "bg-hk-primary text-white border-hk-primary dark:bg-hk-ink-200 dark:text-hk-ink-900" : "bg-hk-surface-card text-hk-muted border-hk-border-soft hover:border-hk-primary/50 hover:text-hk-primary"}`}
									aria-pressed={tagId === null}
									onClick={() => setTagId(null)}
								>
									全部標籤
								</button>
								{tagPool.map((tag) => {
									const active = tagId === tag.id;
									return (
										<button
											key={tag.id}
											type="button"
											className={`h-11 shrink-0 whitespace-nowrap px-3 rounded-full border text-xs font-semibold cursor-pointer transition-colors sm:h-8 ${active ? "bg-hk-primary text-white border-hk-primary dark:bg-hk-ink-200 dark:text-hk-ink-900" : "bg-hk-surface-card text-hk-muted border-hk-border-soft hover:border-hk-primary/50 hover:text-hk-primary"}`}
											aria-pressed={active}
											onClick={() => setTagId(active ? null : tag.id)}
										>
											{tag.name}
										</button>
									);
								})}
							</div>
						</fieldset>
					)}

					<div
						data-mobile-rail="bulk-actions"
						className="flex min-w-0 flex-nowrap items-center gap-2 overflow-x-auto overscroll-x-contain rounded-md border border-hk-border-soft bg-hk-surface-muted p-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden dark:border-[#f5dea5]/14 dark:bg-white/4 sm:flex-wrap sm:overflow-visible sm:px-3"
					>
						<span
							className="max-w-[72vw] shrink-0 truncate whitespace-nowrap text-xs text-hk-muted sm:min-w-[180px] sm:max-w-none sm:flex-1"
							title={`目前篩選：${activeFilterLabel || "全部客戶"} · ${total} 位`}
						>
							目前篩選：{activeFilterLabel || "全部客戶"} · {total} 位
						</span>
						<Button
							type="button"
							variant="bordered"
							color="default"
							size="sm"
							onClick={selectCurrentFilter}
							disabled={bulkSelecting || loading || total === 0}
							isLoading={bulkSelecting}
							loadingLabel="套用分群買家中…"
							className="h-11 min-h-11 shrink-0 whitespace-nowrap sm:h-8 sm:min-h-8"
						>
							全選此分群（{total}）
						</Button>
						<Button
							type="button"
							variant="bordered"
							color="default"
							size="sm"
							onClick={() => selectRows(list)}
							disabled={list.length === 0}
							className="h-11 min-h-11 shrink-0 whitespace-nowrap sm:h-8 sm:min-h-8"
						>
							全選目前頁
						</Button>
						<Button
							type="button"
							variant="bordered"
							color="default"
							size="sm"
							onClick={clearAll}
							disabled={selected.size === 0}
							className="h-11 min-h-11 shrink-0 whitespace-nowrap sm:hidden"
						>
							清除已選
						</Button>
					</div>
				</div>

				{error && (
					<p
						className="m-0 bg-hk-danger/10 px-3 py-3 text-sm text-hk-danger sm:px-5"
						role="alert"
					>
						{error}
					</p>
				)}

				<div
					className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto overscroll-contain px-2 py-2 sm:px-3"
					role="listbox"
					aria-multiselectable
				>
					{loading && list.length === 0 ? (
						<p className="py-6 px-4 m-0 text-center text-hk-muted text-sm">
							載入中…
						</p>
					) : list.length === 0 ? (
						<p className="py-6 px-4 m-0 text-center text-hk-muted text-sm">
							{debounced ? `找不到「${debounced}」` : "目前篩選沒有客戶。"}
						</p>
					) : (
						list.map((c) => {
							const id = String(c.id);
							const isOn = selected.has(id);
							return (
								<label
									key={id}
									className={`flex items-center gap-3 py-2.5 px-3 rounded-md cursor-pointer transition-colors duration-150 hover:bg-hk-surface-muted ${isOn ? "bg-hk-primary/10 dark:bg-hk-ink-300/18" : ""}`}
								>
									<input
										type="checkbox"
										checked={isOn}
										onChange={() => toggle(id, c)}
										aria-label={`選擇 ${c.name}`}
										className="w-4.5 h-4.5 accent-hk-primary cursor-pointer shrink-0"
									/>
									<span className="w-9 h-9 rounded-full overflow-hidden shrink-0 bg-hk-surface-muted">
										<Avatar
											src={c.avatar}
											name={c.name}
											alt=""
											entity="buyer"
											fallback="initial"
										/>
									</span>
									<span className="flex flex-col gap-1 min-w-0 flex-1">
										<span className="flex items-center gap-2 min-w-0">
											<strong className="text-sm font-semibold text-hk-heading truncate">
												{c.name}
											</strong>
											{c.customerType === "vip" && (
												<span className="shrink-0 rounded-full bg-hk-primary/10 text-hk-accent-on-tint px-2 py-0.5 text-[0.6875rem] font-bold dark:bg-hk-ink-200/18 dark:text-hk-ink-200">
													VIP
												</span>
											)}
										</span>
										<small className="text-xs text-hk-muted">
											{c.phone ?? ""}
											{c.phone && c.orderCount ? " · " : ""}
											{c.orderCount ? `${c.orderCount} 筆訂單` : ""}
										</small>
										{c.tags && c.tags.length > 0 && (
											<span className="flex flex-wrap gap-1">
												{c.tags.slice(0, 3).map((tag) => (
													<span
														key={tag}
														className="rounded-full bg-hk-surface-muted px-2 py-0.5 text-[0.6875rem] font-semibold text-hk-muted dark:bg-white/8"
													>
														{tag}
													</span>
												))}
												{c.tags.length > 3 && (
													<span className="rounded-full bg-hk-surface-muted px-2 py-0.5 text-[0.6875rem] font-semibold text-hk-muted dark:bg-white/8">
														+{c.tags.length - 3}
													</span>
												)}
											</span>
										)}
									</span>
								</label>
							);
						})
					)}
				</div>

				<footer
					data-mobile-footer="compact-actions"
					className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-2 border-t border-hk-border-soft bg-hk-surface-card px-3 py-2.5 sm:flex sm:px-5 sm:py-3"
				>
					<span className="min-w-0 truncate whitespace-nowrap text-sm text-hk-muted">
						已選{" "}
						<strong className="text-hk-primary font-bold">
							{selected.size}
						</strong>{" "}
						位
					</span>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						onClick={clearAll}
						disabled={selected.size === 0}
						className="hidden sm:inline-flex"
					>
						清除全部
					</Button>
					<div className="hidden flex-1 sm:block" />
					<Button
						variant="bordered"
						color="default"
						onClick={onClose}
						className="h-11 min-h-11 px-3 sm:h-10 sm:min-h-10 sm:px-4"
					>
						取消
					</Button>
					<Button
						onClick={confirm}
						disabled={selected.size === 0}
						className="h-11 min-h-11 px-3 sm:h-10 sm:min-h-10 sm:px-4"
					>
						<span className="sm:hidden">確認</span>
						<span className="hidden sm:inline">確認（{selected.size}）</span>
					</Button>
				</footer>
			</div>
		</div>
	);
}
