import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * Source contracts for the seller notification inbox and its compact phone
 * entry points. Pixel geometry remains covered by the browser pass.
 */

const root = fileURLToPath(new URL("..", import.meta.url));

function source(relativePath: string): string {
	return readFileSync(path.join(root, relativePath), "utf8");
}

function blockAt(text: string, markerAt: number, label: string): string {
	assert.notEqual(markerAt, -1, `missing source marker: ${label}`);
	const openAt = text.indexOf("{", markerAt);
	assert.notEqual(openAt, -1, `missing opening brace after: ${label}`);

	let depth = 0;
	for (let index = openAt; index < text.length; index += 1) {
		if (text[index] === "{") depth += 1;
		if (text[index] !== "}") continue;
		depth -= 1;
		if (depth === 0) return text.slice(openAt + 1, index);
	}

	assert.fail(`missing closing brace after: ${label}`);
}

const nav = source("features/seller-center/components/sellerNav.tsx");
const shell = source("features/seller-center/components/SellerCenterShell.tsx");
const sellerPage = source(
	"app/[locale]/(seller)/seller-center/notifications/page.tsx",
);
const unreadContext = source("lib/notifications/unread-context.tsx");
const notificationsClient = source(
	"features/account/screens/notifications/NotificationsClient.tsx",
);
const delegatedPolicy = source("features/delegated-operations/nav-policy.ts");
const css = source("styles/features/seller-center.css");

test("通知中心是有未讀 badge 的賣家主導覽項目", () => {
	const groupsAt = nav.indexOf("export const SELLER_NAV_GROUPS");
	const footerAt = nav.indexOf("export const SELLER_FOOTER_ITEMS");
	assert.notEqual(groupsAt, -1);
	assert.ok(footerAt > groupsAt);

	const mainNav = nav.slice(groupsAt, footerAt);
	const footerNav = nav.slice(footerAt);
	assert.match(mainNav, /href:\s*"\/seller-center\/notifications"/);
	assert.match(mainNav, /label:\s*"通知中心"/);
	assert.match(mainNav, /<SellerSidebarIcon name="notification"/);
	assert.match(mainNav, /badgeKey:\s*"notifications"/);
	assert.doesNotMatch(footerNav, /href:\s*"\/seller-center\/notifications"/);
});

test("手機快速入口顯示通知未讀數並辨識通知子路由", () => {
	assert.match(shell, /useNotificationUnread\(\)/);
	assert.match(shell, /notifications:\s*unreadNotificationCount/);
	assert.match(
		shell,
		/pathname\.startsWith\("\/seller-center\/notifications\/"\)/,
	);

	const mobileBarAt = shell.indexOf(
		'<header className="seller-shell__mobile-bar">',
	);
	const mobileBarEnd = shell.indexOf("</header>", mobileBarAt);
	assert.notEqual(mobileBarAt, -1);
	assert.ok(mobileBarEnd > mobileBarAt);
	const mobileBar = shell.slice(mobileBarAt, mobileBarEnd);

	assert.match(mobileBar, /aria-label="賣家中心快速入口"/);
	assert.match(mobileBar, /href="\/seller-center\/notifications"/);
	assert.match(mobileBar, /seller-shell__mobile-notification-tab/);
	assert.match(mobileBar, /aria-current=\{isNotificationsRoute/);
	assert.match(mobileBar, /<SellerSidebarIcon name="notification"/);
	assert.match(mobileBar, /unreadNotificationCount > 99/);
	assert.match(mobileBar, /mobile-quick-label">通知<\/span>/);
	assert.match(mobileBar, /seller-shell__mobile-notification-badge/);
});

test("賣家通知頁使用通知中心標題與共用權威未讀數", () => {
	assert.match(sellerPage, /title:\s*"通知中心 — 賣家中心"/);
	assert.match(sellerPage, /title="通知中心"/);
	assert.match(
		sellerPage,
		/preferencesHref="\/seller-center\/notifications\/preferences"/,
	);
	assert.match(notificationsClient, /count:\s*unreadCount/);
	assert.match(notificationsClient, /loading:\s*unreadCountLoading/);
	assert.match(notificationsClient, /streamStatus/);
	assert.doesNotMatch(
		notificationsClient,
		/list\.items\.filter\([^)]*!\s*[^)]*\.isRead/,
	);
});

test("通知 unread provider 以帳號與角色 fail closed 並隔離快取", () => {
	assert.match(
		unreadContext,
		/resetSignal\s*=\s*`\$\{identity\.accountId \?\? "anon"\}:\$\{activeRole \?\? "none"\}`/,
	);
	// 同一個啟用條件同時管未讀數與唯一的那條串流：兩處都讀它，不各寫一份。
	assert.match(
		unreadContext,
		/const streamEnabled\s*=\s*isLoggedIn\s*&&\s*identity\.accountId !== null\s*&&\s*activeRole !== null/s,
	);
	assert.equal(unreadContext.match(/enabled:\s*streamEnabled/g)?.length, 2);
	assert.match(
		unreadContext,
		/accountId:\s*identity\.accountId \?\? undefined/,
	);
	assert.match(unreadContext, /role:\s*activeRole/);
	assert.match(unreadContext, /onOtherWorkspaceNotification/);
	assert.match(unreadContext, /invalidateNotificationLists/);
});

test("ADM 代行側欄隱藏賣家本人的通知收件匣", () => {
	assert.match(
		delegatedPolicy,
		/DELEGATED_HIDDEN_HREFS[\s\S]*"\/seller-center\/notifications"/,
	);
});

test("320px 手機把兩個快速入口收成同高 icon tabs", () => {
	const compactAt = css.indexOf("@media (max-width: 360px)");
	const compactCss = blockAt(css, compactAt, "360px compact quick tabs");
	assert.match(
		compactCss,
		/\.seller-shell__mobile-quick-tab\s*\{[^}]*width:\s*40px[^}]*min-width:\s*40px/s,
	);
	assert.match(
		compactCss,
		/\.seller-shell__mobile-quick-label\s*\{[^}]*position:\s*absolute[^}]*width:\s*1px[^}]*overflow:\s*hidden/s,
	);
	assert.match(
		compactCss,
		/\.seller-shell__mobile-quick-badge\s*\{[^}]*position:\s*absolute/s,
	);

	const quickTabCss = blockAt(
		css,
		css.indexOf(".seller-shell__mobile-quick-tab {"),
		"base mobile quick tab",
	);
	assert.match(quickTabCss, /height:\s*40px/);
	assert.match(quickTabCss, /white-space:\s*nowrap/);

	const quickTabsCss = blockAt(
		css,
		css.indexOf(".seller-shell__mobile-quick-tabs {"),
		"base mobile quick-tabs rail",
	);
	assert.match(quickTabsCss, /display:\s*inline-flex/);
	assert.match(quickTabsCss, /flex:\s*0 0 auto/);
});
