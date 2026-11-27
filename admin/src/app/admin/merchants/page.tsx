"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import {
	SellerAdminService,
	type StoreListItem as StoreItem,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Image from "next/image";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { StatusBadge } from "@/components/UI";
import StatsCard from "@/components/UI/StatsCard";
import MerchantOperationEntry from "@/features/delegated-operations/components/MerchantOperationEntry";
import { errorMessage } from "@/lib/error-text";
import { formatIsoDate, formatStat } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import {
	FILTER_TAB_LABEL,
	FILTER_TABS,
	type FilterTab,
	parseFilterTab,
} from "@/lib/store-search";
import { unavailableLabels } from "@/lib/unavailability";
import { useStoreSearch } from "@/lib/use-store-search";
import { imageUri } from "@/utils/image";
import { rowBadge, rowLocation, rowState } from "./store-row";
import "@/styles/pages/MerchantManagement.css";
import "@/styles/pages/DelegatedOperations.css";

const PAGE_SIZE = 10;
const COLUMN_COUNT = 6;

export default function MerchantManagement() {
	const router = useRouter();
	const searchParams = useSearchParams();

	const [pageTokens, setPageTokens] = useState(firstPage);
	const [selectedId, setSelectedId] = useState<string | null>(null);

	const [searchKeyword, setSearchKeyword] = useState("");
	const [appliedKeyword, setAppliedKeyword] = useState("");
	const [filterTab, setFilterTab] = useState<FilterTab>(
		parseFilterTab(searchParams.get("filter")),
	);

	const list = useStoreSearch({
		keyword: appliedKeyword,
		tab: filterTab,
		pageToken: currentToken(pageTokens),
		pageSize: PAGE_SIZE,
	});

	const statsQuery = useConnectQuery(
		SellerAdminService.method.getStoreStats,
		{},
	);
	const stats = statsQuery.data;
	const statValue = (value: number | undefined) =>
		formatStat(value, statsQuery.isLoading, statsQuery.isError);

	const merchants = list.items;
	const page = currentPage(pageTokens);

	const handleFilterTabChange = (tab: FilterTab) => {
		setFilterTab(tab);
		setPageTokens(firstPage());
	};

	const handleSearch = () => {
		setPageTokens(firstPage());
		setAppliedKeyword(searchKeyword);
	};

	const formatDate = (item: StoreItem) => {
		const createTime = item.store?.createTime;
		return createTime ? formatIsoDate(timestampDate(createTime)) : "-";
	};

	const getActionButtons = (m: StoreItem) => {
		const storeId = m.store?.id ?? "";
		const { isPending, isRejected, isActive } = rowState(m);
		const delegationBlockedReason =
			m.delegationUnavailabilities.length > 0
				? unavailableLabels(m.delegationUnavailabilities)[0]
				: undefined;

		let btnClass = "btn-action gray";
		if (isPending)
			btnClass = "btn-action warning";
		else if (isRejected)
			btnClass = "btn-action danger";
		else if (isActive) btnClass = "btn-action success";

		return (
			<div className="adm-merchant-row-actions" data-row-action>
				<button
					type="button"
					className={btnClass}
					onClick={(e) => {
						e.stopPropagation();
						router.push(`/admin/merchants/${storeId}`);
					}}
				>
					{isPending ? "審核" : "查看詳情"}
				</button>

				<MerchantOperationEntry
					storeId={storeId}
					enabled
					blockedReason={delegationBlockedReason}
					label="代行營運"
					compact
				/>
			</div>
		);
	};

	const renderTableState = () => {
		if (list.isLoading) {
			return (
				<div className="merchant-table-state">
					<MaterialSymbol
						name="progress_activity"
						className="merchant-table-state-spinner"
					/>
					<p>載入中...</p>
				</div>
			);
		}
		if (list.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>{errorMessage(list.error)}</p>
					<button type="button" className="btn btn-sm" onClick={list.refetch}>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="storefront" />
				<p>無賣家資料</p>
			</div>
		);
	};

	return (
		<div className="merchant-management">

			<div className="page-header">
				<div>
					<h1 className="page-title">賣家管理</h1>
					<p className="page-subtitle">管理平台所有賣家店鋪</p>
				</div>
			</div>

			<aside
				className="adm-operation-safety-note"
				aria-label="代行營運安全說明"
			>
				<MaterialSymbol name="shield_person" />
				<div>
					<strong>受控的 ADM 代行營運</strong>
					<p>
						可在核發範圍內協助店面、商品與內容；ADM
						身分與每次操作都會留在稽核紀錄中。
					</p>
				</div>
				<span>
					<MaterialSymbol name="lock_person" />
					訂單履約與出貨仍由賣家操作
				</span>
			</aside>


			<div className="stats-row-v2 merchant-stats-grid">
				<button
					type="button"
					onClick={() => handleFilterTabChange("all")}
					className={`btn-reset stat-card-clickable ${filterTab === "all" ? "active" : ""}`}
				>
					<StatsCard
						label="總賣家數"
						value={statValue(stats?.totalStores)}
						icon="storefront"
						variant="store"
					/>
				</button>
				<button
					type="button"
					onClick={() => handleFilterTabChange("pending")}
					className={`btn-reset stat-card-clickable ${filterTab === "pending" ? "active" : ""}`}
				>
					<StatsCard
						label="待審核"
						value={statValue(stats?.kycAwaitingReviewStores)}
						icon="pending_actions"
						variant="warning"
					/>
				</button>
				<button
					type="button"
					onClick={() => handleFilterTabChange("rejected")}
					className={`btn-reset stat-card-clickable ${filterTab === "rejected" ? "active" : ""}`}
				>
					<StatsCard
						label="審核失敗"
						value={statValue(stats?.kycRejectedStores)}
						icon="cancel"
						variant="danger"
					/>
				</button>
				<button
					type="button"
					onClick={() => handleFilterTabChange("approved")}
					className={`btn-reset stat-card-clickable ${filterTab === "approved" ? "active" : ""}`}
				>
					<StatsCard
						label="已核准"
						value={statValue(stats?.kycApprovedStores)}
						icon="check_circle"
						variant="success"
					/>
				</button>
				<div>
					<StatsCard
						label="本月增加賣家"
						value={statValue(stats?.newStoresThisMonth)}
						icon="trending_up"
						variant="trending"
					/>
				</div>
			</div>


			<div className="merchant-toolbar">
				<div className="merchant-search-input">
					<MaterialSymbol name="search" size={18} className="search-icon" />
					<input
						type="text"
						aria-label="搜尋賣家"
						placeholder="搜尋店名 / 聯絡電話 / 商店代號 / 登入信箱..."
						value={searchKeyword}
						onChange={(e) => setSearchKeyword(e.target.value)}
						onKeyDown={(e) => e.key === "Enter" && handleSearch()}
					/>
				</div>
				<div className="merchant-filter-tabs">
					{FILTER_TABS.map((tab) => (
						<button
							key={tab}
							type="button"
							className={`filter-tab ${filterTab === tab ? "active" : ""}`}
							onClick={() => handleFilterTabChange(tab)}
						>
							{FILTER_TAB_LABEL[tab]}
						</button>
					))}
				</div>
			</div>


			<div className="merchant-table-container">

				<table className="merchant-table">
					<thead>
						<tr>
							<th>賣家資訊</th>
							<th>商店代號</th>
							<th>地點</th>
							<th>狀態</th>
							<th>加入日期</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{list.isLoading || list.error || merchants.length === 0 ? (
							<tr>
								<td
									colSpan={COLUMN_COUNT}
									className="merchant-table-state-cell"
								>
									{renderTableState()}
								</td>
							</tr>
						) : (
							merchants.map((m) => {
								const store = m.store;
								const storeId = store?.id ?? "";
								const shopName = store?.storeName ?? "";
								const logo = imageUri(store?.logo, "avatar");
								const badge = rowBadge(m);
								return (
									<tr
										key={storeId}
										className={selectedId === storeId ? "selected" : ""}
										onClick={(event) => {
											if (
												event.target instanceof Element &&
												event.target.closest("[data-row-action]")
											) {
												return;
											}
											setSelectedId(selectedId === storeId ? null : storeId);
										}}
									>

										<td>
											<div className="merchant-info-cell">
												<div className="merchant-avatar">
													{logo ? (
														<Image
															src={logo}
															alt={shopName}
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
													<span style={logo ? { display: "none" } : undefined}>
														{shopName?.[0] || "店"}
													</span>
												</div>
												<div className="merchant-details">

													<div className="merchant-name" title={shopName}>
														{shopName}
													</div>
												</div>
											</div>
										</td>

										<td>
											<span className="merchant-shop-code">
												{store?.storeCode || "-"}
											</span>
										</td>

										<td>
											<div className="merchant-type-cell">
												<div className="merchant-location">
													<MaterialSymbol name="location_on" />
													{rowLocation(m)}
												</div>
											</div>
										</td>

										<td>
											<StatusBadge variant={badge.variant}>
												{badge.label}
											</StatusBadge>
										</td>

										<td>
											<span className="merchant-date">{formatDate(m)}</span>
										</td>

										<td>{getActionButtons(m)}</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>


			{(page > 1 || list.nextPageToken) && (
				<div className="merchant-pagination">
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => setPageTokens(previousPage)}
						disabled={page <= 1}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span className="page-info">第 {page} 頁</span>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() =>
							setPageTokens((t) => nextPage(t, list.nextPageToken))
						}
						disabled={!list.nextPageToken}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}
