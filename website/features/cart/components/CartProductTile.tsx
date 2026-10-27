"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import type { CartItem } from "@/lib/cart";
import { formatUnitLabel } from "@/lib/codemeta/enums";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { TemperatureTagRow } from "./TemperatureTag";

// 後端 issue #1127：cart 透過 purchasable + unavailableReason 表達 row 級可購買性，
// reason ∈ {DELISTED, OUT_OF_STOCK, INSUFFICIENT_STOCK, LOOKUP_FAILED}（CartItemUnavailableReason）。
// 前端只看 purchasable 決定反灰；reason 用於 badge 文案與 dialog 提示。
const UNAVAILABLE_REASON_LABEL: Record<string, string> = {
	DELISTED: "已下架",
	OUT_OF_STOCK: "已售完",
	INSUFFICIENT_STOCK: "庫存不足",
	LOOKUP_FAILED: "暫時無法確認",
};

// Legacy 路徑（舊版 backend / 訪客本地快取無 purchasable 欄位時的 fallback）。
// purchasable 欄位優先；item.status 與 stockQty 是 fallback。
const UNAVAILABLE_CART_STATUSES = new Set<string>([
	"SOLD_OUT",
	"OFF_SHELF",
	"sold_out",
	"off_shelf",
]);

interface Props {
	item: CartItem;
	isChecked: boolean;
	isLocked?: boolean;
	editingValue?: string;
	onCheck: (checked: boolean) => void;
	onChangeQty: (raw: string) => void;
	onCommitQty: () => unknown;
	onIncrement: () => unknown;
	onDecrement: () => unknown;
	onSaveForLater: () => unknown;
	onRemove: () => unknown;
}

export type CartRowPendingAction =
	| "commit-quantity"
	| "decrement"
	| "increment"
	| "save-for-later"
	| "remove";

export interface CartRowOperationCoordinator {
	pendingBySku: Readonly<Record<string, CartRowPendingAction | undefined>>;
	blocked: boolean;
	selectionBlocked?: boolean;
	run: (
		skuId: string,
		action: CartRowPendingAction,
		operation: () => unknown,
	) => Promise<void>;
}

export const CartRowOperationContext =
	React.createContext<CartRowOperationCoordinator | null>(null);

function ntFormat(n: number) {
	return `NT$ ${Math.max(0, Math.round(n)).toLocaleString()}`;
}

/**
 * Cart item row matching the app's CartProductTile layout: checkbox |
 * 4:3 thumbnail | name/spec / temp tags / unit price + listPrice strike +
 * stock / quantity stepper. iOS-style swipe-to-delete is left to the
 * outer list — web uses explicit 移除 / 稍後買 buttons.
 *
 * Stock count goes red when ≤ 5; max-quantity input is capped to
 * `item.stockQty` when present so the buyer can't push past stock.
 */
export function CartProductTile({
	item,
	isChecked,
	isLocked,
	editingValue,
	onCheck,
	onChangeQty,
	onCommitQty,
	onIncrement,
	onDecrement,
	onSaveForLater,
	onRemove,
}: Props) {
	const operationCoordinator = React.useContext(CartRowOperationContext);
	const localPendingRef = React.useRef<CartRowPendingAction | null>(null);
	const [localPendingAction, setLocalPendingAction] =
		React.useState<CartRowPendingAction | null>(null);
	const pendingAction =
		operationCoordinator?.pendingBySku[item.id] ?? localPendingAction;
	const operationBlocked = operationCoordinator?.blocked ?? false;
	const runAction = React.useCallback(
		async (action: CartRowPendingAction, operation: () => unknown) => {
			if (operationCoordinator) {
				await operationCoordinator.run(item.id, action, operation);
				return;
			}
			if (localPendingRef.current) return;
			localPendingRef.current = action;
			setLocalPendingAction(action);
			try {
				await operation();
			} finally {
				localPendingRef.current = null;
				setLocalPendingAction(null);
			}
		},
		[item.id, operationCoordinator],
	);
	// 後端 issue #1127：purchasable 為 source of truth。reason 用於 badge 文案。
	// 舊欄位（status / stockQty）保留作 fallback，避免後端未升級或本地舊快取造成全綠誤導。
	const purchasable = item.purchasable !== false;
	const reason = item.unavailableReason ?? "";
	const stock = item.stockQty;
	const stockLow = typeof stock === "number" && stock > 0 && stock <= 5;
	const stockOut = typeof stock === "number" && stock <= 0;
	const legacyUnavailable =
		item.status != null &&
		item.status !== "" &&
		UNAVAILABLE_CART_STATUSES.has(String(item.status));
	const isUnavailable = !purchasable || legacyUnavailable || stockOut;
	const badgeLabel =
		reason && UNAVAILABLE_REASON_LABEL[reason]
			? UNAVAILABLE_REASON_LABEL[reason]
			: stockOut
				? "已售完"
				: item.status === "off-shelf" || item.status === "OFF_SHELF"
					? "已下架"
					: item.status === "out-of-stock" || item.status === "SOLD_OUT"
						? "已售完"
						: "目前不可購買";
	const lineTotal = item.priceValue * item.quantity;
	const listPrice = parseListPrice(item);
	const onSale = listPrice != null && listPrice > item.priceValue;
	const qtyDraft = editingValue ?? String(item.quantity);
	const max = typeof stock === "number" ? stock : 999;
	const atMax = typeof stock === "number" && item.quantity >= stock;
	// 數量加減 / 數量輸入框 unavailable 時 disabled（沒理由改數量）
	const inputDisabled =
		isUnavailable || operationBlocked || pendingAction != null;
	// 保留既有 unavailable 勾選相容行為；不可購買 row 不計入結帳，仍能逐項移除。
	// 選取另一家店前，先用店頭「改選本店商品」切換。issue #1127 user feedback.
	const checkboxDisabled =
		isLocked ||
		operationBlocked ||
		operationCoordinator?.selectionBlocked ||
		pendingAction != null;
	const imageSrc = item.image ? resolveCdnUrl(item.image) : "";

	return (
		<li
			className={`cart-tile${isUnavailable ? " cart-tile--off" : ""}${
				isLocked ? " cart-tile--locked" : ""
			}`}
		>
			<label className="cart-tile__check">
				<input
					type="checkbox"
					checked={!isLocked && isChecked}
					disabled={checkboxDisabled}
					onChange={(e) => onCheck(e.target.checked)}
					aria-label={`選擇 ${item.name}`}
				/>
			</label>

			<div className="cart-tile__thumb">
				{/* 傳入縮圖尺寸，讓缺圖佔位使用小圖示，避免在手機框內裁切。 */}
				<AppImage
					src={imageSrc}
					alt={item.name}
					width={118}
					height={88}
					loading="eager"
					sizes="118px"
					className="size-full object-cover"
				/>
			</div>

			<div className="cart-tile__body">
				<div className="cart-tile__head">
					{item.href ? (
						<Link href={item.href} className="cart-tile__name">
							{item.name}
						</Link>
					) : (
						<strong className="cart-tile__name">{item.name}</strong>
					)}
					{isUnavailable ? (
						<span className="cart-tile__badge">{badgeLabel}</span>
					) : null}
					{item.unit ? (
						<small className="cart-tile__spec">
							規格 : {formatUnitLabel(item.unit)}
						</small>
					) : null}
				</div>

				{item.deliveryTemps && item.deliveryTemps.length > 0 ? (
					<div className="cart-tile__temps">
						<svg
							className="cart-tile__temp-icon"
							viewBox="0 0 24 24"
							width="20"
							height="20"
							fill="none"
							stroke="currentColor"
							strokeWidth="1.8"
							aria-hidden="true"
						>
							<path d="M14 4v10.5a3.5 3.5 0 1 1-4 0V4a2 2 0 1 1 4 0Z" />
						</svg>
						<small>配送溫層</small>
						<TemperatureTagRow values={item.deliveryTemps} />
					</div>
				) : null}

				<div className="cart-tile__meta">
					<div className="cart-tile__price-line">
						<small>單價</small>
						<b>{item.priceValue > 0 ? ntFormat(item.priceValue) : "—"}</b>
						{onSale && listPrice ? (
							<s className="cart-tile__list-price">{ntFormat(listPrice)}</s>
						) : null}
					</div>
					{stock != null ? (
						<div className="cart-tile__stock-line">
							<small>庫存</small>
							<span
								className={
									!purchasable
										? "cart-tile__stock cart-tile__stock--out"
										: stockLow
											? "cart-tile__stock cart-tile__stock--low"
											: "cart-tile__stock"
								}
							>
								{/* 不可購買時庫存欄顯示對應 reason 文字（與 badge 對齊），其餘正常顯示庫存數字 */}
								{reason === "DELISTED"
									? "已下架"
									: reason === "OUT_OF_STOCK" || stockOut
										? "已售完"
										: reason === "INSUFFICIENT_STOCK"
											? `僅剩 ${stock}`
											: reason === "LOOKUP_FAILED"
												? "暫無資料"
												: stock}
							</span>
						</div>
					) : null}
				</div>
			</div>

			<div className="cart-tile__qty">
				<Button
					type="button"
					variant="light"
					color="primary"
					size="md"
					radius="none"
					isIconOnly
					className="cart-tile__qty-button"
					onClick={() => void runAction("decrement", onDecrement)}
					aria-label={`減少 ${item.name} 數量`}
					disabled={inputDisabled || item.quantity <= 0}
					isLoading={pendingAction === "decrement"}
					loadingLabel={
						item.quantity <= 1
							? "移除購物車商品中…"
							: `減少 ${item.name} 數量中…`
					}
				>
					<span className="cart-tile__qty-symbol" aria-hidden="true">
						−
					</span>
				</Button>
				<input
					type="number"
					min={0}
					max={max}
					inputMode="numeric"
					value={qtyDraft}
					onChange={(e) => onChangeQty(e.target.value)}
					onBlur={() => void runAction("commit-quantity", onCommitQty)}
					onKeyDown={(e) => {
						if (e.key === "Enter") e.currentTarget.blur();
					}}
					disabled={inputDisabled}
					aria-label={`${item.name} 數量`}
					aria-busy={pendingAction === "commit-quantity" || undefined}
				/>
				<Button
					type="button"
					variant="light"
					color="primary"
					size="md"
					radius="none"
					isIconOnly
					className="cart-tile__qty-button"
					onClick={() => void runAction("increment", onIncrement)}
					aria-label={`增加 ${item.name} 數量`}
					disabled={inputDisabled || atMax}
					isLoading={pendingAction === "increment"}
					loadingLabel={`增加 ${item.name} 數量中…`}
				>
					<span className="cart-tile__qty-symbol" aria-hidden="true">
						+
					</span>
				</Button>
			</div>

			<div className="cart-tile__total">
				<strong>{ntFormat(lineTotal)}</strong>
			</div>

			<div className="cart-tile__actions">
				{/* 不可購買 row 隱藏「稍後買」（語意不通：下架商品不會再上架；缺貨另議）— issue #1127 */}
				{isUnavailable ? null : (
					<Button
						type="button"
						variant="light"
						color="default"
						size="md"
						className="cart-tile__action-btn"
						onClick={() => void runAction("save-for-later", onSaveForLater)}
						title="移至稍後購買"
						disabled={operationBlocked || pendingAction != null}
						isLoading={pendingAction === "save-for-later"}
						loadingLabel="移動中…"
					>
						稍後買
					</Button>
				)}
				<Button
					type="button"
					variant="light"
					color="danger"
					size="md"
					className="cart-tile__action-btn cart-tile__action-btn--danger"
					onClick={() => void runAction("remove", onRemove)}
					title="移除"
					disabled={operationBlocked || pendingAction != null}
					isLoading={pendingAction === "remove"}
					loadingLabel="移除中…"
				>
					移除
				</Button>
			</div>
		</li>
	);
}

function parseListPrice(item: CartItem): number | null {
	const raw = (item as unknown as Record<string, unknown>).listPrice;
	if (raw == null || raw === "") return null;
	const n = Number(String(raw).replace(/[^\d.]/g, ""));
	return Number.isFinite(n) && n > 0 ? n : null;
}
