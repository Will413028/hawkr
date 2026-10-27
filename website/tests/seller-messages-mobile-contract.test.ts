import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

/**
 * Source contracts for the seller message workspace's phone layout.
 *
 * These assertions intentionally stop at source seams: node:test has no layout
 * engine, so it cannot measure wrapping, overflow, or sticky columns. The real
 * browser behavior remains an E2E concern; this file prevents the declarations
 * that make that behavior possible from being silently removed or relocated.
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

function blockAfter(text: string, marker: string): string {
	return blockAt(text, text.indexOf(marker), marker);
}

const nav = source("features/seller-center/components/sellerNav.tsx");
const shell = source("features/seller-center/components/SellerCenterShell.tsx");
const workspace = source(
	"features/seller-center/messages/SellerMessagesWorkspaceClient.tsx",
);
const messagesCss = source("styles/features/messages.css");

const actionsAnchor = messagesCss.indexOf(".seller-messages__actions-slot");
assert.notEqual(actionsAnchor, -1, "missing seller message action-slot styles");
const mobileMediaAt = messagesCss.lastIndexOf(
	"@media (max-width: 960px)",
	actionsAnchor,
);
assert.notEqual(
	mobileMediaAt,
	-1,
	"seller message styles must have a mobile block",
);
const mobileCss = blockAt(
	messagesCss,
	mobileMediaAt,
	"seller message mobile media query",
);

test("訊息中心是賣家主導覽項目，不再埋在 footer", () => {
	const groupsAt = nav.indexOf("export const SELLER_NAV_GROUPS");
	const footerAt = nav.indexOf("export const SELLER_FOOTER_ITEMS");
	assert.notEqual(groupsAt, -1);
	assert.ok(footerAt > groupsAt);

	const mainNav = nav.slice(groupsAt, footerAt);
	const footerNav = nav.slice(footerAt);
	assert.match(mainNav, /href:\s*"\/seller-center\/messages"/);
	assert.match(mainNav, /label:\s*"訊息中心"/);
	assert.match(mainNav, /badgeKey:\s*"messages"/);
	assert.doesNotMatch(footerNav, /href:\s*"\/seller-center\/messages"/);
});

test("手機賣家列提供可辨識目前路由的訊息 tab", () => {
	const mobileBarAt = shell.indexOf(
		'<header className="seller-shell__mobile-bar">',
	);
	const mobileBarEnd = shell.indexOf("</header>", mobileBarAt);
	assert.notEqual(mobileBarAt, -1);
	assert.ok(mobileBarEnd > mobileBarAt);

	const mobileBar = shell.slice(mobileBarAt, mobileBarEnd);
	assert.match(mobileBar, /href="\/seller-center\/messages"/);
	assert.match(mobileBar, /seller-shell__mobile-message-tab/);
	assert.match(mobileBar, /aria-current=/);
	assert.match(mobileBar, /mobile-quick-label">訊息<\/span>/);
});

test("手機搜尋與重新整理保持單列，篩選 chips 改為水平滑動", () => {
	const actionRule = blockAfter(mobileCss, ".seller-messages__actions-slot,");
	assert.match(actionRule, /flex-wrap:\s*nowrap/);
	assert.match(actionRule, /width:\s*100%/);

	const refreshRule = blockAfter(
		mobileCss,
		".seller-messages__head-actions .seller-orders__btn",
	);
	assert.match(refreshRule, /flex:\s*0 0 auto/);
	assert.match(refreshRule, /white-space:\s*nowrap/);

	const chipRowRule = blockAfter(mobileCss, ".seller-messages__chip-row");
	assert.match(chipRowRule, /overflow-x:\s*auto/);
	assert.match(chipRowRule, /overflow-y:\s*hidden/);
	assert.match(chipRowRule, /flex-wrap:\s*nowrap/);

	const chipRule = blockAfter(
		mobileCss,
		".seller-messages__filters .seller-orders__chip",
	);
	assert.match(chipRule, /flex:\s*0 0 auto/);
	assert.match(chipRule, /white-space:\s*nowrap/);
});

test("訊息篩選選取色不被訂單狀態覆寫，焦點框留在膠囊內", () => {
	const selectedMarker =
		'button.seller-orders__chip.is-selected[aria-pressed="true"]';
	const selectedAt = messagesCss.indexOf(selectedMarker);
	const selectedRule = blockAt(
		messagesCss,
		selectedAt,
		"scoped seller message selected filter",
	);
	const selectedSelector = messagesCss.slice(
		messagesCss.lastIndexOf(".seller-shell", selectedAt),
		messagesCss.indexOf("{", selectedAt),
	);

	assert.match(selectedSelector, /\.seller-messages__filters/);
	assert.match(selectedSelector, /\.seller-messages__chip-row/);
	assert.match(selectedRule, /border-color:\s*var\(--theme-primary-border\)/);
	assert.match(selectedRule, /background:\s*var\(--theme-primary-subtle\)/);
	assert.match(selectedRule, /box-shadow:\s*none/);
	assert.match(selectedRule, /color:\s*var\(--theme-text-accent-on-tint\)/);

	const focusAt = messagesCss.indexOf(
		".seller-orders__chip:focus-visible",
		selectedAt,
	);
	const focusRule = blockAt(
		messagesCss,
		focusAt,
		"scoped seller message filter focus",
	);
	assert.match(focusRule, /outline:\s*2px solid var\(--theme-action-primary\)/);
	assert.match(focusRule, /outline-offset:\s*-3px/);
	assert.match(focusRule, /border-radius:\s*var\(--radius-pill\)/);
});

test("空狀態位於寬表格外，不再被 980px 表格撐寬", () => {
	const wrapAt = workspace.indexOf(
		'<div className="seller-orders__table-wrap seller-messages__table-wrap">',
	);
	const emptyAt = workspace.indexOf("<SellerEmptyState", wrapAt);
	const tableClassAt = workspace.indexOf(
		'className="seller-orders__table seller-messages__table"',
		wrapAt,
	);
	const tableAt = workspace.lastIndexOf("<table", tableClassAt);
	const tableEnd = workspace.indexOf("</table>", tableAt);

	assert.notEqual(wrapAt, -1);
	assert.ok(emptyAt > wrapAt);
	assert.ok(tableAt > emptyAt);
	assert.ok(tableEnd > tableAt);
	assert.match(workspace.slice(wrapAt, tableAt), /filteredRows\.length === 0/);
	assert.match(
		workspace.slice(wrapAt, tableAt),
		/seller-messages__empty-state/,
	);
	assert.doesNotMatch(
		workspace.slice(tableAt, tableEnd),
		/SellerEmptyState|seller-orders__empty/,
	);
});

test("手機訊息表格解除共用規則對第二欄的 sticky 定位", () => {
	assert.match(
		mobileCss,
		/\.seller-messages__table\.seller-orders__table thead th:nth-child\(2\),\s*\.seller-messages__table\.seller-orders__table tbody td:nth-child\(2\)/,
	);
	const resetRule = blockAfter(
		mobileCss,
		".seller-messages__table.seller-orders__table thead th:nth-child(2),",
	);
	assert.match(resetRule, /position:\s*static/);
	assert.match(resetRule, /left:\s*auto/);
	assert.match(resetRule, /z-index:\s*auto/);
	assert.match(resetRule, /background:\s*transparent/);
	assert.match(resetRule, /box-shadow:\s*none/);
});
