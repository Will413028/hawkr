"use client";

import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Avatar from "@/components/shared/Avatar";
import { BackLink } from "@/components/shared/BackLink";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { CheckoutInvoiceSection } from "@/features/invoices/checkout/CheckoutInvoiceSection";
import {
	type InvoiceMethodSelection,
	InvoiceMethodSheet,
} from "@/features/invoices/checkout/InvoiceMethodSheet";
import {
	trackAddPaymentInfo,
	trackAddShippingInfo,
	trackBeginCheckout,
} from "@/lib/analytics/events";
import { setPendingPurchase } from "@/lib/analytics/purchase-store";
import { useZoneIdentity } from "@/lib/auth/zone-identity";
import type { CartItem } from "@/lib/cart";
import {
	CHECKOUT_DRAFT_STORAGE_KEY,
	type CheckoutDraft,
	type CheckoutDraftRecipient,
	clearCheckoutDraft,
	readCheckoutDraft,
} from "@/lib/checkout/draft";
import {
	type CheckoutConfirmInvoice,
	type CheckoutConfirmRequest,
	type CheckoutItemInput,
	type CheckoutPreviewResult,
	confirmCheckout,
	previewCheckout,
} from "@/lib/checkout/repo";
import {
	isShippingDestinationReady,
	shippingDestinationKind,
} from "@/lib/checkout/shipping-requirements";
import {
	formatUnitLabel,
	SHIPPING_METHOD_LABEL,
	ShippingMethod as ShippingMethodCode,
} from "@/lib/codemeta/enums";
import { getEnv } from "@/lib/config/env";
import { useBFCacheReset } from "@/lib/hooks/useBFCacheReset";
import { useCart } from "@/lib/hooks/useCart";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { resolveCdnUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { queryKeys } from "@/lib/query/keys";

// 結帳錯誤狀態：訊息與輔助動作必須同進同出。原本拆成 error / errorAction
// 兩個獨立 useState 會讓不同失敗路徑漏改其中一個（例如成功下單後付款失敗的
// 路徑會給 "view_orders"；一般 4006 過期則不應該給），需要 reducer 強制
// 兩個欄位在每個事件中一起切換。
type CheckoutErrorAction = "view_orders" | null;

interface CheckoutErrorState {
	message: string | null;
	action: CheckoutErrorAction;
}

const INITIAL_CHECKOUT_ERROR: CheckoutErrorState = {
	message: null,
	action: null,
};

// 一次「結帳 attempt」內所有 transient state 集中於此型別。
// ── 新增 transient state 必須加進這個型別 ──
// 這是強制契約:NEW_ATTEMPT 與 BFCache 還原時透過 INITIAL_TRANSIENT 一次
// 全清,不會漏 reset 任何欄位。也讓 reviewer 必須在 type 層面看到全部
// transient state 而非散在 1000+ 行裡的 useState。
//
// 不歸於此的 state (繼續用獨立 useState):
//   · draftLoaded / draft  — storage mirror,跨 attempt 仍有意義
interface CheckoutSessionTransient {
	submitting: boolean;
	orderConfirmed: boolean;
	error: CheckoutErrorState;
	preview: CheckoutPreviewResult | null;
	previewLoading: boolean;
	previewError: string | null;
	previewNonce: number;
}

const INITIAL_TRANSIENT: CheckoutSessionTransient = {
	submitting: false,
	orderConfirmed: false,
	error: INITIAL_CHECKOUT_ERROR,
	preview: null,
	previewLoading: false,
	previewError: null,
	previewNonce: 0,
};

type CheckoutTransientEvent =
	// 一次 attempt 結束(新 attempt 開始 / BFCache 還原):全 reset
	| { type: "NEW_ATTEMPT" }
	// 通用 partial patch:對應原有的 setXxx(value) 直接呼叫
	| { type: "SET"; patch: Partial<CheckoutSessionTransient> }
	// errorState reducer events 維持原 API
	| { type: "RESET_ERROR" }
	| { type: "FAIL"; message: string }
	| { type: "FAIL_WITH_VIEW_ORDERS"; message: string }
	// previewNonce functional setter — 改 dedicated event 避免 stale closure
	| { type: "INCREMENT_PREVIEW_NONCE" };

export function transientReducer(
	state: CheckoutSessionTransient,
	evt: CheckoutTransientEvent,
): CheckoutSessionTransient {
	switch (evt.type) {
		case "NEW_ATTEMPT":
			return INITIAL_TRANSIENT;
		case "SET": {
			const keys = Object.keys(evt.patch) as Array<
				keyof CheckoutSessionTransient
			>;
			return keys.every((key) => Object.is(state[key], evt.patch[key]))
				? state
				: { ...state, ...evt.patch };
		}
		case "RESET_ERROR":
			return { ...state, error: INITIAL_CHECKOUT_ERROR };
		case "FAIL":
			return { ...state, error: { message: evt.message, action: null } };
		case "FAIL_WITH_VIEW_ORDERS":
			return {
				...state,
				error: { message: evt.message, action: "view_orders" },
			};
		case "INCREMENT_PREVIEW_NONCE":
			return { ...state, previewNonce: state.previewNonce + 1 };
	}
}

function money(value: number) {
	return `NT$ ${Math.max(0, Math.round(value)).toLocaleString("zh-TW")}`;
}

function shippingMethodLabel(value: string, fallback?: string) {
	return (
		fallback ||
		SHIPPING_METHOD_LABEL[value as keyof typeof SHIPPING_METHOD_LABEL] ||
		value
	);
}

function isStorePickupMethod(method: string) {
	return method === ShippingMethodCode.SEVEN_ELEVEN_STORE_PICKUP;
}

function CheckoutNotice({
	message,
	link = true,
	busy = false,
}: {
	message: string;
	link?: boolean;
	busy?: boolean;
}) {
	return (
		<section
			className="min-h-[calc(100vh-280px)] flex flex-col items-center justify-center gap-4 text-center px-4"
			aria-busy={busy || undefined}
		>
			<h1 className="sr-only">結帳</h1>
			<p>{message}</p>
			{link && (
				<Link
					href="/cart"
					className="hk-cta-primary inline-flex items-center justify-center min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-primary bg-hk-primary text-hk-inverse font-bold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card"
				>
					回購物車
				</Link>
			)}
		</section>
	);
}

/**
 * 這張草稿的收件／取貨資料齊不齊。
 *
 * 🔴 **未知配送方式一律回 `false`（fail-closed／§1.8）。** 舊寫法是
 * 三元的最後一支 `: true` —— 語意等於「沒見過的配送方式就不必收件資料」，
 * 於是 `3005` 商家自送可以一路結帳完成，
 * 後端拿到的訂單既無收件地址也無取貨門市，而畫面上沒有任何異常。
 */
function hasRequiredRecipientFor(draft: CheckoutDraft): boolean {
	return isShippingDestinationReady(draft.shipping.method, {
		hasAddress: Boolean(draft.recipient?.name && draft.recipient?.phone),
		hasStorePickup: Boolean(
			draft.storePickup?.storeId &&
				draft.storePickup?.receiverName &&
				draft.storePickup?.receiverPhone,
		),
	});
}

function recipientZipCode(recipient: CheckoutDraftRecipient | undefined) {
	const directZip = recipient?.zipCode?.trim() ?? "";
	return directZip;
}

function validPreviewTotal(preview: CheckoutPreviewResult | null) {
	const amount = preview?.totalPayAmount;
	return typeof amount === "number" && Number.isFinite(amount) && amount >= 0
		? amount
		: null;
}

function cartItemsToCheckoutItems(items: CartItem[]): CheckoutItemInput[] {
	return items.map((it) => ({
		skuId: it.skuId,
		productId: it.productId,
		quantity: it.quantity,
	}));
}

export default function CheckoutClient() {
	const router = useObservedRouter("checkout");
	useZoneIdentity();
	const cart = useCart();
	const queryClient = useQueryClient();
	const queryIdentity = useQueryIdentity();

	const [draftLoaded, setDraftLoaded] = React.useState(false);
	const [draft, setDraft] = React.useState<CheckoutDraft | null>(null);

	// B02（發票方式選擇 Sheet）的開關與買家的選擇。
	//
	// 🔴 undefined ＝「買家沒有動過」⇒ confirm **整包省略 invoice**，後端走
	// USE_BUYER_DEFAULT。這與「買家明確選了不使用載具」是兩件事，後者送
	// { kind: "none" } 但同樣省略 invoice —— 差別只在畫面顯示，不在 payload。
	const [invoiceSheetOpen, setInvoiceSheetOpen] = React.useState(false);
	const [invoiceSelection, setInvoiceSelection] = React.useState<
		InvoiceMethodSelection | undefined
	>(undefined);

	// 一次 attempt 內所有 transient state 集中於此 reducer。
	// 設計理由見 CheckoutSessionTransient 型別註解。透過保留同名 setter
	// shim,既有引用點不必動,內部統一走 reducer,新 attempt / BFCache
	// 還原時 NEW_ATTEMPT 一次全清。
	const [transient, dispatchTransient] = React.useReducer(
		transientReducer,
		INITIAL_TRANSIENT,
	);
	const {
		submitting,
		orderConfirmed,
		preview,
		previewLoading,
		previewError,
		previewNonce,
	} = transient;
	const error = transient.error.message;
	const errorAction = transient.error.action;
	// 既有 setXxx API 形狀不變,呼叫端不需要逐處改寫。內部走 reducer。
	// useCallback 穩定 identity,避免 useEffect / useMemo deps 觸發 lint。
	const setSubmitting = React.useCallback(
		(value: boolean) =>
			dispatchTransient({ type: "SET", patch: { submitting: value } }),
		[],
	);
	const setOrderConfirmed = React.useCallback(
		(value: boolean) =>
			dispatchTransient({ type: "SET", patch: { orderConfirmed: value } }),
		[],
	);
	// 訂單一旦確認成立（confirmCheckout 成功 → orderConfirmed=true），就讓
	// buyer.orders 這組 query 失效：之後都是 router.push soft-nav，TanStack in-memory cache
	// 不會重置，30s staleTime 內看不到新單。invalidate 為冪等，重複觸發無害。
	React.useEffect(() => {
		if (!orderConfirmed) return;
		void queryClient.invalidateQueries({
			queryKey: queryKeys.buyer.orders(queryIdentity),
		});
	}, [orderConfirmed, queryClient, queryIdentity]);
	const setPreview = React.useCallback(
		(value: CheckoutPreviewResult | null) =>
			dispatchTransient({ type: "SET", patch: { preview: value } }),
		[],
	);
	const setPreviewLoading = React.useCallback(
		(value: boolean) =>
			dispatchTransient({ type: "SET", patch: { previewLoading: value } }),
		[],
	);
	const setPreviewError = React.useCallback(
		(value: string | null) =>
			dispatchTransient({ type: "SET", patch: { previewError: value } }),
		[],
	);
	// dispatchError 維持原 API (FAIL / FAIL_WITH_VIEW_ORDERS / RESET),
	// 直接 forward 進 transient reducer 的對應 event。
	const dispatchError = React.useCallback(
		(
			evt:
				| { type: "RESET" }
				| { type: "FAIL"; message: string }
				| { type: "FAIL_WITH_VIEW_ORDERS"; message: string },
		) => {
			if (evt.type === "RESET") {
				dispatchTransient({ type: "RESET_ERROR" });
			} else {
				dispatchTransient(evt);
			}
		},
		[],
	);
	// ── 一次 attempt 內所有 ref 在這裡集中宣告 ──
	// 新增 ref 必須同時加進 resetAttemptRefs() 列表。
	const submittingRef = React.useRef(false);
	const previewRequestedRef = React.useRef<string | null>(null);
	const beginCheckoutFiredRef = React.useRef(false);
	const resetAttemptRefs = React.useCallback(() => {
		submittingRef.current = false;
		previewRequestedRef.current = null;
		beginCheckoutFiredRef.current = false;
	}, []);

	// attempt id 用 ref 釘住「component 首次認識的 attempt」,不對齊 draft
	// 存在性 (draft 在 confirm 成功後會被 clearCheckoutDraft 清掉,但 attempt
	// 還活著,使用者可能在看「已建單但付款失敗」訊息或等 native sheet)。
	// 只有真的看到「新的 draft.createdAt」才視為新 attempt 開始。
	const attemptIdRef = React.useRef<number | null>(null);

	// 偵測新 attempt 開始:
	//   · 第一次看到 draft → 認識它,僅記下 ref;state 已是 initial,不 reset
	//   · 同一 attempt id → 不做事
	//   · 新 attempt id   → 一次清光 transient state + refs
	//
	// dep 刻意只看 draft?.createdAt 不看整個 draft:draft 從 T1 → undefined
	// (clearCheckoutDraft 後) 不該觸發 reset,符合「attempt 還活著」業務
	// 語意;從 T1 → T2 才觸發。
	// biome-ignore lint/correctness/useExhaustiveDependencies: 故意只訂閱 createdAt 變化,draft 物件其他欄位變化不該觸發 reset
	React.useEffect(() => {
		if (!draft) return;
		const next = draft.createdAt;
		if (attemptIdRef.current === null) {
			attemptIdRef.current = next;
			return;
		}
		if (attemptIdRef.current === next) return;
		attemptIdRef.current = next;
		dispatchTransient({ type: "NEW_ATTEMPT" });
		resetAttemptRefs();
	}, [draft?.createdAt, resetAttemptRefs]);

	// BFCache 還原(瀏覽器返回鍵)時,transient state 一律不該保留 —— 上一
	// 次的 submitting / errorState 若帶回新一輪會卡使用者(典型:跳 success
	// 後按返回鍵回 checkout,submitting=true 卡按鈕)。attemptIdRef 也清,
	// 讓後續 effect 重新認識下一個 attempt id。
	useBFCacheReset(() => {
		attemptIdRef.current = null;
		dispatchTransient({ type: "NEW_ATTEMPT" });
		resetAttemptRefs();
	});

	React.useEffect(() => {
		setDraft(readCheckoutDraft());
		setDraftLoaded(true);
	}, []);

	// 跨分頁同步：另一分頁 confirm 成功 / 清掉 draft 時，本分頁也跟著 reset。
	// 避免「分頁 A 已下單，分頁 B 還用舊 draft 重送 confirm」造成 4092 / 6005。
	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const onStorage = (e: StorageEvent) => {
			if (e.key !== CHECKOUT_DRAFT_STORAGE_KEY) return;
			if (e.newValue === null) {
				setDraft(null);
				setPreview(null);
				setPreviewError(null);
			} else {
				setDraft(readCheckoutDraft());
			}
		};
		window.addEventListener("storage", onStorage);
		return () => window.removeEventListener("storage", onStorage);
	}, [setPreview, setPreviewError]);

	// Items 來源:後端 cart 經 draft.selectedSkuIds 過濾。useCart 已在 mount
	// 時 fetch backend cart 並訂閱 CART_UPDATED_EVENT,checkout 期間自動同步。
	// 對齊 buyer-app:後端是唯一真實來源,draft 只記「選擇」不記「快照」。
	const selectedSkuIdSet = React.useMemo(
		() => new Set(draft?.selectedSkuIds ?? []),
		[draft?.selectedSkuIds],
	);
	const selectedItems = React.useMemo<CartItem[]>(
		() => cart.items.filter((it) => selectedSkuIdSet.has(it.skuId)),
		[cart.items, selectedSkuIdSet],
	);
	const checkoutItems = React.useMemo(
		() => cartItemsToCheckoutItems(selectedItems),
		[selectedItems],
	);
	const hasInvalidCartItems = React.useMemo(
		() => selectedItems.some((it) => !it.skuId || it.productId == null),
		[selectedItems],
	);
	const isStorePickup = draft
		? isStorePickupMethod(draft.shipping.method)
		: false;
	// 「要不要收件地址」的判準是 `destinationKind`，與購物車的
	// `writeFrozenCheckoutDraft` 同一個判準。
	const destinationKind = shippingDestinationKind(draft?.shipping.method);
	const needsAddress = destinationKind === "address";
	const hasRequiredRecipient = !draft || hasRequiredRecipientFor(draft);
	const itemsSubtotal = React.useMemo(
		() =>
			selectedItems.reduce(
				(sum, item) => sum + item.priceValue * item.quantity,
				0,
			),
		[selectedItems],
	);

	const previewToken = React.useMemo(() => {
		if (!draft) return "";
		const itemSig = selectedItems
			.map((it) => `${it.skuId}|${it.productId}|${it.quantity}`)
			.join(",");
		return [
			draft.createdAt,
			itemSig,
			draft.shipping.method,
			draft.shipping.deliveryTemp,
			draft.coupon?.templateId ?? "",
			previewNonce,
		].join(":");
	}, [draft, selectedItems, previewNonce]);

	React.useEffect(() => {
		setPreviewError(null);
		// Invariant: effect 不打 fetch 時 previewLoading 必為 false。任何
		// 早期 return 都要先清掉 loading,否則前一輪 in-flight fetch 被
		// cancelled 守衛攔下後,UI 會永卡「計算中」。新增 early-return 條件
		// 時也必須沿用此 invariant。
		if (
			!draft ||
			selectedItems.length === 0 ||
			hasInvalidCartItems ||
			!hasRequiredRecipient
		) {
			setPreviewLoading(false);
			return;
		}
		if (previewRequestedRef.current === previewToken) return;

		let cancelled = false;
		previewRequestedRef.current = previewToken;
		setPreview(null);
		setPreviewLoading(true);
		const resolvedZipCode = recipientZipCode(draft.recipient);
		const previewBody = {
			items: checkoutItems,
			shippingMethod: draft.shipping.method,
			deliveryTemp: draft.shipping.deliveryTemp,
			// 🔴 與 confirm 同一個閘門（`needsAddress`）：preview 與 confirm 對**同一筆訂單**
			//    送不同的欄位，就會出現「preview 沒看過的值被寫進訂單」。
			...(needsAddress && resolvedZipCode
				? { receiverZipCode: resolvedZipCode }
				: {}),
			...(isStorePickup && draft.storePickup
				? {
						storeId: draft.storePickup.storeId,
						storeName: draft.storePickup.storeName,
					}
				: {}),
			...(draft.coupon?.templateId
				? { couponTemplateId: draft.coupon.templateId }
				: {}),
		};
		// Always fire a fresh preview — previously this went through a
		// module-scope `previewRequestCache` that persisted across
		// navigations and re-served the same `checkoutId`. That cache was
		// a likely culprit for the 「資源衝突 / cart 已消耗」 409, because
		// returning the same checkoutId on a re-entry meant the next
		// confirm tried to lock an already-emptied cart row. Re-firing
		// preview every time the token changes pays a single extra
		// roundtrip but guarantees a fresh checkoutId.
		void previewCheckout(previewBody)
			.then((result) => {
				if (cancelled) return;
				setPreview(result);
				if (!beginCheckoutFiredRef.current) {
					beginCheckoutFiredRef.current = true;
					trackBeginCheckout(selectedItems, {
						value: Number(result.totalPayAmount) || 0,
						coupon: draft.coupon?.title,
					});
				}
			})
			.catch((e) => {
				if (!cancelled) {
					setPreviewError(e instanceof Error ? e.message : "結帳預覽失敗");
				}
			})
			.finally(() => {
				if (!cancelled) setPreviewLoading(false);
			});
		return () => {
			cancelled = true;
			// React StrictMode in dev intentionally runs effect cleanup and
			// re-runs the effect. If we keep the token marked as requested
			// after that cleanup, the second pass skips the preview request
			// while the first request is no longer allowed to update state,
			// leaving previewLoading stuck at 「計算中」.
			if (previewRequestedRef.current === previewToken) {
				previewRequestedRef.current = null;
			}
		};
	}, [
		checkoutItems,
		draft,
		hasInvalidCartItems,
		hasRequiredRecipient,
		isStorePickup,
		needsAddress,
		previewToken,
		selectedItems,
		setPreview,
		setPreviewError,
		setPreviewLoading,
	]);

	const lastShippingTierRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		const method = draft?.shipping.method;
		if (!method || selectedItems.length === 0) return;
		const value = Number(preview?.totalPayAmount) || 0;
		if (value === 0) return;
		if (lastShippingTierRef.current === method) return;
		lastShippingTierRef.current = method;
		trackAddShippingInfo(selectedItems, {
			value,
			shippingTier: method,
			coupon: draft.coupon?.title,
		});
	}, [draft?.shipping.method, preview?.totalPayAmount, selectedItems, draft]);

	if (!draftLoaded || (cart.loading && cart.items.length === 0)) {
		return <CheckoutNotice message="正在讀取結帳資料…" link={false} busy />;
	}

	if (!draft) {
		return (
			<CheckoutNotice message="結帳資料已過期，請從購物車重新確認商品、配送與優惠券。" />
		);
	}

	if (!orderConfirmed && (selectedItems.length === 0 || hasInvalidCartItems)) {
		return (
			<CheckoutNotice message="商品資料同步中，請回購物車重新確認後再結帳。" />
		);
	}

	if (!hasRequiredRecipient) {
		return (
			<CheckoutNotice message="配送資料不完整，請回購物車重新選擇收件地址或取貨門市。" />
		);
	}

	const previewTotal = validPreviewTotal(preview);
	const previewSubtotal = preview?.subtotal ?? itemsSubtotal;
	const previewShippingFee =
		preview?.shippingFee != null && preview.shippingFee >= 0
			? preview.shippingFee
			: draft.shipping.fee;
	const previewDiscount = preview?.discountAmount ?? 0;
	const grandTotal = previewTotal;
	const itemCount = selectedItems.reduce((sum, item) => sum + item.quantity, 0);

	const isZeroPayment = grandTotal === 0;

	const canCheckout =
		!hasInvalidCartItems &&
		!previewLoading &&
		!submitting &&
		errorAction !== "view_orders" &&
		!!preview?.checkoutId &&
		grandTotal != null;

	const retryPreview = () => {
		setPreview(null);
		setPreviewError(null);
		previewRequestedRef.current = null;
		dispatchTransient({ type: "INCREMENT_PREVIEW_NONCE" });
	};

	const finishSubmitting = () => {
		submittingRef.current = false;
		setSubmitting(false);
	};

	// Build the gRPC-shaped confirm payload from draft + preview. Pure
	// transform — caller (handlePlaceOrder) narrows the nullable state and
	// passes non-null inputs explicitly.
	// 🔴 **B02 的入口只有這一個**。2026-09-01 dev 實測抓到：Sheet 已經渲染在頁尾、
	// state 也在，但 `<CheckoutInvoiceSection>` 少了 `onChangeMethod`
	// ⇒ 沒有任何東西呼叫 setInvoiceSheetOpen(true) ⇒ **整個 B02 在 bundle 裡但打不開**。
	// TypeScript 不會叫（那是選填 prop）、測試不會紅、畫面看起來也正常——
	// 只有真的去看一眼結帳頁才會發現「變更發票方式」那顆鍵不在。
	const buildConfirmPayload = (
		d: CheckoutDraft,
		p: CheckoutPreviewResult,
	): CheckoutConfirmRequest => {
		const orderRemark = d.note.trim();
		const resolvedZipCode = recipientZipCode(d.recipient);
		const payload: CheckoutConfirmRequest = {
			checkoutId: p.checkoutId,
			items: checkoutItems,
			shippingMethod: d.shipping.method,
			deliveryTemp: d.shipping.deliveryTemp,
			shippingNote: orderRemark,
			orderRemark,
			// 閘門是 `needsAddress`（收件目標是地址），與 preview 同一個。
			...(needsAddress && d.recipient
				? {
						...(resolvedZipCode ? { receiverZipCode: resolvedZipCode } : {}),
						recipientName: d.recipient.name,
						recipientPhone: d.recipient.phone,
						shippingAddress: d.recipient.line,
					}
				: {}),
			...(isStorePickup && d.storePickup
				? {
						storeId: d.storePickup.storeId,
						storeName: d.storePickup.storeName,
						recipientName: d.storePickup.receiverName,
						recipientPhone: d.storePickup.receiverPhone,
					}
				: {}),
			...(d.coupon?.templateId
				? { couponTemplateId: d.coupon.templateId }
				: {}),
			// 58 §7.1.2 步驟 1：買家的發票選擇。**沒選就整包省略**（後端走
			// USE_BUYER_DEFAULT）——已上架的 App 就是這樣送的，那條路必須永遠成立。
			...(confirmInvoiceOf(invoiceSelection)
				? { invoice: confirmInvoiceOf(invoiceSelection) }
				: {}),
		};
		if (process.env.NODE_ENV !== "production") {
			console.warn(
				"[checkout] confirm request payload:",
				JSON.stringify(payload, null, 2),
			);
		}
		return payload;
	};

	const logFailure = (stage: "confirm" | "payment", e: unknown) => {
		if (process.env.NODE_ENV === "production") return;
		const status =
			e && typeof e === "object" && "status" in e
				? (e as { status?: number }).status
				: undefined;
		const code =
			e && typeof e === "object" && "code" in e
				? (e as { code?: number }).code
				: undefined;
		const payload =
			e && typeof e === "object" && "payload" in e
				? (e as { payload?: unknown }).payload
				: undefined;
		const reason = e instanceof Error ? e.message : "未知錯誤";
		console.warn(
			`[checkout] place order failed — stage=${stage} status=${status} code=${code} reason=${reason}`,
		);
		// Dev-only: the backend error payload can carry order/PII fields, so keep
		// it out of the production console (buyer's own devtools). stage/status/
		// code/reason above are non-sensitive and stay.
		if (getEnv().enableApiLogging) {
			try {
				console.warn(
					"[checkout] backend payload:",
					JSON.stringify(payload, null, 2),
				);
			} catch {
				console.warn("[checkout] backend payload (raw):", payload);
			}
		}
	};

	// Confirm-time errors: order has NOT been created yet. Map known backend
	// codes to user-friendly copy (4092 pending order, 6005 lock race, 4006
	// expired, etc.) and reset submitting state.
	const handleConfirmError = (e: unknown) => {
		logFailure("confirm", e);
		const status =
			e && typeof e === "object" && "status" in e
				? (e as { status?: number }).status
				: undefined;
		const code =
			e && typeof e === "object" && "code" in e
				? (e as { code?: number }).code
				: undefined;
		const reason = e instanceof Error ? e.message : "未知錯誤";

		if (code === 4092) {
			// 後端 03 §5.1.2：buyer 已有 PENDING_PAYMENT 訂單包含這些 sku。
			const orderNoMatch = reason.match(/(O\d{6,})/);
			const pendingOrderNo = orderNoMatch ? orderNoMatch[1] : null;
			dispatchError({
				type: "FAIL_WITH_VIEW_ORDERS",
				message: pendingOrderNo
					? `你已有未付款訂單 ${pendingOrderNo}，請先完成付款或取消後再下單。`
					: "你已有未付款訂單，請先完成付款或取消後再下單。",
			});
			finishSubmitting();
			return;
		}
		if (code === 6005) {
			dispatchError({
				type: "FAIL",
				message: "結帳鎖定失敗，請回購物車重新確認商品後再試。",
			});
			finishSubmitting();
			return;
		}
		// 46 §7.2：賣家停權收斂點（訂單尚未建立）。單一賣家結帳下這會擋下整張單，
		// 但沿用既定文案；refresh 讓購物車盡快反映（見 CartClient 對稱處理）。
		if (code === 9241) {
			dispatchError({
				type: "FAIL",
				message: "部分商品暫停販售，已自結帳移除",
			});
			void cart.refresh();
			finishSubmitting();
			return;
		}
		// 8026 訂單金額過高（L1）/ 8021 付款方式金額不符（L2，理論上前端 gating 已擋，保留 fallback）
		if (code === 8026) {
			dispatchError({
				type: "FAIL",
				message: "訂單金額超過單筆付款上限，請減少商品數量或分批結帳。",
			});
			finishSubmitting();
			return;
		}
		if (code === 8021) {
			dispatchError({
				type: "FAIL",
				message: "所選付款方式不適用本訂單金額，請改用其他付款方式。",
			});
			finishSubmitting();
			return;
		}
		if (status === 409) {
			dispatchError({
				type: "FAIL",
				message: `下單發生衝突：${reason}。請回購物車重新確認後重試。`,
			});
			finishSubmitting();
			return;
		}
		const isExpired =
			code === 4006 || /\[?4006\]?|已過期|expired/i.test(reason);
		if (isExpired) {
			dispatchError({
				type: "FAIL",
				message: "結帳資訊已過期，已重新預覽，請再按一次「確認下單」。",
			});
			retryPreview();
		} else if (status === 410 || status === 422) {
			dispatchError({
				type: "FAIL",
				message: `結帳資訊已過期（${reason}）— 請回購物車重新確認。`,
			});
		} else {
			dispatchError({ type: "FAIL", message: `下單失敗：${reason}` });
		}
		finishSubmitting();
	};

	// 確認訂單（舊 REST）之後：金額為零的訂單沒有付款步驟，直接進成功頁；
	// 其餘進付款步驟（`/orders/{orderNo}`，Stripe Payment Element）。付款結果由付款步驟以 GetPayment 確認。
	// 一次確認可能拆成多張單，付款只對第一張（`orderNos[0]`）。
	const runCheckout = async (payload: CheckoutConfirmRequest) => {
		try {
			const confirm = await confirmCheckout(payload);
			const orderNo = confirm.orderNos[0];
			if (!orderNo) throw new Error("後端未回傳訂單編號");
			setOrderConfirmed(true);
			clearCheckoutDraft();
			trackAddPaymentInfo(selectedItems, {
				value: Number(confirm.totalPayAmount) || 0,
				paymentType: "stripe",
				coupon: draft.coupon?.title,
			});
			void cart.refresh();
			setPendingPurchase(confirm.orderNos);
			router.push(
				confirm.totalPayAmount === 0
					? `/checkout/success?order=${encodeURIComponent(orderNo)}`
					: `/orders/${encodeURIComponent(orderNo)}`,
			);
		} catch (e) {
			handleConfirmError(e);
		}
	};

	const handlePlaceOrder = () => {
		if (
			submittingRef.current ||
			!canCheckout ||
			!preview ||
			!draft ||
			grandTotal == null
		) {
			return;
		}

		// Validate cart items synchronously (no network roundtrip).
		const selectedItemSkuSet = new Set(selectedItems.map((it) => it.skuId));
		const missingIds = draft.selectedSkuIds.filter(
			(id) => !selectedItemSkuSet.has(id),
		);
		if (missingIds.length > 0) {
			const names = missingIds.map((id) => {
				const found = selectedItems.find((it) => it.skuId === id);
				return found?.name ?? id;
			});
			dispatchError({
				type: "FAIL_WITH_VIEW_ORDERS",
				message: `商品「${names.join("、")}」已不在購物車中(可能在其他裝置完成結帳或商品已下架),請回購物車重新確認。`,
			});
			return;
		}

		submittingRef.current = true;
		setSubmitting(true);
		dispatchError({ type: "RESET" });

		void runCheckout(buildConfirmPayload(draft, preview));
	};

	return (
		<section className="max-w-[1100px] mx-auto py-10 px-4 pb-16 max-md:pb-28">
			<header className="mb-10">
				<BackLink
					fallbackHref="/cart"
					className="inline-flex text-xs font-bold text-foreground-soft no-underline mb-4"
				>
					← 返回
				</BackLink>
				<p className="text-[0.6875rem] font-black text-hk-accent tracking-[0.12em] m-0 mb-1">
					CHECKOUT
				</p>
				<h1>結帳</h1>
			</header>

			<div className="grid grid-cols-1 lg:grid-cols-[1fr_380px] gap-8 items-start">
				<div className="flex flex-col gap-6">
					<section className="bg-content1 border border-divider rounded-[32px] p-[18px_18px_8px] shadow-hk-card">
						<SectionTitle index={1} title="商品確認" />
						<div className="flex items-center gap-3 pb-4 border-b border-dashed border-divider mb-2">
							<SellerLogo src={draft.seller.logoUrl} />
							<div>
								<strong className="text-sm font-extrabold text-foreground block">
									{draft.seller.name}
								</strong>
								<span className="text-xs font-bold text-foreground-soft">
									{itemCount} 件商品 · 小計 {money(itemsSubtotal)}
								</span>
							</div>
						</div>
						<ul className="list-none m-0 p-0">
							{selectedItems.map((item) => (
								<CheckoutItemRow item={item} key={item.id} />
							))}
						</ul>
					</section>

					<section className="bg-content1 border border-divider rounded-[32px] p-[18px] shadow-hk-card">
						<SectionTitle index={2} title="配送與優惠" />
						<div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
							<LockedCard
								icon={<ShippingIcon />}
								label="運送方式"
								value={shippingMethodLabel(
									draft.shipping.method,
									draft.shipping.label,
								)}
								detail={
									draft.shipping.fee === 0 ? "免運" : money(draft.shipping.fee)
								}
							/>
							<LockedCard
								label={isStorePickup ? "取貨門市" : "收件資料"}
								value={
									isStorePickup
										? (draft.storePickup?.storeName ?? "已選擇門市")
										: (draft.recipient?.name ?? "已選擇地址")
								}
								detail={
									isStorePickup && draft.storePickup
										? `${draft.storePickup.receiverName} · ${draft.storePickup.receiverPhone}`
										: draft.recipient
											? `${draft.recipient.phone} · ${draft.recipient.line}`
											: "配送資料已在購物車確認"
								}
							/>
							<LockedCard
								label="優惠券"
								value={draft.coupon?.title ?? "未使用優惠券"}
								detail={draft.coupon ? "優惠券已套用" : "可回購物車更換優惠券"}
							/>
							{draft.note ? (
								<LockedCard
									label="訂單備註"
									value={draft.note}
									detail="已帶到給賣家的訂單備註"
									className="col-span-full"
								/>
							) : null}
						</div>
					</section>

					{/*
					 * 3 · 發票與收據（58A §B01）。
					 *
					 * 🔴 `preview.invoice` 為 null 時整個區塊不渲染：後端拿不到發票服務時
					 * 不回這個 key（gateway 的 DTO 是指標 + omitempty）。渲染一張空卡片
					 * 會讓買家以為系統壞了，而那比少一個區塊更糟。
					 *
					 * ⚠️ 區塊編號是**動態的**：沒有發票區時付款方式仍是 3。寫死 4 會讓
					 * 降級路徑上出現「1 2 4」。
					 */}
					{preview?.invoice ? (
						<CheckoutInvoiceSection
							invoice={preview.invoice}
							sectionIndex={3}
							selection={invoiceSelection}
							onChangeMethod={() => setInvoiceSheetOpen(true)}
						/>
					) : null}

					{isZeroPayment ? (
						<section className="bg-content1 border border-divider rounded-[32px] p-[18px] shadow-hk-card">
							<SectionTitle index={preview?.invoice ? 4 : 3} title="付款方式" />
							<div className="flex items-center gap-3 p-[14px_16px] bg-content2 border border-divider rounded-hkui-lg">
								<span
									className="text-success-600 text-lg shrink-0"
									aria-hidden="true"
								>
									✓
								</span>
								<div className="flex flex-col gap-[2px]">
									<strong className="text-sm font-bold text-foreground">
										無需付款
									</strong>
									<span className="text-xs text-foreground-soft font-normal">
										本訂單金額為 0 元（已全額折抵），確認後直接成立。
									</span>
								</div>
							</div>
						</section>
					) : null}
				</div>

				<aside className="bg-content1 border border-divider rounded-[32px] p-6 shadow-hk-card flex flex-col gap-5">
					<div className="flex flex-col">
						<p className="text-[0.625rem] font-black text-hk-accent tracking-[0.14em] m-0 mb-1">
							ORDER SUMMARY
						</p>
						<h2 className="text-lg font-extrabold text-foreground m-0">
							訂單摘要
						</h2>
					</div>
					<div className="flex items-center gap-3 pb-4 border-b border-dashed border-divider">
						<SellerLogo src={draft.seller.logoUrl} />
						<div>
							<strong className="text-xs font-extrabold text-foreground block">
								{draft.seller.name}
							</strong>
							<span className="text-[0.625rem] font-bold text-foreground-soft">
								{itemCount} 件商品
							</span>
						</div>
					</div>
					<ul className="list-none m-0 p-0 flex flex-col gap-3 max-h-[180px] overflow-y-auto pr-1 border-b border-dashed border-divider pb-4">
						{selectedItems.map((it) => (
							<li className="flex items-center gap-3" key={it.id}>
								<ProductThumb src={it.image} alt={it.name} />
								<div className="flex-1 min-w-0">
									<strong className="text-xs font-extrabold text-foreground block truncate">
										{it.name}
									</strong>
									<span className="text-[0.625rem] font-bold text-foreground-soft">
										{it.unit ? `${formatUnitLabel(it.unit)} · ` : ""}x{" "}
										{it.quantity}
									</span>
								</div>
								<b className="text-xs font-extrabold text-foreground">
									{money(it.priceValue * it.quantity)}
								</b>
							</li>
						))}
					</ul>
					<div className="p-3 bg-content2 border border-divider rounded-hkui-md text-xs flex flex-col gap-1">
						<span className="font-bold text-foreground-soft">
							{isStorePickup ? "取貨資料" : "收件資料"}
						</span>
						<strong className="font-extrabold text-foreground">
							{isStorePickup
								? `${draft.storePickup?.storeName ?? "已選擇門市"} 取貨`
								: draft.recipient?.name}
						</strong>
						<p className="m-0 text-foreground-soft leading-normal">
							{isStorePickup && draft.storePickup
								? `${draft.storePickup.receiverName} · ${draft.storePickup.receiverPhone}`
								: draft.recipient
									? `${draft.recipient.phone} · ${draft.recipient.line}`
									: "配送資料已在購物車確認"}
						</p>
					</div>
					<div className="p-3 bg-content2 border border-divider rounded-hkui-md text-xs flex flex-row justify-between gap-4">
						<div className="flex-1 flex flex-col gap-1">
							<span className="font-bold text-foreground-soft">運送方式</span>
							<strong className="font-extrabold text-foreground">
								{shippingMethodLabel(
									draft.shipping.method,
									draft.shipping.label,
								)}
							</strong>
						</div>
					</div>
					<ul
						className="list-none m-0 p-0 flex flex-col gap-1.5 border-b border-dashed border-divider pb-4"
						aria-label="結帳保障"
					>
						<li className="text-[0.6875rem] font-bold text-foreground-soft flex items-center gap-2 before:content-[''] before:w-1.2 before:h-1.2 before:rounded-full before:bg-hk-accent before:shrink-0">
							購物車已完成配送與優惠券確認
						</li>
						<li className="text-[0.6875rem] font-bold text-foreground-soft flex items-center gap-2 before:content-[''] before:w-1.2 before:h-1.2 before:rounded-full before:bg-hk-accent before:shrink-0">
							訂單成立後可追蹤狀態
						</li>
					</ul>
					<dl className="m-0 flex flex-col gap-2.5">
						<div className="flex justify-between items-center">
							<dt className="text-xs font-bold text-foreground-soft">
								商品小計
							</dt>
							<dd className="m-0 text-sm font-extrabold text-foreground">
								{money(previewSubtotal)}
							</dd>
						</div>
						<div className="flex justify-between items-center">
							<dt className="text-xs font-bold text-foreground-soft">運費</dt>
							<dd className="m-0 text-sm font-extrabold text-foreground">
								{previewLoading
									? "計算中"
									: previewShippingFee === 0
										? "免運"
										: money(previewShippingFee)}
							</dd>
						</div>
						{previewDiscount > 0 ? (
							<div className="flex justify-between items-center">
								<dt className="text-xs font-bold text-foreground-soft">
									優惠折抵
								</dt>
								<dd className="m-0 text-sm font-extrabold text-foreground">
									-{money(previewDiscount)}
								</dd>
							</div>
						) : null}
						<div className="flex justify-between items-center">
							<dt className="text-sm font-extrabold text-foreground">
								應付金額
							</dt>
							<dd className="m-0 text-xl font-black text-hk-accent">
								{previewLoading || grandTotal == null
									? "計算中"
									: money(grandTotal)}
							</dd>
						</div>
					</dl>
					{previewError ? (
						<div
							className="p-3 rounded-hkui-md bg-danger-50 border border-danger-100 flex flex-col gap-2"
							role="alert"
						>
							<p className="hkui-login__hint hkui-login__hint--error">
								{previewError}
							</p>
							<Button
								type="button"
								variant="bordered"
								size="sm"
								onClick={retryPreview}
							>
								重新預覽
							</Button>
						</div>
					) : null}
					{error ? (
						<div
							className="p-3 rounded-hkui-md flex flex-col gap-2 border bg-danger-50 border-danger-100"
							role="alert"
						>
							<p className="hkui-login__hint hkui-login__hint--error">
								{error}
							</p>
							{errorAction === "view_orders" ? (
								<Button href="/account/orders" variant="bordered" size="sm">
									查看我的訂單 →
								</Button>
							) : null}
						</div>
					) : null}
					<Button
						type="button"
						size="lg"
						fullWidth
						className="scroll-mb-24"
						disabled={!canCheckout || submitting}
						isLoading={submitting || previewLoading}
						loadingLabel={submitting ? "建立訂單中…" : "確認金額中…"}
						onClick={handlePlaceOrder}
					>
						確認下單
					</Button>
					<p className="text-[0.625rem] font-bold text-foreground-soft text-center leading-normal m-0 [&_a]:text-hk-accent [&_a]:underline">
						送出代表你同意 <Link href="/terms">服務條款</Link> 與
						<Link href="/return-policy">退換貨政策</Link>。
					</p>
				</aside>
			</div>

			{/*
			 * B02（發票方式選擇 Sheet，58A §B02）。
			 * 只在後端回了 invoice 區塊時才掛載——沒有發票服務就沒有可選的東西。
			 */}
			{preview?.invoice ? (
				<InvoiceMethodSheet
					open={invoiceSheetOpen}
					onClose={() => setInvoiceSheetOpen(false)}
					current={invoiceSelection}
					onSelect={(s) => {
						setInvoiceSelection(s);
						setInvoiceSheetOpen(false);
					}}
				/>
			) : null}
		</section>
	);
}

/**
 * B02 的選擇 → confirm 的 `invoice` 區塊（58 §7.1.2 步驟 1）。
 *
 * 🔴 回 undefined ＝ **整包省略**，後端走 `USE_BUYER_DEFAULT`。兩種情況都回它：
 * 買家沒動過（`undefined`），以及買家明選「不使用載具」（`kind: "none"`）——
 * 後者在後端沒有對應的表達方式（`chk_ois_mode` 的四個值裡沒有「這一單不用載具」，
 * 見 58 §5821 第 4 項），而預設行為本來就是寄 email，兩者結果相同。
 * ⇒ 差別只在畫面顯示，不在 payload。硬要送一個空的 invoice 區塊反而會撞
 * `chk_ois_pref_pair` 落 500。
 */
function confirmInvoiceOf(
	s: InvoiceMethodSelection | undefined,
): CheckoutConfirmInvoice | undefined {
	if (!s) return undefined;
	switch (s.kind) {
		case "profile":
			return { profileId: s.profileId };
		case "taxId":
			return { buyerTaxId: s.buyerTaxId, buyerTitle: s.buyerName };
		case "donation":
			return { donationCode: s.donationCode };
		case "none":
			return undefined;
	}
}

function SectionTitle({ index, title }: { index: number; title: string }) {
	return (
		<div className="mb-5">
			<h2 className="text-lg font-extrabold text-foreground m-0 flex items-center gap-3">
				<span className="w-6 h-6 rounded-full bg-hk-primary text-hk-inverse inline-flex items-center justify-center text-xs font-black">
					{index}
				</span>
				{title}
			</h2>
		</div>
	);
}

function SellerLogo({ src }: { src?: string }) {
	return (
		<span
			className="w-8 h-8 rounded-full border border-divider bg-content2 inline-flex items-center justify-center overflow-hidden shrink-0 text-xs font-bold text-hk-accent"
			aria-hidden="true"
		>
			<Avatar
				src={src}
				alt=""
				entity="seller"
				className="w-full h-full object-cover"
				loading="eager"
				fallback="empty"
			/>
		</span>
	);
}

function ProductThumb({ src, alt }: { src?: string; alt: string }) {
	return (
		<span className="w-[56px] h-[56px] rounded-hkui-md border border-divider bg-content2 inline-flex items-center justify-center overflow-hidden shrink-0">
			<AppImage
				src={resolveCdnUrl(src)}
				alt={alt}
				width={56}
				height={56}
				sizes="56px"
				loading="eager"
				className="w-full h-full object-cover"
			/>
		</span>
	);
}

function CheckoutItemRow({ item }: { item: CartItem }) {
	const content = (
		<>
			<ProductThumb src={item.image} alt={item.name} />
			<div className="flex-1 min-w-0 flex flex-col gap-[2px]">
				<strong className="text-sm font-extrabold text-foreground block truncate">
					{item.name}
				</strong>
				<span className="text-xs font-bold text-foreground-soft">
					{item.unit ? `規格：${formatUnitLabel(item.unit)}` : "規格：預設規格"}{" "}
					· 數量 {item.quantity}
				</span>
				<span className="text-xs font-bold text-foreground-soft">
					單價 {money(item.priceValue)}
				</span>
			</div>
			<div className="flex flex-col items-end gap-[2px]">
				{item.listPrice && item.listPrice > item.priceValue ? (
					<small className="text-xs font-bold text-foreground-muted line-through">
						{money(item.listPrice * item.quantity)}
					</small>
				) : null}
				<b className="text-sm font-extrabold text-foreground">
					{money(item.priceValue * item.quantity)}
				</b>
			</div>
		</>
	);

	return (
		<li>
			{item.href ? (
				<Link
					href={item.href}
					className="flex items-center gap-3 p-3 rounded-hkui-lg bg-content2 border border-divider no-underline text-inherit transition-colors duration-150 hover:border-divider-strong hover:bg-content1"
				>
					{content}
				</Link>
			) : (
				<div className="flex items-center gap-3 p-3 rounded-hkui-lg bg-content2 border border-divider no-underline text-inherit transition-colors duration-150 hover:border-divider-strong hover:bg-content1">
					{content}
				</div>
			)}
		</li>
	);
}

function ShippingIcon() {
	return (
		<span
			className="w-[42px] h-[28px] rounded-hkui-md border border-divider bg-content2 inline-flex items-center justify-center overflow-hidden shrink-0"
			aria-hidden="true"
		>
			<AppIcon name="truck" size={20} fill className="text-hk-primary" />
		</span>
	);
}

function LockedCard({
	icon,
	label,
	value,
	detail,
	className,
}: {
	icon?: React.ReactNode;
	label: string;
	value: string;
	detail?: string;
	className?: string;
}) {
	return (
		<div
			className={`flex items-center gap-3 p-[12px_14px] bg-content2 border border-divider rounded-hkui-lg${className ? ` ${className}` : ""}`}
		>
			{icon}
			<div className="flex-1 min-w-0 flex flex-col gap-[2px]">
				<span className="text-[0.625rem] font-extrabold text-hk-accent tracking-[0.05em]">
					{label}
				</span>
				<strong className="text-xs font-extrabold text-foreground block truncate">
					{value}
				</strong>
				{detail ? (
					<p className="m-0 text-[0.625rem] font-bold text-foreground-soft">
						{detail}
					</p>
				) : null}
			</div>
		</div>
	);
}
