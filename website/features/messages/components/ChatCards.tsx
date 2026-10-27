"use client";

/**
 * Chat structured-message cards. Mirror the buyer APP's
 * `ChatProductBubble` / `ChatOrderBubble` widgets:
 * `hawkr_buyer/lib/shared/widgets/chat_product_bubble.dart`
 * (Figma 10022:101654 / 10022:101644) and `…/chat_order_bubble.dart`
 * (Figma 10022:101541 / 10022:101595).
 *
 *   Product (300w, padding 13/11, rounded 14):
 *   ┌────────────────────────────────────────┐
 *   │ [50px img] │ Title (14 bold)           │
 *   │            │ NT$ Price (16 bold)       │
 *   ├────────────┴───────────────────────────┤
 *   │ [ outline btn ] [ solid btn ]          │  ← web-only
 *   └────────────────────────────────────────┘
 *
 *   Order (300w, padding 12/10; self r=20, peer r=16):
 *   ┌────────────────────────────────────────┐
 *   │ #orderNo [pill]            $amount     │
 *   │ 📦 item1 ×N、item2 …                    │
 *   │ 買家：…              2026/04/18         │
 *   ├────────────────────────────────────────┤
 *   │ [ outline btn ] [ solid btn ]          │  ← web-only
 *   └────────────────────────────────────────┘
 *
 * Self bubbles use the brand-primary-strong green; peer bubbles use white
 * (light) / frosted ink (dark). Product peer price is APP-orange
 * `#EE964B`. Order status codes resolve via CodeMeta
 * (`lib/codemeta/enums.ts → OrderStatus`). The bottom action row is
 * a web-only addition — APP cards are fully tap-through; web keeps
 * them so the affordance is visible without a tap.
 */

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import ImageFallback from "@/components/shared/ImageFallback";
import { ORDER_STATUS_LABEL } from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { ossImageUrl } from "@/lib/media/cdn";

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

function pickNumber(
	obj: Record<string, unknown>,
	keys: readonly string[],
): number | null {
	for (const k of keys) {
		const v = obj[k];
		if (typeof v === "number" && Number.isFinite(v)) return v;
		if (typeof v === "string" && v.trim()) {
			const n = Number(v.replace(/[,$\s]/g, ""));
			if (Number.isFinite(n)) return n;
		}
	}
	return null;
}

function tryParse(content: string): Record<string, unknown> | null {
	const trimmed = (content ?? "").trim();
	if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return null;
	try {
		const parsed = JSON.parse(trimmed);
		if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
	} catch {
		/* ignore */
	}
	return null;
}

function formatTwd(raw: string | number): string {
	const n =
		typeof raw === "number" ? raw : Number(String(raw).replace(/[,$\s]/g, ""));
	if (!Number.isFinite(n)) return typeof raw === "string" ? `NT$ ${raw}` : "";
	return `NT$ ${Math.round(n).toLocaleString("zh-TW")}`;
}

/* ===== Shared media block ===== */

function CardMedia({
	src,
	alt,
	fallback,
}: {
	src: string;
	alt: string;
	fallback: "product" | "order";
}) {
	const hasSrc = Boolean(src);
	return (
		<div className="chat-card__media">
			{hasSrc ? (
				// biome-ignore lint/performance/noImgElement: chat card images are user-provided runtime content
				<img src={ossImageUrl(src, "productThumb")} alt={alt} loading="lazy" />
			) : fallback === "order" ? (
				<AppIcon name="box" className="chat-card__media-icon" />
			) : (
				// No product image → design-system empty image (folder-image),
				// matching the feed/動態 empty state.
				<ImageFallback className="w-full h-full" width={50} height={50} />
			)}
		</div>
	);
}

/* ===== Product card ===== */

export interface ProductCardData {
	id: string;
	name: string;
	price: string;
	imageUrl: string;
	sellerName: string;
	location: string;
	stock: number | null;
	rating: number | null;
	ratingCount: number | null;
}

export function tryParseProduct(content: string): ProductCardData | null {
	const parsed = tryParse(content);
	if (!parsed) return null;
	const type = String(parsed.messageType ?? parsed.type ?? "").toLowerCase();
	if (
		type !== "product" &&
		!parsed.productName &&
		!parsed.skuName &&
		!parsed.name
	) {
		return null;
	}
	return {
		id: pickString(parsed, ["productId", "skuId", "id", "goodsId"]),
		name: pickString(parsed, [
			"productName",
			"skuName",
			"name",
			"goodsName",
			"itemName",
		]),
		price: pickString(parsed, [
			"price",
			"unitPrice",
			"amount",
			"priceMin",
			"salePriceMin",
		]),
		imageUrl: pickString(parsed, ["imageUrl", "image_url", "image", "url"]),
		sellerName: pickString(parsed, [
			"sellerName",
			"shopName",
			"merchant",
			"merchantName",
			"storeName",
		]),
		location: pickString(parsed, [
			"location",
			"origin",
			"region",
			"area",
			"sellerLocation",
		]),
		stock: pickNumber(parsed, ["stock", "remaining", "available", "qty"]),
		rating: pickNumber(parsed, ["rating", "stars", "score"]),
		ratingCount: pickNumber(parsed, [
			"ratingCount",
			"reviews",
			"reviewCount",
			"ratings",
		]),
	};
}

export function ChatProductCard({
	data,
	mine,
	onView,
	shareUrl,
}: {
	data: ProductCardData;
	mine: boolean;
	onView?: () => void;
	/** Full URL copied to the clipboard when 「傳送連結」 is clicked.
	 * Disabled when empty. Falls back to building one from `data.id` is
	 * the parent's job — keeping the URL at the parent keeps the card
	 * agnostic to routing rules. */
	shareUrl?: string;
}) {
	// APP's ChatProductBubble renders only image + name + price. The
	// extra fields (location / sellerName / stock / rating) stay in the
	// parsed data for future surfaces but are not rendered here, so the
	// card height matches the APP and stays compact in the message flow.
	const priceText = data.price ? formatTwd(data.price) : "";
	const [copied, setCopied] = React.useState(false);
	// Cancel any pending "已複製" timer on unmount so we don't setState on a
	// vanished card if the thread is replaced before the 1.6s window ends.
	const copyTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);
	React.useEffect(() => {
		return () => {
			if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
		};
	}, []);

	const handleShare = async () => {
		if (!shareUrl) return;
		try {
			await navigator.clipboard.writeText(shareUrl);
		} catch {
			// Clipboard API can reject (insecure context, denied permission);
			// the optimistic UI still flashes "已複製" briefly which is fine
			// in practice since the failure case is rare and the user can
			// retry. A real toast would surface the error but the card
			// stays self-contained for now.
		}
		setCopied(true);
		if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
		copyTimerRef.current = setTimeout(() => setCopied(false), 1_600);
	};

	return (
		<div
			className="chat-card chat-card--product"
			data-self={mine ? "true" : undefined}
		>
			<div className="chat-card__body">
				<CardMedia src={data.imageUrl} alt={data.name} fallback="product" />
				<div className="chat-card__info">
					{data.name && <p className="chat-card__title">{data.name}</p>}
					{priceText && <p className="chat-card__price">{priceText}</p>}
				</div>
			</div>
			<div className="chat-card__actions">
				<button
					type="button"
					className="chat-card__btn chat-card__btn--outline"
					onClick={handleShare}
					disabled={!shareUrl}
					aria-live="polite"
				>
					{copied ? "已複製連結" : "傳送連結"}
				</button>
				<button
					type="button"
					className="chat-card__btn chat-card__btn--solid"
					onClick={onView}
				>
					查看商品
				</button>
			</div>
		</div>
	);
}

/* ===== Order card ===== */

export interface OrderCardData {
	orderNo: string;
	statusLabel: string;
	amount: string;
	itemsSummary: string;
	displayName: string;
	date: string;
	imageUrl: string;
	variant: "preview" | "check";
}

function flattenLegacyOrder(
	parsed: Record<string, unknown>,
): Record<string, unknown> {
	// Buyer legacy format: { orders: [{ orderSn, productName, ... }] }
	// → flatten the first order onto the top level so downstream readers
	// don't need to know about the nesting.
	if (parsed.orderNo) return parsed;
	const orders = parsed.orders;
	if (Array.isArray(orders) && orders.length > 0) {
		const first = orders[0];
		if (first && typeof first === "object") {
			const firstObj = first as Record<string, unknown>;
			return {
				...parsed,
				...firstObj,
				orderNo: firstObj.orderNo ?? firstObj.orderSn ?? parsed.orderNo,
			};
		}
	}
	return parsed;
}

function formatItemsSummary(parsed: Record<string, unknown>): string {
	const items = parsed.items;
	if (Array.isArray(items) && items.length > 0) {
		const summary = items
			.slice(0, 2)
			.map((raw) => {
				if (!raw || typeof raw !== "object") return "";
				const item = raw as Record<string, unknown>;
				const name = pickString(item, ["productName", "skuName", "name"]);
				const qty = pickString(item, ["quantity"]) || "1";
				return name ? `${name} ×${qty}` : "";
			})
			.filter(Boolean)
			.join("、");
		const rest = items.length - 2;
		return rest > 0 ? `${summary} 等 ${items.length} 件` : summary;
	}
	const count = pickString(parsed, ["itemCount"]);
	if (count) return `共 ${count} 件商品`;
	const single = pickString(parsed, ["productName", "skuName", "name"]);
	const qty = pickString(parsed, ["quantity"]);
	if (single) return qty ? `${single} ×${qty}` : single;
	return "";
}

function formatDisplayName(parsed: Record<string, unknown>): string {
	const buyer = pickString(parsed, ["buyerNickname", "recipientName"]);
	if (buyer) return `買家：${buyer}`;
	const seller = pickString(parsed, ["sellerName"]);
	if (seller) return `賣家：${seller}`;
	return "";
}

function formatDate(raw: string): string {
	// ISO 字串與 unix 秒／毫秒的分流已收斂進 `toEpochMs`；解不出來原字串照登。
	return formatTaipeiDate(raw) || raw;
}

export function tryParseOrder(content: string): OrderCardData | null {
	const raw = tryParse(content);
	if (!raw) return null;
	const type = String(raw.messageType ?? raw.type ?? "").toLowerCase();
	const looksLikeOrder =
		type === "order_prev" ||
		type === "order_check" ||
		type === "order_check_note" ||
		typeof raw.orderNo === "string" ||
		typeof raw.orderSn === "string" ||
		Array.isArray(raw.orders);
	if (!looksLikeOrder) return null;

	const parsed = flattenLegacyOrder(raw);
	const statusRaw = pickString(parsed, ["status"]);
	const statusLabel =
		statusRaw && statusRaw in ORDER_STATUS_LABEL
			? ORDER_STATUS_LABEL[statusRaw as keyof typeof ORDER_STATUS_LABEL]
			: pickString(parsed, ["statusLabel", "statusText", "statusName"]);
	return {
		orderNo: pickString(parsed, ["orderNo", "order_no", "orderSn", "sn"]),
		statusLabel,
		amount: pickString(parsed, [
			"totalActualAmount",
			"totalAmount",
			"amount",
			"total",
			"totalPrice",
		]),
		itemsSummary: formatItemsSummary(parsed),
		displayName: formatDisplayName(parsed),
		date: formatDate(pickString(parsed, ["createTime", "date", "createdAt"])),
		imageUrl: pickString(parsed, [
			"imageUrl",
			"image_url",
			"image",
			"thumbnail",
			"thumb",
			"cover",
		]),
		variant: type === "order_prev" ? "preview" : "check",
	};
}

export function ChatOrderCard({
	data,
	mine,
	onView,
	onSecondary,
}: {
	data: OrderCardData;
	mine: boolean;
	onView?: () => void;
	onSecondary?: () => void;
}) {
	const amountText = data.amount ? formatTwd(data.amount) : "";
	const secondaryLabel = data.variant === "preview" ? "再買一次" : "聯絡賣家";

	return (
		<div
			className="chat-card chat-card--order"
			data-self={mine ? "true" : undefined}
		>
			<div className="chat-card__order-main">
				<div className="chat-card__order-row chat-card__order-row--head">
					{data.orderNo ? (
						<span className="chat-card__order-no">#{data.orderNo}</span>
					) : null}
					{data.statusLabel ? (
						<span className="chat-card__status">{data.statusLabel}</span>
					) : null}
					{amountText ? (
						<span className="chat-card__order-amount">{amountText}</span>
					) : null}
				</div>
				{data.itemsSummary ? (
					<div className="chat-card__order-row chat-card__order-items">
						<AppIcon name="box" size={16} />
						<span>{data.itemsSummary}</span>
					</div>
				) : null}
				{data.displayName || data.date ? (
					<div className="chat-card__order-row chat-card__order-foot">
						{data.displayName ? <span>{data.displayName}</span> : null}
						{data.date ? <span>{data.date}</span> : null}
					</div>
				) : null}
			</div>
			<div className="chat-card__actions">
				<button
					type="button"
					className="chat-card__btn chat-card__btn--outline"
					onClick={onSecondary}
				>
					{secondaryLabel}
				</button>
				<button
					type="button"
					className="chat-card__btn chat-card__btn--solid"
					onClick={onView}
				>
					查看訂單
				</button>
			</div>
		</div>
	);
}
