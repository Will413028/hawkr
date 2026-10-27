import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { fileURLToPath } from "node:url";

/**
 * These places keep their navigation primitive: moving them onto the shared Link (client-side navigation through
 * a View Transition, `prefetch={false}` underneath) is a UI or flow change that needs the owner's consent.
 *   - five links keep `next/link`'s own behavior (viewport prefetch) through `lib/intl/native-link.tsx`, which is
 *     `next/link` plus the prefix (the chat sidebar's rows are pinned in conversation-sidebar-navigation.test.ts);
 *   - the two links to the account page in the checkout's invoice sheet stay plain anchors: leaving the sheet is a
 *     full page load.
 */

function read(path: string): string {
	return readFileSync(
		fileURLToPath(new URL(`../${path}`, import.meta.url)),
		"utf8",
	);
}

const NATIVE_LINK_USERS = [
	"features/procurement-assistant/AssistantWorkspace.tsx",
	"features/procurement-assistant/CandidateBrowser.tsx",
	"features/procurement-assistant/HawkrAgentChatWorkspace.tsx",
	"features/invoices/components/InvoiceDocumentCard.tsx",
	"features/invoices/screens/InvoiceDocumentDetailClient.tsx",
] as const;

for (const file of NATIVE_LINK_USERS) {
	test(`${file} links through next/link with the prefix, not the shared Link`, () => {
		const source = read(file);
		assert.match(source, /import Link from "@\/lib\/intl\/native-link";/);
		assert.doesNotMatch(source, /from "next\/link"/);
		assert.doesNotMatch(
			source,
			/import Link from "@\/components\/shared\/Link"/,
		);
	});
}

test("the invoice sheet's links to the account page are plain anchors with the localized href", () => {
	const source = read("features/invoices/checkout/InvoiceMethodSheet.tsx");
	assert.doesNotMatch(source, /from "next\/link"/);
	assert.doesNotMatch(source, /@\/components\/shared\/Link/);
	assert.doesNotMatch(source, /@\/lib\/intl\/native-link/);
	assert.match(
		source,
		/const invoiceProfilesHref = useLocalizedHref\("\/account\/invoice-profiles"\);/,
	);
	assert.equal(source.match(/<a\b/g)?.length, 2);
	assert.equal(source.match(/href=\{invoiceProfilesHref\}/g)?.length, 2);
	assert.doesNotMatch(source, /href="\/account\/invoice-profiles"/);
});
