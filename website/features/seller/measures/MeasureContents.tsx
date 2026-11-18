"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type ContentRef,
	ContentSnapshotState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerContentKind,
	sellerMeasureRemovedContentsHeading,
	sellerMeasureViewContentLink,
} from "@hawkr/ui-text/keys";
import Link from "@/components/shared/Link";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { optionalText, type UiText, useUiText } from "@/lib/i18n/ui-text";

/**
 * 內容下架對象的連結：貼文與貼文留言連到貼文，討論區留言連到單一討論串頁。目標已不在（`linkAvailable` 為 false）、
 * 快照不在或沒有連結目標時沒有連結。
 */
export function contentHref(content: ContentRef): string | undefined {
	if (!content.linkAvailable) return undefined;
	switch (content.link.case) {
		case "postId":
			return `/feed?post=${encodeURIComponent(content.link.value)}`;
		case "forumThread":
			return `/sellers/${encodeURIComponent(content.link.value.storeId)}/messages/${encodeURIComponent(content.link.value.threadId)}`;
		default:
			return undefined;
	}
}

/**
 * 措施裡被下架的內容，逐則列出種類、摘要、發文時間與連結。快照不在（`UNAVAILABLE`、`CLEARED`）時只列種類：
 * 這兩種狀態的說明文字還沒有核可的文案。
 */
export function MeasureContents({
	contents,
}: {
	contents: readonly ContentRef[];
}) {
	const ut = useUiText();
	if (contents.length === 0) return null;
	return (
		<section className="flex flex-col gap-2">
			<h3 className="m-0 text-sm font-bold text-[var(--hkui-foreground)]">
				{ut(sellerMeasureRemovedContentsHeading())}
			</h3>
			<ul className="m-0 flex list-none flex-col gap-2 p-0">
				{contents.map((content) => (
					<MeasureContentItem
						key={`${content.kind}:${content.id}`}
						ut={ut}
						content={content}
					/>
				))}
			</ul>
		</section>
	);
}

function MeasureContentItem({
	ut,
	content,
}: {
	ut: UiText;
	content: ContentRef;
}) {
	const captured = content.snapshotState === ContentSnapshotState.CAPTURED;
	const kind = optionalText(ut, sellerContentKind(content.kind));
	const href = captured ? contentHref(content) : undefined;
	return (
		<li
			data-hk-measure-content
			className="flex flex-col gap-1 rounded-[var(--hkui-radius-md)] bg-[var(--hkui-default-100)] p-3"
		>
			<div className="flex flex-wrap items-center justify-between gap-2 text-xs text-[var(--hkui-foreground-muted)]">
				{kind && <span>{kind}</span>}
				{captured && content.authoredTime && (
					<time dateTime={timestampDate(content.authoredTime).toISOString()}>
						{formatTaipeiDateTime(timestampDate(content.authoredTime))}
					</time>
				)}
			</div>
			{captured && content.excerpt && (
				<p className="m-0 whitespace-pre-wrap break-words text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{content.excerpt}
				</p>
			)}
			{href && (
				<Link
					href={href}
					className="self-start text-sm font-bold text-[var(--hkui-primary)]"
				>
					{ut(sellerMeasureViewContentLink())}
				</Link>
			)}
		</li>
	);
}
