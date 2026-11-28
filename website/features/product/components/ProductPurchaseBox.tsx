"use client";

import * as React from "react";
import AddToCartButton from "@/components/shared/AddToCartButton";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import { emitCartToast } from "@/components/shared/CartSnackbar";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { addToCart, CartError, type CartInput } from "@/lib/cart";
import { formatUnitLabel } from "@/lib/codemeta/enums";
import { usePathname } from "@/lib/intl/navigation";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { derivePurchaseAvailability } from "@/lib/products/publication";
import {
	fetchRestockSubscriptions,
	subscribeRestock,
	unsubscribeRestock,
} from "@/lib/restock-subscriptions/repo";
import { useLoginRequiredOverlay } from "@/lib/state/use-login-required-overlay";

/**
 * Real-SKU shape consumed by the purchase box. Built by
 * `features/product/ProductDetailPage.tsx` from the backend's
 * `product.skus[]` array. Every field except optional `listPrice`,
 * `unit`, and `stock` is required so the headline price and cart
 * payload always have authoritative numeric data — no synthesised
 * fallbacks inside this component.
 */
export interface PurchaseBoxSku {
	id: string;
	name: string;
	/** Effective unit price (sale-applied). */
	price: number;
	/** Original unit price for strikethrough rendering. Omit when not on sale. */
	listPrice?: number;
	/** Per-SKU unit label override. Falls back to component's `unit` prop. */
	unit?: string;
	/** Inventory count. Disables the "+" button at the cap. */
	stock?: number;
}

function buyNowErrorMessage(err: unknown): string {
	if (err instanceof CartError) {
		if (err.code === 6002 || err.code === 6007) return "商品已下架或暫不可購買";
		if (err.code === 6003) return "商品結帳處理中,請稍候片刻後再試";
		if (err.code === 6006) return "不可購買自己上架的商品";
		return err.message || "加入購物車失敗,請稍後再試";
	}
	return err instanceof Error ? err.message : "加入購物車失敗,請稍後再試";
}

interface ProductPurchaseBoxProps {
	/**
	 * Product-level cart metadata (id, name, image, sellerName, href).
	 * Price / unit / quantity are computed inside this component from the
	 * selected SKU, so they're omitted from the shared item record.
	 */
	item: Omit<CartInput, "quantity" | "price" | "unit">;
	/** SKU options from the product API. Component renders null when empty. */
	skus: PurchaseBoxSku[];
	/** Default unit label used when the selected SKU has no explicit unit. */
	unit?: string;
	/** Optional promo banner under the price card. */
	promoText?: string;
	/** Optional free-shipping note next to the promo. */
	freeShippingNote?: string;
	/**
	 * Initial value of `isRestockSubscribed` from the PDP detail response
	 * (backend issue #942 / 02 §5.13b). Drives the out-of-stock "補貨通知"
	 * button state. Anonymous viewers always see `false`.
	 */
	initialRestockSubscribed?: boolean;
	/** Backend publication policy; false disables purchasing regardless of stock. */
	purchasable?: boolean;
	publicationState?: string;
	/**
	 * 預覽模式：layout 與真實 PDP 完全一致，
	 * 但 AddToCart / 立即購買 / 補貨通知按鈕全部 disabled，no-op。
	 */
	previewMode?: boolean;
}

const CURRENCY = "NT$";

function formatAmount(amount: number): string {
	return Math.round(amount).toLocaleString();
}

function formatCurrency(amount: number): string {
	return `${CURRENCY} ${formatAmount(amount)}`;
}

export default function ProductPurchaseBox({
	item,
	skus,
	unit = "件",
	promoText,
	freeShippingNote,
	initialRestockSubscribed = false,
	purchasable = true,
	publicationState,
	previewMode = false,
}: ProductPurchaseBoxProps) {
	const router = useObservedRouter("product_purchase_box");
	const { status } = useAuth();
	const [selectedSkuId, setSelectedSkuId] = React.useState(skus[0]?.id ?? "");
	const [quantity, setQuantity] = React.useState(1);
	const loginPayload = React.useMemo(
		() => ({
			title: "登入後即可購買",
			description: "登入 Hawkr 即可下單、追蹤訂單，並把購物車跨裝置同步。",
			actionLabel: "登入 / 註冊",
		}),
		[],
	);
	const openLoginRequired = useLoginRequiredOverlay(loginPayload);
	const { authGate, authWaiting } = useAuthGate(openLoginRequired);

	React.useEffect(() => {
		if (!skus.length) return;
		if (!skus.some((s) => s.id === selectedSkuId)) {
			setSelectedSkuId(skus[0].id);
		}
	}, [skus, selectedSkuId]);

	const selected = skus.find((s) => s.id === selectedSkuId) ?? skus[0];

	const [buyingNow, setBuyingNow] = React.useState(false);
	const pathname = usePathname() ?? "";
	React.useEffect(() => {
		void pathname;
		setBuyingNow(false);
	}, [pathname]);

	const productId = item.id;
	const [restockOn, setRestockOn] = React.useState(initialRestockSubscribed);
	const [restockPending, setRestockPending] = React.useState(false);
	const restockPendingRef = React.useRef(false);
	const selectedStock = selected?.stock;
	const purchaseAvailability = derivePurchaseAvailability({
		publicationState,
		purchasable,
		stock: selectedStock,
	});
	const outOfStock = purchaseAvailability.unavailable;

	React.useEffect(() => {
		setRestockOn(initialRestockSubscribed);
	}, [initialRestockSubscribed]);

	React.useEffect(() => {
		if (
			previewMode ||
			status !== "signedIn" ||
			!purchaseAvailability.canSubscribeRestock ||
			initialRestockSubscribed
		)
			return;
		let cancelled = false;
		void (async () => {
			try {
				const page = await fetchRestockSubscriptions({ pageSize: 50 });
				if (cancelled) return;
				if (page.records.some((r) => r.productId === String(productId))) {
					setRestockOn(true);
				}
			} catch {}
		})();
		return () => {
			cancelled = true;
		};
	}, [
		previewMode,
		status,
		purchaseAvailability.canSubscribeRestock,
		productId,
		initialRestockSubscribed,
	]);

	const handleRestockNotify = () => {
		if (restockPendingRef.current) return;
		authGate(() => void runRestockNotify());
	};

	const runRestockNotify = async () => {
		if (restockPendingRef.current) return;
		restockPendingRef.current = true;
		const next = !restockOn;
		setRestockPending(true);
		setRestockOn(next);
		try {
			if (next) {
				await subscribeRestock(productId, selected.id);
				emitCartToast({
					message: "已開啟補貨通知",
					actionHref: null,
				});
			} else {
				await unsubscribeRestock(selected.id);
				emitCartToast({ message: "已取消補貨通知", actionHref: null });
			}
		} catch {
			setRestockOn(!next);
			emitCartToast({
				message: next
					? "設定補貨通知失敗，請稍後再試"
					: "取消補貨通知失敗，請稍後再試",
				tone: "error",
				actionHref: null,
			});
		} finally {
			restockPendingRef.current = false;
			setRestockPending(false);
		}
	};

	if (!selected) return null;

	const unitPrice = selected.price;
	const unitListPrice = selected.listPrice;
	const totalPrice = unitPrice * quantity;
	const totalListPrice = unitListPrice ? unitListPrice * quantity : 0;
	const onSale = unitListPrice != null && unitListPrice > unitPrice;
	const totalSavings = onSale ? totalListPrice - totalPrice : 0;

	const cartItem: CartInput = {
		...item,
		id: selected.id,
		productId: item.id,
		skuId: selected.id,
		price: String(unitPrice),
		unit: formatUnitLabel(selected.unit || unit),
		quantity,
	};

	const handleBuyNow = () => {
		if (buyingNow) return;
		authGate(() => void runBuyNow());
	};

	const runBuyNow = async () => {
		setBuyingNow(true);
		try {
			await addToCart(cartItem);
			router.push("/cart");
		} catch (err) {
			if (err instanceof CartError && err.status === 401) {
				openLoginRequired();
			} else {
				emitCartToast({ message: buyNowErrorMessage(err), tone: "error" });
			}
			setBuyingNow(false);
		}
	};

	const stock = selected.stock;
	const atMaxQty = typeof stock === "number" && quantity >= stock;

	return (
		<div className="flex flex-col gap-[22px]">
			<AuthGateWaitModal open={authWaiting} />
			<div className="bg-[#f3ead7] rounded-[18px] pt-[22px] px-6 pb-[18px] dark:bg-[#f1cd70]/10 dark:shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] max-sm:flex max-sm:flex-col max-sm:gap-1">
				<div className="flex items-baseline flex-wrap gap-x-3.5 gap-y-2.5">
					<span className="text-lg font-semibold text-hk-ink-600 tracking-[0.5px] dark:text-[#ffeec0]">
						{CURRENCY}
					</span>
					<span className="font-[var(--font-chinese)] text-[3.5rem] font-extrabold leading-none text-hk-ink-800 tracking-[-0.5px] dark:text-[#ffeec0] dark:[text-shadow:0_1px_2px_rgba(0,0,0,0.32)] max-md:text-2xl max-sm:text-[1.375rem]">
						{formatAmount(totalPrice)}
					</span>
					{onSale ? (
						<>
							<span className="text-lg text-[#8b8b75] line-through ml-1 dark:text-hk-ink-100/45">
								{formatCurrency(totalListPrice)}
							</span>
							<span className="inline-flex items-center py-1.5 px-3.5 rounded-full bg-[#c4a93a] text-[#2d2010] text-sm font-bold tracking-[0.5px] dark:bg-[#f1cd70]/85 dark:text-[#2d2010]">
								省 {formatCurrency(totalSavings)}
							</span>
						</>
					) : null}
				</div>
				{(promoText || freeShippingNote) && (
					<div className="mt-3 flex flex-wrap items-center gap-2 text-sm text-hk-ink-500 dark:text-hk-ink-100/72">
						{promoText ? (
							<span className="inline-flex items-center gap-1.5">
								<AppIcon name="calendar" size={15} />
								{promoText}
							</span>
						) : null}
						{promoText && freeShippingNote ? (
							<span
								className="text-hk-ink-300 dark:text-hk-ink-100/32"
								aria-hidden="true"
							>
								·
							</span>
						) : null}
						{freeShippingNote ? (
							<span className="inline-flex items-center gap-1.5">
								{freeShippingNote}
							</span>
						) : null}
					</div>
				)}
			</div>

			<section className="flex flex-col gap-3">
				<header className="flex items-baseline gap-3.5">
					<span className="text-[0.9375rem] font-bold text-hk-ink-800 dark:text-[#f5e9c3]">
						規格
					</span>
					<span className="text-sm text-[#8b8b75] dark:text-hk-ink-100/65">
						已選擇：{selected.name}
					</span>
				</header>
				<fieldset
					className="border-0 m-0 p-0 min-w-0 flex flex-wrap gap-2.5"
					aria-label="規格選擇"
				>
					{skus.map((sku) => (
						<button
							key={`sku-${sku.id}`}
							type="button"
							className={
								selected.id === sku.id
									? "min-w-[96px] py-3 px-7 rounded-full border-[1.5px] cursor-pointer transition-[border-color,background,color] duration-[160ms] ease-in-out border-hk-ink-800 bg-[#f0eedf] text-hk-ink-800 dark:bg-[#f1cd70]/18 dark:border-[#f1cd70]/70 dark:text-[#ffeec0]"
									: "min-w-[96px] py-3 px-7 rounded-full border-[1.5px] cursor-pointer transition-[border-color,background,color] duration-[160ms] ease-in-out border-hk-ink-200 bg-[#f0eedf] text-hk-ink-800 hover:border-hk-ink-600 dark:bg-white/4 dark:border-[#f5dea5]/22 dark:text-[#f5e9c3] dark:hover:border-[#f1cd70]/50 dark:hover:bg-[#f5dea5]/8"
							}
							onClick={() => {
								setSelectedSkuId(sku.id);
								if (typeof sku.stock === "number" && quantity > sku.stock) {
									setQuantity(Math.max(1, sku.stock));
								}
							}}
							aria-pressed={selected.id === sku.id}
						>
							{sku.name}
						</button>
					))}
				</fieldset>
			</section>

			<section className="flex items-center gap-[22px] flex-wrap">
				<span className="text-[0.9375rem] font-bold text-hk-ink-800 dark:text-[#f5e9c3]">
					數量
				</span>
				<fieldset
					className="border-0 m-0 min-w-0 inline-flex items-center gap-3 py-1.5 px-2 bg-[#f0eedf] rounded-full dark:bg-white/4"
					aria-label="購買數量"
				>
					<button
						type="button"
						className="w-11 h-11 rounded-full border-[1.5px] border-hk-ink-200 bg-hk-cream grid place-items-center text-hk-ink-600 cursor-pointer transition-[border-color,background,color] duration-[160ms] ease-in-out hover:enabled:border-hk-ink-600 hover:enabled:text-hk-ink-800 disabled:opacity-40 disabled:cursor-not-allowed dark:bg-white/6 dark:border-[#f5dea5]/24 dark:text-[#f5e9c3] dark:hover:enabled:bg-[#f5dea5]/14 dark:hover:enabled:border-[#f1cd70]/50 dark:hover:enabled:text-[#ffeec0]"
						onClick={() => setQuantity((v) => Math.max(1, v - 1))}
						disabled={quantity <= 1}
						aria-label="減少數量"
					>
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2.5"
						>
							<title>減少</title>
							<line x1="5" y1="12" x2="19" y2="12" />
						</svg>
					</button>
					<span className="text-xl font-bold text-hk-ink-800 min-w-[36px] text-center dark:text-[#f5e9c3]">
						{quantity}
					</span>
					<button
						type="button"
						className="w-11 h-11 rounded-full border-[1.5px] border-hk-ink-200 bg-hk-cream grid place-items-center text-hk-ink-600 cursor-pointer transition-[border-color,background,color] duration-[160ms] ease-in-out hover:enabled:border-hk-ink-600 hover:enabled:text-hk-ink-800 disabled:opacity-40 disabled:cursor-not-allowed dark:bg-white/6 dark:border-[#f5dea5]/24 dark:text-[#f5e9c3] dark:hover:enabled:bg-[#f5dea5]/14 dark:hover:enabled:border-[#f1cd70]/50 dark:hover:enabled:text-[#ffeec0]"
						onClick={() =>
							setQuantity((v) =>
								typeof stock === "number" ? Math.min(stock, v + 1) : v + 1,
							)
						}
						disabled={atMaxQty}
						aria-label="增加數量"
					>
						<svg
							width="16"
							height="16"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2.5"
						>
							<title>增加</title>
							<line x1="12" y1="5" x2="12" y2="19" />
							<line x1="5" y1="12" x2="19" y2="12" />
						</svg>
					</button>
				</fieldset>
				{typeof stock === "number" ? (
					<span className="text-sm text-[#8b8b75] dark:text-hk-ink-100/65">
						庫存：{stock} 件
					</span>
				) : null}
			</section>

			<div className="grid grid-cols-2 gap-3.5 mt-1 max-sm:grid-cols-1">
				{outOfStock ? (
					<>
						{purchaseAvailability.canSubscribeRestock && !previewMode ? (
							<Button
								type="button"
								onClick={handleRestockNotify}
								disabled={restockPending}
								isLoading={restockPending}
								loadingLabel={restockOn ? "設定補貨通知中…" : "取消補貨通知中…"}
								loadingReservationLabel="取消補貨通知中…"
								aria-pressed={restockOn}
								className="inline-flex items-center justify-center gap-2.5 min-h-[64px] px-7 rounded-full border-[1.5px] text-[1.0625rem] font-bold tracking-[0.5px] cursor-pointer transition-[filter,transform,background-color] duration-[160ms] ease-in-out hover:enabled:brightness-105 active:enabled:translate-y-0.5 disabled:cursor-not-allowed disabled:opacity-70 max-sm:min-h-[56px] border-hk-ink-800 bg-transparent text-hk-ink-800 dark:border-[#f5dea5]/45 dark:text-[#f5e9c3]"
							>
								<svg
									width="20"
									height="20"
									viewBox="0 0 24 24"
									fill={restockOn ? "currentColor" : "none"}
									stroke="currentColor"
									strokeWidth="2"
									strokeLinecap="round"
									strokeLinejoin="round"
								>
									<title>補貨通知</title>
									<path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
									<path d="M13.73 21a2 2 0 0 1-3.46 0" />
								</svg>
								<span>{restockOn ? "已開啟補貨通知" : "補貨通知"}</span>
							</Button>
						) : null}
						<button
							type="button"
							disabled
							className={`inline-flex items-center justify-center gap-2.5 min-h-[64px] px-7 rounded-full border-0 text-[1.0625rem] font-bold tracking-[0.5px] cursor-not-allowed max-sm:min-h-[56px] bg-hk-ink-200 text-[#8b8b75] dark:bg-white/8 dark:text-hk-ink-100/45 ${purchaseAvailability.canSubscribeRestock && !previewMode ? "" : "col-span-2 max-sm:col-span-1"}`}
						>
							{purchaseAvailability.label}
						</button>
					</>
				) : (
					<>
						<AddToCartButton
							item={cartItem}
							disabled={previewMode}
							className="inline-flex items-center justify-center gap-2.5 min-h-[64px] px-7 rounded-full border-0 text-[1.0625rem] font-bold tracking-[0.5px] cursor-pointer transition-[filter,transform,background-color] duration-[160ms] ease-in-out hover:brightness-105 active:translate-y-0.5 dark:hover:brightness-110 max-sm:min-h-[56px] bg-hk-ink-800 text-hk-cream dark:bg-hk-ink-800/92 dark:text-hk-cream dark:shadow-[inset_0_0_0_1px_rgba(245,222,165,0.18)] disabled:cursor-not-allowed disabled:opacity-50"
							addedLabel={
								<>
									<AppIcon name="cart" size={22} />
									<span>已加入購物車</span>
								</>
							}
						>
							<AppIcon name="cart" size={22} />
							<span>加入購物車</span>
						</AddToCartButton>
						<Button
							type="button"
							disabled={previewMode || buyingNow}
							isLoading={buyingNow}
							loadingLabel="準備購買中…"
							className="inline-flex items-center justify-center gap-2.5 min-h-[64px] px-7 rounded-full border-0 text-[1.0625rem] font-bold tracking-[0.5px] cursor-pointer transition-[filter,transform,background-color] duration-[160ms] ease-in-out hover:brightness-105 active:translate-y-0.5 dark:hover:brightness-110 max-sm:min-h-[56px] bg-[#c4a93a] text-[#2d2010] dark:bg-[#c4a93a]/92 dark:text-[#2d2010] disabled:cursor-not-allowed disabled:opacity-50"
							onClick={previewMode ? undefined : handleBuyNow}
							data-pending={buyingNow ? "true" : "false"}
						>
							<AppIcon name="coin" size={22} />
							<span>立即購買</span>
						</Button>
					</>
				)}
			</div>
		</div>
	);
}
