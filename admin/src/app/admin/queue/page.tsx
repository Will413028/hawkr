"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import { useQuery } from "@connectrpc/connect-query";
import {
	ReviewQueueAdminService,
	type ReviewQueueItem,
	type ReviewQueueItemKind,
} from "@hawkr/api/gen/hawkr/admin/reviewqueue/v1alpha1/review_queue_pb";
import Link from "next/link";
import { useState } from "react";
import { DeadlineText } from "@/components/UI";
import { deadlineView } from "@/lib/deadline";
import { connectErrorText } from "@/lib/error-text";
import { formatDateTimeShort } from "@/lib/format";
import {
	currentPage,
	currentToken,
	firstPage,
	nextPage,
	previousPage,
} from "@/lib/keyset-pages";
import { LIST_REFRESH_MS, useNow } from "@/lib/use-now";
import {
	itemHref,
	QUEUE_KIND_OPTIONS,
	queueKindText,
	queueStoreText,
} from "./queue-item";
import "@/styles/pages/MerchantManagement.css";
import { MaterialSymbol } from "@hawkr/icons/material-symbol";

const PAGE_SIZE = 20;
const COLUMNS = ["種類", "店家", "開始時間", "期限"] as const;
const ALL_KINDS = "";

/**
 * 待辦：通報與案件、主管機關來函、措施與商家認證中等待處理的事項，依期限由急到緩（排序由伺服器決定，這裡不重排）；
 * 每 30 秒自動更新。篩選是單選的種類，換篩選回第一頁。
 */
export default function ReviewQueuePage() {
	const [kind, setKind] = useState<string>(ALL_KINDS);
	const [pageTokens, setPageTokens] = useState(firstPage);
	const now = useNow(LIST_REFRESH_MS);
	const queue = useQuery(
		ReviewQueueAdminService.method.listReviewQueue,
		{
			kinds: kind === ALL_KINDS ? [] : [Number(kind) as ReviewQueueItemKind],
			pageSize: PAGE_SIZE,
			pageToken: currentToken(pageTokens),
		},
		{ refetchInterval: LIST_REFRESH_MS },
	);
	const rows: ReviewQueueItem[] = queue.data?.items ?? [];
	const next = queue.data?.nextPageToken ?? "";
	const page = currentPage(pageTokens);

	const changeKind = (value: string) => {
		setKind(value);
		setPageTokens(firstPage());
	};

	const renderState = () => {
		if (queue.isLoading) {
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
		if (queue.error) {
			return (
				<div className="merchant-table-state" role="alert">
					<MaterialSymbol name="error" />
					<p>{connectErrorText(queue.error, "待辦讀取失敗")}</p>
					<button
						type="button"
						className="btn btn-sm"
						onClick={() => void queue.refetch()}
					>
						重試
					</button>
				</div>
			);
		}
		return (
			<div className="merchant-table-state">
				<MaterialSymbol name="check_circle" />
				<p>{kind === ALL_KINDS ? "目前沒有待辦" : "沒有這個種類的待辦"}</p>
			</div>
		);
	};

	const renderRow = (item: ReviewQueueItem, index: number) => {
		const deadline = deadlineView(item.deadline, now);
		const href = itemHref(item);
		const label = queueKindText(item.kind);
		return (
			<tr key={`${item.kind}-${href ?? index}`}>
				<td>
					{href ? (
						<Link className="merchant-name" href={href}>
							{label}
						</Link>
					) : (
						label
					)}
				</td>
				<td>{queueStoreText(item)}</td>
				<td>
					<span className="merchant-date">
						{item.deadline?.startTime
							? formatDateTimeShort(timestampDate(item.deadline.startTime))
							: "-"}
					</span>
				</td>
				<td>
					<DeadlineText display={deadline} title={deadline.dueText} />
				</td>
			</tr>
		);
	};

	return (
		<div className="merchant-management">
			<div className="page-header">
				<div>
					<h1 className="page-title">待辦</h1>
					<p className="page-subtitle">
						通報與案件、主管機關來函、措施與商家認證中等待處理的事項，依期限由急到緩。每
						30 秒自動更新。
					</p>
				</div>
			</div>

			<div className="merchant-toolbar">
				<select
					aria-label="種類"
					value={kind}
					onChange={(e) => changeKind(e.target.value)}
				>
					<option value={ALL_KINDS}>所有種類</option>
					{QUEUE_KIND_OPTIONS.map((option) => (
						<option key={option} value={String(option)}>
							{queueKindText(option)}
						</option>
					))}
				</select>
			</div>

			<div className="merchant-table-container">
				<table className="merchant-table">
					<thead>
						<tr>
							{COLUMNS.map((column) => (
								<th key={column}>{column}</th>
							))}
						</tr>
					</thead>
					<tbody>
						{queue.isLoading || queue.error || rows.length === 0 ? (
							<tr>
								<td
									colSpan={COLUMNS.length}
									className="merchant-table-state-cell"
								>
									{renderState()}
								</td>
							</tr>
						) : (
							rows.map(renderRow)
						)}
					</tbody>
				</table>
			</div>

			{(page > 1 || next) && (
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
						onClick={() => setPageTokens((t) => nextPage(t, next))}
						disabled={!next}
					>
						下一頁 <MaterialSymbol name="keyboard_arrow_right" />
					</button>
				</div>
			)}
		</div>
	);
}
