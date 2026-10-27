import RootDocument, { rootViewport } from "@/components/shared/RootDocument";
import { defaultLocale } from "@/lib/intl/locales";
import { buildRootMetadata } from "@/lib/seo/metadata";

/**
 * 代行工作區（`/adm`）的根 layout。它是 staff 介面，只有 zh-TW、不本地化（ADR 2026-10-17 D1：
 * admin 與 staff 介面不裝語系路由），所以不在 `app/[locale]` 底下，也不經 next-intl 的 middleware
 * （lib/intl/paths.ts 的 NON_LOCALIZED_PREFIXES）。跨根 layout 的導覽是整頁重載。
 */
export const metadata = buildRootMetadata(defaultLocale);

export const viewport = rootViewport;

export default function DelegatedRootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return <RootDocument locale={defaultLocale}>{children}</RootDocument>;
}
