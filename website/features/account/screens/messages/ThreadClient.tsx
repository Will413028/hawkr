"use client";

import "@/styles/features/messages.css";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import Button from "@/components/ui/Button";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import AttachmentPicker from "@/features/messages/components/AttachmentPicker";
import {
	ChatOrderCard,
	ChatProductCard,
	type OrderCardData,
	type ProductCardData,
	tryParseOrder,
	tryParseProduct,
} from "@/features/messages/components/ChatCards";
import ChatContextPanel from "@/features/messages/components/ChatContextPanel";
import ChatPeerAvatar from "@/features/messages/components/ChatPeerAvatar";
import SendButton from "@/features/messages/components/SendButton";
import { chatWorkspaceRole } from "@/lib/chat/workspace";
import { formatTaipeiTime } from "@/lib/date/taipei";
import { useChatThread, useConversations } from "@/lib/hooks/useChat";
import { useComposerScroll } from "@/lib/hooks/useComposerScroll";
import { useKeyboardInset } from "@/lib/hooks/useKeyboardInset";
import { usePathname } from "@/lib/intl/navigation";
import { buildImageUrl } from "@/lib/media/cdn";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";
import { useStoreCardViews } from "@/lib/seller/useStoreCardViews";

const QUICK_REPLIES = ["你好！", "請問還有現貨嗎？", "想了解出貨時間"];

interface Props {
	targetId: string;
	/** Legacy — kept for call-site compat after the topbar was removed. */
	backHref?: string;
	/** Rendered inside an overlay/drawer; hide the route-level back affordance. */
	embedded?: boolean;
	/** Header hints when opened from the chat drawer (no `?name`/`?avatar`
	 * query). Used for seller → buyer first contact, where the public-profile
	 * fallback can't resolve a name. Falls back to the query params. */
	peerNameHint?: string;
	peerAvatarHint?: string;
}

/**
 * Header status is a simple peer-presence binary: 線上 when our socket
 * is live AND the peer reports online, 離線 otherwise (including while
 * our own connection is down or presence hasn't arrived yet — the
 * compose-bar notice covers "realtime paused" messaging, so the
 * header never shows socket jargon like 未連線).
 */
function resolveHeadStatus(
	connectionState: string,
	peerOnline: boolean | null,
): { state: string; label: string } {
	if (connectionState === "connected" && peerOnline === true) {
		return { state: "online", label: "線上" };
	}
	return { state: "offline", label: "離線" };
}

const formatClock = formatTaipeiTime;

/**
 * Date separators follow the LINE / WhatsApp convention: one centred
 * pill above the first message of each calendar day (individual
 * bubbles keep their own HH:mm, so the pill carries no time).
 *
 * Label format:
 *   - today      → `今天`
 *   - yesterday  → `昨天`
 *   - same year  → `M/D（週X）`
 *   - else       → `YYYY/M/D（週X）`
 */
const WEEKDAY_ZH = ["日", "一", "二", "三", "四", "五", "六"] as const;

function toMessageDate(ts: number): Date | null {
	if (!Number.isFinite(ts) || ts <= 0) return null;
	const d = new Date(ts < 1e12 ? ts * 1000 : ts);
	return Number.isNaN(d.getTime()) ? null : d;
}

function dayKey(ts: number | null | undefined): string | null {
	const d = ts != null ? toMessageDate(ts) : null;
	return d ? `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}` : null;
}

function formatDateSeparator(ts: number): string {
	const d = toMessageDate(ts);
	if (!d) return "";
	const now = new Date();
	const startOfDay = (x: Date) =>
		new Date(x.getFullYear(), x.getMonth(), x.getDate()).getTime();
	const diffDays = Math.round((startOfDay(now) - startOfDay(d)) / 86_400_000);
	if (diffDays === 0) return "今天";
	if (diffDays === 1) return "昨天";
	const md = `${d.getMonth() + 1}/${d.getDate()}（週${WEEKDAY_ZH[d.getDay()]}）`;
	return d.getFullYear() === now.getFullYear()
		? md
		: `${d.getFullYear()}/${md}`;
}

/**
 * Classify a chat message's payload into one of four render shapes.
 * Mirrors the buyer APP's `chat_room_screen` structured-message branch
 * (`isOrderCard` / `isProductCard`) plus a JSON-`text` short-circuit
 * for replies whose wrapper carries only a plain string. The renderer
 * applies a `--card` modifier so structured cards drop the bubble's
 * default padding / background and let the card own its surface.
 */
type BubbleKind =
	| { kind: "text"; text: string }
	| { kind: "image"; url: string }
	| { kind: "product"; data: ProductCardData }
	| { kind: "order"; data: OrderCardData };

function classifyBubble(type: string, content: string): BubbleKind {
	if (type === "image") return { kind: "image", url: content };
	const trimmed = (content ?? "").trim();
	if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
		const order = tryParseOrder(content);
		if (order) return { kind: "order", data: order };
		const product = tryParseProduct(content);
		if (product) return { kind: "product", data: product };
		// Wrapper carrying just `{ text: "..." }` — flatten to plain text.
		try {
			const decoded = JSON.parse(trimmed) as Record<string, unknown>;
			if (decoded && typeof decoded.text === "string" && decoded.text.trim()) {
				return { kind: "text", text: decoded.text.trim() };
			}
			for (const k of ["url", "image", "imageUrl", "img", "src"] as const) {
				const v = decoded?.[k];
				if (typeof v === "string" && v.trim()) {
					return { kind: "image", url: v };
				}
			}
		} catch {
			/* fall through to plain text */
		}
	}
	return { kind: "text", text: content };
}

export default function ThreadClient({
	targetId,
	embedded = false,
	peerNameHint,
	peerAvatarHint,
}: Props) {
	const router = useObservedRouter("message_thread");
	const searchParams = useSearchParams();
	const pathname = usePathname() || "";
	// Pin the compose bar above the soft keyboard on mobile (iOS Safari in
	// particular ignores `interactive-widget`, so `dvh` never shrinks on its
	// own). Publishes `--keyboard-inset` for the chat shell to subtract.
	useKeyboardInset();
	// Seller → buyer first contact carries the customer's name/avatar — as
	// props when opened from the chat drawer, or as query hints on the
	// full-page route — since no `activeConversation` exists until a message
	// is exchanged.
	const hintedName = peerNameHint ?? searchParams?.get("name") ?? "";
	const hintedAvatar = peerAvatarHint ?? searchParams?.get("avatar") ?? "";
	// The persistent messages layout owns the conversation-list socket
	// subscription. This read shares its React Query cache without registering a
	// second invalidation listener; the global drawer explicitly refreshes the
	// list when it closes.
	const conversations = useConversations({ subscribeToSocket: false });
	const {
		messages,
		loading,
		error,
		connectionState,
		peerTyping,
		peerOnline,
		send,
		sendImage,
		notifyTyping,
		// Resolved buyer_id / store_id, not the raw session accountId — the hook
		// owns this derivation so the bubble alignment here can never disagree
		// with the filter that decided which messages land in the thread.
		myId,
	} = useChatThread(targetId);
	const headStatus = resolveHeadStatus(connectionState, peerOnline);
	// Expectation-management line above the composer (the Airbnb
	// "typical response time" slot). Sending goes over HTTP POST, so a
	// dropped WebSocket only pauses live updates — say exactly that
	// instead of leaving "未連線" in the header to read as "broken".
	const composeNotice =
		connectionState === "reconnecting"
			? "即時更新暫停中，傳送的訊息仍會送達。"
			: connectionState === "disconnected"
				? "網路連線異常，恢復後會自動重連；傳送的訊息仍會送達。"
				: headStatus.state === "offline" && messages.length > 0
					? "對方目前離線，訊息會在對方上線後收到。"
					: "";
	const isSeller = chatWorkspaceRole(pathname) === "seller";
	const threadHrefBase = isSeller
		? "/seller-center/messages"
		: "/account/messages";
	const activeConversation = conversations.items.find(
		(item) => String(item.targetUserId) === String(targetId),
	);

	const [draft, setDraft] = React.useState("");
	const [sending, setSending] = React.useState(false);
	const [sendingImage, setSendingImage] = React.useState(false);
	const [pendingImage, setPendingImage] = React.useState<File | null>(null);
	const [attachOpen, setAttachOpen] = React.useState(false);
	const imageInputRef = React.useRef<HTMLInputElement | null>(null);
	const scrollerRef = React.useRef<HTMLDivElement | null>(null);
	const composerRef = React.useRef<HTMLTextAreaElement | null>(null);
	useComposerScroll(composerRef, draft);
	const typingDebounce = React.useRef<ReturnType<typeof setTimeout> | null>(
		null,
	);

	const { views: peerViews } = useStoreCardViews(
		!isSeller && targetId ? [targetId] : [],
	);
	const peerView = peerViews.get(String(targetId));

	const peerNickname =
		activeConversation?.targetNickname || peerView?.name || hintedName || "";
	const peerAvatar = isSeller
		? activeConversation?.targetAvatar || hintedAvatar || ""
		: activeConversation?.logo_url || peerView?.logoUrl || "";
	const peerShopHref = peerView?.href ?? "";
	const peerLabel = peerView?.labelText;
	const lastMessageKey =
		messages.length > 0
			? (messages[messages.length - 1]?.id ?? `${messages.length}`)
			: "empty";

	React.useEffect(() => {
		void lastMessageKey;
		const el = scrollerRef.current;
		if (!el) return;
		el.scrollTop = el.scrollHeight;
	}, [lastMessageKey]);

	const handleSend = async (e: React.FormEvent) => {
		e.preventDefault();
		if (!draft.trim() || sending) return;
		setSending(true);
		try {
			await send(draft.trim());
			setDraft("");
			notifyTyping(false);
		} catch {
			// useChatThread already rolls back the optimistic row and exposes the
			// friendly error. Consume the rejection at the event boundary so an
			// expected business 4xx (including self-message rejection) is not an
			// unhandled promise reported to Sentry; keep the draft for retry.
		} finally {
			setSending(false);
		}
	};

	const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
		setDraft(e.target.value);
		notifyTyping(true);
		if (typingDebounce.current) clearTimeout(typingDebounce.current);
		typingDebounce.current = setTimeout(() => notifyTyping(false), 3_000);
	};

	const handlePickAttachment = async (payload: string) => {
		// Attachment payloads go through the same `send()` pipeline as plain
		// text — the receiver re-parses the JSON via `tryParseOrder` /
		// `tryParseProduct` and renders a ChatCard. We don't pre-validate
		// here because send() carries its own retry / error UI.
		if (sending) return;
		setSending(true);
		try {
			await send(payload);
		} catch {
			// Error UI and optimistic rollback are owned by useChatThread.
		} finally {
			setSending(false);
		}
	};

	const handlePickImage = () => {
		imageInputRef.current?.click();
	};

	const handleImageChange = (event: React.ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0];
		// Reset value so picking the same file twice still fires `change`.
		event.target.value = "";
		if (!file || sendingImage || sending) return;
		// Crop (or 使用原圖) first; the confirmed file sends below.
		setPendingImage(file);
	};

	const sendPickedImage = async (file: File) => {
		setSendingImage(true);
		try {
			// uploadImageFile + the hook own all validation/error surfacing.
			await sendImage(file);
		} catch {
			/* error is surfaced via the thread's `error` state */
		} finally {
			setSendingImage(false);
		}
	};

	const handleQuickReply = (text: string) => {
		setDraft(text);
		// Focus + caret-to-end so the user can hit Enter or keep typing.
		// rAF lets the textarea's value commit before we move the caret.
		requestAnimationFrame(() => {
			const ta = composerRef.current;
			if (!ta) return;
			ta.focus();
			ta.setSelectionRange(text.length, text.length);
		});
	};

	return (
		<>
			<section
				className={["thread-page", embedded ? "thread-page--embedded" : ""]
					.filter(Boolean)
					.join(" ")}
				aria-label="對話內容"
			>
				<header className="thread-page__head">
					{!embedded ? (
						<button
							type="button"
							className="thread-page__back"
							onClick={() => router.push(threadHrefBase)}
							aria-label="返回對話列表"
						>
							<AppIcon name="arrow-left" size={20} />
						</button>
					) : null}
					<ChatPeerAvatar
						src={peerAvatar}
						alt={peerNickname}
						className="thread-page__head-avatar"
						fallback={isSeller ? "icon" : "empty"}
					/>
					<div className="thread-page__head-meta">
						<strong>{peerNickname || `對話對象 #${targetId}`}</strong>
						<span
							className="thread-page__head-status"
							data-state={headStatus.state}
						>
							{headStatus.label}
						</span>
					</div>
				</header>

				{error && (
					<p className="thread-page__alert" role="alert">
						{error}
					</p>
				)}

				<div className="thread-page__body" ref={scrollerRef}>
					{loading && messages.length === 0 ? (
						<p className="my-[var(--hkui-space-4)] text-center text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]">
							載入中…
						</p>
					) : messages.length === 0 ? (
						/* Conversation start — an inline peer-profile block (the
						 * Instagram DM pattern) instead of a floating card. It reads
						 * as "the beginning of this thread": future messages grow
						 * from beneath it, and the quick-reply chips sit where they
						 * act (above the composer), not in the middle of the room. */
						<div className="thread-intro">
							<ChatPeerAvatar
								src={peerAvatar}
								alt={peerNickname}
								className="thread-intro__avatar"
								fallback={isSeller ? "icon" : "empty"}
							/>
							<h2 className="thread-intro__name">
								{peerNickname || `對話對象 #${targetId}`}
							</h2>
							<p className="thread-intro__copy">
								{isSeller
									? `這裡是與${peerNickname || "客戶"}對話的起點。`
									: `打個招呼，開啟與${peerNickname || "賣家"}的對話吧。`}
							</p>
							{!isSeller && peerShopHref ? (
								<Link href={peerShopHref} className="thread-intro__shop">
									查看賣場
								</Link>
							) : null}
							{!isSeller && peerLabel ? (
								<span className="thread-intro__copy">{peerLabel}</span>
							) : null}
						</div>
					) : (
						<ul className="thread-page__list">
							{messages.map((m, i) => {
								const mine = m.senderId === myId;
								const clock = formatClock(m.timestamp);
								const body = classifyBubble(m.type, m.content);
								const isCard = body.kind === "product" || body.kind === "order";
								const prevTs = i > 0 ? messages[i - 1]?.timestamp : null;
								const curDay = dayKey(m.timestamp);
								const separatorLabel =
									curDay && curDay !== dayKey(prevTs)
										? formatDateSeparator(m.timestamp)
										: "";
								const bubbleKey = m.id || `${m.timestamp}-${m.senderId}-${i}`;
								return (
									<React.Fragment key={bubbleKey}>
										{separatorLabel ? (
											<li
												className="thread-page__separator"
												role="presentation"
											>
												<span>{separatorLabel}</span>
											</li>
										) : null}
										<li
											className={[
												"thread-page__bubble",
												mine ? "is-mine" : "",
												isCard ? "thread-page__bubble--card" : "",
											]
												.filter(Boolean)
												.join(" ")}
										>
											{body.kind === "image" ? (
												<ChatImage url={body.url} />
											) : body.kind === "product" ? (
												<ChatProductCard
													data={body.data}
													mine={mine}
													shareUrl={
														body.data.id
															? `${typeof window !== "undefined" ? window.location.origin : ""}/products/${encodeURIComponent(body.data.id)}`
															: undefined
													}
													onView={() => {
														const href = body.data.id
															? `/products/${encodeURIComponent(body.data.id)}`
															: body.data.name
																? `/market/products?q=${encodeURIComponent(body.data.name)}`
																: "/market/products";
														router.push(href);
													}}
												/>
											) : body.kind === "order" ? (
												<ChatOrderCard
													data={body.data}
													mine={mine}
													onView={() => {
														if (!body.data.orderNo) return;
														router.push(
															`${threadHrefBase.startsWith("/seller-center") ? "/seller-center/orders" : "/account/orders"}/${encodeURIComponent(body.data.orderNo)}`,
														);
													}}
													onSecondary={() => {
														if (body.data.variant === "preview") {
															// 再買一次 — drop the buyer into the order detail
															// where the reorder action already lives. The chat
															// card doesn't have enough payload (raw SKU ids /
															// quantities) to populate the cart directly.
															if (!body.data.orderNo) return;
															router.push(
																`/account/orders/${encodeURIComponent(body.data.orderNo)}?action=reorder`,
															);
														} else {
															// 聯絡賣家 — user is already in this thread; pull
															// focus to the compose textarea so they can type.
															composerRef.current?.focus();
														}
													}}
												/>
											) : (
												<p>{body.text}</p>
											)}
											{clock ? <small>{clock}</small> : null}
											{mine && m.read ? (
												<AppIcon
													name="show"
													className="thread-page__read"
													size={14}
													title="已讀"
												/>
											) : null}
										</li>
									</React.Fragment>
								);
							})}
						</ul>
					)}
					{peerTyping && (
						<div className="thread-page__typing" aria-live="polite">
							<span /> <span /> <span /> 對方正在輸入…
						</div>
					)}
				</div>

				{composeNotice ? (
					<p className="thread-page__notice" role="status">
						{composeNotice}
					</p>
				) : null}

				{!loading && messages.length === 0 ? (
					<div className="thread-page__chips">
						{QUICK_REPLIES.map((text) => (
							<button
								key={text}
								type="button"
								className="chat-empty-chip"
								onClick={() => handleQuickReply(text)}
							>
								{text}
							</button>
						))}
					</div>
				) : null}

				<form className="thread-page__compose" onSubmit={handleSend}>
					{/* Single pill: attach / image / textarea / send live inside one
					 * bordered container (instead of four scattered circles), so the
					 * composer reads as one control — the Airbnb/Nextdoor pattern. */}
					<div className="thread-page__field">
						<button
							type="button"
							className="thread-page__attach"
							onClick={() => setAttachOpen((v) => !v)}
							aria-label={attachOpen ? "關閉附件選單" : "附加商品或訂單"}
							aria-haspopup="dialog"
							aria-expanded={attachOpen}
						>
							{/* Matches the image icon beside it: same 20px optical size. The button owns the hit area; this plus mark
							 * intentionally has no glyph-level frame. */}
							<AppIcon name="plus" size={20} />
						</button>
						<Button
							type="button"
							variant="light"
							color="default"
							isIconOnly
							radius="full"
							className="thread-page__attach"
							onClick={handlePickImage}
							disabled={sendingImage}
							isLoading={sendingImage}
							loadingIndicator={<UploadLoadingIndicator size={18} />}
							loadingLabel="傳送圖片中…"
							aria-label={sendingImage ? "圖片傳送中" : "傳送圖片"}
						>
							<AppIcon name="image" size={22} />
						</Button>
						<input
							ref={imageInputRef}
							type="file"
							accept="image/*"
							className="hidden"
							onChange={handleImageChange}
						/>
						<ImageCropDialog
							files={pendingImage ? [pendingImage] : []}
							crop="photo"
							onCropped={(file) => void sendPickedImage(file)}
							onClose={() => setPendingImage(null)}
						/>
						<textarea
							ref={composerRef}
							value={draft}
							onChange={handleChange}
							placeholder="輸入訊息…"
							title="Shift + Enter 可換行"
							rows={1}
							maxLength={2000}
							onKeyDown={(e) => {
								if (e.key === "Enter" && !e.shiftKey) {
									e.preventDefault();
									void handleSend(e);
								}
							}}
						/>
						<SendButton
							type="submit"
							disabled={!draft.trim()}
							loading={sending}
							ariaLabel={sending ? "傳送中" : "傳送"}
						/>
					</div>
				</form>

				<AttachmentPicker
					open={attachOpen}
					onClose={() => setAttachOpen(false)}
					onPick={handlePickAttachment}
					role={isSeller ? "seller" : "buyer"}
					peerId={targetId}
				/>
			</section>
			{/* Buyer-side only: the peer is a seller with a public profile, so
			 * the grid can grow a commerce-context rail at >=1440px. Rendered
			 * as a `.chat-shell` grid sibling, not inside `.thread-page`. */}
			{!isSeller ? <ChatContextPanel sellerId={targetId} /> : null}
		</>
	);
}

/**
 * Image attachment with graceful 404 fallback. Chat images come
 * from user uploads stored on the CDN; URLs can rot when objects
 * are pruned or the bucket prefix changes. Without a fallback the
 * browser draws its native broken-glyph + alt text on top of the
 * `:has(img)` bubble layout, which looks half-rendered. When the
 * image fails (or the URL is empty), we render a framed
 * placeholder card (image-frame glyph + caption) sized like a
 * normal chat image so the conversation rhythm stays intact. The
 * bubble's `:has(img)` rule disengages (no `<img>` in the DOM) and
 * the bubble reverts to text-message padding around the
 * placeholder — timestamp still floats to the bottom-right via the
 * shared `.thread-page__bubble small` rule. */
function ChatImage({ url }: { url: string }) {
	const [broken, setBroken] = React.useState(false);
	const imageSrc = buildImageUrl(url, "chat");
	if (!imageSrc || broken) {
		return (
			<span
				className="thread-page__bubble-attachment"
				role="img"
				aria-label="圖片無法載入"
			>
				{/* Landscape glyph. It uses currentColor so the placeholder adapts to the
				 * bubble's foreground (primary on peer, white on mine, gold-ish
				 * on dark mode). */}
				<AppIcon
					name="image"
					className="thread-page__bubble-attachment__icon"
					size={48}
				/>
				<span className="thread-page__bubble-attachment__label">
					圖片無法載入
				</span>
			</span>
		);
	}
	return (
		// biome-ignore lint/performance/noImgElement: chat image URLs are user-provided runtime content
		<img
			src={imageSrc}
			alt="訊息附件"
			onError={() => setBroken(true)}
			loading="lazy"
		/>
	);
}
