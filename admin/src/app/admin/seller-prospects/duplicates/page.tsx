"use client";

import { useQuery } from "@connectrpc/connect-query";
import {
	type ProspectDuplicate,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { matchReasonsText } from "@/lib/seller-prospects";
import MergeDialog from "../MergeDialog";
import { LIST_PATH } from "../ProspectFilters";
import MarkDistinctDialog from "./MarkDistinctDialog";
import "@/styles/pages/DelegatedOperations.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

/**
 * 站外業者「疑似重複」清單：進行中的業者之間名稱相同或聯絡方式相同、而且還沒標記為不是同一家的配對，keyset 分頁。
 * 清單不帶聯絡方式，每列只顯示名稱與統編；處理方式是「比對並合併」（開 `MergeDialog`，開啟時才讀兩邊的完整資料）
 * 或「不是同一家」。
 *
 * 空清單與載入失敗必須分辨得出來：空清單是「這批資料很乾淨」的好消息，不是出錯，`emptyText` 特別寫成
 * 「目前沒有疑似重複的業者」而不是「查無資料」。
 */

const PAGE_SIZE = 50;

function ProspectSummaryCell({
	prospect,
}: {
	prospect: ProspectDuplicate["left"];
}) {
	return (
		<>
			<div className="cell-main">
				<Link href={`/admin/seller-prospects/${prospect?.id ?? ""}`}>
					{prospect?.displayName || "（未命名）"}
				</Link>
			</div>
			<div className="cell-sub">統編 · {prospect?.taxId || "未填寫"}</div>
		</>
	);
}

export default function SellerProspectDuplicatesPage() {
	const router = useRouter();
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [mergePair, setMergePair] = useState<ProspectDuplicate | null>(null);
	const [distinctPair, setDistinctPair] = useState<ProspectDuplicate | null>(
		null,
	);

	const list = useQuery(
		SellerProspectAdminService.method.listProspectDuplicates,
		{
			pageSize: PAGE_SIZE,
			pageToken: currentToken(pageTokens),
		},
	);
	const duplicates = list.data?.duplicates ?? [];
	const next = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">疑似重複的站外業者</h1>
					<p className="page-subtitle">
						進行中的業者之間名稱相同，或聯絡方式（電子郵件、電話、網站網域、社群帳號）相同、而且還沒標記為不是同一家的配對。可以合併（不可逆），或標記為不是同一家。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="outline"
						icon={<MaterialSymbol name="arrow_back" />}
						onClick={() => router.push(LIST_PATH)}
					>
						返回列表
					</Button>
				</div>
			</div>

			{/* `.table-container` 的 `overflow: hidden`（globals.css）只裁切、不提供橫向捲軸；
			    行內覆寫 `overflow-x: auto` 才是真的讓窄螢幕能在自身容器捲動。 */}
			<div className="table-container" style={{ overflowX: "auto" }}>
				<table className="data-table">
					<thead>
						<tr>
							<th>比對依據</th>
							<th>業者 A</th>
							<th>業者 B</th>
							<th style={{ width: 220 }}>操作</th>
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
							empty={duplicates.length === 0}
							emptyText="目前沒有疑似重複的業者"
							colSpan={4}
							onRetry={() => void list.refetch()}
						>
							{duplicates.map((pair) => (
								<tr key={`${pair.left?.id}:${pair.right?.id}`}>
									<td>
										<StatusBadge variant="warning" size="sm">
											{matchReasonsText(pair.matchedOn)}
										</StatusBadge>
									</td>
									<td>
										<ProspectSummaryCell prospect={pair.left} />
									</td>
									<td>
										<ProspectSummaryCell prospect={pair.right} />
									</td>
									<td>
										<div style={{ display: "flex", gap: 8 }}>
											<Button
												variant="outline"
												size="sm"
												onClick={() => setMergePair(pair)}
											>
												比對並合併
											</Button>
											<Button
												variant="outline"
												size="sm"
												onClick={() => setDistinctPair(pair)}
											>
												不是同一家
											</Button>
										</div>
									</td>
								</tr>
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

			{mergePair?.left && mergePair.right ? (
				<MergeDialog
					leftId={mergePair.left.id}
					rightId={mergePair.right.id}
					matchedOn={mergePair.matchedOn}
					onMerged={() => setMergePair(null)}
					onClose={() => setMergePair(null)}
				/>
			) : null}

			{distinctPair ? (
				<MarkDistinctDialog
					pair={distinctPair}
					onClose={() => setDistinctPair(null)}
				/>
			) : null}
		</div>
	);
}
