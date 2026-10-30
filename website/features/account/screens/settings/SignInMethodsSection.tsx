"use client";

import { useQuery } from "@tanstack/react-query";
import { useSearchParams } from "next/navigation";
import { useTranslations } from "next-intl";
import { buttonClasses } from "@/components/ui/Button";
import { fetchLinkedProviders, type LoginProvider } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import { linkErrorKey, linkStartUrl } from "@/lib/auth/oauth-start";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { localizeHref } from "@/lib/intl/navigation";
import { useRoutedLocale } from "@/lib/intl/routed-locale";
import { queryKeys } from "@/lib/query/keys";
import { DISPLAY_ROW } from "./settings-row";

/** 連結完成或失敗後單體 302 回來的設定頁（站內路徑，語系由出口加）。 */
export const SETTINGS_PATH = "/account/settings";

/** provider 名稱是品牌名，不翻譯。 */
const PROVIDERS: readonly { provider: LoginProvider; name: string }[] = [
	{ provider: "google", name: "Google" },
	{ provider: "line", name: "LINE" },
];

/**
 * 設定頁「帳號安全」的「登入方式」：每家一列，已連結顯示「已連結」，未連結是導向單體
 * `/api/auth/{provider}/start?link=1` 的連結（整頁導覽，不是 fetch）。連結失敗時單體回到這頁並帶
 * `?link_error=<代碼>`，顯示在列上方。
 */
export default function SignInMethodsSection() {
	const t = useTranslations("account.signInMethods");
	const tCommon = useTranslations("common");
	const tAuthError = useTranslations("auth.error");
	const tLinkError = useTranslations("auth.linkError");
	const locale = useRoutedLocale();
	const { hydrated, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const searchParams = useSearchParams();
	const query = useQuery({
		queryKey: queryKeys.buyer.linkedProviders(identity),
		queryFn: fetchLinkedProviders,
		enabled: hydrated && isLoggedIn,
	});

	const linkError = linkErrorKey(searchParams?.get("link_error"));
	const errorText = !linkError
		? null
		: linkError.ns === "auth.error"
			? tAuthError(linkError.key)
			: tLinkError(linkError.key);
	const returnTo = localizeHref(SETTINGS_PATH, locale);
	const linked = new Set(query.data ?? []);

	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>{t("title")}</h2>
			</header>
			{errorText ? (
				<p
					className="my-[var(--hkui-space-3)] p-[var(--hkui-space-3)_var(--hkui-space-4)] rounded-[var(--hkui-radius-md)] bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)] text-[length:var(--hkui-text-small)]"
					role="alert"
				>
					{errorText}
				</p>
			) : null}
			<ul className="list-none m-0 p-0 flex flex-col gap-2">
				{PROVIDERS.map(({ provider, name }) => (
					<li key={provider} className={DISPLAY_ROW}>
						<strong className="text-sm font-semibold text-hk-heading">
							{name}
						</strong>
						{query.isPending ? (
							<small className="text-xs text-foreground-soft">
								{tCommon("loading")}
							</small>
						) : query.isError ? (
							<small className="text-xs text-foreground-soft" role="alert">
								{tCommon("error")}
							</small>
						) : linked.has(provider) ? (
							<small className="text-xs text-foreground-soft">
								{t("linked")}
							</small>
						) : (
							<a
								href={linkStartUrl(provider, returnTo)}
								className={buttonClasses({ variant: "bordered", size: "sm" })}
							>
								{t("link")}
							</a>
						)}
					</li>
				))}
			</ul>
		</section>
	);
}
