"use client";

import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery } from "@tanstack/react-query";
import type React from "react";
import StatCard from "@/components/UI/StatCard";
import { formatStat } from "@/lib/format";
import { api } from "@/services/api";
import { useEnvironment } from "@/services/environment";

interface OrderStats {
	totalOrders: number;
}

interface ProductStats {
	totalProducts: number;
}

interface OrdersResponse {
	code: number;
	data: {
		total: number;
		orders: unknown[];
	};
}

interface ProductsResponse {
	code: number;
	data: {
		total: number;
		products: unknown[];
	};
}

const Analytics: React.FC = () => {
	const { environment } = useEnvironment();

	// environment 進 queryKey 是刻意的 refetch trigger（api 層由 environmentService
	// singleton 取 gateway URL，切環境須重載）。訂單與商品兩個 stat 各自
	// try/catch 隔離：任一掛掉不影響其他已成功的數字。商家數走 Connect（下面的 storeStats）。
	const {
		data,
		isLoading: loading,
		refetch,
	} = useQuery({
		queryKey: ["analytics", "stats", { env: environment }],
		queryFn: async () => {
			let orderStats: OrderStats | null = null;
			let productStats: ProductStats | null = null;

			try {
				const ordersRes = await api.get<OrdersResponse>(
					"/api/v1/admin/orders?page=1&page_size=1",
				);
				if (ordersRes && ordersRes.code === 0) {
					orderStats = { totalOrders: ordersRes.data?.total || 0 };
				}
			} catch (e) {
				console.error("Failed to fetch order stats:", e);
			}

			try {
				const productsRes = await api.get<ProductsResponse>(
					"/api/v1/admin/products?page=1&page_size=1",
				);
				if (productsRes && productsRes.code === 0) {
					productStats = { totalProducts: productsRes.data?.total || 0 };
				}
			} catch (e) {
				console.error("Failed to fetch product stats:", e);
			}

			return { orderStats, productStats };
		},
	});
	const storeStats = useConnectQuery(
		SellerAdminService.method.getStoreStats,
		{},
	);
	const merchantStats = storeStats.data
		? { totalMerchants: storeStats.data.totalStores }
		: null;
	const orderStats = data?.orderStats ?? null;
	const productStats = data?.productStats ?? null;

	return (
		<div>
			<div className="page-header">
				<div>
					<div className="page-title">數據分析</div>
					<div className="page-subtitle">平台概覽</div>
				</div>
				<div className="header-actions">
					<button
						type="button"
						className="btn"
						onClick={() => {
							void refetch();
							void storeStats.refetch();
						}}
					>
						<MaterialSymbol name="refresh" /> 刷新
					</button>
				</div>
			</div>

			{/* 核心統計 */}
			<div className="stats-row">
				<StatCard
					label="總訂單數"
					value={orderStats?.totalOrders ?? "--"}
					icon="assignment"
					variant="success"
				/>
				<StatCard
					label="總商家數"
					value={formatStat(
						storeStats.data?.totalStores,
						storeStats.isLoading,
						storeStats.isError,
					)}
					icon="storefront"
					variant="success"
				/>
				<StatCard
					label="總商品數"
					value={productStats?.totalProducts ?? "--"}
					icon="shopping_bag"
					variant="success"
				/>
			</div>

			{/* 商家與會員統計 */}
			<div className="grid-2 mb-24">
				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="storefront" /> 商家統計
						</h3>
					</div>
					<div className="card-body">
						{storeStats.isLoading ? (
							<div
								style={{
									textAlign: "center",
									padding: "40px",
									color: "#6b7280",
								}}
							>
								<MaterialSymbol
									name="progress_activity"
									className="icon-spin"
									style={{ fontSize: "24px" }}
								/>
							</div>
						) : merchantStats ? (
							<div className="stats-grid">
								<div className="stat-item">
									<div className="stat-value">
										{merchantStats.totalMerchants}
									</div>
									<div className="stat-label">總商家數</div>
								</div>
							</div>
						) : (
							<div
								style={{
									textAlign: "center",
									padding: "40px",
									color: "#6b7280",
								}}
							>
								<p>無法載入商家統計</p>
							</div>
						)}
					</div>
				</div>

				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="group" /> 會員統計
						</h3>
					</div>
					<div className="card-body">
						<div
							style={{ textAlign: "center", padding: "40px", color: "#6b7280" }}
						>
							<MaterialSymbol
								name="lock"
								style={{
									fontSize: "32px",
									marginBottom: "12px",
									display: "block",
									opacity: 0.5,
								}}
							/>
							<p>API 尚未開放</p>
						</div>
					</div>
				</div>
			</div>

			{/* 商品概覽 */}
			<div className="card mb-24">
				<div className="card-header">
					<h3>
						<MaterialSymbol name="shopping_bag" /> 商品概覽
					</h3>
				</div>
				<div className="card-body">
					{loading ? (
						<div
							style={{ textAlign: "center", padding: "40px", color: "#6b7280" }}
						>
							<MaterialSymbol
								name="progress_activity"
								className="icon-spin"
								style={{ fontSize: "24px" }}
							/>
						</div>
					) : productStats ? (
						<div className="order-status-grid">
							<div className="status-card total">
								<MaterialSymbol name="shopping_bag" />
								<div className="status-count">{productStats.totalProducts}</div>
								<div className="status-label">總商品數</div>
							</div>
							<div className="status-card completed">
								<MaterialSymbol name="assignment" />
								<div className="status-count">
									{orderStats?.totalOrders ?? "--"}
								</div>
								<div className="status-label">總訂單數</div>
							</div>
							<div className="status-card pending">
								<MaterialSymbol name="storefront" />
								<div className="status-count">
									{merchantStats?.totalMerchants ?? "--"}
								</div>
								<div className="status-label">總商家數</div>
							</div>
						</div>
					) : (
						<div
							style={{
								textAlign: "center",
								padding: "60px 20px",
								color: "#6b7280",
							}}
						>
							<MaterialSymbol
								name="stacked_bar_chart"
								style={{
									fontSize: "48px",
									marginBottom: "16px",
									display: "block",
									opacity: 0.5,
								}}
							/>
							<p>無法載入商品統計</p>
						</div>
					)}
				</div>
			</div>

			{/* 營收趨勢 */}
			<div className="card">
				<div className="card-header">
					<h3>
						<MaterialSymbol name="bar_chart" /> 營收趨勢
					</h3>
				</div>
				<div
					className="card-body"
					style={{
						textAlign: "center",
						padding: "60px 20px",
						color: "#6b7280",
					}}
				>
					<MaterialSymbol
						name="stacked_bar_chart"
						style={{
							fontSize: "48px",
							marginBottom: "16px",
							display: "block",
							opacity: 0.5,
						}}
					/>
					<p>圖表功能開發中...</p>
					<p style={{ fontSize: "12px", marginTop: "8px" }}>
						需要後端提供時間序列數據 API
					</p>
				</div>
			</div>

			<style>{`
                .stats-grid {
                    display: grid;
                    grid-template-columns: repeat(3, 1fr);
                    gap: 24px;
                    padding: 16px 0;
                }
                .stat-item {
                    text-align: center;
                }
                .stat-value {
                    font-size: 32px;
                    font-weight: 700;
                    color: #111827;
                    line-height: 1.2;
                }
                .stat-label {
                    font-size: 14px;
                    color: #6b7280;
                    margin-top: 4px;
                }
                .order-status-grid {
                    display: grid;
                    grid-template-columns: repeat(3, 1fr);
                    gap: 24px;
                    padding: 16px 0;
                }
                .status-card {
                    text-align: center;
                    padding: 24px;
                    border-radius: 12px;
                    background: #f9fafb;
                }
                .status-card i {
                    font-size: 32px;
                    margin-bottom: 12px;
                }
                .status-card.pending {
                    background: #fef3c7;
                    color: #92400e;
                }
                .status-card.completed {
                    background: var(--color-status-success-soft);
                    color: var(--color-ink-700);
                }
                .status-card.total {
                    background: #dbeafe;
                    color: #1e40af;
                }
                .status-count {
                    font-size: 36px;
                    font-weight: 700;
                }
                .status-label {
                    font-size: 14px;
                    margin-top: 4px;
                    opacity: 0.8;
                }
            `}</style>
		</div>
	);
};

export default Analytics;
