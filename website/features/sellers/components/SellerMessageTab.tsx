"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { useTranslations } from "next-intl";
import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import {
	CommentEditor,
	CommentEntryActions,
	CommentErrorBox,
} from "@/features/feed/components/CommentEntryParts";
import { canBlockForumAuthor } from "@/features/feed/components/commentPermissions";
import { useAuth } from "@/lib/auth/context";
import { formatTaipeiDate } from "@/lib/date/taipei";
import type {
	SellerForumComment,
	SellerForumListResult,
} from "@/lib/forum/repo";
import { useForumReplies, useSellerForum } from "@/lib/hooks/useSellerForum";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { blockBuyerByForumEntry } from "@/lib/seller/block-buyer";

interface SellerMessageTabProps {
	/** store_id: the key of every forum RPC. */
	sellerId: string;
	/** The storefront URL segment (store_code), for the login return path. */
	routeId: string;
	initialData?: SellerForumListResult;
	/** 設了是單一討論串頁：只列這一串（頂層項目與回覆），留言框是對這一串回覆。 */
	threadId?: string;
}

/**
 * Seller forum / message-board tab.
 *
 * - Compose (login-gated), top-level list with cursor paging, empty / error states.
 * - Each top-level entry with replies offers "X 則回覆 ▼"; expanding reads its thread
 *   (`useForumReplies`, cursor paging). "回覆" puts the composer into reply mode.
 * - Own just-submitted entries whose server copy is not in the loaded pages show under
 *   the composer (top-level) or at the bottom of the thread (replies), without actions.
 * - Per-row edit / delete / block follow the server's `viewer_can_*` flags.
 */
export default function SellerMessageTab({
	sellerId,
	routeId,
	initialData,
	threadId,
}: SellerMessageTabProps) {
	const { status } = useAuth();
	const tFeed = useTranslations("feed.comments");
	const tSeller = useTranslations("seller.comments");
	const signedIn = status === "signedIn";
	const forum = useSellerForum(sellerId, initialData, threadId);
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
	const [replyingTo, setReplyingTo] = React.useState<SellerForumComment | null>(
		null,
	);
	const [expandedIds, setExpandedIds] = React.useState<ReadonlySet<string>>(
		() => new Set(threadId ? [threadId] : []),
	);
	const defaultErrors = useDefaultErrorText();
	const composeRef = React.useRef<HTMLTextAreaElement | null>(null);
	const [busy, setBusy] = React.useState<{
		id: string;
		kind: "delete" | "block";
	} | null>(null);
	const [blockNotice, setBlockNotice] = React.useState<string | null>(null);
	const [actionError, setActionError] = React.useState<string | null>(null);
	const [blockedAuthorIds, setBlockedAuthorIds] = React.useState<
		ReadonlySet<string>
	>(() => new Set());
	const busyRef = React.useRef(false);

	// 單一討論串頁：沒有指定回覆對象時，留言框是回覆這串的頂層項目（頂層是佔位項目時不能回覆，留言框只在指定回覆對象時出現）。
	const threadTop = threadId
		? forum.comments.find((c) => c.id === threadId && c.viewerCanReply)
		: undefined;
	const target = replyingTo ?? threadTop ?? null;

	const saveEntry = (entry: SellerForumComment, next: string) =>
		forum.update(entry, next);

	const deleteEntry = async (entry: SellerForumComment, isReply: boolean) => {
		if (busyRef.current) return;
		if (!window.confirm(isReply ? "刪除這則回覆？" : "刪除這則留言？")) return;
		busyRef.current = true;
		setBusy({ id: entry.id, kind: "delete" });
		setActionError(null);
		try {
			await forum.remove(entry);
		} catch (e) {
			setActionError(errorText(e, defaultErrors));
		} finally {
			busyRef.current = false;
			setBusy(null);
		}
	};

	const blockAuthor = async (entry: SellerForumComment) => {
		const name = entry.userName;
		if (
			!window.confirm(
				`${tSeller("blockTitle", { name })}\n\n${tSeller("blockDescription")}`,
			)
		)
			return;
		if (busyRef.current) return;
		busyRef.current = true;
		setBusy({ id: entry.id, kind: "block" });
		setBlockNotice(null);
		setActionError(null);
		try {
			await blockBuyerByForumEntry(entry.id);
			setBlockNotice(tSeller("blocked", { name }));
			const authorId = entry.authorId;
			if (authorId) {
				setBlockedAuthorIds((prev) => new Set(prev).add(authorId));
			}
		} catch (e) {
			setActionError(errorText(e, defaultErrors));
		} finally {
			busyRef.current = false;
			setBusy(null);
		}
	};

	const renderBody: RenderBody = (entry, isReply, view) => (
		<ForumEntryBody
			entry={entry}
			isReply={isReply}
			busy={busy?.id === entry.id}
			deleting={busy?.id === entry.id && busy.kind === "delete"}
			canBlock={
				canBlockForumAuthor(entry) &&
				!(entry.authorId && blockedAuthorIds.has(entry.authorId))
			}
			onSave={saveEntry}
			onDelete={deleteEntry}
			onBlock={blockAuthor}
			{...view}
		/>
	);

	const handleSubmit = async (event: React.FormEvent) => {
		event.preventDefault();
		const content = draft.trim();
		if (!content || submitting) return;
		setSubmitting(true);
		setActionError(null);
		try {
			if (target) {
				await forum.reply(target, content);
				// Reveal the thread so the new reply is visible.
				const revealId = String(target.parentId ?? target.id);
				setExpandedIds((prev) => new Set(prev).add(revealId));
				setReplyingTo(null);
			} else {
				await forum.post(content);
			}
			setDraft("");
		} catch (e) {
			setActionError(errorText(e, defaultErrors));
		} finally {
			setSubmitting(false);
		}
	};

	const startReply = (target: SellerForumComment) => {
		setReplyingTo(target);
		// Defer focus to the next paint so the banner has rendered.
		queueMicrotask(() => composeRef.current?.focus());
	};

	const cancelReply = () => {
		setReplyingTo(null);
	};

	const toggleReplies = (id: string) => {
		setExpandedIds((prev) => {
			const next = new Set(prev);
			if (next.has(id)) next.delete(id);
			else next.add(id);
			return next;
		});
	};

	return (
		<section className="flex flex-col gap-5">
			{signedIn ? (
				!threadId || target ? (
					<form
						className="flex flex-col gap-[10px] p-4 rounded-[var(--radius-lg)] bg-[var(--surface-card,#fffefa)] border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.1))] dark:bg-[rgba(255,255,255,0.04)] dark:border-[rgba(245,222,165,0.14)]"
						onSubmit={handleSubmit}
					>
						{replyingTo ? (
							<div className="flex items-center justify-between gap-2 p-[8px_14px] rounded-full bg-[rgb(var(--color-ink-800-rgb)/0.08)] dark:bg-[rgba(241,205,112,0.12)] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] text-[0.8125rem]">
								<span>
									回覆 <strong>@{replyingTo.userName}</strong>
								</span>
								<button
									type="button"
									className="w-6 h-6 rounded-full border-0 bg-transparent text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] text-[length:var(--text-lg)] leading-none cursor-pointer hover:bg-[rgb(var(--color-ink-800-rgb)/0.1)] dark:hover:bg-[rgba(241,205,112,0.14)] hover:text-[var(--ink-body)] dark:hover:text-[var(--hk-text-strong,#f5e9c3)]"
									onClick={cancelReply}
									aria-label="取消回覆"
								>
									×
								</button>
							</div>
						) : null}
						<textarea
							ref={composeRef}
							className="w-full min-h-[64px] p-[10px_12px] rounded-[var(--radius-md)] border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.12))] bg-[var(--hkui-background,#fdfcf6)] dark:bg-[rgba(255,255,255,0.04)] dark:border-[rgba(245,222,165,0.18)] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] font-inherit resize-y focus:outline-none focus:border-[var(--action-primary,var(--color-ink-800))] dark:focus:border-[rgba(241,205,112,0.6)] focus:shadow-[0_0_0_3px_rgb(var(--color-ink-800-rgb)/0.12)] dark:focus:shadow-[0_0_0_3px_rgba(241,205,112,0.18)]"
							placeholder={
								target ? `回覆 ${target.userName}…` : "留言給這個賣家…"
							}
							value={draft}
							onChange={(event) => setDraft(event.target.value)}
							rows={2}
							maxLength={500}
							disabled={submitting}
						/>
						<Button
							type="submit"
							className="self-end h-auto min-w-[96px] p-[10px_22px] rounded-full border-0 bg-[var(--action-primary,var(--color-ink-800))] dark:bg-[rgba(241,205,112,0.85)] text-[var(--color-neutral-cream)] dark:text-[#2d2010] text-[0.9375rem] font-bold cursor-pointer transition-[filter] duration-[160ms] hover:not-disabled:brightness-[1.08] disabled:opacity-45 disabled:cursor-not-allowed"
							disabled={!draft.trim() || submitting}
							isLoading={submitting}
							loadingLabel={target ? "送出回覆中…" : "送出留言中…"}
						>
							{target ? "回覆" : "送出"}
						</Button>
					</form>
				) : null
			) : status === "unknown" ? null : (
				<div className="flex items-center justify-between gap-3 p-[14px_18px] rounded-[14px] bg-[var(--surface-card,rgb(var(--color-ink-800-rgb)/0.04))] dark:bg-[rgba(255,255,255,0.04)] text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.72)] text-[0.9375rem]">
					<span>登入後可以留言給這個賣家</span>
					<Link
						href={`/login?redirect=${encodeURIComponent(`/sellers/${routeId}/messages${threadId ? `/${threadId}` : ""}`)}`}
						hardNavigation
						className="p-[8px_18px] rounded-full bg-[var(--action-primary,var(--color-ink-800))] dark:bg-[rgba(241,205,112,0.85)] text-[var(--color-neutral-cream)] dark:text-[#2d2010] text-[length:var(--text-sm)] font-bold no-underline hover:brightness-[1.08]"
					>
						登入
					</Link>
				</div>
			)}

			{forum.error != null && !forum.loading ? (
				<CommentErrorBox className="flex items-center justify-between gap-3">
					<span>{errorText(forum.error, defaultErrors)}</span>
					<Button
						type="button"
						size="sm"
						className="h-auto p-[6px_14px] rounded-full border-0 bg-[var(--hkui-danger-700)] text-[var(--hkui-danger-foreground)] text-[0.8125rem] cursor-pointer"
						onClick={() => {
							void forum.refresh();
						}}
						disabled={forum.refreshing}
						isLoading={forum.refreshing}
						loadingLabel="重新載入留言中…"
					>
						重新載入
					</Button>
				</CommentErrorBox>
			) : null}

			{blockNotice ? (
				<p
					className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]"
					role="status"
				>
					{blockNotice}
				</p>
			) : null}

			{actionError ? <CommentErrorBox>{actionError}</CommentErrorBox> : null}

			{forum.submitted.length > 0 ? (
				<ul className="list-none m-0 p-0 flex flex-col gap-[18px]">
					{forum.submitted.map(({ key, entry }) => (
						<ForumEntryItem
							key={key}
							comment={entry}
							sellerId={sellerId}
							signedIn={signedIn}
							expanded={false}
							onToggle={toggleReplies}
							onStartReply={startReply}
							renderBody={renderBody}
						/>
					))}
				</ul>
			) : null}

			{forum.loading && forum.comments.length === 0 ? (
				<p className="p-[32px_16px] text-center text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)]">
					載入中…
				</p>
			) : forum.comments.length === 0 ? (
				<div className="p-[32px_16px] text-center text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)]">
					<p className="m-0 mb-[6px] text-[length:var(--text-base)] font-bold text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)]">
						暫無留言
					</p>
					<small className="text-[0.8125rem] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)]">
						留言可以幫助其他買家了解這個賣家
					</small>
				</div>
			) : (
				<ul className="list-none m-0 p-0 flex flex-col gap-[18px]">
					{forum.comments.map((comment) => (
						<ForumEntryItem
							key={comment.optimistic ? comment.id : `c-${comment.id}`}
							comment={comment}
							sellerId={sellerId}
							signedIn={signedIn}
							expanded={expandedIds.has(String(comment.id))}
							onToggle={toggleReplies}
							onStartReply={startReply}
							renderBody={renderBody}
						/>
					))}
				</ul>
			)}

			{forum.loadMoreError != null ? (
				<CommentErrorBox>
					{errorText(forum.loadMoreError, defaultErrors)}
				</CommentErrorBox>
			) : null}

			{forum.hasMore ? (
				<div className="flex justify-center pt-2">
					<Button
						type="button"
						className="h-auto min-w-[160px] p-[10px_24px] rounded-full border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.18))] dark:border-[rgba(245,222,165,0.22)] bg-transparent text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] text-[length:var(--text-sm)] font-semibold cursor-pointer hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.06)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] disabled:opacity-50 disabled:cursor-not-allowed"
						disabled={forum.loadingMore}
						isLoading={forum.loadingMore}
						loadingLabel="載入留言中…"
						onClick={() => {
							void forum.loadMore();
						}}
					>
						載入更多留言
					</Button>
				</div>
			) : null}
		</section>
	);
}

type RenderBody = (
	entry: SellerForumComment,
	isReply: boolean,
	view: {
		text: React.ReactNode;
		paragraphClassName: string;
		footerClassName: string;
		leading?: React.ReactNode;
	},
) => React.ReactNode;

/** 一則頂層留言與它的回覆串；回覆串展開時才讀，自己剛送出的回覆接在串的最後。 */
function ForumEntryItem({
	comment,
	sellerId,
	signedIn,
	expanded,
	onToggle,
	onStartReply,
	renderBody,
}: {
	comment: SellerForumComment;
	sellerId: string;
	signedIn: boolean;
	expanded: boolean;
	onToggle: (id: string) => void;
	onStartReply: (target: SellerForumComment) => void;
	renderBody: RenderBody;
}) {
	const tFeed = useTranslations("feed.comments");
	const defaultErrors = useDefaultErrorText();
	const idStr = String(comment.id);
	const replies = useForumReplies(
		sellerId,
		idStr,
		comment.userName,
		expanded && !comment.optimistic,
	);
	const loadingReplies = expanded && replies.loading;
	const replyCount = comment.replyCount ?? 0;
	const isPlaceholder = comment.state !== "visible";
	const replyRows = [
		...replies.items.map((reply) => ({
			rowKey: reply.optimistic ? reply.id : `r-${reply.id}`,
			reply,
		})),
		...replies.submitted.map(({ key, entry }) => ({
			rowKey: key,
			reply: entry,
		})),
	];
	return (
		<li
			data-hk-seller-forum-comment
			className="grid grid-cols-[44px_1fr] gap-3 p-4 rounded-[14px] bg-[var(--surface-card,#fffefa)] border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.08))] dark:bg-[rgba(255,255,255,0.04)] dark:border-[rgba(245,222,165,0.14)]"
		>
			{isPlaceholder ? (
				<span aria-hidden="true" />
			) : (
				<Avatar
					src={comment.avatarKey}
					name={comment.userName}
					alt={comment.userName}
					entity="buyer"
					className="w-11 h-11 rounded-full object-cover bg-[var(--hkui-default-200,rgb(var(--color-ink-800-rgb)/0.06))]"
				/>
			)}
			<div className="min-w-0">
				<header className="flex items-baseline justify-between gap-[10px] mb-1">
					{isPlaceholder ? (
						<span />
					) : (
						<strong className="text-[0.9375rem] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)]">
							{comment.userName}
						</strong>
					)}
					<time className="text-[length:var(--text-xs)] text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] whitespace-nowrap">
						{formatTime(comment.createdAt)}
					</time>
				</header>
				{renderBody(comment, false, {
					text: isPlaceholder
						? comment.state === "deleted"
							? tFeed("deleted")
							: comment.state === "hidden"
								? tFeed("hidden")
								: null
						: comment.content,
					paragraphClassName:
						"m-0 text-[0.9375rem] leading-[1.55] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] whitespace-pre-wrap break-words",
					footerClassName: "flex items-center gap-4 mt-[10px]",
					leading: (
						<>
							{replyCount > 0 ? (
								<Button
									type="button"
									className={`h-auto p-[4px_8px] mx-[-8px] border-0 bg-transparent text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] text-[0.8125rem] font-semibold cursor-pointer rounded-[var(--radius-xs)] transition-[background,color] duration-[120ms] hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.08)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] hover:text-[var(--ink-body)] dark:hover:text-[var(--hk-text-strong,#f5e9c3)] disabled:opacity-60 disabled:cursor-not-allowed ${expanded ? "text-[var(--action-primary,var(--color-ink-800))] dark:text-[#ffeec0]" : ""}`}
									onClick={() => {
										onToggle(idStr);
									}}
									disabled={loadingReplies}
									isLoading={loadingReplies}
									loadingLabel="載入回覆中…"
									aria-expanded={expanded}
								>
									{expanded ? "▲ 收合回覆" : `${replyCount} 則回覆 ▼`}
								</Button>
							) : null}
							{signedIn &&
							comment.state === "visible" &&
							!comment.optimistic ? (
								<button
									type="button"
									className="p-[4px_8px] mx-[-8px] border-0 bg-transparent text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] text-[0.8125rem] font-semibold cursor-pointer rounded-[var(--radius-xs)] transition-[background,color] duration-[120ms] hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.08)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] hover:text-[var(--ink-body)] dark:hover:text-[var(--hk-text-strong,#f5e9c3)] disabled:opacity-60 disabled:cursor-not-allowed"
									onClick={() => onStartReply(comment)}
								>
									回覆
								</button>
							) : null}
						</>
					),
				})}
				{expanded && replyRows.length > 0 ? (
					<ul className="list-none mt-[14px] pt-[14px] border-t border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.08))] dark:border-t-[rgba(245,222,165,0.14)] flex flex-col gap-[14px]">
						{replyRows.map(({ rowKey, reply }) => (
							<li key={rowKey} className="grid grid-cols-[32px_1fr] gap-[10px]">
								<Avatar
									src={reply.avatarKey}
									name={reply.userName}
									alt={reply.userName}
									entity="buyer"
									className="w-8 h-8 rounded-full object-cover bg-[var(--hkui-default-200,rgb(var(--color-ink-800-rgb)/0.06))]"
								/>
								<div className="min-w-0">
									<header className="flex items-baseline justify-between gap-[10px] mb-1">
										<strong className="text-[0.9375rem] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)]">
											{reply.userName}
										</strong>
										<time className="text-[length:var(--text-xs)] text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] whitespace-nowrap">
											{formatTime(reply.createdAt)}
										</time>
									</header>
									{renderBody(reply, true, {
										text: (
											<>
												{reply.replyToName &&
												reply.replyToName !== comment.userName ? (
													<span className="text-[var(--action-primary,var(--color-ink-800))] dark:text-[#ffeec0] font-semibold">
														@{reply.replyToName}{" "}
													</span>
												) : null}
												{reply.content}
											</>
										),
										paragraphClassName:
											"m-0 text-[length:var(--text-sm)] leading-[1.55] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] whitespace-pre-wrap break-words",
										footerClassName: "flex flex-wrap gap-3 mt-1",
									})}
								</div>
							</li>
						))}
					</ul>
				) : null}
				{expanded && replies.error != null ? (
					<CommentErrorBox>
						{errorText(replies.error, defaultErrors)}
					</CommentErrorBox>
				) : null}
				{expanded && replies.loadMoreError != null ? (
					<CommentErrorBox>
						{errorText(replies.loadMoreError, defaultErrors)}
					</CommentErrorBox>
				) : null}
				{expanded && replies.hasMore ? (
					<div className="flex justify-center pt-2">
						<Button
							type="button"
							className="h-auto min-w-[160px] p-[10px_24px] rounded-full border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.18))] dark:border-[rgba(245,222,165,0.22)] bg-transparent text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] text-[length:var(--text-sm)] font-semibold cursor-pointer hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.06)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] disabled:opacity-50 disabled:cursor-not-allowed"
							disabled={replies.loadingMore}
							isLoading={replies.loadingMore}
							loadingLabel="載入回覆中…"
							onClick={() => {
								void replies.loadMore();
							}}
						>
							載入更多留言
						</Button>
					</div>
				) : null}
			</div>
		</li>
	);
}

function formatTime(unixSeconds: number): string {
	if (!unixSeconds) return "";
	const ms = unixSeconds < 1e12 ? unixSeconds * 1000 : unixSeconds;
	const date = new Date(ms);
	const diff = Date.now() - date.getTime();
	const minutes = Math.floor(diff / 60_000);
	if (minutes < 1) return "剛剛";
	if (minutes < 60) return `${minutes} 分鐘前`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours} 小時前`;
	const days = Math.floor(hours / 24);
	if (days < 7) return `${days} 天前`;
	return formatTaipeiDate(date);
}

/** 一則發言的內文、就地編輯與操作列；編輯狀態在每一列自己身上，開著一列不影響其他列。 */
function ForumEntryBody({
	entry,
	isReply,
	busy,
	deleting,
	canBlock,
	text,
	paragraphClassName,
	footerClassName,
	leading,
	onSave,
	onDelete,
	onBlock,
}: {
	entry: SellerForumComment;
	isReply: boolean;
	busy: boolean;
	deleting: boolean;
	canBlock: boolean;
	text: React.ReactNode;
	paragraphClassName: string;
	footerClassName: string;
	leading?: React.ReactNode;
	onSave: (entry: SellerForumComment, next: string) => Promise<void>;
	onDelete: (entry: SellerForumComment, isReply: boolean) => Promise<void>;
	onBlock: (entry: SellerForumComment) => Promise<void>;
}) {
	const [editing, setEditing] = React.useState(false);
	const [draft, setDraft] = React.useState("");
	const [saving, setSaving] = React.useState(false);
	const [editError, setEditError] = React.useState<string | null>(null);
	const defaultErrors = useDefaultErrorText();
	const sessionRef = React.useRef(0);
	const live = entry.state === "visible" && !entry.optimistic;

	const startEdit = () => {
		sessionRef.current += 1;
		setDraft(entry.content);
		setEditError(null);
		setEditing(true);
	};
	const cancelEdit = () => {
		sessionRef.current += 1;
		setEditing(false);
		setDraft("");
		setEditError(null);
	};
	const submit = async () => {
		const next = draft.trim();
		if (!next || saving) return;
		if (next === entry.content) {
			setEditing(false);
			return;
		}
		const session = sessionRef.current;
		setSaving(true);
		setEditError(null);
		try {
			await onSave(entry, next);
			if (sessionRef.current === session) setEditing(false);
		} catch (e) {
			if (sessionRef.current === session) {
				setEditError(errorText(e, defaultErrors));
			}
		} finally {
			setSaving(false);
		}
	};

	return (
		<>
			{editing ? (
				<CommentEditor
					draft={draft}
					onDraftChange={setDraft}
					onSubmit={() => void submit()}
					onCancel={cancelEdit}
					saving={saving}
					saveLoadingLabel={isReply ? "儲存回覆中…" : "儲存留言中…"}
				/>
			) : (
				<p className={paragraphClassName}>{text}</p>
			)}
			{editError ? (
				<CommentErrorBox className="mt-1">{editError}</CommentErrorBox>
			) : null}
			<div className={footerClassName}>
				{leading}
				{live && !editing ? (
					<CommentEntryActions
						canEdit={entry.viewerCanEdit}
						canDelete={entry.viewerCanDelete}
						canBlock={canBlock}
						editDisabled={busy}
						deleteDisabled={busy}
						blockDisabled={busy}
						deleting={deleting}
						deleteLoadingLabel={isReply ? "刪除回覆中…" : "刪除留言中…"}
						onEdit={startEdit}
						onDelete={() => void onDelete(entry, isReply)}
						onBlock={() => void onBlock(entry)}
					/>
				) : null}
			</div>
		</>
	);
}
