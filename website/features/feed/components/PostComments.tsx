"use client";

import { gsap } from "gsap";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import { useSellerIdentity } from "@/components/shared/useSellerIdentity";
import Button from "@/components/ui/Button";
import { useAuth } from "@/lib/auth/context";
import { formatTaipeiShortDate } from "@/lib/date/taipei";
import {
	snapshotToComment,
	useCommentReplies,
	usePostComments,
} from "@/lib/hooks/useSocial";
import {
	type Comment,
	createReply,
	deleteComment,
	toggleCommentLike,
	updateComment,
} from "@/lib/social/repo";
import { subscribeCommentEvents } from "@/lib/sse/commentEventBus";
import { canDeleteComment, canEditComment } from "./commentPermissions";

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
	postId: number | string;
	/**
	 * Grant the current viewer post-owner moderation rights — show 刪除 on
	 * every comment / reply (not just one's own), matching the backend's
	 * "own comment OR post owner" DELETE permission. Seller-side surfaces
	 * (e.g. the seller-center post viewer) set this true; buyer surfaces
	 * leave it false so a buyer can still only remove their own comments.
	 */
	canModerate?: boolean;
	/**
	 * Whether the comment SSE stream should be open for this post. Single-
	 * post surfaces (bottom sheet, seller post modal) leave the default
	 * `true`; the snap pager passes `active={isActive}` so only the
	 * currently-shown page holds a live connection.
	 */
	active?: boolean;
}

/** Avatar shown next to a composer (new comment / reply). On the seller
 *  surface this is the store logo so it's clear the reply comes from the
 *  shop; on buyer surfaces it's the member's own avatar. */
type ComposerAvatar = {
	src?: string | null;
	alt: string;
	entity: "buyer" | "seller";
};

/** Inline reply composer shown under a comment when the viewer taps 回覆. */
function ReplyComposer({
	postId,
	parentId,
	replyToId,
	replyToName,
	avatar,
	onSent,
	onCancel,
}: {
	postId: number | string;
	parentId: number | string;
	replyToId: number | string;
	replyToName?: string;
	avatar: ComposerAvatar;
	onSent: (created: Comment) => void;
	onCancel: () => void;
}) {
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
	const textareaRef = React.useRef<HTMLTextAreaElement | null>(null);

	React.useEffect(() => {
		textareaRef.current?.focus();
	}, []);

	const submit = async () => {
		const content = draft.trim();
		if (!content || submitting) return;
		setSubmitting(true);
		try {
			// `createReply` already returns a normalised `Comment` (the repo
			// runs the response through `mapServerComment`), so we can hand it
			// straight to the parent — no optimistic merge needed. Fill in the
			// replyToName display hint if the backend didn't echo it back.
			const clientRequestId =
				typeof crypto !== "undefined" && "randomUUID" in crypto
					? crypto.randomUUID()
					: `cri-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
			// 回覆走專屬 replies 端點（parent 在 path），postId/replyToId 為 TSID
			// 一律以 string 傳；留言身分由伺服器依貼文歸屬決定。
			const created = await createReply(parentId, {
				content,
				postId: postId,
				replyToId: replyToId,
				clientRequestId,
			});
			setDraft("");
			onSent({
				...created,
				replyToName: created.replyToName ?? replyToName,
			});
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
					placeholder={replyToName ? `回覆 ${replyToName}…` : "寫下你的回覆…"}
					rows={1}
					maxLength={500}
				/>
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

/** A reply row under a parent comment — same layout as CommentItem but tighter,
 *  no nested replies (we only render one thread level deep). */
function ReplyItem({
	reply,
	canModerate,
	onRemoved,
}: {
	reply: Comment;
	canModerate: boolean;
	onRemoved: (id: number | string) => void;
}) {
	const [liked, setLiked] = React.useState(reply.isLiked === true);
	const [likeCount, setLikeCount] = React.useState(reply.likeCount ?? 0);
	// Local content mirror — optimistic edit shows immediately, refresh from
	// parent list will overwrite if backend differs.
	const [content, setContent] = React.useState(reply.content);
	const [editing, setEditing] = React.useState(false);
	const [editDraft, setEditDraft] = React.useState("");
	const [savingEdit, setSavingEdit] = React.useState(false);
	const [likePendingAction, setLikePendingAction] = React.useState<
		"like" | "unlike" | null
	>(null);
	const [deleting, setDeleting] = React.useState(false);
	const likePendingRef = React.useRef(false);
	const deletePendingRef = React.useRef(false);
	const editTextareaRef = React.useRef<HTMLTextAreaElement | null>(null);

	React.useEffect(() => {
		if (editing) editTextareaRef.current?.focus();
	}, [editing]);

	const canDelete = canDeleteComment(reply.isMine, canModerate);
	// 編輯只開放給作者本人 — post owner 可刪不能改別人內容
	const canEdit = canEditComment(reply.isMine);

	const handleLike = async () => {
		if (likePendingRef.current || deletePendingRef.current) return;
		likePendingRef.current = true;
		const wasLiked = liked;
		setLikePendingAction(wasLiked ? "unlike" : "like");
		setLiked(!wasLiked);
		setLikeCount((c) => c + (wasLiked ? -1 : 1));
		try {
			const r = await toggleCommentLike(reply.id, wasLiked);
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
		try {
			await deleteComment(reply.id);
			onRemoved(reply.id);
		} catch (e) {
			window.alert(e instanceof Error ? e.message : "刪除失敗");
		} finally {
			deletePendingRef.current = false;
			setDeleting(false);
		}
	};

	const startEdit = () => {
		setEditDraft(content);
		setEditing(true);
	};

	const cancelEdit = () => {
		setEditing(false);
		setEditDraft("");
	};

	const submitEdit = async () => {
		const next = editDraft.trim();
		if (!next || savingEdit) return;
		if (next === content) {
			setEditing(false);
			return;
		}
		setSavingEdit(true);
		try {
			const updated = await updateComment(reply.id, next);
			setContent(updated.content || next);
			setEditing(false);
		} catch (e) {
			window.alert(e instanceof Error ? e.message : "編輯失敗");
		} finally {
			setSavingEdit(false);
		}
	};

	const onEditKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
		if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault();
			void submitEdit();
		} else if (e.key === "Escape") {
			e.preventDefault();
			cancelEdit();
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
					<div className="flex flex-col gap-[var(--hkui-space-2)] mt-1">
						<textarea
							ref={editTextareaRef}
							className="w-full border border-divider rounded-hkui-md py-2 px-3 bg-content1 min-h-10 resize-none focus:outline-none focus:border-hkui-primary [font-family:inherit] [font-weight:inherit] leading-5 text-[length:var(--hkui-text-small)]"
							value={editDraft}
							onChange={(e) => setEditDraft(e.target.value)}
							onKeyDown={onEditKeyDown}
							rows={1}
							maxLength={500}
						/>
						<div className="flex gap-[var(--hkui-space-2)] self-end">
							<Button
								type="button"
								variant="bordered"
								color="default"
								size="sm"
								onClick={cancelEdit}
							>
								取消
							</Button>
							<Button
								type="button"
								size="sm"
								disabled={!editDraft.trim() || savingEdit}
								isLoading={savingEdit}
								loadingLabel="儲存回覆中…"
								onClick={() => void submitEdit()}
							>
								儲存
							</Button>
						</div>
					</div>
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
				<footer className="flex flex-wrap gap-[var(--hkui-space-3)] mt-1">
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
					{canEdit && !editing && (
						<button
							type="button"
							className="bg-transparent border-0 text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-tiny)] cursor-pointer py-[2px] px-[6px] rounded-[var(--hkui-radius-sm)] hover:text-[var(--hkui-primary)] hover:bg-[var(--hkui-content2)]"
							disabled={likePendingAction !== null || deleting}
							onClick={startEdit}
						>
							編輯
						</button>
					)}
					{canDelete && !editing && (
						<Button
							type="button"
							variant="light"
							color="danger"
							size="sm"
							className="h-auto min-w-0 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)] font-normal text-[var(--hkui-foreground-soft)]"
							disabled={likePendingAction !== null}
							isLoading={deleting}
							loadingLabel="刪除回覆中…"
							onClick={() => void handleDelete()}
						>
							刪除
						</Button>
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
	canModerate,
	isLoggedIn,
	composerAvatar,
	onLike,
	onRemove,
}: {
	comment: Comment;
	postId: number | string;
	canModerate: boolean;
	isLoggedIn: boolean;
	composerAvatar: ComposerAvatar;
	onLike: (id: number | string) => Promise<void>;
	onRemove: (id: number | string) => Promise<void>;
}) {
	const replies = useCommentReplies(comment.id);
	const [replyOpen, setReplyOpen] = React.useState(false);
	// Local mirror of replyCount so it bumps the moment we send a reply
	// without waiting for the parent list to refetch.
	const [replyCount, setReplyCount] = React.useState(comment.replyCount ?? 0);
	// Optimistic replies appended after a successful send; useCommentReplies'
	// internal `items` is updated on refresh / expand, so we keep our own
	// list to merge with its load.
	const [pendingReplies, setPendingReplies] = React.useState<Comment[]>([]);
	// Local content mirror — optimistic edit shows immediately
	const [content, setContent] = React.useState(comment.content);
	const [editing, setEditing] = React.useState(false);
	const [editDraft, setEditDraft] = React.useState("");
	const [savingEdit, setSavingEdit] = React.useState(false);
	const [likePendingAction, setLikePendingAction] = React.useState<
		"like" | "unlike" | null
	>(null);
	const [deleting, setDeleting] = React.useState(false);
	const likePendingRef = React.useRef(false);
	const deletePendingRef = React.useRef(false);
	const editTextareaRef = React.useRef<HTMLTextAreaElement | null>(null);

	React.useEffect(() => {
		if (editing) editTextareaRef.current?.focus();
	}, [editing]);

	/* ── SSE reply inline sync ───────────────────────────────────────────
	 * `usePostComments` already bumps our `replyCount` on `comment.replied`
	 * for any post the realtime stream is open on (see
	 * `useSocial.ts::handleReplied`). It also publishes the reply snapshot
	 * to the per-postId comment bus so we can splice the new row into our
	 * thread state without waiting for the user to collapse + re-expand.
	 * Dedup by id against both the REST-loaded thread (`replies.items`) and
	 * the local pending mirror — covers all three orderings of
	 *   (a) own ReplyComposer onSent → SSE echo,
	 *   (b) SSE echo → own onSent,
	 *   (c) cross-tab peer reply.
	 *
	 * `replies.items` lives in a ref so the bus subscription survives a
	 * thread expand / refresh (which mutates `items`) without re-binding —
	 * an in-flight reply event during the resubscribe window would
	 * otherwise be silently dropped.
	 */
	const replyItemsRef = React.useRef(replies.items);
	replyItemsRef.current = replies.items;
	React.useEffect(() => {
		const myCommentId = String(comment.id);
		return subscribeCommentEvents(postId, (event) => {
			if (event.kind !== "reply_arrived") return;
			if (event.parentCommentId !== myCommentId) return;
			const replyId = event.reply.commentId;
			setPendingReplies((prev) => {
				if (prev.some((r) => String(r.id) === replyId)) return prev;
				if (replyItemsRef.current.some((r) => String(r.id) === replyId))
					return prev;
				return [...prev, snapshotToComment(event.reply)];
			});
		});
	}, [postId, comment.id]);

	const canDelete = canDeleteComment(comment.isMine, canModerate);
	// 編輯只開放給作者本人 — post owner 可刪不能改別人內容
	const canEdit = canEditComment(comment.isMine);

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
		try {
			await onRemove(comment.id);
		} finally {
			deletePendingRef.current = false;
			setDeleting(false);
		}
	};

	const startEdit = () => {
		setEditDraft(content);
		setEditing(true);
	};

	const cancelEdit = () => {
		setEditing(false);
		setEditDraft("");
	};

	const submitEdit = async () => {
		const next = editDraft.trim();
		if (!next || savingEdit) return;
		if (next === content) {
			setEditing(false);
			return;
		}
		setSavingEdit(true);
		try {
			const updated = await updateComment(comment.id, next);
			setContent(updated.content || next);
			setEditing(false);
		} catch (e) {
			window.alert(e instanceof Error ? e.message : "編輯失敗");
		} finally {
			setSavingEdit(false);
		}
	};

	const onEditKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
		if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
			e.preventDefault();
			void submitEdit();
		} else if (e.key === "Escape") {
			e.preventDefault();
			cancelEdit();
		}
	};

	const handleReplySent = (created: Comment) => {
		// Dedup: the SSE bus may deliver the same reply (server-echoed) before
		// REST resolves. Whichever path runs first wins; the second one finds
		// the id already in pending and short-circuits, so we never double-
		// insert. Also check `replies.items` for the late-REST case where the
		// thread was refreshed in the same window.
		setPendingReplies((prev) => {
			const id = String(created.id);
			if (prev.some((r) => String(r.id) === id)) return prev;
			if (replies.items.some((r) => String(r.id) === id)) return prev;
			return [...prev, created];
		});
		setReplyCount((c) => c + 1);
		setReplyOpen(false);
		if (!replies.open) void replies.expand();
	};

	const handleReplyRemoved = (replyId: number | string) => {
		setPendingReplies((prev) =>
			prev.filter((r) => String(r.id) !== String(replyId)),
		);
		setReplyCount((c) => Math.max(0, c - 1));
		void replies.refresh();
	};

	// Merge backend-loaded replies with our optimistic ones, de-duped by id.
	const mergedReplies = React.useMemo(() => {
		const seen = new Set<string>();
		const all: Comment[] = [];
		for (const r of replies.items) {
			seen.add(String(r.id));
			all.push(r);
		}
		for (const r of pendingReplies) {
			if (!seen.has(String(r.id))) all.push(r);
		}
		return all;
	}, [replies.items, pendingReplies]);

	return (
		<li
			className="flex gap-[var(--hkui-space-3)]"
			data-comment-anchor={String(comment.id)}
		>
			<span className="block w-9 h-9 shrink-0 rounded-full overflow-hidden bg-[var(--hkui-default-100)]">
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
				{editing ? (
					<div className="flex flex-col gap-[var(--hkui-space-2)] mt-1">
						<textarea
							ref={editTextareaRef}
							className="w-full border border-divider rounded-hkui-md py-2 px-3 bg-content1 min-h-10 resize-none focus:outline-none focus:border-hkui-primary [font-family:inherit] [font-weight:inherit] leading-5 text-[length:var(--hkui-text-small)]"
							value={editDraft}
							onChange={(e) => setEditDraft(e.target.value)}
							onKeyDown={onEditKeyDown}
							rows={1}
							maxLength={500}
						/>
						<div className="flex gap-[var(--hkui-space-2)] self-end">
							<Button
								type="button"
								variant="bordered"
								color="default"
								size="sm"
								onClick={cancelEdit}
							>
								取消
							</Button>
							<Button
								type="button"
								size="sm"
								disabled={!editDraft.trim() || savingEdit}
								isLoading={savingEdit}
								loadingLabel="儲存留言中…"
								onClick={() => void submitEdit()}
							>
								儲存
							</Button>
						</div>
					</div>
				) : (
					<p className="my-1 text-[length:var(--hkui-text-small)] leading-[var(--hkui-line-medium)] whitespace-pre-wrap break-words">
						{content}
					</p>
				)}
				<footer className="flex flex-wrap gap-[var(--hkui-space-3)] mt-1">
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
					{isLoggedIn && !editing && (
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
					{canEdit && !editing && (
						<button
							type="button"
							className="bg-transparent border-0 text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-tiny)] cursor-pointer py-[2px] px-[6px] rounded-[var(--hkui-radius-sm)] hover:text-[var(--hkui-primary)] hover:bg-[var(--hkui-content2)]"
							disabled={likePendingAction !== null || deleting}
							onClick={startEdit}
						>
							編輯
						</button>
					)}
					{canDelete && !editing && (
						<Button
							type="button"
							variant="light"
							color="danger"
							size="sm"
							className="h-auto min-w-0 px-[6px] py-[2px] text-[length:var(--hkui-text-tiny)] font-normal text-[var(--hkui-foreground-soft)]"
							disabled={likePendingAction !== null}
							isLoading={deleting}
							loadingLabel="刪除留言中…"
							onClick={() => void handleDelete()}
						>
							刪除
						</Button>
					)}
				</footer>

				{replyOpen && (
					<ReplyComposer
						postId={postId}
						parentId={comment.id}
						replyToId={comment.id}
						replyToName={comment.userName}
						avatar={composerAvatar}
						onSent={handleReplySent}
						onCancel={() => setReplyOpen(false)}
					/>
				)}

				{replies.open && (
					<ul className="list-none m-0 mt-[var(--hkui-space-3)] pl-[var(--hkui-space-3)] border-l-2 border-[var(--hkui-divider)] flex flex-col gap-[var(--hkui-space-3)]">
						{replies.loading && mergedReplies.length === 0 ? (
							<li className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
								載入中…
							</li>
						) : mergedReplies.length === 0 ? (
							<li className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-soft)]">
								還沒有回覆
							</li>
						) : (
							mergedReplies.map((r) => (
								<ReplyItem
									key={r.id}
									reply={r}
									canModerate={canModerate}
									onRemoved={handleReplyRemoved}
								/>
							))
						)}
					</ul>
				)}
			</div>
		</li>
	);
}

export default function PostComments({
	postId,
	canModerate = false,
	active = true,
}: PostCommentsProps) {
	const { isLoggedIn, identity } = useAuth();
	const sellerIdentity = useSellerIdentity();
	const list = usePostComments(postId, { active });
	const [draft, setDraft] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);
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
	// On the seller surface (canModerate) the viewer replies AS the shop, so
	// the composer shows the store logo; buyer surfaces show the member avatar.
	const composerAvatar: ComposerAvatar = canModerate
		? {
				src: sellerIdentity.avatar,
				alt: sellerIdentity.storeName || "賣家",
				entity: "seller",
			}
		: {
				src: identity?.avatarKey,
				alt: identity?.nickname || "我",
				entity: "buyer",
			};

	const submitDraft = async () => {
		if (!draft.trim() || submitting) return;
		setSubmitting(true);
		try {
			// optimistic 顯示 composer 的名稱與頭像；留言身分由伺服器決定。
			await list.post(draft, {
				authorName: composerAvatar.alt,
				authorAvatar: composerAvatar.src ?? undefined,
			});
			setDraft("");
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
					留言（{list.comments.length}）
				</strong>
			</header>

			{isLoggedIn && (
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
							placeholder="寫下你的留言…"
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

			{list.error && !/HTTP\s*404|404|找不到|not\s*found/i.test(list.error) && (
				<p
					className="my-[var(--hkui-space-4)] p-[var(--hkui-space-3)_var(--hkui-space-4)] rounded-[var(--hkui-radius-md)] bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)] text-[length:var(--hkui-text-small)]"
					role="alert"
				>
					{list.error}
				</p>
			)}

			{/* The SSE client stops for good on rate-limit/cap (design book
			    §11.6) — without this banner the list just silently froze. */}
			{list.streamStatus === "unavailable" && (
				<p
					className="my-[var(--hkui-space-3)] p-[var(--hkui-space-2)_var(--hkui-space-4)] rounded-[var(--hkui-radius-md)] bg-[var(--hkui-warning-50,rgba(216,180,87,0.14))] text-[var(--hkui-foreground-soft)] text-[length:var(--hkui-text-small)]"
					role="status"
				>
					即時更新暫停中，重新整理頁面可恢復。
				</p>
			)}

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
					{list.comments.map((c) => (
						<CommentItem
							key={c.id}
							comment={c}
							postId={postId}
							canModerate={canModerate}
							isLoggedIn={isLoggedIn}
							composerAvatar={composerAvatar}
							onLike={list.like}
							onRemove={list.remove}
						/>
					))}
				</ul>
			)}

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
