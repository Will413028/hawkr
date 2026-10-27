"use client";

import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { SellerAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useRouter } from "next/navigation";
import type React from "react";
import { useCallback, useEffect, useState } from "react";
import StatsCard from "@/components/UI/StatsCard";
import { formatStat } from "@/lib/format";
import { api } from "@/services/api";
import { useEnvironment } from "@/services/environment";

interface DashboardStats {
	memberCount: number;
	productCount: number;
	orderCount: number;
}

const Dashboard: React.FC = () => {
	const router = useRouter();
	const { environment } = useEnvironment();
	const [stats, setStats] = useState<DashboardStats>({
		memberCount: 0,
		productCount: 0,
		orderCount: 0,
	});
	// 商家數與待審核數（SellerAdminService.GetStoreStats）
	const storeStats = useConnectQuery(
		SellerAdminService.method.getStoreStats,
		{},
	);
	const refetchStoreStats = storeStats.refetch;
	const [lastUpdate, setLastUpdate] = useState<string>("--");
	const [isLoading, setIsLoading] = useState(true);

	const refreshDashboard = useCallback(async () => {
		setIsLoading(true);
		try {
			setLastUpdate(new Date().toLocaleTimeString("zh-TW"));

			// GET /api/v1/admin/products → returns total
			try {
				const res = await api.get<{ code: number; data: { total: number } }>(
					"/api/v1/admin/products?page=1&page_size=1",
				);
				if (res?.data) {
					setStats((prev) => ({
						...prev,
						productCount: res.data.total || 0,
					}));
				}
			} catch (e) {
				console.warn("Product stats not available:", e);
			}

			// GET /api/v1/admin/orders → returns total
			try {
				const res = await api.get<{ code: number; data: { total: number } }>(
					"/api/v1/admin/orders?page=1&page_size=1",
				);
				if (res?.data) {
					setStats((prev) => ({
						...prev,
						orderCount: res.data.total || 0,
					}));
				}
			} catch (e) {
				console.warn("Order stats not available:", e);
			}
		} catch (e) {
			console.error("Dashboard load error:", e);
		} finally {
			setIsLoading(false);
		}
	}, []);

	// environment 是刻意的 refetch trigger:api 層由 environmentService(module singleton) 取 gateway URL,
	// 切換環境須重載;在 body 引用 environment 使其成為 biome 可見的真實依賴。
	useEffect(() => {
		refreshDashboard();
		void environment;
	}, [environment, refreshDashboard]);

	return (
		<div>
			{/* Page Header */}
			<div className="page-header">
				<div>
					<h1 className="page-title">平台總覽</h1>
					<p className="page-subtitle">
						Hawkr 管理後台數據概覽 · 更新時間: {lastUpdate}
					</p>
				</div>
				<div className="header-actions">
					<button
						type="button"
						className="btn"
						onClick={() => {
							void refreshDashboard();
							void refetchStoreStats();
						}}
						disabled={isLoading}
					>
						<MaterialSymbol name="refresh" />
						刷新
					</button>
				</div>
			</div>

			{/* Stats Row */}
			<div className="stats-row-v2">
				<button
					type="button"
					className="btn-reset stat-card-clickable"
					onClick={() => router.push("/admin/merchants")}
				>
					<StatsCard
						label="入駐商家"
						value={formatStat(
							storeStats.data?.totalStores,
							storeStats.isLoading,
							storeStats.isError,
						)}
						icon="storefront"
						variant="success"
					/>
				</button>
				<div className="stat-card-clickable">
					<StatsCard
						label="商品總數"
						value={isLoading ? "--" : stats.productCount.toLocaleString()}
						icon="inventory_2"
						variant="success"
					/>
				</div>
				<div className="stat-card-clickable">
					<StatsCard
						label="訂單總數"
						value={isLoading ? "--" : stats.orderCount.toLocaleString()}
						icon="receipt_long"
						variant="success"
					/>
				</div>
				<button
					type="button"
					className="btn-reset stat-card-clickable"
					onClick={() => router.push("/admin/merchants?filter=pending")}
				>
					<StatsCard
						label="賣家待審核"
						value={formatStat(
							storeStats.data?.kycAwaitingReviewStores,
							storeStats.isLoading,
							storeStats.isError,
						)}
						icon="pending_actions"
						variant="warning"
					/>
				</button>
			</div>

			{/* Quick Actions */}
			<div className="card">
				<div className="card-header">
					<h3>
						<MaterialSymbol name="grid_view" /> 快捷操作
					</h3>
				</div>
				<div className="card-body">
					<div className="admin-quick-actions">
						<button
							type="button"
							className="btn admin-quick-action"
							onClick={() => router.push("/admin/merchants")}
						>
							<MaterialSymbol name="shield_person" />
							<span>賣家管理</span>
						</button>
						<button
							type="button"
							className="btn admin-quick-action"
							onClick={() => router.push("/admin/products")}
						>
							<MaterialSymbol name="potted_plant" />
							<span>商品管理</span>
						</button>
						<button
							type="button"
							className="btn admin-quick-action"
							onClick={() => router.push("/admin/transactions")}
						>
							<MaterialSymbol name="currency_exchange" />
							<span>訂單管理</span>
						</button>
						<button
							type="button"
							className="btn admin-quick-action"
							onClick={() => router.push("/admin/category-audit")}
						>
							<MaterialSymbol name="device_hub" />
							<span>分類管理</span>
						</button>
					</div>
				</div>
			</div>
		</div>
	);
};

export default Dashboard;
