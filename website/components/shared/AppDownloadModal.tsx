"use client";

import { QRCodeSVG } from "qrcode.react";
import AppIcon from "@/components/shared/AppIcon";
import { Dialog } from "@/components/ui/Dialog";

interface AppDownloadModalProps {
	open: boolean;
	onClose: () => void;
}

/**
 * 買家 app「Hawkr」只發 iOS（2026-06 上架）。QR 與 App Store 按鈕皆指向 App Store 頁。
 *
 * Built on the shared `Dialog`: the hand-rolled portal here had no focus trap,
 * no initial focus, no focus return, and a full-screen backdrop button that
 * was the dialog's first Tab stop. `Dialog` owns all of that (plus Escape and
 * the body scroll lock), so none of it is re-implemented.
 */
const APP_STORE_URL = "https://apps.apple.com/tw/app/id6757265571";

export default function AppDownloadModal({
	open,
	onClose,
}: AppDownloadModalProps) {
	return (
		<Dialog
			open={open}
			onClose={onClose}
			size="sm"
			className="w-[min(380px,100%)] bg-[radial-gradient(circle_at_82%_8%,var(--page-login-modal-glow),transparent_34%),linear-gradient(180deg,var(--hkui-content1),var(--hkui-content2))]"
			showCloseButton
			title={
				<>
					<span className="mb-1 block text-[length:var(--hkui-text-tiny)] font-semibold tracking-[0.04em] text-[var(--hkui-primary)]">
						Hawkr App
					</span>
					用 App 享受更好的體驗
				</>
			}
			description={
				<>
					下載 Hawkr App，享受完整購物功能、即時訊息通知與更多優惠。
					<br />
					掃描下方 QR Code 立即下載。
				</>
			}
			bodyClassName="flex flex-col items-center gap-[var(--hkui-space-3)] text-center"
		>
			<div className="inline-flex items-center justify-center rounded-[var(--hkui-radius-lg)] bg-[var(--color-neutral-cream)] p-[var(--hkui-space-4)] shadow-[0_8px_24px_rgb(15_33_15_/_0.12),0_1px_0_rgb(255_255_255_/_0.9)_inset] [&_svg]:block [&_svg]:size-[192px] max-[640px]:[&_svg]:size-[168px]">
				{/* QR sits on a light card in BOTH themes, so the modules are a
				    fixed dark ink — a theme-var foreground would flip light in
				    dark mode and leave a light-on-light, unscannable code. */}
				<QRCodeSVG
					value={APP_STORE_URL}
					size={192}
					level="M"
					marginSize={2}
					bgColor="transparent"
					fgColor="var(--color-ink-900)"
				/>
			</div>

			{/* Not a `nav`: a landmark nested inside a modal dialog is announced
			    as a page-level region that a screen-reader user cannot navigate
			    away to. A labelled group says the same thing without lying. */}
			<fieldset
				aria-label="App 商店連結"
				className="m-0 mt-[var(--hkui-space-3)] grid w-full grid-cols-2 gap-[var(--hkui-space-3)] border-0 p-0"
			>
				<a
					className="inline-flex min-h-11 items-center justify-center gap-[var(--hkui-space-2)] rounded-[var(--hkui-radius-full)] border border-[var(--hkui-divider)] bg-[var(--hkui-foreground)] px-[var(--hkui-space-4)] text-[length:var(--hkui-text-small)] font-semibold text-[var(--hkui-content1)] no-underline transition-[background,color] duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:bg-[var(--hkui-primary)] hover:text-[var(--hkui-primary-foreground)] motion-reduce:transition-none"
					href={APP_STORE_URL}
					target="_blank"
					rel="noopener noreferrer"
				>
					<AppIcon name="download" size={26} />
					App Store
				</a>
			</fieldset>
		</Dialog>
	);
}
