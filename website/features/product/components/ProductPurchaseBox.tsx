"use client";

import * as React from "react";
import AddToCartButton from "@/components/shared/AddToCartButton";
import AppIcon from "@/components/shared/AppIcon";
import { emitCartToast } from "@/components/shared/CartSnackbar";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
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
	 * 預覽模式（賣家匯入精靈 Page 5 用）：layout 與真實 PDP 完全一致，
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
	const { isLoggedIn } = useAuth();
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

	// Re-sync selection if the parent swaps the SKU list (e.g. variant
	// switch on the same PDP). Keeps the controlled selection valid.
	React.useEffect(() => {
		if (!skus.length) return;
		if (!skus.some((s) => s.id === selectedSkuId)) {
			setSelectedSkuId(skus[0].id);
		}
	}, [skus, selectedSkuId]);

	const selected = skus.find((s) => s.id === selectedSkuId) ?? skus[0];

	const [buyingNow, setBuyingNow] = React.useState(false);
	// 成功路徑刻意不立即重置 buyingNow：router.push("/cart") 到 commit 之間
	// 本頁仍可互動，立刻重置會開出慢網路下重複加購（數量翻倍）的窗口。改在
	// 路由變化時復位——cacheComponents 保留實例不 remount（PR#221），返回本頁
	// 靠這裡解除 disabled+spinner，兩種卡死/重複情境都關掉。
	const pathname = usePathname() ?? "";
	React.useEffect(() => {
		// 值本身不用，「變化」即復位訊號。
		void pathname;
		setBuyingNow(false);
	}, [pathname]);

	// Out-of-stock SKUs swap the buy buttons for a "已售完" notice + a
	// "補貨通知" subscribe action. Subscription is independent of favouriting:
	// the PDP detail endpoint returns `isRestockSubscribed` separately from
	// `isFavorited`, and the dedicated endpoints
	// `POST/DELETE /app/products/{id}/restock_subscriptions` toggle it.
	// (Previously this re-used the favourites context — that was a UX bug.)
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

	// Resync if PDP swaps the product underneath us (e.g. variant route change).
	// Depending only on initialRestockSubscribed is enough: the parent passes a
	// fresh value with the new productId, so it always changes together.
	React.useEffect(() => {
		setRestockOn(initialRestockSubscribed);
	}, [initialRestockSubscribed]);

	// PDP is fetched via the public (anonymous) cached endpoint so the SSR
	// `isRestockSubscribed` flag is always false for logged-in buyers. Mirror
	// the favourites context pattern: client-side reconcile against the buyer's
	// own subscription list on mount. Page size 50 is enough for typical buyers;
	// if larger lists are ever common the gateway should add a per-product check
	// (`GET /app/products/{id}/restock_subscriptions`) — for now keep it simple.
	// ⚠️ `previewMode` 在這裡是**必要**條件，不是保險。預覽路徑（匯入精靈、代行工作區）
	// 傳進來的 `productId` 不是買家端認得的商品，而這一發是 `/app/*` 的買家端請求。
	// 今天它不會送出，只是因為那兩個預覽面恰好都沒有 www 登入態（`isLoggedIn` 為
	// false）—— 那是巧合不是保證：管理員本人若剛好也在同一個瀏覽器登入過 www，這一發
	// 就會用他自己的買家身分打出去。按鈕全部 disabled 卻仍發請求，正是 previewMode
	// 要防的形狀。
	React.useEffect(() => {
		if (
			previewMode ||
			!isLoggedIn ||
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
			} catch {
				/* keep current state — initial server value is the fallback */
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [
		previewMode,
		isLoggedIn,
		purchaseAvailability.canSubscribeRestock,
		productId,
		initialRestockSubscribed,
	]);

	const handleRestockNotify = async () => {
		if (restockPendingRef.current) return;
		if (!isLoggedIn) {
			openLoginRequired();
			return;
		}
		restockPendingRef.current = true;
		const next = !restockOn;
		setRestockPending(true);
		setRestockOn(next);
		try {
			if (next) {
				await subscribeRestock(productId);
				emitCartToast({
					message: "已開啟補貨通知",
					actionHref: null,
				});
			} else {
				await unsubscribeRestock(productId);
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

	// Defensive guard: products with no SKUs can't be purchased. The page
	// is responsible for hiding / messaging this case; we just no-op.
	if (!selected) return null;

	const unitPrice = selected.price;
	const unitListPrice = selected.listPrice;
	const totalPrice = unitPrice * quantity;
	const totalListPrice = unitListPrice ? unitListPrice * quantity : 0;
	const onSale = unitListPrice != null && unitListPrice > unitPrice;
	const totalSavings = onSale ? totalListPrice - totalPrice : 0;

	const cartItem: CartInput = {
		...item,
		// Cart mutations are keyed by SKU id; productId stays on the record for
		// checkout validation, coupon scope, and analytics.
		id: selected.id,
		productId: item.id,
		skuId: selected.id,
		price: String(unitPrice),
		unit: formatUnitLabel(selected.unit || unit),
		quantity,
	};

	const handleBuyNow = async () => {
		if (buyingNow) return;
		// Guests can't check out — prompt login up front instead of letting the
		// cart API fail with the raw backend "guest_id is required" error. The
		// 401 catch below stays as a fallback for stale/expired sessions.
		if (!isLoggedIn) {
			openLoginRequired();
			return;
		}
		setBuyingNow(true);
		try {
			// Await backend so the cart page sees the new SKU on mount.
			// Previously this was fire-and-forget which could race the cart
			// page's loadCart and leave it showing stale (or empty) state.
			await addToCart(cartItem);
			// Route to /cart, NOT /checkout. Checkout requires the draft
			// written from the cart page (shipping/address/coupon).
			router.push("/cart");
		} catch (err) {
			if (err instanceof CartError && err.status === 401) {
				openLoginRequired();
			} else {
				emitCartToast({ message: buyNowErrorMessage(err), tone: "error" });
			}
			// 失敗留在本頁，pathname 不變 → 這裡明確重置；成功路徑由上方的
			// pathname effect 在路由變化時復位。
			setBuyingNow(false);
		}
	};

	const stock = selected.stock;
	const atMaxQty = typeof stock === "number" && quantity >= stock;

	return (
		<div className="flex flex-col gap-[22px]">
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
								// Reset qty if the new SKU's stock is below current qty.
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
				{/* ⚠️ 這裡的條件**不能**是 `outOfStock && !previewMode`。預覽存在的理由就是
				    「買家會看到什麼」，把不可購買的商品畫成可購買，等於預覽說了一句買家頁
				    不會說的話——而那正是代行者上架前唯一能拿來判斷的東西（#2479）。
				    `previewMode` 真正要擋的是**會發請求的那一顆**（補貨通知打 `/app/*`，
				    見上方 effect 的註解），不是整個不可購買狀態。 */}
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
							// The icon stays through the added state. Swapping to a
							// text-only label pulled the glyph out and let the row
							// re-centre, so a confirmation read as a jolt.
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
