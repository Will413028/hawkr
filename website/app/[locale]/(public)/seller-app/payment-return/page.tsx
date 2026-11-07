import type { Metadata } from "next";
import { Suspense } from "react";
import { localeFromParam, noindexMetadata } from "@/lib/seo/metadata";
import { ReturnHeading } from "./ReturnHeading";

export const metadata: Metadata = noindexMetadata();

/**
 * 賣家 App 開通收款帳戶後，金流商的託管頁面回跳的靜態頁（免登入）：App 在外部瀏覽器開連結，
 * 完成或連結過期都回到這裡，賣家看到提示後自己切回 App（App 回到前景時重讀狀態）。
 * 頁面本身是靜態的；標題讀 `?state=done|expired`（`ReturnHeading`）。
 */
const COPY = {
	"zh-TW": {
		done: "收款帳戶設定完成",
		expired: "連結已過期",
		action: "請回到 hawkr 賣家 App",
	},
	en: {
		done: "Payment account setup is complete",
		expired: "This link has expired",
		action: "Please return to the hawkr Seller app",
	},
} as const;

export default async function SellerAppPaymentReturnPage({
	params,
}: {
	params: Promise<{ locale: string }>;
}) {
	const { locale } = await params;
	const copy = COPY[localeFromParam(locale)];
	return (
		<main className="mx-auto flex min-h-[60vh] max-w-md flex-col items-center justify-center gap-3 px-4 text-center">
			<Suspense fallback={<h1 className="text-xl font-bold">{copy.done}</h1>}>
				<ReturnHeading done={copy.done} expired={copy.expired} />
			</Suspense>
			<p>{copy.action}</p>
		</main>
	);
}
