"use client";

import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { BuyerAdminService } from "@hawkr/api/gen/hawkr/admin/buyer/v1alpha1/buyer_pb";
import { IdentityAdminService } from "@hawkr/api/gen/hawkr/admin/identity/v1alpha1/identity_pb";
import { keepPreviousData } from "@tanstack/react-query";
import type React from "react";
import { useState } from "react";
import { formatStat } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	pageCount,
	previousPage,
} from "@/lib/keyset-pages";
import { formatCreateDate } from "./account-list";
import "@/styles/pages/MemberList.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const MemberList: React.FC = () => {
	const buyerQuery = useConnectQuery(
		BuyerAdminService.method.getBuyerStats,
		{},
	);

	const [pageTokens, setPageTokens] = useState(firstPage);
	const [searchKeyword, setSearchKeyword] = useState("");
	const [appliedKeyword, setAppliedKeyword] = useState("");
	const pageSize = 10;

	// 賣家已經有獨立的「賣家管理 / 審核」頁面，這裡固定只看買家，不開放身份篩選。
	// 每個帳號都是買家（買家不是契約的 Role），所以不帶角色篩選就是全部買家。
	// query key 由 connect-query 以方法與請求（含 page token 與關鍵字）組成。
	const listQuery = useConnectQuery(
		IdentityAdminService.method.listAccounts,
		{
			pageSize,
			pageToken: currentToken(pageTokens),
			query: appliedKeyword,
		},
		{ placeholderData: keepPreviousData },
	);

	const accountList = listQuery.data?.accounts ?? [];
	const page = currentPage(pageTokens);
	const totalPages = pageCount(listQuery.data?.totalSize ?? 0, pageSize);
	// 換頁載入時畫面還是上一頁的資料，它的 next_page_token 已經用過，不能再往下翻。
	const nextPageToken = listQuery.isPlaceholderData
		? ""
		: (listQuery.data?.nextPageToken ?? "");

	// page token 綁定篩選條件：換關鍵字時回到第一頁。
	const applySearch = () => {
		setPageTokens(firstPage());
		setAppliedKeyword(searchKeyword);
	};

	return (
		<div className="member-list-page">
			<div className="page-header">
				<div className="header-left">
					<h1 className="page-title">買家管理</h1>
					<p className="page-subtitle">管理平台所有買家帳號</p>
				</div>
			</div>

			<div className="stats-row">
				<div className="stat-card">
					<div className="stat-icon blue">
						<MaterialSymbol name="person" />
					</div>
					<div className="stat-content">
						<div className="stat-label">買家用戶</div>
						<div className="stat-value">
							{formatStat(
								buyerQuery.data?.totalBuyers,
								buyerQuery.isLoading,
								buyerQuery.isError,
							)}
						</div>
					</div>
				</div>
			</div>

			<div className="toolbar">
				<div className="search-input">
					<MaterialSymbol name="search" />
					<input
						type="text"
						placeholder="搜尋帳號 ID / Email"
						value={searchKeyword}
						onChange={(e) => setSearchKeyword(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") applySearch();
						}}
					/>
				</div>
				<button type="button" className="btn-search" onClick={applySearch}>
					<MaterialSymbol name="search" /> 搜尋
				</button>
			</div>

			<div className="table-container">
				<table className="member-table">
					<thead>
						<tr>
							<th>帳號 ID</th>
							<th>Email</th>
							<th>建立時間</th>
						</tr>
					</thead>
					<tbody>
						{listQuery.isLoading ? (
							<tr>
								<td colSpan={3} className="loading-cell">
									<MaterialSymbol
										name="progress_activity"
										className="icon-spin"
									/>
									<p>載入中…</p>
								</td>
							</tr>
						) : listQuery.isError ? (
							<tr>
								<td colSpan={3} className="empty-cell">
									<MaterialSymbol name="error" />
									<p>載入失敗，請重新整理</p>
								</td>
							</tr>
						) : accountList.length === 0 ? (
							<tr>
								<td colSpan={3} className="empty-cell">
									<MaterialSymbol name="person_remove" />
									<p>查無符合條件的會員</p>
								</td>
							</tr>
						) : (
							accountList.map((account) => (
								<tr key={account.id}>
									<td>{account.id}</td>
									<td>{account.email || "-"}</td>
									<td>{formatCreateDate(account.createTime)}</td>
								</tr>
							))
						)}
					</tbody>
				</table>
			</div>

			{totalPages > 1 && (
				<div className="pagination">
					<button
						type="button"
						className="btn-page"
						disabled={page <= 1}
						onClick={() => setPageTokens(previousPage)}
					>
						<MaterialSymbol name="keyboard_arrow_left" /> 上一頁
					</button>
					<span className="page-info">
						第 {page} / {totalPages} 頁
					</span>
					<button
						type="button"
						className="btn-page"
						disabled={!nextPageToken}
						onClick={() => setPageTokens((t) => nextPage(t, nextPageToken))}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
};

export default MemberList;
