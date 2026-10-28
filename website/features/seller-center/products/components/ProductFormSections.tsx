"use client";

/**
 * The product form's section body — 基本資訊 / 圖片 / 規格 / 認證 / 進階設定.
 *
 * Extracted from {@link ProductFormPage} so the same fields power two surfaces
 * without duplication:
 *   - `variant="page"`   → Shopify-style two-column grid (left content + right
 *                          rail) wrapped by the page's `<form>` + mobile wizard.
 *   - `variant="drawer"` → single-column stack for the right-side quick-edit
 *                          drawer's expanded mode.
 *
 * The hosting surface owns the form element, header, gates and submit toolbar;
 * this component renders only the section cards and reads/writes through the
 * shared {@link useProductForm} instance passed in.
 */

import AppIcon from "@/components/shared/AppIcon";
import { DatePicker } from "@/components/ui";
import { Input } from "@/components/ui/Input";
import CategorySelector from "@/features/seller/components/CategorySelector";
import DescriptionEditor from "@/features/seller/components/DescriptionEditor";
import ProductImageUploader from "@/features/seller/components/ProductImageUploader";
import { parseDescription } from "@/lib/products/seller-repo";
import type { useProductForm } from "../hooks/useProductForm";
import ProductFormSkuEditor from "./ProductFormSkuEditor";
import { SectionHeader } from "./SectionHeader";

/** Shared card shell so every section reads with the same rhythm. */
export const CARD =
	"bg-hk-surface-card border border-hk-border-soft rounded-[24px] p-6 flex flex-col gap-4 scroll-mt-5 dark:bg-white/4 dark:border-[#f5dea5]/14";

const ERROR_SURFACE =
	"m-0 py-3 px-4 bg-hk-danger/10 text-hk-danger rounded-md text-sm dark:bg-[rgba(218,110,110,0.16)] dark:text-[#e6a3a3]";

/**
 * The form's one error surface — the same danger-tinted `role="alert"` block the
 * page and the drawer have always shown, now able to carry more than one line.
 *
 * A single problem stays a single sentence (a request failure, or the one field
 * the seller missed). Several become a counted list, because that is the whole
 * point of #435: the backend rejects on the first bad field and returns, so a
 * seller who left the 包裝尺寸 section empty learned about 重量, 長, 寬 and 高 on
 * four separate submits. The form knows all of them before the first request.
 */
export function ProductFormErrors({ errors }: { errors: string[] }) {
	if (errors.length === 0) return null;
	if (errors.length === 1) {
		return (
			<p className={ERROR_SURFACE} role="alert">
				{errors[0]}
			</p>
		);
	}
	return (
		<div className={ERROR_SURFACE} role="alert">
			<p className="m-0 font-bold">還有 {errors.length} 項需要補完：</p>
			<ul className="mt-2 mb-0 ps-5 list-disc flex flex-col gap-1">
				{errors.map((message) => (
					<li key={message}>{message}</li>
				))}
			</ul>
		</div>
	);
}

type ProductFormVariant = "page" | "drawer";

interface Props {
	form: ReturnType<typeof useProductForm>;
	variant?: ProductFormVariant;
}

/**
 * On the page the sections carry `id` + `data-step` so the mobile wizard CSS
 * (`.product-form-wizard[data-wizard-step]`) can step them; the drawer shows
 * everything in one scroll, so those attributes are dropped.
 */
function stepAttrs(
	variant: ProductFormVariant,
	step: number,
	id: string,
): { id?: string; "data-step"?: number } {
	return variant === "page" ? { id, "data-step": step } : {};
}

export default function ProductFormSections({ form, variant = "page" }: Props) {
	const basicSection = (
		<section className={CARD} {...stepAttrs(variant, 1, "section-basic")}>
			<SectionHeader
				title="基本資訊"
				desc="商品名、描述、分類與產地 — 買家最先看到的內容。"
			/>
			<Input
				id={variant === "drawer" ? "drawer-product-name" : "product-name"}
				variant="bordered"
				size="lg"
				label="商品名稱"
				isRequired
				value={form.draft.name}
				onChange={(e) => form.update({ name: e.target.value })}
				// `product.name` 是 `VARCHAR(255)`（product-service `V1__init_product_schema.sql:56`）。
				// 前端自己設一個更嚴的上限，會讓合法的商品名在送出之前就被擋掉，而且
				// 擋掉的方式是「打不進去」——賣家看不到任何理由。
				maxLength={255}
				placeholder="例：埔里高山高麗菜（限量）"
				description={`${form.draft.name.length} / 255 字`}
			/>
			<div className="flex flex-col gap-1.5 w-full">
				<span className="text-sm font-medium text-foreground" data-slot="label">
					商品描述
				</span>
				<DescriptionEditor
					value={parseDescription(form.draft.description)}
					onChange={(next) => form.update({ description: next })}
				/>
			</div>
			<div className="flex flex-col gap-1.5 w-full">
				<span className="text-sm font-medium text-foreground" data-slot="label">
					商品分類
				</span>
				<CategorySelector
					value={form.draft.category_id}
					onChange={(id) => form.update({ category_id: id })}
					allowCreate
				/>
				<small className="text-xs text-hk-muted" data-slot="hint">
					商品需選到第三層細項。沒有合適選項時，可在選定的上層分類下直接新增。
				</small>
			</div>

			<Input
				id={variant === "drawer" ? "drawer-product-origin" : "product-origin"}
				variant="bordered"
				size="lg"
				label="產地"
				value={form.draft.origin ?? ""}
				onChange={(e) => form.update({ origin: e.target.value })}
				placeholder="例：南投縣埔里鎮"
				maxLength={40}
			/>
		</section>
	);

	const imagesSection = (
		<section className={CARD} {...stepAttrs(variant, 1, "section-images")}>
			<SectionHeader
				title="圖片"
				desc="第一張會作為主圖。各種比例皆可，會優先完整顯示；建議最短邊至少 600 px，最多 9 張。"
			/>
			<ProductImageUploader
				images={form.draft.images ?? []}
				onChange={form.handleImagesChange}
				max={9}
			/>
		</section>
	);

	const skuSection = (
		<ProductFormSkuEditor
			skus={form.draft.skus}
			onUpdate={form.updateSku}
			onAdd={form.addSku}
			onRemove={form.removeSku}
		/>
	);

	const errorBlock = <ProductFormErrors errors={form.errors} />;

	const advancedSection = (
		<section
			className={`${CARD} relative overflow-hidden`}
			{...stepAttrs(variant, 3, "section-advanced")}
		>
			<div
				className="flex flex-col gap-4 pointer-events-none select-none"
				aria-hidden="true"
			>
				<SectionHeader
					title="進階設定"
					desc="預購、安全庫存、效期、限購等，依商品特性選填。"
				/>

				<DatePicker
					label="預購出貨日"
					variant="bordered"
					size="lg"
					value={form.draft.preorder_ship_at ?? ""}
					onChange={(v) => form.update({ preorder_ship_at: v || null })}
					minDate={new Date().toISOString().slice(0, 10)}
					description="設定後商品卡會出現「預購」標籤"
				/>
				<Input
					id={variant === "drawer" ? "drawer-product-expiry" : "product-expiry"}
					variant="bordered"
					size="lg"
					type="number"
					min="0"
					label="效期 (天)"
					value={String(form.draft.expiry_days ?? "")}
					onChange={(e) => form.update({ expiry_days: Number(e.target.value) })}
					description="剩 ≤ 30% 時自動標「即期優惠」"
				/>
				<Input
					id={
						variant === "drawer"
							? "drawer-product-low-stock"
							: "product-low-stock"
					}
					variant="bordered"
					size="lg"
					type="number"
					min="0"
					label="低庫存提醒門檻"
					value={String(form.draft.low_stock_threshold ?? "")}
					onChange={(e) =>
						form.update({ low_stock_threshold: Number(e.target.value) })
					}
				/>
				<Input
					id={
						variant === "drawer"
							? "drawer-product-max-order"
							: "product-max-order"
					}
					variant="bordered"
					size="lg"
					type="number"
					min="0"
					label="每筆訂單上限"
					value={String(form.draft.max_per_order ?? "")}
					onChange={(e) =>
						form.update({ max_per_order: Number(e.target.value) })
					}
					placeholder="0 = 不限"
				/>

				<label className="flex items-start gap-3 px-4 py-3 bg-hk-surface-muted rounded-lg cursor-pointer dark:bg-white/4">
					<input
						type="checkbox"
						checked={form.draft.show_sale_countdown ?? false}
						onChange={(e) =>
							form.update({ show_sale_countdown: e.target.checked })
						}
						className="mt-1 accent-hkui-primary w-[18px] h-[18px] shrink-0"
					/>
					<div className="flex flex-col gap-[2px]">
						<strong className="text-sm font-bold text-foreground">
							優惠售價顯示倒數
						</strong>
						<small className="text-xs text-foreground-soft">
							買家進 PDP 會看到「優惠剩 hh:mm:ss」加強轉換率
						</small>
					</div>
				</label>
			</div>

			{/* 半透明遮罩：進階設定尚未開放 */}
			<div
				className="absolute inset-0 z-10 grid place-items-center rounded-[24px] bg-hk-surface-card/55 backdrop-blur-[1.5px] dark:bg-[#14140f]/45"
				role="note"
				aria-label="進階設定尚未開放"
			>
				<span className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-full bg-hk-surface-card border border-hk-border-soft text-hk-muted text-sm font-semibold shadow-sm dark:bg-white/10 dark:border-[#f5dea5]/18 dark:text-[#f5e9c3]">
					<AppIcon name="lock" size={14} />
					尚未開放
				</span>
			</div>
		</section>
	);

	// Drawer：單欄堆疊（基本 → 圖片 → 規格 → 認證 → 進階）。
	if (variant === "drawer") {
		return (
			<div className="flex flex-col gap-5 min-w-0">
				{basicSection}
				{imagesSection}
				{skuSection}
				{advancedSection}
				{errorBlock}
			</div>
		);
	}

	// Page：Shopify 式兩欄 — 左主欄內容、右側欄認證與進階。
	return (
		<>
			{/* ───────── 左主欄：商品內容 ───────── */}
			<div className="flex flex-col gap-5 min-w-0">
				{basicSection}
				{imagesSection}
				{/* 規格 / SKU — wrapped so the wizard can step it */}
				<div data-step={2} className="contents max-[960px]:block">
					{skuSection}
				</div>
				{errorBlock}
			</div>

			{/* ───────── 右側欄：認證與進階設定 ───────── */}
			<aside className="flex flex-col gap-5 min-w-0">{advancedSection}</aside>
		</>
	);
}

/* ============================================================
 *               Locked surface (上架中)
 * ============================================================ */

/**
 * Applied to the editable surface (form / fields) while a product is 上架中:
 * blurs it and turns off interaction. Pair with {@link LockedOverlay} inside a
 * `relative` wrapper. Same treatment as the「進階設定 尚未開放」mask so on-shelf
 * lock and not-yet-available read as one pattern.
 */
export const LOCKED_SURFACE_CLASS =
	"pointer-events-none select-none opacity-60 blur-[1.5px]";

/**
 * Frosted overlay carrying the「先下架」reminder. Sits over a
 * {@link LOCKED_SURFACE_CLASS} surface; the seller flips the product off-shelf
 * via the status toggle above it to unlock editing.
 */
export function LockedOverlay() {
	return (
		<div
			className="absolute inset-0 z-10 flex items-start justify-center rounded-[24px] bg-hk-surface-card/55 px-4 pt-10 backdrop-blur-[1.5px] dark:bg-[#14140f]/45"
			role="note"
			aria-label="商品上架中無法編輯，請先下架"
		>
			<div className="flex max-w-[18rem] flex-col items-center gap-2 text-center">
				<span className="inline-flex items-center gap-1.5 rounded-full border border-hk-border-soft bg-hk-surface-card px-3.5 py-1.5 text-sm font-semibold text-hk-muted shadow-sm dark:border-[#f5dea5]/18 dark:bg-white/10 dark:text-[#f5e9c3]">
					<LockGlyph />
					上架中無法編輯
				</span>
				<p className="m-0 text-xs text-hk-muted">
					請先點上方狀態切換為「下架」，即可修改原價、售價、庫存與其他內容。
				</p>
			</div>
		</div>
	);
}

function LockGlyph() {
	return <AppIcon name="lock" size={14} />;
}
