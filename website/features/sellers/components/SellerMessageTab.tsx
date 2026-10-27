"use client";

import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useSellerForum } from "@/lib/hooks/useSellerForum";
import type {
	SellerForumComment,
	SellerForumListResult,
} from "@/lib/seller/forum-repo";

interface SellerMessageTabProps {
	sellerId: string;
	initialData?: SellerForumListResult;
}

/**
 * Seller forum / message-board tab — phases 2 + 3 combined.
 *
 * Phase 2 (already shipped):
 *   - Compose (login-gated), top-level list, empty / error / load more.
 *
 * Phase 3 (this update):
 *   - "X 則回覆 ▼" affordance only when replyCount > 0.
 *   - Click expands the comment, lazy-loads `replies[]` via the hook,
 *     re-clicks toggle collapse without refetching.
 *   - "回覆" button per comment puts the compose into reply mode.
 *     A banner above the textarea names the target (with × to cancel);
 *     submit routes through forum.reply(parentId, content). Optimistic
 *     reply lands inside the parent's `replies[]` immediately.
 *
 * Still deferred to later phases:
 *   - Like / edit / delete on each row (phase 4)
 *   - Sticky mobile input, reply-line treatment, date polish (phase 5)
 */
export default function SellerMessageTab({
	sellerId,
	initialData,
}: SellerMessageTabProps) {
	const { isLoggedIn } = useAuth();
	const forum = useSellerForum(sellerId, initialData);
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
	const [replyingTo, setReplyingTo] = React.useState<SellerForumComment | null>(
		null,
	);
	const [expandedIds, setExpandedIds] = React.useState<ReadonlySet<string>>(
		() => new Set(),
	);
	const [loadingReplyIds, setLoadingReplyIds] = React.useState<
		ReadonlySet<string>
	>(() => new Set());
	const composeRef = React.useRef<HTMLTextAreaElement | null>(null);

	const handleSubmit = async (event: React.FormEvent) => {
		event.preventDefault();
		const content = draft.trim();
		if (!content || submitting) return;
		setSubmitting(true);
		try {
			if (replyingTo) {
				await forum.reply(replyingTo.id, content);
				// Reveal the parent's replies so the new one is visible.
				setExpandedIds((prev) => {
					const next = new Set(prev);
					next.add(String(replyingTo.id));
					return next;
				});
				setReplyingTo(null);
			} else {
				await forum.post(content);
			}
			setDraft("");
		} catch {
			/* hook surfaces error via forum.error */
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

	const toggleReplies = async (comment: SellerForumComment) => {
		const id = String(comment.id);
		const isExpanded = expandedIds.has(id);
		// Collapse: no refetch, just hide.
		if (isExpanded) {
			setExpandedIds((prev) => {
				const next = new Set(prev);
				next.delete(id);
				return next;
			});
			return;
		}
		// Expand: if replies aren't loaded yet, fetch them. Optimistic
		// replies count as loaded — the optimistic reply that triggered
		// "auto-expand" already put a record into `comment.replies`.
		const needsFetch = !comment.replies || comment.replies.length === 0;
		if (needsFetch) {
			setLoadingReplyIds((prev) => {
				const next = new Set(prev);
				next.add(id);
				return next;
			});
			try {
				await forum.loadReplies(comment.id);
			} catch {
				/* swallow — user can retry by re-clicking */
			} finally {
				setLoadingReplyIds((prev) => {
					const next = new Set(prev);
					next.delete(id);
					return next;
				});
			}
		}
		setExpandedIds((prev) => {
			const next = new Set(prev);
			next.add(id);
			return next;
		});
	};

	return (
		<section className="flex flex-col gap-5">
			{isLoggedIn ? (
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
							replyingTo ? `回覆 ${replyingTo.userName}…` : "留言給這個賣家…"
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
						loadingLabel={replyingTo ? "送出回覆中…" : "送出留言中…"}
					>
						{replyingTo ? "回覆" : "送出"}
					</Button>
				</form>
			) : (
				<div className="flex items-center justify-between gap-3 p-[14px_18px] rounded-[14px] bg-[var(--surface-card,rgb(var(--color-ink-800-rgb)/0.04))] dark:bg-[rgba(255,255,255,0.04)] text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.72)] text-[0.9375rem]">
					<span>登入後可以留言給這個賣家</span>
					<Link
						href={`/login?redirect=${encodeURIComponent(`/sellers/${sellerId}/messages`)}`}
						hardNavigation
						className="p-[8px_18px] rounded-full bg-[var(--action-primary,var(--color-ink-800))] dark:bg-[rgba(241,205,112,0.85)] text-[var(--color-neutral-cream)] dark:text-[#2d2010] text-[length:var(--text-sm)] font-bold no-underline hover:brightness-[1.08]"
					>
						登入
					</Link>
				</div>
			)}

			{forum.error && !forum.loading ? (
				<div
					className="flex items-center justify-between gap-3 p-3 rounded-[var(--radius-md)] bg-[rgba(196,60,60,0.08)] dark:bg-[rgba(196,60,60,0.18)] text-[#8a3a3a] dark:text-[#f5c4c4] text-[length:var(--text-sm)]"
					role="alert"
				>
					<span>{forum.error}</span>
					<Button
						type="button"
						size="sm"
						className="h-auto p-[6px_14px] rounded-full border-0 bg-[#8a3a3a] dark:bg-[rgba(196,60,60,0.7)] text-[var(--color-neutral-cream)] text-[0.8125rem] cursor-pointer"
						onClick={() => {
							void forum.refresh();
						}}
						disabled={forum.refreshing}
						isLoading={forum.refreshing}
						loadingLabel="重新載入留言中…"
					>
						重新載入
					</Button>
				</div>
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
					{forum.comments.map((comment) => {
						const idStr = String(comment.id);
						const expanded = expandedIds.has(idStr);
						const loadingReplies = loadingReplyIds.has(idStr);
						const replyCount = comment.replyCount ?? 0;
						return (
							<li
								key={`c-${idStr}`}
								data-hk-seller-forum-comment
								className="grid grid-cols-[44px_1fr] gap-3 p-4 rounded-[14px] bg-[var(--surface-card,#fffefa)] border border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.08))] dark:bg-[rgba(255,255,255,0.04)] dark:border-[rgba(245,222,165,0.14)]"
							>
								<Avatar
									src={comment.avatarKey}
									name={comment.userName}
									alt={comment.userName}
									entity="buyer"
									className="w-11 h-11 rounded-full object-cover bg-[var(--hkui-default-200,rgb(var(--color-ink-800-rgb)/0.06))]"
								/>
								<div className="min-w-0">
									<header className="flex items-baseline justify-between gap-[10px] mb-1">
										<strong className="text-[0.9375rem] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)]">
											{comment.userName}
										</strong>
										<time className="text-[length:var(--text-xs)] text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] whitespace-nowrap">
											{formatTime(comment.createdAt)}
										</time>
									</header>
									<p className="m-0 text-[0.9375rem] leading-[1.55] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] whitespace-pre-wrap break-words">
										{comment.content}
									</p>
									<div className="flex items-center gap-4 mt-[10px]">
										{replyCount > 0 ? (
											<Button
												type="button"
												className={`h-auto p-[4px_8px] mx-[-8px] border-0 bg-transparent text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] text-[0.8125rem] font-semibold cursor-pointer rounded-[var(--radius-xs)] transition-[background,color] duration-[120ms] hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.08)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] hover:text-[var(--ink-body)] dark:hover:text-[var(--hk-text-strong,#f5e9c3)] disabled:opacity-60 disabled:cursor-not-allowed ${expanded ? "text-[var(--action-primary,var(--color-ink-800))] dark:text-[#ffeec0]" : ""}`}
												onClick={() => {
													void toggleReplies(comment);
												}}
												disabled={loadingReplies}
												isLoading={loadingReplies}
												loadingLabel="載入回覆中…"
												aria-expanded={expanded}
											>
												{expanded ? "▲ 收合回覆" : `${replyCount} 則回覆 ▼`}
											</Button>
										) : null}
										{isLoggedIn ? (
											<button
												type="button"
												className="p-[4px_8px] mx-[-8px] border-0 bg-transparent text-[var(--ink-soft,var(--color-ink-500))] dark:text-[rgb(var(--color-ink-100-rgb)/0.6)] text-[0.8125rem] font-semibold cursor-pointer rounded-[var(--radius-xs)] transition-[background,color] duration-[120ms] hover:not-disabled:bg-[rgb(var(--color-ink-800-rgb)/0.08)] dark:hover:not-disabled:bg-[rgba(245,222,165,0.08)] hover:text-[var(--ink-body)] dark:hover:text-[var(--hk-text-strong,#f5e9c3)] disabled:opacity-60 disabled:cursor-not-allowed"
												onClick={() => startReply(comment)}
											>
												回覆
											</button>
										) : null}
									</div>
									{expanded && comment.replies && comment.replies.length > 0 ? (
										<ul className="list-none mt-[14px] pt-[14px] border-t border-[var(--surface-border,rgb(var(--color-ink-800-rgb)/0.08))] dark:border-t-[rgba(245,222,165,0.14)] flex flex-col gap-[14px]">
											{comment.replies.map((reply) => (
												<li
													key={`r-${reply.id}`}
													className="grid grid-cols-[32px_1fr] gap-[10px]"
												>
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
														<p className="m-0 text-[length:var(--text-sm)] leading-[1.55] text-[var(--ink-body)] dark:text-[var(--hk-text-strong,#f5e9c3)] whitespace-pre-wrap break-words">
															{reply.replyToName &&
															reply.replyToName !== comment.userName ? (
																<span className="text-[var(--action-primary,var(--color-ink-800))] dark:text-[#ffeec0] font-semibold">
																	@{reply.replyToName}{" "}
																</span>
															) : null}
															{reply.content}
														</p>
													</div>
												</li>
											))}
										</ul>
									) : null}
								</div>
							</li>
						);
					})}
				</ul>
			)}

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
