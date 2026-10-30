"use client";

import type * as React from "react";
import Button from "@/components/ui/Button";
import type { LoginProvider } from "@/lib/api/auth";
import { cn } from "@/lib/utils/cn";

const GoogleGIcon = () => (
	<svg
		width="20"
		height="20"
		viewBox="0 0 24 24"
		aria-hidden="true"
		focusable="false"
	>
		<path
			fill="#4285F4"
			d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
		/>
		<path
			fill="#34A853"
			d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
		/>
		<path
			fill="#FBBC05"
			d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
		/>
		<path
			fill="#EA4335"
			d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
		/>
	</svg>
);

/** LINE 的品牌圖示（官方綠色）。 */
const LineIcon = () => (
	<svg
		width="20"
		height="20"
		viewBox="0 0 24 24"
		aria-hidden="true"
		focusable="false"
	>
		<path
			fill="#06C755"
			d="M19.365 9.863c.349 0 .63.285.63.631 0 .345-.281.63-.63.63H17.61v1.125h1.755c.349 0 .63.283.63.63 0 .344-.281.629-.63.629h-2.386c-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63h2.386c.346 0 .627.285.627.63 0 .349-.281.63-.63.63H17.61v1.125h1.755zm-3.855 3.016c0 .27-.174.51-.432.596-.064.021-.133.031-.199.031-.211 0-.391-.09-.51-.25l-2.443-3.317v2.94c0 .344-.279.629-.631.629-.346 0-.626-.285-.626-.629V8.108c0-.27.173-.51.43-.595.06-.023.136-.033.194-.033.195 0 .375.104.495.254l2.462 3.33V8.108c0-.345.282-.63.63-.63.345 0 .63.285.63.63v4.771zm-5.741 0c0 .344-.282.629-.631.629-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63.346 0 .628.285.628.63v4.771zm-2.466.629H4.917c-.345 0-.63-.285-.63-.629V8.108c0-.345.285-.63.63-.63.348 0 .63.285.63.63v4.141h1.756c.348 0 .629.283.629.63 0 .344-.282.629-.629.629M24 10.314C24 4.943 18.615.572 12 .572S0 4.943 0 10.314c0 4.811 4.27 8.842 10.035 9.608.391.082.923.258 1.058.59.12.301.079.766.038 1.08l-.164 1.02c-.045.301-.24 1.186 1.049.645 1.291-.539 6.916-4.078 9.436-6.975C23.176 14.393 24 12.458 24 10.314"
		/>
	</svg>
);

const PROVIDER_CONTENT: Record<
	LoginProvider,
	{ icon: () => React.JSX.Element; label: string }
> = {
	google: { icon: GoogleGIcon, label: "用 Google 繼續" },
	line: { icon: LineIcon, label: "用 LINE 繼續" },
};

interface Props {
	provider: LoginProvider;
	onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
	/**
	 * 設定時，這個登入方式在目前的瀏覽器（App 的內建瀏覽器）用不了：按鈕改為打開外部瀏覽器指引的
	 * dialog，值是那個 dialog 是否開著。
	 */
	guideOpen?: boolean;
	/** 取代預設的按鈕文字。 */
	label?: string;
	className?: string;
}

/**
 * 登入頁的社群登入按鈕。LINE 是共用的 `Button`；Google 是原生按鈕，只靠
 * `.hkui-login__oauth-btn` 的樣式（圖示與文字之間有間距，`Button` 沒有）。
 */
export default function OAuthLoginButton({
	provider,
	onClick,
	guideOpen,
	label,
	className,
}: Props) {
	const { icon: Icon, label: defaultLabel } = PROVIDER_CONTENT[provider];
	const guided = guideOpen !== undefined;
	if (provider === "google") {
		return (
			<button
				type="button"
				className={cn(
					className,
					"focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--hkui-focus)]",
				)}
				onClick={onClick}
				aria-haspopup={guided ? "dialog" : undefined}
				aria-expanded={guided ? guideOpen : undefined}
				data-webview-guide-trigger={guided ? provider : undefined}
				data-oauth-provider={provider}
			>
				<Icon />
				{label ?? defaultLabel}
			</button>
		);
	}
	return (
		<Button
			type="button"
			variant="bordered"
			color="default"
			size="lg"
			fullWidth
			className={className}
			onClick={onClick}
			aria-haspopup={guided ? "dialog" : undefined}
			aria-expanded={guided ? guideOpen : undefined}
			data-webview-guide-trigger={guided ? provider : undefined}
			data-oauth-provider={provider}
		>
			<Icon />
			{label ?? defaultLabel}
		</Button>
	);
}
