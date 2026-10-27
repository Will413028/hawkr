import { stripLocale } from "@/lib/intl/paths";

/** ADM 代行工作區的路徑前綴（56 號）。 */
const DELEGATED_WORKSPACE_ROOT = "/adm";

/**
 * 代行工作區自帶完整外框（ADM 窄欄 ＋ 頂部導覽 ＋ 賣家中心側欄），站台層的
 * navbar／footer／手機底部列一律不渲染。
 *
 * ⚠️ 這不只是視覺問題：那些 chrome 讀的是 **www 的登入態**，而代行者在 www 上
 * 通常沒有登入態（有的話那是他自己的買家／賣家身分，與被代行的店家無關）。
 * `SellerWorkspaceLock` 更直接 —— 一個「自己也是賣家、且正以賣家視角瀏覽」的
 * 管理員進到 `/adm`，會被它導去 `/seller-center/dashboard`，代行工作區根本打不開。
 */
export function isDelegatedWorkspacePath(
	rawPathname: string | null | undefined,
): boolean {
	if (!rawPathname) return false;
	const pathname = stripLocale(rawPathname);
	return (
		pathname === DELEGATED_WORKSPACE_ROOT ||
		pathname.startsWith(`${DELEGATED_WORKSPACE_ROOT}/`)
	);
}
