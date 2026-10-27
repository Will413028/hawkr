import { normalizeWebHref } from "../navigation/web-href";
import type { BannerAction } from "./repo";

export function resolveBannerActionHref(
	action: BannerAction | undefined,
): string | undefined {
	if (!action || action.type === "popup") return undefined;
	if (action.type === "url") {
		return normalizeWebHref(action.url, { allowExternal: true });
	}
	// ID 是不透明的字串（ADR 2026-09-25-row-identity-by-data-kind D5），只編碼、不檢查格式。
	const id = encodeURIComponent(action.id);
	return action.type === "product" ? `/products/${id}` : `/feed?post=${id}`;
}

export function bannerActionLabel(action: BannerAction | undefined): string {
	if (action?.type === "product") return "立即購買";
	if (action?.type === "post") return "查看貼文";
	if (action?.type === "url") return "查看詳情";
	return "";
}
