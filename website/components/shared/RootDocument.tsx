import { ClerkProvider } from "@clerk/nextjs";
import type { Viewport } from "next";
import { Suspense } from "react";
import { ViewTransitions } from "@/components/shared/view-transitions";
import "@/styles/globals.css";
import "@/styles/components/product-certification.css";
import "@/styles/components/product-modal.css";
import "@/styles/components/ui-kit.css";
import "@/styles/components/input-field-fx.css";
import JsonLd from "@/components/seo/JsonLd";
import AppProviders from "@/components/shared/AppProviders";
import CartSnackbar from "@/components/shared/CartSnackbar";
import DevRoleSwitcherSlot from "@/components/shared/DevRoleSwitcherSlot";
import FooterCategoryColumns from "@/components/shared/FooterCategoryColumns";
import { UIProvider } from "@/components/shared/HeroUIProvider";
import InitialPageLoader from "@/components/shared/InitialPageLoader";
import MainTabPending from "@/components/shared/MainTabPending";
import MobileBottomNav from "@/components/shared/MobileBottomNav";
import NotificationToastBridge from "@/components/shared/NotificationToastBridge";
import PageTransition from "@/components/shared/PageTransition";
import ProductQuickViewPendingBridge from "@/components/shared/ProductQuickViewPendingBridge";
import RoleAwareFooter from "@/components/shared/RoleAwareFooter";
import RoleAwareNavbar from "@/components/shared/RoleAwareNavbar";
import SellerWorkspaceLock from "@/components/shared/SellerWorkspaceLock";
import ChatDrawer from "@/features/messages/components/ChatDrawer";
import Analytics from "@/lib/analytics/Analytics";
import { AuthProvider } from "@/lib/auth/context";
import { ChatUnreadProvider } from "@/lib/chat/unread-context";
import { DEV_OVERLAYS_ENABLED } from "@/lib/config/env";
import { openhuninn } from "@/lib/fonts/openhuninn";
import { roboto } from "@/lib/fonts/roboto";
import { type AppLocale, messagesFor } from "@/lib/intl/config";
import { withLocale } from "@/lib/intl/paths";
import { INITIAL_LOADER_WATCHDOG_SCRIPT } from "@/lib/loading/initial-loader-watchdog";
import { INITIAL_PUBLIC_CONTENT_SCRIPT } from "@/lib/loading/initial-public-content";
import { STALE_CHUNK_RELOAD_BOOT_SCRIPT } from "@/lib/loading/stale-chunk-reload";
import { NotificationToastProvider } from "@/lib/notifications/toast-context";
import { NotificationUnreadProvider } from "@/lib/notifications/unread-context";
import { organizationSchema, webSiteSchema } from "@/lib/seo/jsonld";
import { THEME_BOOT_SCRIPT } from "@/lib/theme/context";
import { TEXT_SCALE_BOOT_SCRIPT } from "@/lib/theme/text-scale";

/**
 * `interactive-widget=resizes-content`：手機軟鍵盤彈出時讓 layout viewport
 * （含 `dvh`/`vh`）一起縮短到鍵盤上方，full-height 的聊天室就會收合在鍵盤之上，
 * 而不是被瀏覽器整頁往上頂、把表頭/頁尾推出畫面。
 *
 * ⚠️ **不要加 `viewportFit: "cover"`**（web#417 的明確裁決）。
 *
 * 本站**不採用 safe-area 版面**：iPhone 的 home indicator 區域由瀏覽器自己負責
 * ——沒有 `viewport-fit: cover` 時，Safari/Chrome 的 layout viewport 一開始就
 * **不包含**安全區，瀏覽器已把內容縮排在圓角與 home indicator 之內。也就是說
 * 「底部被遮住」這件事在現行設定下不會發生，不需要任何 `env()` 補償。
 *
 * 加上 `viewport-fit: cover` 會反過來把 layout viewport 撐到整塊實體螢幕，
 * **此時才**需要全站逐處用 `env(safe-area-inset-*)` 把內容縮回來。repo 內原本
 * 散落 12+ 處 `env(safe-area-inset-*)`，在沒有 cover 的前提下它們全部恆等於 0
 * （死碼），已於 web#417 全數移除、視覺零變化。
 *
 * 因此「補一行 viewport 設定」不是一行改動：它會在同一刻讓十幾處版面同時改變，
 * 而那些補償寫法已經不存在了。若日後真要採用 safe-area，正確順序是
 * **先**逐處把淨空補回來並真機驗證，**最後**才打開這個開關。
 */
export const rootViewport: Viewport = {
	width: "device-width",
	initialScale: 1,
	interactiveWidget: "resizes-content",
};

/**
 * 整份 HTML 文件：每個根 layout 都用它（`app/[locale]/layout.tsx` 是買家與賣家介面，
 * `app/(delegated)/layout.tsx` 是只有 zh-TW 的代行工作區，`app/global-not-found.tsx` 是找不到路由的 404）。
 * `<html lang>` 與整棵 provider 樹的語系都由 `locale` 決定（ADR 2026-10-17 D1：lang 跟 locale，值是 zh-TW、en）。
 */
export default function RootDocument({
	locale,
	children,
}: {
	locale: AppLocale;
	children: React.ReactNode;
}) {
	return (
		<html
			lang={locale}
			// Font CSS variables MUST live on <html> (:root), not <body>:
			// the font tokens (--font-primary/--font-brand/--font-chinese) are
			// declared in :root and reference --font-roboto/--font-openhuninn, so
			// those vars have to be in :root scope or the whole token computes to
			// the guaranteed-invalid value and headings/body fall back to the
			// Tailwind default sans.
			className={`${roboto.variable} ${openhuninn.variable} initial-loader-active`}
			data-scroll-behavior="smooth"
			// The theme boot script mutates <html> before hydration to avoid
			// first-paint colour flash, which can otherwise trigger a hydration
			// attribute warning.
			suppressHydrationWarning
		>
			<head>
				<JsonLd data={[organizationSchema(locale), webSiteSchema(locale)]} />
			</head>
			<body>
				{/* Keep synchronous bootstrap code as the first React-owned body
				    nodes. App Router serializes inline scripts declared in <head>
				    into the body stream without adding them to the body fiber, so
				    hydration can see SCRIPT where it expects InitialPageLoader.
				    These scripts still block parsing before visual body content. */}
				{/* OpenNext keepNames compatibility shim; must precede theme code. */}
				<script
					id="hk-name-shim"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted shim
					dangerouslySetInnerHTML={{
						__html: "self.__name||(self.__name=function(t){return t});",
					}}
				/>
				<script
					id="hk-theme-boot"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted boot script
					dangerouslySetInnerHTML={{ __html: THEME_BOOT_SCRIPT }}
				/>
				{/* Text-size preference (字體大小：小 / 中 / 大). Must run with the
				    theme boot script — it changes the root font-size, so applying it
				    after hydration would reflow the whole document. */}
				<script
					id="hk-text-scale-boot"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted boot script
					dangerouslySetInnerHTML={{ __html: TEXT_SCALE_BOOT_SCRIPT }}
				/>
				<script
					id="hk-initial-loader-watchdog"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted watchdog
					dangerouslySetInnerHTML={{
						__html:
							INITIAL_LOADER_WATCHDOG_SCRIPT + INITIAL_PUBLIC_CONTENT_SCRIPT,
					}}
				/>
				<script
					id="hk-stale-chunk-reload-boot"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted boot script
					dangerouslySetInnerHTML={{
						__html: STALE_CHUNK_RELOAD_BOOT_SCRIPT,
					}}
				/>
				{/* Block native image dragging without blocking files dragged into
				    upload wrapper elements. */}
				<script
					id="hk-image-drag-guard"
					// biome-ignore lint/security/noDangerouslySetInnerHtml: trusted boot script
					dangerouslySetInnerHTML={{
						__html:
							"document.addEventListener('dragstart',function(e){if(e.target&&e.target.tagName==='IMG')e.preventDefault()},true)",
					}}
				/>
				{/* Keep the client context inside the document body. A client
					    provider above <html> can be detached from the replayed route
					    subtree during a Cloudflare/Next SSR POST, leaving shared Link
					    consumers without their ViewTransitions context. */}
				<ViewTransitions>
					<ProductQuickViewPendingBridge />
					<InitialPageLoader />
					{/* SVG noise filter for frosted glass texture */}
					<svg
						style={{
							position: "absolute",
							width: 0,
							height: 0,
							pointerEvents: "none",
						}}
						aria-hidden="true"
					>
						<defs>
							<filter id="frosted-noise">
								<feTurbulence
									type="fractalNoise"
									baseFrequency="0.65"
									numOctaves={3}
									stitchTiles="stitch"
									result="noise"
								/>
								<feColorMatrix
									type="saturate"
									values="0"
									in="noise"
									result="grayNoise"
								/>
								<feBlend in="SourceGraphic" in2="grayNoise" mode="multiply" />
							</filter>
						</defs>
					</svg>

					{/* Sign-in state is Clerk's (ADR 2026-09-27-identity-clerk); the login
					    screens are our own, so Clerk's prebuilt UI bundle is never
					    prefetched. */}
					<ClerkProvider
						signInUrl={withLocale("/login", locale)}
						signUpUrl={withLocale("/login", locale)}
						afterSignOutUrl={withLocale("/", locale)}
						prefetchUI={false}
						telemetry={false}
					>
						<AppProviders locale={locale} messages={messagesFor(locale)}>
							<AuthProvider>
								<Analytics />
								<NotificationUnreadProvider>
									<ChatUnreadProvider>
										<NotificationToastProvider>
											<UIProvider>
												<Suspense fallback={null}>
													<SellerWorkspaceLock />
												</Suspense>
												<div className="page-wrapper">
													{/* Keep the root navbar free of server-side async work. The
												    client usePathname() sits behind a persistent Suspense
												    boundary, but unlike the old cookies()/gateway boundary
												    this one completes in the static shell and cannot be
												    stranded when /cart finishes streaming. */}
													<Suspense
														fallback={
															<div
																className="root-nav-layout-reservation"
																aria-hidden="true"
															/>
														}
													>
														<RoleAwareNavbar />
													</Suspense>
													<main className="page-content">
														{/* Keep route children outside a root Suspense boundary so
											    notFound() can set the document HTTP status. The controller
											    remains isolated because it owns no route content. */}
														<Suspense fallback={null}>
															<PageTransition controllerOnly />
															<MainTabPending />
														</Suspense>
														<div className="page-transition">{children}</div>
													</main>
													<Suspense fallback={null}>
														<MobileBottomNav />
													</Suspense>
													<Suspense fallback={null}>
														<RoleAwareFooter
															categoryColumns={<FooterCategoryColumns />}
														/>
													</Suspense>
												</div>
												{/* 掛載條件走共用開關而非各自寫 NODE_ENV：本機 E2E 要能把
											    所有 dev-only 浮動工具一次關掉，量到與 CI 相同的
											    DOM（web#425）。 */}
												{DEV_OVERLAYS_ENABLED && (
													<Suspense fallback={null}>
														<DevRoleSwitcherSlot />
													</Suspense>
												)}
												<CartSnackbar />
												{/* App-wide in-place chat: any 發訊息 / 聯絡買家 /
												    聊天 button opens this drawer via the chat-drawer
												    store. Lives under AuthProvider so the embedded
												    ThreadClient has session + query context.
												    Reads usePathname() at render (navigation-dismiss),
												    which is a request-time read — so it
												    needs the same <Suspense> boundary as the other
												    usePathname()-reading chrome above, or it blocks the
												    static shell during streaming. */}
												<Suspense fallback={null}>
													<ChatDrawer />
												</Suspense>
												{/* Effects-only, renders null, reads no request-time
												    data at render — safe to mount bare (no Suspense
												    dynamic boundary needed). */}
												<NotificationToastBridge />
											</UIProvider>
										</NotificationToastProvider>
									</ChatUnreadProvider>
								</NotificationUnreadProvider>
							</AuthProvider>
						</AppProviders>
					</ClerkProvider>
				</ViewTransitions>
			</body>
		</html>
	);
}
