"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { Button, Dialog, StatusBadge } from "@/components/UI";
import { REPORT_REASON_CODE_LABEL, toOptions } from "@/lib/codemeta";
import { formatDate } from "@/lib/format";
import {
	type AdminPostListItem,
	selectPostThumbKey,
} from "@/lib/posts/adminList";
import { api } from "@/services/api";
import { buildImageUrl } from "@/utils/media";
import "@/styles/pages/PostManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

interface PostListResponse {
	code: number;
	data: {
		records: AdminPostListItem[];
		total: string;
		page: string;
		pageSize: string;
	};
}

type FilterTab = "all" | "published" | "hidden";

const TAB_LABELS: Record<FilterTab, string> = {
	all: "全部",
	published: "上架中",
	hidden: "已下架",
};

function formatTimestamp(ts: number): string {
	if (!ts) return "-";
	// formatDate 自己分得出秒／毫秒，並釘台北（backend#3053 的稽查）。
	return formatDate(ts);
}

function PostStatusBadge({ status }: { status: string }) {
	switch (status) {
		case "published":
			return <StatusBadge variant="success">上架中</StatusBadge>;
		case "hidden":
			return <StatusBadge variant="danger">已下架</StatusBadge>;
		case "deleted":
			return <StatusBadge variant="gray">已刪除</StatusBadge>;
		default:
			return <StatusBadge variant="gray">{status}</StatusBadge>;
	}
}

export default function PostManagementPage() {
	const queryClient = useQueryClient();
	const [currentPage, setCurrentPage] = useState(1);
	const [filterTab, setFilterTab] = useState<FilterTab>("all");

	const [showModal, setShowModal] = useState(false);
	const [moderatingPost, setModeratingPost] =
		useState<AdminPostListItem | null>(null);
	const [moderateAction, setModerateAction] = useState<"HIDE" | "RESTORE">(
		"HIDE",
	);
	const [hideReason, setHideReason] = useState("");
	const [reasonCode, setReasonCode] = useState(""); // 下架分類，空=未選

	const pageSize = 20;

	const { data, isLoading: loading } = useQuery({
		queryKey: ["posts", "list", { page: currentPage, filter: filterTab }],
		queryFn: async () => {
			let url = `/api/v1/admin/posts?page=${currentPage}&page_size=${pageSize}`;
			if (filterTab === "published") url += "&status=published";
			else if (filterTab === "hidden") url += "&status=hidden";

			const res = await api.get<PostListResponse>(url);
			if (res?.code === 0 && res?.data) {
				return {
					posts: res.data.records || [],
					total: Number.parseInt(res.data.total, 10) || 0,
				};
			}
			return { posts: [] as AdminPostListItem[], total: 0 };
		},
	});
	const posts = data?.posts ?? [];
	const totalCount = data?.total ?? 0;

	// 下架 / 還原 → 成功後 invalidate 自動重抓
	const moderateMutation = useMutation({
		mutationFn: (vars: {
			postId: string;
			action: "HIDE" | "RESTORE";
			reason: string;
			reasonCode: string;
		}) =>
			api.patch(`/api/v1/admin/posts/${vars.postId}`, {
				action: vars.action,
				reason: vars.reason,
				reasonCode: vars.reasonCode,
			}),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["posts"] });
		},
	});
	const moderating = moderateMutation.isPending;

	const handleFilterChange = (tab: FilterTab) => {
		setFilterTab(tab);
		setCurrentPage(1);
	};

	const openHideModal = (post: AdminPostListItem) => {
		setModeratingPost(post);
		setModerateAction("HIDE");
		setHideReason("");
		setReasonCode("");
		setShowModal(true);
	};

	const openRestoreModal = (post: AdminPostListItem) => {
		setModeratingPost(post);
		setModerateAction("RESTORE");
		setHideReason("");
		setReasonCode("");
		setShowModal(true);
	};

	const closeModal = () => {
		if (moderating) return;
		setShowModal(false);
		setModeratingPost(null);
	};

	const handleModerate = async () => {
		if (!moderatingPost) return;
		if (moderateAction === "HIDE" && !reasonCode) {
			alert("請選擇下架分類");
			return;
		}
		if (moderateAction === "HIDE" && !hideReason.trim()) {
			alert("請輸入下架原因");
			return;
		}
		try {
			await moderateMutation.mutateAsync({
				postId: moderatingPost.postId,
				action: moderateAction,
				reason: moderateAction === "HIDE" ? hideReason.trim() : "",
				reasonCode: moderateAction === "HIDE" ? reasonCode : "",
			});
			closeModal();
		} catch (error) {
			alert(
				`操作失敗：${error instanceof Error ? error.message : "請稍後再試"}`,
			);
		}
	};

	const totalPages = Math.ceil(totalCount / pageSize);

	return (
		<div className="post-mgmt-page">
			<div className="page-header">
				<div>
					<h1 className="page-title">貼文管理</h1>
					<p className="page-subtitle">
						平台貼文內容審核與管理，共 {totalCount} 則
					</p>
				</div>
			</div>

			<div className="post-mgmt-tabs">
				{(["all", "published", "hidden"] as FilterTab[]).map((tab) => (
					<button
						type="button"
						key={tab}
						className={`post-mgmt-tab${filterTab === tab ? " active" : ""}`}
						onClick={() => handleFilterChange(tab)}
					>
						{TAB_LABELS[tab]}
					</button>
				))}
			</div>

			<div className="table-container">
				<table className="data-table post-mgmt-table">
					<thead>
						<tr>
							<th>貼文 ID</th>
							<th>賣家</th>
							<th>內容</th>
							<th>檢舉數</th>
							<th>最新檢舉原因</th>
							<th>狀態</th>
							<th>下架原因</th>
							<th>建立時間</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{loading ? (
							<tr>
								<td
									colSpan={9}
									style={{
										textAlign: "center",
										padding: "40px",
										color: "var(--gray-500)",
									}}
								>
									載入中...
								</td>
							</tr>
						) : posts.length === 0 ? (
							<tr>
								<td
									colSpan={9}
									style={{
										textAlign: "center",
										padding: "40px",
										color: "var(--gray-400)",
									}}
								>
									暫無資料
								</td>
							</tr>
						) : (
							posts.map((post) => {
								const thumbKey = selectPostThumbKey(post);
								return (
									<tr key={post.postId}>
										<td className="post-id-cell">#{post.postId}</td>
										<td>
											<div className="post-seller-info">
												<span className="post-seller-name">
													{post.sellerName}
												</span>
												<span className="post-seller-id">
													ID: {post.storeId}
												</span>
											</div>
										</td>
										<td>
											<div className="post-content-cell">
												{thumbKey && (
													<Image
														src={buildImageUrl(thumbKey, "thumb")}
														alt=""
														className="post-thumb"
														width={40}
														height={40}
														unoptimized
													/>
												)}
												<span className="post-content-text">
													{post.content}
												</span>
											</div>
										</td>
										<td>
											{post.reportCount > 0 ? (
												<Link
													href="/admin/reports?target_type=POST"
													className="report-count-badge"
													title="查看內容審核佇列"
												>
													{post.reportCount}
												</Link>
											) : (
												<span className="text-gray">0</span>
											)}
										</td>
										<td
											className="report-reason-cell"
											title={post.latestReportReason}
										>
											{post.latestReportReason || "-"}
										</td>
										<td>
											<PostStatusBadge status={post.status} />
										</td>
										<td
											className="hidden-reason-cell"
											title={post.hiddenReason}
										>
											{post.hiddenReason || "-"}
										</td>
										<td>{formatTimestamp(post.createdAt)}</td>
										<td>
											{post.status !== "deleted" &&
												(post.status === "hidden" ? (
													<Button
														size="sm"
														variant="outline"
														onClick={() => openRestoreModal(post)}
													>
														恢復
													</Button>
												) : (
													<Button
														size="sm"
														variant="danger"
														onClick={() => openHideModal(post)}
													>
														下架
													</Button>
												))}
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>

			{totalPages > 1 && (
				<div className="post-mgmt-pagination">
					<button
						type="button"
						className="btn"
						disabled={currentPage === 1}
						onClick={() => setCurrentPage((p) => p - 1)}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span className="page-info">
						第 {currentPage} / {totalPages} 頁
					</span>
					<button
						type="button"
						className="btn"
						disabled={currentPage >= totalPages}
						onClick={() => setCurrentPage((p) => p + 1)}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}

			{showModal && moderatingPost && (
				<Dialog
					title={
						<>
							<MaterialSymbol
								name={
									moderateAction === "HIDE" ? "visibility_off" : "visibility"
								}
								style={{ marginRight: 8 }}
							/>
							{moderateAction === "HIDE" ? "下架貼文" : "恢復貼文"}
						</>
					}
					className="post-moderate-modal"
					onClose={closeModal}
					footer={
						<>
							<Button variant="outline" onClick={closeModal}>
								取消
							</Button>
							<Button
								variant={moderateAction === "HIDE" ? "danger" : "primary"}
								loading={moderating}
								onClick={handleModerate}
							>
								{moderateAction === "HIDE" ? "確認下架" : "確認恢復"}
							</Button>
						</>
					}
				>
					<div className="moderate-post-preview">
						<div className="moderate-info-row">
							<span className="moderate-label">貼文 ID</span>
							<span>#{moderatingPost.postId}</span>
						</div>
						<div className="moderate-info-row">
							<span className="moderate-label">賣家</span>
							<span>{moderatingPost.sellerName}</span>
						</div>
						<div className="moderate-info-row">
							<span className="moderate-label">內容</span>
							<span className="moderate-content">{moderatingPost.content}</span>
						</div>
						{moderatingPost.reportCount > 0 && (
							<div className="moderate-info-row">
								<span className="moderate-label">檢舉次數</span>
								<span className="report-count-badge">
									{moderatingPost.reportCount}
								</span>
							</div>
						)}
					</div>

					{moderateAction === "HIDE" ? (
						<div className="moderate-reason-section">
							<label
								className="moderate-reason-label"
								htmlFor="hide-reason-code"
							>
								下架分類 <span className="required">*</span>
							</label>
							<select
								id="hide-reason-code"
								className="moderate-reason-input"
								value={reasonCode}
								onChange={(e) => setReasonCode(e.target.value)}
							>
								<option value="" disabled>
									請選擇分類
								</option>
								{toOptions(REPORT_REASON_CODE_LABEL).map(([code, label]) => (
									<option key={code} value={code}>
										{label}
									</option>
								))}
							</select>
							<label
								className="moderate-reason-label"
								htmlFor="hide-reason-input"
							>
								下架原因 <span className="required">*</span>
							</label>
							<textarea
								id="hide-reason-input"
								className="moderate-reason-input"
								rows={3}
								placeholder="請說明下架原因..."
								value={hideReason}
								onChange={(e) => setHideReason(e.target.value)}
							/>
						</div>
					) : (
						<div className="moderate-restore-tip">
							<MaterialSymbol name="info" />
							確認後此貼文將恢復上架，可再次被使用者瀏覽。
						</div>
					)}
				</Dialog>
			)}
		</div>
	);
}
