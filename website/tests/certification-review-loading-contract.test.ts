import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

function source(path: string): string {
	return readFileSync(new URL(path, import.meta.url), "utf8");
}

function discardButtonAttributes(markup: string, initiator: string): string {
	const identity = `isLoading={discardInitiator === "${initiator}"}`;
	const identityIndex = markup.indexOf(identity);
	assert.notEqual(identityIndex, -1, `missing ${initiator} loading identity`);
	const start = markup.lastIndexOf("<Button", identityIndex);
	const label = 'loadingLabel="捨棄草稿中…"';
	const labelIndex = markup.indexOf(label, identityIndex);
	assert.notEqual(start, -1, `missing shared Button for ${initiator}`);
	assert.notEqual(labelIndex, -1, `missing discard label for ${initiator}`);
	return markup.slice(start, labelIndex + label.length);
}

test("certification discard entries share an immediate guard and retain initiator identity", () => {
	const certifications = source(
		"../features/seller-center/profile/certifications/CertificationsClient.tsx",
	);
	const typeSwitch = discardButtonAttributes(certifications, "type-switch");
	const actionBar = discardButtonAttributes(certifications, "action-bar");

	assert.match(
		certifications,
		/type DiscardDraftInitiator = "type-switch" \| "action-bar"/,
	);
	assert.match(
		certifications,
		/const discardPendingRef = React\.useRef\(false\)/,
	);
	assert.match(
		certifications,
		/if \(!application \|\| discardPendingRef\.current\) return/,
	);
	assert.equal(certifications.match(/loadingLabel="捨棄草稿中…"/g)?.length, 2);
	assert.match(
		typeSwitch,
		/disabled=\{busy === "submit" \|\| discardPending\}/,
	);
	assert.match(actionBar, /disabled=\{busy === "submit" \|\| discardPending\}/);
	assert.match(
		certifications,
		/onClick=\{\(\) => void handleCancel\("type-switch"\)\}/,
	);
	assert.match(
		certifications,
		/onClick=\{\(\) => void handleCancel\("action-bar"\)\}/,
	);

	const handlerStart = certifications.indexOf("const handleCancel = async");
	const handlerEnd = certifications.indexOf("const formMainRef", handlerStart);
	assert.notEqual(handlerStart, -1);
	assert.notEqual(handlerEnd, -1);
	const handler = certifications.slice(handlerStart, handlerEnd);
	const guardIndex = handler.indexOf("discardPendingRef.current = true");
	const requestIndex = handler.indexOf("await cancelKyc(etagRef.current)");
	const resetIndex = handler.indexOf("setMyKyc(null)");
	const catchIndex = handler.indexOf("} catch (e) {");
	assert.ok(guardIndex >= 0 && guardIndex < requestIndex);
	assert.ok(requestIndex < resetIndex && resetIndex < catchIndex);
	// 失敗時顯示伺服器或目錄的句子（L′-9：@hawkr/api 的 errorText），不再有呼叫點自寫的備用句。
	assert.match(handler, /setError\(errorText\(e, defaults\)\)/);
	assert.doesNotMatch(handler, /"捨棄認證草稿失敗"/);
	assert.match(handler, /discardPendingRef\.current = false/);
	assert.match(handler, /setDiscardInitiator\(null\)/);
	assert.doesNotMatch(handler, /setBusy\("cancel"\)/);
});

test("review photo upload tile keeps 72px geometry and exposes Upload status", () => {
	const review = source("../features/account/screens/orders/ReviewClient.tsx");
	const uploadIndicator = source(
		"../components/shared/UploadLoadingIndicator.tsx",
	);

	assert.match(
		review,
		/import UploadLoadingIndicator from "@\/components\/shared\/UploadLoadingIndicator"/,
	);
	assert.equal(review.match(/<UploadLoadingIndicator\b/g)?.length, 1);
	assert.match(review, /className=\{`flex size-\[72px\]/);
	assert.match(review, /aria-busy=\{d\.uploading\}/);
	assert.match(review, /aria-disabled=\{d\.uploading \|\| undefined\}/);
	assert.match(review, /role="status"/);
	assert.match(review, /aria-live="polite"/);
	assert.match(review, /aria-atomic="true"/);
	assert.match(review, /size=\{22\}/);
	assert.match(review, /className="size-\[22px\] shrink-0"/);
	assert.match(review, /上傳評價照片中…/);
	assert.match(
		review,
		/<input[\s\S]*type="file"[\s\S]*className="hidden"[\s\S]*disabled=\{d\.uploading\}/,
	);
	assert.match(uploadIndicator, /name="upload"/);
	assert.match(uploadIndicator, /motion-reduce:animate-none/);
});
