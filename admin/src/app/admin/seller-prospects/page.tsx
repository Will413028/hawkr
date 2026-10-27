"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	type Prospect,
	ProspectStatus,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useState } from "react";
import { Button, StatCard, StateRow, StatusBadge } from "@/components/UI";
import type { BadgeVariant } from "@/components/UI/StatusBadge";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import {
	hasActiveFilter,
	listProspectsFields,
	outreachStatusText,
	prospectStatusText,
	readProspectFilters,
	vendorTypeText,
} from "@/lib/seller-prospects";
import CreateProspectDialog from "./CreateProspectDialog";
import ProspectBrandImage from "./ProspectBrandImage";
import "@/styles/pages/SellerProspectMedia.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { connectErrorText } from "@/lib/error-text";
import ProspectFilters, { LIST_PATH } from "./ProspectFilters";

const PAGE_SIZE = 20;
const COUNT_CARD_PAGE_SIZE = 50;
const COUNT_CARD_STALE_MS = 5 * 60 * 1000;

function statusBadgeVariant(status: ProspectStatus): BadgeVariant {
	switch (status) {
		case ProspectStatus.ACTIVE:
			return "success";
		case ProspectStatus.CONVERTED:
			return "info";
		default:
			return "gray"; // ARCHIVED 與未知值都算「不是進行中」，用同一顆灰色徽章
	}
}

/** 統計卡的數字：第一頁的筆數，後面還有下一頁時加「+」。 */
function countCardValue(
	isError: boolean,
	count: number | undefined,
	hasMore: boolean,
): string {
	if (isError) return "—";
	if (count === undefined) return "--";
	return `${count}${hasMore ? "+" : ""}`;
}

function ProspectRow({ prospect }: { prospect: Prospect }) {
	return (
		<tr>
			<td>
				<Link
					href={`/admin/seller-prospects/${prospect.id}`}
					className="cell-main prospect-media__list-name"
				>
					{prospect.logoImage && (
						<ProspectBrandImage
							className="prospect-media__list-logo"
							image={prospect.logoImage}
							preset="avatar"
							alt=""
							empty=""
						/>
					)}
					{prospect.displayName}
				</Link>
				{/* 併入別筆的列不能只顯示原資料：沒有這個徽章與連結，管理員會誤以為這是一筆還在獨立追蹤的業者。 */}
				{prospect.mergedIntoId ? (
					<div className="cell-sub">
						<StatusBadge variant="warning" size="sm">
							已合併
						</StatusBadge>{" "}
						<Link href={`/admin/seller-prospects/${prospect.mergedIntoId}`}>
							查看主檔
						</Link>
					</div>
				) : null}
			</td>
			<td>{vendorTypeText(prospect.vendorType)}</td>
			<td>
				<StatusBadge variant={statusBadgeVariant(prospect.status)}>
					{prospectStatusText(prospect.status)}
				</StatusBadge>
			</td>
			<td>{outreachStatusText(prospect.outreachStatus)}</td>
			<td>{prospect.category || "-"}</td>
			<td>{prospect.region || "-"}</td>
			<td>{prospect.taxId || "-"}</td>
			<td>
				{prospect.updateTime
					? formatDateTimeShort(timestampDate(prospect.updateTime))
					: "-"}
			</td>
		</tr>
	);
}

export default function SellerProspectsPage() {
	const router = useRouter();
	const searchParams = useSearchParams();
	const filters = readProspectFilters(searchParams);
	const hasFilter = hasActiveFilter(filters);

	// token 綁定篩選條件，換條件時回到第一頁。用 render 期間同步調整 state 的寫法（React 官方建議的
	// 「prop 變了就重置某個 state」），不用 useEffect。
	const filtersKey = JSON.stringify(filters);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [pageResetKey, setPageResetKey] = useState(filtersKey);
	if (filtersKey !== pageResetKey) {
		setPageResetKey(filtersKey);
		setPageTokens(firstPage());
	}

	const [showCreateDialog, setShowCreateDialog] = useState(false);

	const list = useQuery(SellerProspectAdminService.method.listProspects, {
		...listProspectsFields(filters),
		pageSize: PAGE_SIZE,
		pageToken: currentToken(pageTokens),
	});
	const records = list.data?.prospects ?? [];
	const next = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	// 兩張統計卡只顯示入口上的待處理數量：第一頁的筆數，後面還有就加「+」。
	const duplicates = useQuery(
		SellerProspectAdminService.method.listProspectDuplicates,
		{ pageSize: COUNT_CARD_PAGE_SIZE },
		{ staleTime: COUNT_CARD_STALE_MS },
	);
	const taxIdMatches = useQuery(
		SellerProspectAdminService.method.listProspectTaxIDMatches,
		{ pageSize: COUNT_CARD_PAGE_SIZE },
		{ staleTime: COUNT_CARD_STALE_MS },
	);
	const duplicatesValue = countCardValue(
		duplicates.isError,
		duplicates.data?.duplicates.length,
		(duplicates.data?.nextPageToken ?? "") !== "",
	);
	const taxIdMatchesValue = countCardValue(
		taxIdMatches.isError,
		taxIdMatches.data?.matches.length,
		(taxIdMatches.data?.nextPageToken ?? "") !== "",
	);

	const clearFilters = () => router.replace(LIST_PATH);

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">站外業者主檔</h1>
					<p className="page-subtitle">
						尚未加入平台的目標賣家名單，供業務團隊追蹤聯絡與入駐進度。預設只列進行中的業者，已封存或已入駐的請切換狀態篩選。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="primary"
						icon={<MaterialSymbol name="add" />}
						onClick={() => setShowCreateDialog(true)}
					>
						新增站外業者
					</Button>
				</div>
			</div>

			<div className="stats-row-v2">
				<button
					type="button"
					className="btn-reset stat-card-clickable"
					onClick={() => router.push("/admin/seller-prospects/duplicates")}
					aria-label={`疑似重複：${duplicatesValue} 組候選，點選查看清單`}
				>
					<StatCard
						label="疑似重複"
						value={duplicatesValue}
						icon="content_copy"
						variant="warning"
					/>
				</button>
				<button
					type="button"
					className="btn-reset stat-card-clickable"
					onClick={() => router.push("/admin/seller-prospects/reconciliation")}
					aria-label={`統編對帳：${taxIdMatchesValue} 筆待覆核，點選查看清單`}
				>
					<StatCard
						label="統編對帳"
						value={taxIdMatchesValue}
						icon="balance"
						variant="default"
					/>
				</button>
			</div>

			<ProspectFilters />

			{/* `.table-container` 的 `overflow: hidden`（globals.css）只裁切、不提供橫向捲軸；
			    行內覆寫 `overflow-x: auto` 才是真的讓窄螢幕能在自身容器捲動。 */}
			<div className="table-container" style={{ overflowX: "auto" }}>
				<table className="data-table">
					<thead>
						<tr>
							<th>業者名稱</th>
							<th>類型</th>
							<th>狀態</th>
							<th>招商狀態</th>
							<th>分類</th>
							<th>地區</th>
							<th>統編</th>
							<th>更新時間</th>
						</tr>
					</thead>
					<tbody>
						<StateRow
							loading={list.isLoading}
							error={
								list.isError
									? new Error(
											connectErrorText(list.error, "載入失敗，請稍後重試。"),
										)
									: null
							}
							empty={records.length === 0}
							emptyText={
								hasFilter
									? "目前沒有符合條件的站外業者"
									: "目前尚未建立任何站外業者主檔"
							}
							emptyAction={
								hasFilter ? (
									<Button variant="outline" size="sm" onClick={clearFilters}>
										清除篩選
									</Button>
								) : null
							}
							colSpan={8}
							onRetry={() => void list.refetch()}
						>
							{records.map((p) => (
								<ProspectRow key={p.id} prospect={p} />
							))}
						</StateRow>
					</tbody>
				</table>
			</div>

			{page > 1 || next ? (
				<div
					style={{
						display: "flex",
						alignItems: "center",
						justifyContent: "center",
						gap: 16,
						marginTop: 20,
					}}
				>
					<Button
						variant="outline"
						size="sm"
						disabled={page === 1}
						onClick={() => setPageTokens(previousPage(pageTokens))}
					>
						上一頁
					</Button>
					<span>第 {page} 頁</span>
					<Button
						variant="outline"
						size="sm"
						disabled={!next}
						onClick={() => setPageTokens(nextPage(pageTokens, next))}
					>
						下一頁
					</Button>
				</div>
			) : null}

			{showCreateDialog ? (
				<CreateProspectDialog onClose={() => setShowCreateDialog(false)} />
			) : null}
		</div>
	);
}
