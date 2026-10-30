"use client";

import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthBrandLoading from "@/components/shared/AuthBrandLoading";
import Link from "@/components/shared/Link";
import LoginShell from "@/features/auth/components/login/LoginShell";
import OAuthLoginButton from "@/features/auth/components/login/OAuthLoginButton";
import WebViewBrowserGuide, {
	type WebViewBrowserGuideVariant,
} from "@/features/auth/components/login/WebViewBrowserGuide";
import { trackLogin } from "@/lib/analytics/events";
import type { LoginProvider } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import {
	loginErrorKey,
	oauthStartUrl,
	parseLoginProvider,
} from "@/lib/auth/oauth-start";
import { resolvePostLoginTarget } from "@/lib/auth/post-login-redirect";
import {
	loginPath,
	redirectForRole,
	safeRedirectPath,
} from "@/lib/auth/redirect";
import {
	detectInAppBrowser,
	EMPTY_IN_APP_BROWSER_INFO,
	type InAppBrowserInfo,
} from "@/lib/browser/in-app-browser";
import { localizeHref } from "@/lib/intl/navigation";
import { stripLocale } from "@/lib/intl/paths";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import { navigateDocument } from "@/lib/navigation/observability";

const OAUTH_BUTTON_CLASS = "hkui-login__oauth-btn h-[52px]!";

interface LoginPageProps {
	initialInAppBrowser?: InAppBrowserInfo;
}

export default function LoginPage({
	initialInAppBrowser = EMPTY_IN_APP_BROWSER_INFO,
}: LoginPageProps) {
	return (
		<React.Suspense fallback={<AuthBrandLoading />}>
			<LoginPageInner initialInAppBrowser={initialInAppBrowser} />
		</React.Suspense>
	);
}

function LoginPageInner({
	initialInAppBrowser,
}: {
	initialInAppBrowser: InAppBrowserInfo;
}) {
	const searchParams = useSearchParams();
	if (!searchParams) return <AuthBrandLoading />;
	return (
		<LoginPageContent
			searchParams={searchParams}
			initialInAppBrowser={initialInAppBrowser}
		/>
	);
}

function LoginPageContent({
	searchParams,
	initialInAppBrowser,
}: {
	searchParams: NonNullable<ReturnType<typeof useSearchParams>>;
	initialInAppBrowser: InAppBrowserInfo;
}) {
	// 單體的 callback 沒有完成登入時 302 回這頁並帶 `?error=<代碼>`（weblogin.go）；代碼對到三句之一
	// （取消授權、服務暫時不能用、其他失敗），不認得的代碼不顯示。
	const tError = useTranslations("auth.error");
	const errorKey = loginErrorKey(searchParams.get("error"));
	const error = errorKey ? tError(errorKey) : null;
	const locale = useRoutedLocale();
	// 單體 302 回來時帶 `?provider=`：剛從 Google 或 LINE 登入完成。
	const returnedProvider = parseLoginProvider(searchParams.get("provider"));
	// True from the click until the browser leaves for the start URL — the form
	// can't flash idle or accept a second provider.
	const [signingIn, setSigningIn] = React.useState(false);
	// Back from the provider's page restores this page from the bfcache with the
	// overlay still up; the flow it covered is no longer running.
	React.useEffect(() => {
		const onPageShow = (event: PageTransitionEvent) => {
			if (event.persisted) setSigningIn(false);
		};
		window.addEventListener("pageshow", onPageShow);
		return () => window.removeEventListener("pageshow", onPageShow);
	}, []);
	const [inAppBrowser, setInAppBrowser] =
		React.useState<InAppBrowserInfo>(initialInAppBrowser);
	const [browserGuideOpen, setBrowserGuideOpen] = React.useState(false);
	const browserGuideTriggerRef = React.useRef<HTMLButtonElement | null>(null);
	React.useEffect(() => {
		setInAppBrowser(detectInAppBrowser());
	}, []);
	const openBrowserGuide = React.useCallback(
		(event: React.MouseEvent<HTMLButtonElement>) => {
			browserGuideTriggerRef.current = event.currentTarget;
			setBrowserGuideOpen(true);
		},
		[],
	);
	const closeBrowserGuide = React.useCallback(() => {
		setBrowserGuideOpen(false);
		requestAnimationFrame(() => {
			browserGuideTriggerRef.current?.focus({ preventScroll: true });
		});
	}, []);

	const roleHint = searchParams.get("role");
	const initialRole: "buyer" | "seller" =
		roleHint === "seller" ? "seller" : "buyer";
	const [selectedRole, setSelectedRole] = React.useState<"buyer" | "seller">(
		initialRole,
	);
	const redirectAfter = safeRedirectPath(searchParams.get("redirect") || "");
	const requestedGuideVariant = searchParams.get("webviewGuide");
	const guideVariant: WebViewBrowserGuideVariant =
		requestedGuideVariant === "inline" ? "inline" : "sheet";
	const requiresBrowserGuide = inAppBrowser.isInApp;
	const showBrowserGuide = requiresBrowserGuide && browserGuideOpen;

	// middleware.ts never bounces /login (hawkr-website#259): a session cookie
	// the monolith refuses (suspended/deleted account) would otherwise
	// loop the visitor away from this form. The AUTHORITATIVE check happens here
	// — once the signed-in account resolves to a real identity, leave
	// immediately. Hard nav (not router.replace): matches the provider-return redirect's
	// own reasoning, and an already-logged-in visitor landing here has nothing
	// to lose from a full reload.
	const { identity, isLoggedIn, hydrated, activeRole, switchRole } = useAuth();
	const [redirectingAway, setRedirectingAway] = React.useState(false);
	const awayNavigationRef = React.useRef(false);
	// Guard a self-referential `?redirect=/login`: middleware used to be the
	// backstop against that (it always intercepted /login), so redirectForRole
	// never had to. Now that /login is unguarded, redirecting straight back to
	// /login would loop forever.
	const awayTarget = stripLocale(redirectAfter).startsWith("/login")
		? ""
		: redirectAfter;
	React.useEffect(() => {
		// Once a provider flow is in progress, the browser is about to leave for
		// the provider.
		if (signingIn || awayNavigationRef.current) return;
		if (!hydrated || !isLoggedIn || !identity) return;
		awayNavigationRef.current = true;
		setRedirectingAway(true);
		// 從 provider 回來（單體 302 到這頁）才是剛登入；已經登入的人直接打開這頁不算。
		if (returnedProvider) trackLogin(returnedProvider);
		const requestedRole =
			roleHint === "buyer" || roleHint === "seller" ? roleHint : activeRole;
		const target = resolvePostLoginTarget({
			requestedRole,
			redirectAfter: awayTarget,
			account: identity,
			// GetSession does not expose this field. Preserve the existing
			// logged-in behavior for an owned seller role; a buyer-only seller intent
			// still fails closed to onboarding through hasSellerRole=false.
			needsOnboarding: false,
		});
		if (target.activeRole && target.activeRole !== activeRole) {
			switchRole(target.activeRole);
		}
		if (
			!navigateDocument(
				redirectForRole(target.destination, target.activeRole),
				{ componentId: "login", actionId: "already_authenticated" },
			)
		) {
			awayNavigationRef.current = false;
			setRedirectingAway(false);
		}
	}, [
		signingIn,
		hydrated,
		isLoggedIn,
		identity,
		activeRole,
		awayTarget,
		roleHint,
		switchRole,
		returnedProvider,
	]);

	const signInWith = React.useCallback(
		(provider: LoginProvider) => {
			if (signingIn) return;
			// 單體登入成功後 302 回這頁（帶著工作區、目的地與登入方式），上面的 effect 讀到帳號就離開。
			const returnTo = localizeHref(
				loginPath({ role: selectedRole, redirect: redirectAfter, provider }),
				locale,
			);
			setSigningIn(true);
			if (
				!navigateDocument(oauthStartUrl(provider, returnTo), {
					componentId: "login",
					actionId: `oauth_start_${provider}`,
				})
			) {
				setSigningIn(false);
			}
		},
		[signingIn, selectedRole, redirectAfter, locale],
	);

	const subtitle = "用 Google 或 LINE 繼續，第一次使用會自動為你建立帳號。";
	const lineLoginAction = (placement: "page" | "sheet") => (
		<OAuthLoginButton
			key={placement}
			provider="line"
			className={
				placement === "sheet"
					? "hkui-login__oauth-btn h-auto! min-h-[52px] whitespace-normal! px-[12px]! py-[12px] text-center leading-5"
					: OAUTH_BUTTON_CLASS
			}
			label={placement === "sheet" ? "使用 LINE 登入" : undefined}
			onClick={() => signInWith("line")}
		/>
	);

	return (
		<LoginShell
			eyebrow="Hawkr"
			title="登入 / 註冊"
			subtitle={subtitle}
			hero={
				<span className="hkui-login__hero-mark">
					<AppIcon name="login" size={88} title="登入" />
				</span>
			}
		>
			<div
				className="grid grid-cols-2 gap-2 mb-[var(--hkui-space-4)] p-1 bg-[var(--hkui-content2)] rounded-[var(--radius-pill)]"
				role="tablist"
				aria-label="身份切換"
			>
				<button
					type="button"
					role="tab"
					aria-selected={selectedRole === "buyer"}
					className={[
						"inline-flex min-w-0 items-center justify-center gap-2.5 whitespace-nowrap py-2 px-[18px] border-0 font-inherit text-[length:var(--text-sm)] font-semibold rounded-[var(--radius-pill)] cursor-pointer transition-[background-color,color,box-shadow] duration-160 ease-out enabled:hover:text-[var(--hkui-foreground)] disabled:opacity-50 disabled:cursor-not-allowed max-[360px]:gap-1.5 max-[360px]:px-2.5",
						selectedRole === "buyer"
							? "bg-[var(--hkui-content1)] text-[var(--hkui-primary-700)] shadow-[0_4px_12px_rgb(var(--color-ink-900-rgb)/0.18),inset_0_0_0_1px_rgb(var(--color-ink-900-rgb)/0.16)]"
							: "bg-transparent text-[var(--hkui-foreground-muted)]",
					]
						.filter(Boolean)
						.join(" ")}
					onClick={() => setSelectedRole("buyer")}
				>
					<span
						className={[
							"inline-grid place-items-center w-8 h-8 rounded-[var(--radius-pill)] bg-[var(--color-neutral-cream)] dark:bg-[rgba(245,233,195,0.1)] overflow-hidden shrink-0 text-[var(--action-primary,var(--color-ink-800))] transition-[border-color,color] duration-160 ease-out max-[360px]:h-7 max-[360px]:w-7 [&>svg]:w-5 [&>svg]:h-5 [&>svg]:block [&>div]:grid [&>div]:place-items-center [&>div_svg]:block",
							selectedRole === "buyer"
								? "border-2 border-[var(--hkui-primary)]"
								: "border border-[rgba(0,0,0,0.06)] dark:border-[rgba(245,222,165,0.2)]",
						]
							.filter(Boolean)
							.join(" ")}
						aria-hidden="true"
					>
						<AppIcon name="bag" size={22} />
					</span>
					我是買家
				</button>
				<button
					type="button"
					role="tab"
					aria-selected={selectedRole === "seller"}
					className={[
						"inline-flex min-w-0 items-center justify-center gap-2.5 whitespace-nowrap py-2 px-[18px] border-0 font-inherit text-[length:var(--text-sm)] font-semibold rounded-[var(--radius-pill)] cursor-pointer transition-[background-color,color,box-shadow] duration-160 ease-out enabled:hover:text-[var(--hkui-foreground)] disabled:opacity-50 disabled:cursor-not-allowed max-[360px]:gap-1.5 max-[360px]:px-2.5",
						selectedRole === "seller"
							? "bg-[var(--hkui-content1)] text-[var(--hkui-primary-700)] shadow-[0_4px_12px_rgb(var(--color-ink-900-rgb)/0.18),inset_0_0_0_1px_rgb(var(--color-ink-900-rgb)/0.16)]"
							: "bg-transparent text-[var(--hkui-foreground-muted)]",
					]
						.filter(Boolean)
						.join(" ")}
					onClick={() => setSelectedRole("seller")}
				>
					<span
						className={[
							"inline-grid place-items-center w-8 h-8 rounded-[var(--radius-pill)] bg-[var(--color-neutral-cream)] dark:bg-[rgba(245,233,195,0.1)] overflow-hidden shrink-0 text-[var(--action-primary,var(--color-ink-800))] transition-[border-color,color] duration-160 ease-out max-[360px]:h-7 max-[360px]:w-7 [&>svg]:w-5 [&>svg]:h-5 [&>svg]:block [&>div]:grid [&>div]:place-items-center [&>div_svg]:block",
							selectedRole === "seller"
								? "border-2 border-[var(--hkui-primary)]"
								: "border border-[rgba(0,0,0,0.06)] dark:border-[rgba(245,222,165,0.2)]",
						]
							.filter(Boolean)
							.join(" ")}
						aria-hidden="true"
					>
						<AppIcon name="home" size={22} />
					</span>
					我是賣家
				</button>
			</div>

			<div className="hkui-login__oauth hkui-login__oauth--big">
				<OAuthLoginButton
					provider="google"
					className="hkui-login__oauth-btn"
					guideOpen={requiresBrowserGuide ? showBrowserGuide : undefined}
					onClick={
						requiresBrowserGuide ? openBrowserGuide : () => signInWith("google")
					}
				/>
				{lineLoginAction("page")}
				{showBrowserGuide && (
					<WebViewBrowserGuide
						browser={inAppBrowser}
						role={selectedRole}
						variant={guideVariant}
						open={showBrowserGuide}
						onClose={closeBrowserGuide}
						alternativeAction={
							guideVariant === "sheet" ? lineLoginAction("sheet") : undefined
						}
					/>
				)}
			</div>
			{error && (
				<p className="hkui-login__hint hkui-login__hint--error" role="alert">
					{error}
				</p>
			)}
			<p className="hkui-login__terms">
				繼續代表你同意 <Link href="/terms">服務條款</Link> 與{" "}
				<Link href="/privacy">隱私政策</Link>。
			</p>
			{/* Fixed fullscreen overlay while the browser leaves for the provider. */}
			{(signingIn || redirectingAway || (returnedProvider && !hydrated)) && (
				<AuthBrandLoading />
			)}
		</LoginShell>
	);
}
