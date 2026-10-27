"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Image from "next/image";
import { useRouter } from "next/navigation";
import type React from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import StatCard from "@/components/UI/StatCard";
import { codeLabel, PRODUCT_STATUS_LABEL, ProductStatus } from "@/lib/codemeta";
import { formatDate as formatTaipeiDate } from "@/lib/format";
import { formatProductPriceRange } from "@/lib/product-price";
import { getStatusMeta } from "@/lib/product-status";
import { api } from "@/services/api";
import { useEnvironment } from "@/services/environment";

// GET /api/v1/admin/products?keyword=X&page=X&page_size=X
// PUT /api/v1/admin/products/{id}/status
// DELETE /api/v1/admin/products/cache
//
// 商品狀態是業務碼字串，值域與顯示文字都以 CodeMeta 的 `ProductStatus` 為準
// （4001 草稿 / 4002 審核中 / 4003 未上架 / 4004 已上架）——這裡不再複述，
// 複述過的那一版就是漏了 4002 的那一版。id/storeId 是 TSID，一律 string。

interface Product {
	id: string;
	name: string;
	origin?: string;
	status: string;
	categoryId?: string;
	categoryName?: string;
	storeId: string;
	priceMin?: string;
	priceMax?: string;
	stock?: string;
	createdAt?: string; // unix 秒數字串
	imageUrl?: string; // gateway 已組好的完整 CDN URL（非 objectKey）
}

interface ProductListResponse {
	code: number;
	data: {
		records: Product[];
		total: string;
	};
}

const ProductList: React.FC = () => {
	const { environment } = useEnvironment();
	const router = useRouter();
	const [products, setProducts] = useState<Product[]>([]);
	const [loading, setLoading] = useState(true);
	const [searchKeyword, setSearchKeyword] = useState("");
	const [currentPage, setCurrentPage] = useState(1);
	const [totalCount, setTotalCount] = useState(0);
	const pageSize = 10;
	// searchKeyword 以 ref 持有最新值,讓 loadProducts 不隨輸入變動(搜尋為手動觸發)
	const searchKeywordRef = useRef(searchKeyword);
	searchKeywordRef.current = searchKeyword;

	const loadProducts = useCallback(async () => {
		setLoading(true);
		try {
			const params = new URLSearchParams({
				page: currentPage.toString(),
				page_size: pageSize.toString(),
			});
			if (searchKeywordRef.current) {
				params.append("keyword", searchKeywordRef.current);
			}

			const res = await api.get<ProductListResponse>(
				`/api/v1/admin/products?${params}`,
			);
			if (res?.data) {
				setProducts(res.data.records || []);
				setTotalCount(Number(res.data.total) || 0);
			}
		} catch (error) {
			console.error("Failed to load products:", error);
			setProducts([]);
		} finally {
			setLoading(false);
		}
	}, [currentPage]);

	// environment 是刻意的 refetch trigger;keyword 經由 searchKeywordRef 讀取,輸入時不重抓,
	// 僅手動搜尋(handleSearch)/分頁/環境變動時載入。
	useEffect(() => {
		loadProducts();
		void environment;
	}, [loadProducts, environment]);

	const handleSearch = () => {
		setCurrentPage(1);
		loadProducts();
	};

	const handleToggleStatus = async (product: Product) => {
		const newStatus =
			product.status === ProductStatus.ACTIVE
				? ProductStatus.INACTIVE
				: ProductStatus.ACTIVE;
		try {
			await api.put(`/api/v1/admin/products/${product.id}/status`, {
				status: newStatus,
			});
			setProducts((prev) =>
				prev.map((p) =>
					p.id === product.id ? { ...p, status: newStatus } : p,
				),
			);
		} catch (error: unknown) {
			alert(`操作失敗：${error instanceof Error ? error.message : "未知錯誤"}`);
		}
	};

	const totalProducts = totalCount;
	const publishedProducts = products.filter(
		(p) => p.status === ProductStatus.ACTIVE,
	).length;
	const draftProducts = products.filter(
		(p) => p.status === ProductStatus.DRAFT,
	).length;

	const getStatusBadge = (status: string) => {
		const meta = getStatusMeta(status);
		return (
			<span className={`status-badge ${meta.className}`}>
				<MaterialSymbol name={meta.icon} />{" "}
				{codeLabel(PRODUCT_STATUS_LABEL, status)}
			</span>
		);
	};

	// createdAt 是 unix 秒數字串（非 ISO），要先乘 1000 轉毫秒再交給 Date
	const formatDate = (unixSeconds?: string) => {
		if (!unixSeconds) return "--";
		const seconds = Number(unixSeconds);
		if (Number.isNaN(seconds)) return "--";
		// formatDate 自己分得出秒／毫秒，並釘台北（backend#3053 的稽查）。
		return formatTaipeiDate(seconds);
	};

	const totalPages = Math.ceil(totalCount / pageSize);

	return (
		<div>
			<div className="page-header">
				<div>
					<div className="page-title">商品管理</div>
					<div className="page-subtitle">
						商品總數：<span className="count">{totalProducts}</span> 件
					</div>
				</div>
				<div className="header-actions">
					<button type="button" className="btn" onClick={loadProducts}>
						<MaterialSymbol name="refresh" /> 重新整理
					</button>
				</div>
			</div>

			<div className="stats-row">
				<StatCard
					label="商品總數"
					value={totalProducts}
					icon="shopping_bag"
					variant="success"
				/>
				{/* KPI 卡與同頁列表的狀態徽章講的是同一個碼，中間只隔幾十像素——文字必須
				    同源，否則會出現「同一個狀態、兩種說法」。這裡是**狀態標籤**，走 CodeMeta；
				    下方那兩顆「上架 / 下架」按鈕是**動作用語**，不在此列、不要一起改。 */}
				<StatCard
					label={codeLabel(PRODUCT_STATUS_LABEL, ProductStatus.ACTIVE)}
					value={publishedProducts}
					icon="check_circle"
					variant="success"
				/>
				<StatCard
					label={codeLabel(PRODUCT_STATUS_LABEL, ProductStatus.DRAFT)}
					value={draftProducts}
					icon="edit"
					variant="warning"
				/>
			</div>

			<div className="filter-bar">
				<div className="filter-search">
					<MaterialSymbol name="search" />
					<input
						type="text"
						placeholder="搜尋商品名稱..."
						value={searchKeyword}
						onChange={(e) => setSearchKeyword(e.target.value)}
						onKeyPress={(e) => e.key === "Enter" && handleSearch()}
					/>
				</div>
				<button type="button" className="btn" onClick={handleSearch}>
					<MaterialSymbol name="search" /> 搜尋
				</button>
			</div>

			<div className="table-container">
				<table className="data-table">
					<thead>
						<tr>
							<th>商品資訊</th>
							<th>分類</th>
							<th>產地</th>
							<th>庫存</th>
							<th>狀態</th>
							<th>建立時間</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: "40px" }}
								>
									載入中...
								</td>
							</tr>
						) : products.length === 0 ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: "40px" }}
								>
									無商品資料
								</td>
							</tr>
						) : (
							products.map((p) => (
								<tr
									key={p.id}
									onClick={() => router.push(`/admin/products/${p.id}`)}
									style={{ cursor: "pointer" }}
								>
									<td>
										<div className="shop-info">
											<div
												className="shop-logo"
												style={{ borderRadius: "10px", background: "#f3f4f6" }}
											>
												{p.imageUrl ? (
													<Image
														src={p.imageUrl}
														alt={p.name}
														width={50}
														height={50}
														unoptimized
														onError={(e) => {
															const img = e.target as HTMLImageElement;
															img.style.display = "none";
															const fallback =
																img.nextElementSibling as HTMLElement;
															if (fallback) fallback.style.display = "flex";
														}}
													/>
												) : null}
												<MaterialSymbol
													name="eco"
													style={{
														fontSize: "24px",
														color: "var(--color-ink-500)",
														display: p.imageUrl ? "none" : "flex",
													}}
												/>
											</div>
											<div className="shop-details">
												<div className="shop-name">{p.name}</div>
												<div className="shop-type">ID: {p.id}</div>
												<div className="shop-type">
													{formatProductPriceRange(p.priceMin, p.priceMax)}
												</div>
											</div>
										</div>
									</td>
									<td>
										<div className="cell-main">{p.categoryName || "--"}</div>
									</td>
									<td>
										<div className="cell-main">{p.origin || "--"}</div>
									</td>
									<td>
										<div className="cell-main">{p.stock ?? "--"}</div>
									</td>
									<td>{getStatusBadge(p.status)}</td>
									<td>
										<div className="cell-sub">{formatDate(p.createdAt)}</div>
									</td>
									<td>
										{/* 按鈕上的「上架 / 下架」是**動作用語**，不是狀態標籤——不走 CodeMeta
										    Label（那邊是「已上架 / 未上架」）。這條界線與官網賣家中心一致。 */}
										{p.status !== ProductStatus.DRAFT && (
											<button
												type="button"
												className="btn-reset action-link"
												style={{
													color:
														p.status === ProductStatus.ACTIVE
															? "#ef4444"
															: "var(--color-status-success)",
												}}
												onClick={(e) => {
													e.stopPropagation();
													handleToggleStatus(p);
												}}
											>
												{p.status === ProductStatus.ACTIVE ? "下架" : "上架"}
											</button>
										)}
									</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			{totalPages > 1 && (
				<div
					style={{
						display: "flex",
						justifyContent: "center",
						gap: "8px",
						marginTop: "24px",
					}}
				>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
						disabled={currentPage === 1}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span style={{ padding: "6px 12px", color: "#6b7280" }}>
						第 {currentPage} / {totalPages} 頁
					</span>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
						disabled={currentPage === totalPages}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
};

export default ProductList;
