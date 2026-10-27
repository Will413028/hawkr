import type { Metadata } from "next";
import { notFound } from "next/navigation";
import RootDocument, { rootViewport } from "@/components/shared/RootDocument";
import {
	allRoutableLocales,
	defaultLocale,
	resolveRoutableLocale,
} from "@/lib/intl/locales";
import { buildRootMetadata } from "@/lib/seo/metadata";

type LocaleParams = { params: Promise<{ locale: string }> };

/**
 * 買家與賣家介面的根 layout（ADR 2026-10-17 D1）：`[locale]` 是 URL 的第一段，zh-TW 在 `/…`
 * （next-intl 的 `as-needed` 由 middleware 改寫成 `/zh-TW/…`），其他語系在 `/<locale>/…`。
 * `<html lang>` 與 provider 的語系取自這一段；整份 HTML 文件在 components/shared/RootDocument.tsx。
 *
 * 只預先產生這個部署會路由的語系（lib/intl/locales.ts）：prod 的 `/en/…` 不是語系，回 404。
 */
export function generateStaticParams() {
	return allRoutableLocales().map((locale) => ({ locale }));
}

export async function generateMetadata({
	params,
}: LocaleParams): Promise<Metadata> {
	// An unroutable locale 404s in the layout below; its metadata still needs `metadataBase`
	// (Next resolves the root `opengraph-image` against it even for the 404), so it gets the default's.
	const locale = resolveRoutableLocale((await params).locale) ?? defaultLocale;
	return buildRootMetadata(locale);
}

export const viewport = rootViewport;

export default async function LocaleLayout({
	children,
	params,
}: LocaleParams & { children: React.ReactNode }) {
	const locale = resolveRoutableLocale((await params).locale);
	if (!locale) notFound();
	return <RootDocument locale={locale}>{children}</RootDocument>;
}
