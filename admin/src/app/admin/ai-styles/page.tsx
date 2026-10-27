"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import type React from "react";
import { useRef, useState } from "react";
import { Dialog } from "@/components/UI";
import { api } from "@/services/api";
import { useEnvironment } from "@/services/environment";
import { uploadImageToMedia } from "@/services/media-upload";
import { resolveOssUrl } from "@/utils/oss";

interface AiStyle {
	id: string;
	name: string;
	label: string;
	prompt: string;
	thumbnailUrl: string;
	sortOrder: string;
	category: "product" | "post";
	isDefault: boolean;
	enabled: boolean;
	createdAt: string;
	updatedAt: string;
}

interface ListResponse {
	code: number;
	data: AiStyle[];
}

interface QuotaData {
	storeId: string;
	weeklyLimit: string;
	isCustom: boolean;
	weeklyUsage: string;
	memo: string;
	updatedAt: string;
}

interface QuotaResponse {
	code: number;
	data: QuotaData;
}

type CategoryFilter = "product" | "post";

const CATEGORY_LABELS: Record<CategoryFilter, string> = {
	product: "商品",
	post: "貼文",
};

const AiStyleManagement: React.FC = () => {
	const { environment } = useEnvironment();
	const queryClient = useQueryClient();

	// 風格管理
	const [editingStyle, setEditingStyle] = useState<AiStyle | null>(null);
	const [showForm, setShowForm] = useState(false);
	const [saving, setSaving] = useState(false);

	// 表單
	const [formName, setFormName] = useState("");
	const [formLabel, setFormLabel] = useState("");
	const [formPrompt, setFormPrompt] = useState("");
	const [formThumbnailFile, setFormThumbnailFile] = useState<File | null>(null);
	const [thumbnailPreview, setThumbnailPreview] = useState<string | null>(null);
	const [thumbnailError, setThumbnailError] = useState("");
	const [formSortOrder, setFormSortOrder] = useState("0");
	const [formCategory, setFormCategory] = useState<CategoryFilter>("product");
	const [formEnabled, setFormEnabled] = useState(true);
	const thumbnailInputRef = useRef<HTMLInputElement>(null);

	// 額度管理
	const [quotaStoreId, setQuotaStoreId] = useState("");
	const [quotaData, setQuotaData] = useState<QuotaData | null>(null);
	const [quotaLoading, setQuotaLoading] = useState(false);
	const [quotaLimit, setQuotaLimit] = useState("");
	const [quotaMemo, setQuotaMemo] = useState("");
	const [quotaSaving, setQuotaSaving] = useState(false);

	// 目前 tab
	const [activeTab, setActiveTab] = useState<"styles" | "quotas">("styles");

	// 分類篩選（風格 Prompt 管理 tab 內）
	const [categoryFilter, setCategoryFilter] =
		useState<CategoryFilter>("product");

	// 風格列表（display read）。environment 進 queryKey：切環境 api baseUrl 改變須重抓。
	const { data: styles = [], isLoading: loading } = useQuery({
		queryKey: [
			"ai_styles",
			"list",
			{ env: environment, category: categoryFilter },
		],
		queryFn: async () => {
			const res = await api.get<ListResponse>(
				`/api/v1/admin/ai_styles?category=${categoryFilter}`,
			);
			return res?.data ?? [];
		},
	});
	const invalidateStyles = () =>
		queryClient.invalidateQueries({ queryKey: ["ai_styles"] });

	const resetForm = () => {
		setFormName("");
		setFormLabel("");
		setFormPrompt("");
		setFormThumbnailFile(null);
		setThumbnailPreview(null);
		setThumbnailError("");
		setFormSortOrder("0");
		setFormEnabled(true);
		setFormCategory(categoryFilter);
		setEditingStyle(null);
	};

	const openCreateForm = () => {
		resetForm();
		setShowForm(true);
	};

	const openEditForm = (style: AiStyle) => {
		setEditingStyle(style);
		setFormName(style.name);
		setFormLabel(style.label);
		setFormPrompt(style.prompt);
		setFormThumbnailFile(null);
		setThumbnailPreview(resolveOssUrl(style.thumbnailUrl));
		setThumbnailError("");
		setFormSortOrder(style.sortOrder);
		setFormEnabled(style.enabled);
		setFormCategory(style.category);
		setShowForm(true);
	};

	const handleThumbnailSelect = (file: File) => {
		setThumbnailError("");
		if (file.size > 5 * 1024 * 1024) {
			setThumbnailError("圖片大小不可超過 5MB");
			return;
		}
		if (!["image/png", "image/jpeg"].includes(file.type)) {
			setThumbnailError("僅支援 PNG、JPG 格式");
			return;
		}
		setFormThumbnailFile(file);
		setThumbnailPreview(URL.createObjectURL(file));
	};

	const handleThumbnailDrop = (e: React.DragEvent<HTMLButtonElement>) => {
		e.preventDefault();
		const file = e.dataTransfer.files[0];
		if (file) handleThumbnailSelect(file);
	};

	const handleSave = async () => {
		if (!formLabel.trim() || !formPrompt.trim()) {
			alert("標籤和 Prompt 為必填");
			return;
		}
		setSaving(true);
		try {
			let thumbnailStagedToken: string | undefined;
			if (formThumbnailFile) {
				thumbnailStagedToken = await uploadImageToMedia(formThumbnailFile);
			}

			if (editingStyle) {
				const payload: Record<string, unknown> = {
					label: formLabel,
					prompt: formPrompt,
					sortOrder: formSortOrder,
					enabled: formEnabled,
				};
				if (thumbnailStagedToken) {
					payload.thumbnailStagedToken = thumbnailStagedToken;
				}
				await api.put(`/api/v1/admin/ai_styles/${editingStyle.id}`, payload);
			} else {
				if (!formName.trim()) {
					alert("識別名為必填");
					setSaving(false);
					return;
				}
				const payload: Record<string, unknown> = {
					name: formName,
					label: formLabel,
					prompt: formPrompt,
					sortOrder: formSortOrder,
					category: formCategory,
					isDefault: false,
					enabled: formEnabled,
				};
				if (thumbnailStagedToken) {
					payload.thumbnailStagedToken = thumbnailStagedToken;
				}
				await api.post("/api/v1/admin/ai_styles", payload);
			}
			setShowForm(false);
			resetForm();
			invalidateStyles();
		} catch (err: unknown) {
			alert(`儲存失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		} finally {
			setSaving(false);
		}
	};

	const handleDelete = async (style: AiStyle) => {
		if (style.isDefault) {
			alert("預設風格不可刪除");
			return;
		}
		if (!confirm(`確定要刪除「${style.label}」？`)) return;
		try {
			await api.delete(`/api/v1/admin/ai_styles/${style.id}`);
			invalidateStyles();
		} catch (err: unknown) {
			alert(`刪除失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		}
	};

	const handleToggleEnabled = async (style: AiStyle) => {
		try {
			await api.put(`/api/v1/admin/ai_styles/${style.id}`, {
				enabled: !style.enabled,
			});
			invalidateStyles();
		} catch (err: unknown) {
			alert(`更新失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		}
	};

	// 額度查詢
	const handleQueryQuota = async () => {
		if (!quotaStoreId.trim()) return;
		setQuotaLoading(true);
		setQuotaData(null);
		try {
			const res = await api.get<QuotaResponse>(
				`/api/v1/admin/ai_quotas/${quotaStoreId}`,
			);
			if (res?.data) {
				setQuotaData(res.data);
				setQuotaLimit(res.data.weeklyLimit);
				setQuotaMemo(res.data.memo);
			}
		} catch (err: unknown) {
			alert(`查詢失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		} finally {
			setQuotaLoading(false);
		}
	};

	const handleSaveQuota = async () => {
		if (!quotaStoreId.trim()) return;
		const limit = Number.parseInt(quotaLimit, 10);
		if (Number.isNaN(limit) || limit < 0 || limit > 1000) {
			alert("額度必須為 0~1000 的整數");
			return;
		}
		setQuotaSaving(true);
		try {
			await api.put(`/api/v1/admin/ai_quotas/${quotaStoreId}`, {
				weeklyLimit: quotaLimit,
				memo: quotaMemo,
			});
			alert("儲存成功");
			handleQueryQuota();
		} catch (err: unknown) {
			alert(`儲存失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		} finally {
			setQuotaSaving(false);
		}
	};

	const handleResetQuota = async () => {
		if (!quotaStoreId.trim()) return;
		if (!confirm("確定要恢復預設額度（10 次/週）？")) return;
		try {
			await api.delete(`/api/v1/admin/ai_quotas/${quotaStoreId}`);
			alert("已恢復預設");
			handleQueryQuota();
		} catch (err: unknown) {
			alert(`操作失敗：${err instanceof Error ? err.message : "未知錯誤"}`);
		}
	};

	return (
		<div style={{ padding: "24px" }}>
			<h1 style={{ fontSize: "24px", fontWeight: 700, marginBottom: "24px" }}>
				<MaterialSymbol name="wand_stars" style={{ marginRight: "8px" }} />
				AI 修圖管理
			</h1>

			{/* Tab 切換 */}
			<div
				style={{
					display: "flex",
					gap: "4px",
					marginBottom: "24px",
					borderBottom: "1px solid #e5e7eb",
				}}
			>
				<button
					type="button"
					onClick={() => setActiveTab("styles")}
					style={{
						padding: "10px 20px",
						border: "none",
						cursor: "pointer",
						fontSize: "14px",
						fontWeight: 600,
						background: "transparent",
						color:
							activeTab === "styles" ? "var(--admin-primary-strong)" : "#666",
						borderBottom:
							activeTab === "styles"
								? "2px solid var(--admin-primary-strong)"
								: "2px solid transparent",
					}}
				>
					風格 Prompt 管理
				</button>
				<button
					type="button"
					onClick={() => setActiveTab("quotas")}
					style={{
						padding: "10px 20px",
						border: "none",
						cursor: "pointer",
						fontSize: "14px",
						fontWeight: 600,
						background: "transparent",
						color:
							activeTab === "quotas" ? "var(--admin-primary-strong)" : "#666",
						borderBottom:
							activeTab === "quotas"
								? "2px solid var(--admin-primary-strong)"
								: "2px solid transparent",
					}}
				>
					賣家額度管理
				</button>
			</div>

			{activeTab === "styles" && (
				<div>
					{/* 分類篩選 */}
					<div style={{ display: "flex", gap: "8px", marginBottom: "16px" }}>
						{(["product", "post"] as CategoryFilter[]).map((cat) => (
							<button
								key={cat}
								type="button"
								onClick={() => setCategoryFilter(cat)}
								style={{
									padding: "6px 16px",
									border: "1px solid #e5e7eb",
									borderRadius: "999px",
									cursor: "pointer",
									fontSize: "13px",
									fontWeight: 600,
									background:
										categoryFilter === cat
											? "var(--admin-primary-strong)"
											: "white",
									color: categoryFilter === cat ? "white" : "#666",
								}}
							>
								{CATEGORY_LABELS[cat]}
							</button>
						))}
					</div>

					{/* 新增按鈕 */}
					<div style={{ marginBottom: "16px" }}>
						<button
							type="button"
							onClick={openCreateForm}
							style={{
								padding: "8px 16px",
								background: "var(--admin-primary-strong)",
								color: "white",
								border: "none",
								borderRadius: "8px",
								cursor: "pointer",
								fontSize: "14px",
							}}
						>
							<MaterialSymbol name="add" style={{ marginRight: "4px" }} />
							新增風格
						</button>
					</div>

					{/* 風格列表 */}
					{loading ? (
						<p style={{ color: "#999" }}>載入中...</p>
					) : (
						<table
							style={{
								width: "100%",
								borderCollapse: "collapse",
								fontSize: "14px",
							}}
						>
							<thead>
								<tr
									style={{
										borderBottom: "2px solid #e5e7eb",
										textAlign: "left",
									}}
								>
									<th style={{ padding: "10px 12px" }}>縮圖</th>
									<th style={{ padding: "10px 12px" }}>排序</th>
									<th style={{ padding: "10px 12px" }}>分類</th>
									<th style={{ padding: "10px 12px" }}>識別名</th>
									<th style={{ padding: "10px 12px" }}>標籤</th>
									<th style={{ padding: "10px 12px", maxWidth: "400px" }}>
										Prompt
									</th>
									<th style={{ padding: "10px 12px" }}>狀態</th>
									<th style={{ padding: "10px 12px" }}>操作</th>
								</tr>
							</thead>
							<tbody>
								{styles.map((style) => (
									<tr
										key={style.id}
										style={{ borderBottom: "1px solid #f0f0f0" }}
									>
										<td style={{ padding: "10px 12px" }}>
											{style.thumbnailUrl ? (
												<Image
													src={
														resolveOssUrl(style.thumbnailUrl) ??
														style.thumbnailUrl
													}
													alt={style.label}
													width={48}
													height={48}
													unoptimized
													style={{
														width: "48px",
														height: "48px",
														objectFit: "cover",
														borderRadius: "6px",
													}}
												/>
											) : (
												<div
													style={{
														width: "48px",
														height: "48px",
														borderRadius: "6px",
														background: "#f3f4f6",
														display: "flex",
														alignItems: "center",
														justifyContent: "center",
														color: "#ccc",
														fontSize: "20px",
													}}
												>
													<MaterialSymbol name="image" />
												</div>
											)}
										</td>
										<td style={{ padding: "10px 12px" }}>{style.sortOrder}</td>
										<td style={{ padding: "10px 12px" }}>
											{CATEGORY_LABELS[style.category]}
										</td>
										<td
											style={{ padding: "10px 12px", fontFamily: "monospace" }}
										>
											{style.name}
											{style.isDefault && (
												<span
													style={{
														marginLeft: "6px",
														padding: "2px 6px",
														background: "var(--color-ink-100)",
														color: "var(--admin-primary-strong)",
														borderRadius: "4px",
														fontSize: "11px",
													}}
												>
													DEFAULT
												</span>
											)}
										</td>
										<td style={{ padding: "10px 12px", fontWeight: 600 }}>
											{style.label}
										</td>
										<td
											style={{
												padding: "10px 12px",
												maxWidth: "400px",
												overflow: "hidden",
												textOverflow: "ellipsis",
												whiteSpace: "nowrap",
												color: "#666",
											}}
										>
											{style.prompt}
										</td>
										<td style={{ padding: "10px 12px" }}>
											<button
												type="button"
												className="btn-reset"
												onClick={() => handleToggleEnabled(style)}
												style={{
													padding: "3px 10px",
													borderRadius: "12px",
													fontSize: "12px",
													cursor: "pointer",
													background: style.enabled
														? "var(--color-ink-100)"
														: "#f3f4f6",
													color: style.enabled
														? "var(--admin-primary-strong)"
														: "#999",
												}}
											>
												{style.enabled ? "啟用" : "停用"}
											</button>
										</td>
										<td style={{ padding: "10px 12px" }}>
											<div style={{ display: "flex", gap: "8px" }}>
												<button
													type="button"
													onClick={() => openEditForm(style)}
													style={{
														padding: "4px 10px",
														border: "1px solid #ddd",
														borderRadius: "6px",
														cursor: "pointer",
														fontSize: "12px",
														background: "white",
													}}
												>
													編輯
												</button>
												{!style.isDefault && (
													<button
														type="button"
														onClick={() => handleDelete(style)}
														style={{
															padding: "4px 10px",
															border: "1px solid #fca5a5",
															borderRadius: "6px",
															cursor: "pointer",
															fontSize: "12px",
															background: "white",
															color: "#E7000B",
														}}
													>
														刪除
													</button>
												)}
											</div>
										</td>
									</tr>
								))}
							</tbody>
						</table>
					)}
				</div>
			)}

			{activeTab === "quotas" && (
				<div>
					{/* 賣家搜尋 */}
					<div
						style={{
							display: "flex",
							gap: "8px",
							marginBottom: "24px",
							alignItems: "center",
						}}
					>
						<input
							type="text"
							value={quotaStoreId}
							onChange={(e) => setQuotaStoreId(e.target.value)}
							placeholder="輸入 Store ID（賣家管理頁面可查）..."
							onKeyDown={(e) => e.key === "Enter" && handleQueryQuota()}
							style={{
								padding: "8px 12px",
								border: "1px solid #ddd",
								borderRadius: "8px",
								fontSize: "14px",
								width: "240px",
							}}
						/>
						<button
							type="button"
							onClick={handleQueryQuota}
							disabled={quotaLoading}
							style={{
								padding: "8px 16px",
								background: "var(--admin-primary-strong)",
								color: "white",
								border: "none",
								borderRadius: "8px",
								cursor: "pointer",
								fontSize: "14px",
							}}
						>
							{quotaLoading ? "查詢中..." : "查詢"}
						</button>
					</div>

					{quotaData && (
						<div
							style={{
								background: "white",
								border: "1px solid #e5e7eb",
								borderRadius: "12px",
								padding: "24px",
								maxWidth: "500px",
							}}
						>
							<h3
								style={{
									fontSize: "16px",
									fontWeight: 600,
									marginBottom: "16px",
								}}
							>
								Store {quotaData.storeId}
							</h3>

							<div
								style={{ display: "grid", gap: "12px", marginBottom: "20px" }}
							>
								<div
									style={{ display: "flex", justifyContent: "space-between" }}
								>
									<span style={{ color: "#666" }}>本週已使用</span>
									<span style={{ fontWeight: 600 }}>
										{quotaData.weeklyUsage} 次
									</span>
								</div>
								<div
									style={{ display: "flex", justifyContent: "space-between" }}
								>
									<span style={{ color: "#666" }}>額度類型</span>
									<span
										style={{
											padding: "2px 8px",
											borderRadius: "8px",
											fontSize: "12px",
											background: quotaData.isCustom
												? "#FEF3C7"
												: "var(--color-ink-100)",
											color: quotaData.isCustom
												? "#D97706"
												: "var(--admin-primary-strong)",
										}}
									>
										{quotaData.isCustom ? "自訂" : "預設"}
									</span>
								</div>
							</div>

							<div style={{ display: "grid", gap: "12px" }}>
								<div>
									<label
										htmlFor="quota-weekly-limit"
										style={{
											display: "block",
											fontSize: "13px",
											color: "#666",
											marginBottom: "4px",
										}}
									>
										每週額度
									</label>
									<input
										id="quota-weekly-limit"
										type="number"
										value={quotaLimit}
										onChange={(e) => setQuotaLimit(e.target.value)}
										min={0}
										max={1000}
										style={{
											width: "100%",
											padding: "8px 12px",
											border: "1px solid #ddd",
											borderRadius: "8px",
											fontSize: "14px",
										}}
									/>
									<span style={{ fontSize: "12px", color: "#999" }}>
										0 = 禁用，預設 10，最高 1000
									</span>
								</div>
								<div>
									<label
										htmlFor="quota-memo"
										style={{
											display: "block",
											fontSize: "13px",
											color: "#666",
											marginBottom: "4px",
										}}
									>
										備註
									</label>
									<input
										id="quota-memo"
										type="text"
										value={quotaMemo}
										onChange={(e) => setQuotaMemo(e.target.value)}
										placeholder="調整原因..."
										style={{
											width: "100%",
											padding: "8px 12px",
											border: "1px solid #ddd",
											borderRadius: "8px",
											fontSize: "14px",
										}}
									/>
								</div>
							</div>

							<div style={{ display: "flex", gap: "8px", marginTop: "20px" }}>
								<button
									type="button"
									onClick={handleSaveQuota}
									disabled={quotaSaving}
									style={{
										padding: "8px 20px",
										background: "var(--admin-primary-strong)",
										color: "white",
										border: "none",
										borderRadius: "8px",
										cursor: "pointer",
										fontSize: "14px",
									}}
								>
									{quotaSaving ? "儲存中..." : "儲存"}
								</button>
								{quotaData.isCustom && (
									<button
										type="button"
										onClick={handleResetQuota}
										style={{
											padding: "8px 20px",
											background: "white",
											color: "#666",
											border: "1px solid #ddd",
											borderRadius: "8px",
											cursor: "pointer",
											fontSize: "14px",
										}}
									>
										恢復預設
									</button>
								)}
							</div>
						</div>
					)}
				</div>
			)}

			{/* 新增/編輯 Modal */}
			{showForm && (
				<Dialog
					busy={saving}
					maxWidth={560}
					title={editingStyle ? `編輯風格：${editingStyle.label}` : "新增風格"}
					onClose={() => setShowForm(false)}
					footer={
						<>
							<button
								type="button"
								onClick={() => setShowForm(false)}
								disabled={saving}
								style={{
									padding: "8px 20px",
									background: "white",
									color: "#666",
									border: "1px solid #ddd",
									borderRadius: "8px",
									cursor: "pointer",
									fontSize: "14px",
								}}
							>
								取消
							</button>
							<button
								type="button"
								onClick={handleSave}
								disabled={saving}
								style={{
									padding: "8px 20px",
									background: saving
										? "var(--color-ink-200)"
										: "var(--admin-primary-strong)",
									color: "white",
									border: "none",
									borderRadius: "8px",
									cursor: saving ? "not-allowed" : "pointer",
									fontSize: "14px",
								}}
							>
								{saving ? "上傳並儲存中..." : "儲存"}
							</button>
						</>
					}
				>
					<div style={{ display: "grid", gap: "16px" }}>
						{!editingStyle && (
							<div>
								<label
									htmlFor="form-name"
									style={{
										display: "block",
										fontSize: "13px",
										color: "#666",
										marginBottom: "4px",
									}}
								>
									識別名 <span style={{ color: "#E7000B" }}>*</span>
								</label>
								<input
									id="form-name"
									type="text"
									value={formName}
									onChange={(e) => setFormName(e.target.value)}
									placeholder="英文小寫+底線，如 tropical"
									style={{
										width: "100%",
										padding: "8px 12px",
										border: "1px solid #ddd",
										borderRadius: "8px",
										fontSize: "14px",
									}}
								/>
							</div>
						)}
						{!editingStyle && (
							<div>
								<label
									htmlFor="form-category"
									style={{
										display: "block",
										fontSize: "13px",
										color: "#666",
										marginBottom: "4px",
									}}
								>
									分類 <span style={{ color: "#E7000B" }}>*</span>
								</label>
								<select
									id="form-category"
									value={formCategory}
									onChange={(e) =>
										setFormCategory(e.target.value as CategoryFilter)
									}
									style={{
										width: "100%",
										padding: "8px 12px",
										border: "1px solid #ddd",
										borderRadius: "8px",
										fontSize: "14px",
									}}
								>
									<option value="product">商品</option>
									<option value="post">貼文</option>
								</select>
							</div>
						)}
						<div>
							<label
								htmlFor="form-label"
								style={{
									display: "block",
									fontSize: "13px",
									color: "#666",
									marginBottom: "4px",
								}}
							>
								標籤 <span style={{ color: "#E7000B" }}>*</span>
							</label>
							<input
								id="form-label"
								type="text"
								value={formLabel}
								onChange={(e) => setFormLabel(e.target.value)}
								placeholder="顯示名稱，如 熱帶風情"
								style={{
									width: "100%",
									padding: "8px 12px",
									border: "1px solid #ddd",
									borderRadius: "8px",
									fontSize: "14px",
								}}
							/>
						</div>
						<div>
							<label
								htmlFor="form-prompt"
								style={{
									display: "block",
									fontSize: "13px",
									color: "#666",
									marginBottom: "4px",
								}}
							>
								Prompt <span style={{ color: "#E7000B" }}>*</span>
							</label>
							<textarea
								id="form-prompt"
								value={formPrompt}
								onChange={(e) => setFormPrompt(e.target.value)}
								placeholder="Gemini prompt 內容..."
								rows={6}
								style={{
									width: "100%",
									padding: "8px 12px",
									border: "1px solid #ddd",
									borderRadius: "8px",
									fontSize: "14px",
									resize: "vertical",
								}}
							/>
							<span style={{ fontSize: "12px", color: "#999" }}>
								{formPrompt.length}/2000
							</span>
						</div>
						{/* 縮圖上傳區塊 */}
						<div>
							<label
								htmlFor="form-thumbnail"
								style={{
									display: "block",
									fontSize: "13px",
									color: "#666",
									marginBottom: "4px",
								}}
							>
								縮圖
							</label>
							<input
								id="form-thumbnail"
								ref={thumbnailInputRef}
								type="file"
								accept="image/png,image/jpeg"
								style={{ display: "none" }}
								onChange={(e) => {
									const file = e.target.files?.[0];
									if (file) handleThumbnailSelect(file);
									e.target.value = "";
								}}
							/>
							{thumbnailPreview ? (
								<div style={{ position: "relative", display: "inline-block" }}>
									<Image
										src={thumbnailPreview}
										alt="縮圖預覽"
										width={120}
										height={120}
										unoptimized
										style={{
											width: "120px",
											height: "120px",
											objectFit: "cover",
											borderRadius: "8px",
											border: "1px solid #e5e7eb",
										}}
									/>
									<button
										type="button"
										onClick={() => thumbnailInputRef.current?.click()}
										style={{
											marginLeft: "12px",
											padding: "4px 10px",
											border: "1px solid #ddd",
											borderRadius: "6px",
											cursor: "pointer",
											fontSize: "12px",
											background: "white",
											verticalAlign: "top",
										}}
									>
										更換圖片
									</button>
								</div>
							) : (
								<button
									type="button"
									className="btn-reset"
									onClick={() => thumbnailInputRef.current?.click()}
									onDragOver={(e) => e.preventDefault()}
									onDrop={handleThumbnailDrop}
									style={{
										display: "block",
										width: "100%",
										border: "2px dashed #d1d5db",
										borderRadius: "8px",
										padding: "24px",
										textAlign: "center",
										cursor: "pointer",
										color: "#9ca3af",
										fontSize: "13px",
									}}
								>
									<MaterialSymbol
										name="cloud_upload"
										style={{
											fontSize: "24px",
											display: "block",
											marginBottom: "6px",
										}}
									/>
									點擊或拖曳上傳縮圖
									<div style={{ fontSize: "12px", marginTop: "4px" }}>
										PNG / JPG，不超過 5MB
									</div>
								</button>
							)}
							{thumbnailError && (
								<div
									style={{
										color: "#E7000B",
										fontSize: "12px",
										marginTop: "4px",
									}}
								>
									{thumbnailError}
								</div>
							)}
						</div>
						<div style={{ display: "flex", gap: "16px" }}>
							<div style={{ flex: 1 }}>
								<label
									htmlFor="form-sort-order"
									style={{
										display: "block",
										fontSize: "13px",
										color: "#666",
										marginBottom: "4px",
									}}
								>
									排序
								</label>
								<input
									id="form-sort-order"
									type="number"
									value={formSortOrder}
									onChange={(e) => setFormSortOrder(e.target.value)}
									min={0}
									style={{
										width: "100%",
										padding: "8px 12px",
										border: "1px solid #ddd",
										borderRadius: "8px",
										fontSize: "14px",
									}}
								/>
							</div>
							<div
								style={{
									flex: 1,
									display: "flex",
									alignItems: "end",
									gap: "8px",
								}}
							>
								<label
									style={{
										display: "flex",
										alignItems: "center",
										gap: "6px",
										cursor: "pointer",
									}}
								>
									<input
										type="checkbox"
										checked={formEnabled}
										onChange={(e) => setFormEnabled(e.target.checked)}
									/>
									<span style={{ fontSize: "14px" }}>啟用</span>
								</label>
							</div>
						</div>
					</div>
				</Dialog>
			)}
		</div>
	);
};

export default AiStyleManagement;
