"use client";

/**
 * Chat attachment picker — lets the user pick a 商品 (product) or 訂單
 * (order) from a small list and turn it into a structured chat message
 * (the same JSON payload that `ChatCards.tryParseProduct` /
 * `tryParseOrder` consume on the rendering side).
 *
 * Mirrors the buyer APP's "+" attach menu in chat. Web makes it a
 * single panel with two tabs rather than the APP's stacked sheets
 * because cursor users get to the data faster without an extra tap.
 *
 * Data routing:
 *   - role=buyer (peer is seller): products come from peer's public
 *     catalog (`fetchSellerProducts(peerId)`); orders come from the
 *     buyer's own `/app/orders` filtered to peer's seller id.
 *   - role=seller (peer is buyer): products come from the seller's
 *     own catalog (`/app/products`); orders come from the same
 *     `/app/orders` filtered to peer's buyer id (the endpoint already
 *     returns rows visible to the active role).
 *
 * The picker is a controlled UI — the parent owns `open` state and
 * decides what to do with the resulting payload (typically `send()`).
 */

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { fetchSellerProducts as fetchPublicSellerProducts } from "@/lib/api";
import {
	ORDER_STATUS_LABEL,
	type OrderStatusValue,
} from "@/lib/codemeta/enums";
import { buildImageUrl } from "@/lib/media/cdn";
import { fetchOrders } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { fetchSellerProducts as fetchOwnSellerProducts } from "@/lib/products/seller-repo";

type Role = "buyer" | "seller";

interface AttachmentPickerProps {
	open: boolean;
	onClose: () => void;
	/** Called with the JSON-string payload ready to feed `send()`. */
	onPick: (payload: string) => void;
	/** Active role of the local user — controls which list each tab fetches. */
	role: Role;
	/** Peer user id (other side of the conversation). */
	peerId: string;
}

interface ProductRow {
	id: string;
	name: string;
	price: string;
	imageUrl: string;
	sellerName: string;
}

interface OrderRow {
	orderNo: string;
	statusLabel: string;
	statusCode: string;
	amount: string;
	createdAt: number;
	primaryItemName: string;
	itemCount: number;
	imageUrl: string;
	storeId: string;
	buyerId: string;
	rawItems: Order["items"];
	sellerName: string;
}

/** Neutral fallback for product rows with no usable imageUrl. Replaces
 *  the previous `seller_cover.png` URL — that asset is the *wide brand
 *  banner* (with the "Hawkr" wordmark) and squishing it
 *  into a 44×44 thumb made the watermark visibly leak into the row. */
function ProductThumbFallback() {
	return (
		<span
			className="attachment-picker__thumb attachment-picker__thumb--empty"
			aria-hidden="true"
		>
			<AppIcon name="image" />
		</span>
	);
}

function formatTwd(raw: string | number): string {
	const n =
		typeof raw === "number" ? raw : Number(String(raw).replace(/[,$\s]/g, ""));
	if (!Number.isFinite(n) || n <= 0) return "";
	return `NT$ ${Math.round(n).toLocaleString("zh-TW")}`;
}

function buildProductPayload(row: ProductRow): string {
	return JSON.stringify({
		messageType: "product",
		productId: row.id,
		productName: row.name,
		price: row.price,
		imageUrl: row.imageUrl,
		sellerName: row.sellerName,
	});
}

function buildOrderPayload(row: OrderRow, role: Role): string {
	// `order_prev` = buyer-side preview (rebuy CTA); `order_check` =
	// seller-side or status-check (contact-seller CTA). Mirrors the APP.
	const messageType = role === "buyer" ? "order_prev" : "order_check";
	const items = row.rawItems.slice(0, 5).map((it: Order["items"][number]) => ({
		productName: it.name,
		quantity: String(it.quantity ?? 1),
	}));
	return JSON.stringify({
		messageType,
		orderNo: row.orderNo,
		status: row.statusCode,
		statusLabel: row.statusLabel,
		totalActualAmount: row.amount,
		itemCount: row.itemCount,
		items,
		createTime: row.createdAt,
		imageUrl: row.imageUrl,
		sellerName: row.sellerName,
	});
}

export default function AttachmentPicker({
	open,
	onClose,
	onPick,
	role,
	peerId,
}: AttachmentPickerProps) {
	const [activeTab, setActiveTab] = React.useState<"product" | "order">(
		"product",
	);
	const [products, setProducts] = React.useState<ProductRow[]>([]);
	const [orders, setOrders] = React.useState<OrderRow[]>([]);
	const [loading, setLoading] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	// Fetch when the picker opens or tab changes. Avoid refetching if the
	// list already exists for this tab so reopening the picker mid-session
	// stays instant.
	React.useEffect(() => {
		if (!open) return;
		let cancelled = false;

		async function loadProducts() {
			setLoading(true);
			setError(null);
			try {
				if (role === "buyer") {
					const data = (await fetchPublicSellerProducts(peerId, 1, 30)) as {
						records?: unknown[];
						data?: { records?: unknown[] };
					};
					const records =
						(data?.records as Record<string, unknown>[]) ??
						(data?.data?.records as Record<string, unknown>[]) ??
						[];
					if (cancelled) return;
					setProducts(records.map(normalizePublicProduct));
				} else {
					const result = await fetchOwnSellerProducts({ pageSize: 30 });
					if (cancelled) return;
					setProducts(
						(result.records as unknown as Record<string, unknown>[]).map(
							normalizeOwnProduct,
						),
					);
				}
			} catch (e) {
				if (cancelled) return;
				setError(e instanceof Error ? e.message : "讀取商品失敗");
				setProducts([]);
			} finally {
				if (!cancelled) setLoading(false);
			}
		}

		async function loadOrders() {
			setLoading(true);
			setError(null);
			try {
				const result = await fetchOrders({
					pageSize: 30,
					buyerId: role === "seller" ? peerId : undefined,
				});
				if (cancelled) return;
				const allRows = result.records.map(normalizeOrder);
				// Only show orders that involve the peer — chats are 1:1, so
				// listing unrelated orders would be confusing.
				const filtered = allRows.filter((row) =>
					role === "buyer" ? row.storeId === peerId : row.buyerId === peerId,
				);
				setOrders(peerId ? filtered : allRows);
			} catch (e) {
				if (cancelled) return;
				setError(e instanceof Error ? e.message : "讀取訂單失敗");
				setOrders([]);
			} finally {
				if (!cancelled) setLoading(false);
			}
		}

		if (activeTab === "product" && products.length === 0) {
			void loadProducts();
		} else if (activeTab === "order" && orders.length === 0) {
			void loadOrders();
		}

		return () => {
			cancelled = true;
		};
	}, [open, activeTab, role, peerId, products.length, orders.length]);

	// Close on Esc — keep this picker keyboard-friendly.
	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [open, onClose]);

	if (!open) return null;

	const handlePickProduct = (row: ProductRow) => {
		onPick(buildProductPayload(row));
		onClose();
	};
	const handlePickOrder = (row: OrderRow) => {
		onPick(buildOrderPayload(row, role));
		onClose();
	};

	return (
		<>
			<div
				className="attachment-picker__backdrop"
				onClick={onClose}
				aria-hidden="true"
			/>
			<div
				className="attachment-picker"
				role="dialog"
				aria-modal="true"
				aria-label="附加商品或訂單"
			>
				<header className="attachment-picker__head">
					<div className="attachment-picker__tabs" role="tablist">
						<button
							type="button"
							role="tab"
							aria-selected={activeTab === "product"}
							data-active={activeTab === "product"}
							className="attachment-picker__tab"
							onClick={() => setActiveTab("product")}
						>
							商品
						</button>
						<button
							type="button"
							role="tab"
							aria-selected={activeTab === "order"}
							data-active={activeTab === "order"}
							className="attachment-picker__tab"
							onClick={() => setActiveTab("order")}
						>
							訂單
						</button>
					</div>
					<button
						type="button"
						className="attachment-picker__close"
						onClick={onClose}
						aria-label="關閉"
					>
						✕
					</button>
				</header>

				<div className="attachment-picker__body">
					{loading ? (
						<p className="attachment-picker__hint">讀取中…</p>
					) : error ? (
						<p className="attachment-picker__hint attachment-picker__hint--error">
							{error}
						</p>
					) : activeTab === "product" ? (
						products.length === 0 ? (
							<p className="attachment-picker__hint">
								{role === "buyer"
									? "這位賣家目前沒有可分享的商品。"
									: "你尚未建立任何商品。"}
							</p>
						) : (
							<ul className="attachment-picker__list">
								{products.map((row) => (
									<li key={row.id} className="attachment-picker__row">
										{row.imageUrl ? (
											/* biome-ignore lint/performance/noImgElement: thumbnail from runtime CDN */
											<img
												src={row.imageUrl}
												alt=""
												className="attachment-picker__thumb"
												loading="lazy"
											/>
										) : (
											<ProductThumbFallback />
										)}
										<div className="attachment-picker__meta">
											<strong>{row.name || "未命名商品"}</strong>
											<small>
												{[row.sellerName, formatTwd(row.price)]
													.filter(Boolean)
													.join("｜")}
											</small>
										</div>
										<button
											type="button"
											className="attachment-picker__pick"
											onClick={() => handlePickProduct(row)}
										>
											選擇
										</button>
									</li>
								))}
							</ul>
						)
					) : orders.length === 0 ? (
						<p className="attachment-picker__hint">
							{role === "buyer"
								? "你還沒有與這位賣家的訂單。"
								: "尚未找到與這位買家相關的訂單。"}
						</p>
					) : (
						<ul className="attachment-picker__list">
							{orders.map((row) => (
								<li key={row.orderNo} className="attachment-picker__row">
									<div
										className="attachment-picker__order-stub"
										aria-hidden="true"
									>
										<AppIcon name="box" size={20} />
									</div>
									<div className="attachment-picker__meta">
										<strong>#{row.orderNo}</strong>
										<small>
											{[
												row.primaryItemName +
													(row.itemCount > 1 ? ` 等 ${row.itemCount} 件` : ""),
												row.statusLabel,
												formatTwd(row.amount),
											]
												.filter(Boolean)
												.join("｜")}
										</small>
									</div>
									<button
										type="button"
										className="attachment-picker__pick"
										onClick={() => handlePickOrder(row)}
									>
										選擇
									</button>
								</li>
							))}
						</ul>
					)}
				</div>
			</div>
		</>
	);
}

/* ===== Normalizers ===== */

function pickString(
	obj: Record<string, unknown>,
	keys: readonly string[],
): string {
	for (const k of keys) {
		const v = obj[k];
		if (typeof v === "string" && v.trim()) return v.trim();
		if (typeof v === "number" && Number.isFinite(v)) return String(v);
	}
	return "";
}

function normalizePublicProduct(raw: Record<string, unknown>): ProductRow {
	const imageUrl = buildImageUrl(
		pickString(raw, ["imageUrl", "image_url", "image", "cover", "thumb"]),
		"thumb",
	);
	return {
		id: pickString(raw, ["id", "productId", "product_id"]),
		name: pickString(raw, ["name", "productName", "product_name"]),
		price: pickString(raw, [
			"priceMin",
			"price_min",
			"price",
			"unitPrice",
			"unit_price",
		]),
		imageUrl,
		sellerName: pickString(raw, [
			"sellerName",
			"seller_name",
			"storeName",
			"store_name",
		]),
	};
}

function normalizeOwnProduct(raw: Record<string, unknown>): ProductRow {
	const imageUrl = buildImageUrl(
		pickString(raw, [
			"imageUrl",
			"image_url",
			"coverImage",
			"cover_image",
			"thumb",
		]),
		"thumb",
	);
	return {
		id: pickString(raw, ["id", "productId", "product_id"]),
		name: pickString(raw, ["name", "productName"]),
		price: pickString(raw, ["priceMin", "price_min", "price"]),
		imageUrl,
		sellerName: pickString(raw, ["sellerName", "seller_name"]),
	};
}

function normalizeOrder(o: Order): OrderRow {
	const firstItem = o.items?.[0];
	const statusCode = o.status ?? "";
	const statusLabel =
		statusCode && statusCode in ORDER_STATUS_LABEL
			? ORDER_STATUS_LABEL[statusCode as OrderStatusValue]
			: statusCode;
	return {
		orderNo: o.orderNo,
		statusLabel,
		statusCode,
		amount: o.totalActualAmount ?? "",
		createdAt: o.createdAt ?? 0,
		primaryItemName: firstItem?.name ?? "",
		itemCount: o.items?.length ?? 0,
		imageUrl: buildImageUrl(firstItem?.image, "thumb"),
		storeId: o.storeId ?? "",
		buyerId: o.buyerId ?? "",
		rawItems: o.items ?? [],
		sellerName: o.sellerName ?? "",
	};
}
