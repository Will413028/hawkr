import { normalizeWebHref } from "../navigation/web-href";
import type { BannerAction } from "./repo";

export function resolveBannerActionHref(
	action: BannerAction | undefined,
): string | undefined {
	if (!action || action.type === "popup") return undefined;
	if (action.type === "url") {
		return normalizeWebHref(action.url, { allowExternal: true });
	}
	const id = encodeURIComponent(action.id);
	return action.type === "product" ? `/products/${id}` : `/feed?post=${id}`;
}

export function bannerActionLabel(action: BannerAction | undefined): string {
	if (action?.type === "product") return "立即購買";
	if (action?.type === "post") return "查看貼文";
	if (action?.type === "url") return "查看詳情";
	return "";
}
