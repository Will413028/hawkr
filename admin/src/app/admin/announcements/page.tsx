"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	type Announcement,
	NotificationAdminService,
} from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { useState } from "react";
import { Button, StateRow, StatusBadge } from "@/components/UI";
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
import CreateAnnouncementDialog from "./CreateAnnouncementDialog";

const PAGE_SIZE = 20;

function AnnouncementRow({ announcement }: { announcement: Announcement }) {
	return (
		<tr>
			<td>
				<div className="cell-main">{announcement.title}</div>
				<div className="cell-sub" style={{ whiteSpace: "pre-wrap" }}>
					{announcement.body}
				</div>
			</td>
			<td>{announcement.audiences.map(audienceText).join("、") || "-"}</td>
			<td>{announcement.inboxCount}</td>
			<td>
				<StatusBadge variant={announcement.dispatched ? "success" : "warning"}>
					{announcement.dispatched ? "已派送" : "派送中"}
				</StatusBadge>
			</td>
			<td>
				{announcement.createTime
					? formatDateTimeShort(timestampDate(announcement.createTime))
					: "-"}
			</td>
		</tr>
	);
}

export default function AnnouncementsPage() {
	const [pageTokens, setPageTokens] = useState(firstPage);
	const [showCreateDialog, setShowCreateDialog] = useState(false);

	const list = useQuery(NotificationAdminService.method.listAnnouncements, {
		pageSize: PAGE_SIZE,
		pageToken: currentToken(pageTokens),
	});
	const records = list.data?.announcements ?? [];
	const next = list.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	return (
		<div>
			<div className="page-header">
				<div>
					<h1 className="page-title">系統公告</h1>
					<p className="page-subtitle">
						發送給所有買家或賣家的系統公告，收件匣與推播分開派送；公告一律只寫英文。
					</p>
				</div>
				<div className="header-actions">
					<Button
						variant="primary"
						icon={<MaterialSymbol name="campaign" />}
						onClick={() => setShowCreateDialog(true)}
					>
						發送系統公告
					</Button>
				</div>
			</div>

			<div className="table-container" style={{ overflowX: "auto" }}>
				<table className="data-table">
					<thead>
						<tr>
							<th>公告</th>
							<th>發送對象</th>
							<th>收件匣筆數</th>
							<th>派送狀態</th>
							<th>建立時間</th>
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
							emptyText="目前尚未發送任何系統公告"
							colSpan={5}
							onRetry={() => void list.refetch()}
						>
							{records.map((a) => (
								<AnnouncementRow key={a.id} announcement={a} />
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
				<CreateAnnouncementDialog onClose={() => setShowCreateDialog(false)} />
			) : null}
		</div>
	);
}
