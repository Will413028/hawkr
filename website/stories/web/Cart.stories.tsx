import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { expect, fn, mocked, userEvent, waitFor, within } from "storybook/test";
import { ViewTransitions } from "@/components/shared/view-transitions";
import Button from "@/components/ui/Button";
import {
	cartSellerKeyOf,
	selectCartItem,
	selectCartSeller,
} from "@/features/cart/cartSelection";
import { AddressPickerSheet } from "@/features/cart/components/AddressPickerSheet";
import { CartPageHeader } from "@/features/cart/components/CartPageHeader";
import { CartProductTile } from "@/features/cart/components/CartProductTile";
import { CouponSelectionSheet } from "@/features/cart/components/CouponSelectionSheet";
import { SellerGroupCard } from "@/features/cart/components/SellerGroupCard";
import { ShippingMethodGroup } from "@/features/cart/components/ShippingMethodGroup";
import { StorePickupPickerSheet } from "@/features/cart/components/StorePickupPickerSheet";
import { type Address, addressText } from "@/lib/addresses/repo";
import type { CartItem } from "@/lib/cart";
import type { AvailableShippingOption } from "@/lib/checkout/repo";
import type { StorePickup } from "@/lib/checkout/store-pickups";
import {
	ShippingFeePayer,
	ShippingMethod,
	ShippingTemperature,
} from "@/lib/codemeta/enums";
import type { CartAvailableCoupon } from "@/lib/coupons/repo";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import "@/styles/features/cart.css";
import "@/styles/components/sheet.css";

declare const __STORYBOOK_BASE_PATH__: string;

// Local, deliberately labelled fixtures. No cart, account, checkout or map API.
const products: CartItem[] = [
	{
		id: "preview-rice",
		skuId: "preview-rice",
		name: "預覽商品・花蓮小農香米",
		price: "NT$ 280",
		priceValue: 280,
		listPrice: 320,
		quantity: 2,
		stockQty: 8,
		unit: "1 公斤／包",
	},
	{
		id: "preview-fish",
		skuId: "preview-fish",
		name: "預覽商品・東海岸鮮凍去刺魚片家庭分享組・獨立真空包裝",
		price: "NT$ 1,280",
		priceValue: 1280,
		quantity: 1,
		stockQty: 3,
		unit: "200 公克 × 6 包／組",
	},
];

const unavailable: CartItem = {
	...products[0],
	id: "preview-unavailable",
	skuId: "preview-unavailable",
	name: "預覽商品・本週限定蔬菜箱",
	quantity: 1,
	stockQty: 0,
	purchasable: false,
	unavailableReason: "OUT_OF_STOCK",
};

const previewSellers = [
	{
		sellerStoreId: "preview-seller-a",
		sellerName: "預覽店家・花蓮食材選物",
		sellerCity: "花蓮縣",
		sellerDistrict: "壽豐鄉",
	},
	{
		sellerStoreId: "preview-seller-b",
		sellerName: "預覽店家・範例蔬果合作社",
		sellerCity: "臺中市",
		sellerDistrict: "后里區",
	},
];
const sellerProducts: CartItem[] = [
	{
		...products[0],
		...previewSellers[0],
		id: "preview-a-rice",
		skuId: "preview-a-rice",
	},
	{
		...products[1],
		...previewSellers[0],
		id: "preview-a-fish",
		skuId: "preview-a-fish",
	},
	{
		...products[0],
		...previewSellers[1],
		id: "preview-b-produce",
		skuId: "preview-b-produce",
		name: "預覽商品・當季蔬果家庭箱",
	},
	{
		...products[1],
		...previewSellers[1],
		id: "preview-b-eggs",
		skuId: "preview-b-eggs",
		name: "預覽商品・冷藏放牧雞蛋十入盒",
	},
];

function guestSellerPreview() {
	// Only the identity hook is stubbed; SellerGroupCard, query setup and
	// selection helpers stay real. isLoggedIn=false disables shipping queries.
	const identity = mocked(useQueryIdentity);
	identity.mockReturnValue({ accountId: null, activeRole: null });
	return () => identity.mockRestore();
}

function SellerSelectionPreview() {
	const [queryClient] = useState(
		() =>
			new QueryClient({
				defaultOptions: { queries: { retry: false, gcTime: 0 } },
			}),
	);
	const [items, setItems] = useState(sellerProducts);
	const [selected, setSelected] = useState<Record<string, boolean>>({
		"preview-a-rice": true,
	});
	const [drafts, setDrafts] = useState<Record<string, string>>({});
	const [notice, setNotice] = useState("");
	const activeItem = items.find((item) => selected[item.id]);
	const activeSeller = activeItem ? cartSellerKeyOf(activeItem) : null;
	function removeItems(ids: string[], message: string) {
		const removed = new Set(ids);
		setItems((current) => current.filter((item) => !removed.has(item.id)));
		setSelected((current) =>
			Object.fromEntries(
				Object.entries(current).filter(([id]) => !removed.has(id)),
			),
		);
		setNotice(message);
	}
	function updateQuantity(item: CartItem, next: number) {
		const quantity = Math.max(0, Math.min(item.stockQty ?? 999, next));
		if (quantity === 0) removeItems([item.id], `已移除 ${item.name}`);
		else
			setItems((current) =>
				current.map((row) => (row.id === item.id ? { ...row, quantity } : row)),
			);
		setDrafts((current) => {
			const nextDrafts = { ...current };
			delete nextDrafts[item.id];
			return nextDrafts;
		});
	}
	return (
		<QueryClientProvider client={queryClient}>
			<div className="catalog-stack" style={{ maxWidth: 1000 }}>
				{previewSellers.map((seller) => {
					const groupItems = items.filter(
						(item) => cartSellerKeyOf(item) === seller.sellerStoreId,
					);
					if (groupItems.length === 0) return null;
					return (
						<SellerGroupCard
							key={seller.sellerStoreId}
							group={{
								...seller,
								items: groupItems,
								subtotal: groupItems.reduce(
									(sum, item) => sum + item.priceValue * item.quantity,
									0,
								),
							}}
							groupKey={seller.sellerStoreId}
							isLocked={
								activeSeller !== null && activeSeller !== seller.sellerStoreId
							}
							isLoggedIn={false}
							selected={selected}
							preferredMethod={ShippingMethod.TAQBIN}
							editingQty={drafts}
							onToggleGroup={(key, next) =>
								setSelected((current) =>
									selectCartSeller(items, current, key, next),
								)
							}
							onToggleItem={(item, next) =>
								setSelected((current) =>
									selectCartItem(items, current, item, next),
								)
							}
							onChangeQty={(id, raw) =>
								setDrafts((current) => ({ ...current, [id]: raw }))
							}
							onCommitQty={(id) => {
								const item = items.find((row) => row.id === id);
								if (item)
									updateQuantity(item, Number(drafts[id] ?? item.quantity));
							}}
							onUpdateQty={updateQuantity}
							onSaveForLater={(item) =>
								removeItems([item.id], `已將 ${item.name} 移至稍後購買`)
							}
							onRemove={(item) => removeItems([item.id], `已移除 ${item.name}`)}
						/>
					);
				})}
				<p role="status" className="cart-type-support">
					{notice}
				</p>
			</div>
		</QueryClientProvider>
	);
}

const addresses: Address[] = [
	{
		id: "preview-home",
		etag: "preview",
		receiverName: "預覽收件人",
		receiverPhone: "0900000000",
		townCode: "63000050",
		addressDetail: "預覽路一段 100 號 12 樓（僅供元件預覽）",
		isDefault: true,
	},
	{
		id: "preview-office",
		etag: "preview",
		receiverName: "預覽辦公室",
		receiverPhone: "0900000001",
		townCode: "65000010",
		addressDetail:
			"預覽大道二段 200 巷 12 弄 8 號 15 樓之 2，請交由管理室代收（僅供元件預覽）",
		isDefault: false,
	},
];

const shippingOptions: AvailableShippingOption[] = [
	{
		shippingMethod: ShippingMethod.TAQBIN,
		deliveryTemp: ShippingTemperature.CHILLED,
		label: "冷藏宅配",
		fee: 205,
		feePayer: ShippingFeePayer.SELLER,
		freeShippingThreshold: 2000,
		freeShippingEligible: false,
	},
	{
		shippingMethod: ShippingMethod.TAQBIN,
		deliveryTemp: ShippingTemperature.FROZEN,
		label: "冷凍宅配",
		fee: 290,
		freeShippingEligible: false,
	},
	{
		shippingMethod: ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP,
		deliveryTemp: ShippingTemperature.AMBIENT,
		label: "常溫超商取貨",
		fee: 65,
		freeShippingEligible: false,
	},
];

const pickup: StorePickup = {
	id: "preview-pickup",
	receiverName: "預覽人",
	receiverPhone: "0900000000",
	storeId: "000000",
	storeName: "預覽門市・長門市名稱閱讀檢查",
	storeAddress: "臺北市中正區預覽路一段 100 號 1 樓（僅供元件預覽）",
	isDefault: true,
};

const coupons: CartAvailableCoupon[] = [
	{
		id: 1,
		templateId: "preview-coupon-available",
		name: "預覽優惠券・訂單折抵",
		kind: "cash",
		discountAmount: 100,
		estimatedDiscount: 100,
		thresholdAmount: 1000,
		expires_at: Date.UTC(2026, 11, 31),
	},
	{
		id: 2,
		templateId: "preview-coupon-unavailable",
		name: "預覽優惠券・滿額門檻尚未達到",
		kind: "cash",
		discountAmount: 200,
		estimatedDiscount: 0,
		thresholdAmount: 3000,
		expires_at: Date.UTC(2026, 11, 31),
	},
];

function fixedPreviewDate() {
	const now = Date.now;
	Date.now = () => Date.UTC(2026, 8, 12, 4);
	return () => {
		Date.now = now;
	};
}

// The production pickup sheet owns its map request. Keep even manually
// clicking that control inside this preview's offline boundary.
function offlinePickupBoundary() {
	const originalFetch = window.fetch;
	window.fetch = (input, init) => {
		const url = new URL(
			input instanceof Request ? input.url : String(input),
			window.location.href,
		);
		if (
			url.origin !== window.location.origin ||
			/\/(?:app|api|auth)\//.test(url.pathname)
		) {
			return Promise.resolve(
				new Response(
					JSON.stringify({ message: "元件預覽不會連線至門市地圖。" }),
					{
						status: 503,
						headers: { "Content-Type": "application/json" },
					},
				),
			);
		}
		return originalFetch(input, init);
	};
	return () => {
		window.fetch = originalFetch;
	};
}

function PickupPreview() {
	const [open, setOpen] = useState(false);
	const [record, setRecord] = useState(pickup);
	return (
		<>
			<Button size="lg" onClick={() => setOpen(true)}>
				編輯預覽取貨人
			</Button>
			<p role="status" className="catalog-note">
				目前取貨人：{record.receiverName}
			</p>
			<StorePickupPickerSheet
				open={open}
				pickups={[record]}
				selectedId={record.id}
				editTarget={record}
				onSelect={() => {}}
				onCreate={async (body) => setRecord({ ...body, id: record.id })}
				onUpdate={async (id, body) => setRecord({ ...body, id })}
				onClose={() => setOpen(false)}
			/>
		</>
	);
}

function CouponPreview() {
	const [open, setOpen] = useState(false);
	const [selected, setSelected] = useState<CartAvailableCoupon | null>(null);
	return (
		<>
			<Button size="lg" onClick={() => setOpen(true)}>
				選擇預覽優惠券
			</Button>
			<p role="status" className="catalog-note">
				{selected ? `已選擇：${selected.name}` : "尚未選擇優惠券"}
			</p>
			<CouponSelectionSheet
				open={open}
				coupons={coupons}
				selectedTemplateId={selected?.templateId ?? null}
				orderAmount={1500}
				shippingFee={205}
				loading={false}
				onSelect={(coupon) => {
					setSelected(coupon);
					setOpen(false);
				}}
				onClear={() => setSelected(null)}
				onRetry={() => {}}
				onClose={() => setOpen(false)}
			/>
		</>
	);
}

function ProductRows({ items = products }: { items?: CartItem[] }) {
	const [rows, setRows] = useState(items);
	const [selected, setSelected] = useState(
		() => new Set(items.map((item) => item.id)),
	);
	const [drafts, setDrafts] = useState<Record<string, string | undefined>>({});
	const [notice, setNotice] = useState("");
	function changeQuantity(item: CartItem, next: number) {
		const quantity = Math.max(0, Math.min(item.stockQty ?? 999, next));
		setRows((current) =>
			current.flatMap((row) =>
				row.id !== item.id ? [row] : quantity ? [{ ...row, quantity }] : [],
			),
		);
		setDrafts((current) => ({ ...current, [item.id]: undefined }));
	}
	return (
		<div className="cart-page__seller-group">
			<ul aria-label="購物車商品" style={{ padding: 0, margin: 0 }}>
				{rows.map((item) => (
					<CartProductTile
						key={item.id}
						item={item}
						isChecked={selected.has(item.id)}
						editingValue={drafts[item.id]}
						onCheck={(checked) =>
							setSelected((current) => {
								const next = new Set(current);
								if (checked) next.add(item.id);
								else next.delete(item.id);
								return next;
							})
						}
						onChangeQty={(raw) =>
							setDrafts((current) => ({ ...current, [item.id]: raw }))
						}
						onCommitQty={() =>
							changeQuantity(item, Number(drafts[item.id] ?? item.quantity))
						}
						onIncrement={() => changeQuantity(item, item.quantity + 1)}
						onDecrement={() => changeQuantity(item, item.quantity - 1)}
						onSaveForLater={() => {
							setRows((current) => current.filter((row) => row.id !== item.id));
							setNotice(`已將 ${item.name} 移至稍後購買`);
						}}
						onRemove={() => {
							setRows((current) => current.filter((row) => row.id !== item.id));
							setNotice(`已移除 ${item.name}`);
						}}
					/>
				))}
			</ul>
			<p role="status" className="catalog-note">
				{notice}
			</p>
		</div>
	);
}

function ShippingAndAddressPreview() {
	const [method, setMethod] = useState<string>(ShippingMethod.TAQBIN);
	const [temperature, setTemperature] = useState<string>(
		ShippingTemperature.CHILLED,
	);
	const [selectedAddress, setSelectedAddress] = useState<string | null>(null);
	const [open, setOpen] = useState(false);
	const [pickup, setPickup] = useState(false);
	const address = addresses.find((value) => value.id === selectedAddress);
	return (
		<>
			<ShippingMethodGroup
				options={shippingOptions}
				selectedMethod={method}
				selectedDeliveryTemp={temperature}
				onSelect={(nextMethod, nextTemperature) => {
					setMethod(nextMethod);
					setTemperature(nextTemperature);
				}}
				addressLabel={
					address ? `${address.receiverName}・${addressText(address)}` : null
				}
				storePickupLabel={pickup ? "預覽門市・臺北市中正區預覽路 100 號" : null}
				onPickAddress={() => setOpen(true)}
				onPickStorePickup={() => setPickup(true)}
				addressRequired
				storePickupRequired
			/>
			<AddressPickerSheet
				open={open}
				addresses={addresses}
				selectedId={selectedAddress}
				onSelect={setSelectedAddress}
				onClose={() => setOpen(false)}
			/>
		</>
	);
}

const meta = {
	title: "Web/Cart/Cart",
	component: CartProductTile,
	parameters: {
		docs: {
			description: {
				component:
					"正式購物車商品列、配送方式及地址抽屜。僅使用本地預覽資料，操作不會呼叫購物車、結帳或地址 API。",
			},
		},
	},
	decorators: [
		(Story) => (
			<ViewTransitions>
				<div className="cart-typography" style={{ maxWidth: 1000 }}>
					<Story />
				</div>
			</ViewTransitions>
		),
	],
} satisfies Meta;
export default meta;
type Story = StoryObj;

export const Products: Story = {
	name: "商品列・數量與選取",
	render: () => <ProductRows />,
	play: async ({ canvas, canvasElement }) => {
		const name = products[0].name;
		const increase = canvas.getByRole("button", { name: `增加 ${name} 數量` });
		for (const button of canvasElement.querySelectorAll(
			".cart-tile__qty button, .cart-tile__actions button",
		)) {
			// Fractional transforms can report 43.9999px after a 44px control settles.
			await waitFor(() => {
				const rect = button.getBoundingClientRect();
				expect(rect.height).toBeGreaterThanOrEqual(43.99);
				expect(rect.width).toBeGreaterThanOrEqual(43.99);
			});
		}
		await userEvent.click(increase);
		await waitFor(() =>
			expect(
				canvas.getByRole("spinbutton", { name: `${name} 數量` }),
			).toHaveValue(3),
		);
		await userEvent.click(
			canvas.getByRole("button", { name: `減少 ${name} 數量` }),
		);
		await waitFor(() =>
			expect(
				canvas.getByRole("spinbutton", { name: `${name} 數量` }),
			).toHaveValue(2),
		);
		const checkbox = canvas.getByRole("checkbox", { name: `選擇 ${name}` });
		checkbox.focus();
		await expect(checkbox).toHaveFocus();
		await userEvent.keyboard(" ");
		await expect(checkbox).not.toBeChecked();
		await userEvent.keyboard(" ");
		await expect(checkbox).toBeChecked();
	},
};

async function expectProductTextScale(
	canvasElement: HTMLElement,
	scale: number,
) {
	await waitFor(() =>
		expect(
			canvasElement.ownerDocument.documentElement.style.getPropertyValue(
				"--hk-text-scale",
			),
		).toBe(String(scale)),
	);
	const productName =
		canvasElement.querySelector<HTMLElement>(".cart-tile__name");
	const specification =
		canvasElement.querySelector<HTMLElement>(".cart-tile__spec");
	const quantity = canvasElement.querySelector<HTMLInputElement>(
		".cart-tile__qty input",
	);
	await expect(productName).not.toBeNull();
	await expect(specification).not.toBeNull();
	await expect(quantity).not.toBeNull();
	if (!productName || !specification || !quantity) return;
	await waitFor(() => {
		expect(
			Number.parseFloat(getComputedStyle(productName).fontSize),
		).toBeCloseTo(16 * scale, 1);
		expect(
			Number.parseFloat(getComputedStyle(specification).fontSize),
		).toBeCloseTo(14 * scale, 1);
		expect(Number.parseFloat(getComputedStyle(quantity).fontSize)).toBeCloseTo(
			16 * scale,
			1,
		);
	});
	for (const text of canvasElement.querySelectorAll<HTMLElement>(
		".cart-tile__name, .cart-tile__spec, .cart-tile__actions",
	)) {
		await expect(text.scrollWidth).toBeLessThanOrEqual(text.clientWidth + 1);
	}
}

export const ProductsSmallText: Story = {
	...Products,
	name: "小字級・0.875 倍",
	globals: { textScale: "0.875" },
	play: async (context) => {
		await Products.play?.(context);
		await expectProductTextScale(context.canvasElement, 0.875);
	},
};

export const ProductsLargeText: Story = {
	...Products,
	name: "大字級・1.125 倍",
	globals: { textScale: "1.125" },
	play: async (context) => {
		await Products.play?.(context);
		await expectProductTextScale(context.canvasElement, 1.125);
	},
};

export const Typography: Story = {
	name: "字級層級・標題、內文與金額",
	render: () => (
		<section
			className="catalog-stack"
			style={{ maxWidth: 1000 }}
			aria-label="購物車字級層級"
		>
			<header>
				<h1 className="cart-type-page-title">購物車</h1>
				<p className="cart-type-body">確認選購商品，再選擇適合的配送方式。</p>
				<p className="cart-type-support">固定預覽資料；操作只會更新此畫面。</p>
			</header>
			<section aria-label="商品清單">
				<h2 className="cart-type-section-title">商品清單</h2>
				<ProductRows />
			</section>
			<div>
				<p className="cart-type-support">金額字級範例</p>
				<p className="cart-type-total">NT$ 1,840</p>
			</div>
		</section>
	),
	play: async ({ canvasElement, globals }) => {
		const scale = Number(globals.textScale ?? 1);
		await expectProductTextScale(canvasElement, scale);
		const typography = [
			[".cart-type-page-title", 22, 28],
			[".cart-type-section-title", 22, 28],
			[".cart-type-body", 16, 24],
			[".cart-type-support", 14, 20],
			[".cart-type-total", 24, 32],
		] as const;
		for (const [selector, fontSize, lineHeight] of typography) {
			const element = canvasElement.querySelector<HTMLElement>(selector);
			await expect(element).not.toBeNull();
			if (!element) continue;
			await waitFor(() => {
				const style = getComputedStyle(element);
				expect(Number.parseFloat(style.fontSize)).toBeCloseTo(
					fontSize * scale,
					1,
				);
				expect(Number.parseFloat(style.lineHeight)).toBeCloseTo(
					lineHeight * scale,
					1,
				);
			});
		}
	},
};

function layoutBounds(root: HTMLElement, selector: string) {
	const element = root.querySelector(selector);
	if (!element) throw new Error(`Missing cart layout element: ${selector}`);
	return element.getBoundingClientRect();
}

async function expectQuantityBesidePhoto(root: HTMLElement) {
	for (const tile of root.querySelectorAll<HTMLElement>(".cart-tile")) {
		const photo = layoutBounds(tile, ".cart-tile__thumb");
		const quantity = layoutBounds(tile, ".cart-tile__qty");
		await expect(quantity.left).toBeGreaterThanOrEqual(photo.right);
		await expect(quantity.top).toBeLessThan(photo.bottom);
		await expect(quantity.bottom).toBeGreaterThan(photo.top);
		await expect(tile.scrollWidth).toBeLessThanOrEqual(tile.clientWidth + 1);
	}
}

async function expectSellerLinkInHeader(root: HTMLElement) {
	for (const header of root.querySelectorAll<HTMLElement>(
		".cart-page__seller-header",
	)) {
		const link = within(header).getByRole("link", { name: "賣家商店" });
		const name = layoutBounds(header, ".cart-page__seller-name");
		const target = link.getBoundingClientRect();
		await expect(target.left).toBeGreaterThanOrEqual(name.right);
		await expect(target.top).toBeLessThan(name.bottom);
		await expect(target.bottom).toBeGreaterThan(name.top);
		await expect(target.height).toBeGreaterThanOrEqual(43.99);
	}
}

export const NarrowLargeText: Story = {
	name: "320px・長品名與放大字級",
	globals: { textScale: "1.4" },
	render: () => (
		<div style={{ width: 320, maxWidth: "100%" }}>
			<ProductRows items={[...products, unavailable]} />
		</div>
	),
	play: async ({ canvasElement }) => {
		await waitFor(() =>
			expect(
				canvasElement.ownerDocument.documentElement.style.getPropertyValue(
					"--hk-text-scale",
				),
			).toBe("1.4"),
		);
		await expectQuantityBesidePhoto(canvasElement);
		for (const tile of canvasElement.querySelectorAll<HTMLElement>(
			".cart-tile",
		)) {
			await expect(tile.scrollWidth).toBeLessThanOrEqual(tile.clientWidth + 1);
			for (const control of tile.querySelectorAll<HTMLElement>(
				".cart-tile__qty, .cart-tile__total, .cart-tile__actions",
			)) {
				await expect(control.getBoundingClientRect().right).toBeLessThanOrEqual(
					tile.getBoundingClientRect().right + 1,
				);
			}
		}
	},
};

export const ProductsDark: Story = {
	...Products,
	name: "深色購物車・文字與操作",
	globals: { theme: "dark" },
};

export const SellerSelection: Story = {
	name: "依店家選取・切店與逐項移除",
	beforeEach: guestSellerPreview,
	render: () => <SellerSelectionPreview />,
	play: async ({ canvas, canvasElement }) => {
		await expectSellerLinkInHeader(canvasElement);
		const firstSeller = previewSellers[0];
		const secondSeller = previewSellers[1];
		const first = within(
			canvas.getByRole("region", { name: `${firstSeller.sellerName}購物車` }),
		);
		const secondRegion = canvas.getByRole("region", {
			name: `${secondSeller.sellerName}購物車`,
		});
		const second = within(secondRegion);
		const firstGroup = first.getByRole("checkbox", {
			name: `選取${firstSeller.sellerName}的全部商品`,
		});
		await expect(firstGroup).toBePartiallyChecked();
		await userEvent.click(firstGroup);
		await expect(firstGroup).toBeChecked();
		await expect(getComputedStyle(secondRegion).opacity).toBe("1");
		const inactiveRow = within(
			second
				.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` })
				.closest("li") as HTMLElement,
		);
		await expect(inactiveRow.getByRole("spinbutton")).toBeEnabled();
		await userEvent.click(
			inactiveRow.getByRole("button", {
				name: `增加 ${sellerProducts[2].name} 數量`,
			}),
		);
		await expect(inactiveRow.getByRole("spinbutton")).toHaveValue(3);
		await expect(firstGroup).toBeChecked();
		await userEvent.click(
			inactiveRow.getByRole("button", {
				name: `減少 ${sellerProducts[2].name} 數量`,
			}),
		);
		await expect(inactiveRow.getByRole("spinbutton")).toHaveValue(2);
		await expect(
			inactiveRow.getByRole("button", { name: "移除" }),
		).toBeEnabled();
		const saveLater = inactiveRow.getByRole("button", { name: "稍後買" });
		await expect(saveLater).toBeEnabled();
		// This also rejects pointer-events:none on an inactive seller card.
		await userEvent.hover(saveLater);
		const switchStore = second.getByRole("button", { name: "改選本店商品" });
		switchStore.focus();
		await expect(switchStore).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		await expect(
			second.getByRole("checkbox", {
				name: `選取${secondSeller.sellerName}的全部商品`,
			}),
		).toBeChecked();
		await expect(
			second.getByRole("checkbox", {
				name: `選取${secondSeller.sellerName}的全部商品`,
			}),
		).toHaveFocus();
		for (const item of sellerProducts.slice(0, 2)) {
			const checkbox = first.getByRole("checkbox", {
				name: `選擇 ${item.name}`,
			});
			await expect(checkbox).not.toBeChecked();
			await expect(checkbox).toBeDisabled();
		}
		await userEvent.click(
			second.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` }),
		);
		await expect(
			second.getByRole("checkbox", {
				name: `選取${secondSeller.sellerName}的全部商品`,
			}),
		).toBePartiallyChecked();
		await expect(
			canvas.queryByRole("button", { name: /移除.*已選/ }),
		).not.toBeInTheDocument();
		const selectedRow = within(
			second
				.getByRole("checkbox", { name: `選擇 ${sellerProducts[3].name}` })
				.closest("li") as HTMLElement,
		);
		await userEvent.click(selectedRow.getByRole("button", { name: "移除" }));
		await expect(
			second.queryByRole("checkbox", {
				name: `選擇 ${sellerProducts[3].name}`,
			}),
		).not.toBeInTheDocument();
		await expect(
			second.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` }),
		).toBeInTheDocument();
		for (const item of sellerProducts.slice(0, 2))
			await expect(
				first.getByRole("checkbox", { name: `選擇 ${item.name}` }),
			).toBeInTheDocument();
		await expect(canvas.getByRole("status")).toHaveTextContent(
			`已移除 ${sellerProducts[3].name}`,
		);
		// Leave both real seller groups present for visual comparison.
		await userEvent.click(
			first.getByRole("checkbox", {
				name: `選取${firstSeller.sellerName}的全部商品`,
			}),
		);
		await expect(
			first.getByRole("checkbox", {
				name: `選取${firstSeller.sellerName}的全部商品`,
			}),
		).toBeChecked();
		await expect(
			second.getByRole("button", { name: "改選本店商品" }),
		).toBeEnabled();
	},
};

export const SellerSelectionCompact: Story = {
	...SellerSelection,
	name: "手機・商店入口與商品旁數量",
	render: () => (
		<div style={{ width: 390, maxWidth: "100%" }}>
			<SellerSelectionPreview />
		</div>
	),
	play: async (context) => {
		await expectQuantityBesidePhoto(context.canvasElement);
		await SellerSelection.play?.(context);
	},
};

export const SellerSelectionNarrowLargeText: Story = {
	...SellerSelection,
	name: "320px・大字級店家範圍操作",
	globals: { textScale: "1.4" },
	render: () => (
		<div style={{ width: 320, maxWidth: "100%" }}>
			<SellerSelectionPreview />
		</div>
	),
	play: async (context) => {
		await expectQuantityBesidePhoto(context.canvasElement);
		await SellerSelection.play?.(context);
		for (const group of context.canvasElement.querySelectorAll<HTMLElement>(
			".cart-page__seller-group",
		)) {
			await expect(group.scrollWidth).toBeLessThanOrEqual(
				group.clientWidth + 1,
			);
			for (const control of group.querySelectorAll<HTMLElement>(
				".cart-page__seller-switch, .cart-page__shop-link, .cart-page__seller-select",
			)) {
				await waitFor(() =>
					expect(control.getBoundingClientRect().height).toBeGreaterThanOrEqual(
						43.99,
					),
				);
				await expect(control.getBoundingClientRect().right).toBeLessThanOrEqual(
					group.getBoundingClientRect().right + 1,
				);
			}
		}
	},
};

export const UnavailableAndRemove: Story = {
	name: "缺貨商品・可選取與移除",
	render: () => <ProductRows items={[unavailable]} />,
	play: async ({ canvas }) => {
		await expect(canvas.getByRole("spinbutton")).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: `增加 ${unavailable.name} 數量` }),
		).toBeDisabled();
		await expect(canvas.getByRole("checkbox")).toBeEnabled();
		await expect(
			canvas.queryByRole("button", { name: "稍後買" }),
		).not.toBeInTheDocument();
		await userEvent.click(canvas.getByRole("button", { name: "移除" }));
		await expect(canvas.queryByRole("spinbutton")).not.toBeInTheDocument();
		await expect(canvas.getByRole("status")).toHaveTextContent(
			`已移除 ${unavailable.name}`,
		);
	},
};

export const ShippingAndAddress: Story = {
	name: "配送方式・選取長地址",
	beforeEach: fixedPreviewDate,
	render: () => (
		<div style={{ maxWidth: 520 }}>
			<ShippingAndAddressPreview />
		</div>
	),
	play: async ({ canvas, canvasElement }) => {
		await userEvent.click(canvas.getByRole("radio", { name: /冷凍宅配/ }));
		await expect(canvas.getByRole("radio", { name: /冷凍宅配/ })).toBeChecked();
		await expect(
			canvas.getByRole("radio", { name: /冷藏宅配/ }),
		).not.toBeChecked();
		await userEvent.click(canvas.getByRole("button", { name: /收件地址/ }));
		const page = within(canvasElement.ownerDocument.body);
		const dialog = await page.findByRole("dialog", { name: "選擇收件地址" });
		await waitFor(() => expect(dialog).toBeVisible());
		const row = within(dialog).getByRole("button", { name: /預覽辦公室/ });
		row.focus();
		await expect(row).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		await waitFor(() =>
			expect(page.queryByRole("dialog")).not.toBeInTheDocument(),
		);
		await expect(
			canvas.getByRole("button", { name: /收件地址/ }),
		).toHaveTextContent(addresses[1].addressDetail);
		await userEvent.click(canvas.getByRole("button", { name: /收件地址/ }));
		await page.findByRole("dialog", { name: "選擇收件地址" });
		await userEvent.keyboard("{Escape}");
		await waitFor(() =>
			expect(page.queryByRole("dialog")).not.toBeInTheDocument(),
		);
	},
};

export const ShippingNarrowLargeText: Story = {
	...ShippingAndAddress,
	name: "窄版配送方式・放大字級",
	globals: { textScale: "1.4" },
	render: () => (
		<div style={{ width: 320, maxWidth: "100%" }}>
			<ShippingAndAddressPreview />
		</div>
	),
	play: async (context) => {
		await ShippingAndAddress.play?.(context);
		for (const target of context.canvasElement.querySelectorAll<HTMLElement>(
			".shipping-group__target-row, .shipping-group__header",
		)) {
			await waitFor(() =>
				expect(target.getBoundingClientRect().height).toBeGreaterThanOrEqual(
					43.99,
				),
			);
			await expect(target.scrollWidth).toBeLessThanOrEqual(
				target.clientWidth + 1,
			);
		}
	},
};

export const PickupTextAndControls: Story = {
	name: "取貨門市・欄位與操作尺寸",
	beforeEach: offlinePickupBoundary,
	render: () => <PickupPreview />,
	play: async ({ canvas, canvasElement }) => {
		await userEvent.click(
			canvas.getByRole("button", { name: "編輯預覽取貨人" }),
		);
		const page = within(canvasElement.ownerDocument.body);
		const dialog = await page.findByRole("dialog", { name: "選擇取貨門市" });
		const fields = within(dialog);
		const receiver = await fields.findByRole("textbox", { name: /取貨人姓名/ });
		await waitFor(() => expect(receiver).toHaveValue(pickup.receiverName));
		await waitFor(() =>
			expect(receiver.getBoundingClientRect().height).toBeGreaterThanOrEqual(
				47.99,
			),
		);
		const changeStore = fields.getByRole("button", { name: "重新選擇" });
		await waitFor(() =>
			expect(changeStore.getBoundingClientRect().height).toBeGreaterThanOrEqual(
				43.99,
			),
		);
		await userEvent.clear(receiver);
		await userEvent.type(receiver, "新收件人");
		await userEvent.click(fields.getByRole("button", { name: "更新" }));
		await waitFor(() =>
			expect(page.queryByRole("dialog")).not.toBeInTheDocument(),
		);
		await expect(canvas.getByRole("status")).toHaveTextContent(
			"目前取貨人：新收件人",
		);
		// Leave the form open for the actual visual/a11y preview.
		await userEvent.click(
			canvas.getByRole("button", { name: "編輯預覽取貨人" }),
		);
		await page.findByRole("dialog", { name: "選擇取貨門市" });
	},
};

export const CouponSelection: Story = {
	name: "優惠券・可用與未達門檻",
	beforeEach: fixedPreviewDate,
	render: () => <CouponPreview />,
	play: async ({ canvas, canvasElement, globals }) => {
		await userEvent.click(
			canvas.getByRole("button", { name: "選擇預覽優惠券" }),
		);
		const page = within(canvasElement.ownerDocument.body);
		let dialog = await page.findByRole("dialog", { name: "選擇優惠券" });
		const unavailableCoupon = within(dialog).getByRole("button", {
			name: /滿額門檻尚未達到/,
		});
		await expect(unavailableCoupon).toBeDisabled();
		await userEvent.click(
			within(dialog).getByRole("button", { name: /訂單折抵/ }),
		);
		await waitFor(() =>
			expect(page.queryByRole("dialog")).not.toBeInTheDocument(),
		);
		await expect(canvas.getByRole("status")).toHaveTextContent(
			"已選擇：預覽優惠券・訂單折抵",
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "選擇預覽優惠券" }),
		);
		dialog = await page.findByRole("dialog", { name: "選擇優惠券" });
		await expect(
			within(dialog).getByRole("button", { name: /訂單折抵/ }),
		).toHaveAttribute("aria-pressed", "true");
		const clear = within(dialog).getByRole("button", { name: "不使用優惠券" });
		await expect(
			Number.parseFloat(getComputedStyle(clear).fontSize),
		).toBeCloseTo(16 * Number(globals.textScale ?? 1), 1);
		await waitFor(() =>
			expect(clear.getBoundingClientRect().height).toBeGreaterThanOrEqual(
				43.99,
			),
		);
	},
};

export const BuyerApp: Story = {
	name: "買家 App・正式 Flutter 購物車",
	tags: ["!test"],
	parameters: {
		docs: {
			description: {
				story:
					"編譯自買家 App main_catalog.dart 的購物車元件；互動與大字級由 Flutter widget tests 驗證。",
			},
		},
	},
	render: () => (
		<iframe
			className="catalog-app-frame"
			title="買家 App・正式 Flutter 購物車"
			src={`${__STORYBOOK_BASE_PATH__}flutter/buyer/index.html?component=cart&textScale=1&embedded=true`}
			width={390}
			height={940}
			style={{ width: 390, maxWidth: "100%" }}
		/>
	),
};

export const BuyerAppLargeText: Story = {
	...BuyerApp,
	name: "買家 App・320px 放大字級",
	render: () => (
		<iframe
			className="catalog-app-frame"
			title="買家 App・320px 放大字級購物車"
			src={`${__STORYBOOK_BASE_PATH__}flutter/buyer/index.html?component=cart&textScale=1.4&embedded=true`}
			width={320}
			height={1040}
			style={{ width: 320, maxWidth: "100%" }}
		/>
	),
};

export const PageHeader: StoryObj<typeof CartPageHeader> = {
	name: "頁首・標題與清空同列",
	args: { onClear: fn() },
	render: (args) => <CartPageHeader onClear={args.onClear} />,
	play: async ({ canvas, canvasElement, args }) => {
		const title = canvas.getByRole("heading", { name: "購物車", level: 1 });
		const clear = canvas.getByRole("button", {
			name: "清空購物車",
		});
		const titleBox = title.getBoundingClientRect();
		const clearBox = clear.getBoundingClientRect();
		await expect(clearBox.left).toBeGreaterThanOrEqual(titleBox.right);
		await expect(clearBox.top).toBeLessThan(titleBox.bottom);
		await expect(clearBox.bottom).toBeGreaterThan(titleBox.top);
		await expect(clearBox.height).toBeGreaterThanOrEqual(44);
		const header = canvasElement.querySelector("header");
		if (!header) throw new Error("Missing cart page header");
		await expect(header.scrollWidth).toBeLessThanOrEqual(
			header.clientWidth + 1,
		);
		await expect(header.querySelector("p")).toBeNull();
		await userEvent.click(clear);
		await expect(args.onClear).toHaveBeenCalledOnce();
	},
};

export const PageHeaderNarrowLargeText: StoryObj<typeof CartPageHeader> = {
	...PageHeader,
	name: "320px 大字・清空與標題同列",
	globals: { textScale: "1.4" },
	render: (args) => (
		<div style={{ width: 320, maxWidth: "100%" }}>
			<CartPageHeader onClear={args.onClear} />
		</div>
	),
};
