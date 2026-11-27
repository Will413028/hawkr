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
	setActiveLocale(appLocale);
	const [queryClient] = React.useState(createAppQueryClient);
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

							<React.Suspense fallback={null}>
								<AppOverlayHost />
							</React.Suspense>
							<AppToaster />

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
