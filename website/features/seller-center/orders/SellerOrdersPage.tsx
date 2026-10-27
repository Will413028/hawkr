"use client";

/**
 * 賣家訂單列表。`/app/orders?view=seller` 已由 gateway 依 JWT session 圈定範圍，
 * 前端**不要**再做 client-side re-scoping（賣家 App 端同樣沒做，行為一致）。
 *
 * 曾有一道 guard 拿訂單的 `storeId` 去比對賣家自身記錄的 `account_id`——那是兩個
 * 不同的 id 空間（見設計書 44 買賣家身分分離），結果每一列合法資料都被濾掉：訂單
 * 列表、儀表板「待出貨」區塊、訂單詳情頁全空白，統計卡卻因直接讀同一份回應的
 * `total` 而顯示真實數字。真要再加，先確認比對的是同一個 id 空間。
 */

import { useSearchParams } from "next/navigation";
import {
	debounce,
	parseAsString,
	parseAsStringEnum,
	useQueryState,
	useQueryStates,
} from "nuqs";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { TruckOverlay, TruckSpinner } from "@/components/shared/TruckSpinner";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import Button from "@/components/ui/Button";
import { ImageUploader } from "@/components/ui/ImageUploader";
import { TemperatureTag } from "@/features/cart/components/TemperatureTag";
import {
	CartIcon,
	NoteIcon,
	PhoneIcon,
	SHIPMENT_PHOTO_TILE_STYLE,
	ShippingBrandLogo,
} from "@/features/seller/components/ShipmentDialogs";
import { ApiClientError } from "@/lib/api/client";
import {
	formatShippingMethodLabel,
	REFUND_REASON_LABEL,
	type RefundReasonValue,
	ShippingMethod,
} from "@/lib/codemeta/enums";
import { downloadCsv } from "@/lib/csv/export";
import { formatTaipeiDateTime, formatTaipeiShortDate } from "@/lib/date/taipei";
import { useAllSellerOrders } from "@/lib/hooks/useOrders";
import { useSellerAfterSales } from "@/lib/hooks/useSellerAfterSales";
import { useSellerCustomerLookup } from "@/lib/hooks/useSellerCustomerLookup";
import { useSellerDashboard } from "@/lib/hooks/useSellerDashboard";
import {
	useSellerShipmentAnomalies,
	useSellerShipmentAnomalyStats,
} from "@/lib/hooks/useSellerShipmentAnomalies";
import { uploadImageFile } from "@/lib/media/upload";
import { LOCATION_CHANGE_EVENT } from "@/lib/navigation/location-change";
import { useNotificationStreamStatus } from "@/lib/notifications/unread-context";
import { fetchOrder } from "@/lib/orders/repo";
import type { Order } from "@/lib/orders/types";
import { STATUS_LABEL } from "@/lib/orders/types";
import {
	AFTER_SALE_STATUS_LABEL,
	type AfterSaleCase,
	type AfterSaleGroup,
	afterSaleGroupOf,
	afterSaleIsTerminal,
	afterSaleNeedsSeller,
} from "@/lib/seller/after-sales";
import {
	formatNullableCount,
	type SellerOrderTabCounts,
} from "@/lib/seller/order-dashboard-counts";
import {
	type BusinessDay,
	callTcat,
	classifyShipmentError,
	confirmShipment,
	fetchShippingBusinessDays,
	generateHomeLabel,
	guardShipmentCooldown,
	isLabelExpired,
	isRunnableShipmentAction,
	markShipmentCreated,
	nextShipmentAction,
	printCvsLabel,
	type RunnableShipmentActionKind,
	resetShipmentRetryCount,
	type ShipmentAction,
	setLocalOdnoOverride,
} from "@/lib/seller/shipment";
import type { ShipmentAnomalyItem } from "@/lib/shipping/anomalies";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import { SellerEmptyState } from "../components/SellerEmptyState";
import { SellerOrdersSkeletonRows } from "../components/SellerPageSkeletons";
import { SummaryStatCard } from "../components/SummaryStatCard";
import { AfterSalePanel } from "./AfterSalePanel";
import CancelOrderDialog from "./CancelOrderDialog";
import { deliveryTempLabel, formatDeadline } from "./orderDisplay";
import {
	SELLER_ORDER_TAB_LABEL,
	type SellerOrderTabKey,
	sellerOrderTabFor,
} from "./orderTabs";
import { RealtimeSyncNotice } from "./RealtimeSyncNotice";
import SellerOrderDetailDrawer from "./SellerOrderDetailDrawer";
import {
	isShipmentOperationExpired,
	type SellerShipmentOperation,
	SHIPMENT_FALLBACK_REFRESH_DELAY_MS,
	shipmentOperationExpiresAt,
	shipmentOperationLabel,
	useSellerShipmentOperationStore,
} from "./shipmentOperationStore";

/**
 * Seller-side orders table — aligned with the seller APP's
 * `order_list_screen.dart` flow (3-tab grouping + quick-filter chips).
 *
 * Tab mapping mirrors `order_tab_mapper.dart` in the seller APP:
 *   - 待出貨   → status codes 10/20 (paid / preparing)
 *   - 配送中   → 30/32/35/40 (shipping / delivered), unless the order has an
 *                active/closed after-sale case
 *   - 已結束   → 50/60/70/80 (completed / refunded / cancelled) —— 桶的語意是
 *                「賣家無需再動作」，**不是成交數**（含 80 已取消）。標籤一律取自
 *                `SELLER_ORDER_TAB_LABEL`，不要在這裡寫成「已完成」：那是 CodeMeta
 *                OrderStatus 50 單一狀態的 Label（見 orderTabs.ts 的說明）。
 *
 * Quick-filter chips mirror APP's `_OrderQuickFilter`:
 *   - 全部:      no extra filter
 *   - 待處理:    orders that need the seller's next action (nextShipmentAction
 *               returns a non-noop kind, OR an active after_sale)
 *   - 售後處理:  standalone case table; after-sale orders do not stay in the
 *               generic 配送中 list just because the order lifecycle is delivered
 *
 * The whole fulfilment queue is fetched once (`useAllSellerOrders`, no
 * `status=` filter) and classified client-side by `sellerOrderTabFor`. Each
 * `TabDef.statuses` (web semantic names like "paid"/"preparing") only backs the
 * legacy `?status=` deep-link → tab mapping in `tabFromQuery`.
 *
 * 四張狀態卡與 tab 徽章的數字**一律取自後端 dashboard**（#1188 起是權威，
 * 見 :623 的實作註解）；表格則分頁呈現當前 tab 的桶（「載入更多」）。後端與
 * 本頁採同一套分類規則，所以卡片數 == 該 tab 打開後看到的列數。
 *
 * ⚠️ 這一段在 9a5598c4 改成 dashboard 權威之後，有一段時間仍寫著「dashboard
 * 不再是計數來源 —— 它與清單不一致」的舊敘述，與 :623 直接矛盾。那正是
 * 「數字 vs 清單」這類回報難以判讀的根源（#377 item3 就是因此被誤報成 bug：
 * 賣家真的有 27 筆已結束、0 筆待出貨，而預設 tab 是待出貨，於是「共 27 筆 +
 * 全部 0 + 空清單」同時出現在畫面上，卻是正確渲染）。判別法：看四張卡裡哪
 * 一張是選中的。
 */

type TabKey = SellerOrderTabKey;
type QuickFilter = "all" | "pending_action" | "anomaly";

const TAB_KEYS: ReadonlyArray<TabKey> = [
	"pending",
	"in_transit",
	"completed",
	"aftersale",
];
const QUICK_FILTERS: ReadonlyArray<QuickFilter> = [
	"all",
	"pending_action",
	"anomaly",
];
/** Rows revealed per 「載入更多」 click — an in-memory window over the already
 * fully loaded order set (the whole queue is fetched up front for counts). */
const ORDER_PAGE_SIZE = 20;
const ORDER_URL_STATE_OPTIONS = {
	history: "replace",
	shallow: true,
	scroll: false,
} as const;
const tabQueryParser = parseAsStringEnum<TabKey>([...TAB_KEYS])
	.withOptions(ORDER_URL_STATE_OPTIONS)
	.withDefault("pending");
const quickFilterQueryParser = parseAsStringEnum<QuickFilter>([
	...QUICK_FILTERS,
])
	.withOptions(ORDER_URL_STATE_OPTIONS)
	.withDefault("all");
const orderSearchQueryParser = parseAsString
	.withOptions({
		...ORDER_URL_STATE_OPTIONS,
		limitUrlUpdates: debounce(250),
	})
	.withDefault("");
const legacyOrderQueryParser = parseAsString.withOptions(
	ORDER_URL_STATE_OPTIONS,
);

/** True when the order is a 配送異常 — detected shipment anomaly or the order
 * itself flipped into the shipping_abnormal lane. Drives the 配送異常 quick
 * filter + its chip count (only meaningful on the 配送中 tab). */
function orderHasAnomaly(
	order: Order,
	anomalyLookup: Map<string, ShipmentAnomalyItem>,
): boolean {
	return (
		anomalyLookup.has(order.orderNo) || order.status === "shipping_abnormal"
	);
}

interface TabDef {
	key: TabKey;
	label: string;
	/** Semantic statuses owned by the tab — used only by `tabFromQuery()` to map
	 * a legacy `?status=` deep-link onto its owning tab. Live classification of
	 * loaded orders runs through `sellerOrderTabFor`, not this list. */
	statuses: ReadonlyArray<string>;
}

const TAB_DEFS: ReadonlyArray<TabDef> = [
	{
		key: "pending",
		label: SELLER_ORDER_TAB_LABEL.pending,
		statuses: ["paid", "preparing"],
	},
	{
		key: "in_transit",
		label: SELLER_ORDER_TAB_LABEL.in_transit,
		statuses: ["shipping", "waiting_pickup", "shipping_abnormal", "delivered"],
	},
	{
		key: "completed",
		label: SELLER_ORDER_TAB_LABEL.completed,
		statuses: [
			"completed",
			"cancelled",
			"refunded",
			"partially_refunded",
			"after_sale",
		],
	},
];

/** Display labels for each bulk-action kind (key = ShipmentAction["kind"]). */
const ACTION_LABEL: Record<ShipmentAction["kind"], string> = {
	"tcat-reserve": "預約出貨",
	"tcat-call": "呼叫黑貓取件",
	"tcat-pdf": "下載託運單 PDF",
	"cvs-print": "預約取號",
	"cvs-confirm": "出貨確認",
	none: "—",
	// 未知配送方式沒有可執行的出貨動作 —— 這一格永遠不會被渲染成按鈕
	// （`isRunnableShipmentAction` 先擋掉），列在這裡只為讓型別窮舉成立。
	unsupported: "—",
};

/** Each kind maps to the same brand tone used by status badges so the
 * bulk-action bar's button colors don't fight with the row badges:
 *   - primary (primary)  — forward shipping actions (預約 / 呼叫 / 確認)
 *   - info (primary)     — non-destructive utility (下載 PDF / 取號)
 */
const ACTION_TONE: Record<ShipmentAction["kind"], "primary" | "info"> = {
	"tcat-reserve": "primary",
	"tcat-call": "primary",
	"tcat-pdf": "info",
	"cvs-print": "info",
	"cvs-confirm": "primary",
	none: "primary",
	unsupported: "primary",
};

// 🔴 批次列只收「真的按得下去」的動作。`unsupported`（未知配送方式）不能進來——
// 一旦進來，賣家可以把一批大宗寄倉／商家自送的訂單勾起來一次送超商取號。
type BulkKind = RunnableShipmentActionKind;
type RunBatchOptions = { skipPhoto?: boolean };

type BulkTarget = { kind: BulkKind; orderNos: string[] } | null;

/** Compute the next-action kind for each selected order — drives the
 * bulk-action bar's per-kind button row. */
function groupSelectionByKind(
	orders: ReadonlyArray<Order>,
	selected: ReadonlySet<string>,
	pendingReservationOrderNos: ReadonlySet<string>,
): Record<BulkKind, Order[]> {
	const out: Record<BulkKind, Order[]> = {
		"tcat-reserve": [],
		"tcat-call": [],
		"tcat-pdf": [],
		"cvs-print": [],
		"cvs-confirm": [],
	};
	for (const o of orders) {
		if (!selected.has(o.orderNo)) continue;
		if (pendingReservationOrderNos.has(o.orderNo)) continue;
		const action = nextShipmentAction(o.orderNo, o.shipping, o.status);
		if (!isRunnableShipmentAction(action)) continue;
		out[action.kind].push(o);
	}
	return out;
}

/** True when the order needs the seller's attention — used by 「待處理」.
 *
 * Mirrors APP `_needsSellerAction()`:
 *   - 售後 case 在 PENDING(0) / RETURN_RECEIVED(30) → 需處理
 *   - 待出貨 (paid / preparing) 且有非 noop 出貨動作 → 需處理
 */
function needsSellerAction(
	order: Order,
	afterSale: AfterSaleCase | undefined,
	reservationPending = false,
): boolean {
	if (afterSaleNeedsSeller(afterSale)) return true;
	if (reservationPending) return false;
	if (order.status === "paid" || order.status === "preparing") {
		const action = nextShipmentAction(
			order.orderNo,
			order.shipping,
			order.status,
		);
		return isRunnableShipmentAction(action);
	}
	return false;
}

/** Resolve the initial tab from the URL. Deep-link forms honoured:
 *   - `?tab=aftersale` — the 售後處理 tab (退換貨 cases).
 *   - `?tab=anomalies` (legacy) — the standalone 配送異常 tab was folded into
 *     配送中 (anomalous orders ride inline in the list); map it to `in_transit`.
 *   - `?tab=exceptions` (legacy) — the old 異常處理 umbrella tab. Its 配送異常
 *     sub-view now lives inline under 配送中, its 售後處理 sub-view became the
 *     standalone 售後處理 tab — so route by `?view=`.
 *   - `?status=preparing` (legacy) — maps a semantic status to its owning tab.
 * Anything else lands on 待出貨. */
function tabFromQuery(
	tab: string | null | undefined,
	status: string | null | undefined,
	view: string | null | undefined,
): TabKey {
	if (tab) {
		if (tab === "anomalies") return "in_transit"; // legacy alias
		if (tab === "exceptions") {
			// legacy umbrella tab: 配送異常 → 配送中, 售後處理 → 售後處理
			return view === "anomaly" ? "in_transit" : "aftersale";
		}
		if (TAB_KEYS.includes(tab as TabKey)) return tab as TabKey;
	}
	if (status) {
		if (status === "after_sale") return "aftersale";
		for (const def of TAB_DEFS) {
			if (def.statuses.includes(status)) return def.key;
		}
	}
	return "pending";
}

function isLegacyTabQuery(tab: string | null | undefined): boolean {
	if (!tab) return false;
	return tab === "anomalies" || tab === "exceptions";
}

interface OrderViewState {
	tab: TabKey;
	filter: QuickFilter;
	caseNo: string | null;
}

function orderViewStateFromSearchParams(
	params: URLSearchParams,
): OrderViewState {
	const filter = params.get("filter");
	return {
		tab: tabFromQuery(
			params.get("tab"),
			params.get("status"),
			params.get("view"),
		),
		filter: QUICK_FILTERS.includes(filter as QuickFilter)
			? (filter as QuickFilter)
			: "all",
		caseNo: params.get("caseNo"),
	};
}

export default function SellerOrdersPage() {
	const params = useSearchParams();
	const queryTab = params?.get("tab");
	const [queryStatus, setQueryStatusParam] = useQueryState(
		"status",
		legacyOrderQueryParser,
	);
	const [queryView, setQueryViewParam] = useQueryState(
		"view",
		legacyOrderQueryParser,
	);
	const [
		{ tab: urlActiveTab, filter: urlQuickFilter, caseNo: urlQueryCaseNo },
		setOrderViewParams,
	] = useQueryStates({
		tab: tabQueryParser,
		filter: quickFilterQueryParser,
		caseNo: legacyOrderQueryParser,
	});
	const [search, setSearchParam] = useQueryState("q", orderSearchQueryParser);
	const [focusOrderNo, setFocusOrderNoParam] = useQueryState(
		"focus",
		parseAsString,
	);
	// Next's shallow-history bridge can briefly replay its previous search-param
	// snapshot after a URL write. Keep the visible work queue locally controlled;
	// URL commits and back/forward only sync from the address bar's final value.
	const [orderView, setOrderView] = React.useState<OrderViewState>(() => ({
		tab: urlActiveTab,
		filter: urlQuickFilter,
		caseNo: urlQueryCaseNo,
	}));
	const {
		tab: activeTab,
		filter: quickFilter,
		caseNo: queryCaseNo,
	} = orderView;
	React.useEffect(() => {
		const syncFromLocation = () => {
			const next = orderViewStateFromSearchParams(
				new URLSearchParams(window.location.search),
			);
			setOrderView((current) =>
				current.tab === next.tab &&
				current.filter === next.filter &&
				current.caseNo === next.caseNo
					? current
					: next,
			);
		};
		window.addEventListener("popstate", syncFromLocation);
		window.addEventListener(LOCATION_CHANGE_EVENT, syncFromLocation);
		return () => {
			window.removeEventListener("popstate", syncFromLocation);
			window.removeEventListener(LOCATION_CHANGE_EVENT, syncFromLocation);
		};
	}, []);
	const setQueryCaseNoParam = React.useCallback(
		(caseNo: string | null) => {
			setOrderView((current) => ({ ...current, caseNo }));
			return setOrderViewParams({ caseNo });
		},
		[setOrderViewParams],
	);
	const queryFocusOrderNo = focusOrderNo?.trim() || null;
	const initialCaseNo = queryCaseNo?.trim() || undefined;
	const setQuickFilter = React.useCallback(
		(filter: QuickFilter) => {
			setOrderView((current) => ({ ...current, filter }));
			void setOrderViewParams({ filter });
		},
		[setOrderViewParams],
	);
	// One fetch for the WHOLE fulfilment queue (every lifecycle stage). The tabs
	// are slices of this same set, classified client-side, so a card count can
	// never disagree with — or jump away from — the rows under it.
	// `view: "seller"` is the crucial bit — `/app/orders` defaults to the BUYER
	// view (the account's own purchases). Without it the seller order page shows
	// what the account BOUGHT, not what it needs to fulfil.
	const { orders, loading, refreshing, refresh } = useAllSellerOrders({
		view: "seller",
	});
	const shipmentOperations = useSellerShipmentOperationStore(
		(state) => state.operations,
	);
	const clearShipmentOperation = useSellerShipmentOperationStore(
		(state) => state.clearOperation,
	);
	const pruneExpiredShipmentOperations = useSellerShipmentOperationStore(
		(state) => state.pruneExpired,
	);
	// Dashboard is the authoritative source for the four status-card counts
	// (#1188): order-service computes the same four buckets the tabs use —
	// lifecycle-by-status + after-sale override + self-purchase exclusion — so we
	// no longer derive the card numbers locally. We still refresh it after a
	// shipment / cancel action so the sidebar badges (which read it) stay current.
	const { data: sellerDashboard, refresh: refreshSellerDashboard } =
		useSellerDashboard();
	// `/app/orders?view=seller` already excludes true self-purchases (orders where
	// the same account is both buyer and seller) at the backend (#1188) — the
	// seller workbench only shows orders this account must fulfil for others. No
	// client-side self-purchase filter is needed any more; render what it returns.
	const scopedOrders = orders;
	const orderByNo = React.useMemo(
		() => new Map(scopedOrders.map((order) => [order.orderNo, order])),
		[scopedOrders],
	);
	const reservationOperationByOrderNo = React.useMemo(() => {
		const pending = new Map<string, SellerShipmentOperation>();
		for (const [orderNo, operation] of Object.entries(shipmentOperations)) {
			if (
				operation.kind !== "tcat-reserve" ||
				isShipmentOperationExpired(operation)
			) {
				continue;
			}
			const order = orderByNo.get(orderNo);
			if (!order) continue;
			const action = nextShipmentAction(
				order.orderNo,
				order.shipping,
				order.status,
			);
			if (action.kind === "tcat-reserve") pending.set(orderNo, operation);
		}
		return pending;
	}, [orderByNo, shipmentOperations]);
	const pendingReservationOrderNos = React.useMemo(
		() => new Set(reservationOperationByOrderNo.keys()),
		[reservationOperationByOrderNo],
	);
	// After-sale cases. `lookup` (keyed by orderNo) feeds the 待處理 chip count
	// and the order-row red dot; `afterSaleRecords` is the raw list backing the
	// 售後處理 case table on the 售後處理 tab — one fetch shared by both.
	const {
		records: afterSaleRecords,
		lookup: afterSaleLookup,
		loading: afterSaleLoading,
		refresh: refreshAfterSales,
	} = useSellerAfterSales();
	// Shipment-anomaly lookup keyed by orderNo. Anomalous orders aren't a
	// separate tab/panel any more — they ride inline in the 配送中 list and just
	// surface a 「異常」 badge in the 訂單狀態 column (see `OrderRow`); clicking the
	// row opens the order-detail drawer, which shows the anomaly reason.
	const { lookup: anomalyLookup, refresh: refreshAnomalies } =
		useSellerShipmentAnomalies();
	// 賣家工作區 realtime：後端 webhook / 物流 / 售後 / 排程改狀態時不重整頁面即同步
	// （統計卡 / 訂單列 / 售後列 / drawer），由外框的 `useSellerWorkspaceRealtime` 讓快取失效。
	// 連線狀態必須接出來：連線被 429 回絕或重連預算耗盡時串流會停止供應更新且不報錯，
	// 丟掉它等於讓賣家在毫無提示的情況下失去即時同步——畫面照常，只是新訂單再也不會自己出現。
	const realtimeStatus = useNotificationStreamStatus();
	// The 配送異常 quick-filter badge must mirror the backend rollup exactly.
	// Do not derive this number from currently loaded order rows: pagination,
	// order visibility, or orderNo de-dupe can make the row count differ from
	// `/app/shipments/anomalies/stats.total`.
	const { stats: anomalyStats, refresh: refreshAnomalyStats } =
		useSellerShipmentAnomalyStats();
	const [anomalyOrderOverrides, setAnomalyOrderOverrides] = React.useState<
		Map<string, Order>
	>(() => new Map());

	// 售後處理 tab filters (the case table filters `afterSaleRecords`
	// client-side; see AfterSalePanel). `顯示已結案` lives in the header so it
	// sits next to 重新整理.
	const [afterSaleGroup, setAfterSaleGroup] =
		React.useState<AfterSaleGroup>("all");
	const [afterSaleShowClosed, setAfterSaleShowClosed] = React.useState(false);
	React.useEffect(() => {
		if (!afterSaleShowClosed && afterSaleGroup === "closed") {
			setAfterSaleGroup("all");
		}
	}, [afterSaleGroup, afterSaleShowClosed]);
	// `scopedOrders` is first filtered by the current seller id. The defensive
	// `sellerOrderTabFor` re-check catches unexpected statuses so rows are hidden
	// rather than rendered in the wrong tab. After-sale cases are layered on top
	// so a delivered order with an active/closed case no longer pollutes 配送中.
	const tabOrders = React.useMemo(
		() =>
			scopedOrders.filter(
				(o) =>
					sellerOrderTabFor(o, afterSaleLookup.get(o.orderNo)) === activeTab,
			),
		[scopedOrders, afterSaleLookup, activeTab],
	);

	React.useEffect(() => {
		if (activeTab !== "in_transit" || anomalyLookup.size === 0) {
			setAnomalyOrderOverrides((prev) => (prev.size === 0 ? prev : new Map()));
			return;
		}

		const anomalyOrderNos = new Set(anomalyLookup.keys());
		setAnomalyOrderOverrides((prev) => {
			let changed = false;
			const next = new Map<string, Order>();
			for (const [orderNo, order] of prev) {
				if (anomalyOrderNos.has(orderNo)) next.set(orderNo, order);
				else changed = true;
			}
			return changed ? next : prev;
		});

		const loadedOrderNos = new Set(scopedOrders.map((o) => o.orderNo));
		const missingOrderNos = [...anomalyOrderNos].filter(
			(orderNo) =>
				!loadedOrderNos.has(orderNo) && !anomalyOrderOverrides.has(orderNo),
		);
		if (missingOrderNos.length === 0) return;

		let cancelled = false;
		void Promise.all(
			missingOrderNos.map(async (orderNo) => {
				try {
					return [
						orderNo,
						await fetchOrder(orderNo, { view: "seller" }),
					] as const;
				} catch {
					return [orderNo, null] as const;
				}
			}),
		).then((pairs) => {
			if (cancelled) return;
			setAnomalyOrderOverrides((prev) => {
				let changed = false;
				const next = new Map(prev);
				for (const [orderNo, order] of pairs) {
					if (order && !next.has(orderNo)) {
						next.set(orderNo, order);
						changed = true;
					}
				}
				return changed ? next : prev;
			});
		});

		return () => {
			cancelled = true;
		};
	}, [activeTab, anomalyLookup, anomalyOrderOverrides, scopedOrders]);

	const anomalyFilterOrders = React.useMemo(() => {
		const merged = new Map<string, Order>();
		for (const order of tabOrders) merged.set(order.orderNo, order);
		for (const order of anomalyOrderOverrides.values()) {
			merged.set(order.orderNo, order);
		}
		return [...merged.values()].filter((o) =>
			orderHasAnomaly(o, anomalyLookup),
		);
	}, [tabOrders, anomalyLookup, anomalyOrderOverrides]);

	const filtered = React.useMemo(() => {
		switch (quickFilter) {
			case "pending_action":
				return tabOrders.filter((o) =>
					needsSellerAction(o, afterSaleLookup.get(o.orderNo)),
				);
			case "anomaly":
				return anomalyFilterOrders;
			default:
				return tabOrders;
		}
	}, [tabOrders, quickFilter, afterSaleLookup, anomalyFilterOrders]);

	const visible = React.useMemo(() => {
		const q = search.trim().toLowerCase();
		if (!q) return filtered;
		return filtered.filter((o) => {
			if (o.orderNo.toLowerCase().includes(q)) return true;
			const name = o.shipping?.receiverName?.toLowerCase() ?? "";
			if (name.includes(q)) return true;
			const phone = o.shipping?.receiverPhone ?? "";
			if (phone.includes(q)) return true;
			return false;
		});
	}, [filtered, search]);
	// The full set is already in memory, so 「載入更多」 just widens an in-memory
	// window instead of fetching. Reset to the first page whenever the tab /
	// quick-filter / search changes so a deep scroll doesn't leak across views.
	const [visibleLimit, setVisibleLimit] = React.useState(ORDER_PAGE_SIZE);
	// biome-ignore lint/correctness/useExhaustiveDependencies: activeTab/quickFilter/search are the reset signal; we intentionally re-window when any flips.
	React.useEffect(() => {
		setVisibleLimit(ORDER_PAGE_SIZE);
	}, [activeTab, quickFilter, search]);
	const pagedVisible = React.useMemo(
		() => visible.slice(0, visibleLimit),
		[visible, visibleLimit],
	);
	const hasMore = visible.length > visibleLimit;
	const loadMore = React.useCallback(() => {
		setVisibleLimit((n) => n + ORDER_PAGE_SIZE);
	}, []);
	const afterSaleVisibleRecords = React.useMemo(() => {
		const byClosed = afterSaleShowClosed
			? afterSaleRecords
			: afterSaleRecords.filter((c) => !afterSaleIsTerminal(c));
		const byGroup =
			afterSaleGroup === "all"
				? byClosed
				: byClosed.filter((c) => afterSaleGroupOf(c) === afterSaleGroup);
		const q = search.trim().toLowerCase();
		if (!q) return byGroup;
		return byGroup.filter(
			(c) =>
				c.caseNo.toLowerCase().includes(q) ||
				c.orderNo.toLowerCase().includes(q) ||
				(c.buyerName ?? "").toLowerCase().includes(q),
		);
	}, [afterSaleGroup, afterSaleRecords, afterSaleShowClosed, search]);
	// 四張狀態卡 + tab 徽章數字一律取自後端 dashboard（#1188 權威：lifecycle-by-
	// status + 售後覆蓋 + 自買排除）。後端與本頁列表採同一套分類規則 —— 三桶以
	// order.status 分類、售後 active case 從生命週期排除並計入售後 queue、自買單
	// 由後端列表一併排除 —— 故各卡片數 == 對應 tab 的列表筆數。dashboard 尚未載入
	// 或訂單狀態區塊沒設時為 null，避免閃出誤導的 0。秒級的黑貓叫件樂觀窗口（前端 isTcatCalled 先把
	// row 移到配送中、後端 order.status 由 MQ 稍後推進）為設計接受的暫態差異。
	const orderStatusCounts = sellerDashboard?.orderStatusCounts;
	const displayTabCounts = React.useMemo<SellerOrderTabCounts>(
		() => ({
			pending: orderStatusCounts?.pendingShipmentCount ?? null,
			in_transit: orderStatusCounts?.inProgressCount ?? null,
			completed: orderStatusCounts?.closedCount ?? null,
			aftersale: orderStatusCounts?.aftersalesCount ?? null,
		}),
		[orderStatusCounts],
	);
	const visibleOrderTotal = orderStatusCounts
		? orderStatusCounts.pendingShipmentCount +
			orderStatusCounts.inProgressCount +
			orderStatusCounts.closedCount
		: null;

	const pendingActionCount = React.useMemo(
		() =>
			tabOrders.filter((o) =>
				needsSellerAction(
					o,
					afterSaleLookup.get(o.orderNo),
					pendingReservationOrderNos.has(o.orderNo),
				),
			).length,
		[tabOrders, afterSaleLookup, pendingReservationOrderNos],
	);

	const anomalyCount = anomalyStats.total;
	const [workspaceRefreshing, setWorkspaceRefreshing] = React.useState(false);
	const refreshOrderWorkspacePromiseRef = React.useRef<Promise<void> | null>(
		null,
	);
	const refreshOrderWorkspace = React.useCallback(() => {
		if (refreshOrderWorkspacePromiseRef.current) {
			return refreshOrderWorkspacePromiseRef.current;
		}
		setWorkspaceRefreshing(true);
		const pending = Promise.allSettled([
			refresh(),
			refreshAfterSales(),
			refreshSellerDashboard(),
			refreshAnomalies(),
			refreshAnomalyStats(),
		])
			.then(() => undefined)
			.finally(() => {
				if (refreshOrderWorkspacePromiseRef.current === pending) {
					refreshOrderWorkspacePromiseRef.current = null;
					setWorkspaceRefreshing(false);
				}
			});
		refreshOrderWorkspacePromiseRef.current = pending;
		return pending;
	}, [
		refresh,
		refreshAfterSales,
		refreshSellerDashboard,
		refreshAnomalies,
		refreshAnomalyStats,
	]);

	// Realtime events are the primary sync path. As soon as the order read model
	// advances, release the local operation lock; no interval polling is needed.
	React.useEffect(() => {
		pruneExpiredShipmentOperations();
		for (const [orderNo, operation] of Object.entries(shipmentOperations)) {
			const order = orderByNo.get(orderNo);
			if (!order || isShipmentOperationExpired(operation)) continue;
			const currentAction = nextShipmentAction(
				order.orderNo,
				order.shipping,
				order.status,
			);
			if (currentAction.kind !== operation.kind) {
				clearShipmentOperation(orderNo);
			}
		}
	}, [
		clearShipmentOperation,
		orderByNo,
		pruneExpiredShipmentOperations,
		shipmentOperations,
	]);

	// The mutation handler already refreshes once, and useSellerWorkspaceRealtime
	// invalidates the order query when the backend publishes its event. Do one
	// delayed fallback refresh for the whole pending set in case that event is
	// missed; a stable key guarantees this never becomes a polling loop.
	const pendingShipmentFallbackKey = React.useMemo(
		() =>
			Object.entries(shipmentOperations)
				.filter(
					([orderNo, operation]) =>
						pendingReservationOrderNos.has(orderNo) &&
						operation.phase === "syncing" &&
						!isShipmentOperationExpired(operation),
				)
				.map(
					([orderNo, operation]) =>
						`${orderNo}:${operation.phase}:${operation.updatedAt}`,
				)
				.sort()
				.join("|"),
		[pendingReservationOrderNos, shipmentOperations],
	);
	const refreshOrdersRef = React.useRef(refresh);
	refreshOrdersRef.current = refresh;
	React.useEffect(() => {
		if (!pendingShipmentFallbackKey) return;
		const timer = window.setTimeout(() => {
			pruneExpiredShipmentOperations();
			void refreshOrdersRef.current();
		}, SHIPMENT_FALLBACK_REFRESH_DELAY_MS);
		return () => window.clearTimeout(timer);
	}, [pendingShipmentFallbackKey, pruneExpiredShipmentOperations]);

	// Expiry is a local safety valve only. Schedule the nearest unlock without
	// any network request so a missed event can never leave the CTA stuck.
	const nextShipmentOperationExpiryAt = React.useMemo(() => {
		const expiries = Object.values(shipmentOperations)
			.filter((operation) => !isShipmentOperationExpired(operation))
			.map(shipmentOperationExpiresAt);
		return expiries.length > 0 ? Math.min(...expiries) : null;
	}, [shipmentOperations]);
	React.useEffect(() => {
		if (nextShipmentOperationExpiryAt == null) return;
		const delay = Math.max(0, nextShipmentOperationExpiryAt - Date.now() + 50);
		const timer = window.setTimeout(
			() => pruneExpiredShipmentOperations(),
			delay,
		);
		return () => window.clearTimeout(timer);
	}, [nextShipmentOperationExpiryAt, pruneExpiredShipmentOperations]);

	/* ===== Batch shipping ============================================
	 * Sellers often clear a queue of 待出貨 orders in one go (especially
	 * after a big sale day). Desktop keeps persistent table checkboxes; phones
	 * enter an explicit selection mode so ordinary card scanning stays clean.
	 * Selection is auto-cleared when the tab/quick filter changes so a hidden
	 * row never ends up silently in a batch.
	 * ================================================================ */
	const [selectedOrderNos, setSelectedOrderNos] = React.useState<Set<string>>(
		() => new Set(),
	);
	const [mobileSelectionMode, setMobileSelectionMode] = React.useState(false);
	const toggleSelectedOrder = React.useCallback((orderNo: string) => {
		setSelectedOrderNos((prev) => {
			const next = new Set(prev);
			if (next.has(orderNo)) next.delete(orderNo);
			else next.add(orderNo);
			return next;
		});
	}, []);
	const setSelectedOrders = React.useCallback((orderNos: Iterable<string>) => {
		setSelectedOrderNos(new Set(orderNos));
	}, []);
	const clearSelection = React.useCallback(() => {
		setSelectedOrderNos(new Set());
	}, []);
	// Clicking a row opens the full order detail in a right-side drawer (over the
	// list) instead of navigating away or expanding inline — same Sheet primitive
	// as the customer / after-sales drawers. Only one is open at a time, matching
	// the APP's per-card drawer feel. The `/orders/[orderNo]` route stays as the
	// deep-link / refresh fallback.
	const [detailOrderNo, setDetailOrderNo] = React.useState<string | null>(null);
	const [dismissedFocusOrderNo, setDismissedFocusOrderNo] = React.useState<
		string | null
	>(null);
	// biome-ignore lint/correctness/useExhaustiveDependencies: queryFocusOrderNo is the reset signal for the local URL-focus dismissal guard.
	React.useEffect(() => {
		setDismissedFocusOrderNo(null);
	}, [queryFocusOrderNo]);
	const effectiveDetailOrderNo =
		queryFocusOrderNo && dismissedFocusOrderNo !== queryFocusOrderNo
			? queryFocusOrderNo
			: detailOrderNo;
	const [bulkTarget, setBulkTarget] = React.useState<BulkTarget>(null);
	const bulkTargetOrders = React.useMemo(() => {
		if (!bulkTarget) return [];
		const lookup = new Map<string, Order>();
		for (const order of [...orders, ...anomalyOrderOverrides.values()]) {
			lookup.set(order.orderNo, order);
		}
		return bulkTarget.orderNos
			.map((orderNo) => lookup.get(orderNo))
			.filter((order): order is Order => Boolean(order));
	}, [anomalyOrderOverrides, bulkTarget, orders]);
	React.useEffect(() => {
		if (bulkTarget && bulkTargetOrders.length === 0) {
			setBulkTarget(null);
		}
	}, [bulkTarget, bulkTargetOrders.length]);
	const setActiveTab = React.useCallback(
		(tab: TabKey) => {
			if (tab === activeTab) return;

			// A status-card click is one in-page state change. Commit the tab and its
			// default lens together so React never paints "new tab + old filter" or
			// lets nuqs restore one setter's stale URL snapshot over the other.
			setVisibleLimit(ORDER_PAGE_SIZE);
			clearSelection();
			setMobileSelectionMode(false);
			setDetailOrderNo(null);
			setOrderView((current) => ({
				...current,
				tab,
				filter: "all",
				caseNo: tab === "aftersale" ? current.caseNo : null,
			}));
			void setOrderViewParams({
				tab,
				filter: "all",
				...(tab === "aftersale" ? {} : { caseNo: null }),
			});
		},
		[activeTab, clearSelection, setOrderViewParams],
	);
	React.useEffect(() => {
		if (!isLegacyTabQuery(queryTab) && !queryStatus && !queryView) return;
		const nextTab = tabFromQuery(queryTab, queryStatus, queryView);
		if (nextTab !== activeTab) setActiveTab(nextTab);
		if (queryStatus) void setQueryStatusParam(null);
		if (queryView) void setQueryViewParam(null);
	}, [
		activeTab,
		queryTab,
		queryStatus,
		queryView,
		setActiveTab,
		setQueryStatusParam,
		setQueryViewParam,
	]);
	// biome-ignore lint/correctness/useExhaustiveDependencies: deps drive the trigger — close the drawer whenever the tab/filter flips so it never lingers over a now-hidden row.
	React.useEffect(() => {
		setDetailOrderNo(null);
	}, [activeTab, quickFilter, setDetailOrderNo]);
	// biome-ignore lint/correctness/useExhaustiveDependencies: deps drive the trigger — the body doesn't read activeTab/quickFilter/search, but we want a fresh selection set whenever the visible queue changes.
	React.useEffect(() => {
		clearSelection();
		setMobileSelectionMode(false);
	}, [activeTab, clearSelection, quickFilter, search]);

	// "Selectable" = a row whose next action is something we can batch.
	// Rows past 待出貨 (or with no action) are skipped so a tick can only
	// land on orders the bar can actually act on. Scoped to the rows currently
	// rendered (the 載入更多 window) so 全選 matches what the seller sees.
	const selectableOrderNos = React.useMemo(() => {
		const set = new Set<string>();
		for (const o of pagedVisible) {
			if (pendingReservationOrderNos.has(o.orderNo)) continue;
			const action = nextShipmentAction(o.orderNo, o.shipping, o.status);
			if (isRunnableShipmentAction(action)) set.add(o.orderNo);
		}
		return set;
	}, [pagedVisible, pendingReservationOrderNos]);

	const allSelected =
		selectableOrderNos.size > 0 &&
		[...selectableOrderNos].every((no) => selectedOrderNos.has(no));
	const someSelected =
		!allSelected &&
		[...selectableOrderNos].some((no) => selectedOrderNos.has(no));

	const toggleOne = (orderNo: string) => toggleSelectedOrder(orderNo);

	const toggleAll = () => {
		if (allSelected) {
			setSelectedOrders(
				[...selectedOrderNos].filter((no) => !selectableOrderNos.has(no)),
			);
			return;
		}
		setSelectedOrders(new Set([...selectedOrderNos, ...selectableOrderNos]));
	};

	const selectionByKind = React.useMemo(
		() =>
			groupSelectionByKind(
				[...orders, ...anomalyOrderOverrides.values()],
				selectedOrderNos,
				pendingReservationOrderNos,
			),
		[
			orders,
			anomalyOrderOverrides,
			selectedOrderNos,
			pendingReservationOrderNos,
		],
	);

	const handleBulkComplete = (succeeded: number) => {
		setBulkTarget(null);
		if (succeeded > 0) {
			clearSelection();
			setMobileSelectionMode(false);
			void refreshOrderWorkspace();
		}
	};

	const exportCsv = () => {
		// Quick CSV export — joins the rendered rows. For richer exports
		// (with pagination across all filtered orders) we'd need a backend
		// /app/sellers/me/orders/export endpoint; this covers the
		// "right-now I want a spreadsheet" case.
		const rows =
			activeTab === "aftersale"
				? [
						[
							"案件編號",
							"訂單編號",
							"買家",
							"原因",
							"可退金額",
							"狀態",
							"申請時間",
						],
						...afterSaleVisibleRecords.map((c) => [
							c.caseNo,
							c.orderNo,
							c.buyerName ?? "",
							REFUND_REASON_LABEL[c.reason as RefundReasonValue] ??
								c.reason ??
								"",
							String(c.refundableAmount ?? "0"),
							AFTER_SALE_STATUS_LABEL[c.status] ?? c.status,
							c.createdAt ?? "",
						]),
					]
				: [
						[
							"訂單編號",
							"下單時間",
							"顧客",
							"電話",
							"商品",
							"數量",
							"金額",
							"配送方式",
							"狀態",
						],
						...visible.map((o) => [
							o.orderNo,
							new Date(o.createdAt < 1e12 ? o.createdAt * 1000 : o.createdAt)
								.toISOString()
								.replace("T", " ")
								.slice(0, 19),
							o.shipping?.receiverName ?? "",
							o.shipping?.receiverPhone ?? "",
							o.items?.[0]?.name ?? "",
							String(
								(o.items ?? []).reduce(
									(sum, it) => sum + Number(it.quantity || 0),
									0,
								),
							),
							String(o.totalActualAmount ?? "0"),
							o.shipping?.method ?? "",
							STATUS_LABEL[o.status] || o.status,
						]),
					];
		downloadCsv(
			`${activeTab === "aftersale" ? "after-sales" : "orders"}-${new Date().toISOString().slice(0, 10)}.csv`,
			rows,
		);
	};
	const handleRefresh = refreshOrderWorkspace;

	const isAfterSaleTab = activeTab === "aftersale";
	const headerInitialLoading = isAfterSaleTab ? afterSaleLoading : loading;
	const headerBusy = headerInitialLoading || refreshing || workspaceRefreshing;
	const exportDisabled = isAfterSaleTab
		? afterSaleVisibleRecords.length === 0
		: visible.length === 0;

	return (
		<SellerPageShell
			className="seller-orders"
			title="訂單管理"
			actionsClassName={
				isAfterSaleTab ? undefined : "seller-orders__header-actions"
			}
			subtitle={
				isAfterSaleTab
					? "買家的退換貨申請，待處理的請優先回應。"
					: visibleOrderTotal == null
						? "訂單彙總載入中"
						: `共 ${visibleOrderTotal.toLocaleString()} 筆訂單`
			}
			actions={
				// Search / refresh / export stay anchored on every tab so switching
				// modes does not collapse the header actions and create CTA jitter.
				<>
					{isAfterSaleTab ? (
						<label className="seller-orders__toggle">
							<input
								type="checkbox"
								checked={afterSaleShowClosed}
								onChange={(e) => setAfterSaleShowClosed(e.target.checked)}
							/>
							<span>顯示已結案</span>
						</label>
					) : null}
					<div className="seller-orders__search">
						<IconSearch />
						<input
							type="search"
							value={search}
							onChange={(e) => {
								void setSearchParam(e.target.value);
							}}
							placeholder={
								isAfterSaleTab
									? "搜尋案件 / 訂單 / 買家…"
									: "搜尋訂單編號或顧客…"
							}
							aria-label={isAfterSaleTab ? "搜尋售後案件" : "搜尋訂單"}
						/>
					</div>
					<Button
						type="button"
						variant="bordered"
						color="default"
						size="sm"
						className="seller-orders__btn"
						onClick={() => void handleRefresh()}
						disabled={headerBusy}
						isLoading={headerBusy}
						loadingLabel={
							isAfterSaleTab ? "重新整理售後案件中…" : "重新整理訂單中…"
						}
					>
						<AppIcon name="refresh" size={14} />
						重新整理
					</Button>
					<button
						type="button"
						className="seller-orders__btn seller-orders__btn--primary"
						onClick={exportCsv}
						disabled={exportDisabled}
					>
						<IconDownload />
						匯出
					</button>
				</>
			}
		>
			<div
				className="seller-inventory__stats seller-orders__status-tabs"
				role="tablist"
				aria-label="訂單篩選"
			>
				<SummaryStatCard
					label={SELLER_ORDER_TAB_LABEL.pending}
					value={formatNullableCount(displayTabCounts.pending)}
					tint="accent"
					icon={<IconPackage />}
					active={activeTab === "pending"}
					onClick={() => setActiveTab("pending")}
				/>
				<SummaryStatCard
					label={SELLER_ORDER_TAB_LABEL.in_transit}
					value={formatNullableCount(displayTabCounts.in_transit)}
					tint="info"
					icon={<IconTruck />}
					active={activeTab === "in_transit"}
					onClick={() => setActiveTab("in_transit")}
				/>
				<SummaryStatCard
					label={SELLER_ORDER_TAB_LABEL.completed}
					value={formatNullableCount(displayTabCounts.completed)}
					tint="primary"
					icon={<IconCheck />}
					active={activeTab === "completed"}
					onClick={() => setActiveTab("completed")}
				/>
				<SummaryStatCard
					label={SELLER_ORDER_TAB_LABEL.aftersale}
					value={formatNullableCount(displayTabCounts.aftersale)}
					tint="accent"
					icon={<IconBell />}
					active={activeTab === "aftersale"}
					onClick={() => setActiveTab("aftersale")}
				/>
			</div>

			{/* 統計卡與清單之間：兩者一起停止更新，提示放在中間才同時涵蓋。 */}
			<RealtimeSyncNotice status={realtimeStatus} />

			{isAfterSaleTab ? (
				<AfterSalePanel
					records={afterSaleRecords}
					loading={afterSaleLoading}
					showClosed={afterSaleShowClosed}
					group={afterSaleGroup}
					search={search}
					onGroupChange={setAfterSaleGroup}
					onRefresh={handleRefresh}
					initialCaseNo={initialCaseNo}
					onInitialCaseConsumed={() => {
						void setQueryCaseNoParam(null);
					}}
				/>
			) : (
				<OrdersTabBody
					quickFilter={quickFilter}
					setQuickFilter={setQuickFilter}
					pendingActionCount={pendingActionCount}
					anomalyCount={anomalyCount}
					allCount={displayTabCounts[activeTab] ?? tabOrders.length}
					showAnomalyChip={activeTab === "in_transit"}
					selectedOrderNos={selectedOrderNos}
					mobileSelectionMode={mobileSelectionMode}
					setMobileSelectionMode={setMobileSelectionMode}
					selectionByKind={selectionByKind}
					clearSelection={clearSelection}
					setBulkTarget={(target) =>
						setBulkTarget({
							kind: target.kind,
							orderNos: target.orders.map((order) => order.orderNo),
						})
					}
					loading={loading}
					scopedOrders={scopedOrders}
					visible={pagedVisible}
					search={search}
					selectableOrderNos={selectableOrderNos}
					toggleOne={toggleOne}
					openDetail={setDetailOrderNo}
					afterSaleLookup={afterSaleLookup}
					anomalyLookup={anomalyLookup}
					reservationOperationByOrderNo={reservationOperationByOrderNo}
					handleRefresh={handleRefresh}
					allSelected={allSelected}
					someSelected={someSelected}
					toggleAll={toggleAll}
					hasMore={hasMore}
					loadMore={loadMore}
				/>
			)}

			{bulkTarget && bulkTargetOrders.length > 0 && (
				<BulkShipmentDialog
					kind={bulkTarget.kind}
					orders={bulkTargetOrders}
					anomalyLookup={anomalyLookup}
					onClose={() => setBulkTarget(null)}
					onComplete={handleBulkComplete}
				/>
			)}

			<SellerOrderDetailDrawer
				open={effectiveDetailOrderNo !== null}
				orderNo={effectiveDetailOrderNo}
				onMutated={handleRefresh}
				onClose={() => {
					setDetailOrderNo(null);
					if (queryFocusOrderNo) {
						setDismissedFocusOrderNo(queryFocusOrderNo);
						void setFocusOrderNoParam(null);
					}
					// A shipment / cancel action inside the drawer changes the order's
					// row state — refresh the list so it reflects on close.
					void handleRefresh();
				}}
			/>
		</SellerPageShell>
	);
}

/* ===== Orders tab body ============================================
 * The order-centric content (quick filters + bulk bar + table). Split
 * out of `SellerOrdersPage` so the page render can cleanly swap between
 * this and `<AfterSalePanel/>` on the 售後處理 tab without the bulk-dialog
 * and stat-card chrome leaking into the after-sale view.
 * ================================================================== */

interface OrdersTabBodyProps {
	quickFilter: QuickFilter;
	setQuickFilter: (q: QuickFilter) => void;
	pendingActionCount: number;
	anomalyCount: number;
	allCount: number;
	/** Show the 配送異常 quick-filter chip — only on the 配送中 tab, where
	 * anomalies live. */
	showAnomalyChip: boolean;
	selectedOrderNos: Set<string>;
	mobileSelectionMode: boolean;
	setMobileSelectionMode: (enabled: boolean) => void;
	selectionByKind: Record<BulkKind, Order[]>;
	clearSelection: () => void;
	setBulkTarget: (t: { kind: BulkKind; orders: Order[] }) => void;
	loading: boolean;
	scopedOrders: ReadonlyArray<Order>;
	visible: ReadonlyArray<Order>;
	search: string;
	selectableOrderNos: ReadonlySet<string>;
	toggleOne: (orderNo: string) => void;
	/** Open the full order detail in the right-side drawer. */
	openDetail: (orderNo: string) => void;
	afterSaleLookup: Map<string, AfterSaleCase>;
	anomalyLookup: Map<string, ShipmentAnomalyItem>;
	reservationOperationByOrderNo: ReadonlyMap<string, SellerShipmentOperation>;
	handleRefresh: () => void | Promise<void>;
	allSelected: boolean;
	someSelected: boolean;
	toggleAll: () => void;
	hasMore: boolean;
	loadMore: () => void | Promise<void>;
}

function OrdersTabBody({
	quickFilter,
	setQuickFilter,
	pendingActionCount,
	anomalyCount,
	allCount,
	showAnomalyChip,
	selectedOrderNos,
	mobileSelectionMode,
	setMobileSelectionMode,
	selectionByKind,
	clearSelection,
	setBulkTarget,
	loading,
	scopedOrders,
	visible,
	search,
	selectableOrderNos,
	toggleOne,
	openDetail,
	afterSaleLookup,
	anomalyLookup,
	reservationOperationByOrderNo,
	handleRefresh,
	allSelected,
	someSelected,
	toggleAll,
	hasMore,
	loadMore,
}: OrdersTabBodyProps) {
	const [expandedMobileOrderNo, setExpandedMobileOrderNo] = React.useState<
		string | null
	>(null);
	const exitMobileSelectionMode = React.useCallback(() => {
		clearSelection();
		setMobileSelectionMode(false);
	}, [clearSelection, setMobileSelectionMode]);

	// biome-ignore lint/correctness/useExhaustiveDependencies: these visible-list controls intentionally collapse any open phone card when their value changes.
	React.useEffect(() => {
		setExpandedMobileOrderNo(null);
	}, [mobileSelectionMode, quickFilter, search]);

	React.useEffect(() => {
		setExpandedMobileOrderNo((current) =>
			current && !visible.some((order) => order.orderNo === current)
				? null
				: current,
		);
	}, [visible]);

	return (
		<>
			<div className="seller-orders__quick-filters">
				<IconTune />
				<QuickFilterChip
					label="全部"
					count={allCount}
					selected={quickFilter === "all"}
					onSelect={() => setQuickFilter("all")}
				/>
				<QuickFilterChip
					label="待處理"
					count={pendingActionCount}
					tone="warning"
					selected={quickFilter === "pending_action"}
					onSelect={() => setQuickFilter("pending_action")}
				/>
				{showAnomalyChip && (
					<QuickFilterChip
						label="配送異常"
						count={anomalyCount}
						tone="danger"
						selected={quickFilter === "anomaly"}
						onSelect={() => setQuickFilter("anomaly")}
					/>
				)}
			</div>

			{selectedOrderNos.size > 0 && (
				<BulkActionBar
					className="seller-orders__bulk--desktop"
					selectedCount={selectedOrderNos.size}
					selectionByKind={selectionByKind}
					onClear={clearSelection}
					onPick={(kind, orders) => setBulkTarget({ kind, orders })}
				/>
			)}

			<div
				className="seller-orders__card seller-orders__desktop-view"
				data-loading={loading && scopedOrders.length > 0 ? "true" : undefined}
			>
				<div className="seller-orders__table-wrap">
					<table className="seller-orders__table seller-orders__table--orders">
						<thead>
							<tr>
								<th className="seller-orders__col-select">
									<label className="seller-orders__check">
										<input
											type="checkbox"
											aria-label="全選此頁可出貨訂單"
											checked={allSelected}
											ref={(el) => {
												if (el) el.indeterminate = someSelected;
											}}
											disabled={selectableOrderNos.size === 0}
											onChange={toggleAll}
										/>
									</label>
								</th>
								<th>訂單編號</th>
								<th>顧客資訊</th>
								<th>商品</th>
								<th className="is-center">數量</th>
								<th className="is-right">金額</th>
								<th className="is-center">配送方式</th>
								<th className="is-center">訂單狀態</th>
								<th className="is-center">操作</th>
							</tr>
						</thead>
						<tbody>
							{loading && scopedOrders.length === 0 ? (
								<SellerOrdersSkeletonRows rows={6} />
							) : visible.length === 0 ? (
								<tr>
									<td colSpan={9} className="seller-orders__empty">
										<SellerEmptyState
											variant="orders"
											searchQuery={search}
											seamless
										/>
									</td>
								</tr>
							) : (
								visible.map((o) => (
									<OrderRow
										key={o.orderNo}
										order={o}
										selectable={selectableOrderNos.has(o.orderNo)}
										selected={selectedOrderNos.has(o.orderNo)}
										onToggleSelect={() => toggleOne(o.orderNo)}
										onOpenDetail={() => openDetail(o.orderNo)}
										afterSale={afterSaleLookup.get(o.orderNo)}
										anomaly={anomalyLookup.get(o.orderNo)}
										shipmentOperation={reservationOperationByOrderNo.get(
											o.orderNo,
										)}
										onChanged={handleRefresh}
										onAction={(kind, target) =>
											setBulkTarget({ kind, orders: [target] })
										}
									/>
								))
							)}
						</tbody>
					</table>
				</div>
				{hasMore && (
					<div className="seller-orders__more">
						<Button
							variant="bordered"
							color="default"
							size="sm"
							onClick={() => void loadMore()}
						>
							載入更多
						</Button>
					</div>
				)}
			</div>

			<section
				className="seller-orders__mobile-view"
				aria-label="訂單清單"
				aria-busy={loading && scopedOrders.length > 0 ? true : undefined}
			>
				{(selectableOrderNos.size > 0 || mobileSelectionMode) && (
					<div className="seller-orders__mobile-toolbar">
						{mobileSelectionMode ? (
							<>
								<strong role="status" aria-live="polite">
									已選 {selectedOrderNos.size} 筆
								</strong>
								<div className="seller-orders__mobile-toolbar-actions">
									<button
										type="button"
										disabled={selectableOrderNos.size === 0}
										onClick={toggleAll}
									>
										{allSelected ? "取消全選" : "全選"}
									</button>
									<button type="button" onClick={exitMobileSelectionMode}>
										取消
									</button>
								</div>
							</>
						) : (
							<>
								<span>可批次處理 {selectableOrderNos.size} 筆</span>
								<button
									type="button"
									onClick={() => setMobileSelectionMode(true)}
								>
									選取訂單
								</button>
							</>
						)}
					</div>
				)}

				{loading && scopedOrders.length === 0 ? (
					<div className="seller-orders__mobile-loading" role="status">
						<TruckSpinner size={24} label="載入訂單中" />
						<span>載入訂單中…</span>
					</div>
				) : visible.length === 0 ? (
					<div className="seller-orders__mobile-empty">
						<SellerEmptyState variant="orders" searchQuery={search} seamless />
					</div>
				) : (
					<ul className="seller-orders__mobile-list">
						{visible.map((order) => (
							<li key={order.orderNo}>
								<MobileOrderCard
									order={order}
									expanded={expandedMobileOrderNo === order.orderNo}
									onToggleExpanded={() =>
										setExpandedMobileOrderNo((current) =>
											current === order.orderNo ? null : order.orderNo,
										)
									}
									selectionMode={mobileSelectionMode}
									selectable={selectableOrderNos.has(order.orderNo)}
									selected={selectedOrderNos.has(order.orderNo)}
									onToggleSelect={() => toggleOne(order.orderNo)}
									onOpenDetail={() => openDetail(order.orderNo)}
									afterSale={afterSaleLookup.get(order.orderNo)}
									anomaly={anomalyLookup.get(order.orderNo)}
									shipmentOperation={reservationOperationByOrderNo.get(
										order.orderNo,
									)}
									onChanged={handleRefresh}
									onAction={(kind, target) =>
										setBulkTarget({ kind, orders: [target] })
									}
								/>
							</li>
						))}
					</ul>
				)}

				{hasMore && (
					<div className="seller-orders__more seller-orders__mobile-more">
						<Button
							variant="bordered"
							color="default"
							size="sm"
							onClick={() => void loadMore()}
						>
							載入更多
						</Button>
					</div>
				)}

				{mobileSelectionMode && (
					<BulkActionBar
						className="seller-orders__bulk--mobile seller-orders__mobile-bulk"
						selectedCount={selectedOrderNos.size}
						selectionByKind={selectionByKind}
						onClear={exitMobileSelectionMode}
						onPick={(kind, orders) => setBulkTarget({ kind, orders })}
					/>
				)}
			</section>
		</>
	);
}

/* ===== Bulk action bar + dialog ====================================
 * Seller batch shipping flow — sellers select multiple 待出貨 rows/cards then
 * fire the same action across all of them (預約出貨 / 呼叫黑貓 / 取號 /
 * 出貨確認 / 下載 PDF). Each kind has its own minimal form so the seller
 * only confirms what's strictly required for that step (e.g. 黑貓
 * 出貨日, 上門取件 Y/N). Execution is sequential to keep API pressure
 * low and let the seller see per-order results.
 * ==================================================================== */

interface BulkActionBarProps {
	className?: string;
	selectedCount: number;
	selectionByKind: Record<BulkKind, Order[]>;
	onClear: () => void;
	onPick: (kind: BulkKind, orders: Order[]) => void;
}

function BulkActionBar({
	className,
	selectedCount,
	selectionByKind,
	onClear,
	onPick,
}: BulkActionBarProps) {
	const entries = (
		Object.entries(selectionByKind) as Array<[BulkKind, Order[]]>
	).filter(([, list]) => list.length > 0);

	return (
		<section
			className={["seller-orders__bulk", className].filter(Boolean).join(" ")}
			aria-label="批量操作"
		>
			<div className="seller-orders__bulk-info">
				<strong>已選 {selectedCount} 筆</strong>
				<button
					type="button"
					className="seller-orders__bulk-clear"
					onClick={onClear}
				>
					取消選取
				</button>
			</div>
			<nav className="seller-orders__bulk-actions" aria-label="批量動作">
				{entries.length === 0 ? (
					<span className="seller-orders__bulk-empty">
						目前選取的訂單沒有可批次處理的動作
					</span>
				) : (
					entries.map(([kind, list]) => (
						<button
							key={kind}
							type="button"
							className={`seller-orders__bulk-btn seller-orders__bulk-btn--${ACTION_TONE[kind]}`}
							onClick={() => onPick(kind, list)}
						>
							{ACTION_LABEL[kind]} ({list.length})
						</button>
					))
				)}
			</nav>
		</section>
	);
}

interface BulkResult {
	orderNo: string;
	ok: boolean;
	message?: string;
}

interface ShipmentAnomalyDialogRow {
	orderNo: string;
	reason: string;
	severityLabel?: string | null;
}

function extractProviderMessage(payload: unknown): string | null {
	if (!payload || typeof payload !== "object") {
		return null;
	}
	const record = payload as Record<string, unknown>;
	const direct = record.providerMessage ?? record.provider_message;
	if (typeof direct === "string" && direct.trim()) {
		return direct.trim();
	}
	return extractProviderMessage(record.data);
}

function payuniProviderErrorMessage(error: unknown): string | null {
	if (!(error instanceof ApiClientError)) {
		return null;
	}
	const providerMessage = extractProviderMessage(error.payload);
	return providerMessage ? `PayUni：${providerMessage}` : null;
}

interface BulkShipmentDialogProps {
	kind: BulkKind;
	orders: Order[];
	anomalyLookup: Map<string, ShipmentAnomalyItem>;
	onClose: () => void;
	onComplete: (succeeded: number) => void;
}

function BulkShipmentDialog({
	kind,
	orders,
	anomalyLookup,
	onClose,
	onComplete,
}: BulkShipmentDialogProps) {
	const [days, setDays] = React.useState<BusinessDay[] | null>(null);
	const [dayError, setDayError] = React.useState<string | null>(null);
	const [selectedDay, setSelectedDay] = React.useState(0);
	const [isContact, setIsContact] = React.useState<"Y" | "N">("N");
	const [isTrolley, setIsTrolley] = React.useState<"Y" | "N">("N");
	const [memo, setMemo] = React.useState("");
	const [running, setRunning] = React.useState(false);
	const [runMode, setRunMode] = React.useState<
		"with-photo" | "without-photo" | "default" | null
	>(null);
	const [done, setDone] = React.useState(0);
	const [results, setResults] = React.useState<BulkResult[]>([]);
	const [photoPreview, setPhotoPreview] = React.useState<string | null>(null);
	const [stagedToken, setStagedToken] = React.useState<string | undefined>();
	const [uploadingPhoto, setUploadingPhoto] = React.useState(false);
	const [uploadProgress, setUploadProgress] = React.useState<
		number | undefined
	>();
	const [uploadError, setUploadError] = React.useState<string | null>(null);

	const total = orders.length;
	const isCvsConfirm = kind === "cvs-confirm";
	const succeeded = results.filter((r) => r.ok).length;
	const failed = results.filter((r) => !r.ok).length;
	const finished = !running && results.length === total && total > 0;
	const singleCvsConfirmCode = React.useMemo(() => {
		if (!isCvsConfirm || total !== 1) return "";
		const order = orders[0];
		if (!order) return "";
		const action = nextShipmentAction(
			order.orderNo,
			order.shipping,
			order.status,
		);
		if (action.kind !== "cvs-confirm") return "";
		return `${action.odno}${action.validationNo}`;
	}, [isCvsConfirm, orders, total]);
	const anomalyRows = React.useMemo<ShipmentAnomalyDialogRow[]>(
		() =>
			orders.flatMap((order) => {
				const anomaly = anomalyLookup.get(order.orderNo);
				if (!anomaly) {
					return [];
				}
				return [
					{
						orderNo: order.orderNo,
						reason: formatShipmentAnomalyReason(anomaly),
						severityLabel: anomaly.severityLabel,
					},
				];
			}),
		[anomalyLookup, orders],
	);

	// 黑貓預約出貨：與 APP 一致，只能選後端回傳的工作天（排週日 + 國定假日）
	React.useEffect(() => {
		if (kind !== "tcat-reserve") return;
		setDayError(null);
		fetchShippingBusinessDays()
			.then((d) => {
				setDays(d);
				if (d.length === 0) setDayError("無可選出貨日期");
			})
			.catch(() => setDayError("載入日期失敗"));
	}, [kind]);

	const orderNosKey = React.useMemo(
		() => orders.map((order) => order.orderNo).join("|"),
		[orders],
	);

	// biome-ignore lint/correctness/useExhaustiveDependencies: reset the upload state whenever the target action/order set changes.
	React.useEffect(() => {
		setPhotoPreview(null);
		setStagedToken(undefined);
		setUploadingPhoto(false);
		setUploadProgress(undefined);
		setUploadError(null);
	}, [kind, orderNosKey]);

	const handleCvsConfirmPhoto = async (file: File) => {
		setUploadingPhoto(true);
		setUploadProgress(undefined);
		setUploadError(null);
		setStagedToken(undefined);
		const localPreview = URL.createObjectURL(file);
		setPhotoPreview(localPreview);
		try {
			const asset = await uploadImageFile(file, setUploadProgress);
			setPhotoPreview(asset.previewUrl || localPreview);
			setStagedToken(asset.stagedToken);
		} catch (e) {
			setUploadError(e instanceof Error ? e.message : "上傳失敗");
			setPhotoPreview(null);
			setStagedToken(undefined);
		} finally {
			setUploadingPhoto(false);
		}
	};

	const clearCvsConfirmPhoto = () => {
		setPhotoPreview(null);
		setStagedToken(undefined);
		setUploadProgress(undefined);
		setUploadError(null);
	};

	const execOne = async (
		order: Order,
		confirmPhotoToken?: string,
	): Promise<BulkResult> => {
		let startedTcatReservation = false;
		try {
			if (kind === "tcat-reserve") {
				const operationState = useSellerShipmentOperationStore.getState();
				const existing = operationState.operations[order.orderNo];
				if (existing && !isShipmentOperationExpired(existing)) {
					return {
						orderNo: order.orderNo,
						ok: true,
						message:
							existing.phase === "submitting"
								? "預約正在送出，請勿重複操作"
								: "預約處理中，完成後訂單會自動更新",
					};
				}
				const shipDate = days?.[selectedDay]?.date.replaceAll("-", "") ?? "";
				if (!shipDate) throw new Error("尚未選擇出貨日");
				operationState.startTcatReservation(order.orderNo);
				startedTcatReservation = true;
				await generateHomeLabel(order.orderNo, { shipDate });
				markShipmentCreated(order.orderNo);
				resetShipmentRetryCount(order.orderNo);
				useSellerShipmentOperationStore
					.getState()
					.syncTcatReservation(order.orderNo);
				return {
					orderNo: order.orderNo,
					ok: true,
					message: "預約處理中，完成後訂單會自動更新",
				};
			} else if (kind === "tcat-call") {
				await callTcat(order.orderNo, {
					isContact,
					isTrolley,
					memo: memo.trim() || undefined,
				});
			} else if (kind === "cvs-print") {
				const cooldownMsg = guardShipmentCooldown(order.orderNo);
				if (cooldownMsg) {
					return { orderNo: order.orderNo, ok: false, message: cooldownMsg };
				}
				// 對齊單筆詳情頁（OrderDetailClient）：取號成功後 webhook 可能尚未
				// 回傳交貨便代碼（55 秒內未收到 → 後台自動補拉），此時不算失敗，
				// 提示稍後刷新即可；拿到 odno 才寫入本地 override（B-147 清單延遲）。
				const label = await printCvsLabel(order.orderNo);
				markShipmentCreated(order.orderNo);
				resetShipmentRetryCount(order.orderNo);
				if (!label.webhookReceived) {
					return {
						orderNo: order.orderNo,
						ok: true,
						message: "已取號，交貨便代碼即將生成，請稍後刷新",
					};
				}
				if (!label.odno) {
					return {
						orderNo: order.orderNo,
						ok: false,
						message: "無法取得交貨便代碼",
					};
				}
				setLocalOdnoOverride(order.orderNo, label.odno, label.validationNo);
			} else if (kind === "cvs-confirm") {
				await confirmShipment(order.orderNo, {
					stagedTokens: confirmPhotoToken ? [confirmPhotoToken] : undefined,
				});
			} else if (kind === "tcat-pdf") {
				const action = nextShipmentAction(
					order.orderNo,
					order.shipping,
					order.status,
				);
				const labelUrl = action.kind === "tcat-pdf" ? action.labelUrl : "";
				if (!labelUrl) throw new Error("沒有可下載的託運單");
				window.open(labelUrl, "_blank", "noopener,noreferrer");
			}
			return { orderNo: order.orderNo, ok: true };
		} catch (e) {
			// 與單筆詳情頁一致：用 classifyShipmentError 轉成友善訊息，
			// 而非直接吐後端原始字串（如「[5002] 物流單尚未取得序號」）。
			const providerMessage = kind.startsWith("tcat")
				? null
				: payuniProviderErrorMessage(e);
			const outcome = classifyShipmentError(e, order.orderNo);
			if (kind === "tcat-reserve" && startedTcatReservation) {
				if (
					outcome.shouldReload ||
					outcome.bizCode === 5046 ||
					outcome.bizCode === 5047
				) {
					useSellerShipmentOperationStore
						.getState()
						.syncTcatReservation(order.orderNo);
				} else {
					useSellerShipmentOperationStore
						.getState()
						.clearOperation(order.orderNo);
				}
			}
			const message = providerMessage ?? outcome.message;
			return { orderNo: order.orderNo, ok: false, message };
		}
	};

	const runBatch = async (options?: RunBatchOptions) => {
		if (running) return;
		setRunning(true);
		setRunMode(
			isCvsConfirm
				? options?.skipPhoto
					? "without-photo"
					: "with-photo"
				: "default",
		);
		setResults([]);
		setDone(0);
		const confirmPhotoToken =
			isCvsConfirm && !options?.skipPhoto ? stagedToken : undefined;
		const accum: BulkResult[] = [];
		for (const order of orders) {
			const r = await execOne(order, confirmPhotoToken);
			accum.push(r);
			setResults([...accum]);
			setDone(accum.length);
		}
		setRunning(false);
		setRunMode(null);
	};

	const handlePrimary = () => {
		if (finished) {
			onComplete(succeeded);
			return;
		}
		void runBatch();
	};

	return (
		<div className="seller-orders__dialog-backdrop">
			<button
				type="button"
				className="seller-orders__dialog-backdrop-btn"
				onClick={onClose}
				aria-label="關閉對話框"
				disabled={running}
			/>
			<div
				className="seller-orders__dialog"
				role="dialog"
				aria-modal="true"
				aria-labelledby="bulk-dialog-title"
			>
				<header className="seller-orders__dialog-head">
					<div className="seller-orders__dialog-head-row">
						{kind.startsWith("tcat") && <ShippingBrandLogo brand="tcat" />}
						{kind.startsWith("cvs") && <ShippingBrandLogo brand="711" />}
						<div className="seller-orders__dialog-head-text">
							<h2 id="bulk-dialog-title">
								{isCvsConfirm && total === 1
									? "7-11 出貨確認"
									: ACTION_LABEL[kind]}
							</h2>
							<p>
								{isCvsConfirm && total === 1 && singleCvsConfirmCode ? (
									<>交貨便代碼：{singleCvsConfirmCode}</>
								) : (
									<>
										對 <strong>{total}</strong> 筆訂單執行批次操作
									</>
								)}
							</p>
						</div>
					</div>
				</header>

				<div className="seller-orders__dialog-body">
					{isCvsConfirm && !finished && (
						<section className="shipment-dialog__field">
							<p className="shipment-dialog__field-title">
								上傳產品的照片並出貨
							</p>
							<ImageUploader
								variant="card"
								size="md"
								accept="image/*"
								crop="photo"
								tileStyle={SHIPMENT_PHOTO_TILE_STYLE}
								previewFit="contain"
								actionsVariant="button"
								fileHint="JPG / PNG / WebP · 10 MB 以下"
								value={photoPreview ?? ""}
								uploading={uploadingPhoto}
								progress={uploadProgress}
								disabled={running}
								onPick={handleCvsConfirmPhoto}
								onClear={clearCvsConfirmPhoto}
								hint="上傳產品或包裝照片"
								error={uploadError}
								fullWidth
							/>
						</section>
					)}

					{kind === "tcat-reserve" && (
						<section className="shipment-dialog__field">
							<p className="shipment-dialog__field-title">出貨日</p>
							{days === null ? (
								<p className="shipment-dialog__placeholder">載入中…</p>
							) : dayError ? (
								<p className="shipment-dialog__placeholder">{dayError}</p>
							) : (
								<div className="shipment-dialog__chip-row">
									{days.map((d, i) => {
										const dateLabel = formatTaipeiShortDate(d.date);
										const active = selectedDay === i;
										return (
											<button
												key={d.date}
												type="button"
												aria-pressed={active}
												className={`shipment-dialog__chip ${active ? "is-active" : ""}`}
												disabled={running || finished}
												onClick={() => setSelectedDay(i)}
											>
												<small>週{d.dayOfWeek}</small>
												<strong>{dateLabel}</strong>
											</button>
										);
									})}
								</div>
							)}
						</section>
					)}

					{kind === "tcat-call" && (
						<>
							<div className="shipment-dialog__toggle-group">
								<label className="shipment-dialog__toggle">
									<PhoneIcon className="shipment-dialog__toggle-icon" />
									<span className="shipment-dialog__toggle-label">
										電話聯絡
									</span>
									<input
										type="checkbox"
										checked={isContact === "Y"}
										disabled={running || finished}
										onChange={(e) => setIsContact(e.target.checked ? "Y" : "N")}
									/>
									<span
										className="shipment-dialog__switch"
										aria-hidden="true"
									/>
								</label>
								<label className="shipment-dialog__toggle">
									<CartIcon className="shipment-dialog__toggle-icon" />
									<span className="shipment-dialog__toggle-label">
										需要推車
									</span>
									<input
										type="checkbox"
										checked={isTrolley === "Y"}
										disabled={running || finished}
										onChange={(e) => setIsTrolley(e.target.checked ? "Y" : "N")}
									/>
									<span
										className="shipment-dialog__switch"
										aria-hidden="true"
									/>
								</label>
							</div>
							<section className="shipment-dialog__field">
								<label htmlFor="bulk-tcat-memo">
									<NoteIcon className="shipment-dialog__label-icon" />
									給宅配人員的備註（選填）
								</label>
								<textarea
									id="bulk-tcat-memo"
									value={memo}
									maxLength={200}
									rows={3}
									disabled={running || finished}
									onChange={(e) => setMemo(e.target.value)}
									placeholder="例如：請小心輕放"
								/>
							</section>
						</>
					)}

					{(kind === "cvs-print" || kind === "cvs-confirm") && (
						<p className="seller-orders__dialog-hint">
							{kind === "cvs-print"
								? "將為每張訂單向 7-11 取號，取號後請至 ibon 機列印寄件單。"
								: "確認後訂單會進入「配送中」狀態，請確保已交付商品。"}
						</p>
					)}

					{kind === "tcat-pdf" && (
						<p className="seller-orders__dialog-hint">
							將為每張訂單開啟託運單 PDF — 請允許瀏覽器彈出新分頁。
						</p>
					)}

					{anomalyRows.length > 0 ? (
						<section
							className="seller-orders__dialog-anomalies"
							aria-label="配送異常原因"
						>
							<div className="seller-orders__dialog-anomalies-head">
								<strong>配送異常原因</strong>
								<span>{anomalyRows.length} 筆</span>
							</div>
							<ul className="seller-orders__dialog-anomalies-list">
								{anomalyRows.map((row) => (
									<li
										className="seller-orders__dialog-anomaly"
										key={row.orderNo}
									>
										<code>{row.orderNo}</code>
										<span title={row.reason}>
											{row.severityLabel ? `${row.severityLabel}｜` : ""}
											{row.reason}
										</span>
									</li>
								))}
							</ul>
						</section>
					) : null}

					<details className="seller-orders__dialog-list">
						<summary>
							<span className="seller-orders__dialog-list-heading">
								<span>訂單列表</span>
								<span className="seller-orders__dialog-list-count">
									{total} 筆
								</span>
							</span>
							{results.length > 0 ? (
								<span className="seller-orders__dialog-list-progress">
									進度 {done}/{total}
								</span>
							) : null}
						</summary>
						<ul className="seller-orders__dialog-list-items">
							{orders.map((o, index) => {
								const r = results.find((x) => x.orderNo === o.orderNo);
								const statusTone: "pending" | "idle" | "ok" | "fail" =
									r == null
										? running
											? "pending"
											: "idle"
										: r.ok
											? "ok"
											: "fail";
								const statusMessage =
									r == null
										? running
											? "排隊中"
											: "等待執行"
										: r.message || (r.ok ? "成功" : "失敗");

								return (
									<li
										className={`seller-orders__dialog-order seller-orders__dialog-order--${statusTone}`}
										key={o.orderNo}
									>
										<span
											className="seller-orders__dialog-order-index"
											aria-hidden="true"
										>
											{index + 1}
										</span>
										<span className="seller-orders__dialog-order-main">
											<span className="seller-orders__dialog-order-label">
												訂單編號
											</span>
											<code>{o.orderNo}</code>
										</span>
										<span
											className={`seller-orders__dialog-status seller-orders__dialog-status--${statusTone}`}
											title={statusMessage}
										>
											{statusTone === "pending" ? (
												<TruckSpinner
													className="seller-orders__dialog-status-truck"
													label="排隊中"
													size={16}
												/>
											) : statusTone === "fail" ? null : (
												<span
													className="seller-orders__dialog-status-dot"
													aria-hidden="true"
												/>
											)}
											<span>{statusMessage}</span>
										</span>
									</li>
								);
							})}
						</ul>
					</details>

					{running && (
						<TruckOverlay
							fullscreen={false}
							message={`處理中… (${done}/${total})`}
						/>
					)}

					{finished && (
						<p className="seller-orders__dialog-summary">
							已完成 {succeeded} 筆，
							<strong>{failed > 0 ? `${failed} 筆失敗` : "全部成功"}</strong>
						</p>
					)}
				</div>

				<footer
					className={[
						"seller-orders__dialog-foot",
						isCvsConfirm && !finished
							? "seller-orders__dialog-foot--split"
							: "",
					]
						.filter(Boolean)
						.join(" ")}
				>
					<Button
						variant="bordered"
						color="default"
						onClick={onClose}
						disabled={running}
					>
						{finished ? "關閉" : "取消"}
					</Button>
					{isCvsConfirm && !finished ? (
						<>
							<Button
								variant="bordered"
								color="default"
								onClick={() => void runBatch()}
								disabled={!stagedToken || uploadingPhoto || running}
								isLoading={running && runMode === "with-photo"}
								loadingIndicator={<UploadLoadingIndicator size={18} />}
								loadingLabel="送出照片中…"
							>
								送出照片
							</Button>
							<Button
								onClick={() => void runBatch({ skipPhoto: true })}
								disabled={uploadingPhoto || running}
								isLoading={running && runMode === "without-photo"}
								loadingIndicator={
									<TruckSpinner size={18} label="確認批次出貨中" />
								}
								loadingLabel="確認出貨中…"
							>
								出貨且略過
							</Button>
						</>
					) : (
						<Button
							onClick={handlePrimary}
							disabled={
								running ||
								(kind === "tcat-reserve" &&
									!finished &&
									(days == null || days.length === 0))
							}
							isLoading={running && runMode === "default"}
							loadingIndicator={
								<TruckSpinner size={18} label={`${ACTION_LABEL[kind]}中`} />
							}
							loadingLabel={`${ACTION_LABEL[kind]}中…`}
						>
							{finished ? "完成" : `開始執行 (${total})`}
						</Button>
					)}
				</footer>
			</div>
		</div>
	);
}

function formatShipmentAnomalyReason(anomaly: ShipmentAnomalyItem): string {
	const providerMessage = anomaly.providerMessage?.trim();
	if (providerMessage) {
		return `PayUni：${providerMessage}`;
	}

	const anomalyTypeLabel = anomaly.anomalyTypeLabel?.trim();
	if (anomalyTypeLabel) {
		return anomalyTypeLabel;
	}

	return "物流商未提供明確原因";
}

function formatShipmentAnomalyStatusReason(
	anomaly: ShipmentAnomalyItem | undefined,
): { label: string; title: string } | null {
	if (!anomaly) {
		return null;
	}

	const fullReason = formatShipmentAnomalyReason(anomaly);
	const simpleReason = fullReason
		.replace(/^PayUni：/, "")
		.replace(/\s+/g, " ")
		.trim();

	if (!simpleReason) {
		return null;
	}

	return {
		label:
			simpleReason.length > 30
				? `${simpleReason.slice(0, 30)}...`
				: simpleReason,
		title: fullReason,
	};
}

interface QuickFilterChipProps {
	label: string;
	count: number;
	selected: boolean;
	tone?: "default" | "warning" | "danger";
	onSelect: () => void;
}

/**
 * Pill toggle inside the quick-filter row. Mirrors the APP's
 * `_OrderFilterChip` — small rounded pill with label + count badge,
 * tone-coloured ring when selected so the chip reads as the "active
 * lens" over the current tab.
 */
function QuickFilterChip({
	label,
	count,
	selected,
	tone = "default",
	onSelect,
}: QuickFilterChipProps) {
	const hasCount = count > 0;
	return (
		<button
			type="button"
			className={[
				"seller-orders__chip",
				selected ? "is-selected" : "",
				!hasCount ? "is-empty" : "",
				tone !== "default" ? `seller-orders__chip--${tone}` : "",
				hasCount && !selected && tone !== "default" ? "has-attention" : "",
			]
				.filter(Boolean)
				.join(" ")}
			aria-pressed={selected}
			onClick={onSelect}
		>
			<span>{label}</span>
			{hasCount ? (
				<span className="seller-orders__chip-count">{count}</span>
			) : (
				<span className="seller-orders__chip-count seller-orders__chip-count--zero">
					0
				</span>
			)}
		</button>
	);
}

interface OrderRowProps {
	order: Order;
	selectable: boolean;
	selected: boolean;
	shipmentOperation?: SellerShipmentOperation;
	onToggleSelect: () => void;
	/** Open the full order detail in the right-side drawer. */
	onOpenDetail: () => void;
	afterSale: AfterSaleCase | undefined;
	anomaly: ShipmentAnomalyItem | undefined;
	onChanged: () => void | Promise<void>;
	/** Open the shipment dialog inline for this single order (same dialog the
	 * bulk-action bar uses), instead of navigating to the detail page. */
	onAction: (kind: BulkKind, order: Order) => void;
}

interface MobileOrderCardProps extends OrderRowProps {
	expanded: boolean;
	onToggleExpanded: () => void;
	selectionMode: boolean;
}

/**
 * Phone-only order work card. The desktop row and this card receive the same
 * authoritative Order plus the same nextShipmentAction result; this component
 * only changes information density and touch interaction, never order truth.
 */
function MobileOrderCard({
	order,
	expanded,
	onToggleExpanded,
	selectionMode,
	selectable,
	selected,
	shipmentOperation,
	onToggleSelect,
	onOpenDetail,
	afterSale,
	anomaly,
	onChanged,
	onAction,
}: MobileOrderCardProps) {
	const headingId = React.useId();
	const detailsId = React.useId();
	const firstItem = order.items?.[0];
	const itemCount = (order.items ?? []).reduce(
		(sum, item) => sum + Number(item.quantity || 0),
		0,
	);
	const createdLabel = formatTaipeiDateTime(order.createdAt);
	const isShipDeadline =
		order.status === "paid" || order.status === "preparing";
	const deadlineLabel = formatDeadline(order.autoCancelAt);
	const tempLabel = deliveryTempLabel(order.shipping?.deliveryTemp);
	const anomalyStatusReason = formatShipmentAnomalyStatusReason(anomaly);
	const shipmentProcessing = Boolean(shipmentOperation);
	const shipmentProcessingLabel = shipmentOperation
		? shipmentOperationLabel(shipmentOperation)
		: "";
	const hasMobileFacts = Boolean(
		shipmentProcessingLabel || afterSale || anomalyStatusReason,
	);
	const action = nextShipmentAction(
		order.orderNo,
		order.shipping,
		order.status,
	);
	const isReservationProcessing =
		shipmentProcessing && action.kind === "tcat-reserve";

	const handleToggle = () => {
		if (selectionMode && selectable) {
			onToggleSelect();
			return;
		}
		if (!selectionMode) onToggleExpanded();
	};

	return (
		<article
			className={[
				"seller-orders__mobile-card",
				expanded && !selectionMode
					? "seller-orders__mobile-card--expanded"
					: "",
				selectionMode ? "seller-orders__mobile-card--selecting" : "",
				selected ? "seller-orders__mobile-card--selected" : "",
				afterSale ? "seller-orders__mobile-card--has-case" : "",
			]
				.filter(Boolean)
				.join(" ")}
			aria-labelledby={headingId}
		>
			{selectionMode && (
				<label className="seller-orders__mobile-select">
					<input
						type="checkbox"
						aria-label={`選取訂單 ${order.orderNo}`}
						checked={selected}
						disabled={!selectable}
						onChange={onToggleSelect}
					/>
				</label>
			)}

			<button
				type="button"
				className="seller-orders__mobile-card-toggle"
				onClick={handleToggle}
				aria-expanded={selectionMode ? undefined : expanded}
				aria-controls={selectionMode ? undefined : detailsId}
				aria-pressed={selectionMode && selectable ? selected : undefined}
			>
				<MobileShippingMethodMark method={order.shipping?.method} />
				<strong id={headingId} className="seller-orders__mobile-card-order">
					{order.orderNo}
				</strong>
				<span className="seller-orders__mobile-card-temperature">
					{tempLabel && (
						<TemperatureTag value={order.shipping?.deliveryTemp ?? ""} />
					)}
				</span>
				<OrderStatusSummary
					order={order}
					anomaly={anomaly}
					shipmentOperation={shipmentOperation}
					compact
					className="seller-orders__mobile-card-status seller-orders__mobile-card-status--compact"
				/>
				<span className="seller-orders__mobile-card-chevron" aria-hidden="true">
					<AppIcon name="chevron-down" size={18} />
				</span>
				<span className="visually-hidden">
					{selectionMode && selectable
						? selected
							? "取消選取訂單"
							: "選取訂單"
						: expanded
							? "收合訂單摘要"
							: "展開訂單摘要"}
				</span>
			</button>

			<div
				id={detailsId}
				className="seller-orders__mobile-card-details"
				hidden={!expanded || selectionMode}
			>
				<button
					type="button"
					className="seller-orders__mobile-card-main"
					onClick={onOpenDetail}
				>
					<span className="visually-hidden">查看訂單詳情</span>
					<span className="seller-orders__mobile-card-meta">
						<small className="seller-orders__mobile-card-time">
							{createdLabel}
						</small>
						{deadlineLabel && (
							<span
								className={[
									"seller-orders__mobile-card-deadline",
									isShipDeadline
										? "seller-orders__mobile-card-deadline--ship"
										: "seller-orders__mobile-card-deadline--cancel",
								].join(" ")}
							>
								{isShipDeadline ? "出貨期限" : "取消期限"} {deadlineLabel}
							</span>
						)}
					</span>

					<span className="seller-orders__mobile-card-summary">
						<MobileBuyerName
							buyerName={order.buyerName}
							buyerId={order.buyerId}
						/>
						<strong className="seller-orders__mobile-card-amount">
							NT$ {Number(order.totalActualAmount || 0).toLocaleString()}
						</strong>
					</span>

					<span className="seller-orders__mobile-card-product">
						<span
							className="seller-orders__mobile-card-thumb"
							aria-hidden="true"
						>
							{firstItem?.image ? (
								<AppImage
									src={firstItem.image}
									alt=""
									width={44}
									height={44}
									sizes="44px"
									preset="thumb"
								/>
							) : (
								<AppIcon name="image" size={24} />
							)}
						</span>
						<span className="seller-orders__mobile-card-product-text">
							<strong>{firstItem?.name?.trim() || "—"}</strong>
							<small>共 {itemCount} 件</small>
						</span>
					</span>

					{hasMobileFacts && (
						<span className="seller-orders__mobile-card-facts">
							{shipmentProcessingLabel && (
								<span>{shipmentProcessingLabel}</span>
							)}
							{afterSale && <span>售後待處理</span>}
							{anomalyStatusReason && (
								<span title={anomalyStatusReason.title}>
									{anomalyStatusReason.label}
								</span>
							)}
						</span>
					)}
				</button>

				<div className="seller-orders__mobile-card-actions">
					<button
						type="button"
						className="seller-orders__mobile-card-detail"
						onClick={onOpenDetail}
					>
						查看詳情
					</button>
					{isRunnableShipmentAction(action) && (
						<button
							type="button"
							className={[
								"seller-orders__action",
								"seller-orders__mobile-card-primary",
								isReservationProcessing
									? "seller-orders__action--processing"
									: "seller-orders__action--primary",
							].join(" ")}
							disabled={isReservationProcessing}
							aria-busy={isReservationProcessing || undefined}
							onClick={() => {
								if (!isReservationProcessing) onAction(action.kind, order);
							}}
						>
							{isReservationProcessing ? (
								<>
									<TruckSpinner
										className="seller-orders__action-spinner"
										label={shipmentProcessingLabel}
										size={15}
									/>
									<span>{shipmentProcessingLabel}</span>
								</>
							) : (
								ACTION_LABEL[action.kind]
							)}
						</button>
					)}
					<OrderActionsMenu
						order={order}
						onOpenDetail={onOpenDetail}
						onChanged={onChanged}
						shipmentProcessing={shipmentProcessing}
					/>
				</div>
			</div>
		</article>
	);
}

function MobileShippingMethodMark({ method }: { method?: string }) {
	const brand =
		method === ShippingMethod.TAQBIN
			? "tcat"
			: method === ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP
				? "711"
				: null;
	if (brand) return <ShippingBrandLogo brand={brand} compact />;

	const label = shippingMethodLabel(method);
	return (
		<span className="seller-orders__mobile-card-carrier-fallback" title={label}>
			<AppIcon name="truck" size={18} aria-hidden="true" />
			<span className="visually-hidden">{label}</span>
		</span>
	);
}

function MobileBuyerName({
	buyerName,
	buyerId,
}: {
	buyerName?: string;
	buyerId?: string;
}) {
	const snapshotName = buyerName?.trim() || "";
	const currentProfile = useSellerCustomerLookup(buyerId, {
		enabled: !snapshotName,
	});
	return (
		<strong className="seller-orders__mobile-card-buyer">
			{snapshotName || currentProfile?.name.trim() || "會員"}
		</strong>
	);
}

function OrderStatusSummary({
	order,
	anomaly,
	shipmentOperation,
	compact = false,
	className,
}: {
	order: Order;
	anomaly: ShipmentAnomalyItem | undefined;
	shipmentOperation?: SellerShipmentOperation;
	compact?: boolean;
	className?: string;
}) {
	const hasAnomaly = Boolean(anomaly) || order.status === "shipping_abnormal";
	const anomalyStatusReason = formatShipmentAnomalyStatusReason(anomaly);
	const shipmentProcessingLabel = shipmentOperation
		? shipmentOperationLabel(shipmentOperation)
		: "";

	return (
		<span
			className={["seller-orders__status-stack", className]
				.filter(Boolean)
				.join(" ")}
		>
			{shipmentOperation ? (
				<>
					<span
						className={`seller-orders__status ${statusClass(order.status)}`}
					>
						{STATUS_LABEL[order.status] || order.status}
					</span>
					{!compact && (
						<span className="seller-orders__status-note seller-orders__status-note--processing">
							<TruckSpinner
								className="seller-orders__status-spinner"
								label={shipmentProcessingLabel}
								size={14}
							/>
							<span>{shipmentProcessingLabel}</span>
						</span>
					)}
				</>
			) : hasAnomaly ? (
				<>
					<span className="seller-orders__status seller-orders__status--urgent">
						異常
					</span>
					{!compact && anomalyStatusReason && (
						<span
							className="seller-orders__status-note"
							title={anomalyStatusReason.title}
						>
							{anomalyStatusReason.label}
						</span>
					)}
				</>
			) : (
				<span className={`seller-orders__status ${statusClass(order.status)}`}>
					{STATUS_LABEL[order.status] || order.status}
				</span>
			)}
		</span>
	);
}

function OrderRow({
	order,
	selectable,
	selected,
	shipmentOperation,
	onToggleSelect,
	onOpenDetail,
	afterSale,
	anomaly,
	onChanged,
	onAction,
}: OrderRowProps) {
	const firstItem = order.items?.[0];
	const itemCount = (order.items ?? []).reduce(
		(sum, it) => sum + Number(it.quantity || 0),
		0,
	);
	const createdLabel = formatTaipeiDateTime(order.createdAt);
	// 出貨/取消期限 — 對齊 App OrderInfoColumn：待出貨(paid/preparing) 顯示「出貨
	// 期限」(amber)，其餘顯示「取消期限」(red)。
	const isShipDeadline =
		order.status === "paid" || order.status === "preparing";
	const deadlineLabel = formatDeadline(order.autoCancelAt);
	const tempLabel = deliveryTempLabel(order.shipping?.deliveryTemp);
	const shipmentProcessing = Boolean(shipmentOperation);
	const shipmentProcessingLabel = shipmentOperation
		? shipmentOperationLabel(shipmentOperation)
		: "";

	return (
		<tr
			className={[
				"seller-orders__row",
				"seller-orders__row--clickable",
				selected ? "seller-orders__row--selected" : "",
				afterSale ? "seller-orders__row--has-case" : "",
			]
				.filter(Boolean)
				.join(" ")}
			onClick={onOpenDetail}
		>
			{/* biome-ignore lint/a11y/useKeyWithClickEvents: stops the row click from opening the drawer when toggling the checkbox; the checkbox itself stays keyboard-accessible. */}
			<td
				className="seller-orders__col-select"
				onClick={(e) => e.stopPropagation()}
			>
				<label className="seller-orders__check">
					<input
						type="checkbox"
						aria-label={`選取訂單 ${order.orderNo}`}
						checked={selected}
						disabled={!selectable}
						onChange={onToggleSelect}
					/>
				</label>
			</td>
			<td>
				<button
					type="button"
					className="seller-orders__open-btn"
					onClick={(e) => {
						e.stopPropagation();
						onOpenDetail();
					}}
				>
					<span>
						<strong className="seller-orders__order-no">{order.orderNo}</strong>
						<small>{createdLabel}</small>
						{deadlineLabel && (
							<small
								className={[
									"seller-orders__deadline",
									isShipDeadline
										? "seller-orders__deadline--ship"
										: "seller-orders__deadline--cancel",
								].join(" ")}
							>
								{isShipDeadline ? "出貨期限" : "取消期限"} {deadlineLabel}
							</small>
						)}
					</span>
				</button>
			</td>
			<td>
				<BuyerCell
					avatar={order.buyerAvatar}
					buyerName={order.buyerName}
					buyerId={order.buyerId}
					hasAfterSale={Boolean(afterSale)}
				/>
			</td>
			<td className="seller-orders__product">
				<div className="seller-orders__product-cell">
					<span className="seller-orders__product-thumb" aria-hidden="true">
						{firstItem?.image ? (
							<AppImage
								src={firstItem.image}
								alt=""
								width={40}
								height={40}
								sizes="40px"
								preset="thumb"
							/>
						) : (
							<AppIcon name="image" size={24} />
						)}
					</span>
					<span className="seller-orders__product-text">
						<strong>{firstItem?.name?.trim() || "—"}</strong>
						{(order.items?.length ?? 0) > 1 && (
							<small>等 {order.items?.length} 項</small>
						)}
					</span>
				</div>
			</td>
			<td className="is-center">{itemCount}</td>
			<td className="is-right seller-orders__amount">
				NT$ {Number(order.totalActualAmount || 0).toLocaleString()}
			</td>
			<td className="is-center seller-orders__shipping">
				<span className="seller-orders__shipping-method">
					{shippingMethodLabel(order.shipping?.method)}
				</span>
				{tempLabel && (
					<TemperatureTag value={order.shipping?.deliveryTemp ?? ""} />
				)}
			</td>
			<td className="is-center">
				<OrderStatusSummary
					order={order}
					anomaly={anomaly}
					shipmentOperation={shipmentOperation}
				/>
			</td>
			<td className="is-center">
				<div className="seller-orders__actions">
					{(() => {
						const action = nextShipmentAction(
							order.orderNo,
							order.shipping,
							order.status,
						);
						const isReservationProcessing =
							shipmentProcessing && action.kind === "tcat-reserve";
						let label: string | null = null;
						let opensDetail = false;
						switch (action.kind) {
							case "tcat-reserve":
								label = "預約出貨";
								break;
							case "tcat-call":
								label = "呼叫黑貓";
								break;
							case "tcat-pdf":
								label = "下載 PDF";
								break;
							case "cvs-print":
								label = "預約取號";
								break;
							case "cvs-confirm":
								label = "出貨確認";
								break;
							default:
								label = "查看詳情";
								opensDetail = true;
						}
						return (
							<>
								{label && (
									<button
										type="button"
										className={[
											"seller-orders__action",
											opensDetail
												? "seller-orders__action--secondary"
												: isReservationProcessing
													? "seller-orders__action--processing"
													: "seller-orders__action--primary",
										].join(" ")}
										disabled={isReservationProcessing}
										aria-busy={isReservationProcessing || undefined}
										onClick={(e) => {
											e.stopPropagation();
											if (isReservationProcessing) return;
											if (opensDetail) {
												onOpenDetail();
												return;
											}
											if (isRunnableShipmentAction(action)) {
												onAction(action.kind, order);
											}
										}}
									>
										{isReservationProcessing ? (
											<>
												<TruckSpinner
													className="seller-orders__action-spinner"
													label={shipmentProcessingLabel}
													size={15}
												/>
												<span>{shipmentProcessingLabel}</span>
											</>
										) : (
											label
										)}
									</button>
								)}
								<OrderActionsMenu
									order={order}
									onOpenDetail={onOpenDetail}
									onChanged={onChanged}
									shipmentProcessing={shipmentProcessing}
								/>
							</>
						);
					})()}
				</div>
			</td>
		</tr>
	);
}

function statusClass(status: string): string {
	switch (status) {
		// 待付款 / 已付款 / 準備中 → 賣家還沒出貨，amber
		case "pending_payment":
		case "paid":
		case "preparing":
			return "seller-orders__status--pending";
		// 配送異常 → 紅色 urgent，賣家必須立刻處理
		case "shipping_abnormal":
			return "seller-orders__status--urgent";
		// 待取貨 → 7-11 已到店等買家取，amber 提示
		case "waiting_pickup":
			return "seller-orders__status--warning";
		// 一般配送中 / 已送達 → 中性 info
		case "shipping":
		case "delivered":
			return "seller-orders__status--shipping";
		// 已完成 → 品牌色
		case "completed":
			return "seller-orders__status--complete";
		// 部分退款 → warning（60 僅出現於已完成訂單的部分退款終態）
		case "partially_refunded":
			return "seller-orders__status--warning";
		// 已退款 / 退換貨中 → 紅色 alert
		case "refunded":
		case "after_sale":
			return "seller-orders__status--alert";
		// 已取消 → 灰色 muted（不是錯誤、不需要動作）
		case "cancelled":
			return "seller-orders__status--muted";
		default:
			return "seller-orders__status--pending";
	}
}

/** Map backend shipping method code ("3001" / "3002" / …) to a human label.
 *
 * 🔴 空值回 `"—"`，**不得回「宅配」**：那是憑空捏造一個物流商，而空值真正的
 * 意思是「後端沒給」。未知碼一律照顯原碼（65A §1.3 第 4 條：不猜、不隱藏）。
 * 值域與別名表都在 `lib/codemeta/enums.ts`（CodeMeta 生成檔為 SSOT）。 */
function shippingMethodLabel(raw: string | undefined): string {
	return formatShippingMethodLabel(raw);
}

/* ============================================================
   Buyer cell — avatar + name + phone + after-sale dot.
   Mirrors the seller app's order-row buyer column. Avatar
   falls back to the first character of the buyer's name on a
   brand-tinted circle if the buyer has no profile image.
   ============================================================ */

function BuyerCell({
	avatar,
	buyerName,
	buyerId,
	hasAfterSale,
}: {
	avatar?: string;
	buyerName?: string;
	buyerId?: string;
	hasAfterSale: boolean;
}) {
	const snapshotName = buyerName?.trim() || "";
	const currentProfile = useSellerCustomerLookup(buyerId, {
		enabled: !snapshotName,
	});
	const primary = snapshotName || currentProfile?.name.trim() || "會員";
	const resolvedAvatar = avatar || currentProfile?.avatar;
	const initial = primary.charAt(0);
	// Jump to the buyer's CRM record. Prefer the buyer ID → detail page;
	// otherwise fall back to the customers list pre-filtered by the buyer's
	// own site name. Receiver data belongs in the detail drawer only.
	const crmQuery = snapshotName || currentProfile?.name.trim() || "";
	const crmHref = buyerId
		? `/seller-center/customers/${encodeURIComponent(buyerId)}`
		: crmQuery
			? `/seller-center/customers?q=${encodeURIComponent(crmQuery)}`
			: null;
	// 配送異常已改在「訂單狀態」欄以「異常」徽章呈現，買家頭像的紅點只保留
	// 待處理售後提示。
	const showDot = hasAfterSale;
	const dotLabel = hasAfterSale ? "待處理售後" : "";
	return (
		<div className="seller-orders__buyer">
			<span className="seller-orders__buyer-avatar" aria-hidden="true">
				{resolvedAvatar ? (
					<AppImage
						src={resolvedAvatar}
						alt=""
						width={36}
						height={36}
						sizes="36px"
						preset="avatar"
					/>
				) : (
					<span className="seller-orders__buyer-initial">{initial}</span>
				)}
				{showDot && (
					<span
						role="img"
						aria-label={dotLabel}
						title={dotLabel}
						className="seller-orders__buyer-alert"
					/>
				)}
			</span>
			<span className="seller-orders__buyer-text">
				{crmHref ? (
					<Link
						href={crmHref}
						className="seller-orders__buyer-link"
						onClick={(event) => event.stopPropagation()}
					>
						<strong>{primary}</strong>
					</Link>
				) : (
					<strong>{primary}</strong>
				)}
			</span>
		</div>
	);
}

function IconSearch() {
	return <AppIcon name="search" />;
}
function IconDownload() {
	return <AppIcon name="download" />;
}
function IconPackage() {
	return <AppIcon name="box" />;
}
function IconTruck() {
	return <AppIcon name="truck" />;
}
function IconCheck() {
	return <AppIcon name="check" />;
}
function IconBell() {
	return <AppIcon name="notification" />;
}
function IconTune() {
	return (
		<AppIcon name="filter" className="seller-orders__quick-filters-icon" />
	);
}
function IconMore() {
	return <AppIcon name="more" />;
}

/* ===== Row overflow menu (查看詳情 / 列印出貨單 / 聯絡買家 / 取消訂單) =====
 * Mirrors the products table's kebab menu pattern. Items are gated by what
 * the order actually supports so we never render a dead action:
 *   - 列印出貨單  only when 黑貓 has already produced a label PDF
 *   - 聯絡買家    only when we know the buyer's account id (messaging route)
 *   - 取消訂單    only in the 待出貨 lane (paid/preparing) — the backend
 *                seller_cancellations endpoint accepts status 10/20 only.
 * ==================================================================== */
function OrderActionsMenu({
	order,
	onOpenDetail,
	onChanged,
	shipmentProcessing,
}: {
	order: Order;
	onOpenDetail: () => void;
	onChanged: () => void | Promise<void>;
	shipmentProcessing: boolean;
}) {
	const [open, setOpen] = React.useState(false);
	const [cancelOpen, setCancelOpen] = React.useState(false);
	const rootRef = React.useRef<HTMLDivElement>(null);
	const openChat = useChatDrawerStore((state) => state.openChat);

	React.useEffect(() => {
		if (!open) return;
		const onDown = (e: MouseEvent) => {
			if (!rootRef.current?.contains(e.target as Node)) setOpen(false);
		};
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") setOpen(false);
		};
		document.addEventListener("mousedown", onDown);
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("mousedown", onDown);
			document.removeEventListener("keydown", onKey);
		};
	}, [open]);

	const canCancel =
		!shipmentProcessing &&
		(order.status === "paid" || order.status === "preparing");
	// 託運單代碼取號後 4 天失效（對齊 App）— 失效就不再給「列印出貨單」，
	// 點了也只會拿到作廢的 PDF。
	const labelUrl = isLabelExpired(order.shipping)
		? undefined
		: order.shipping?.labelUrl;
	const buyerId = order.buyerId;

	return (
		<div className="seller-orders__menu" ref={rootRef}>
			<button
				type="button"
				className="seller-orders__action-icon"
				aria-label="更多操作"
				aria-haspopup="menu"
				aria-expanded={open}
				onClick={(e) => {
					e.stopPropagation();
					setOpen((v) => !v);
				}}
			>
				<IconMore />
			</button>
			{open && (
				<div
					className="seller-orders__menu-list"
					role="menu"
					onClick={(e) => e.stopPropagation()}
					onKeyDown={(e) => e.stopPropagation()}
				>
					<button
						type="button"
						role="menuitem"
						onClick={() => {
							setOpen(false);
							onOpenDetail();
						}}
					>
						查看詳情
					</button>
					{labelUrl && (
						<a
							href={labelUrl}
							target="_blank"
							rel="noopener noreferrer"
							role="menuitem"
							onClick={() => setOpen(false)}
						>
							列印出貨單
						</a>
					)}
					{buyerId && (
						<button
							type="button"
							role="menuitem"
							onClick={() => {
								setOpen(false);
								openChat(buyerId, {
									name: order.buyerName,
									avatar: order.buyerAvatar,
								});
							}}
						>
							聯絡買家
						</button>
					)}
					{canCancel && (
						<button
							type="button"
							role="menuitem"
							className="is-danger"
							onClick={() => {
								setOpen(false);
								setCancelOpen(true);
							}}
						>
							取消訂單
						</button>
					)}
				</div>
			)}
			{cancelOpen && (
				<CancelOrderDialog
					order={order}
					onClose={() => setCancelOpen(false)}
					onDone={async () => {
						setCancelOpen(false);
						await onChanged();
					}}
				/>
			)}
		</div>
	);
}
