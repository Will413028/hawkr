/**
 * Seller-side shipment anomaly repo.
 *
 * Backend spec: 06 §5.21（物流微服務「配送異常賣家通知」）。Endpoints under
 * `/app/shipments/anomalies` are routed via the `/app/*` authed pipeline,
 * keyed by the seller JWT.
 *
 *   GET   /app/shipments/anomalies                  list
 *   GET   /app/shipments/anomalies/stats            severity counts + total
 *   POST  /app/shipments/anomalies/{shipment_id}/{anomaly_type}/acknowledge
 *   POST  /app/shipments/anomalies/{shipment_id}/{anomaly_type}/ignore
 *
 * Both the dashboard banner (seller homepage) and the orders-list red-dot
 * indicator share the same `fetchSellerShipmentAnomalies` / `fetchStats`
 * calls — the list page just enables filters and pagination.
 */

import { authedFetch } from "../api/client";

/** Backend `ShipmentAnomalyType` (`内部技術碼`). Snake-case codes mirror
 *  the gateway response 1:1. */
export const SHIPMENT_ANOMALY_TYPE = {
	TCAT_CALL_FAILED: "TCAT_CALL_FAILED",
	CVS_LABEL_UNPRINTED: "CVS_LABEL_UNPRINTED",
	PROVIDER_STATUS_ABNORMAL: "PROVIDER_STATUS_ABNORMAL",
	PACKAGE_LOST: "PACKAGE_LOST",
	DELIVERY_OVERDUE: "DELIVERY_OVERDUE",
	PAYUNI_LGS_NO_CODE: "PAYUNI_LGS_NO_CODE",
	RETURN_STUCK: "RETURN_STUCK",
} as const;

export type ShipmentAnomalyType =
	(typeof SHIPMENT_ANOMALY_TYPE)[keyof typeof SHIPMENT_ANOMALY_TYPE];

/** Human-readable labels for each `ShipmentAnomalyType` — kept here so the
 *  UI can fall back gracefully when the backend response omits the label
 *  (older payloads). Matches `ShipmentAnomalyType.label` from CodeMeta. */
export const SHIPMENT_ANOMALY_TYPE_LABEL: Record<ShipmentAnomalyType, string> =
	{
		TCAT_CALL_FAILED: "黑貓叫件未完成",
		CVS_LABEL_UNPRINTED: "超商寄件單未列印",
		PROVIDER_STATUS_ABNORMAL: "物流商回報異常",
		PACKAGE_LOST: "包裹疑似遺失",
		DELIVERY_OVERDUE: "預計到貨日已過",
		PAYUNI_LGS_NO_CODE: "PayUni 三段碼缺失",
		RETURN_STUCK: "退貨運送卡關",
	};

/** Severity tiers — used by the dashboard's tone-coloured count tiles. */
export const SHIPMENT_ANOMALY_SEVERITY = {
	HIGH: "HIGH",
	MEDIUM: "MEDIUM",
	LOW: "LOW",
} as const;

export type ShipmentAnomalySeverity =
	(typeof SHIPMENT_ANOMALY_SEVERITY)[keyof typeof SHIPMENT_ANOMALY_SEVERITY];

export const SHIPMENT_ANOMALY_SEVERITY_LABEL: Record<
	ShipmentAnomalySeverity,
	string
> = {
	HIGH: "高",
	MEDIUM: "中",
	LOW: "低",
};

/** Ack action — written via either the `/acknowledge` or `/ignore`
 *  endpoint, returned in the list response so the UI can mark rows as
 *  「已處理」/「已忽略」. */
export type ShipmentAnomalyAckAction = "ACKNOWLEDGED" | "IGNORED";

export interface ShipmentAnomalyItem {
	shipmentId: string;
	orderNo: string;
	anomalyType: ShipmentAnomalyType;
	anomalyTypeLabel: string;
	severity: ShipmentAnomalySeverity;
	severityLabel: string;
	detectedAt: string; // ISO 8601 UTC
	/** Buyer-facing receiver name surfaced on the row. */
	buyerName?: string;
	/** Last status the backend recorded (e.g. `33`, `41`, `44`). */
	shipmentStatus?: string;
	shippingMethod?: string;
	/** Logistics provider failure reason returned by PayUni. */
	providerMessage?: string;
	/** `null` when the seller hasn't acted yet. */
	actedAt?: string;
	ackAction?: ShipmentAnomalyAckAction;
}

interface ServerAnomalyRow {
	shipment_id?: string | number;
	shipmentId?: string | number;
	order_no?: string;
	orderNo?: string;
	anomaly_type?: string;
	anomalyType?: string;
	anomaly_type_label?: string;
	anomalyTypeLabel?: string;
	severity?: string;
	severity_label?: string;
	severityLabel?: string;
	detected_at?: string;
	detectedAt?: string;
	buyer_name?: string;
	buyerName?: string;
	shipment_status?: string;
	shipmentStatus?: string;
	shipping_method?: string;
	shippingMethod?: string;
	provider_message?: string | null;
	providerMessage?: string | null;
	acted_at?: string | null;
	actedAt?: string | null;
	ack_action?: string | null;
	ackAction?: string | null;
}

function normalizeItem(raw: ServerAnomalyRow): ShipmentAnomalyItem {
	const anomalyType = String(
		raw.anomalyType ?? raw.anomaly_type ?? "",
	) as ShipmentAnomalyType;
	const severity = String(raw.severity ?? "") as ShipmentAnomalySeverity;
	return {
		shipmentId: String(raw.shipmentId ?? raw.shipment_id ?? ""),
		orderNo: String(raw.orderNo ?? raw.order_no ?? ""),
		anomalyType,
		anomalyTypeLabel: String(
			raw.anomalyTypeLabel ??
				raw.anomaly_type_label ??
				SHIPMENT_ANOMALY_TYPE_LABEL[anomalyType] ??
				anomalyType,
		),
		severity,
		severityLabel: String(
			raw.severityLabel ??
				raw.severity_label ??
				SHIPMENT_ANOMALY_SEVERITY_LABEL[severity] ??
				severity,
		),
		detectedAt: String(raw.detectedAt ?? raw.detected_at ?? ""),
		buyerName: (raw.buyerName ?? raw.buyer_name) as string | undefined,
		shipmentStatus: (raw.shipmentStatus ?? raw.shipment_status) as
			| string
			| undefined,
		shippingMethod: (raw.shippingMethod ?? raw.shipping_method) as
			| string
			| undefined,
		providerMessage: (raw.providerMessage ??
			raw.provider_message ??
			undefined) as string | undefined,
		actedAt: (raw.actedAt ?? raw.acted_at) as string | undefined,
		ackAction: (raw.ackAction ?? raw.ack_action) as
			| ShipmentAnomalyAckAction
			| undefined,
	};
}

export interface ShipmentAnomalyListResult {
	records: ShipmentAnomalyItem[];
	total: number;
	page: number;
	pageSize: number;
}

export interface FetchShipmentAnomaliesParams {
	anomalyType?: ShipmentAnomalyType;
	severity?: ShipmentAnomalySeverity;
	/** `true` includes rows where the seller has acted (already
	 *  acknowledged / ignored). Default `false` so the dashboard only
	 *  shows live work. */
	showAcknowledged?: boolean;
	page?: number;
	pageSize?: number;
}

/** GET /app/shipments/anomalies — list seller's open anomalies. Soft-fails
 *  with an empty list on 404 so the rest of the page can still render if
 *  the backend hasn't deployed §5.21 yet. */
export async function fetchSellerShipmentAnomalies(
	params: FetchShipmentAnomaliesParams = {},
): Promise<ShipmentAnomalyListResult> {
	const qs = new URLSearchParams();
	if (params.anomalyType) qs.set("anomaly_type", params.anomalyType);
	if (params.severity) qs.set("severity", params.severity);
	if (params.showAcknowledged != null)
		qs.set("show_acknowledged", params.showAcknowledged ? "true" : "false");
	if (params.page && params.page > 0) qs.set("page", String(params.page));
	if (params.pageSize && params.pageSize > 0)
		qs.set("page_size", String(params.pageSize));

	const path = qs.toString()
		? `/app/shipments/anomalies?${qs.toString()}`
		: "/app/shipments/anomalies";
	try {
		const env = await authedFetch<{
			records?: ServerAnomalyRow[];
			total?: number | string;
			page?: number | string;
			pageSize?: number | string;
			page_size?: number | string;
		}>(path);
		const list = env.data?.records ?? [];
		return {
			records: list.map(normalizeItem),
			total: Number(env.data?.total ?? list.length),
			page: Number(env.data?.page ?? params.page ?? 1),
			pageSize: Number(
				env.data?.pageSize ?? env.data?.page_size ?? params.pageSize ?? 20,
			),
		};
	} catch (e) {
		if (e instanceof Error && /\b404\b/.test(e.message)) {
			return {
				records: [],
				total: 0,
				page: params.page ?? 1,
				pageSize: params.pageSize ?? 20,
			};
		}
		throw e;
	}
}

export interface ShipmentAnomalyStats {
	total: number;
	high: number;
	medium: number;
	low: number;
}

interface ServerStats {
	total?: number | string;
	// Backend Gateway 實際回傳的 camelCase 欄位（gateway/internal/handler/shipment_anomaly.go）
	highSeverityCount?: number | string;
	mediumSeverityCount?: number | string;
	lowSeverityCount?: number | string;
	// snake_case fallback（若 gateway 未來改格式）
	high_severity_count?: number | string;
	medium_severity_count?: number | string;
	low_severity_count?: number | string;
	// 簡短 alias / 早期版本 fallback
	high?: number | string;
	medium?: number | string;
	low?: number | string;
	by_severity?: Partial<Record<string, number | string>>;
	bySeverity?: Partial<Record<string, number | string>>;
}

/** GET /app/shipments/anomalies/stats — total + per-severity counts. */
export async function fetchSellerShipmentAnomalyStats(): Promise<ShipmentAnomalyStats> {
	try {
		const env = await authedFetch<ServerStats>(
			"/app/shipments/anomalies/stats",
		);
		const data = env.data ?? {};
		const bucket = data.bySeverity ?? data.by_severity ?? {};
		const num = (v: unknown) => {
			const n = Number(v ?? 0);
			return Number.isFinite(n) ? n : 0;
		};
		return {
			total: num(data.total),
			high: num(
				data.highSeverityCount ??
					data.high_severity_count ??
					data.high ??
					bucket.HIGH ??
					bucket.high,
			),
			medium: num(
				data.mediumSeverityCount ??
					data.medium_severity_count ??
					data.medium ??
					bucket.MEDIUM ??
					bucket.medium,
			),
			low: num(
				data.lowSeverityCount ??
					data.low_severity_count ??
					data.low ??
					bucket.LOW ??
					bucket.low,
			),
		};
	} catch (e) {
		if (e instanceof Error && /\b404\b/.test(e.message)) {
			return { total: 0, high: 0, medium: 0, low: 0 };
		}
		throw e;
	}
}

/** POST /app/shipments/anomalies/{shipment_id}/{anomaly_type}/acknowledge */
export async function acknowledgeShipmentAnomaly(
	shipmentId: string,
	anomalyType: ShipmentAnomalyType,
): Promise<void> {
	await authedFetch(
		`/app/shipments/anomalies/${encodeURIComponent(shipmentId)}/${encodeURIComponent(anomalyType)}/acknowledge`,
		{ method: "POST" },
	);
}

/** POST /app/shipments/anomalies/{shipment_id}/{anomaly_type}/ignore */
export async function ignoreShipmentAnomaly(
	shipmentId: string,
	anomalyType: ShipmentAnomalyType,
): Promise<void> {
	await authedFetch(
		`/app/shipments/anomalies/${encodeURIComponent(shipmentId)}/${encodeURIComponent(anomalyType)}/ignore`,
		{ method: "POST" },
	);
}

/** Build the `orderNo → ShipmentAnomalyItem` lookup that the orders list
 *  uses to surface the red-corner dot. When an order has multiple anomaly
 *  types, the highest severity (HIGH > MEDIUM > LOW) wins so the
 *  indicator's tone always reflects the worst issue. */
const SEVERITY_ORDER: Record<ShipmentAnomalySeverity, number> = {
	HIGH: 3,
	MEDIUM: 2,
	LOW: 1,
};

export function buildShipmentAnomalyLookup(
	items: ReadonlyArray<ShipmentAnomalyItem>,
): Map<string, ShipmentAnomalyItem> {
	const map = new Map<string, ShipmentAnomalyItem>();
	for (const item of items) {
		if (!item.orderNo) continue;
		// Only un-acknowledged rows light up the red dot — once the seller
		// has acted (ack / ignore), the order card returns to a clean state
		// even if the underlying shipment is still anomalous.
		if (item.ackAction) continue;
		const existing = map.get(item.orderNo);
		if (
			!existing ||
			(SEVERITY_ORDER[item.severity] ?? 0) >
				(SEVERITY_ORDER[existing.severity] ?? 0)
		) {
			map.set(item.orderNo, item);
		}
	}
	return map;
}
