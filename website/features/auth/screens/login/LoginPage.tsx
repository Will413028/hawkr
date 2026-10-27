"use client";

import { useSearchParams } from "next/navigation";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import AuthBrandLoading from "@/components/shared/AuthBrandLoading";
import Link from "@/components/shared/Link";
import LoginShell from "@/features/auth/components/login/LoginShell";
import OAuthLoginButton from "@/features/auth/components/login/OAuthLoginButton";
import WebViewBrowserGuide, {
	type WebViewBrowserGuideVariant,
} from "@/features/auth/components/login/WebViewBrowserGuide";
import type { LoginProvider } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import { resolvePostLoginTarget } from "@/lib/auth/post-login-redirect";
import { redirectForRole, safeRedirectPath } from "@/lib/auth/redirect";
import { LOGIN_FAILED_MESSAGE } from "@/lib/auth/sso-callback";
import { useOAuthSignIn } from "@/lib/auth/useOAuthSignIn";
import {
	detectInAppBrowser,
	EMPTY_IN_APP_BROWSER_INFO,
	type InAppBrowserInfo,
} from "@/lib/browser/in-app-browser";
import { stripLocale } from "@/lib/intl/paths";
import { navigateDocument } from "@/lib/navigation/observability";

/** `?error=` 的值：SSO callback 沒有完成登入時帶回來（SsoCallbackPage）。 */
const ERROR_LABELS: Record<string, string> = {
	failed: LOGIN_FAILED_MESSAGE,
};

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
	const errorCode = searchParams.get("error") || "";
	const [error, setError] = React.useState<string | null>(
		errorCode ? (ERROR_LABELS[errorCode] ?? null) : null,
	);
	// True from the click until the browser leaves for the provider (or the
	// start fails) — the form can't flash idle or accept a second provider.
	const [signingIn, setSigningIn] = React.useState(false);
	const onLoginError = React.useCallback((message: string) => {
		setSigningIn(false);
		setError(message);
	}, []);
	const startOAuth = useOAuthSignIn(onLoginError);
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

	// middleware.ts never bounces /login (hawkr-website#259): a Clerk
	// session the monolith refuses (suspended/deleted account) would otherwise
	// loop the visitor away from this form. The AUTHORITATIVE check happens here
	// — once the signed-in account resolves to a real identity, leave
	// immediately. Hard nav (not router.replace): matches useCompleteLogin's
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
		// the provider; SsoCallbackPage owns the navigation after sign-in.
		if (signingIn || awayNavigationRef.current) return;
		if (!hydrated || !isLoggedIn || !identity) return;
		awayNavigationRef.current = true;
		setRedirectingAway(true);
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
	]);

	const signInWith = React.useCallback(
		(provider: LoginProvider) => {
			if (signingIn) return;
			setSigningIn(true);
			void startOAuth(provider, selectedRole, redirectAfter);
		},
		[signingIn, startOAuth, selectedRole, redirectAfter],
	);

	const subtitle = "用 Google 或 Apple 繼續，第一次使用會自動為你建立帳號。";
	const appleLoginAction = (placement: "page" | "sheet") => (
		<OAuthLoginButton
			key={placement}
			provider="apple"
			className={
				placement === "sheet"
					? "hkui-login__oauth-btn h-auto! min-h-[52px] whitespace-normal! px-[12px]! py-[12px] text-center leading-5"
					: OAUTH_BUTTON_CLASS
			}
			label={placement === "sheet" ? "使用 Apple 登入" : undefined}
			onClick={() => signInWith("apple")}
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
				{appleLoginAction("page")}
				<OAuthLoginButton
					provider="line"
					className={OAUTH_BUTTON_CLASS}
					onClick={() => signInWith("line")}
				/>
				{showBrowserGuide && (
					<WebViewBrowserGuide
						browser={inAppBrowser}
						role={selectedRole}
						variant={guideVariant}
						open={showBrowserGuide}
						onClose={closeBrowserGuide}
						appleAction={
							guideVariant === "sheet" ? appleLoginAction("sheet") : undefined
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
			{(signingIn || redirectingAway) && <AuthBrandLoading />}
		</LoginShell>
	);
}
