"use client";

import { useQuery } from "@connectrpc/connect-query";
import {
	type FoodRegistration,
	SellerAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/seller_pb";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { DeadlineText, StatusBadge } from "@/components/UI";
import { deadlineView } from "@/lib/deadline";
import { errorMessage } from "@/lib/error-text";
import {
	LIST_TABS,
	listRequest,
	stateBadge,
	submitTimeText,
} from "@/lib/food-registration";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { kycStatusText } from "@/lib/kyc-status";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import "@/styles/pages/MerchantManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const PAGE_SIZE = 20;
const COLUMN_COUNT = 7;

export default function FoodRegistrationInbox() {
	const router = useRouter();
	const now = useNow(LIST_REFRESH_MS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [searchKeyword, setSearchKeyword] = useState("");
	const [appliedKeyword, setAppliedKeyword] = useState("");
	const [tab, setTab] = useState(LIST_TABS[0]);

	const list = useQuery(
		SellerAdminService.method.listFoodRegistrations,
		listRequest({
			state: tab.state,
			inbox: tab.inbox,
			keyword: appliedKeyword,
			pageToken: currentToken(pageTokens),
			pageSize: PAGE_SIZE,
		}),
	);
	const requests: FoodRegistration[] = list.data?.registrations ?? [];
	const nextPageToken = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const changeTab = (next: (typeof LIST_TABS)[number]) => {
		setTab(next);
		setPageTokens(firstPage());
	};
	const handleSearch = () => {
		setPageTokens(firstPage());
		setAppliedKeyword(searchKeyword);
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
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void list.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="verified" />
				<p>{tab.emptyText}</p>
			</div>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">食品業者登錄審核</h1>
					<p className="page-subtitle">
						對照食藥署的登錄資料核對賣家送出的字號；「待審核」是店家已通過商家認證、可以裁定的件，「等商家認證」是還不能核准的件。
					</p>
				</div>
			</div>

			<div className="merchant-toolbar">
				<div className="merchant-search-input">
					<input
						type="text"
						aria-label="搜尋食品業者登錄"
						placeholder="搜尋字號 / 店名..."
						value={searchKeyword}
						onChange={(e) => setSearchKeyword(e.target.value)}
						onKeyDown={(e) => e.key === "Enter" && handleSearch()}
					/>
				</div>
				<div className="merchant-filter-tabs">
					{LIST_TABS.map((item) => (
						<button
							key={item.key}
							type="button"
							className={`filter-tab ${tab.key === item.key ? "active" : ""}`}
							onClick={() => changeTab(item)}
						>
							{item.label}
						</button>
					))}
				</div>
			</div>

			<div className="merchant-table-container">
				<table className="merchant-table">
					<thead>
						<tr>
							<th>店家</th>
							<th>送出的字號</th>
							<th>商家認證</th>
							<th>送出時間</th>
							<th>期限</th>
							<th>狀態</th>
							<th>操作</th>
						</tr>
					</thead>
					<tbody>
						{list.isLoading || list.error || requests.length === 0 ? (
							<tr>
								<td
									colSpan={COLUMN_COUNT}
									className="merchant-table-state-cell"
								>
									{renderTableState()}
								</td>
							</tr>
						) : (
							requests.map((r) => {
								const badge = stateBadge(r.state);
								const deadline = deadlineView(r.decideDeadline, now);
								return (
									<tr key={r.id}>
										<td>
											<div className="merchant-name" title={r.storeName}>
												{r.storeName || "-"}
											</div>
										</td>
										<td>
											<span className="merchant-shop-code">
												{r.registrationNumber}
											</span>
										</td>
										<td>{kycStatusText(r.kycStatus)}</td>
										<td>
											<span className="merchant-date">{submitTimeText(r)}</span>
										</td>
										<td>
											<DeadlineText
												display={deadline}
												title={deadline.dueText}
											/>
										</td>
										<td>
											<StatusBadge variant={badge.variant}>
												{badge.label}
											</StatusBadge>
										</td>
										<td>
											<button
												type="button"
												className={`btn-action ${r.canReject ? "warning" : "gray"}`}
												onClick={() =>
													router.push(`/admin/food-registrations/${r.id}`)
												}
											>
												{r.canReject ? "審核" : "查看詳情"}
											</button>
										</td>
									</tr>
								);
							})
						)}
					</tbody>
				</table>
			</div>

			{(page > 1 || nextPageToken) && (
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
						onClick={() => setPageTokens((t) => nextPage(t, nextPageToken))}
						disabled={!nextPageToken}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}
