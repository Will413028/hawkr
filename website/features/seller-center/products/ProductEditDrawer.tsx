"use client";

import * as React from "react";
import { Drawer } from "@/components/shared/Drawer";
import Button from "@/components/ui/Button";
import { DrawerSkeleton } from "@/components/ui/DrawerSkeleton";
import { Input } from "@/components/ui/Input";
import { ossImageUrl } from "@/lib/media/cdn";
import type { SellerProductDetail } from "@/lib/products/seller-repo";
import ShelfStatusControl from "../components/ShelfStatusControl";
import { resolveStatusButtons } from "./adapters/productFormAdapter";
import ProductFormSections, {
	LOCKED_SURFACE_CLASS,
	LockedOverlay,
	ProductFormErrors,
} from "./components/ProductFormSections";
import { useProductForm } from "./hooks/useProductForm";

/**
 * Click handler for the list rows / edit affordances: opens the quick-edit
 * drawer on a plain left-click, while letting ⌘/Ctrl/middle/modified clicks
 * fall through to the deep-link edit page (e.g. open in a new tab).
 *
 * On a handled (plain) click it also stops propagation, so an inner link that
 * opens the drawer doesn't additionally bubble to a clickable row.
 */
export function openDrawerOnPlainClick(
	e: React.MouseEvent,
	open: () => void,
): void {
	if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0)
		return;
	e.preventDefault();
	e.stopPropagation();
	open();
}

interface Props {
	/** Product to edit; `null` keeps the drawer closed. */
	productId: string | null;
	/** Narrow quick-edit (false) vs wide full-edit (true). */
	expanded: boolean;
	onClose: () => void;
	onToggleExpanded: () => void;
	/** 任一成功 mutation（儲存 / 上架 / 下架）後即時通知父頁重新同步清單 + 統計卡。 */
	onMutated?: () => void;
}

/**
 * Right-side quick-edit drawer for a seller product — same {@link Drawer}
 * primitive as the order / customer drawers. Two widths off one form state:
 *
 *   - Quick (narrow)  — per-SKU 售價 / 庫存 + 上架/下架 toggle, the everyday tweak.
 *   - Expanded (wide) — the full {@link ProductFormSections} (基本/圖片/規格/認證).
 *
 * 上架中商品內容不可改的後端規則，改以「進來就看得到、但鎖住」呈現：頂部一律可點
 * 上下架切換，未下架時下方欄位以毛玻璃遮罩鎖住並提示先下架（沿用進階設定的遮罩樣
 * 式）。下架後就地重抓即可編輯，不離開抽屜。整頁 `/products/[id]/edit` 仍是深連結。
 */
export default function ProductEditDrawer({
	productId,
	expanded,
	onClose,
	onToggleExpanded,
	onMutated,
}: Props) {
	const form = useProductForm({
		mode: "edit",
		productId: productId ?? undefined,
		onMutated,
	});
	const [pendingSubmitAction, setPendingSubmitAction] = React.useState<
		Parameters<typeof form.submit>[0] | null
	>(null);

	// 儲存 / 上架：成功後 form 已內部 emit onMutated（父頁即時同步）；抽屜的職責
	// 只剩「成功就關閉」。下架（unshelf）走 form.unshelf —— 就地解鎖編輯、抽屜保持
	// 開啟，故不經此關閉，但同樣會 emit onMutated。emit 與 close 徹底分離（#1247）。
	const { submit } = form;
	const submitAndClose = React.useCallback(
		async (action: Parameters<typeof submit>[0]) => {
			const succeeded = await submit(action);
			if (succeeded) onClose();
			return succeeded;
		},
		[submit, onClose],
	);
	const submitFromButton = React.useCallback(
		async (action: Parameters<typeof submit>[0]) => {
			if (form.submitting) return;
			setPendingSubmitAction(action);
			try {
				const succeeded = await submitAndClose(action);
				if (!succeeded) setPendingSubmitAction(null);
			} catch (error) {
				setPendingSubmitAction(null);
				throw error;
			}
		},
		[form.submitting, submitAndClose],
	);

	// The host keys this component by productId, so each open mounts a fresh
	// form whose `loading` starts true — no need to diff ids to avoid flashing
	// the previous product.
	const ready = !!productId && !form.loading;
	// 上架中(=1)：內容鎖住，需先下架。其餘（草稿/審核/下架）可編輯。
	const locked = ready && Number(form.draft.status ?? 0) === 1;

	const { primary, secondary } = resolveStatusButtons(
		"edit",
		Number(form.draft.status ?? 0),
	);

	// 頂部上下架切換：下架就地重抓轉可編輯；上架等同「儲存並上架」（存目前欄位後上架）。
	const handleShelfToggle = (target: "4003" | "4004") => {
		if (target === "4003") {
			void form.unshelf();
		} else {
			void submitAndClose("publish");
		}
	};

	const footer = ready ? (
		<div className="flex w-full items-center justify-between gap-3">
			<Button
				type="button"
				variant="bordered"
				color="default"
				size="sm"
				disabled={form.submitting}
				onClick={onToggleExpanded}
			>
				{expanded ? "收合" : "展開編輯更多"}
			</Button>
			{!locked && (
				<div className="flex gap-2">
					<Button
						variant="bordered"
						size="sm"
						disabled={form.submitting}
						isLoading={
							form.submitting && pendingSubmitAction === primary.action
						}
						loadingLabel={`${primary.label}中…`}
						onClick={() => void submitFromButton(primary.action)}
					>
						{primary.label}
					</Button>
					{secondary && (
						<Button
							size="sm"
							disabled={form.submitting}
							isLoading={
								form.submitting && pendingSubmitAction === secondary.action
							}
							loadingLabel={`${secondary.label}中…`}
							onClick={() => void submitFromButton(secondary.action)}
						>
							{secondary.label}
						</Button>
					)}
				</div>
			)}
		</div>
	) : undefined;

	return (
		<Drawer
			open={!!productId}
			onClose={onClose}
			onMutated={onMutated}
			side="right"
			title={expanded ? "編輯商品" : "快速編輯"}
			description={ready ? form.draft.name || "未命名商品" : undefined}
			className={`overflow-clip transition-[max-width] duration-300 ease-[var(--hkui-ease)] ${
				expanded ? "max-w-3xl" : "max-w-md"
			}`}
			footer={footer}
		>
			{!ready ? (
				<DrawerSkeleton />
			) : (
				<div className="flex flex-col gap-5">
					<ProductSummary
						draft={form.draft}
						busy={form.submitting && pendingSubmitAction === null}
						onToggleShelf={
							form.submitting && pendingSubmitAction !== null
								? undefined
								: handleShelfToggle
						}
					/>
					<LockableContent locked={locked}>
						{expanded ? (
							<ProductFormSections variant="drawer" form={form} />
						) : (
							<QuickSkuFields form={form} />
						)}
					</LockableContent>
					{!expanded && !locked && (
						<p className="m-0 text-xs text-hk-muted">
							只調整原價與庫存即可直接儲存；名稱、圖片、描述、認證等請按「展開編輯更多」。
						</p>
					)}
				</div>
			)}
		</Drawer>
	);
}

/* ============================================================
 *               Product summary + shelf toggle
 * ============================================================ */

function ProductSummary({
	draft,
	busy,
	onToggleShelf,
}: {
	draft: SellerProductDetail;
	busy: boolean;
	onToggleShelf?: (target: "4003" | "4004") => void;
}) {
	const mainImage = draft.main_image || draft.images?.[0]?.url || "";
	return (
		<div className="flex items-center gap-3">
			<span className="grid size-14 shrink-0 place-items-center overflow-hidden rounded-hkui-lg bg-hk-surface-muted dark:bg-white/6">
				{mainImage ? (
					// biome-ignore lint/performance/noImgElement: Seller product images may be remote API URLs.
					<img
						src={ossImageUrl(mainImage, "productThumb")}
						alt=""
						className="size-full object-cover"
						loading="lazy"
					/>
				) : (
					<span aria-hidden="true" />
				)}
			</span>
			<div className="flex min-w-0 flex-col items-start gap-1.5">
				<strong className="truncate text-sm font-bold text-hk-heading">
					{draft.name || "未命名商品"}
				</strong>
				{/* 一律可點：上架中按一下即下架（解鎖下方欄位），下架/草稿按一下即上架。 */}
				<ShelfStatusControl
					status={draft.status}
					busy={busy}
					onToggle={onToggleShelf}
				/>
			</div>
		</div>
	);
}

/* ============================================================
 *               Quick-edit fields (原價 / 庫存)
 * ============================================================ */

function QuickSkuFields({ form }: { form: ReturnType<typeof useProductForm> }) {
	const skus = form.draft.skus ?? [];
	const single = skus.length <= 1;

	return (
		<div className="flex flex-col gap-3">
			{skus.map((sku, i) => (
				<div
					// biome-ignore lint/suspicious/noArrayIndexKey: SKUs have no stable client-side id; index identifies the row's position in the form
					key={i}
					className="flex flex-col gap-3 rounded-hkui-lg border border-hk-border-soft p-4 dark:border-[#f5dea5]/14"
				>
					<strong className="text-sm font-semibold text-foreground">
						{sku.sku_name?.trim() || (single ? "預設規格" : `規格 ${i + 1}`)}
					</strong>
					<div className="grid grid-cols-2 gap-3">
						<Input
							variant="bordered"
							size="lg"
							type="number"
							min="0"
							label="原價 (NT$)"
							value={Number(sku.price) > 0 ? String(sku.price) : ""}
							placeholder="0"
							onChange={(e) => form.updateSku(i, { price: e.target.value })}
						/>
						<Input
							variant="bordered"
							size="lg"
							type="number"
							min="0"
							label="庫存"
							value={Number(sku.stock) > 0 ? String(sku.stock) : ""}
							placeholder="0"
							onChange={(e) =>
								form.updateSku(i, { stock: Number(e.target.value) })
							}
						/>
					</div>
				</div>
			))}

			<ProductFormErrors errors={form.errors} />
		</div>
	);
}

/* ============================================================
 *               Locked overlay (上架中)
 * ============================================================ */

/**
 * Wraps the editable area; when `locked` (商品上架中) the content is blurred and
 * non-interactive behind {@link LockedOverlay}'s「先下架」reminder.
 */
function LockableContent({
	locked,
	children,
}: {
	locked: boolean;
	children: React.ReactNode;
}) {
	if (!locked) return <>{children}</>;
	return (
		<div className="relative">
			<div className={LOCKED_SURFACE_CLASS} aria-hidden="true">
				{children}
			</div>
			<LockedOverlay />
		</div>
	);
}

/* ============================================================
 *               Skeleton
 * ============================================================ */
