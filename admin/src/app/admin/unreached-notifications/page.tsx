"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	NotificationAdminService,
	type UnreachedNotification,
} from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import { notificationNotificationCategory } from "@hawkr/ui-text/keys";
import { useState } from "react";
import { Button, StateRow } from "@/components/UI";
import { audienceText } from "@/lib/announcement";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { staffTextOrUnknown } from "@/lib/ui-text";
import {
	channelResultText,
	UNREACHED_DAYS,
} from "@/lib/unreached-notification";

const PAGE_SIZE = 20;

function UnreachedRow({ item }: { item: UnreachedNotification }) {
	return (
		<tr>
			<td>
				<div className="cell-main">
					{staffTextOrUnknown(notificationNotificationCategory(item.category))}
				</div>
				<div className="cell-sub">{item.type}</div>
			</td>
			<td>
				<div className="cell-main">{audienceText(item.audience)}</div>
				<div className="cell-sub">
					帳號 {String(item.accountId)}
					{item.storeId ? `／店家 ${String(item.storeId)}` : ""}
				</div>
			</td>
			<td>
				{item.channels.map((c) => (
					<div key={c.channel}>
						{channelResultText(c.channel, c.failureReason)}
					</div>
				))}
			</td>
			<td>
				{item.createTime
					? formatDateTimeShort(timestampDate(item.createTime))
					: "-"}
			</td>
		</tr>
	);
}

export default function UnreachedNotificationsPage() {
	const [pageTokens, setPageTokens] = useState(firstPage);
	const list = useQuery(
		NotificationAdminService.method.listUnreachedNotifications,
		{
			days: UNREACHED_DAYS,
			pageSize: PAGE_SIZE,
			pageToken: currentToken(pageTokens),
		},
	);
	const records = list.data?.notifications ?? [];
	const next = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">未觸達通知</h1>
					<p className="page-subtitle">
						近 {UNREACHED_DAYS}{" "}
						天交易必要的通知（使用者不能關閉的分類）中，推播與 email
						都沒有送達的收件人
						{list.data ? `，共 ${String(list.data.unreachedCount)} 筆` : ""}
						。收件匣不算送達；還在待送、刻意不送與收件人已刪除的不計。
					</p>
				</div>
			</div>

			<div className="table-container" style={{ overflowX: "auto" }}>
				<table className="data-table">
					<thead>
						<tr>
							<th>通知</th>
							<th>收件人</th>
							<th>渠道結果</th>
							<th>通知時間</th>
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
							emptyText="近期沒有未觸達的交易必要通知"
							colSpan={4}
							onRetry={() => void list.refetch()}
						>
							{records.map((n) => (
								<UnreachedRow key={n.deliveryId} item={n} />
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
		</div>
	);
}
