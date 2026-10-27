"use client";

import { useQuery } from "@tanstack/react-query";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { Button } from "@/components/UI";
import {
	codeLabel,
	PRODUCT_STATUS_LABEL,
	SHIPPING_FEE_PAYER_LABEL,
	SHIPPING_METHOD_LABEL,
	SHIPPING_TEMPERATURE_LABEL,
	WEIGHT_UNIT_LABEL,
} from "@/lib/codemeta";
import { formatCategoryBreadcrumb } from "@/lib/product-description";
import { getStatusMeta } from "@/lib/product-status";
import { getProductDetail } from "@/services/products";
import "@/styles/pages/ProductDetail.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { buildImageUrl } from "@/utils/media";
import DescriptionBlockView from "./DescriptionBlockView";

export default function ProductDetailPage() {
	const params = useParams<{ id: string }>();
	const productId = params.id;
	const router = useRouter();

	const { data: product = null, isLoading } = useQuery({
		queryKey: ["products", "detail", productId],
		queryFn: () => getProductDetail(productId),
		enabled: !!productId,
	});

	if (isLoading) {
		return <div style={{ padding: 40, textAlign: "center" }}>載入中...</div>;
	}

	if (!product) {
		return (
			<div style={{ padding: 40, textAlign: "center" }}>
				<p style={{ marginBottom: 16 }}>找不到商品資料</p>
				<Button
					variant="primary"
					onClick={() => router.push("/admin/products")}
				>
					返回列表
				</Button>
			</div>
		);
	}

	const statusMeta = getStatusMeta(product.status);
	const categoryBreadcrumb = formatCategoryBreadcrumb([
		product.categoryLv1Name,
		product.categoryLv2Name,
		product.categoryLv3Name,
	]);

	return (
		<div>
			<div className="detail-header">
				<div className="detail-header-left">
					<button
						type="button"
						className="btn-back"
						aria-label="返回列表"
						onClick={() => router.push("/admin/products")}
					>
						<MaterialSymbol name="arrow_back" />
					</button>
					<div className="detail-title-section">
						<div className="detail-title-row">
							<h1 className="detail-title">{product.name}</h1>
							<span className={`status-badge ${statusMeta.className}`}>
								<MaterialSymbol name={statusMeta.icon} />{" "}
								{codeLabel(PRODUCT_STATUS_LABEL, product.status)}
							</span>
						</div>
					</div>
				</div>
			</div>

			<div className="product-detail-cards">
				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="info" /> 基本資訊
						</h3>
					</div>
					<div className="card-body">
						<div className="info-grid">
							<div className="info-field">
								<div className="field-label">分類</div>
								<div className="field-value">{categoryBreadcrumb}</div>
							</div>
							<div className="info-field">
								<div className="field-label">產地</div>
								<div className="field-value">{product.origin || "-"}</div>
							</div>
							<div className="info-field">
								<div className="field-label">評分</div>
								<div className="field-value">{product.rating || "-"}</div>
							</div>
							<div className="info-field">
								<div className="field-label">銷量 / 收藏 / 評論</div>
								<div className="field-value">
									{product.salesCount || "0"} / {product.favoriteCount} /{" "}
									{product.reviewCount}
								</div>
							</div>
						</div>
					</div>
				</div>

				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="image" /> 商品圖片
						</h3>
					</div>
					<div className="card-body">
						{product.gallery && product.gallery.length > 0 ? (
							<div className="product-gallery-grid">
								{product.gallery.map((item, i) => (
									<div
										className="product-gallery-thumb"
										key={item.assetId || `${item.imageKey}-${i}`}
									>
										<Image
											src={buildImageUrl(item.imageKey, "card")}
											alt={product.name}
											width={200}
											height={200}
											unoptimized
										/>
									</div>
								))}
							</div>
						) : (
							<p>尚無商品圖片</p>
						)}
					</div>
				</div>

				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="description" /> 商品描述
						</h3>
					</div>
					<div className="card-body">
						{product.description?.blocks &&
						product.description.blocks.length > 0 ? (
							product.description.blocks.map((block, i) => (
								<DescriptionBlockView key={block.id || i} block={block} />
							))
						) : (
							<p>尚無商品描述</p>
						)}
					</div>
				</div>

				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="sell" /> SKU 規格
						</h3>
					</div>
					<div className="card-body" style={{ overflowX: "auto" }}>
						{product.skus && product.skus.length > 0 ? (
							<table className="data-table">
								<thead>
									<tr>
										<th>規格名稱</th>
										<th>重量/單位</th>
										<th>價格</th>
										<th>特價</th>
										<th>庫存</th>
										<th>尺寸 (長x寬x高 cm)</th>
									</tr>
								</thead>
								<tbody>
									{product.skus.map((sku) => (
										<tr key={sku.id}>
											<td>{sku.skuName}</td>
											<td>
												{sku.weight
													? `${sku.weight} ${codeLabel(WEIGHT_UNIT_LABEL, sku.unit)}`
													: codeLabel(WEIGHT_UNIT_LABEL, sku.unit)}
											</td>
											<td>NT$ {sku.price}</td>
											<td>{sku.salePrice ? `NT$ ${sku.salePrice}` : "-"}</td>
											<td>{sku.stockQty}</td>
											<td>
												{sku.lengthCm && sku.widthCm && sku.heightCm
													? `${sku.lengthCm} x ${sku.widthCm} x ${sku.heightCm}`
													: "-"}
											</td>
										</tr>
									))}
								</tbody>
							</table>
						) : (
							<p>尚無 SKU 資料</p>
						)}
					</div>
				</div>

				<div className="card">
					<div className="card-header">
						<h3>
							<MaterialSymbol name="local_shipping" /> 運送方式
						</h3>
					</div>
					<div className="card-body">
						{product.shippingMethods && product.shippingMethods.length > 0 ? (
							<table className="data-table">
								<thead>
									<tr>
										<th>類型</th>
										<th>溫層</th>
										<th>免運門檻</th>
										<th>運費負擔方</th>
									</tr>
								</thead>
								<tbody>
									{product.shippingMethods.map((method, i) => (
										// biome-ignore lint/suspicious/noArrayIndexKey: shipping methods are positional, no stable id from backend
										<tr key={`${method.type}-${i}`}>
											<td>{codeLabel(SHIPPING_METHOD_LABEL, method.type)}</td>
											<td>
												{codeLabel(
													SHIPPING_TEMPERATURE_LABEL,
													method.temperature,
												)}
											</td>
											<td>NT$ {method.freeThreshold}</td>
											<td>
												{codeLabel(SHIPPING_FEE_PAYER_LABEL, method.feePayer)}
											</td>
										</tr>
									))}
								</tbody>
							</table>
						) : (
							<p>尚無運送方式資料</p>
						)}
					</div>
				</div>
			</div>
		</div>
	);
}
