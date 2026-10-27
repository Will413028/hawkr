"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery as useConnectQuery } from "@connectrpc/connect-query";
import { SellerGovernanceAdminService } from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/governance_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import Link from "next/link";
import { useState } from "react";
import { Button, StatusBadge } from "@/components/UI";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	type PageTokens,
	previousPage,
} from "@/lib/keyset-pages";
import { requestStateText } from "../../measures/measure-inbox";
import {
	measureBasisText,
	measureStateBadge,
	revokeDetailText,
} from "./measure";

const PAGE_SIZE = 10;

/**
 * 措施歷史（帳號範圍，新到舊；警告、停權、已結束與已撤銷的都在）。上一頁／下一頁，不顯示總數（K2）。請求欄列出申覆與解除請求的全部種類，
 * 「查看」連到措施詳情頁（/admin/measures/[id]）。
 * 換店家時由呼叫端以 key 重新掛載，page token 不會跨店家沿用。
 */
export default function MeasureHistoryCard({
	accountId,
}: {
	accountId: string;
}) {
	const [pageTokens, setPageTokens] = useState<PageTokens>(firstPage);
	const query = useConnectQuery(
		SellerGovernanceAdminService.method.listMeasures,
		{
			accountId,
			pageSize: PAGE_SIZE,
			pageToken: currentToken(pageTokens),
		},
	);
	const measures = query.data?.measures ?? [];
	const nextPageToken = query.data?.nextPageToken ?? "";

	return (
		<div className="detail-card suspension-history-card">
			<div className="section-title">
				<MaterialSymbol name="history" />
				<span>措施歷史</span>
			</div>
			<div className="card-body" style={{ overflowX: "auto" }}>
				{query.isLoading ? (
					<div className="tab-empty-state">
						<div className="loading-spinner"></div>
						<span>載入措施紀錄中...</span>
					</div>
				) : query.isError ? (
					<div className="tab-empty-state">
						<MaterialSymbol name="error" />
						<span>{connectErrorText(query.error, "措施紀錄讀取失敗")}</span>
						<Button variant="outline" onClick={() => void query.refetch()}>
							重試
						</Button>
					</div>
				) : measures.length === 0 ? (
					<div className="tab-empty-state">
						<MaterialSymbol name="history" />
						<span>尚無措施紀錄</span>
					</div>
				) : (
					<table className="tab-table">
						<thead>
							<tr>
								<th>建立時間</th>
								<th>依據</th>
								<th>給賣家的理由</th>
								<th>結束時間</th>
								{/* 「操作者 ID」：後台只顯示管理員 ID，不查姓名（K16）。 */}
								<th>操作者 ID</th>
								<th>狀態</th>
								<th>請求</th>
								<th>詳情</th>
							</tr>
						</thead>
						<tbody>
							{measures.map((m) => {
								const state = measureStateBadge(m.state, m.kind);
								const revoked = revokeDetailText(m);
								return (
									<tr key={m.id}>
										<td>
											{m.createTime
												? formatDateTimeShort(timestampDate(m.createTime))
												: "-"}
										</td>
										<td>{measureBasisText(m)}</td>
										<td>{m.statement || "-"}</td>
										<td>
											{m.endTime
												? formatDateTimeShort(timestampDate(m.endTime))
												: "-"}
										</td>
										<td>{m.createdByAdminId || "-"}</td>
										<td>
											<StatusBadge variant={state.variant}>
												{state.label}
											</StatusBadge>
											{revoked && (
												<div className="adm-operation-field-note">
													撤銷：{revoked}
												</div>
											)}
										</td>
										<td>
											{m.requests.length > 0
												? m.requests.map(requestStateText).join("、")
												: "-"}
										</td>
										<td>
											<Link
												className="btn-action gray"
												href={`/admin/measures/${m.id}`}
											>
												查看
											</Link>
										</td>
									</tr>
								);
							})}
						</tbody>
					</table>
				)}
				<div
					style={{
						display: "flex",
						justifyContent: "flex-end",
						gap: 8,
						marginTop: 12,
					}}
				>
					<Button
						variant="outline"
						disabled={currentPage(pageTokens) <= 1 || query.isFetching}
						onClick={() => setPageTokens(previousPage(pageTokens))}
					>
						上一頁
					</Button>
					<Button
						variant="outline"
						disabled={!nextPageToken || query.isFetching}
						onClick={() => setPageTokens(nextPage(pageTokens, nextPageToken))}
					>
						下一頁
					</Button>
				</div>
			</div>
		</div>
	);
}
