"use client";

import * as React from "react";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import { Button, Dialog } from "@/components/ui";
import { buttonClasses } from "@/components/ui/Button";
import {
	buildAndroidExternalBrowserIntent,
	buildIosChromeExternalBrowserUrl,
	buildLoginBrowserHandoffUrl,
	getInAppBrowserNativeMenuGuide,
	type InAppBrowserGuideArrow,
	type InAppBrowserInfo,
	type InAppBrowserNativeMenuGuide,
	type LoginRoleHint,
} from "@/lib/browser/in-app-browser";
import { cn } from "@/lib/utils/cn";

export type WebViewBrowserGuideVariant = "inline" | "sheet";

interface WebViewBrowserGuideProps {
	browser: InAppBrowserInfo;
	role: LoginRoleHint;
	variant: WebViewBrowserGuideVariant;
	/** 內建瀏覽器用不了 Google 時，改用的另一種登入方式（sheet 版放按鈕）。 */
	alternativeAction?: React.ReactNode;
	open?: boolean;
	onClose?: () => void;
}

type ActiveAction = "primary" | "copy" | null;

const FOCUSABLE_SELECTOR =
	"a[href], button:not([disabled]), [tabindex]:not([tabindex='-1'])";

/** Diagonal pointers are the vertical arrow turned 45° toward the corner. */
const GUIDE_ARROW_GLYPHS = {
	"left-up": { name: "arrow-up", rotateDeg: -45 },
	"right-up": { name: "arrow-up", rotateDeg: 45 },
	"left-down": { name: "arrow-down", rotateDeg: 45 },
	"right-down": { name: "arrow-down", rotateDeg: -45 },
} satisfies Record<
	InAppBrowserGuideArrow,
	{ name: AppIconName; rotateDeg: number }
>;

function NativeBrowserMenuHint({
	guide,
}: {
	guide: InAppBrowserNativeMenuGuide;
}) {
	const pointsLeft = guide.corner.endsWith("left");

	return (
		<div
			className={cn(
				"pointer-events-none fixed z-20 flex min-w-0 items-start gap-1.5",
				guide.corner.startsWith("top")
					? "top-4 max-w-[min(15rem,calc(100vw-96px))]"
					: "bottom-4 max-w-[min(15rem,calc(100vw-32px))]",
				pointsLeft ? "left-4" : "right-4",
				pointsLeft ? "flex-row-reverse text-left" : "text-right",
			)}
			data-webview-native-menu-hint="true"
			data-webview-guide-corner={guide.corner}
			data-webview-guide-arrow={guide.arrow}
			data-webview-guide-positioning="fixed"
		>
			<span className="grid min-w-0 gap-0.5 pt-1">
				<strong className="text-[length:var(--hkui-text-tiny)] leading-5 text-hk-cream">
					點{guide.cornerLabel}「{guide.menuSymbol}」
				</strong>
				<span className="text-[length:var(--hkui-text-tiny)] leading-5 text-hk-cream/72">
					選「{guide.actionLabel}」
				</span>
			</span>
			<AppIcon
				name={GUIDE_ARROW_GLYPHS[guide.arrow].name}
				size={32}
				className="-mt-1 shrink-0 text-hk-cream"
				style={{
					transform: `rotate(${GUIDE_ARROW_GLYPHS[guide.arrow].rotateDeg}deg)`,
				}}
			/>
		</div>
	);
}

function trapGuideFocus(event: React.KeyboardEvent<HTMLDivElement>) {
	if (event.key !== "Tab") return;
	const focusables = Array.from(
		event.currentTarget.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR),
	).filter(
		(element) =>
			!element.closest("[inert], [aria-hidden='true']") &&
			element.getAttribute("aria-disabled") !== "true",
	);
	if (!focusables.length) {
		event.preventDefault();
		event.currentTarget.focus({ preventScroll: true });
		return;
	}

	const first = focusables[0];
	const last = focusables[focusables.length - 1];
	const active = document.activeElement;
	if (
		event.shiftKey &&
		(active === first || !event.currentTarget.contains(active))
	) {
		event.preventDefault();
		last.focus({ preventScroll: true });
	} else if (
		!event.shiftKey &&
		(active === last || !event.currentTarget.contains(active))
	) {
		event.preventDefault();
		first.focus({ preventScroll: true });
	}
}

async function copyText(value: string): Promise<boolean> {
	try {
		if (navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(value);
			return true;
		}
	} catch {
		// Some in-app browsers expose Clipboard but deny it at runtime.
	}

	const textarea = document.createElement("textarea");
	try {
		textarea.value = value;
		textarea.setAttribute("readonly", "");
		textarea.style.position = "fixed";
		textarea.style.opacity = "0";
		document.body.appendChild(textarea);
		textarea.select();
		return document.execCommand("copy");
	} catch {
		return false;
	} finally {
		textarea.remove();
	}
}

function browserLabel(platform: InAppBrowserInfo["platform"]): string {
	if (platform === "ios") return "Safari 或 Chrome";
	if (platform === "android") return "Chrome";
	return "外部瀏覽器";
}

function primaryActionLabel(platform: InAppBrowserInfo["platform"]): string {
	if (platform === "ios") return "使用 Chrome 開啟";
	if (platform === "android") return "使用瀏覽器開啟";
	return "複製登入連結";
}

function guideCopy(
	browser: InAppBrowserInfo,
	nativeMenuGuide: InAppBrowserNativeMenuGuide,
) {
	const targetBrowser = browserLabel(browser.platform);
	const source = browser.appName ?? "目前的 App";

	return {
		title: `用 ${targetBrowser} 完成 Google 登入`,
		description: browser.isInApp
			? `${source} 的內建瀏覽器不支援 Google 登入，也可直接改用 LINE 登入。`
			: "Google 登入目前無法在此瀏覽器啟動，請改用其他瀏覽器開啟本頁。",
		instruction: `點${nativeMenuGuide.cornerLabel}「${nativeMenuGuide.menuSymbol}」，再選「${nativeMenuGuide.actionLabel}」。`,
	};
}

export default function WebViewBrowserGuide({
	browser,
	role,
	variant,
	alternativeAction,
	open = true,
	onClose,
}: WebViewBrowserGuideProps) {
	const [activeAction, setActiveAction] = React.useState<ActiveAction>(null);
	const [feedback, setFeedback] = React.useState("");
	const [externalBrowserHref, setExternalBrowserHref] = React.useState("");
	const externalActionRef = React.useRef<HTMLAnchorElement>(null);
	const nativeMenuGuide = getInAppBrowserNativeMenuGuide(browser);
	const isTopMenuGuide = nativeMenuGuide.corner.startsWith("top");
	const isLeftMenuGuide = nativeMenuGuide.corner.endsWith("left");
	const copy = guideCopy(browser, nativeMenuGuide);

	React.useEffect(() => {
		const handoffUrl = buildLoginBrowserHandoffUrl(window.location.href, role);
		if (browser.platform === "android") {
			setExternalBrowserHref(buildAndroidExternalBrowserIntent(handoffUrl));
			return;
		}
		if (browser.platform === "ios") {
			setExternalBrowserHref(buildIosChromeExternalBrowserUrl(handoffUrl));
			return;
		}
		setExternalBrowserHref("");
	}, [browser.platform, role]);

	React.useEffect(() => {
		if (variant !== "sheet" || !open || !externalBrowserHref) return;
		const frame = requestAnimationFrame(() => {
			externalActionRef.current?.focus({ preventScroll: true });
		});
		return () => cancelAnimationFrame(frame);
	}, [externalBrowserHref, open, variant]);

	const handoffUrl = React.useCallback(
		() => buildLoginBrowserHandoffUrl(window.location.href, role),
		[role],
	);

	const copyLoginLink = React.useCallback(
		async (action: ActiveAction = "copy") => {
			setActiveAction(action);
			try {
				const copied = await copyText(handoffUrl());
				setFeedback(
					copied
						? "登入連結已複製，請貼到 Safari 或 Chrome 網址列。"
						: "無法自動複製，請使用 App 選單分享本頁。",
				);
			} finally {
				setActiveAction(null);
			}
		},
		[handoffUrl],
	);

	const closeSheet = React.useCallback(() => {
		onClose?.();
	}, [onClose]);

	const actions = (
		<>
			{browser.platform === "android" || browser.platform === "ios" ? (
				<a
					ref={externalActionRef}
					href={externalBrowserHref || undefined}
					className={`${buttonClasses({ size: "lg", fullWidth: true })} ${
						variant === "sheet"
							? "h-auto! min-h-[52px] gap-[10px]! whitespace-normal! px-[12px]! py-[12px] text-center leading-5 shadow-[0_14px_30px_-16px_rgb(var(--color-ink-700-rgb)/0.82),0_1px_0_rgba(255,255,255,0.3)_inset]"
							: "h-[52px]!"
					}`}
					aria-disabled={
						!externalBrowserHref || activeAction !== null || undefined
					}
					data-webview-external-action="true"
					onClick={(event) => {
						if (!externalBrowserHref || activeAction !== null) {
							event.preventDefault();
							if (!externalBrowserHref) void copyLoginLink("primary");
							return;
						}
						setFeedback(
							browser.platform === "ios"
								? "正在開啟 Chrome；若沒有跳轉，請確認裝置已安裝 Chrome。"
								: "正在交給系統瀏覽器。",
						);
					}}
				>
					<span>{primaryActionLabel(browser.platform)}</span>
					<AppIcon name="arrow-right" size={20} />
				</a>
			) : (
				<Button
					size="lg"
					fullWidth
					isLoading={activeAction === "primary"}
					loadingLabel="複製登入連結中…"
					disabled={activeAction !== null}
					className={
						variant === "sheet"
							? "h-auto! min-h-[52px] whitespace-normal! px-[12px]! py-[12px] text-center leading-5"
							: "h-[52px]!"
					}
					onClick={() => void copyLoginLink("primary")}
				>
					{primaryActionLabel(browser.platform)}
				</Button>
			)}
			{variant !== "sheet" &&
				(browser.platform === "android" || browser.platform === "ios") && (
					<Button
						size="lg"
						variant="bordered"
						color="default"
						fullWidth
						isLoading={activeAction === "copy"}
						loadingLabel="複製登入連結中…"
						disabled={activeAction !== null}
						className="h-[52px]!"
						startContent={<AppIcon name="clipboard" size={20} />}
						onClick={() => void copyLoginLink()}
					>
						複製登入連結
					</Button>
				)}
		</>
	);

	const status = (
		<p
			className="m-0 min-h-5 text-center text-[length:var(--hkui-text-tiny)] leading-5 text-[var(--hkui-foreground)]"
			role="status"
			aria-live="polite"
		>
			{feedback}
		</p>
	);

	if (variant === "sheet") {
		return (
			<Dialog
				open={open}
				onClose={closeSheet}
				size="full"
				placement="center"
				showCloseButton={false}
				overlayClassName="bg-[rgb(var(--color-ink-950-rgb)/0.56)]! p-0! backdrop-blur-[18px]! backdrop-saturate-[82%]! dark:bg-[rgb(var(--color-ink-950-rgb)/0.68)]!"
				className="max-w-none! rounded-none! border-0! bg-transparent! shadow-none!"
				bodyClassName="flex min-h-0 flex-1 p-0! [scrollbar-gutter:auto]"
				aria-label="外部瀏覽器登入聚光指引"
				data-webview-guide="sheet"
				data-webview-guide-mode="spotlight"
				onKeyDown={trapGuideFocus}
			>
				<div
					className={cn(
						"relative mx-auto flex min-h-full w-full min-w-0 max-w-[960px] flex-col gap-5 overflow-x-hidden px-4 pt-4 text-hk-cream",
						isTopMenuGuide ? "pb-4" : "pb-[6.5rem]",
					)}
				>
					<span
						className="pointer-events-none absolute -left-24 top-[16%] size-72 rounded-full bg-[rgb(var(--color-ink-200-rgb)/0.14)] blur-[72px]"
						aria-hidden="true"
					/>
					<span
						className="pointer-events-none absolute -right-28 bottom-[8%] size-80 rounded-full bg-[rgba(255,255,255,0.1)] blur-[80px]"
						aria-hidden="true"
					/>

					<NativeBrowserMenuHint guide={nativeMenuGuide} />

					<header
						className={cn(
							"relative z-10 flex min-h-[64px] items-start gap-3",
							isTopMenuGuide && isLeftMenuGuide
								? "justify-end"
								: "justify-start",
						)}
					>
						<button
							type="button"
							className="grid size-[48px] shrink-0 touch-manipulation place-items-center text-hk-cream/78 transition-[color,transform] duration-200 hover:text-hk-cream active:scale-[0.94] focus-visible:!outline-none focus-visible:![box-shadow:0_0_0_3px_rgba(255,255,255,0.72)]"
							onClick={closeSheet}
							aria-label="關閉開啟方式"
						>
							<span
								className="text-[2rem] font-light leading-none"
								aria-hidden="true"
							>
								×
							</span>
						</button>
					</header>

					<div
						className="relative z-10 grid! min-w-0 flex-1 content-center gap-8 md:grid-cols-2 md:items-center md:gap-10"
						data-webview-guide-layout="true"
					>
						<section className="grid min-w-0 justify-items-center gap-4 text-center md:justify-items-start md:text-left!">
							<span className="grid size-[72px] place-items-center">
								<AppIcon name="discovery" size={64} className="text-hk-cream" />
							</span>

							<div className="grid w-full min-w-0 max-w-[30rem] gap-2">
								<h2 className="m-0 break-words text-[clamp(1.65rem,7vw,2.25rem)] font-bold leading-[1.16] tracking-[-0.025em] text-hk-cream [overflow-wrap:anywhere] [text-wrap:balance] md:text-[clamp(1.75rem,4vw,2.1rem)]">
									請使用外部瀏覽器完成登入
								</h2>
								<p className="m-0 text-[length:var(--hkui-text-small)] leading-6 text-hk-cream/78">
									{browser.appName ?? "目前的 App"} 內建瀏覽器不支援 Google
									登入，可使用 Chrome 或 LINE 繼續。
								</p>
							</div>
						</section>

						<section
							className="relative isolate grid w-full min-w-0 max-w-[420px] justify-self-center gap-3 md:justify-self-end before:pointer-events-none before:absolute before:-inset-10 before:-z-10 before:rounded-full before:bg-[radial-gradient(circle,rgb(var(--color-ink-100-rgb)/0.22),transparent_68%)] before:blur-2xl"
							data-webview-guide-spotlight="external-browser"
							aria-label="外部瀏覽器開啟操作區"
						>
							<div className="grid w-full">{actions}</div>
							{alternativeAction}
							<span className="sr-only" role="status" aria-live="polite">
								{feedback}
							</span>
						</section>
					</div>
				</div>
			</Dialog>
		);
	}

	return (
		<section
			className="grid gap-4 rounded-[var(--hkui-radius-lg)] border border-[color-mix(in_srgb,var(--hkui-primary)_38%,var(--hkui-divider))] bg-[color-mix(in_srgb,var(--hkui-primary)_7%,var(--hkui-content1))] p-4"
			aria-label="外部瀏覽器登入指引"
			aria-live="polite"
			data-webview-guide="inline"
		>
			<div className="flex items-start gap-3">
				<span className="grid size-12 shrink-0 place-items-center rounded-full border border-[color-mix(in_srgb,var(--hkui-primary)_24%,transparent)] bg-[var(--hkui-content1)]">
					<AppIcon
						name="discovery"
						size={38}
						className="text-[var(--hkui-primary)]"
					/>
				</span>
				<div className="grid min-w-0 gap-1">
					<h2 className="m-0 text-[length:var(--hkui-text-medium)] font-bold leading-6 text-[var(--hkui-foreground)]">
						{copy.title}
					</h2>
					<p className="m-0 text-[length:var(--hkui-text-tiny)] leading-5 text-[var(--hkui-foreground-muted)]">
						{copy.description}
					</p>
				</div>
			</div>

			<div className="flex items-center gap-3 rounded-[var(--hkui-radius-md)] border border-[var(--hkui-divider)] bg-[var(--hkui-content1)] px-3 py-2.5">
				<AppIcon
					name="more"
					size={22}
					className="shrink-0 text-[var(--hkui-primary)]"
				/>
				<span className="text-[length:var(--hkui-text-tiny)] font-medium leading-5 text-[var(--hkui-foreground)]">
					{copy.instruction}
				</span>
			</div>

			<div className="grid gap-2">{actions}</div>
			{status}
		</section>
	);
}
