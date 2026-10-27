"use client";

import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import Link from "@/components/shared/Link";
import { Dialog } from "@/components/ui/Dialog";
import { brandAssets } from "@/lib/media/public-assets";

interface LoginRequiredModalProps {
	open: boolean;
	onClose: () => void;
	redirect: string;
	title?: React.ReactNode;
	description?: React.ReactNode;
	actionLabel?: string;
}

/**
 * 未登入時擋在動作前的登入引導。
 *
 * 焦點與捲動全部交給共用 `Dialog`：這裡曾自帶一份 focus trap（其 focusable
 * 過濾沒有排除零尺寸元素，隱藏的可聚焦節點會困住 Tab），以及在 window 上
 * `wheel` / `touchmove` 一律 preventDefault 的「捲動鎖」——那條連 modal 內部
 * 都擋，description 變長時短視窗根本捲不到按鈕。Dialog 用的是共用的
 * body scroll lock，面板內仍可捲。
 *
 * 版面沿用原本的「品牌圓盤 → 眉標 → 標題 → 說明 → 兩顆動作」，所以標題留在
 * 內容區而不是 Dialog 的 header；`aria-labelledby` 直接指向那個 h2（rest props
 * 會覆蓋 Dialog 自己的值）。
 */
export default function LoginRequiredModal({
	open,
	onClose,
	redirect,
	title = "登入後即可繼續",
	description = "登入 Hawkr 後，就能訂閱商家、收藏商品，並接收最新出貨與優惠通知。",
	actionLabel = "前往登入",
}: LoginRequiredModalProps) {
	const titleId = React.useId();
	const loginHref = `/login?redirect=${encodeURIComponent(redirect)}`;

	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			aria-labelledby={titleId}
			className="w-[min(440px,100%)] border-[color:var(--color-neutral-panel)] text-hk-body bg-[radial-gradient(circle_at_82%_8%,var(--page-login-modal-glow),transparent_34%),linear-gradient(180deg,var(--hkui-content1),var(--hkui-content2))] shadow-[0_28px_70px_rgb(15_33_15_/_0.24),0_1px_0_rgb(255_255_255_/_0.8)_inset]"
			overlayClassName="max-[640px]:items-end max-[640px]:p-3"
			bodyClassName="p-0"
		>
			<div className="flex items-center px-4 pt-4">
				<div
					className="inline-flex size-12 items-center justify-center rounded-full border border-[var(--page-login-modal-disc-border)] bg-[var(--page-login-modal-disc-bg)] shadow-[var(--hkui-shadow-sm)] [&_img]:size-8 [&_img]:object-contain"
					aria-hidden="true"
				>
					<AppImage src={brandAssets.logoGrass} alt="" width={72} height={72} />
				</div>
			</div>
			<div className="grid gap-2 px-6 pt-5 pb-4 max-[640px]:px-4">
				<span className="text-hk-primary! text-xs font-semibold tracking-[0]">
					會員功能
				</span>
				<h2
					id={titleId}
					className="m-0 text-hk-heading text-2xl leading-[1.18] font-bold tracking-[0]"
				>
					{title}
				</h2>
				<p className="m-0 text-hk-muted text-sm leading-[1.65]">
					{description}
				</p>
			</div>
			<div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1.35fr)] gap-3 px-6 pb-6 max-[640px]:grid-cols-1 max-[640px]:px-4 max-[640px]:pb-4">
				<button
					type="button"
					className="inline-flex min-h-11 cursor-pointer items-center justify-center rounded-hk-pill border border-hk-border-soft bg-[var(--page-login-modal-ghost-bg)] px-4 text-sm font-semibold text-hk-body no-underline hover:bg-[var(--page-login-modal-ghost-hover-bg)] hover:text-hk-heading dark:text-[var(--page-status-link-fg)] dark:hover:text-[var(--page-status-link-hover-fg)]"
					onClick={onClose}
				>
					先逛逛
				</button>
				<Link
					className="inline-flex min-h-11 cursor-pointer items-center justify-center rounded-hk-pill border border-hk-primary bg-hk-primary px-4 text-sm font-semibold text-hk-inverse no-underline hover:border-hk-primary-hover hover:bg-hk-primary-hover"
					href={loginHref}
					hardNavigation
				>
					{actionLabel}
				</Link>
			</div>
		</Dialog>
	);
}
