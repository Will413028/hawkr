import { stripLocale } from "@/lib/intl/paths";
import type { RoleSlot } from "./storage";

/**
 * Who may open a path: a buyer-workspace account, a seller-workspace account,
 * or any signed-in account regardless of workspace (`account`).
 */
export type RequiredAccess = RoleSlot | "account";

interface AccessClass {
	/** The workspace the class belongs to; `null` means any workspace. */
	workspace: RoleSlot | null;
	/** Whether the signed-in account may open pages of this class. */
	owns: (account: { isLoggedIn: boolean; hasSeller: boolean }) => boolean;
}

// 每個登入的帳號都是買家；賣家區要有 ROLE_SELLER；`account` 登入即可、不分工作區。
export const ACCESS_CLASSES: Record<RequiredAccess, AccessClass> = {
	buyer: { workspace: "buyer", owns: ({ isLoggedIn }) => isLoggedIn },
	seller: { workspace: "seller", owns: ({ hasSeller }) => hasSeller },
	account: { workspace: null, owns: ({ isLoggedIn }) => isLoggedIn },
};

const ACCESS_RULES: ReadonlyArray<{
	access: RequiredAccess;
	prefixes: readonly string[];
}> = [
	{ access: "buyer", prefixes: ["/account", "/checkout"] },
	{
		access: "seller",
		prefixes: ["/seller-center", "/seller-product-import-preview"],
	},
	{ access: "account", prefixes: ["/seller/measures"] },
];

export function roleLabel(slot: RoleSlot) {
	return slot === "seller" ? "賣家" : "買家";
}

export function safeRedirectPath(value: string | null | undefined) {
	if (!value?.startsWith("/")) return "";
	// Protocol-relative ("//host") resolves to an off-origin host.
	if (value.startsWith("//")) return "";
	// Backslashes normalise to "/" in the WHATWG URL parser, so "/\host"
	// becomes "//host" — a protocol-relative off-origin redirect that would
	// otherwise slip past the leading-slash check.
	if (value.includes("\\")) return "";
	// Browsers strip control chars (tab/newline/CR) before re-parsing the URL,
	// which can likewise turn a "safe" path into a protocol-relative one.
	for (let i = 0; i < value.length; i++) {
		const code = value.charCodeAt(i);
		if (code <= 0x1f || code === 0x7f) return "";
	}
	return value;
}

/**
 * Builds a `/login` URL, dropping empty params. Single formatter so every
 * entry point (navbar, provider return, re-login) round-trips `role` /
 * `redirect` / `error` exactly the way middleware.ts and LoginPage read them.
 */
export function loginPath(
	params: {
		role?: RoleSlot;
		error?: string;
		redirect?: string;
		/** 剛用哪一家登入完成回到這頁（單體 302 回來的網址帶著它，登入頁據此記 `login` 事件）。 */
		provider?: string;
	} = {},
) {
	const qs = new URLSearchParams();
	if (params.role) qs.set("role", params.role);
	if (params.error) qs.set("error", params.error);
	if (params.redirect) qs.set("redirect", params.redirect);
	if (params.provider) qs.set("provider", params.provider);
	const s = qs.toString();
	return s ? `/login?${s}` : "/login";
}

export function requiredAccessForPath(pathname: string): RequiredAccess | null {
	// The rules are written on locale-free paths; `/en/account` is `/account` in English.
	const path = stripLocale(pathname);
	for (const { access, prefixes } of ACCESS_RULES) {
		if (
			prefixes.some(
				(prefix) => path === prefix || path.startsWith(`${prefix}/`),
			)
		) {
			return access;
		}
	}
	return null;
}

/** The workspace a path belongs to; public and `account` paths belong to none. */
export function workspaceOfPath(pathname: string): RoleSlot | null {
	const access = requiredAccessForPath(pathname);
	return access ? ACCESS_CLASSES[access].workspace : null;
}

export function defaultPathForRole(slot: RoleSlot | null | undefined) {
	return slot === "seller" ? "/seller-center/dashboard" : "/";
}

export function redirectForRole(
	redirect: string,
	slot: RoleSlot | null | undefined,
) {
	const safeRedirect = safeRedirectPath(redirect);
	if (!safeRedirect) return defaultPathForRole(slot);
	// Role matching is a pathname contract. Passing the full redirect through
	// made exact protected roots such as `/account?tab=orders` look public,
	// because they no longer equalled `/account` or started with `/account/`.
	const pathname = new URL(safeRedirect, "https://hawkr.local").pathname;
	const workspace = workspaceOfPath(pathname);
	if (workspace && workspace !== slot) {
		return defaultPathForRole(slot);
	}
	return safeRedirect;
}
