"use client";

import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { PaymentAdminService } from "@hawkr/api/gen/hawkr/admin/payment/v1alpha1/payment_pb";
import {
	PaymentRefundStatus,
	PaymentStatus,
	PaymentTransferStatus,
} from "@hawkr/api/gen/hawkr/payment/v1alpha1/payment_pb";
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
	PROVIDER_SOURCE_LABEL,
	SHIPMENT_ANOMALY_ACK_ACTION_LABEL,
	SHIPMENT_ANOMALY_SEVERITY_LABEL,
	SHIPMENT_ANOMALY_SEVERITY_ORDER,
	SHIPMENT_ANOMALY_TYPE_LABEL,
	SHIPPING_METHOD_LABEL,
	SHIPPING_STATUS_LABEL,
	toOptions,
} from "@/lib/codemeta";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTime as formatTaipeiDateTimeShared } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { api } from "@/services/api";
import {
	DEFAULT_PAYMENT_FILTERS,
	DEFAULT_REFUND_FILTERS,
	DEFAULT_TRANSFER_FILTERS,
	listPaymentRefundsRequest,
	listPaymentsRequest,
	listPaymentTransfersRequest,
	PAYMENT_STATUS_OPTIONS,
	paymentRow,
	paymentStatusText,
	REFUND_STATUS_OPTIONS,
	refundRow,
	refundStatusText,
	TRANSFER_STATUS_OPTIONS,
	transferRow,
	transferStatusText,
} from "./payment-lists";
import ResolveAnomalyAction from "./ResolveAnomalyAction";


type TabKey = "shipping" | "orders" | "payments" | "refunds" | "grants";

const TABS: Array<{ key: TabKey; label: string; icon: MaterialSymbolName }> = [
	{ key: "shipping", label: "配送異常", icon: "local_shipping" },
	{ key: "orders", label: "異常訂單", icon: "assignment_late" },
	{ key: "payments", label: "付款異常", icon: "credit_card" },
	{ key: "refunds", label: "退款異常", icon: "money_off" },
	{ key: "grants", label: "轉帳異常", icon: "paid" },
];

const PAGE_SIZE = 20;


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
	ackAction: string;
	actedAt: string;
	sellerAckAction: string;
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

const ABNORMAL_ORDER_STATUS_OPTS = ["35", "30", "32", "40", "60", "70", "80"];

function AbnormalOrdersTab({ handoff }: { handoff: AnomalyHandoff }) {
	const [page, setPage] = useState(1);
	const [status, setStatus] = useState(handoff.status ?? "35");
	const [providerSource, setProviderSource] = useState("");
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


const subTextStyle: React.CSSProperties = { fontSize: 12, color: "#9ca3af" };

function KeysetPagination({
	page,
	hasNext,
	onPrevious,
	onNext,
}: {
	page: number;
	hasNext: boolean;
	onPrevious: () => void;
	onNext: () => void;
}) {
	if (page <= 1 && !hasNext) return null;
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
				onClick={onPrevious}
				disabled={page <= 1}
				style={pageBtnStyle(page <= 1)}
			>
				<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
			</button>
			<span style={{ color: "#6b7280" }}>第 {page} 頁</span>
			<button
				type="button"
				onClick={onNext}
				disabled={!hasNext}
				style={pageBtnStyle(!hasNext)}
			>
				下一頁 <MaterialSymbol name="keyboard_arrow_right" />
			</button>
		</div>
	);
}

/** 錯誤列：讀取失敗不能顯示成「暫無資料」。 */
function ErrorRow({
	error,
	colSpan,
	onRetry,
}: {
	error: unknown;
	colSpan: number;
	onRetry: () => void;
}) {
	return (
		<tr>
			<td
				colSpan={colSpan}
				role="alert"
				style={{ textAlign: "center", padding: 40, color: "#b91c1c" }}
			>
				<div>{connectErrorText(error, "讀取失敗")}</div>
				<button
					type="button"
					style={{ ...actionBtnStyle, marginTop: 8 }}
					onClick={onRetry}
				>
					重試
				</button>
			</td>
		</tr>
	);
}

function OrderNoFilter({ onApply }: { onApply: (orderNo: string) => void }) {
	const [value, setValue] = useState("");
	return (
		<input
			className="filter-select"
			type="text"
			placeholder="訂單編號"
			aria-label="訂單編號"
			value={value}
			onChange={(e) => setValue(e.target.value)}
			onKeyDown={(e) => {
				if (e.key === "Enter") onApply(value.trim());
			}}
		/>
	);
}

function ReasonCell({ values }: { values: readonly string[] }) {
	const shown = values.filter((v) => v !== "");
	if (shown.length === 0) return <>—</>;
	return (
		<>
			{shown.map((v) => (
				<div key={v} style={{ wordBreak: "break-all" }}>
					{v}
				</div>
			))}
		</>
	);
}

function PaymentAnomaliesTab() {
	const [filters, setFilters] = useState(DEFAULT_PAYMENT_FILTERS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const list = useConnectQuery(
		PaymentAdminService.method.listPayments,
		listPaymentsRequest(filters, currentToken(pageTokens)),
	);
	const rows = (list.data?.payments ?? []).map(paymentRow);
	const next = list.data?.nextPageToken ?? "";

	const change = (patch: Partial<typeof filters>) => {
		setFilters((f) => ({ ...f, ...patch }));
		setPageTokens(firstPage());
	};

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					aria-label="付款狀態"
					value={filters.status}
					onChange={(e) =>
						change({ status: Number(e.target.value) as PaymentStatus })
					}
				>
					<option value={PaymentStatus.UNSPECIFIED}>所有付款狀態</option>
					{PAYMENT_STATUS_OPTIONS.map((s) => (
						<option key={s} value={s}>
							{paymentStatusText(s)}
						</option>
					))}
				</select>
				<OrderNoFilter onApply={(orderNo) => change({ orderNo })} />
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>付款編號</th>
							<th style={thStyle}>訂單 / 買家</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>金流商</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>錯誤 / 原因</th>
							<th style={thStyle}>建立時間</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						{list.error ? (
							<ErrorRow
								error={list.error}
								colSpan={8}
								onRetry={() => void list.refetch()}
							/>
						) : (
							<StateRow
								loading={list.isLoading}
								empty={rows.length === 0}
								colSpan={8}
							>
								{rows.map((r) => (
									<tr key={r.id}>
										<td style={tdStyle}>
											<div style={{ fontWeight: 600 }}>{r.id}</div>
											<div style={subTextStyle}>{r.externalPaymentId}</div>
										</td>
										<td style={tdStyle}>
											{r.orders.map((o) => (
												<div key={`${o.orderNo}:${o.storeId}`}>
													<div>{o.orderNo}</div>
													<div style={subTextStyle}>店 {o.storeId}</div>
												</div>
											))}
											<div style={subTextStyle}>買 {r.buyerId}</div>
										</td>
										<td style={tdStyle}>{r.amount}</td>
										<td style={tdStyle}>{r.provider}</td>
										<td style={tdStyle}>
											<StatusBadge variant={r.statusVariant}>
												{r.status}
											</StatusBadge>
										</td>
										<td style={tdStyle}>
											<ReasonCell values={[r.failureReason, r.lastError]} />
										</td>
										<td style={tdStyle}>{r.createTime}</td>
										<td style={tdStyle}>
											<ResolveAnomalyAction
												targets={r.resolveTargets}
												displayId={r.id}
											/>
										</td>
									</tr>
								))}
							</StateRow>
						)}
					</tbody>
				</table>
			</div>
			<KeysetPagination
				page={currentPage(pageTokens)}
				hasNext={next !== ""}
				onPrevious={() => setPageTokens(previousPage)}
				onNext={() => setPageTokens((t) => nextPage(t, next))}
			/>
		</div>
	);
}

function RefundAnomaliesTab() {
	const [filters, setFilters] = useState(DEFAULT_REFUND_FILTERS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const list = useConnectQuery(
		PaymentAdminService.method.listPaymentRefunds,
		listPaymentRefundsRequest(filters, currentToken(pageTokens)),
	);
	const rows = (list.data?.refunds ?? []).map(refundRow);
	const next = list.data?.nextPageToken ?? "";

	const change = (patch: Partial<typeof filters>) => {
		setFilters((f) => ({ ...f, ...patch }));
		setPageTokens(firstPage());
	};

	return (
		<div>
			<div
				className="filters"
				style={{
					display: "flex",
					gap: 12,
					marginBottom: 16,
					flexWrap: "wrap",
					alignItems: "center",
				}}
			>
				<select
					className="filter-select"
					aria-label="退款狀態"
					value={filters.status}
					onChange={(e) =>
						change({ status: Number(e.target.value) as PaymentRefundStatus })
					}
				>
					<option value={PaymentRefundStatus.UNSPECIFIED}>所有退款狀態</option>
					{REFUND_STATUS_OPTIONS.map((s) => (
						<option key={s} value={s}>
							{refundStatusText(s)}
						</option>
					))}
				</select>
				<OrderNoFilter onApply={(orderNo) => change({ orderNo })} />
				<label style={{ display: "flex", alignItems: "center", gap: 6 }}>
					<input
						type="checkbox"
						checked={filters.anomalyOnly}
						onChange={(e) => change({ anomalyOnly: e.target.checked })}
					/>
					只看異常
				</label>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>退款編號</th>
							<th style={thStyle}>訂單</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>收回 / 補差額</th>
							<th style={thStyle}>原因</th>
							<th style={thStyle}>建立時間</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						{list.error ? (
							<ErrorRow
								error={list.error}
								colSpan={8}
								onRetry={() => void list.refetch()}
							/>
						) : (
							<StateRow
								loading={list.isLoading}
								empty={rows.length === 0}
								colSpan={8}
							>
								{rows.map((r) => (
									<tr key={r.id}>
										<td style={tdStyle}>
											<div style={{ fontWeight: 600 }}>{r.id}</div>
											<div style={subTextStyle}>{r.externalRefundId}</div>
										</td>
										<td style={tdStyle}>
											<div>{r.orderNo}</div>
											<div style={subTextStyle}>{r.auto}</div>
										</td>
										<td style={tdStyle}>{r.amount}</td>
										<td style={tdStyle}>
											<StatusBadge variant={r.statusVariant}>
												{r.status}
											</StatusBadge>
										</td>
										<td style={tdStyle}>
											<div>收回 {r.reversalStatus}</div>
											<div style={subTextStyle}>補差額 {r.topupStatus}</div>
										</td>
										<td style={tdStyle}>
											<ReasonCell values={[r.failureReason]} />
										</td>
										<td style={tdStyle}>{r.createTime}</td>
										<td style={tdStyle}>
											<ResolveAnomalyAction
												targets={r.resolveTargets}
												displayId={r.id}
											/>
										</td>
									</tr>
								))}
							</StateRow>
						)}
					</tbody>
				</table>
			</div>
			<KeysetPagination
				page={currentPage(pageTokens)}
				hasNext={next !== ""}
				onPrevious={() => setPageTokens(previousPage)}
				onNext={() => setPageTokens((t) => nextPage(t, next))}
			/>
		</div>
	);
}

function GrantAnomaliesTab() {
	const [filters, setFilters] = useState(DEFAULT_TRANSFER_FILTERS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const list = useConnectQuery(
		PaymentAdminService.method.listPaymentTransfers,
		listPaymentTransfersRequest(filters, currentToken(pageTokens)),
	);
	const rows = (list.data?.transfers ?? []).map(transferRow);
	const next = list.data?.nextPageToken ?? "";

	const change = (patch: Partial<typeof filters>) => {
		setFilters((f) => ({ ...f, ...patch }));
		setPageTokens(firstPage());
	};

	return (
		<div>
			<div
				className="filters"
				style={{ display: "flex", gap: 12, marginBottom: 16, flexWrap: "wrap" }}
			>
				<select
					className="filter-select"
					aria-label="轉帳狀態"
					value={filters.status}
					onChange={(e) =>
						change({ status: Number(e.target.value) as PaymentTransferStatus })
					}
				>
					<option value={PaymentTransferStatus.UNSPECIFIED}>
						所有轉帳狀態
					</option>
					{TRANSFER_STATUS_OPTIONS.map((s) => (
						<option key={s} value={s}>
							{transferStatusText(s)}
						</option>
					))}
				</select>
				<OrderNoFilter onApply={(orderNo) => change({ orderNo })} />
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th style={thStyle}>轉帳編號</th>
							<th style={thStyle}>訂單 / 店家</th>
							<th style={thStyle}>金額</th>
							<th style={thStyle}>已收回金額</th>
							<th style={thStyle}>狀態</th>
							<th style={thStyle}>原因</th>
							<th style={thStyle}>建立時間</th>
							<th style={thStyle}>操作</th>
						</tr>
					</thead>
					<tbody>
						{list.error ? (
							<ErrorRow
								error={list.error}
								colSpan={8}
								onRetry={() => void list.refetch()}
							/>
						) : (
							<StateRow
								loading={list.isLoading}
								empty={rows.length === 0}
								colSpan={8}
							>
								{rows.map((r) => (
									<tr key={r.id}>
										<td style={tdStyle}>
											<div style={{ fontWeight: 600 }}>{r.id}</div>
											<div style={subTextStyle}>{r.externalTransferId}</div>
										</td>
										<td style={tdStyle}>
											<div>{r.orderNo}</div>
											<div style={subTextStyle}>店 {r.storeId}</div>
										</td>
										<td style={tdStyle}>{r.amount}</td>
										<td style={tdStyle}>{r.reversedAmount}</td>
										<td style={tdStyle}>
											<StatusBadge variant={r.statusVariant}>
												{r.status}
											</StatusBadge>
										</td>
										<td style={tdStyle}>
											<ReasonCell values={[r.failureReason]} />
										</td>
										<td style={tdStyle}>{r.createTime}</td>
										<td style={tdStyle}>
											<ResolveAnomalyAction
												targets={r.resolveTargets}
												displayId={r.id}
											/>
										</td>
									</tr>
								))}
							</StateRow>
						)}
					</tbody>
				</table>
			</div>
			<KeysetPagination
				page={currentPage(pageTokens)}
				hasNext={next !== ""}
				onPrevious={() => setPageTokens(previousPage)}
				onNext={() => setPageTokens((t) => nextPage(t, next))}
			/>
		</div>
	);
}


export default function AnomaliesPage() {
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
						配送 / 訂單 / 付款 / 退款 / 轉帳 跨服務異常集中處理（issue #1327）
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
