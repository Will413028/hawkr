"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthGateWaitModal from "@/components/shared/AuthGateWaitModal";
import { prewarmChatDrawerThread } from "@/features/messages/components/ChatDrawerThreadLoader";
import { useAuth } from "@/lib/auth/context";
import { useAuthGate } from "@/lib/auth/useAuthGate";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { isStoreMissing } from "@/lib/seller/store";
import { useAppUiStore } from "@/lib/state/app-ui-store";
import { useChatDrawerStore } from "@/lib/state/chat-drawer-store";
import { decideSellerChatAction, isOwnSellerTarget } from "./seller-chat-guard";

interface SellerChatButtonProps {
	sellerId: string;
	className?: string;
	/** Buyer-view preview keeps the exact CTA but must never open or mutate chat. */
	previewMode?: boolean;
	/**
	 * Use the same message icon the buyer Navbar uses instead of the chat
	 * bubble. Opt-in: the product-page CTA keeps the chat bubble on its filled
	 * green pill.
	 */
	navbarIcon?: boolean;
}

const ChatIcon = ({ size = 22 }: { size?: number }) => (
	<AppIcon name="chat" size={size} />
);

const CHAT_DRAWER_HOVER_PREWARM_MS = 140;

export default function SellerChatButton({
	sellerId,
	className,
	previewMode = false,
	navbarIcon = false,
}: SellerChatButtonProps) {
	const noticeId = React.useId();
	const [notice, setNotice] = React.useState<string | null>(null);
	const chatHref = `/account/messages/${encodeURIComponent(sellerId)}`;
	const openChat = useChatDrawerStore((state) => state.openChat);
	const openLoginRequired = useAppUiStore((state) => state.openLoginRequired);
	const { activeRole, hasSeller, hydrated, isLoggedIn } = useAuth();
	const {
		data: sellerSelf,
		error: sellerSelfError,
		loading: sellerSelfLoading,
	} = useSellerSelf();
	const sellerProfileMissing = isStoreMissing(sellerSelfError);
	const sellerIdentityRef = React.useRef({
		hasSellerRole: hasSeller,
		sellerProfileMissing,
		sellerSelf,
		sellerSelfLoading,
	});
	// `useAuthGate` may replay a pre-hydration click later. Keep ownership data
	// fresh so that replay cannot close over the initial guest/null snapshot.
	sellerIdentityRef.current = {
		hasSellerRole: hasSeller,
		sellerProfileMissing,
		sellerSelf,
		sellerSelfLoading,
	};
	const canPrewarmChat =
		!previewMode &&
		hydrated &&
		isLoggedIn &&
		decideSellerChatAction({
			activeRole,
			targetSellerId: sellerId,
			...sellerIdentityRef.current,
		}) === "open";
	const canPrewarmChatRef = React.useRef(canPrewarmChat);
	const hoverPrewarmTimerRef = React.useRef<number | null>(null);
	canPrewarmChatRef.current = canPrewarmChat;

	const cancelHoverPrewarm = React.useCallback(() => {
		if (hoverPrewarmTimerRef.current === null) return;
		window.clearTimeout(hoverPrewarmTimerRef.current);
		hoverPrewarmTimerRef.current = null;
	}, []);
	const prewarmIfEligible = React.useCallback(() => {
		if (canPrewarmChatRef.current) prewarmChatDrawerThread();
	}, []);
	const scheduleHoverPrewarm = React.useCallback(() => {
		cancelHoverPrewarm();
		if (!canPrewarmChatRef.current) return;

		hoverPrewarmTimerRef.current = window.setTimeout(() => {
			hoverPrewarmTimerRef.current = null;
			prewarmIfEligible();
		}, CHAT_DRAWER_HOVER_PREWARM_MS);
	}, [cancelHoverPrewarm, prewarmIfEligible]);

	React.useEffect(() => {
		if (!canPrewarmChat) cancelHoverPrewarm();
		return cancelHoverPrewarm;
	}, [canPrewarmChat, cancelHoverPrewarm]);
	const { authGate, authWaiting } = useAuthGate(() => {
		setNotice(null);
		openLoginRequired({
			redirect: chatHref,
			title: "登入後即可和商家聊天",
			description: "登入 Hawkr 後，可以直接詢問商品、配送與庫存狀態。",
			actionLabel: "登入 / 註冊",
		});
	});

	React.useEffect(() => {
		if (!notice) return;
		const timer = window.setTimeout(() => setNotice(null), 4200);
		return () => window.clearTimeout(timer);
	}, [notice]);

	const handleClick = () => {
		if (previewMode) return;
		// Role comes from the gate's post-hydration snapshot — the render-time
		// value is stale when the click happened before the session resolved.
		authGate(({ activeRole }) => {
			const decision = decideSellerChatAction({
				activeRole,
				targetSellerId: sellerId,
				...sellerIdentityRef.current,
			});

			switch (decision) {
				case "switch_role":
					setNotice("請切換到買家帳號後再和商家聊天。");
					return;
				case "own_store":
					setNotice("這是你的商店，無法傳訊息給自己。");
					return;
				case "checking_identity":
					setNotice("正在確認商家身分，請稍候再試。");
					return;
				case "identity_unavailable":
					setNotice("目前無法確認商家身分，請稍後再試。");
					return;
				case "open":
					prewarmChatDrawerThread();
					openChat(sellerId);
			}
		});
	};

	// Once `GetMyStore` proves this is the viewer's own store, remove the
	// impossible CTA instead of leaving a control that can only return an error.
	if (!previewMode && hasSeller && isOwnSellerTarget(sellerId, sellerSelf)) {
		return null;
	}

	const sellerIdentityBlocked =
		hasSeller && !sellerSelf && !sellerProfileMissing;

	return (
		<div className="relative flex justify-end w-full">
			<button
				type="button"
				className={className}
				onClick={handleClick}
				onMouseEnter={scheduleHoverPrewarm}
				onMouseLeave={cancelHoverPrewarm}
				onFocus={() => {
					cancelHoverPrewarm();
					prewarmIfEligible();
				}}
				onPointerDown={(event) => {
					if (event.pointerType !== "touch") return;
					cancelHoverPrewarm();
					prewarmIfEligible();
				}}
				aria-busy={authWaiting || sellerSelfLoading}
				aria-disabled={previewMode || sellerIdentityBlocked || undefined}
				aria-describedby={notice ? noticeId : undefined}
				title={previewMode ? "買家視角預覽不會開啟聊天" : undefined}
			>
				{navbarIcon ? <AppIcon name="message" size={20} /> : <ChatIcon />}
				<span>聊天</span>
			</button>
			<AuthGateWaitModal open={authWaiting} />
			{notice ? (
				<p
					id={noticeId}
					className="absolute top-[calc(100%+10px)] right-0 z-[2] w-max max-w-[260px] m-0 py-2.5 px-3 text-hk-heading text-sm leading-[1.5] bg-hk-surface-card border border-hk-border-soft rounded-hk-sm shadow-hk-card"
					role="alert"
				>
					{notice}
				</p>
			) : null}
		</div>
	);
}
