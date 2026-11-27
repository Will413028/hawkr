"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useMutation, useQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import {
	type AdminPost,
	SocialAdminService,
} from "@hawkr/api/gen/hawkr/admin/social/v1alpha1/social_pb";
import { PostStatus } from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import Link from "next/link";
import { useState } from "react";
import { Button, StatusBadge } from "@/components/UI";
import { useToast } from "@/components/UI/Toast";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { unknownText } from "@/lib/ui-text";
import { imageUri } from "@/utils/image";
import ContentTakedownDialog from "./ContentTakedownDialog";
import {
	isTakedownAlreadyActive,
	listPostsRequest,
	takedownMeasureOf,
} from "./post-actions";
import RestorePostDialog from "./RestorePostDialog";
import "@/styles/pages/PostManagement.css";

type Tab = "all" | "takenDown";

const TAB_LABELS: Record<Tab, string> = {
	all: "全部",
	takenDown: "已下架",
};

const STATUS_TEXT: Record<
	Exclude<PostStatus, PostStatus.UNSPECIFIED>,
	string
> = {
	[PostStatus.DRAFT]: "草稿",
	[PostStatus.PUBLISHED]: "已發布",
	[PostStatus.UNPUBLISHED]: "已取消發布",
};

type DialogState =
	| { name: "takedown"; post: AdminPost }
	| { name: "restore"; measureId: string }
	| null;

/**
 * 貼文管理：keyset 的上一頁與下一頁，篩選是全部或只看被下架的。「下架」建立以這則貼文為對象的內容下架措施，
 * 「恢復」撤銷下架它的措施（社群模組沒有平台隱藏貼文，所以沒有第二個恢復入口）。
 */
export default function PostManagementPage() {
	const queryClient = useQueryClient();
	const { showToast } = useToast();
	const [tab, setTab] = useState<Tab>("all");
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [dialog, setDialog] = useState<DialogState>(null);
	const posts = useQuery(
		SocialAdminService.method.listPosts,
		listPostsRequest(tab === "takenDown", currentToken(pageTokens)),
	);
	const createMeasure = useMutation(
		SellerGovernanceAdminService.method.createMeasure,
	);
	const rows = posts.data?.posts ?? [];
	const next = posts.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const changeTab = (value: Tab) => {
		setTab(value);
		setPageTokens(firstPage());
	};
	const refresh = () => void queryClient.invalidateQueries();

	const renderState = () => {
		if (posts.isLoading) return <p role="status">載入中...</p>;
		if (posts.error) {
			return (
				<div role="alert">
					<MaterialSymbol name="error" />
					<p>{connectErrorText(posts.error, "貼文讀取失敗")}</p>
					<Button variant="outline" onClick={() => void posts.refetch()}>
						重試
					</Button>
				</div>
			);
		}
		return <p>沒有符合條件的貼文</p>;
	};

	return (
		<div className="post-mgmt-page">
			<div className="page-header">
				<div>
					<h1 className="page-title">貼文管理</h1>
					<p className="page-subtitle">
						店家貼文的審核與下架；下架是內容下架措施，恢復是撤銷措施。
					</p>
				</div>
			</div>

			<div className="post-mgmt-tabs">
				{(Object.keys(TAB_LABELS) as Tab[]).map((value) => (
					<button
						type="button"
						key={value}
						className={`post-mgmt-tab${tab === value ? " active" : ""}`}
						onClick={() => changeTab(value)}
					>
						{TAB_LABELS[value]}
					</button>
				))}
			</div>

			<div className="table-container">
				<table className="data-table post-mgmt-table">
					<thead>
						<tr>
							<th>貼文 ID</th>
							<th>店家</th>
							<th>內容</th>
							<th>未結案檢舉</th>
							<th>狀態</th>
							<th>建立時間</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{posts.isLoading || posts.error || rows.length === 0 ? (
							<tr>
								<td
									colSpan={7}
									style={{ textAlign: "center", padding: "40px" }}
								>
									{renderState()}
								</td>
							</tr>
						) : (
							rows.map((post) => {
								const measureId = takedownMeasureOf(post);
								const thumb = imageUri(post.image, "thumb");
								return (
									<tr key={post.id}>
										<td className="post-id-cell">#{post.id}</td>
										<td>
											<Link href={`/admin/merchants/${post.storeId}`}>
												{post.storeId}
											</Link>
										</td>
										<td>
											<div className="post-content-cell">
												{thumb && (
													<Image
														src={thumb}
														alt=""
														className="post-thumb"
														width={40}
														height={40}
														unoptimized
													/>
												)}
												<span className="post-content-text">{post.body}</span>
											</div>
										</td>
										<td>
											{post.openReportCount > BigInt(0) ? (
												<Link
													href="/admin/reports"
													className="report-count-badge"
													title="查看社群檢舉案件"
												>
													{String(post.openReportCount)}
												</Link>
											) : (
												<span className="text-gray">0</span>
											)}
										</td>
										<td>
											{measureId ? (
												<StatusBadge variant="danger">已下架</StatusBadge>
											) : (
												<StatusBadge variant="gray">
													{STATUS_TEXT[
														post.status as keyof typeof STATUS_TEXT
													] ?? unknownText()}
												</StatusBadge>
											)}
										</td>
										<td>
											{post.createTime
												? formatDateTimeShort(timestampDate(post.createTime))
												: "-"}
										</td>
										<td>
											{measureId ? (
												<Button
													size="sm"
													variant="outline"
													onClick={() =>
														setDialog({ name: "restore", measureId })
													}
												>
													恢復
												</Button>
											) : (
												<Button
													size="sm"
													variant="danger"
													onClick={() => setDialog({ name: "takedown", post })}
												>
													下架
												</Button>
											)}
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>

			{(page > 1 || next) && (
				<div className="post-mgmt-pagination">
					<button
						type="button"
						className="btn"
						disabled={page <= 1}
						onClick={() => setPageTokens(previousPage)}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span className="page-info">第 {page} 頁</span>
					<button
						type="button"
						className="btn"
						disabled={!next}
						onClick={() => setPageTokens((t) => nextPage(t, next))}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}

			{dialog?.name === "takedown" && (
				<ContentTakedownDialog
					post={dialog.post}
					onClose={() => setDialog(null)}
					onConfirm={async (request) => {
						try {
							await createMeasure.mutateAsync(request);
						} catch (caught) {
							if (isTakedownAlreadyActive(caught)) refresh();
							throw caught;
						}
						setDialog(null);
						showToast("已下架貼文", "success");
						refresh();
					}}
				/>
			)}
			{dialog?.name === "restore" && (
				<RestorePostDialog
					measureId={dialog.measureId}
					onClose={() => setDialog(null)}
					onRestored={() => {
						setDialog(null);
						showToast("已恢復貼文", "success");
						refresh();
					}}
				/>
			)}
		</div>
	);
}
