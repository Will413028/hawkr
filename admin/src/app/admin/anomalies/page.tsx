"use client";

import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import type React from "react";
import { useMemo, useState } from "react";
import { StatusBadge } from "@/components/UI";
import type { BadgeVariant } from "@/components/UI/StatusBadge";
import {
	type AnomalyHandoff,
	parseAnomalyHandoff,
} from "@/features/delegated-operations/anomaly-handoff";
import {
	codeLabel,
	ORDER_STATUS_LABEL,
	PAYMENT_METHOD_LABEL,
	PAYMENT_TRANSACTION_STATUS_LABEL,
	PROVIDER_SOURCE_LABEL,
	REFUND_STATUS_LABEL,
	SHIPMENT_ANOMALY_ACK_ACTION_LABEL,
	SHIPMENT_ANOMALY_SEVERITY_LABEL,
	SHIPMENT_ANOMALY_SEVERITY_ORDER,
	SHIPMENT_ANOMALY_TYPE_LABEL,
	SHIPPING_METHOD_LABEL,
	SHIPPING_STATUS_LABEL,
	toOptions,
} from "@/lib/codemeta";
import { formatDateTime as formatTaipeiDateTimeShared } from "@/lib/format";
import { api } from "@/services/api";

// issue #1327：後端 6 群 admin 異常查詢 API 早已存在但前端零串接。本頁補上「異常監控」
// 多 tab 面板：配送異常 / 異常訂單 / 付款異常 / 退款異常 / 撥款異常。
// 全走既有 BFF（api → /api/bff/[...path]），分頁 envelope {records,total,page,pageSize}（字串）。

type TabKey = "shipping" | "orders" | "payments" | "refunds" | "grants";

const TABS: Array<{ key: TabKey; label: string; icon: MaterialSymbolName }> = [
	{ key: "shipping", label: "配送異常", icon: "local_shipping" },
	{ key: "orders", label: "異常訂單", icon: "assignment_late" },
	{ key: "payments", label: "付款異常", icon: "credit_card" },
	{ key: "refunds", label: "退款異常", icon: "money_off" },
	{ key: "grants", label: "撥款異常", icon: "paid" },
];

const PAGE_SIZE = 20;

// ==================== 共用 UI ====================

const thStyle: React.CSSProperties = {
	padding: "12px 16px",
	textAlign: "left",
	fontSize: 13,
	fontWeight: 600,
	color: "#374151",
	whiteSpace: "nowrap",
};

const tdStyle: React.CSSProperties = {
	padding: "14px 16px",
	fontSize: 14,
	color: "#374151",
	verticalAlign: "middle",
};

// 時間一律走 lib/format 的台北釘死版（backend#3053 的稽查）：本地這份用
// toLocaleString 會跟著看的人的瀏覽器時區跑，而管理員不一定在台灣。
const formatDT = formatTaipeiDateTimeShared;

function formatAmount(raw: string | undefined): string {
	const n = Number(raw);
	if (!Number.isFinite(n)) return raw || "—";
	return `NT$ ${n.toLocaleString()}`;
}

function severityVariant(sev: string): BadgeVariant {
	switch (sev) {
		case "HIGH":
			return "danger";
		case "MEDIUM":
			return "warning";
		default:
			return "gray";
	}
}

function pageBtnStyle(disabled: boolean): React.CSSProperties {
	return {
		background: "white",
		border: "1px solid #dadada",
		borderRadius: 8,
		padding: "6px 14px",
		fontSize: 14,
		fontWeight: 600,
		color: disabled ? "#9ca3af" : "#374151",
		cursor: disabled ? "not-allowed" : "pointer",
		opacity: disabled ? 0.6 : 1,
	};
}

function Pagination({
	page,
	total,
	onPage,
}: {
	page: number;
	total: number;
	onPage: (p: number) => void;
}) {
	const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
	if (totalPages <= 1) return null;
	return (
		<div
			style={{
				display: "flex",
				justifyContent: "center",
				alignItems: "center",
				gap: 16,
				marginTop: 16,
			}}
		>
			<button
				type="button"
				onClick={() => onPage(Math.max(1, page - 1))}
				disabled={page === 1}
				style={pageBtnStyle(page === 1)}
			>
				<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
			</button>
			<span style={{ color: "#6b7280" }}>
				第 {page} / {totalPages} 頁，共 {total} 筆
			</span>
			<button
				type="button"
				onClick={() => onPage(Math.min(totalPages, page + 1))}
				disabled={page >= totalPages}
				style={pageBtnStyle(page >= totalPages)}
			>
				下一頁 <MaterialSymbol name="keyboard_arrow_right" />
			</button>
		</div>
	);
}

function StateRow({
	loading,
	empty,
	colSpan,
	children,
}: {
	loading: boolean;
	empty: boolean;
	colSpan: number;
	children: React.ReactNode;
}) {
	if (loading) {
		return (
			<tr>
				<td
					colSpan={colSpan}
					style={{ textAlign: "center", padding: 40, color: "#9ca3af" }}
				>
					載入中...
				</td>
			</tr>
		);
	}
	if (empty) {
		return (
			<tr>
				<td
					colSpan={colSpan}
					style={{ textAlign: "center", padding: 40, color: "#9ca3af" }}
				>
					暫無異常資料
				</td>
			</tr>
		);
	}
	return <>{children}</>;
}

const actionBtnStyle: React.CSSProperties = {
	border: "1px solid var(--admin-primary-strong)",
	background: "white",
	color: "var(--admin-primary-strong)",
	borderRadius: 6,
	padding: "4px 10px",
	fontSize: 13,
	fontWeight: 600,
	cursor: "pointer",
	marginRight: 6,
};

const ignoreBtnStyle: React.CSSProperties = {
	...actionBtnStyle,
	border: "1px solid #9ca3af",
	color: "#6b7280",
};

// ==================== 配送異常 ====================

interface AdminShipmentAnomaly {
	shipmentId: string;
	orderNo: string;
	storeId: string;
	anomalyType: string;
	anomalyTypeLabel: string;
	severity: string;
	shippingMethod: string;
	shipmentStatus: string;
	detectedAt: string;
	ackAction: string; // admin 端處置
	actedAt: string;
	sellerAckAction: string; // #1327 賣家端處置（唯讀）
	sellerActedAt: string;
}

interface ShipmentAnomalyListResp {
	code: number;
	data: {
		records: AdminShipmentAnomaly[];
		total: string;
		page: string;
		pageSize: string;
	};
}

function ShippingAnomaliesTab() {
	const qc = useQueryClient();
	const [page, setPage] = useState(1);
	const [anomalyType, setAnomalyType] = useState("");
	const [severity, setSeverity] = useState("");
	const [showAcked, setShowAcked] = useState(false);
	const [sellerId, setSellerId] = useState("");
	const [sellerIdApplied, setSellerIdApplied] = useState("");

	const queryKey = [
		"anomalies",
		"shipping",
		{ page, anomalyType, severity, showAcked, sellerId: sellerIdApplied },
	];

	const { data, isLoading } = useQuery({
		queryKey,
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
				show_acknowledged: String(showAcked),
			});
			if (anomalyType) params.set("anomaly_type", anomalyType);
			if (severity) params.set("severity", severity);
			// 後端 /admin/shipments/anomalies 讀的是 store_id（見
			// shipment_anomaly.go resolveOptionalSellerFilterQuery，內部把此值當
			// account_id 解析成現任 store_id 過濾——鍵名雖叫 store_id，值本身仍是
			// 原始 account_id，gateway 負責解析）。2026-08 硬切自舊的
			// seller_account_id，後端已移除舊鍵 fallback，改錯鍵名會讓篩選被後端
			// DefaultQuery 忽略、靜默退化成「顯示全部」。
			if (sellerIdApplied) params.set("store_id", sellerIdApplied);
			const res = await api.get<ShipmentAnomalyListResp>(
				`/api/v1/admin/shipments/anomalies?${params}`,
			);
			return {
				records: res?.data?.records || [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const act = useMutation({
		mutationFn: (vars: {
			shipmentId: string;
			anomalyType: string;
			action: "acknowledge" | "ignore";
		}) =>
			api.post(
				`/api/v1/admin/shipments/anomalies/${vars.shipmentId}/${vars.anomalyType}/${vars.action}`,
				{},
			),
		onSuccess: () =>
			qc.invalidateQueries({ queryKey: ["anomalies", "shipping"] }),
		onError: (e: unknown) =>
			alert(`操作失敗：${e instanceof Error ? e.message : String(e)}`),
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					value={anomalyType}
					onChange={(e) => {
						setAnomalyType(e.target.value);
						setPage(1);
					}}
				>
					<option value="">所有異常類型</option>
					{toOptions(SHIPMENT_ANOMALY_TYPE_LABEL).map(([v, l]) => (
						<option key={v} value={v}>
							{l}
						</option>
					))}
				</select>
				<select
					className="filter-select"
					value={severity}
					onChange={(e) => {
						setSeverity(e.target.value);
						setPage(1);
					}}
				>
					<option value="">所有嚴重度</option>
					{toOptions(
						SHIPMENT_ANOMALY_SEVERITY_LABEL,
						SHIPMENT_ANOMALY_SEVERITY_ORDER,
					).map(([v, l]) => (
						<option key={v} value={v}>
							{l}
						</option>
					))}
				</select>
				<input
					className="filter-select"
					type="text"
					placeholder="賣家 ID"
					value={sellerId}
					onChange={(e) => setSellerId(e.target.value)}
					onKeyDown={(e) => {
						if (e.key === "Enter") {
							setSellerIdApplied(sellerId.trim());
							setPage(1);
						}
					}}
				/>
				<label
					style={{
						display: "flex",
						alignItems: "center",
						gap: 6,
						color: "#374151",
					}}
				>
					<input
						type="checkbox"
						checked={showAcked}
						onChange={(e) => {
							setShowAcked(e.target.checked);
							setPage(1);
						}}
					/>
					含已處理 / 已忽略
				</label>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>訂單 / 出貨單</th>
							<th style={thStyle}>賣家</th>
							<th style={thStyle}>異常類型</th>
							<th style={thStyle}>嚴重度</th>
							<th style={thStyle}>物流方式 / 貨態</th>
							<th style={thStyle}>偵測時間</th>
							<th style={thStyle}>Admin 處置</th>
							<th style={thStyle}>賣家處置</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							empty={records.length === 0}
							colSpan={9}
						>
							{records.map((r) => {
								const acted = !!r.ackAction;
								return (
									<tr key={`${r.shipmentId}-${r.anomalyType}`}>
										<td style={tdStyle}>
											<div style={{ fontWeight: 600 }}>{r.orderNo || "—"}</div>
											<div style={{ fontSize: 12, color: "#9ca3af" }}>
												#{r.shipmentId}
											</div>
										</td>
										<td style={tdStyle}>{r.storeId || "—"}</td>
										<td style={tdStyle}>
											{r.anomalyTypeLabel ||
												codeLabel(SHIPMENT_ANOMALY_TYPE_LABEL, r.anomalyType)}
										</td>
										<td style={tdStyle}>
											<StatusBadge variant={severityVariant(r.severity)}>
												{codeLabel(SHIPMENT_ANOMALY_SEVERITY_LABEL, r.severity)}
											</StatusBadge>
										</td>
										<td style={tdStyle}>
											<div>
												{codeLabel(SHIPPING_METHOD_LABEL, r.shippingMethod)}
											</div>
											<div style={{ fontSize: 12, color: "#9ca3af" }}>
												{codeLabel(SHIPPING_STATUS_LABEL, r.shipmentStatus)}
											</div>
										</td>
										<td style={tdStyle}>{formatDT(r.detectedAt)}</td>
										<td style={tdStyle}>
											{acted ? (
												<div>
													<StatusBadge
														variant={
															r.ackAction === "IGNORED" ? "gray" : "success"
														}
													>
														{codeLabel(
															SHIPMENT_ANOMALY_ACK_ACTION_LABEL,
															r.ackAction,
														)}
													</StatusBadge>
													<div style={{ fontSize: 12, color: "#9ca3af" }}>
														{formatDT(r.actedAt)}
													</div>
												</div>
											) : (
												<span style={{ color: "#9ca3af" }}>未處理</span>
											)}
										</td>
										<td style={tdStyle}>
											{r.sellerAckAction ? (
												<div>
													<StatusBadge
														variant={
															r.sellerAckAction === "IGNORED"
																? "gray"
																: "success"
														}
													>
														{codeLabel(
															SHIPMENT_ANOMALY_ACK_ACTION_LABEL,
															r.sellerAckAction,
														)}
													</StatusBadge>
													<div style={{ fontSize: 12, color: "#9ca3af" }}>
														{formatDT(r.sellerActedAt)}
													</div>
												</div>
											) : (
												<span style={{ color: "#9ca3af" }}>—</span>
											)}
										</td>
										<td style={tdStyle}>
											<button
												type="button"
												style={actionBtnStyle}
												disabled={act.isPending}
												onClick={() =>
													act.mutate({
														shipmentId: r.shipmentId,
														anomalyType: r.anomalyType,
														action: "acknowledge",
													})
												}
											>
												標記已處理
											</button>
											<button
												type="button"
												style={ignoreBtnStyle}
												disabled={act.isPending}
												onClick={() =>
													act.mutate({
														shipmentId: r.shipmentId,
														anomalyType: r.anomalyType,
														action: "ignore",
													})
												}
											>
												忽略
											</button>
										</td>
									</tr>
								);
							})}
						</StateRow>
					</tbody>
				</table>
			</div>
			<Pagination page={page} total={total} onPage={setPage} />
		</div>
	);
}

// ==================== 異常訂單 ====================

interface AbnormalOrder {
	orderNo: string;
	buyerId: string;
	storeId: string;
	status: string;
	totalActualAmount: string;
	paymentMethod: string;
	shippingMethod: string;
	providerStatus: string;
	providerMessage: string;
	providerSource: string;
	createdAt: string;
	updatedAt: string;
}

interface AbnormalOrderListResp {
	code: number;
	data: {
		records: AbnormalOrder[];
		total: string;
		page: string;
		pageSize: string;
	};
}

// 異常訂單 status 常見值（預設 35 物流異常）。
const ABNORMAL_ORDER_STATUS_OPTS = ["35", "30", "32", "40", "60", "70", "80"];

function AbnormalOrdersTab({ handoff }: { handoff: AnomalyHandoff }) {
	const [page, setPage] = useState(1);
	// 代行交接帶了狀態就用它：`GET /admin/orders/abnormal` 未帶 status 時後端預設
	// 只回 35，從一筆付款異常的訂單點過來會落在一份查無此單的清單上。
	const [status, setStatus] = useState(handoff.status ?? "35");
	// providerSource 後端無 server 端過濾參數（僅 provider_status raw-code exact match），
	// 故此處為本頁 client 端檢視過濾；筆數以伺服器 total 為準。
	const [providerSource, setProviderSource] = useState("");
	// 交接帶來的訂單編號同樣沒有 server 參數（該端點的 @Param 清單無 order_no），
	// 所以它是**本頁**過濾；真正把範圍收窄的是 store_id，那個有 server 參數。
	const [orderNo, setOrderNo] = useState(handoff.orderNo ?? "");
	const storeId = handoff.storeId;

	const { data, isLoading } = useQuery({
		queryKey: ["anomalies", "orders", { page, status, storeId }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
			});
			if (status) params.set("status", status);
			if (storeId) params.set("store_id", storeId);
			const res = await api.get<AbnormalOrderListResp>(
				`/api/v1/admin/orders/abnormal?${params}`,
			);
			return {
				records: res?.data?.records || [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;
	const visible = useMemo(() => {
		let rows = records;
		if (providerSource) {
			rows = rows.filter((r) => r.providerSource === providerSource);
		}
		if (orderNo) {
			rows = rows.filter((r) => r.orderNo.includes(orderNo));
		}
		return rows;
	}, [records, providerSource, orderNo]);

	// 交接帶來的 status 可能不在常見值清單裡（例如從一筆已付款訂單點過來），
	// 補進選項才不會出現一個「選了卻不在選單裡」的下拉。
	const statusOpts = useMemo(
		() =>
			status && !ABNORMAL_ORDER_STATUS_OPTS.includes(status)
				? [status, ...ABNORMAL_ORDER_STATUS_OPTS]
				: ABNORMAL_ORDER_STATUS_OPTS,
		[status],
	);

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					value={status}
					onChange={(e) => {
						setStatus(e.target.value);
						setPage(1);
					}}
				>
					<option value="">所有訂單狀態</option>
					{statusOpts.map((v) => (
						<option key={v} value={v}>
							{codeLabel(ORDER_STATUS_LABEL, v)}
						</option>
					))}
				</select>
				<input
					className="filter-select"
					placeholder="訂單編號（本頁過濾）"
					value={orderNo}
					onChange={(e) => setOrderNo(e.target.value)}
					title="本頁檢視過濾（後端無此 server 參數）"
				/>
				<select
					className="filter-select"
					value={providerSource}
					onChange={(e) => setProviderSource(e.target.value)}
					title="本頁檢視過濾（後端無此 server 參數）"
				>
					<option value="">所有異常來源（本頁）</option>
					{toOptions(PROVIDER_SOURCE_LABEL).map(([v, l]) => (
						<option key={v} value={v}>
							{l}
						</option>
					))}
				</select>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>訂單編號</th>
							<th style={thStyle}>買家 / 賣家</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>付款 / 物流</th>
							<th style={thStyle}>異常來源</th>
							<th style={thStyle}>Provider 狀態 / 訊息</th>
							<th style={thStyle}>更新時間</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							empty={visible.length === 0}
							colSpan={8}
						>
							{visible.map((r) => (
								<tr
									key={r.orderNo}
									// 交接帶來的那一筆標出來：清單裡同店同狀態可能有好幾筆，
									// 「帶了訂單編號過來」卻要人自己一列一列對，等於沒帶。
									style={
										handoff.orderNo && r.orderNo === handoff.orderNo
											? { background: "#fff7e6" }
											: undefined
									}
								>
									<td style={tdStyle}>
										<span style={{ fontWeight: 600 }}>{r.orderNo}</span>
									</td>
									<td style={tdStyle}>
										<div>買 {r.buyerId || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											賣 {r.storeId || "—"}
										</div>
									</td>
									<td style={tdStyle}>
										<StatusBadge
											variant={r.status === "35" ? "danger" : "warning"}
										>
											{codeLabel(ORDER_STATUS_LABEL, r.status)}
										</StatusBadge>
									</td>
									<td style={tdStyle}>{formatAmount(r.totalActualAmount)}</td>
									<td style={tdStyle}>
										<div>
											{codeLabel(PAYMENT_METHOD_LABEL, r.paymentMethod)}
										</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{codeLabel(SHIPPING_METHOD_LABEL, r.shippingMethod)}
										</div>
									</td>
									<td style={tdStyle}>
										{r.providerSource
											? codeLabel(PROVIDER_SOURCE_LABEL, r.providerSource)
											: "—"}
									</td>
									<td style={tdStyle}>
										<div>{r.providerStatus || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerMessage || ""}
										</div>
									</td>
									<td style={tdStyle}>{formatDT(r.updatedAt)}</td>
								</tr>
							))}
						</StateRow>
					</tbody>
				</table>
			</div>
			<Pagination page={page} total={total} onPage={setPage} />
		</div>
	);
}

// ==================== 付款異常 ====================

interface PaymentTxn {
	tradeNo: string;
	orderNo: string;
	buyerId: string;
	merId: string;
	amount: string;
	paymentType: string;
	status: string;
	providerStatus: string;
	providerMessage: string;
	providerTradeNo: string;
	createdAt: string;
	updatedAt: string;
}

interface PaymentTxnListResp {
	code: number;
	data: {
		records: PaymentTxn[];
		total: string;
		page: string;
		pageSize: string;
	};
}

function PaymentAnomaliesTab() {
	const [page, setPage] = useState(1);
	const [paymentType, setPaymentType] = useState("");
	const [orderNo, setOrderNo] = useState("");
	const [orderNoApplied, setOrderNoApplied] = useState("");

	const { data, isLoading } = useQuery({
		queryKey: [
			"anomalies",
			"payments",
			{ page, paymentType, orderNo: orderNoApplied },
		],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
				abnormal_only: "true",
			});
			if (paymentType) params.set("payment_type", paymentType);
			if (orderNoApplied) params.set("order_no", orderNoApplied);
			const res = await api.get<PaymentTxnListResp>(
				`/api/v1/admin/payment_transactions?${params}`,
			);
			return {
				records: res?.data?.records || [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					value={paymentType}
					onChange={(e) => {
						setPaymentType(e.target.value);
						setPage(1);
					}}
				>
					<option value="">所有付款方式</option>
					{toOptions(PAYMENT_METHOD_LABEL).map(([v, l]) => (
						<option key={v} value={v}>
							{l}
						</option>
					))}
				</select>
				<input
					className="filter-select"
					type="text"
					placeholder="訂單編號"
					value={orderNo}
					onChange={(e) => setOrderNo(e.target.value)}
					onKeyDown={(e) => {
						if (e.key === "Enter") {
							setOrderNoApplied(orderNo.trim());
							setPage(1);
						}
					}}
				/>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>交易編號</th>
							<th style={thStyle}>訂單 / 買家</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>付款方式</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>Provider 狀態 / 訊息</th>
							<th style={thStyle}>建立時間</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							empty={records.length === 0}
							colSpan={7}
						>
							{records.map((r) => (
								<tr key={r.tradeNo}>
									<td style={tdStyle}>
										<div style={{ fontWeight: 600 }}>{r.tradeNo}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerTradeNo || ""}
										</div>
									</td>
									<td style={tdStyle}>
										<div>{r.orderNo || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											買 {r.buyerId || "—"}
										</div>
									</td>
									<td style={tdStyle}>{formatAmount(r.amount)}</td>
									<td style={tdStyle}>
										{codeLabel(PAYMENT_METHOD_LABEL, r.paymentType)}
									</td>
									<td style={tdStyle}>
										<StatusBadge
											variant={
												r.status === "1"
													? "success"
													: r.status === "2" || r.status === "4"
														? "danger"
														: "warning"
											}
										>
											{codeLabel(PAYMENT_TRANSACTION_STATUS_LABEL, r.status)}
										</StatusBadge>
									</td>
									<td style={tdStyle}>
										<div>{r.providerStatus || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerMessage || ""}
										</div>
									</td>
									<td style={tdStyle}>{formatDT(r.createdAt)}</td>
								</tr>
							))}
						</StateRow>
					</tbody>
				</table>
			</div>
			<Pagination page={page} total={total} onPage={setPage} />
		</div>
	);
}

// ==================== 退款異常 ====================

interface RefundRow {
	refundNo: string;
	tradeNo: string;
	orderNo: string;
	amount: string;
	status: string;
	providerStatus: string;
	providerMessage: string;
	providerRefundNo: string;
	reason: string;
	completedAt: string;
	createdAt: string;
	updatedAt: string;
}

interface RefundListResp {
	code: number;
	data: {
		records: RefundRow[];
		total: string;
		page: string;
		pageSize: string;
	};
}

function RefundAnomaliesTab() {
	const [page, setPage] = useState(1);
	const [status, setStatus] = useState("");

	const { data, isLoading } = useQuery({
		queryKey: ["anomalies", "refunds", { page, status }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
				abnormal_only: "true",
			});
			if (status) params.set("status", status);
			const res = await api.get<RefundListResp>(
				`/api/v1/admin/payment_refunds?${params}`,
			);
			return {
				records: res?.data?.records || [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const records = data?.records ?? [];
	const total = data?.total ?? 0;

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					value={status}
					onChange={(e) => {
						setStatus(e.target.value);
						setPage(1);
					}}
				>
					<option value="">所有退款狀態</option>
					{toOptions(REFUND_STATUS_LABEL).map(([v, l]) => (
						<option key={v} value={v}>
							{l}
						</option>
					))}
				</select>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>退款編號</th>
							<th style={thStyle}>交易 / 訂單</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>Provider 狀態 / 訊息</th>
							<th style={thStyle}>原因</th>
							<th style={thStyle}>建立時間</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							empty={records.length === 0}
							colSpan={7}
						>
							{records.map((r) => (
								<tr key={r.refundNo}>
									<td style={tdStyle}>
										<div style={{ fontWeight: 600 }}>{r.refundNo}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerRefundNo || ""}
										</div>
									</td>
									<td style={tdStyle}>
										<div>{r.tradeNo || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.orderNo || "—"}
										</div>
									</td>
									<td style={tdStyle}>{formatAmount(r.amount)}</td>
									<td style={tdStyle}>
										<StatusBadge
											variant={
												r.status === "2"
													? "success"
													: r.status === "3"
														? "danger"
														: "warning"
											}
										>
											{codeLabel(REFUND_STATUS_LABEL, r.status)}
										</StatusBadge>
									</td>
									<td style={tdStyle}>
										<div>{r.providerStatus || "—"}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerMessage || ""}
										</div>
									</td>
									<td style={tdStyle}>{r.reason || "—"}</td>
									<td style={tdStyle}>{formatDT(r.createdAt)}</td>
								</tr>
							))}
						</StateRow>
					</tbody>
				</table>
			</div>
			<Pagination page={page} total={total} onPage={setPage} />
		</div>
	);
}

// ==================== 撥款異常 ====================

interface GrantRow {
	tradeNo: string;
	orderNo: string;
	amount: string;
	grantStatus: string;
	grantMessage: string;
	grantedAt: string;
	providerTradeNo: string;
	createdAt: string;
	updatedAt: string;
}

interface GrantListResp {
	code: number;
	data: {
		records: GrantRow[];
		total: string;
		page: string;
		pageSize: string;
	};
}

function GrantAnomaliesTab() {
	const qc = useQueryClient();
	const [page, setPage] = useState(1);

	const { data, isLoading } = useQuery({
		queryKey: ["anomalies", "grants", { page }],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: String(page),
				page_size: String(PAGE_SIZE),
				abnormal_only: "true",
			});
			const res = await api.get<GrantListResp>(
				`/api/v1/admin/grants?${params}`,
			);
			return {
				records: res?.data?.records || [],
				total: Number(res?.data?.total) || 0,
			};
		},
	});

	const retry = useMutation({
		mutationFn: (vars: { orderNo: string; operatorReason: string }) =>
			api.post(`/api/v1/admin/orders/${vars.orderNo}/retry_grant`, {
				operatorReason: vars.operatorReason,
			}),
		onSuccess: () => {
			qc.invalidateQueries({ queryKey: ["anomalies", "grants"] });
			alert("已重新觸發撥款，Worker 下一輪掃描會重試。");
		},
		onError: (e: unknown) =>
			alert(`重試撥款失敗：${e instanceof Error ? e.message : String(e)}`),
	});

	const handleRetry = (orderNo: string) => {
		const reason = window.prompt(
			`重試訂單 ${orderNo} 的撥款？可填重發原因（稽核用，選填）：`,
			"",
		);
		if (reason === null) return; // 取消
		retry.mutate({ orderNo, operatorReason: reason });
	};

	const records = data?.records ?? [];
	const total = data?.total ?? 0;

	return (
		<div>
			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>交易編號</th>
							<th style={thStyle}>訂單編號</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>撥款狀態 / 訊息</th>
							<th style={thStyle}>撥款時間</th>
							<th style={thStyle}>更新時間</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={isLoading}
							empty={records.length === 0}
							colSpan={7}
						>
							{records.map((r) => (
								<tr key={r.tradeNo}>
									<td style={tdStyle}>
										<div style={{ fontWeight: 600 }}>{r.tradeNo}</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.providerTradeNo || ""}
										</div>
									</td>
									<td style={tdStyle}>{r.orderNo || "—"}</td>
									<td style={tdStyle}>{formatAmount(r.amount)}</td>
									<td style={tdStyle}>
										<div style={{ fontWeight: 600, color: "#b45309" }}>
											{r.grantStatus || "—"}
										</div>
										<div style={{ fontSize: 12, color: "#9ca3af" }}>
											{r.grantMessage || ""}
										</div>
									</td>
									<td style={tdStyle}>{formatDT(r.grantedAt)}</td>
									<td style={tdStyle}>{formatDT(r.updatedAt)}</td>
									<td style={tdStyle}>
										<button
											type="button"
											style={actionBtnStyle}
											disabled={retry.isPending || !r.orderNo}
											onClick={() => handleRetry(r.orderNo)}
										>
											重試撥款
										</button>
									</td>
								</tr>
							))}
						</StateRow>
					</tbody>
				</table>
			</div>
			<Pagination page={page} total={total} onPage={setPage} />
		</div>
	);
}

// ==================== 頁面 ====================

export default function AnomaliesPage() {
	// 代行工作區的訂單抽屜會帶著 tab／store_id／order_no／status 導到這裡（§6.7
	// 「平台異常處置會離開代行工作區並重新驗治理權限」）。治理權限就是 /admin 的
	// admin session，這些參數只決定落在哪個 tab、看哪一家店，不參與任何授權判定。
	const searchParams = useSearchParams();
	const handoff = useMemo(
		() => parseAnomalyHandoff(searchParams),
		[searchParams],
	);
	const [tab, setTab] = useState<TabKey>(handoff.tab ?? "shipping");

	return (
		<div>
			<div className="page-header">
				<div>
					<div className="page-title">異常監控</div>
					<div className="page-subtitle">
						配送 / 訂單 / 付款 / 退款 / 撥款 跨服務異常集中處理（issue #1327）
					</div>
				</div>
			</div>

			{handoff.orderNo ? (
				<div
					role="note"
					style={{
						display: "flex",
						alignItems: "center",
						gap: 8,
						marginBottom: 16,
						padding: "10px 14px",
						borderRadius: 8,
						border: "1px solid #f0c987",
						background: "#fff7e6",
						color: "#374151",
						fontSize: 14,
					}}
				>
					<MaterialSymbol name="swap_horiz" />
					<span>
						自代行工作區帶入：訂單 <strong>{handoff.orderNo}</strong>
						{handoff.storeId ? `（賣家 ${handoff.storeId}）` : ""}
						。此處為治理入口，處置以目前的 ADM 身分執行。
					</span>
				</div>
			) : null}

			<div
				style={{
					display: "flex",
					gap: 8,
					marginBottom: 20,
					flexWrap: "wrap",
				}}
			>
				{TABS.map((t) => (
					<button
						type="button"
						key={t.key}
						onClick={() => setTab(t.key)}
						style={{
							display: "flex",
							alignItems: "center",
							gap: 6,
							padding: "8px 16px",
							borderRadius: 8,
							border: "1px solid",
							borderColor:
								tab === t.key ? "var(--admin-primary-strong)" : "#dadada",
							background:
								tab === t.key ? "var(--admin-primary-strong)" : "white",
							color: tab === t.key ? "white" : "#374151",
							fontWeight: 600,
							cursor: "pointer",
						}}
					>
						<MaterialSymbol name={t.icon} />
						{t.label}
					</button>
				))}
			</div>

			{tab === "shipping" && <ShippingAnomaliesTab />}
			{tab === "orders" && <AbnormalOrdersTab handoff={handoff} />}
			{tab === "payments" && <PaymentAnomaliesTab />}
			{tab === "refunds" && <RefundAnomaliesTab />}
			{tab === "grants" && <GrantAnomaliesTab />}
		</div>
	);
}
