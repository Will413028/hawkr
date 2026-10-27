"use client";

import { createClient } from "@connectrpc/connect";
import {
	createConnectQueryKey,
	useInfiniteQuery,
	useQuery,
	useTransport,
} from "@connectrpc/connect-query";
import {
	type Banner as AdminBanner,
	BannerSchema,
	ContentAdminService,
} from "@hawkr/api/gen/hawkr/admin/content/v1alpha1/content_pb";
import type { Image as ApiImage } from "@hawkr/api/gen/hawkr/common/v1alpha1/image_pb";
import { ContentPublicService } from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import { useRouter } from "next/navigation";
import type React from "react";
import { useMemo } from "react";
import { actionFromProto } from "@/components/banner/action";
import { endDateOf, startDateOf } from "@/components/banner/schedule";
import { errorMessage } from "@/lib/error-text";
import { imageUri } from "@/utils/image";

const BannerList: React.FC = () => {
	const router = useRouter();
	const queryClient = useQueryClient();
	const transport = useTransport();
	const client = useMemo(
		() => createClient(ContentAdminService, transport),
		[transport],
	);

	// 依顯示順序分頁，「載入更多」讀下一頁。
	const {
		data,
		isLoading: loading,
		error: queryError,
		refetch,
		hasNextPage,
		isFetchingNextPage,
		fetchNextPage,
	} = useInfiniteQuery(
		ContentAdminService.method.listBanners,
		{ pageSize: 50, pageToken: "" },
		{
			pageParamKey: "pageToken",
			getNextPageParam: (res) => res.nextPageToken || undefined,
		},
	);
	const banners = useMemo(
		() => data?.pages.flatMap((page) => page.banners) ?? [],
		[data],
	);
	const error = queryError ? errorMessage(queryError) : null;

	// 預覽：此刻在 App 首頁輪播的 banner（前台的 ListLiveBanners，啟用中而且在排程期間內；最多 20 則，一頁就是
	// 全部）。不從上面的清單篩：清單分頁，啟用中的可能還沒載入。
	const { data: live } = useQuery(
		ContentPublicService.method.listLiveBanners,
		{},
	);
	const liveBanners = live?.banners ?? [];

	const invalidateBanners = () =>
		Promise.all([
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: ContentAdminService.method.listBanners,
					cardinality: "infinite",
				}),
			}),
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: ContentPublicService.method.listLiveBanners,
					cardinality: "finite",
				}),
			}),
		]);

	// 切換啟用狀態：只改 active，帶著清單讀到的 etag；別人先改過就回版本衝突、不覆蓋。
	const toggleMutation = useMutation({
		mutationFn: (vars: { banner: AdminBanner }) =>
			client.updateBanner({
				banner: {
					id: vars.banner.id,
					etag: vars.banner.etag,
					active: !vars.banner.active,
				},
				updateMask: fieldMask(BannerSchema, ["active"]),
			}),
		onSettled: invalidateBanners,
	});

	const deleteMutation = useMutation({
		mutationFn: (banner: AdminBanner) =>
			client.deleteBanner({ id: banner.id, etag: banner.etag }),
		onSettled: invalidateBanners,
	});

	const handleToggle = (banner: AdminBanner) => {
		toggleMutation.mutate(
			{ banner },
			{
				onError: (e) => {
					console.error("Failed to toggle banner:", e);
					alert(errorMessage(e));
				},
			},
		);
	};

	const handleDelete = (banner: AdminBanner) => {
		if (!confirm(`確定要刪除「${banner.title}」嗎？`)) return;
		deleteMutation.mutate(banner, {
			onError: (e) => {
				console.error("Failed to delete banner:", e);
				alert(errorMessage(e));
			},
		});
	};

	const getImageUrl = (banner: { previewImage?: ApiImage }): string | null =>
		imageUri(banner.previewImage, "banner") ?? null;

	// Loading skeleton
	if (loading) {
		return (
			<div className="space-y-6 font-['Roboto','Noto_Sans_JP',sans-serif]">
				<div className="animate-pulse space-y-6">
					<div className="flex justify-between items-center">
						<div>
							<div className="h-9 bg-[var(--color-11)] rounded w-40 mb-2"></div>
							<div className="h-5 bg-[var(--color-11)]/50 rounded w-80"></div>
						</div>
						<div className="h-12 bg-[var(--color-11)] rounded-lg w-36"></div>
					</div>
					<div className="h-64 bg-[var(--color-11)]/30 rounded-[14px]"></div>
					<div className="h-96 bg-[var(--color-11)]/30 rounded-[15px]"></div>
				</div>
			</div>
		);
	}

	// Error state
	if (error) {
		return (
			<div className="space-y-6 font-['Roboto','Noto_Sans_JP',sans-serif]">
				<div className="bg-[var(--color-10)]/10 border border-[var(--color-10)]/20 rounded-[14px] p-6">
					<p className="font-bold text-[18px] text-[var(--color-10)]">
						{error}
					</p>
					<button
						type="button"
						onClick={() => refetch()}
						className="mt-3 text-[14px] text-[var(--color-10)] underline hover:no-underline"
					>
						重試
					</button>
				</div>
			</div>
		);
	}

	return (
		<div className="flex flex-col gap-[16px] font-['Roboto','Noto_Sans_JP',sans-serif]">
			{/* Page Header */}
			<div className="flex items-center justify-between mb-[8px]">
				<div>
					<h2 className="text-[28px] font-bold text-black leading-[36px]">
						上傳Banner
					</h2>
					<p
						className="text-[18px] text-[var(--brand-gray1-100)] mt-[8px]"
						style={{ letterSpacing: "0.77px" }}
					>
						管理 App 首頁頂部的輪播廣告圖片與連結
					</p>
				</div>
				<button
					type="button"
					onClick={() => router.push("/admin/banners/editor")}
					className="inline-flex items-center gap-[6px] px-[12px] py-[12px] bg-[var(--color)] hover:opacity-90 text-white rounded-[8px] text-[18px] font-bold transition-opacity"
				>
					{/* Plus icon */}
					<svg
						aria-hidden="true"
						className="w-[24px] h-[24px]"
						viewBox="0 0 24 24"
						fill="none"
					>
						<path
							d="M12 5V19M5 12H19"
							stroke="currentColor"
							strokeWidth="2.5"
							strokeLinecap="round"
							strokeLinejoin="round"
						/>
					</svg>
					上傳新Banner
				</button>
			</div>

			{/* Preview Section - Figma Design */}
			<div
				className="rounded-[14px] border border-[var(--border-color)] shadow-[0px_4px_4px_0px_rgba(0,0,0,0.1)] p-[20px] flex flex-col gap-[16px]"
				style={{
					background: "linear-gradient(to right, var(--bgapp), var(--white))",
				}}
			>
				{/* Card Header */}
				<div className="flex flex-col gap-[8px]">
					<div className="flex items-center gap-[8px]">
						{/* Grid icon */}
						<svg
							aria-hidden="true"
							className="w-[20px] h-[20px] text-[var(--brand-primary-gray1)]"
							viewBox="0 0 20 20"
							fill="none"
						>
							<rect
								x="2.5"
								y="2.5"
								width="15"
								height="15"
								rx="2"
								stroke="currentColor"
								strokeWidth="1.5"
							/>
							<path
								d="M6 8.5H9M11 8.5H14M6 6H9M11 6H14"
								stroke="currentColor"
								strokeWidth="1.5"
								strokeLinecap="round"
							/>
							<path
								d="M6 11.5H14M6 14H11"
								stroke="currentColor"
								strokeWidth="1.5"
								strokeLinecap="round"
							/>
						</svg>
						<span className="text-[18px] font-bold text-[var(--text)] leading-[1.23]">
							預覽目前Banner
						</span>
					</div>
					<p className="text-[16px] text-[var(--brand-primary-gray1)] leading-[24px]">
						此刻在 App 首頁輪播的 Banner（啟用中，而且在排程期間內）
					</p>
				</div>

				{/* Banner Preview - horizontally scrollable */}
				<div className="flex gap-[16px] pb-[4px] overflow-x-auto">
					{liveBanners.length > 0
						? liveBanners.map((banner, index) => {
								const imgUrl = getImageUrl(banner);
								const action = actionFromProto(banner.action);
								return (
									<div
										key={banner.id}
										className="relative rounded-[15px] overflow-hidden cursor-pointer group flex-shrink-0"
										style={{
											width: "400px",
											height: "200px",
											animation: `bannerFadeIn 0.6s ease-out ${index * 0.15}s both`,
										}}
									>
										{/* Background Image */}
										{imgUrl ? (
											<Image
												src={imgUrl}
												alt={banner.title}
												width={400}
												height={200}
												unoptimized
												className="absolute inset-0 w-full h-full object-cover transition-transform duration-500 group-hover:scale-105"
											/>
										) : (
											<div
												className="absolute inset-0"
												style={{
													background:
														"linear-gradient(135deg, var(--color), var(--c2))",
												}}
											/>
										)}

										{/* Gradient Overlay - from bottom */}
										<div
											className="absolute inset-0"
											style={{
												background:
													"linear-gradient(to top, rgba(0,0,0,0.6) 0%, transparent 44.5%)",
											}}
										/>

										{/* Text Content - positioned at bottom */}
										<div className="absolute left-[25px] bottom-[16px] right-[16px]">
											{/* Title */}
											<p className="font-bold text-[18px] text-white leading-[28px] tracking-[-0.44px] mb-[0px]">
												{banner.title}
											</p>
											{/* Action label */}
											{action.actionType !== "popup" && (
												<div className="flex items-center gap-[4px] mt-[0px]">
													<svg
														aria-hidden="true"
														className="w-[12px] h-[12px] text-white/80"
														viewBox="0 0 16 16"
														fill="none"
													>
														<path
															d="M6.667 8.667a3.333 3.333 0 005.027.353l2-2a3.333 3.333 0 00-4.714-4.713L7.82 3.467"
															stroke="currentColor"
															strokeWidth="1.5"
															strokeLinecap="round"
															strokeLinejoin="round"
														/>
														<path
															d="M9.333 7.333a3.333 3.333 0 00-5.026-.353l-2 2a3.333 3.333 0 004.713 4.713l1.16-1.16"
															stroke="currentColor"
															strokeWidth="1.5"
															strokeLinecap="round"
															strokeLinejoin="round"
														/>
													</svg>
													<span className="text-[14px] text-white/80 leading-[20px] tracking-[-0.15px]">
														{action.actionType === "url"
															? action.actionUrl
															: `${action.actionType === "product" ? "商品" : "貼文"} #${action.actionId}`}
													</span>
												</div>
											)}
										</div>
									</div>
								);
							})
						: // Empty state - 3 placeholder cards
							[1, 2, 3].map((index) => (
								<div
									key={index}
									className="relative rounded-[15px] overflow-hidden bg-[#f5f5f5] border border-dashed border-[var(--border-color)] flex flex-col items-center justify-center flex-shrink-0"
									style={{
										width: "400px",
										height: "200px",
										animation: `bannerFadeIn 0.6s ease-out ${index * 0.15}s both`,
									}}
								>
									<svg
										aria-hidden="true"
										className="w-10 h-10 text-[#9ca3af] mb-2"
										fill="none"
										stroke="currentColor"
										viewBox="0 0 24 24"
									>
										<path
											strokeLinecap="round"
											strokeLinejoin="round"
											strokeWidth={1.5}
											d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
										/>
									</svg>
									<span className="text-[#6b6b6b] text-[14px] font-medium">
										Banner {index}
									</span>
									<span className="text-[#9ca3af] text-[12px] mt-1">
										尚未設定
									</span>
								</div>
							))}
				</div>
			</div>

			{/* Table Section */}
			<div className="bg-white rounded-[15px] border border-[var(--border-color)] shadow-[0px_4px_4px_0px_rgba(0,0,0,0.1)] overflow-hidden">
				<table className="w-full border-collapse">
					<thead className="bg-[var(--color-11)]">
						<tr>
							<th className="w-[104px] px-[16px] py-[12px] text-center text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								拖拉排序
							</th>
							<th className="w-[220px] pl-[31px] py-[12px] text-left text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								圖片
							</th>
							<th className="pl-[31px] py-[12px] text-left text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								標題/連結
							</th>
							<th className="w-[210px] pl-[31px] py-[12px] text-left text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								排程時間
							</th>
							<th className="w-[180px] pl-[31px] py-[12px] text-left text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								狀態
							</th>
							<th className="w-[210px] pl-[32px] py-[12px] text-left text-[18px] font-bold text-[var(--brand-gray1-100)] border border-[var(--border-color)]">
								操作
							</th>
						</tr>
					</thead>
					<tbody>
						{banners.length === 0 ? (
							<tr>
								<td
									colSpan={6}
									className="px-8 py-16 text-center text-[var(--brand-gray1-100)]"
								>
									<svg
										aria-hidden="true"
										className="w-12 h-12 mx-auto mb-3 text-[var(--color-11)]"
										fill="none"
										stroke="currentColor"
										viewBox="0 0 24 24"
									>
										<path
											strokeLinecap="round"
											strokeLinejoin="round"
											strokeWidth={1.5}
											d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
										/>
									</svg>
									<span className="text-[16px]">尚無 Banner 資料</span>
								</td>
							</tr>
						) : (
							banners.map((banner) => {
								const imgUrl = getImageUrl(banner);
								const action = actionFromProto(banner.action);
								return (
									<tr
										key={banner.id}
										className="bg-white hover:bg-gray-50 transition-colors h-[100px]"
									>
										{/* 拖拉排序 */}
										<td className="px-[18px] border border-[var(--border-color)] align-middle">
											<div className="flex items-center justify-center">
												<span className="inline-flex items-center gap-[4px] px-[8px] py-[4px] rounded-[8px] bg-[var(--color-11)] text-[var(--brand-gray1-100)] cursor-grab hover:brightness-95 transition-all">
													<svg
														aria-hidden="true"
														className="w-[16px] h-[16px]"
														viewBox="0 0 24 24"
														fill="currentColor"
													>
														<path d="M7 10l5-5 5 5H7z" />
														<path d="M7 14l5 5 5-5H7z" />
													</svg>
													<span className="text-[16px] font-bold">
														#{banner.displayOrder}
													</span>
												</span>
											</div>
										</td>

										{/* 圖片 */}
										<td className="px-[32px] border border-[var(--border-color)] align-middle">
											<div className="flex items-center justify-center">
												<div className="w-[150px] h-[75px] rounded-[8px] overflow-hidden">
													{imgUrl ? (
														<Image
															src={imgUrl}
															alt={banner.title}
															width={150}
															height={75}
															unoptimized
															className="w-full h-full object-cover"
														/>
													) : (
														<div className="w-full h-full bg-[#f0f0f0] flex items-center justify-center text-[#9ca3af] rounded-[8px]">
															<svg
																aria-hidden="true"
																className="w-7 h-7"
																fill="none"
																stroke="currentColor"
																viewBox="0 0 24 24"
															>
																<path
																	strokeLinecap="round"
																	strokeLinejoin="round"
																	strokeWidth={1.5}
																	d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
																/>
															</svg>
														</div>
													)}
												</div>
											</div>
										</td>

										{/* 標題/連結 */}
										<td className="px-[32px] border border-[var(--border-color)] align-middle">
											<div className="flex flex-col gap-[6px]">
												<span className="text-[18px] font-bold text-black leading-[1.23]">
													{banner.title}
												</span>
												{action.actionType !== "popup" && (
													<span className="text-[14px] text-[var(--brand-gray1-80)] tracking-[1px]">
														{action.actionType === "url"
															? `🔗 ${action.actionUrl}`
															: `${action.actionType === "product" ? "📦 商品" : "📝 貼文"} #${action.actionId}`}
													</span>
												)}
											</div>
										</td>

										{/* 排程時間 */}
										<td className="px-[32px] border border-[var(--border-color)] align-middle">
											<div className="flex flex-col gap-[6px]">
												<div className="flex items-center gap-[6px]">
													<svg
														aria-hidden="true"
														className="w-[20px] h-[20px] text-[var(--brand-gray1-100)] shrink-0"
														viewBox="0 0 20 20"
														fill="currentColor"
													>
														<path d="M15.833 3.333h-.833V2.5c0-.46-.373-.833-.833-.833-.46 0-.834.373-.834.833v.833H6.667V2.5c0-.46-.373-.833-.834-.833-.46 0-.833.373-.833.833v.833h-.833c-.917 0-1.659.75-1.659 1.667l-.008 11.667c0 .916.75 1.666 1.667 1.666h11.666c.917 0 1.667-.75 1.667-1.666V5c0-.917-.75-1.667-1.667-1.667zm0 13.334H4.167V7.5h11.666v9.167zm-10-7.5h2.5v2.5h-2.5v-2.5zm3.334 0h2.5v2.5h-2.5v-2.5zm3.333 0h2.5v2.5h-2.5v-2.5z" />
													</svg>
													<span className="text-[16px] text-[var(--brand-gray1-100)] leading-[24px]">
														{startDateOf(banner.startTime) || "-"}
													</span>
												</div>
												{banner.endTime && (
													<span className="text-[14px] text-[var(--brand-gray1-80)] tracking-[1px]">
														至 {endDateOf(banner.endTime)}
													</span>
												)}
											</div>
										</td>

										{/* 狀態 */}
										<td className="px-[32px] border border-[var(--border-color)] align-middle">
											<div className="flex items-center gap-[12px]">
												<button
													type="button"
													onClick={() => handleToggle(banner)}
													className={`relative w-[30px] h-[18px] rounded-full transition-colors duration-200 ${
														banner.active ? "bg-[var(--color)]" : "bg-[#a6a6a6]"
													}`}
												>
													<span
														className={`absolute top-[1px] w-[16px] h-[16px] rounded-full bg-white shadow transition-transform duration-200 ${
															banner.active
																? "translate-x-[13px]"
																: "translate-x-[1px]"
														}`}
													/>
												</button>
												<span
													className={`text-[16px] leading-[24px] ${banner.active ? "font-bold text-[var(--color)]" : "text-[var(--brand-gray1-50)]"}`}
												>
													{banner.active ? "上架中" : "已下架"}
												</span>
											</div>
										</td>

										{/* 操作 */}
										<td className="px-[32px] border border-[var(--border-color)] align-middle">
											<div className="flex items-center gap-[13px]">
												<button
													type="button"
													onClick={() =>
														router.push(`/admin/banners/editor/${banner.id}`)
													}
													className="px-[10px] py-[6px] bg-[var(--color)] hover:opacity-90 text-white text-[16px] font-bold leading-[24px] rounded-[8px] transition-opacity"
												>
													編輯
												</button>
												<button
													type="button"
													onClick={() => handleDelete(banner)}
													className="px-[10px] py-[6px] bg-white border-2 border-[var(--color-10)] text-[var(--color-10)] hover:bg-[#fff0f0] text-[16px] font-bold leading-[24px] rounded-[8px] transition-colors"
												>
													刪除
												</button>
											</div>
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>
			{hasNextPage && (
				<div className="flex justify-center">
					<button
						type="button"
						onClick={() => void fetchNextPage()}
						disabled={isFetchingNextPage}
						className="px-[16px] py-[10px] rounded-[8px] border border-[var(--border-color)] bg-white text-[16px] text-[var(--text)] hover:bg-gray-50 disabled:opacity-50"
					>
						{isFetchingNextPage ? "載入中…" : "載入更多"}
					</button>
				</div>
			)}
		</div>
	);
};

export default BannerList;
