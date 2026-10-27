import type { Metadata } from "next";
import NotFoundView from "@/components/shared/NotFoundView";
import RootDocument, { rootViewport } from "@/components/shared/RootDocument";
import { defaultLocale } from "@/lib/intl/locales";
import { buildRootMetadata, SITE_NAME } from "@/lib/seo/metadata";

/**
 * 找不到任何路由的 404（`experimental.globalNotFound`，next.config.ts）。根 layout 在 `app/[locale]`
 * 與 `app/(delegated)`，沒有單一的 layout 可以組出全站 404，所以這裡自己組整份文件，外殼與一般頁面相同。
 * 這個檔案不知道請求的語系（Next 不傳 params），一律用預設語系；route 之內的 `notFound()` 仍由
 * `app/[locale]/not-found.tsx` 依語系渲染。
 *
 * Next 把這個模組當 `/_not-found` 的 layout 用，所以 metadata 要自己帶根層那份（`metadataBase`、
 * og 預設值）與頁面標題，否則 og 網址會退回 localhost。
 * 這個檔案和 `app/opengraph-image.tsx` 在同一層，分享圖由檔案慣例提供（帶內容雜湊），所以不自己指定 `images`。
 */
export const metadata: Metadata = {
	...buildRootMetadata(defaultLocale, { shareImage: false }),
	title: `找不到頁面 | ${SITE_NAME}`,
};

export const viewport = rootViewport;

export default function GlobalNotFound() {
	return (
		<RootDocument locale={defaultLocale}>
			<NotFoundView />
		</RootDocument>
	);
}
