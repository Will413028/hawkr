"use client";

import {
	AuthenticateWithRedirectCallback,
	useAuth as useClerkAuth,
} from "@clerk/nextjs";
import { Code, ConnectError } from "@connectrpc/connect";
import { useSearchParams } from "next/navigation";
import * as React from "react";
import AuthBrandLoading from "@/components/shared/AuthBrandLoading";
import { completeSignIn, type LoginProvider } from "@/lib/api/auth";
import { parseRole, type RoleSlot } from "@/lib/auth/active-role";
import { useAuth } from "@/lib/auth/context";
import { resolvePostLoginTarget } from "@/lib/auth/post-login-redirect";
import { loginPath, safeRedirectPath } from "@/lib/auth/redirect";
import { parseLoginProvider, ssoCallbackPath } from "@/lib/auth/sso-callback";
import { useCompleteLogin } from "@/lib/auth/useCompleteLogin";
import { localizeHref } from "@/lib/intl/navigation";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import { navigateDocument } from "@/lib/navigation/observability";
import { useObservedRouter } from "@/lib/navigation/use-observed-router";

/**
 * Clerk 的 SDK 吞掉 OAuth callback 處理時的錯誤（isomorphic 的 handleRedirectCallback 不回傳結果），
 * 到這個時間還沒有 session 就當成失敗，回到登入頁。
 */
const SSO_CALLBACK_TIMEOUT_MS = 20_000;

interface CallbackContext {
	role: RoleSlot;
	redirectAfter: string;
	provider: LoginProvider | null;
	/** 沒有完成登入時回去的登入頁（帶著原本的工作區與目的地）。 */
	failedUrl: string;
}

/**
 * OAuth 回來的頁面（/login/sso-callback）。Clerk 還沒建立 session 時交給它的 callback 處理（新使用者在這裡
 * 由 sign-in 轉成 sign-up）；有 session 之後以 GetSession 讀帳號，照登入頁選的工作區離開。
 */
export default function SsoCallbackPage() {
	return (
		<React.Suspense fallback={<AuthBrandLoading />}>
			<SsoCallback />
		</React.Suspense>
	);
}

function leaveTo(url: string, actionId: string): void {
	navigateDocument(url, { componentId: "login", actionId, replace: true });
}

function SsoCallback() {
	const searchParams = useSearchParams();
	const locale = useRoutedLocale();
	const role = parseRole(searchParams?.get("role")) ?? "buyer";
	const redirectAfter = safeRedirectPath(searchParams?.get("redirect") ?? "");
	const provider = parseLoginProvider(searchParams?.get("provider"));
	const context: CallbackContext = {
		role,
		redirectAfter,
		provider,
		// Handed to Clerk, which navigates there itself, so it carries the page's language already.
		failedUrl: localizeHref(
			loginPath({ role, redirect: redirectAfter, error: "failed" }),
			locale,
		),
	};
	const { isLoaded, isSignedIn } = useClerkAuth();
	const [completing, setCompleting] = React.useState(false);
	if (isSignedIn && !completing) setCompleting(true);

	if (!isLoaded) return <AuthBrandLoading />;
	if (isSignedIn || completing) {
		return (
			<>
				<AuthBrandLoading />
				<CompleteSignIn {...context} />
			</>
		);
	}
	const callbackUrl = localizeHref(
		ssoCallbackPath({
			role,
			redirect: redirectAfter,
			provider,
		}),
		locale,
	);
	const failedUrl = context.failedUrl;
	return (
		<>
			<AuthBrandLoading />
			{/* Clerk 的 bot protection 在這裡掛 captcha（sign-in 轉成 sign-up 時）。 */}
			<div id="clerk-captcha" />
			<CallbackWatchdog failedUrl={failedUrl} />
			<AuthenticateWithRedirectCallback
				signInForceRedirectUrl={callbackUrl}
				signUpForceRedirectUrl={callbackUrl}
				signInUrl={failedUrl}
				signUpUrl={failedUrl}
				firstFactorUrl={failedUrl}
				secondFactorUrl={failedUrl}
				resetPasswordUrl={failedUrl}
				continueSignUpUrl={failedUrl}
				verifyEmailAddressUrl={failedUrl}
				verifyPhoneNumberUrl={failedUrl}
				signInProtectCheckUrl={failedUrl}
				signUpProtectCheckUrl={failedUrl}
			/>
		</>
	);
}

function CallbackWatchdog({ failedUrl }: { failedUrl: string }) {
	React.useEffect(() => {
		const timer = setTimeout(
			() => leaveTo(failedUrl, "sso_callback_timeout"),
			SSO_CALLBACK_TIMEOUT_MS,
		);
		return () => clearTimeout(timer);
	}, [failedUrl]);
	return null;
}

function CompleteSignIn({
	role,
	redirectAfter,
	provider,
	failedUrl,
}: CallbackContext) {
	const { rejectSession } = useAuth();
	const router = useObservedRouter("login");
	const { completeLogin } = useCompleteLogin();
	const started = React.useRef(false);
	React.useEffect(() => {
		if (started.current) return;
		started.current = true;
		completeSignIn(provider).then(
			(account) => {
				const target = resolvePostLoginTarget({
					requestedRole: role,
					redirectAfter,
					account,
					needsOnboarding: account.needsOnboarding,
				});
				completeLogin(target.destination, target.activeRole, account);
			},
			async (error: unknown) => {
				const code = ConnectError.from(error).code;
				// 單體不認這個 session（帳號已停權或刪除）：結束 Clerk 的 session，才能改用別的帳號登入。
				// 暫時答不出來時留著它，回到登入頁後由 auth context 的重試接手。
				if (code === Code.Unauthenticated || code === Code.PermissionDenied) {
					await rejectSession(
						code === Code.PermissionDenied
							? ConnectError.from(error)
							: undefined,
					);
				}
				router.replace(failedUrl);
			},
		);
	}, [
		rejectSession,
		completeLogin,
		provider,
		role,
		redirectAfter,
		failedUrl,
		router,
	]);
	return null;
}
