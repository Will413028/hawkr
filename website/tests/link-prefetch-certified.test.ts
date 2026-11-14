import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

/**
 * 全站共用的 `Link` 在每一頁的 bundle 裡：錢包預取的「已認證」判斷不得在這裡用魔術數字重寫一份
 * `SellerRequirement`，要走動態載入的共用推導（`lib/seller/wallet-transactions.ts` → `isStoreCertified`）。
 */
test("Link 的錢包預取不重寫已認證判斷", () => {
	const source = readFileSync(
		new URL("../components/shared/Link.tsx", import.meta.url),
		"utf8",
	);
	assert.doesNotMatch(source, /satisfies SellerRequirement/);
	assert.doesNotMatch(source, /missingRequirements/);
	assert.match(source, /import\("@\/lib\/seller\/wallet-transactions"\)/);
	assert.doesNotMatch(
		source,
		/import \{[^}]*\} from "@hawkr\/api\/gen\/hawkr\/seller\/v1alpha1\/seller_pb"/,
		"Link 不得以值匯入 seller 生成碼（只能 import type）",
	);
});

test("Link 的錢包預取只在開通摘要的平台審核完成後讀收款帳戶", () => {
	const source = readFileSync(
		new URL("../components/shared/Link.tsx", import.meta.url),
		"utf8",
	);
	assert.doesNotMatch(source, /platformReviewApproved|KYC_STATUS_APPROVED/);
	assert.match(source, /fetchMyOnboarding\(\)/);
	const approved = source.indexOf("platformReviewCompleted(onboarding)");
	const payment = source.indexOf("paymentRepo.fetchMyPaymentAccount");
	assert.ok(approved > 0 && payment > approved);
});
