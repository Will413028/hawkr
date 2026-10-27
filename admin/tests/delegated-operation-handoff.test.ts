import assert from "node:assert/strict";
import test from "node:test";
import {
	buildHandoffLocation,
	isDelegatedHandoffCode,
	isSafeOperationCredential,
	isSameOriginSubmission,
	resolveWebOrigin,
} from "../src/features/delegated-operations/server/handoff";

/**
 * 兌換去向只能來自部署設定。設計書 L8346 明令「不得由請求的 `Host` 推導」——
 * 那讓任何能控制 `Host` 的人指定 credential 的兌換去向；這裡守的是「只吃
 * 一個乾淨的 origin，其餘一律 fail-closed」。
 */
test("WEB_ORIGIN 只接受乾淨的 origin", () => {
	assert.equal(
		resolveWebOrigin({ WEB_ORIGIN: "https://web-dev.example.com" }),
		"https://web-dev.example.com",
	);
	assert.equal(
		resolveWebOrigin({ WEB_ORIGIN: " https://example.com/ " }),
		"https://example.com",
	);
	for (const value of [
		undefined,
		"",
		"   ",
		"example.com",
		"//example.com",
		"javascript:alert(1)",
		// 帶路徑／查詢／片段的值會讓 `${origin}/adm/enter` 組出一個看起來合法、
		// 實際打到別處的網址。
		"https://example.com/adm",
		"https://example.com/?next=x",
		"https://example.com/#x",
		"https://user:pass@example.com",
	]) {
		assert.equal(resolveWebOrigin({ WEB_ORIGIN: value }), null, String(value));
	}
});

test("http 只在非 production 可用，部署環境一律 https", () => {
	assert.equal(
		resolveWebOrigin({
			WEB_ORIGIN: "http://localhost:3000",
			NODE_ENV: "development",
		}),
		"http://localhost:3000",
	);
	assert.equal(
		resolveWebOrigin({
			WEB_ORIGIN: "http://localhost:3000",
			NODE_ENV: "production",
		}),
		null,
	);
	assert.equal(
		resolveWebOrigin({
			WEB_ORIGIN: "https://app.example.com",
			NODE_ENV: "production",
		}),
		"https://app.example.com",
	);
});

test("兌換網址把碼放在 query 且逐字編碼", () => {
	assert.equal(
		buildHandoffLocation(
			"https://example.com",
			"dhof_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8",
		),
		"https://example.com/adm/enter?code=dhof_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8",
	);
	assert.equal(
		buildHandoffLocation("https://example.com", "a b&c=d"),
		"https://example.com/adm/enter?code=a%20b%26c%3Dd",
	);
});

test("handoff code 形狀：dhof_ ＋ 43 字 base64url", () => {
	const valid = "dhof_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8";
	assert.equal(isDelegatedHandoffCode(valid), true);
	for (const value of [
		"",
		null,
		42,
		"dhof_short",
		`${valid}x`,
		// credential 的前綴，不是兌換碼
		"dops_9tJ0mQx7Kd2ZfVb3Rn8sYuC1Ea6Ph4Lw5Ti0Og2NkJ8",
		"dhof_4kR2sPy8Mf3ZgWc5Tn9uZvD1Fb7Qi6Nx8Uj0Rh3Pm!2",
	]) {
		assert.equal(isDelegatedHandoffCode(value), false, String(value));
	}
});

test("credential 形狀守的是「不會被塞進標頭」", () => {
	assert.equal(isSafeOperationCredential("a".repeat(32)), true);
	for (const value of [
		"short",
		null,
		"has space".padEnd(32, "a"),
		"has;semicolon".padEnd(32, "a"),
		"has,comma".padEnd(32, "a"),
		`has\nnewline`.padEnd(32, "a"),
	]) {
		assert.equal(isSafeOperationCredential(value), false, String(value));
	}
});

test("same-origin 檢查：Origin 與 Host 必須同時存在且相符", () => {
	const of = (headers: Record<string, string>) =>
		isSameOriginSubmission(new Headers(headers));
	assert.equal(
		of({ origin: "http://localhost:3001", host: "localhost:3001" }),
		true,
	);
	assert.equal(
		of({
			origin: "https://admin.example.com",
			host: "admin.example.com",
		}),
		true,
	);
	assert.equal(
		of({ origin: "https://evil.example", host: "admin.example.com" }),
		false,
	);
	assert.equal(of({ host: "admin.example.com" }), false);
	assert.equal(of({ origin: "https://admin.example.com" }), false);
	assert.equal(of({ origin: "null", host: "admin.example.com" }), false);
	// port 不同就是不同 origin。
	assert.equal(
		of({ origin: "http://localhost:3000", host: "localhost:3001" }),
		false,
	);
});
