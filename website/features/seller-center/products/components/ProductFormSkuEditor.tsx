"use client";

/**
 * Section 4 of the product form — repeating SKU rows with inline sale price.
 * Extracted from ProductFormPage because the
 * inline JSX was the longest sub-tree (~150 lines) and the bottom of
 * the form effectively becomes scroll-fluff once you've defined the
 * first SKU.
 */

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import type {
	ProductSku,
	SellerProductDetail,
} from "@/lib/products/seller-repo";
import { SectionHeader } from "./SectionHeader";
import SkuShippingEditor from "./SkuShippingEditor";

interface Props {
	skus: SellerProductDetail["skus"];
	onUpdate: (index: number, patch: Partial<ProductSku>) => void;
	onAdd: () => void;
	onRemove: (index: number) => void;
}

export default function ProductFormSkuEditor({
	skus,
	onUpdate,
	onAdd,
	onRemove,
}: Props) {
	const list = skus ?? [];
	const canRemove = list.length > 1;

	const scrollerRef = React.useRef<HTMLDivElement>(null);
	const cardRefs = React.useRef<Array<HTMLDivElement | null>>([]);
	const [active, setActive] = React.useState(0);
	const prevLen = React.useRef(list.length);
	const safeActive = Math.min(active, Math.max(0, list.length - 1));

	const scrollToCard = React.useCallback((i: number) => {
		cardRefs.current[i]?.scrollIntoView({
			behavior: "smooth",
			inline: "start",
			block: "nearest",
		});
	}, []);

	// Land the seller on a freshly-added spec card.
	React.useEffect(() => {
		if (list.length > prevLen.current) {
			const i = list.length - 1;
			setActive(i);
			requestAnimationFrame(() => scrollToCard(i));
		}
		prevLen.current = list.length;
	}, [list.length, scrollToCard]);

	// Track which card is snapped into view (drives the pager dots).
	const onScroll = () => {
		const scroller = scrollerRef.current;
		if (!scroller) return;
		const left = scroller.scrollLeft;
		let best = 0;
		let bestDist = Number.POSITIVE_INFINITY;
		cardRefs.current.slice(0, list.length).forEach((el, i) => {
			if (!el) return;
			const dist = Math.abs(el.offsetLeft - left);
			if (dist < bestDist) {
				bestDist = dist;
				best = i;
			}
		});
		setActive(best);
	};

	return (
		<section
			className="bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 scroll-mt-5 dark:bg-white/4 dark:border-[#f5dea5]/14"
			id="section-skus"
		>
			<SectionHeader
				title="規格 / SKU"
				desc="每個規格可設定原價、選填售價與庫存。未填售價時會以原價販售；填寫時需低於原價。"
				action={
					<Button variant="bordered" size="sm" onClick={onAdd}>
						新增規格
					</Button>
				}
			/>

			<div
				ref={scrollerRef}
				onScroll={onScroll}
				className="relative flex flex-row gap-4 overflow-x-auto snap-x snap-mandatory pb-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
			>
				{list.map((sku, i) => (
					<div
						// biome-ignore lint/suspicious/noArrayIndexKey: SKUs have no stable client-side id; index identifies the card's position in the form
						key={i}
						ref={(el) => {
							cardRefs.current[i] = el;
						}}
						className="snap-start shrink-0 w-full"
					>
						<SkuCard
							sku={sku}
							index={i}
							canRemove={canRemove}
							onUpdate={onUpdate}
							onRemove={onRemove}
						/>
					</div>
				))}
			</div>

			{list.length > 1 && (
				<div className="flex items-center justify-center gap-2">
					<PagerButton
						dir="prev"
						disabled={safeActive === 0}
						onClick={() => scrollToCard(safeActive - 1)}
					/>
					<div className="flex items-center gap-1.5">
						{list.map((_, i) => (
							<button
								// biome-ignore lint/suspicious/noArrayIndexKey: positional pager dots
								key={i}
								type="button"
								aria-label={`第 ${i + 1} 個規格`}
								aria-current={i === safeActive}
								onClick={() => scrollToCard(i)}
								className={`h-2 rounded-full transition-all duration-200 ${
									i === safeActive
										? "w-5 bg-hkui-primary"
										: "w-2 bg-hk-border-soft hover:bg-hkui-primary/40"
								}`}
							/>
						))}
					</div>
					<PagerButton
						dir="next"
						disabled={safeActive === list.length - 1}
						onClick={() => scrollToCard(safeActive + 1)}
					/>
					<span className="text-xs text-hk-muted ml-1 tabular-nums">
						{safeActive + 1} / {list.length}
					</span>
				</div>
			)}
		</section>
	);
}

function PagerButton({
	dir,
	disabled,
	onClick,
}: {
	dir: "prev" | "next";
	disabled: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			disabled={disabled}
			onClick={onClick}
			aria-label={dir === "prev" ? "上一個規格" : "下一個規格"}
			className="grid place-items-center w-8 h-8 rounded-full border border-hk-border-soft text-foreground-muted transition-colors hover:border-hkui-primary hover:text-hkui-primary disabled:opacity-35 disabled:cursor-not-allowed disabled:hover:border-hk-border-soft disabled:hover:text-foreground-muted"
		>
			<AppIcon
				name={dir === "prev" ? "chevron-left" : "chevron-right"}
				size={16}
			/>
		</button>
	);
}

interface SkuCardProps {
	sku: ProductSku;
	index: number;
	canRemove: boolean;
	onUpdate: (index: number, patch: Partial<ProductSku>) => void;
	onRemove: (index: number) => void;
}

function SkuCard({ sku, index, canRemove, onUpdate, onRemove }: SkuCardProps) {
	const original = Number(sku.price ?? 0);
	const sale = Number(sku.sale_price ?? 0);
	const validSale = sale > 0 && original > sale;
	const discount = validSale ? Math.round((1 - sale / original) * 100) : 0;

	return (
		<div className="bg-hk-surface-card border border-hk-border-soft rounded-hkui-lg shadow-[0_4px_16px_-12px_rgb(var(--color-ink-800-rgb)/0.4)] p-4 flex flex-col gap-3 dark:bg-white/6 dark:border-[#f5dea5]/14">
			<div className="flex items-center justify-between gap-2">
				<span className="inline-flex items-center gap-2 text-sm font-bold text-foreground">
					<span className="grid place-items-center min-w-6 h-6 px-1.5 rounded-full bg-hkui-primary/12 text-hkui-primary text-xs tabular-nums">
						{index + 1}
					</span>
					規格
				</span>
				{canRemove && (
					<Button
						variant="bordered"
						color="danger"
						size="sm"
						className="whitespace-nowrap"
						onClick={() => onRemove(index)}
					>
						移除
					</Button>
				)}
			</div>

			<div className="grid grid-cols-[minmax(220px,2fr)_repeat(3,minmax(110px,1fr))] gap-3 items-end max-[900px]:grid-cols-2">
				<TextField
					id={`sku-${index}-name`}
					label="規格名稱"
					value={sku.sku_name ?? ""}
					placeholder="例：1 公斤 / 2 公斤 / 禮盒裝"
					onChange={(v) => onUpdate(index, { sku_name: v })}
				/>
				<NumberField
					label="原價 (NT$)"
					value={Number(sku.price) > 0 ? String(sku.price) : ""}
					placeholder="0"
					onChange={(v) => onUpdate(index, { price: v })}
				/>
				<NumberField
					label={validSale ? `售價 (NT$) - 省 ${discount}%` : "售價 (NT$)"}
					value={String(sku.sale_price ?? "")}
					placeholder="選填"
					onChange={(v) => onUpdate(index, { sale_price: v })}
				/>
				<NumberField
					label="庫存"
					value={Number(sku.stock) > 0 ? String(sku.stock) : ""}
					placeholder="0"
					onChange={(v) => onUpdate(index, { stock: Number(v) })}
				/>
			</div>

			<SkuShippingEditor sku={sku} index={index} onUpdate={onUpdate} />
		</div>
	);
}

/* ===== Inline atoms (form field shells) ===== */

function TextField({
	id,
	label,
	value,
	placeholder,
	onChange,
}: {
	id: string;
	label: string;
	value: string;
	placeholder?: string;
	onChange: (v: string) => void;
}) {
	// Keep IME composition local until compositionend. Updating the whole
	// product draft on every composing keystroke can make Safari/Chromium commit
	// the first glyph and drop focus, which made a newly-added SKU appear to
	// accept only one character.
	const [composingValue, setComposingValue] = React.useState<string | null>(
		null,
	);
	const composingRef = React.useRef(false);

	return (
		<Input
			id={id}
			variant="bordered"
			size="lg"
			label={label}
			value={composingValue ?? value}
			placeholder={placeholder}
			onBlur={(e) => {
				if (!composingRef.current) return;
				composingRef.current = false;
				setComposingValue(null);
				onChange(e.currentTarget.value);
			}}
			onCompositionStart={(e) => {
				composingRef.current = true;
				setComposingValue(e.currentTarget.value);
			}}
			onCompositionEnd={(e) => {
				composingRef.current = false;
				setComposingValue(null);
				onChange(e.currentTarget.value);
			}}
			onChange={(e) => {
				const next = e.target.value;
				if (composingRef.current) setComposingValue(next);
				else onChange(next);
			}}
		/>
	);
}

function NumberField({
	label,
	value,
	placeholder,
	onChange,
}: {
	label: string;
	value: string;
	placeholder?: string;
	onChange: (v: string) => void;
}) {
	return (
		<Input
			variant="bordered"
			size="lg"
			type="number"
			min="0"
			label={label}
			value={value}
			placeholder={placeholder}
			onChange={(e) => onChange(e.target.value)}
		/>
	);
}
