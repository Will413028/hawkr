"use client";

import { useTranslations } from "next-intl";
import { useAuth } from "@/lib/auth/context";
import {
	suspensionCategoryText,
	suspensionDateText,
	suspensionReasonText,
} from "@/lib/auth/suspension";
import { useUiText } from "@/lib/i18n/ui-text";

/**
 * 設定頁頂端的「帳號受限」區塊（受限登入，A7-D17）：帳號被停權時列停權日期、類別與說明。
 * 刪除帳號沿用同頁的 `DeleteAccountSection`。沒有停權時不顯示。
 */
export default function AccountRestrictedSection() {
	const ut = useUiText();
	const t = useTranslations("account.suspension");
	const auth = useAuth();
	if (auth.status !== "signedIn") return null;
	const suspension = auth.identity.suspension;
	if (!suspension) return null;
	const date = suspensionDateText(suspension);
	const category = suspensionCategoryText(ut, suspension);
	const reason = suspensionReasonText(suspension);
	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>{t("sectionTitle")}</h2>
			</header>
			<dl className="m-0 flex flex-col gap-2 text-sm">
				{date ? (
					<div className="flex flex-col gap-0.5">
						<dt className="text-xs text-foreground-soft">{t("date")}</dt>
						<dd className="m-0 font-semibold text-hk-heading">{date}</dd>
					</div>
				) : null}
				{category ? (
					<div className="flex flex-col gap-0.5">
						<dt className="text-xs text-foreground-soft">{t("category")}</dt>
						<dd className="m-0 font-semibold text-hk-heading">{category}</dd>
					</div>
				) : null}
				{reason ? (
					<div className="flex flex-col gap-0.5">
						<dt className="text-xs text-foreground-soft">{t("reason")}</dt>
						<dd className="m-0 font-semibold text-hk-heading">{reason}</dd>
					</div>
				) : null}
			</dl>
			<p className="m-0 mt-3 text-sm text-foreground-soft">{t("note")}</p>
		</section>
	);
}
