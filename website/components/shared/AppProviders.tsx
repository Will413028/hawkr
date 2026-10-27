"use client";

import { TransportProvider } from "@connectrpc/connect-query";
import { QueryClientProvider } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import type { AbstractIntlMessages } from "next-intl";
import { NextIntlClientProvider } from "next-intl";
import { ThemeProvider as NextThemesProvider } from "next-themes";
import { NuqsAdapter } from "nuqs/adapters/next/app";
import * as React from "react";
import AppOverlayHost from "@/components/shared/AppOverlayHost";
import AppToaster from "@/components/shared/AppToaster";
import SiteGrainientBackground from "@/components/shared/SiteGrainientBackground";
import { DEV_OVERLAYS_ENABLED } from "@/lib/config/env";
import { browserTransport } from "@/lib/connect/transport";
import { setActiveLocale } from "@/lib/intl/active-locale";
import { defaultLocale, isAppLocale } from "@/lib/intl/locales";
import { RoutedLocaleProvider } from "@/lib/intl/routed-locale";
import { createAppQueryClient } from "@/lib/query/client";
import { STORAGE_KEY } from "@/lib/theme/context";
import "@/styles/components/site-grainient.css";

/**
 * Devtools are a development affordance, so the runtime flag must not be the
 * only thing keeping them out of production: a static import puts the whole
 * panel in the client graph of every page regardless of whether the flag ever
 * turns it on. `ssr: false` because the panel is browser-only, and the dynamic
 * import means the chunk is requested only where `DEV_OVERLAYS_ENABLED` holds.
 */
const ReactQueryDevtools = dynamic(
	() =>
		import("@tanstack/react-query-devtools").then((m) => m.ReactQueryDevtools),
	{ ssr: false },
);

interface AppProvidersProps {
	children: React.ReactNode;
	locale: string;
	messages: AbstractIntlMessages;
}

export default function AppProviders({
	children,
	locale,
	messages,
}: AppProvidersProps) {
	const appLocale = isAppLocale(locale) ? locale : defaultLocale;
	// 瀏覽器端 React 之外的程式（沒傳語系的 Connect 呼叫、硬導覽）讀這個；idempotent，server 端是空操作。
	setActiveLocale(appLocale);
	const [queryClient] = React.useState(createAppQueryClient);
	// connect-query 的讀取（useQuery(方法, 輸入)）走這個 transport：依 service 分派，*AppService 帶登入
	// 憑證，公開的 service 不帶（lib/connect/transport.ts 的 browserTransport）。
	const [transport] = React.useState(() =>
		browserTransport({ locale: appLocale }),
	);

	return (
		<NextIntlClientProvider locale={locale} messages={messages}>
			<RoutedLocaleProvider value={appLocale}>
				<NextThemesProvider
					attribute="data-theme"
					defaultTheme="light"
					enableSystem={false}
					storageKey={STORAGE_KEY}
				>
					<SiteGrainientBackground />
					<NuqsAdapter>
						<QueryClientProvider client={queryClient}>
							<TransportProvider transport={transport}>
								{children}
							</TransportProvider>
							{/* AppOverlayHost 讀 usePathname()（render 期讀路由屬 dynamic）——
						    比照 components/shared/RootDocument.tsx 其他 usePathname chrome，包 <Suspense> 讓
						    靜態外殼可 prerender，dynamic 部分延後，避免 prerender 失敗。 */}
							<React.Suspense fallback={null}>
								<AppOverlayHost />
							</React.Suspense>
							<AppToaster />
							{/* 掛載條件走共用開關而非各自寫 NODE_ENV：本機 E2E 要能把所有
						    dev-only 浮動工具一次關掉，量到與 CI 相同的 DOM（web#425）。 */}
							{DEV_OVERLAYS_ENABLED && (
								<ReactQueryDevtools initialIsOpen={false} />
							)}
						</QueryClientProvider>
					</NuqsAdapter>
				</NextThemesProvider>
			</RoutedLocaleProvider>
		</NextIntlClientProvider>
	);
}
