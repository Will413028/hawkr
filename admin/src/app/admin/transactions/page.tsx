"use client";

import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import type React from "react";
import { useState } from "react";
import StatCard from "@/components/UI/StatCard";
import {
	codeLabel,
	ORDER_STATUS_LABEL,
	OrderStatus,
	PAYMENT_METHOD_LABEL,
} from "@/lib/codemeta";
import { formatDateTimeShort } from "@/lib/format";
import { api } from "@/services/api";
import { useEnvironment } from "@/services/environment";

interface OrderSummary {
	orderNo: string;
	status: string;
	totalActualAmount: string;
	buyerId: string;
	buyerNickname?: string;
	storeId: string;
	sellerName?: string;
	shippingMethod?: string;
	paymentMethod?: string;
	createTime: string;
}

interface OrderListResponse {
	code: number;
	data: {
		records: OrderSummary[];
		total: string;
		page: string;
		pageSize: string;
	};
}

interface AdminOrderDetailResponse {
	code: number;
	data: {
		orderInfo?: {
			orderNo: string;
			status: string;
			totalActualAmount?: string;
		};
	};
}

const STATUS_STYLE: Record<
	string,
	{ variant: string; icon: MaterialSymbolName }
> = {
	"0": { variant: "pending", icon: "schedule" },
	"10": { variant: "approved", icon: "credit_card" },
	"20": { variant: "approved", icon: "move_to_inbox" },
	"30": { variant: "approved", icon: "local_shipping" },
	"32": { variant: "approved", icon: "storefront" },
	"35": { variant: "rejected", icon: "error" },
	"40": { variant: "approved", icon: "check_circle" },
	"50": { variant: "approved", icon: "verified_user" },
	"60": { variant: "pending", icon: "assignment_return" },
	"70": { variant: "rejected", icon: "money_off" },
	"80": { variant: "rejected", icon: "cancel" },
};

const STATUS_TAB_VALUES: Array<string | null> = [
	null,
	OrderStatus.PENDING_PAYMENT,
	OrderStatus.PAID,
	OrderStatus.SHIPPING_ABNORMAL,
	OrderStatus.COMPLETED,
	OrderStatus.FULLY_REFUNDED,
];

const STATUS_TABS: Array<{ label: string; value: string | null }> =
	STATUS_TAB_VALUES.map((value) => ({
		value,
		label: value === null ? "全部" : codeLabel(ORDER_STATUS_LABEL, value),
	}));

const TransactionList: React.FC = () => {
	const { environment } = useEnvironment();
	const [page, setPage] = useState(1);
	const [keyword, setKeyword] = useState("");
	const [appliedKeyword, setAppliedKeyword] = useState("");
	const [statusFilter, setStatusFilter] = useState<string | null>(null);
	const pageSize = 20;

	const {
		data,
		isLoading: loading,
		refetch,
	} = useQuery({
		queryKey: [
			"orders",
			"list",
			{ page, status: statusFilter, keyword: appliedKeyword, env: environment },
		],
		queryFn: async () => {
			const params = new URLSearchParams({
				page: page.toString(),
				page_size: pageSize.toString(),
			});
			if (statusFilter !== null) {
				params.append("status", statusFilter);
			}
			if (appliedKeyword) {
				params.append("order_no", appliedKeyword);
			}

			const result = await api.get<OrderListResponse>(
				`/api/v1/admin/orders?${params}`,
			);
			return {
				orders: result?.data?.records || [],
				total: Number(result?.data?.total) || 0,
			};
		},
	});
	const orders = data?.orders ?? [];
	const total = data?.total ?? 0;

	const { data: pendingCount = 0 } = useQuery({
		queryKey: ["orders", "count", "pending", environment],
		queryFn: async () => {
			const res = await api.get<OrderListResponse>(
				"/api/v1/admin/orders?status=0&page=1&page_size=1",
			);
			return Number(res?.data?.total) || 0;
		},
	});
	const { data: refundCount = 0 } = useQuery({
		queryKey: ["orders", "count", "refund", environment],
		queryFn: async () => {
			const res = await api.get<OrderListResponse>(
				"/api/v1/admin/orders?status=70&page=1&page_size=1",
			);
			return Number(res?.data?.total) || 0;
		},
	});

	const handleSearch = () => {
		setPage(1);
		setAppliedKeyword(keyword);
	};

	const getStatusBadge = (status: string) => {
		const style = STATUS_STYLE[status] ?? {
			variant: "pending",
			icon: "help",
		};
		return (
			<span className={`status-badge ${style.variant}`}>
				<MaterialSymbol name={style.icon} />{" "}
				{codeLabel(ORDER_STATUS_LABEL, status)}
			</span>
		);
	};

	const formatDateTime = formatDateTimeShort;

	const handleViewDetail = async (orderNo: string) => {
		try {
			const result = await api.get<AdminOrderDetailResponse>(
				`/api/v1/admin/orders/${orderNo}`,
			);
			const info = result?.data?.orderInfo;
			if (info) {
				alert(
					`訂單詳情:\n訂單編號: ${info.orderNo}\n金額: NT$ ${Number(
						info.totalActualAmount || 0,
					).toLocaleString()}\n狀態: ${codeLabel(ORDER_STATUS_LABEL, info.status)}`,
				);
			}
		} catch (error) {
			console.error("Failed to fetch order detail:", error);
			alert("無法載入訂單詳情");
		}
	};

	const totalPages = Math.ceil(total / pageSize);

	return (
		<div>
			<div className="page-header">
				<div>
					<div className="page-title">交易管理</div>
					<div className="page-subtitle">
						總交易：<span className="count">{total || "--"}</span> 筆
					</div>
				</div>
				<div className="header-actions">
					<button
						type="button"
						className="btn"
						onClick={() => {
							refetch();
						}}
					>
						<MaterialSymbol name="refresh" /> 刷新
					</button>
				</div>
			</div>

			<div className="stats-row">
				<StatCard
					label="交易筆數"
					value={total || "--"}
					icon="assignment"
					variant="success"
				/>
				<StatCard
					label="待付款"
					value={pendingCount || "--"}
					icon="schedule"
					variant="warning"
				/>
				<StatCard
					label="已退款"
					value={refundCount || "--"}
					icon="assignment_return"
					variant="danger"
				/>
			</div>

			<div className="filter-bar">
				<div className="filter-search">
					<MaterialSymbol name="search" />
					<input
						type="text"
						placeholder="搜尋訂單編號..."
						value={keyword}
						onChange={(e) => setKeyword(e.target.value)}
						onKeyPress={(e) => e.key === "Enter" && handleSearch()}
					/>
				</div>
				<div className="filter-tabs">
					{STATUS_TABS.map((tab) => (
						<button
							key={tab.label}
							type="button"
							className={`filter-tab ${statusFilter === tab.value ? "active" : ""}`}
							onClick={() => {
								setStatusFilter(tab.value);
								setPage(1);
							}}
						>
							{tab.label}
						</button>
					))}
				</div>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>訂單編號 / 時間</th>
							<th>買家ID</th>
							<th>賣家ID</th>
							<th>金額</th>
							<th>付款方式</th>
							<th>狀態</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td
									colSpan={7}
									style={{
										textAlign: "center",
										padding: "60px 20px",
										color: "#6b7280",
									}}
								>
									<MaterialSymbol
										name="progress_activity"
										className="icon-spin"
										style={{
											fontSize: "32px",
											marginBottom: "16px",
											display: "block",
										}}
									/>
									<p>載入中...</p>
								</td>
							</tr>
						) : orders.length === 0 ? (
							<tr>
								<td
									colSpan={7}
									style={{
										textAlign: "center",
										padding: "60px 20px",
										color: "#6b7280",
									}}
								>
									<MaterialSymbol
										name="currency_exchange"
										style={{
											fontSize: "48px",
											marginBottom: "16px",
											display: "block",
											opacity: 0.5,
										}}
									/>
									<p>尚無交易資料</p>
								</td>
							</tr>
						) : (
							orders.map((order) => (
								<tr key={order.orderNo}>
									<td>
										<div className="cell-main">{order.orderNo}</div>
										<div className="cell-sub">
											<MaterialSymbol name="schedule" />
											{formatDateTime(order.createTime)}
										</div>
									</td>
									<td>
										<div className="cell-main">
											{order.buyerNickname || order.buyerId}
										</div>
									</td>
									<td>
										<div className="cell-main">
											{order.sellerName || order.storeId}
										</div>
									</td>
									<td>
										<div
											className="cell-main"
											style={{
												fontWeight: 600,
												color: "var(--color-status-success)",
											}}
										>
											NT${" "}
											{Number(order.totalActualAmount || 0).toLocaleString()}
										</div>
									</td>
									<td>
										<div className="cell-main">
											{codeLabel(PAYMENT_METHOD_LABEL, order.paymentMethod)}
										</div>
									</td>
									<td>{getStatusBadge(order.status)}</td>
									<td>
										<button
											type="button"
											className="btn-reset action-link"
											onClick={() => handleViewDetail(order.orderNo)}
										>
											查看詳情
										</button>
									</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>


			{totalPages > 1 && (
				<div className="pagination">
					<button
						type="button"
						className="btn-page"
						disabled={page === 1}
						onClick={() => setPage((p) => p - 1)}
					>
						<MaterialSymbol name="keyboard_arrow_left" />
					</button>
					<span className="page-info">
						第 {page} / {totalPages} 頁，共 {total} 筆
					</span>
					<button
						type="button"
						className="btn-page"
						disabled={page === totalPages}
						onClick={() => setPage((p) => p + 1)}
					>
						<MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
};

export default TransactionList;
