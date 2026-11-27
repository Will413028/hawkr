import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { screen, within } from "@testing-library/dom";
import { cleanup } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { ViewTransitions } from "@/components/shared/view-transitions";
import Button from "@/components/ui/Button";
import {
	cartSellerKeyOf,
	selectCartItem,
	selectCartSeller,
} from "@/features/cart/cartSelection";
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
import {
	expectAbsent,
	expectTextContains,
	required,
	waitFor,
} from "@/tests/browser/dom";
import { renderWithIntl } from "@/tests/browser/render";
import "@/styles/features/cart.css";
import "@/styles/components/sheet.css";
import { AddressPickerSheet } from "./AddressPickerSheet";
import { CartPageHeader } from "./CartPageHeader";
import { CartProductTile } from "./CartProductTile";
import { CouponSelectionSheet } from "./CouponSelectionSheet";
import { SellerGroupCard } from "./SellerGroupCard";
import { ShippingMethodGroup } from "./ShippingMethodGroup";
import { StorePickupPickerSheet } from "./StorePickupPickerSheet";

vi.mock("@/lib/intl/use-router", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/intl/use-router")>();
	const { staticRouter } = await import("@/tests/browser/static-router");
	return { ...actual, useRouter: () => staticRouter };
});
vi.mock("@/lib/hooks/useQueryIdentity", { spy: true });

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
	const identity = vi.mocked(useQueryIdentity);
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
							storeView={{
								id: seller.sellerStoreId,
								name: seller.sellerName,
								logoUrl: "",
								href: `/sellers/${seller.sellerStoreId}`,
							}}
							groupKey={seller.sellerStoreId}
							isLocked={
								activeSeller !== null && activeSeller !== seller.sellerStoreId
							}
							signedIn={false}
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

const cleanups: Array<() => void> = [];
afterEach(() => {
	cleanup();
	while (cleanups.length > 0) cleanups.pop()?.();
});
beforeEach(() => {
	expect(screen.queryByRole("dialog")).toBeNull();
});

function renderCart(
	ui: React.ReactNode,
	options: { textScale?: number; theme?: "light" | "dark" } = {},
) {
	return renderWithIntl(
		<ViewTransitions>
			<div className="cart-typography" style={{ maxWidth: 1000 }}>
				{ui}
			</div>
		</ViewTransitions>,
		{ textScale: options.textScale ?? 1, theme: options.theme },
	);
}

function narrow(width: number, ui: React.ReactNode) {
	return <div style={{ width, maxWidth: "100%" }}>{ui}</div>;
}

const dialogOpts = { timeout: 5000 };
const findDialog = (name: string | RegExp) =>
	screen.findByRole("dialog", { name }, dialogOpts);

async function expectMinHeight(element: Element, min: number) {
	await waitFor(() =>
		expect(element.getBoundingClientRect().height).toBeGreaterThanOrEqual(min),
	);
}

async function playProducts(container: HTMLElement) {
	const canvas = within(container);
	const name = products[0].name;
	const increase = canvas.getByRole("button", { name: `增加 ${name} 數量` });
	for (const button of container.querySelectorAll(
		".cart-tile__qty button, .cart-tile__actions button",
	)) {
		await waitFor(() => {
			const rect = button.getBoundingClientRect();
			expect(rect.height).toBeGreaterThanOrEqual(43.99);
			expect(rect.width).toBeGreaterThanOrEqual(43.99);
		});
	}
	await userEvent.click(increase);
	await expect
		.element(canvas.getByRole("spinbutton", { name: `${name} 數量` }))
		.toHaveValue(3);
	await userEvent.click(
		canvas.getByRole("button", { name: `減少 ${name} 數量` }),
	);
	await expect
		.element(canvas.getByRole("spinbutton", { name: `${name} 數量` }))
		.toHaveValue(2);
	const checkbox = canvas.getByRole("checkbox", { name: `選擇 ${name}` });
	checkbox.focus();
	await expect.element(checkbox).toHaveFocus();
	await userEvent.keyboard(" ");
	await expect.element(checkbox).not.toBeChecked();
	await userEvent.keyboard(" ");
	await expect.element(checkbox).toBeChecked();
}

async function expectProductTextScale(container: HTMLElement, scale: number) {
	await waitFor(() =>
		expect(
			document.documentElement.style.getPropertyValue("--hk-text-scale"),
		).toBe(String(scale)),
	);
	const productName = required(
		container.querySelector<HTMLElement>(".cart-tile__name"),
		".cart-tile__name",
	);
	const specification = required(
		container.querySelector<HTMLElement>(".cart-tile__spec"),
		".cart-tile__spec",
	);
	const quantity = required(
		container.querySelector<HTMLInputElement>(".cart-tile__qty input"),
		".cart-tile__qty input",
	);
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
	for (const text of container.querySelectorAll<HTMLElement>(
		".cart-tile__name, .cart-tile__spec, .cart-tile__actions",
	)) {
		expect(text.scrollWidth).toBeLessThanOrEqual(text.clientWidth + 1);
	}
}

function layoutBounds(root: HTMLElement, selector: string) {
	const element = root.querySelector(selector);
	if (!element) throw new Error(`Missing cart layout element: ${selector}`);
	return element.getBoundingClientRect();
}

function expectQuantityBesidePhoto(root: HTMLElement) {
	for (const tile of root.querySelectorAll<HTMLElement>(".cart-tile")) {
		const photo = layoutBounds(tile, ".cart-tile__thumb");
		const quantity = layoutBounds(tile, ".cart-tile__qty");
		expect(quantity.left).toBeGreaterThanOrEqual(photo.right);
		expect(quantity.top).toBeLessThan(photo.bottom);
		expect(quantity.bottom).toBeGreaterThan(photo.top);
		expect(tile.scrollWidth).toBeLessThanOrEqual(tile.clientWidth + 1);
	}
}

function expectSellerLinkInHeader(root: HTMLElement) {
	for (const header of root.querySelectorAll<HTMLElement>(
		".cart-page__seller-header",
	)) {
		const link = within(header).getByRole("link", { name: "賣家商店" });
		const name = layoutBounds(header, ".cart-page__seller-name");
		const target = link.getBoundingClientRect();
		expect(target.left).toBeGreaterThanOrEqual(name.right);
		expect(target.top).toBeLessThan(name.bottom);
		expect(target.bottom).toBeGreaterThan(name.top);
		expect(target.height).toBeGreaterThanOrEqual(43.99);
	}
}

async function playSellerSelection(container: HTMLElement) {
	const canvas = within(container);
	expectSellerLinkInHeader(container);
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
	await expect.element(firstGroup).toBePartiallyChecked();
	await userEvent.click(firstGroup);
	await expect.element(firstGroup).toBeChecked();
	expect(getComputedStyle(secondRegion).opacity).toBe("1");
	const inactiveRow = within(
		required(
			second
				.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` })
				.closest("li"),
			"li",
		),
	);
	await expect.element(inactiveRow.getByRole("spinbutton")).toBeEnabled();
	await userEvent.click(
		inactiveRow.getByRole("button", {
			name: `增加 ${sellerProducts[2].name} 數量`,
		}),
	);
	await expect.element(inactiveRow.getByRole("spinbutton")).toHaveValue(3);
	await expect.element(firstGroup).toBeChecked();
	await userEvent.click(
		inactiveRow.getByRole("button", {
			name: `減少 ${sellerProducts[2].name} 數量`,
		}),
	);
	await expect.element(inactiveRow.getByRole("spinbutton")).toHaveValue(2);
	await expect
		.element(inactiveRow.getByRole("button", { name: "移除" }))
		.toBeEnabled();
	const saveLater = inactiveRow.getByRole("button", { name: "稍後買" });
	await expect.element(saveLater).toBeEnabled();
	await userEvent.hover(saveLater);
	const switchStore = second.getByRole("button", { name: "改選本店商品" });
	switchStore.focus();
	await expect.element(switchStore).toHaveFocus();
	await userEvent.keyboard("{Enter}");
	const secondGroup = () =>
		page
			.getByRole("region", {
				name: `${secondSeller.sellerName}購物車`,
				exact: true,
			})
			.getByRole("checkbox", {
				name: `選取${secondSeller.sellerName}的全部商品`,
				exact: true,
			});
	await expect.element(secondGroup()).toBeChecked();
	await expect.element(secondGroup()).toHaveFocus();
	for (const item of sellerProducts.slice(0, 2)) {
		const checkbox = first.getByRole("checkbox", {
			name: `選擇 ${item.name}`,
		});
		await expect.element(checkbox).not.toBeChecked();
		await expect.element(checkbox).toBeDisabled();
	}
	await userEvent.click(
		second.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` }),
	);
	await expect.element(secondGroup()).toBePartiallyChecked();
	await expectAbsent(() =>
		canvas.queryByRole("button", { name: /移除.*已選/ }),
	);
	const selectedRow = within(
		required(
			second
				.getByRole("checkbox", { name: `選擇 ${sellerProducts[3].name}` })
				.closest("li"),
			"li",
		),
	);
	await userEvent.click(selectedRow.getByRole("button", { name: "移除" }));
	await expectAbsent(() =>
		second.queryByRole("checkbox", {
			name: `選擇 ${sellerProducts[3].name}`,
		}),
	);
	await expect
		.element(
			second.getByRole("checkbox", { name: `選擇 ${sellerProducts[2].name}` }),
		)
		.toBeInTheDocument();
	for (const item of sellerProducts.slice(0, 2))
		await expect
			.element(first.getByRole("checkbox", { name: `選擇 ${item.name}` }))
			.toBeInTheDocument();
	await expectTextContains(
		() => canvas.getByRole("status"),
		`已移除 ${sellerProducts[3].name}`,
	);
	await userEvent.click(
		first.getByRole("checkbox", {
			name: `選取${firstSeller.sellerName}的全部商品`,
		}),
	);
	await expect
		.element(
			first.getByRole("checkbox", {
				name: `選取${firstSeller.sellerName}的全部商品`,
			}),
		)
		.toBeChecked();
	await expect
		.element(second.getByRole("button", { name: "改選本店商品" }))
		.toBeEnabled();
}

async function playShippingAndAddress(container: HTMLElement) {
	const canvas = within(container);
	await userEvent.click(canvas.getByRole("radio", { name: /冷凍宅配/ }));
	await expect
		.element(canvas.getByRole("radio", { name: /冷凍宅配/ }))
		.toBeChecked();
	await expect
		.element(canvas.getByRole("radio", { name: /冷藏宅配/ }))
		.not.toBeChecked();
	await userEvent.click(canvas.getByRole("button", { name: /收件地址/ }));
	const dialog = await findDialog("選擇收件地址");
	await expect.element(dialog).toBeVisible();
	const row = within(dialog).getByRole("button", { name: /預覽辦公室/ });
	row.focus();
	await expect.element(row).toHaveFocus();
	await userEvent.keyboard("{Enter}");
	await expectAbsent(() => screen.queryByRole("dialog"));
	await expectTextContains(
		() => canvas.getByRole("button", { name: /收件地址/ }),
		addresses[1].addressDetail,
	);
	await userEvent.click(canvas.getByRole("button", { name: /收件地址/ }));
	await findDialog("選擇收件地址");
	await userEvent.keyboard("{Escape}");
	await expectAbsent(() => screen.queryByRole("dialog"));
}

async function playPageHeader(container: HTMLElement, onClear: () => void) {
	const canvas = within(container);
	const title = canvas.getByRole("heading", { name: "購物車", level: 1 });
	const clear = canvas.getByRole("button", { name: "清空購物車" });
	const titleBox = title.getBoundingClientRect();
	const clearBox = clear.getBoundingClientRect();
	expect(clearBox.left).toBeGreaterThanOrEqual(titleBox.right);
	expect(clearBox.top).toBeLessThan(titleBox.bottom);
	expect(clearBox.bottom).toBeGreaterThan(titleBox.top);
	expect(clearBox.height).toBeGreaterThanOrEqual(44);
	const header = required(container.querySelector("header"), "header");
	expect(header.scrollWidth).toBeLessThanOrEqual(header.clientWidth + 1);
	expect(header.querySelector("p")).toBeNull();
	await userEvent.click(clear);
	expect(onClear).toHaveBeenCalledOnce();
}

describe("Cart products", () => {
	it("changes quantity and toggles selection by keyboard", async () => {
		const { container } = renderCart(<ProductRows />);
		await playProducts(container);
	});

	it("keeps the product row readable at 0.875x text", async () => {
		const { container } = renderCart(<ProductRows />, { textScale: 0.875 });
		await playProducts(container);
		await expectProductTextScale(container, 0.875);
	});

	it("keeps the product row readable at 1.125x text", async () => {
		const { container } = renderCart(<ProductRows />, { textScale: 1.125 });
		await playProducts(container);
		await expectProductTextScale(container, 1.125);
	});

	it("keeps quantity and selection working in the dark theme", async () => {
		const { container } = renderCart(<ProductRows />, { theme: "dark" });
		expect(document.documentElement.dataset.theme).toBe("dark");
		await playProducts(container);
	});

	it("applies the cart typography scale to headings, body and totals", async () => {
		const scale = 1;
		const { container } = renderCart(
			<section
				className="catalog-stack"
				style={{ maxWidth: 1000 }}
				aria-label="購物車字級層級"
			>
				<header>
					<h1 className="cart-type-page-title">購物車</h1>
					<p className="cart-type-body">確認選購商品，再選擇適合的配送方式。</p>
					<p className="cart-type-support">
						固定預覽資料；操作只會更新此畫面。
					</p>
				</header>
				<section aria-label="商品清單">
					<h2 className="cart-type-section-title">商品清單</h2>
					<ProductRows />
				</section>
				<div>
					<p className="cart-type-support">金額字級範例</p>
					<p className="cart-type-total">NT$ 1,840</p>
				</div>
			</section>,
			{ textScale: scale },
		);
		await expectProductTextScale(container, scale);
		const typography = [
			[".cart-type-page-title", 22, 28],
			[".cart-type-section-title", 22, 28],
			[".cart-type-body", 16, 24],
			[".cart-type-support", 14, 20],
			[".cart-type-total", 24, 32],
		] as const;
		for (const [selector, fontSize, lineHeight] of typography) {
			const element = required(
				container.querySelector<HTMLElement>(selector),
				selector,
			);
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
	});

	it("keeps long names and controls inside a 320px tile at 1.4x text", async () => {
		const { container } = renderCart(
			narrow(320, <ProductRows items={[...products, unavailable]} />),
			{ textScale: 1.4 },
		);
		await waitFor(() =>
			expect(
				document.documentElement.style.getPropertyValue("--hk-text-scale"),
			).toBe("1.4"),
		);
		expectQuantityBesidePhoto(container);
		for (const tile of container.querySelectorAll<HTMLElement>(".cart-tile")) {
			expect(tile.scrollWidth).toBeLessThanOrEqual(tile.clientWidth + 1);
			for (const control of tile.querySelectorAll<HTMLElement>(
				".cart-tile__qty, .cart-tile__total, .cart-tile__actions",
			)) {
				expect(control.getBoundingClientRect().right).toBeLessThanOrEqual(
					tile.getBoundingClientRect().right + 1,
				);
			}
		}
	});

	it("disables quantity for an unavailable item but still allows removing it", async () => {
		const { container } = renderCart(<ProductRows items={[unavailable]} />);
		const canvas = within(container);
		await expect.element(canvas.getByRole("spinbutton")).toBeDisabled();
		await expect
			.element(
				canvas.getByRole("button", { name: `增加 ${unavailable.name} 數量` }),
			)
			.toBeDisabled();
		await expect.element(canvas.getByRole("checkbox")).toBeEnabled();
		expect(canvas.queryByRole("button", { name: "稍後買" })).toBeNull();
		await userEvent.click(canvas.getByRole("button", { name: "移除" }));
		await expectAbsent(() => canvas.queryByRole("spinbutton"));
		await expectTextContains(
			() => canvas.getByRole("status"),
			`已移除 ${unavailable.name}`,
		);
	});
});

describe("Cart seller selection", () => {
	beforeEach(() => {
		cleanups.push(guestSellerPreview());
	});

	it("switches seller scope and removes items one by one", async () => {
		const { container } = renderCart(<SellerSelectionPreview />);
		await playSellerSelection(container);
	});

	it("keeps quantity beside the photo on a 390px phone", async () => {
		const { container } = renderCart(narrow(390, <SellerSelectionPreview />));
		expectQuantityBesidePhoto(container);
		await playSellerSelection(container);
	});

	it("keeps seller controls inside the card at 320px and 1.4x text", async () => {
		const { container } = renderCart(narrow(320, <SellerSelectionPreview />), {
			textScale: 1.4,
		});
		expectQuantityBesidePhoto(container);
		await playSellerSelection(container);
		for (const group of container.querySelectorAll<HTMLElement>(
			".cart-page__seller-group",
		)) {
			expect(group.scrollWidth).toBeLessThanOrEqual(group.clientWidth + 1);
			for (const control of group.querySelectorAll<HTMLElement>(
				".cart-page__seller-switch, .cart-page__shop-link, .cart-page__seller-select",
			)) {
				await expectMinHeight(control, 43.99);
				expect(control.getBoundingClientRect().right).toBeLessThanOrEqual(
					group.getBoundingClientRect().right + 1,
				);
			}
		}
	});
});

describe("Cart shipping and address", () => {
	beforeEach(() => {
		cleanups.push(fixedPreviewDate());
	});

	it("selects a shipping method and a long address", async () => {
		const { container } = renderCart(
			<div style={{ maxWidth: 520 }}>
				<ShippingAndAddressPreview />
			</div>,
		);
		await playShippingAndAddress(container);
	});

	it("keeps shipping targets tall and unclipped at 320px and 1.4x text", async () => {
		const { container } = renderCart(
			narrow(320, <ShippingAndAddressPreview />),
			{
				textScale: 1.4,
			},
		);
		await playShippingAndAddress(container);
		for (const target of container.querySelectorAll<HTMLElement>(
			".shipping-group__target-row, .shipping-group__header",
		)) {
			await expectMinHeight(target, 43.99);
			expect(target.scrollWidth).toBeLessThanOrEqual(target.clientWidth + 1);
		}
	});
});

describe("Cart pickup and coupon sheets", () => {
	it("edits the pickup receiver with correctly sized controls", async () => {
		cleanups.push(offlinePickupBoundary());
		const { container } = renderCart(<PickupPreview />);
		const canvas = within(container);
		await userEvent.click(
			canvas.getByRole("button", { name: "編輯預覽取貨人" }),
		);
		const dialog = await findDialog("選擇取貨門市");
		const fields = within(dialog);
		const receiver = await fields.findByRole(
			"textbox",
			{ name: /取貨人姓名/ },
			dialogOpts,
		);
		await expect.element(receiver).toHaveValue(pickup.receiverName);
		await expectMinHeight(receiver, 47.99);
		const changeStore = fields.getByRole("button", { name: "重新選擇" });
		await expectMinHeight(changeStore, 43.99);
		await userEvent.clear(receiver);
		await userEvent.type(receiver, "新收件人");
		await userEvent.click(fields.getByRole("button", { name: "更新" }));
		await expectAbsent(() => screen.queryByRole("dialog"));
		await expectTextContains(
			() => canvas.getByRole("status"),
			"目前取貨人：新收件人",
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "編輯預覽取貨人" }),
		);
		await findDialog("選擇取貨門市");
	});

	it("selects an eligible coupon and disables one under its threshold", async () => {
		cleanups.push(fixedPreviewDate());
		const { container } = renderCart(<CouponPreview />);
		const canvas = within(container);
		await userEvent.click(
			canvas.getByRole("button", { name: "選擇預覽優惠券" }),
		);
		let dialog = await findDialog("選擇優惠券");
		const unavailableCoupon = within(dialog).getByRole("button", {
			name: /滿額門檻尚未達到/,
		});
		await expect.element(unavailableCoupon).toBeDisabled();
		await userEvent.click(
			within(dialog).getByRole("button", { name: /訂單折抵/ }),
		);
		await expectAbsent(() => screen.queryByRole("dialog"));
		await expectTextContains(
			() => canvas.getByRole("status"),
			"已選擇：預覽優惠券・訂單折抵",
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "選擇預覽優惠券" }),
		);
		dialog = await findDialog("選擇優惠券");
		await expect
			.element(within(dialog).getByRole("button", { name: /訂單折抵/ }))
			.toHaveAttribute("aria-pressed", "true");
		const clear = within(dialog).getByRole("button", { name: "不使用優惠券" });
		expect(Number.parseFloat(getComputedStyle(clear).fontSize)).toBeCloseTo(
			16,
			1,
		);
		await expectMinHeight(clear, 43.99);
	});
});

describe("CartPageHeader", () => {
	it("puts the title and the clear action on one row", async () => {
		const onClear = vi.fn();
		const { container } = renderCart(<CartPageHeader onClear={onClear} />);
		await playPageHeader(container, onClear);
	});

	it("keeps the title and the clear action on one row at 320px and 1.4x text", async () => {
		const onClear = vi.fn();
		const { container } = renderCart(
			narrow(320, <CartPageHeader onClear={onClear} />),
			{ textScale: 1.4 },
		);
		await playPageHeader(container, onClear);
	});
});
