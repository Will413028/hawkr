"use client";

import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
	type ContentRef,
	ContentSnapshotState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerContentKind,
	sellerMeasureRemovedContentsCleared,
	sellerMeasureRemovedContentsHeading,
	sellerMeasureViewContentLink,
} from "@hawkr/ui-text/keys";
import Link from "@/components/shared/Link";
import { formatTaipeiDateTime } from "@/lib/date/taipei";
import { optionalText, type UiText, useUiText } from "@/lib/i18n/ui-text";

/** 賣家自己的店，用來組討論串連結。 */
export interface OwnStore {
	id: string;
	storeCode: string;
}

/**
 * 內容下架對象的連結：貼文與貼文留言連到賣家中心自己的貼文（下架中也看得到），討論區留言連到單一討論串頁。賣家點了看不到（`linkAvailable` 為 false，由伺服器依刪除與下架判斷）、
 * 快照不在或沒有連結目標時沒有連結。討論串頁的網址段是店家的 store_code，只有討論串屬於 `store`（賣家自己的店）時
 * 才讀得到，否則沒有連結。
 */
export function contentHref(
	content: ContentRef,
	store?: OwnStore,
): string | undefined {
	if (!content.linkAvailable) return undefined;
	switch (content.link.case) {
		case "postId":
			return `/seller-center/posts?post=${encodeURIComponent(content.link.value)}`;
		case "forumThread":
			if (!store?.storeCode || store.id !== content.link.value.storeId) {
				return undefined;
			}
			return `/sellers/${encodeURIComponent(store.storeCode)}/messages/${encodeURIComponent(content.link.value.threadId)}`;
		default:
			return undefined;
	}
}

/**
 * 措施裡被下架的內容，逐則列出種類、摘要、發文時間與連結。快照已清空（`CLEARED`）時只列種類，
 * 並以目錄文案說明副本已超過保留期限刪除。
 */
export function MeasureContents({
	contents,
	store,
}: {
	contents: readonly ContentRef[];
	store?: OwnStore;
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
						store={store}
					/>
				))}
			</ul>
		</section>
	);
}

function MeasureContentItem({
	ut,
	content,
	store,
}: {
	ut: UiText;
	content: ContentRef;
	store?: OwnStore;
}) {
	const captured = content.snapshotState === ContentSnapshotState.CAPTURED;
	const kind = optionalText(ut, sellerContentKind(content.kind));
	const href = captured ? contentHref(content, store) : undefined;
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
			{!captured && (
				<p className="m-0 text-sm leading-[1.6] text-[var(--hkui-foreground-soft)]">
					{ut(sellerMeasureRemovedContentsCleared())}
				</p>
			)}
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
