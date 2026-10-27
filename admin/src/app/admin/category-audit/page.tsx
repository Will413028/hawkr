"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type React from "react";
import { useMemo, useState } from "react";
import { api } from "@/services/api";
import type { CategoryNode } from "@/types";
import "@/styles/pages/CategoryAudit.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/** 狀態碼 */
const STATUS_PENDING = "9002";
const STATUS_INACTIVE = "9003";
const STATUS_ACTIVE = "9004";

interface TreeResponse {
	code: number;
	data: CategoryNode[];
}

/** 將三層樹狀結構扁平化為第三層分類列表，附上父路徑 */
interface FlatCategory {
	id: string;
	name: string;
	parentPath: string; // e.g. "蔬果 > 葉菜類"
	status: string;
}

function flattenTree(nodes: CategoryNode[]): FlatCategory[] {
	const result: FlatCategory[] = [];
	for (const lv1 of nodes) {
		for (const lv2 of lv1.children || []) {
			for (const lv3 of lv2.children || []) {
				result.push({
					id: lv3.id,
					name: lv3.name,
					parentPath: `${lv1.name} > ${lv2.name}`,
					status: lv3.status,
				});
			}
		}
	}
	return result;
}

const PAGE_SIZE = 20;

const CategoryAudit: React.FC = () => {
	const queryClient = useQueryClient();
	const [keyword, setKeyword] = useState("");
	const [searchInput, setSearchInput] = useState("");
	const [currentPage, setCurrentPage] = useState(1);
	const [statusFilter, setStatusFilter] = useState<string>(STATUS_PENDING);

	// 批次選取
	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

	// 整棵分類樹一次抓，client 端 filter / 分頁 / 統計
	const { data: allCategories = [], isLoading: loading } = useQuery({
		queryKey: ["categories", "tree"],
		queryFn: async () => {
			const res = await api.get<TreeResponse>("/api/v1/admin/categories/tree");
			return res?.code === 0 ? flattenTree(res.data) : [];
		},
	});

	// 審核（批次 / 單筆同一個 endpoint）→ 成功後 invalidate 自動重抓
	const updateMutation = useMutation({
		mutationFn: (vars: { categoryIds: string[]; status: string }) =>
			api.patch("/api/v1/admin/categories", vars),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["categories"] });
		},
	});
	const submitting = updateMutation.isPending;

	// 篩選 + 搜尋
	const filtered = useMemo(() => {
		return allCategories.filter((c) => {
			if (statusFilter && c.status !== statusFilter) return false;
			if (keyword) {
				const kw = keyword.toLowerCase();
				return (
					c.name.toLowerCase().includes(kw) ||
					c.parentPath.toLowerCase().includes(kw)
				);
			}
			return true;
		});
	}, [allCategories, statusFilter, keyword]);

	// 統計
	const stats = useMemo(() => {
		let pending = 0,
			active = 0,
			inactive = 0;
		for (const c of allCategories) {
			if (c.status === STATUS_PENDING) pending++;
			else if (c.status === STATUS_ACTIVE) active++;
			else if (c.status === STATUS_INACTIVE) inactive++;
		}
		return { pending, active, inactive };
	}, [allCategories]);

	// 分頁
	const totalPages = Math.ceil(filtered.length / PAGE_SIZE);
	const pageItems = filtered.slice(
		(currentPage - 1) * PAGE_SIZE,
		currentPage * PAGE_SIZE,
	);

	// 切換篩選時重置頁碼和選取
	const changeFilter = (s: string) => {
		setStatusFilter(s);
		setCurrentPage(1);
		setSelectedIds(new Set());
	};

	const handleSearch = (e: React.FormEvent) => {
		e.preventDefault();
		setCurrentPage(1);
		setKeyword(searchInput.trim());
	};

	// 勾選
	const toggleSelect = (id: string) => {
		setSelectedIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	};
	const toggleAll = () => {
		if (selectedIds.size === pageItems.length) {
			setSelectedIds(new Set());
		} else {
			setSelectedIds(new Set(pageItems.map((c) => c.id)));
		}
	};

	// 批次審核
	const batchUpdate = async (status: string) => {
		if (selectedIds.size === 0) return;
		await updateMutation.mutateAsync({
			categoryIds: Array.from(selectedIds),
			status,
		});
		setSelectedIds(new Set());
	};

	// 單筆快速審核
	const singleUpdate = async (id: string, status: string) => {
		await updateMutation.mutateAsync({ categoryIds: [id], status });
	};

	const statusLabel = (s: string) => {
		if (s === STATUS_PENDING)
			return <span className="cat-status-pending">待審核</span>;
		if (s === STATUS_ACTIVE)
			return <span className="cat-status-approved">已啟用</span>;
		if (s === STATUS_INACTIVE)
			return <span style={{ color: "#9ca3af", fontSize: 14 }}>已停用</span>;
		return <span>{s}</span>;
	};

	return (
		<div className="category-audit">
			<div className="page-header">
				<div>
					<div className="page-title">分類審核</div>
					<div className="page-subtitle">審核賣家提交的新增商品分類申請</div>
				</div>
			</div>

			{/* 統計卡片 */}
			<div className="category-stats-row">
				<button
					type="button"
					className={`btn-reset merchant-stat-card${statusFilter === STATUS_PENDING ? " cat-stat-active" : ""}`}
					style={{ flex: 1, cursor: "pointer" }}
					onClick={() => changeFilter(STATUS_PENDING)}
				>
					<div
						style={{
							fontSize: 14,
							fontWeight: 700,
							color: "#565656",
							marginBottom: 8,
						}}
					>
						待審核
					</div>
					<div style={{ fontSize: 32, fontWeight: 700, color: "#e7000b" }}>
						{stats.pending}
					</div>
				</button>
				<button
					type="button"
					className={`btn-reset merchant-stat-card${statusFilter === STATUS_ACTIVE ? " cat-stat-active" : ""}`}
					style={{ flex: 1, cursor: "pointer" }}
					onClick={() => changeFilter(STATUS_ACTIVE)}
				>
					<div
						style={{
							fontSize: 14,
							fontWeight: 700,
							color: "#565656",
							marginBottom: 8,
						}}
					>
						已啟用
					</div>
					<div
						style={{
							fontSize: 32,
							fontWeight: 700,
							color: "var(--admin-primary-strong)",
						}}
					>
						{stats.active}
					</div>
				</button>
				<button
					type="button"
					className={`btn-reset merchant-stat-card${statusFilter === STATUS_INACTIVE ? " cat-stat-active" : ""}`}
					style={{ flex: 1, cursor: "pointer" }}
					onClick={() => changeFilter(STATUS_INACTIVE)}
				>
					<div
						style={{
							fontSize: 14,
							fontWeight: 700,
							color: "#565656",
							marginBottom: 8,
						}}
					>
						已停用
					</div>
					<div style={{ fontSize: 32, fontWeight: 700, color: "#565656" }}>
						{stats.inactive}
					</div>
				</button>
				<button
					type="button"
					className={`btn-reset merchant-stat-card${statusFilter === "" ? " cat-stat-active" : ""}`}
					style={{ flex: 1, cursor: "pointer" }}
					onClick={() => changeFilter("")}
				>
					<div
						style={{
							fontSize: 14,
							fontWeight: 700,
							color: "#565656",
							marginBottom: 8,
						}}
					>
						全部
					</div>
					<div style={{ fontSize: 32, fontWeight: 700, color: "#565656" }}>
						{allCategories.length}
					</div>
				</button>
			</div>

			{/* 搜尋列 + 批次操作 */}
			<div
				style={{
					display: "flex",
					gap: 12,
					alignItems: "center",
					marginBottom: 16,
				}}
			>
				<form
					className="category-search-bar"
					onSubmit={handleSearch}
					style={{ flex: 1, marginBottom: 0 }}
				>
					<MaterialSymbol name="search" />
					<input
						type="text"
						placeholder="搜尋分類名稱..."
						value={searchInput}
						onChange={(e) => setSearchInput(e.target.value)}
					/>
				</form>
				{selectedIds.size > 0 && statusFilter === STATUS_PENDING && (
					<div style={{ display: "flex", gap: 8 }}>
						<button
							type="button"
							className="btn-cat-approve"
							disabled={submitting}
							onClick={() => batchUpdate(STATUS_ACTIVE)}
						>
							<MaterialSymbol name="check_circle" />
							批次通過 ({selectedIds.size})
						</button>
						<button
							type="button"
							className="btn-cat-reject"
							disabled={submitting}
							onClick={() => batchUpdate(STATUS_INACTIVE)}
						>
							<MaterialSymbol name="close" />
							批次駁回 ({selectedIds.size})
						</button>
					</div>
				)}
			</div>

			{/* 表格 */}
			<div className="category-table-container">
				<div style={{ overflowX: "auto" }}>
					<table>
						<thead>
							<tr>
								{statusFilter === STATUS_PENDING && (
									<th style={{ width: 40 }}>
										<input
											type="checkbox"
											checked={
												pageItems.length > 0 &&
												selectedIds.size === pageItems.length
											}
											onChange={toggleAll}
										/>
									</th>
								)}
								<th>分類 ID</th>
								<th>第一 / 第二分類</th>
								<th>第三分類名稱</th>
								<th>狀態</th>
								<th>操作</th>
							</tr>
						</thead>
						<tbody>
							{loading ? (
								<tr>
									<td
										colSpan={statusFilter === STATUS_PENDING ? 6 : 5}
										className="category-empty"
									>
										<MaterialSymbol
											name="progress_activity"
											style={{ fontSize: 24, marginRight: 8 }}
										/>
										載入中...
									</td>
								</tr>
							) : pageItems.length === 0 ? (
								<tr>
									<td
										colSpan={statusFilter === STATUS_PENDING ? 6 : 5}
										className="category-empty"
									>
										<MaterialSymbol
											name="device_hub"
											style={{
												fontSize: 36,
												display: "block",
												marginBottom: 8,
												opacity: 0.4,
											}}
										/>
										<div>無符合條件的分類</div>
									</td>
								</tr>
							) : (
								pageItems.map((cat) => (
									<tr key={cat.id}>
										{statusFilter === STATUS_PENDING && (
											<td>
												<input
													type="checkbox"
													checked={selectedIds.has(cat.id)}
													onChange={() => toggleSelect(cat.id)}
												/>
											</td>
										)}
										<td>
											<div className="cat-app-no">{cat.id}</div>
										</td>
										<td>
											<span className="cat-path-badge">{cat.parentPath}</span>
										</td>
										<td>
											<span className="cat-name-cell">{cat.name}</span>
										</td>
										<td>{statusLabel(cat.status)}</td>
										<td>
											<div className="cat-action-cell">
												{cat.status === STATUS_PENDING && (
													<>
														<button
															type="button"
															className="btn-cat-reject"
															disabled={submitting}
															onClick={() =>
																singleUpdate(cat.id, STATUS_INACTIVE)
															}
														>
															<MaterialSymbol name="close" />
															駁回
														</button>
														<button
															type="button"
															className="btn-cat-approve"
															disabled={submitting}
															onClick={() =>
																singleUpdate(cat.id, STATUS_ACTIVE)
															}
														>
															<MaterialSymbol name="check_circle" />
															通過
														</button>
													</>
												)}
												{cat.status === STATUS_ACTIVE && (
													<button
														type="button"
														className="btn-cat-reject"
														disabled={submitting}
														onClick={() =>
															singleUpdate(cat.id, STATUS_INACTIVE)
														}
													>
														<MaterialSymbol name="close" />
														停用
													</button>
												)}
												{cat.status === STATUS_INACTIVE && (
													<button
														type="button"
														className="btn-cat-approve"
														disabled={submitting}
														onClick={() => singleUpdate(cat.id, STATUS_ACTIVE)}
													>
														<MaterialSymbol name="check_circle" />
														啟用
													</button>
												)}
											</div>
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</div>

				{/* 分頁 */}
				{totalPages > 1 && (
					<div className="category-pagination">
						<button
							type="button"
							disabled={currentPage <= 1}
							onClick={() => setCurrentPage((p) => p - 1)}
						>
							<MaterialSymbol name="keyboard_arrow_left" />
						</button>
						{Array.from({ length: totalPages }, (_, i) => i + 1)
							.filter((p) => Math.abs(p - currentPage) <= 2)
							.map((p) => (
								<button
									type="button"
									key={p}
									className={p === currentPage ? "active" : ""}
									onClick={() => setCurrentPage(p)}
								>
									{p}
								</button>
							))}
						<button
							type="button"
							disabled={currentPage >= totalPages}
							onClick={() => setCurrentPage((p) => p + 1)}
						>
							<MaterialSymbol name="keyboard_arrow_right" />
						</button>
					</div>
				)}
			</div>
		</div>
	);
};

export default CategoryAudit;
