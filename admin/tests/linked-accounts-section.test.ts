import assert from "node:assert/strict";
import test from "node:test";
import { create } from "@bufbuild/protobuf";
import { createRouterTransport } from "@connectrpc/connect";
import {
	LinkedAccountSchema,
	LinkedIdentity,
	MeasureSchema,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/measure_pb";
import {
	bodyText,
	buttonsByText,
	click,
	dom,
	mount,
	React,
	requireFromTest,
} from "./fixtures/dom-harness";

/**
 * 處分頁的關聯帳號列：每列有錨點（待辦「共用銀行帳號」的連結落在這裡），「標為已檢視」只在伺服器說這一對在待辦
 * （shared_bank_account_pending）時出現，按下以措施與該列的帳號呼叫處置。
 */

const Section = requireFromTest(
	"../src/app/admin/measures/[id]/LinkedAccountsSection",
).default as unknown;
const h = React.createElement;

const row = (accountId: string, pending: boolean) =>
	create(LinkedAccountSchema, {
		accountId,
		storeId: `${accountId}1`,
		storeCode: `${accountId}2`,
		matchedIdentity: LinkedIdentity.BANK_ACCOUNT,
		sharedBankAccountPending: pending,
	});

test("標為已檢視只在待辦中的列出現，列有錨點，按下帶措施與帳號", async () => {
	const calls: [string, string][] = [];
	const measure = create(MeasureSchema, {
		id: "m1",
		linkedAccounts: [row("901", true), row("902", false)],
	});
	const m = mount(
		h(Section, {
			measure,
			onCreateLinked: () => {},
			onMarkReviewed: (measureId: string, account: { accountId: string }) =>
				calls.push([measureId, account.accountId]),
		}),
		createRouterTransport(() => {}),
	);
	await m.render({ settle: false });
	assert.ok(dom.window.document.getElementById("linked-account-901"));
	assert.ok(dom.window.document.getElementById("linked-account-902"));
	assert.equal(buttonsByText("標為已檢視").length, 1);
	assert.ok(
		dom.window.document
			.getElementById("linked-account-901")
			?.textContent?.includes("標為已檢視"),
	);
	assert.doesNotMatch(
		dom.window.document.getElementById("linked-account-902")?.textContent ?? "",
		/標為已檢視/u,
	);
	const lastCell = (id: string) =>
		dom.window.document
			.getElementById(id)
			?.querySelector("td:last-child")
			?.textContent?.trim();
	assert.equal(lastCell("linked-account-901"), "標為已檢視");
	assert.equal(lastCell("linked-account-902"), "-");
	await click("標為已檢視");
	assert.deepEqual(calls, [["m1", "901"]]);
	assert.match(bodyText(), /相同身分的其他帳號/u);
});
