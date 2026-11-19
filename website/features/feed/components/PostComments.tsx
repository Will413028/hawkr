"use client";

import { Code, ConnectError } from "@connectrpc/connect";
import { errorText } from "@hawkr/api/connect/errors";
import { gsap } from "gsap";
import { useTranslations } from "next-intl";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import Button from "@/components/ui/Button";
import {
	CommentEditor,
	CommentEntryActions,
	CommentErrorBox,
} from "@/features/feed/components/CommentEntryParts";
import type { PostViewerState } from "@/lib/api/openapi-types";
import { useAuth } from "@/lib/auth/context";
import {
	type Comment,
	deleteComment,
	setCommentLiked,
	updateComment,
} from "@/lib/comments/repo";
import { formatTaipeiShortDate } from "@/lib/date/taipei";
import {
	useCommentReplies,
	usePostComments,
	usePostViewerState,
} from "@/lib/hooks/useSocial";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { blockBuyerByComment } from "@/lib/seller/block-buyer";
import {
	canBlockCommentAuthor,
	canComposeComment,
	canDeleteComment,
	canEditComment,
	canReplyToComment,
	storeActorName,
} from "./commentPermissions";

function isNotFound(error: unknown): boolean {
	return ConnectError.from(error).code === Code.NotFound;
}

function formatTime(unixSeconds: number): string {
	if (!unixSeconds) return "";
	const ms = unixSeconds < 1e12 ? unixSeconds * 1000 : unixSeconds;
	const d = new Date(ms);
	const diff = Date.now() - d.getTime();
	const min = Math.floor(diff / 60_000);
	if (min < 1) return "剛剛";
	if (min < 60) return `${min} 分鐘前`;
	const hr = Math.floor(min / 60);
	if (hr < 24) return `${hr} 小時前`;
	const day = Math.floor(hr / 24);
	if (day < 7) return `${day} 天前`;
	return formatTaipeiShortDate(ms);
}

/* ── deep-link to a specific comment (通知「跳到該留言」) ──────────────────
 * A notification deep link lands on `…?post=<id>&comment=<id>`. Once the post
 * detail opens, the matching `<PostComments>` reads those params and reveals
 * the target row. Comments load async (REST + SSE) and replies lazy-expand, so
 * we poll for the anchor briefly, then fall back to the comment section. */

/** Nearest scrollable ancestor — the surface that actually scrolls around this
 *  comments list (the viewer aside, the bottom sheet body, the seller modal).
 *  Used instead of `scrollIntoView`, which would walk the whole box chain and
 *  jolt the feed pager surface (see PostDetailViewer's handleRequestComments). */
function findScrollParent(el: HTMLElement): HTMLElement | null {
	let node = el.parentElement;
	while (node) {
		const style = window.getComputedStyle(node);
		if (
			(style.overflowY === "auto" || style.overflowY === "scroll") &&
			node.scrollHeight > node.clientHeight + 1
		) {
			return node;
		}
		node = node.parentElement;
	}
	return null;
}

function revealInScrollParent(el: HTMLElement, offset = 80) {
	const parent = findScrollParent(el);
	if (!parent) {
		el.scrollIntoView({ behavior: "smooth", block: "center" });
		return;
	}
	const parentTop = parent.getBoundingClientRect().top;
	const elTop = el.getBoundingClientRect().top;
	parent.scrollTo({
		top: parent.scrollTop + (elTop - parentTop) - offset,
		behavior: "smooth",
	});
}

/** Gentle primary-tint flash so the eye lands on the right row, then fades out.
 *  Colour matches the feed's primary tint; radius is temporary so the pulse
 *  reads as a rounded highlight without leaving layout changes behind. */
function highlightComment(el: HTMLElement) {
	const prevRadius = el.style.borderRadius;
	el.style.borderRadius = "14px";
	gsap.fromTo(
		el,
		{ backgroundColor: "rgb(var(--color-brand-primary-strong-rgb) / 0.18)" },
		{
			backgroundColor: "rgb(var(--color-brand-primary-strong-rgb) / 0)",
			duration: 2.4,
			ease: "power2.out",
			onComplete: () => {
				el.style.borderRadius = prevRadius;
				el.style.backgroundColor = "";
			},
		},
	);
}

interface PostCommentsProps {
	postId: string;
	/**
	 * 這則貼文依觀看者算的留言權限與身分（`Post.viewer_can_comment`、`viewer_actor`）。貼文已經是依觀看者讀的
	 * （賣家中心的店主清單）時傳進來；不傳就由元件向伺服器讀單篇。
	 */
	viewer?: PostViewerState;
	/**
	 * 賣家中心的留言區：別人寫的、店家能刪的留言多一個「封鎖此人」（呼叫賣家的 `BlockBuyer`）。
	 * 公開頁不傳：封鎖名單屬於店家的賣家中心。
	 */
	allowBlockAuthor?: boolean;
	/**
	 * Whether the comment SSE stream should be open for this post. Single-
	 * post surfaces (bottom sheet, seller post modal) leave the default
	 * `true`; the snap pager passes `active={isActive}` so only the
	 * currently-shown page holds a live connection.
	 */
	active?: boolean;
}

/** Avatar shown next to a composer (new comment / reply). It follows the
 *  server's `viewer_actor`: the store logo when the viewer comments as the
 *  shop, otherwise the member's own avatar. */
type ComposerAvatar = {
	src?: string | null;
	alt: string;
	entity: "buyer" | "seller";
};

/** Inline reply composer shown under a comment when the viewer taps 回覆. */
function ReplyComposer({
	placeholder,
	avatar,
	onSubmit,
	onCancel,
}: {
	/** 提示字：店家身分時是「以「店名」身分回覆…」，否則依被回覆者。 */
	placeholder: string;
	avatar: ComposerAvatar;
	onSubmit: (content: string) => Promise<void>;
	onCancel: () => void;
}) {
	const defaultErrors = useDefaultErrorText();
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const textareaRef = React.useRef<HTMLTextAreaElement | null>(null);

	React.useEffect(() => {
		textareaRef.current?.focus();
	}, []);

	const submit = async () => {
		const content = draft.trim();
		if (!content || submitting) return;
		setSubmitting(true);
		setError(null);
		try {
			await onSubmit(content);
			setDraft("");
		} catch (e) {
			setError(errorText(e, defaultErrors));
		} finally {
			setSubmitting(false);
		}
	};

	const onFormSubmit = (event: React.FormEvent) => {
		event.preventDefault();
		void submit();
	};

	// Enter 送出、Shift+Enter 換行；IME 組字中（注音/中文）跳過避免送出半字
	const onKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
		if (
			event.key === "Enter" &&
			!event.shiftKey &&
			!event.nativeEvent.isComposing
		) {
			event.preventDefault();
			void submit();
		}
	};

	return (
		<form
			className="flex gap-[var(--hkui-space-3)] items-start mt-[var(--hkui-space-2)]"
			onSubmit={onFormSubmit}
		>
			<span className="block w-7 h-7 shrink-0 rounded-full overflow-hidden bg-[var(--hkui-default-100)]">
				<Avatar
					src={avatar.src}
					alt={avatar.alt}
					entity={avatar.entity}
					className="w-full h-full object-cover"
					fallback={avatar.entity === "seller" ? "empty" : "placeholder"}
				/>
			</span>
			<div className="flex-1 flex flex-col gap-[var(--hkui-space-2)]">
				<textarea
					ref={textareaRef}
					className="w-full border border-divider rounded-hkui-md py-2 px-3 bg-content1 min-h-10 resize-none focus:outline-none focus:border-hkui-primary [font-family:inherit] [font-weight:inherit] leading-5 text-[length:var(--hkui-text-small)]"
					value={draft}
					onChange={(e) => setDraft(e.target.value)}
					onKeyDown={onKeyDown}
					placeholder={placeholder}
					rows={1}
					maxLength={500}
				/>
				{error ? <CommentErrorBox>{error}</CommentErrorBox> : null}
				<div className="flex gap-[var(--hkui-space-2)] self-end">
					<Button
						type="button"
						variant="bordered"
						color="default"
						size="sm"
						onClick={onCancel}
					>
						取消
					</Button>
					<Button
						type="submit"
						size="sm"
						disabled={!draft.trim() || submitting}
						isLoading={submitting}
						loadingLabel="送出回覆中…"
					>
						回覆
					</Button>
				</div>
			</div>
		</form>
	);
}

/** 送出中或伺服器那份還不在已載入頁的留言與回覆：只顯示，沒有任何操作。 */
function OptimisticCommentRow({
	comment,
	variant,
}: {
	comment: Comment;
	variant: "comment" | "reply";
}) {
	const isReply = variant === "reply";
	return (
		<li
			className={`flex ${isReply ? "gap-[var(--hkui-space-2)]" : "gap-[var(--hkui-space-3)]"}`}
			data-comment-anchor={comment.id}
			data-comment-optimistic
		>
			<span
				className={`block ${isReply ? "w-7 h-7" : "w-9 h-9"} shrink-0 rounded-full overflow-hidden bg-[var(--hkui-default-100)]`}
			>
				<Avatar
					src={comment.avatarKey}
					alt={comment.userName}
					entity="buyer"
					className="w-full h-full object-cover"
					fallbackClassName="block w-full h-full bg-[var(--hkui-primary-100)]"
					fallback="empty"
				/>
			</span>
			<div className="flex-1 min-w-0">
				<header className="flex items-baseline gap-[var(--hkui-space-2)]">
					<strong className="text-[length:var(--hkui-text-small)] font-semibold">
						{comment.userName}
					</strong>
					<small className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
						{formatTime(comment.createdAt)}
					</small>
				</header>
				<p className="my-1 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-medium)] whitespace-pre-wrap break-words">
					{isReply && comment.replyToName ? (
						<span className="text-[var(--hkui-foreground-soft)] mr-1">
							@{comment.replyToName}
						</span>
					) : null}
					{comment.content}
				</p>
			</div>
		</li>
	);
}

/** A reply row under a parent comment — same layout as CommentItem but tighter,
 *  no nested replies (we only render one thread level deep). */
function ReplyItem({
	reply,
	onRemoved,
	onBlockAuthor,
	blockBusy = false,
}: {
	reply: Comment;
	onRemoved: (id: string) => void;
	onBlockAuthor?: (comment: Comment) => Promise<void>;
	blockBusy?: boolean;
}) {
	const [liked, setLiked] = React.useState(reply.isLiked === true);
	const [likeCount, setLikeCount] = React.useState(reply.likeCount ?? 0);
	// Local content mirror — optimistic edit shows immediately, refresh from
	// parent list will overwrite if backend differs.
	const [content, setContent] = React.useState(reply.content);
	// 修改成功後伺服器換新版本：之後的修改與刪除要帶新的 etag。
	const [etag, setEtag] = React.useState(reply.etag);
	const [editing, setEditing] = React.useState(false);
	const [editDraft, setEditDraft] = React.useState("");
	const [savingEdit, setSavingEdit] = React.useState(false);
	const [likePendingAction, setLikePendingAction] = React.useState<
		"like" | "unlike" | null
	>(null);
	const [deleting, setDeleting] = React.useState(false);
	const [rowError, setRowError] = React.useState<string | null>(null);
	const defaultErrors = useDefaultErrorText();
	const likePendingRef = React.useRef(false);
	const deletePendingRef = React.useRef(false);

	const live = reply.state === "visible";
	const canDelete = live && canDeleteComment(reply);
	const canEdit = live && canEditComment(reply);
	const canBlock = live && onBlockAuthor && canBlockCommentAuthor(reply);

	const handleLike = async () => {
		if (likePendingRef.current || deletePendingRef.current) return;
		likePendingRef.current = true;
		const wasLiked = liked;
		setLikePendingAction(wasLiked ? "unlike" : "like");
		setLiked(!wasLiked);
		setLikeCount((c) => c + (wasLiked ? -1 : 1));
		try {
			const r = await setCommentLiked(reply.id, !wasLiked);
			setLiked(r.liked);
		} catch {
			setLiked(wasLiked);
			setLikeCount((c) => c + (wasLiked ? 1 : -1));
		} finally {
			likePendingRef.current = false;
			setLikePendingAction(null);
		}
	};

	const handleDelete = async () => {
		if (deletePendingRef.current || likePendingRef.current) return;
		if (!window.confirm("刪除這則回覆？")) return;
		deletePendingRef.current = true;
		setDeleting(true);
		setRowError(null);
		try {
			await deleteComment(reply.id, etag);
			onRemoved(reply.id);
		} catch (e) {
			setRowError(errorText(e, defaultErrors));
		} finally {
			deletePendingRef.current = false;
			setDeleting(false);
		}
	};

	const startEdit = () => {
		setEditDraft(content);
		setRowError(null);
		setEditing(true);
	};

	const cancelEdit = () => {
		setEditing(false);
		setEditDraft("");
		setRowError(null);
	};

	const submitEdit = async () => {
		const next = editDraft.trim();
		if (!next || savingEdit) return;
		if (next === content) {
			setEditing(false);
			return;
		}
		setSavingEdit(true);
		setRowError(null);
		try {
			const updated = await updateComment(reply.id, next, etag ?? "");
			setEtag(updated.etag);
			setContent(updated.content || next);
			setEditing(false);
		} catch (e) {
			setRowError(errorText(e, defaultErrors));
		} finally {
			setSavingEdit(false);
		}
	};

	return (
		<li
			className="flex gap-[var(--hkui-space-2)]"
			data-comment-anchor={String(reply.id)}
		>
			<span className="block w-7 h-7 shrink-0 rounded-full overflow-hidden bg-[var(--hkui-default-100)]">
				<Avatar
					src={reply.avatarKey}
					alt={reply.userName}
					entity="buyer"
					className="w-full h-full object-cover"
					fallbackClassName="block w-full h-full bg-[var(--hkui-primary-100)]"
					fallback="empty"
				/>
			</span>
			<div className="flex-1 min-w-0">
				<header className="flex items-baseline gap-[var(--hkui-space-2)]">
					<strong className="text-[length:var(--hkui-text-small)] font-semibold">
						{reply.userName}
					</strong>
					<small className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
						{formatTime(reply.createdAt)}
					</small>
				</header>
				{editing ? (
					<CommentEditor
						draft={editDraft}
						onDraftChange={setEditDraft}
						onSubmit={() => void submitEdit()}
						onCancel={cancelEdit}
						saving={savingEdit}
						saveLoadingLabel="儲存回覆中…"
					/>
				) : (
					<p className="my-1 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-medium)] whitespace-pre-wrap break-words">
						{reply.replyToName ? (
							<span className="text-[var(--hkui-foreground-soft)] mr-1">
								@{reply.replyToName}
							</span>
						) : null}
						{content}
					</p>
				)}
				{rowError ? (
					<CommentErrorBox className="my-1">{rowError}</CommentErrorBox>
				) : null}
				<footer className="flex flex-wrap gap-[var(--hkui-space-3)] mt-1">
					{live ? (
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							aria-pressed={liked}
							/* Liked reds come from `text-hk-like`, not `color="danger"` —
						   danger is 柿橘 orange and is the error role. */
							className={`h-auto min-w-0 gap-1 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)] ${liked ? "font-bold text-hk-like" : "font-normal text-[var(--hkui-foreground-soft)]"}`}
							disabled={deleting}
							/* Optimistic-only: `handleLike` flips the shared heart before
						   the request and rolls back on failure; `likePendingRef` drops
						   overlapping taps, so the fill animation is the feedback. */
							aria-busy={likePendingAction !== null || undefined}
							onClick={() => void handleLike()}
						>
							<AppIcon name="heart" fill={liked} size={16} />
							<span className="sr-only">按讚</span>
							<span>{likeCount}</span>
						</Button>
					) : null}
					{!editing && (
						<CommentEntryActions
							canEdit={canEdit}
							canDelete={canDelete}
							canBlock={Boolean(canBlock)}
							editDisabled={likePendingAction !== null || deleting}
							deleteDisabled={likePendingAction !== null}
							blockDisabled={deleting || blockBusy}
							deleting={deleting}
							deleteLoadingLabel="刪除回覆中…"
							onEdit={startEdit}
							onDelete={() => void handleDelete()}
							onBlock={() => void onBlockAuthor?.(reply)}
						/>
					)}
				</footer>
			</div>
		</li>
	);
}

/** A top-level comment row with its reply composer + reply thread. */
function CommentItem({
	comment,
	postId,
	replyPlaceholder,
	composerAvatar,
	onLike,
	onRemove,
	onEdited,
	onBlockAuthor,
	blockBusy = false,
}: {
	comment: Comment;
	postId: string;
	/** 店家身分時的回覆提示字；不是店家身分時是 undefined，改用依被回覆者的提示。 */
	replyPlaceholder?: string;
	composerAvatar: ComposerAvatar;
	onLike: (id: string) => Promise<void>;
	onRemove: (id: string) => Promise<void>;
	onEdited: (updated: Comment) => void;
	onBlockAuthor?: (comment: Comment) => Promise<void>;
	blockBusy?: boolean;
}) {
	const tFeed = useTranslations("feed.comments");
	const defaultErrors = useDefaultErrorText();
	const replies = useCommentReplies(postId, comment.id, comment.userName);
	const [replyOpen, setReplyOpen] = React.useState(false);
	const replyCount = comment.replyCount ?? 0;
	const [rowError, setRowError] = React.useState<string | null>(null);
	// Local content mirror — optimistic edit shows immediately
	const [content, setContent] = React.useState(comment.content);
	const [etag, setEtag] = React.useState(comment.etag);
	const [editing, setEditing] = React.useState(false);
	const [editDraft, setEditDraft] = React.useState("");
	const [savingEdit, setSavingEdit] = React.useState(false);
	const [likePendingAction, setLikePendingAction] = React.useState<
		"like" | "unlike" | null
	>(null);
	const [deleting, setDeleting] = React.useState(false);
	const likePendingRef = React.useRef(false);
	const deletePendingRef = React.useRef(false);

	const live = comment.state === "visible";
	const canDelete = live && canDeleteComment(comment);
	const canEdit = live && canEditComment(comment);
	const canReply = live && canReplyToComment(comment);
	const canBlock = live && onBlockAuthor && canBlockCommentAuthor(comment);

	const handleLike = async () => {
		if (likePendingRef.current || deletePendingRef.current) return;
		likePendingRef.current = true;
		setLikePendingAction(comment.isLiked ? "unlike" : "like");
		try {
			await onLike(comment.id);
		} finally {
			likePendingRef.current = false;
			setLikePendingAction(null);
		}
	};

	const handleDelete = async () => {
		if (deletePendingRef.current || likePendingRef.current) return;
		if (!window.confirm("刪除這則留言？")) return;
		deletePendingRef.current = true;
		setDeleting(true);
		setRowError(null);
		try {
			await onRemove(comment.id);
		} catch (e) {
			setRowError(errorText(e, defaultErrors));
		} finally {
			deletePendingRef.current = false;
			setDeleting(false);
		}
	};

	const startEdit = () => {
		setEditDraft(content);
		setRowError(null);
		setEditing(true);
	};

	const cancelEdit = () => {
		setEditing(false);
		setEditDraft("");
		setRowError(null);
	};

	const submitEdit = async () => {
		const next = editDraft.trim();
		if (!next || savingEdit) return;
		if (next === content) {
			setEditing(false);
			return;
		}
		setSavingEdit(true);
		setRowError(null);
		try {
			const updated = await updateComment(comment.id, next, etag ?? "");
			setEtag(updated.etag);
			setContent(updated.content || next);
			onEdited(updated);
			setEditing(false);
		} catch (e) {
			setRowError(errorText(e, defaultErrors));
		} finally {
			setSavingEdit(false);
		}
	};

	const sendReply = async (content: string) => {
		await replies.reply(content, {
			replyToId: comment.id,
			replyToName: comment.userName,
			authorName: composerAvatar.alt,
			authorAvatar: composerAvatar.src ?? undefined,
		});
		setReplyOpen(false);
		if (!replies.open) replies.expand();
	};

	return (
		<li
			className="flex gap-[var(--hkui-space-3)]"
			data-comment-anchor={String(comment.id)}
		>
			<span className="block w-9 h-9 shrink-0 rounded-full overflow-hidden bg-[var(--hkui-default-100)]">
				{live ? (
					<Avatar
						src={comment.avatarKey}
						alt={comment.userName}
						entity="buyer"
						className="w-full h-full object-cover"
						fallbackClassName="block w-full h-full bg-[var(--hkui-primary-100)]"
						fallback="empty"
					/>
				) : null}
			</span>
			<div className="flex-1 min-w-0">
				<header className="flex items-baseline gap-[var(--hkui-space-2)]">
					{live ? (
						<strong className="text-[length:var(--hkui-text-small)] font-semibold">
							{comment.userName}
						</strong>
					) : null}
					<small className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
						{formatTime(comment.createdAt)}
					</small>
				</header>
				{editing ? (
					<CommentEditor
						draft={editDraft}
						onDraftChange={setEditDraft}
						onSubmit={() => void submitEdit()}
						onCancel={cancelEdit}
						saving={savingEdit}
						saveLoadingLabel="儲存留言中…"
					/>
				) : (
					<p className="my-1 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-medium)] whitespace-pre-wrap break-words">
						{live
							? content
							: comment.state === "deleted"
								? tFeed("deleted")
								: comment.state === "hidden"
									? tFeed("hidden")
									: null}
					</p>
				)}
				{rowError ? (
					<CommentErrorBox className="my-1">{rowError}</CommentErrorBox>
				) : null}
				<footer className="flex flex-wrap gap-[var(--hkui-space-3)] mt-1">
					{live ? (
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							aria-pressed={comment.isLiked === true}
							/* See the reply row's note — liked red is `text-hk-like`. */
							className={`h-auto min-w-0 gap-1 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)] ${comment.isLiked ? "font-bold text-hk-like" : "font-normal text-[var(--hkui-foreground-soft)]"}`}
							disabled={deleting}
							/* Optimistic-only — see the reply row's note above. */
							aria-busy={likePendingAction !== null || undefined}
							onClick={() => void handleLike()}
						>
							<AppIcon name="heart" fill={comment.isLiked === true} size={16} />
							<span className="sr-only">按讚</span>
							<span>{comment.likeCount ?? 0}</span>
						</Button>
					) : null}
					{canReply && !editing && (
						<button
							type="button"
							className="bg-transparent border-0 text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-tiny)] cursor-pointer py-[2px] px-[6px] rounded-[var(--hkui-radius-sm)] hover:text-[var(--hkui-primary)] hover:bg-[var(--hkui-content2)]"
							onClick={() => setReplyOpen((v) => !v)}
						>
							{replyOpen ? "取消回覆" : "回覆"}
						</button>
					)}
					{replyCount > 0 && !editing && (
						<Button
							type="button"
							variant="light"
							color="primary"
							size="sm"
							className="h-auto min-w-0 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)]"
							disabled={likePendingAction !== null || deleting}
							isLoading={replies.loading}
							loadingLabel="載入回覆中…"
							onClick={() => {
								if (replies.open) replies.collapse();
								else void replies.expand();
							}}
						>
							{replies.open ? "收合回覆" : `查看 ${replyCount} 則回覆`}
						</Button>
					)}
					{!editing && (
						<CommentEntryActions
							canEdit={canEdit}
							canDelete={canDelete}
							canBlock={Boolean(canBlock)}
							editDisabled={likePendingAction !== null || deleting}
							deleteDisabled={likePendingAction !== null}
							blockDisabled={deleting || blockBusy}
							deleting={deleting}
							deleteLoadingLabel="刪除留言中…"
							onEdit={startEdit}
							onDelete={() => void handleDelete()}
							onBlock={() => void onBlockAuthor?.(comment)}
						/>
					)}
				</footer>

				{replyOpen && (
					<ReplyComposer
						placeholder={
							replyPlaceholder ??
							(comment.userName
								? tFeed("replyToName", { name: comment.userName })
								: tFeed("replyPlaceholder"))
						}
						avatar={composerAvatar}
						onSubmit={sendReply}
						onCancel={() => setReplyOpen(false)}
					/>
				)}

				{replies.open && (
					<ul className="list-none m-0 mt-[var(--hkui-space-3)] pl-[var(--hkui-space-3)] border-l-2 border-[var(--hkui-divider)] flex flex-col gap-[var(--hkui-space-3)]">
						{replies.loading &&
						replies.items.length === 0 &&
						replies.submitted.length === 0 ? (
							<li className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
								載入中…
							</li>
						) : replies.items.length === 0 && replies.submitted.length === 0 ? (
							<li className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
								還沒有回覆
							</li>
						) : (
							replies.items.map((r) =>
								r.optimistic ? (
									<OptimisticCommentRow
										key={r.id}
										comment={r}
										variant="reply"
									/>
								) : (
									<ReplyItem
										key={r.id}
										reply={r}
										onRemoved={replies.removed}
										onBlockAuthor={onBlockAuthor}
										blockBusy={blockBusy}
									/>
								),
							)
						)}
						{replies.loadMoreError ? (
							<li className="list-none">
								<CommentErrorBox>
									{errorText(replies.loadMoreError, defaultErrors)}
								</CommentErrorBox>
							</li>
						) : null}
						{replies.hasMore ? (
							<li className="list-none">
								<Button
									type="button"
									variant="light"
									color="primary"
									size="sm"
									className="h-auto min-w-0 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)]"
									isLoading={replies.loadingMore}
									loadingLabel="載入回覆中…"
									onClick={() => void replies.loadMore()}
								>
									載入更多
								</Button>
							</li>
						) : null}
						{replies.submitted.map(({ key, comment: submitted }) => (
							<OptimisticCommentRow
								key={key}
								comment={submitted}
								variant="reply"
							/>
						))}
					</ul>
				)}
			</div>
		</li>
	);
}

export default function PostComments({
	postId,
	viewer: viewerProp,
	allowBlockAuthor = false,
	active = true,
}: PostCommentsProps) {
	const { status, identity } = useAuth();
	const signedIn = status === "signedIn";
	const t = useTranslations("feed.comments");
	const tSeller = useTranslations("seller.comments");
	const fetchedViewer = usePostViewerState(
		postId,
		signedIn && viewerProp === undefined,
	);
	const viewer = viewerProp ?? fetchedViewer.data;
	const list = usePostComments(postId, { active });
	const defaultErrors = useDefaultErrorText();
	const [blockNotice, setBlockNotice] = React.useState<string | null>(null);
	const [blockError, setBlockError] = React.useState<string | null>(null);
	const [blocking, setBlocking] = React.useState(false);
	const blockingRef = React.useRef(false);
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
	const [submitError, setSubmitError] = React.useState<string | null>(null);
	const rootRef = React.useRef<HTMLElement | null>(null);
	const deepLinkDoneRef = React.useRef(false);

	/* Deep-link: notification → `…?post=<id>&comment=<id>` reveals the target
	 * row once this post's detail is open. Comments stream in async, so poll the
	 * anchor for ~3s before degrading to the comment section. */
	React.useEffect(() => {
		if (deepLinkDoneRef.current || typeof window === "undefined") return;
		const params = new URLSearchParams(window.location.search);
		const targetPost = params.get("post");
		const targetComment = params.get("comment");
		if (!targetPost || !targetComment) return;
		if (String(postId) !== targetPost) return;
		deepLinkDoneRef.current = true;
		// Effect is client-only (guarded above), so CSS.escape is available;
		// comment ids are numeric TSIDs but escape keeps the selector safe.
		const selector = `[data-comment-anchor="${CSS.escape(targetComment)}"]`;
		let tries = 0;
		const reveal = () => {
			const root = rootRef.current;
			const el = root?.querySelector<HTMLElement>(selector);
			if (el) {
				revealInScrollParent(el);
				highlightComment(el);
				return true;
			}
			return false;
		};
		if (reveal()) return;
		const timer = window.setInterval(() => {
			tries += 1;
			if (reveal() || tries >= 20) {
				window.clearInterval(timer);
				if (tries >= 20 && rootRef.current)
					revealInScrollParent(rootRef.current);
			}
		}, 150);
		return () => window.clearInterval(timer);
	}, [postId]);
	// 留言身分由伺服器的 `viewer_actor` 決定（店主在自己店的貼文是店家），不看現在在哪個畫面。
	const actor = viewer?.actor;
	const storeName = storeActorName(viewer);
	const composerAvatar: ComposerAvatar =
		actor?.kind === "store"
			? {
					src: actor.avatarKey,
					alt: storeName ?? "",
					entity: "seller",
				}
			: {
					src: actor?.avatarKey ?? identity?.avatarKey,
					alt: actor?.name ?? identity?.nickname ?? t("selfAlt"),
					entity: "buyer",
				};
	const commentPlaceholder = storeName
		? t("composeAsStore", { storeName })
		: t("composePlaceholder");
	const replyPlaceholder = storeName
		? t("replyAsStore", { storeName })
		: undefined;

	const blockAuthor = React.useCallback(
		async (comment: Comment) => {
			const name = comment.userName;
			if (
				!window.confirm(
					`${tSeller("blockTitle", { name })}\n\n${tSeller("blockDescription")}`,
				)
			)
				return;
			if (blockingRef.current) return;
			blockingRef.current = true;
			setBlocking(true);
			setBlockNotice(null);
			setBlockError(null);
			try {
				await blockBuyerByComment(comment.id);
				setBlockNotice(tSeller("blocked", { name }));
			} catch (e) {
				setBlockError(errorText(e, defaultErrors));
			} finally {
				blockingRef.current = false;
				setBlocking(false);
			}
		},
		[tSeller, defaultErrors],
	);

	const submitDraft = async () => {
		if (!draft.trim() || submitting) return;
		setSubmitting(true);
		setSubmitError(null);
		try {
			// optimistic 顯示 composer 的名稱與頭像；留言身分由伺服器決定。
			await list.post(draft, {
				authorName: composerAvatar.alt,
				authorAvatar: composerAvatar.src ?? undefined,
			});
			setDraft("");
		} catch (e) {
			setSubmitError(errorText(e, defaultErrors));
		} finally {
			setSubmitting(false);
		}
	};

	const handleSend = (e: React.FormEvent) => {
		e.preventDefault();
		void submitDraft();
	};

	// Enter 送出、Shift+Enter 換行；IME 組字中跳過避免送出半字
	const handleDraftKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
		if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault();
			void submitDraft();
		}
	};

	return (
		<section
			ref={rootRef}
			className="post-comments mt-[var(--hkui-space-5)] p-[var(--hkui-space-4)_var(--hkui-space-5)_var(--hkui-space-5)] border-t border-[var(--hkui-divider)]"
		>
			<header className="flex justify-between items-center mb-[var(--hkui-space-3)]">
				<strong className="text-[length:var(--hkui-text-medium)]">
					留言（{list.comments.length + list.submitted.length}）
				</strong>
			</header>

			{blockNotice ? (
				<p
					className="mb-[var(--hkui-space-3)] text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]"
					role="status"
				>
					{blockNotice}
				</p>
			) : null}

			{blockError ? (
				<CommentErrorBox className="my-[var(--hkui-space-3)]">
					{blockError}
				</CommentErrorBox>
			) : null}

			{viewerProp === undefined && signedIn && fetchedViewer.isError ? (
				<CommentErrorBox className="my-[var(--hkui-space-3)] flex items-center justify-between gap-[var(--hkui-space-3)]">
					<span>{errorText(fetchedViewer.error, defaultErrors)}</span>
					<Button
						variant="bordered"
						color="default"
						size="sm"
						disabled={fetchedViewer.isFetching}
						onClick={() => void fetchedViewer.refetch()}
					>
						重試
					</Button>
				</CommentErrorBox>
			) : null}

			{canComposeComment(signedIn, viewer) && (
				<form
					className="flex gap-[var(--hkui-space-3)] items-start mb-[var(--hkui-space-4)]"
					onSubmit={handleSend}
				>
					<div className="flex-1 flex flex-col gap-[var(--hkui-space-2)]">
						<textarea
							className="w-full border border-divider rounded-hkui-md py-2 px-3 bg-content1 min-h-11 resize-none focus:outline-none focus:border-hkui-primary [font-family:inherit] [font-size:inherit] [font-weight:inherit] leading-6"
							value={draft}
							onChange={(e) => setDraft(e.target.value)}
							onKeyDown={handleDraftKeyDown}
							placeholder={commentPlaceholder}
							rows={1}
							maxLength={500}
						/>
						<Button
							type="submit"
							size="sm"
							className="self-end"
							disabled={!draft.trim() || submitting}
							isLoading={submitting}
							loadingLabel="送出留言中…"
						>
							送出
						</Button>
					</div>
				</form>
			)}

			{submitError ? (
				<CommentErrorBox className="my-[var(--hkui-space-4)]">
					{submitError}
				</CommentErrorBox>
			) : null}

			{list.submitted.length > 0 ? (
				<ul className="list-none m-0 mb-[var(--hkui-space-4)] p-0 flex flex-col gap-[var(--hkui-space-4)]">
					{list.submitted.map(({ key, comment }) => (
						<OptimisticCommentRow
							key={key}
							comment={comment}
							variant="comment"
						/>
					))}
				</ul>
			) : null}

			{list.error != null && !isNotFound(list.error) ? (
				<CommentErrorBox className="my-[var(--hkui-space-4)]">
					{errorText(list.error, defaultErrors)}
				</CommentErrorBox>
			) : null}

			{list.loading && list.comments.length === 0 ? (
				<p className="my-[var(--hkui-space-4)] text-center text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]">
					載入中…
				</p>
			) : list.comments.length === 0 ? (
				<EmptyStatePanel
					icon="message"
					title="還沒有留言"
					description="第一個留下心得，分享你對這個品項的感覺。"
					className="text-[var(--hkui-foreground-soft)] text-center py-[var(--hkui-space-5)] px-0 bg-transparent border-0 min-h-0"
					iconSize={88}
				/>
			) : (
				<ul className="list-none m-0 p-0 flex flex-col gap-[var(--hkui-space-4)]">
					{list.comments.map((c) =>
						c.optimistic ? (
							<OptimisticCommentRow key={c.id} comment={c} variant="comment" />
						) : (
							<CommentItem
								key={c.id}
								comment={c}
								postId={postId}
								replyPlaceholder={replyPlaceholder}
								composerAvatar={composerAvatar}
								onLike={list.like}
								onRemove={list.remove}
								onEdited={list.edited}
								onBlockAuthor={allowBlockAuthor ? blockAuthor : undefined}
								blockBusy={blocking}
							/>
						),
					)}
				</ul>
			)}

			{list.loadMoreError != null ? (
				<CommentErrorBox className="my-[var(--hkui-space-4)]">
					{errorText(list.loadMoreError, defaultErrors)}
				</CommentErrorBox>
			) : null}

			{list.hasMore && (
				<div className="mt-[var(--hkui-space-4)] flex justify-center">
					<Button
						variant="bordered"
						color="default"
						size="sm"
						onClick={() => void list.loadMore()}
						disabled={list.loadingMore}
						isLoading={list.loadingMore}
						loadingLabel="載入留言中…"
					>
						載入更多留言
					</Button>
				</div>
			)}
		</section>
	);
}
