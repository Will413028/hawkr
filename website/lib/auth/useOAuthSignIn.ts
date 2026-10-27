"use client";

import { useClerk, useSignIn } from "@clerk/nextjs";
import * as Sentry from "@sentry/nextjs";
import * as React from "react";
import type { LoginProvider } from "@/lib/api/auth";
import { env } from "@/lib/config/env";
import { localizeHref } from "@/lib/intl/navigation";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import type { RoleSlot } from "./active-role";
import { LOGIN_FAILED_MESSAGE, ssoCallbackPath } from "./sso-callback";

const STRATEGY = {
	google: "oauth_google",
	apple: "oauth_apple",
	line: "oauth_line",
} as const satisfies Record<LoginProvider, string>;

type ClerkInstance = ReturnType<typeof useClerk>;

/**
 * Clerk 的 SDK 載入失敗（script 被擋、離線）時 resolve：`signIn.sso()` 會一直等 SDK 載入，沒有這個就會
 * 永遠卡在載入畫面。
 */
function whenClerkFails(clerk: ClerkInstance): {
	failed: Promise<void>;
	dispose: () => void;
} {
	let dispose = () => {};
	const failed = new Promise<void>((resolve) => {
		if (clerk.status === "error") {
			resolve();
			return;
		}
		const onStatus = (status: ClerkInstance["status"]) => {
			if (status === "error") resolve();
		};
		clerk.on("status", onStatus);
		dispose = () => clerk.off("status", onStatus);
	});
	return { failed, dispose };
}

/**
 * 以 Clerk 的 OAuth redirect 登入（Clerk 的自訂流程，畫面是自己的）。成功時瀏覽器離開這個頁面去 provider，
 * 之後回到 SSO_CALLBACK_PATH；resolve 為 false 表示沒有離開，錯誤已經交給 `onError`。
 */
export function useOAuthSignIn(onError: (message: string) => void) {
	const { signIn } = useSignIn();
	const clerk = useClerk();
	const locale = useRoutedLocale();
	return React.useCallback(
		async (
			provider: LoginProvider,
			role: RoleSlot,
			redirect: string,
		): Promise<boolean> => {
			// Clerk sends the browser there itself, so the path carries the page's language already.
			const callback = localizeHref(
				ssoCallbackPath({ role, redirect, provider }),
				locale,
			);
			const guard = whenClerkFails(clerk);
			try {
				const outcome = await Promise.race([
					signIn.sso({
						strategy: STRATEGY[provider],
						redirectUrl: callback,
						redirectCallbackUrl: callback,
					}),
					guard.failed.then(() => ({ error: "clerk_unavailable" as const })),
				]);
				if (!outcome.error) return true;
				const code =
					typeof outcome.error === "string"
						? outcome.error
						: (outcome.error.code ?? "unknown");
				Sentry.captureMessage("web login failed to start", {
					level: "warning",
					tags: {
						auth_provider: provider,
						clerk_error_code: code,
						flavor: env.flavor,
					},
				});
			} catch (error) {
				Sentry.captureException(error, {
					level: "warning",
					tags: { auth_provider: provider, flavor: env.flavor },
				});
			} finally {
				guard.dispose();
			}
			onError(LOGIN_FAILED_MESSAGE);
			return false;
		},
		[signIn, clerk, locale, onError],
	);
}
