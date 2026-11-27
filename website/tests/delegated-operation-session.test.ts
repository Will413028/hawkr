import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
	createDelegatedCookieOptions,
	DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS,
	getDelegatedCookieName,
	isDelegatedEndProven,
	readDelegatedCookie,
	serializeDelegatedCookie,
} from "../features/delegated-operations/server/session";

/**
 * 由 `hawkr-admin` 的 `tests/delegated-operation-session.test.ts` 搬移
 * （56 號 L1078）。斷言的**性質**不變，只換 repo、cookie 名與 `Path=/adm`；
 * 終態證明那一條改以 gateway 的數字業務碼表述（admin 那份的字串碼是更早的形狀）。
 *
 * 這支測試鎖住的是「credential 只在證明對帳通過之後才被交出去、且只在證明終止
 * 之後才被清掉」—— 換 host 不改變它的必要性。
 */

const now = Date.parse("2026-08-14T04:00:00.000Z");

test("delegated cookie name is __Secure- in production and bare locally", () => {
	assert.equal(
		getDelegatedCookieName("production"),
		"__Secure-hk_delegated_operation",
	);
	assert.equal(getDelegatedCookieName("development"), "hk_delegated_operation");
});

test("delegated cookie is HttpOnly, Lax, /adm scoped, and capped at 30 minutes", () => {
	assert.deepEqual(
		createDelegatedCookieOptions("2026-08-14T05:00:00.000Z", now, "production"),
		{
			httpOnly: true,
			secure: true,
			sameSite: "lax",
			path: "/adm",
			maxAge: 1800 + DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS,
		},
	);
	assert.equal(
		createDelegatedCookieOptions("2026-08-14T03:59:59.000Z", now, "production"),
		null,
	);
});

/**
 * **cookie 的到期時間必須 ≥ 工作階段的 `expiresAt`。**（P1-5）
 *
 * 早死一秒的後果不是「早一秒被踢出去」，是**換一種錯誤**：cookie 不在 ⇒ BFF 回
 * `401 CREDENTIAL_MISSING`（字串碼，對不上終態數字表）⇒ 工作區顯示「目前無法確認代行
 * 工作階段」而不是「代行工作階段已到期」。dev 實測早 3.3 秒，於是 expired 那段文案
 * 從來沒有出現過。
 *
 * 這一條刻意用「換算回絕對時間再比」而不是比對某個常數：改 grace 的值不會讓它變紅，
 * 改回 `Math.floor`／把 grace 拿掉才會 —— 守的是不變式，不是一個數字。
 */
test("cookie expiry is never earlier than the session expiry", () => {
	const cases = [
		{
			expiresAt: "2026-08-14T04:02:05.000Z",
			nowAt: "2026-08-14T04:00:00.000Z",
		},
		{
			expiresAt: "2026-08-14T04:02:05.700Z",
			nowAt: "2026-08-14T04:00:00.000Z",
		},
		{
			expiresAt: "2026-08-14T04:00:00.001Z",
			nowAt: "2026-08-14T04:00:00.000Z",
		},
		{
			expiresAt: "2026-08-14T04:30:00.000Z",
			nowAt: "2026-08-14T04:00:00.000Z",
		},
	];
	for (const { expiresAt, nowAt } of cases) {
		const issuedAt = Date.parse(nowAt);
		const options = createDelegatedCookieOptions(
			expiresAt,
			issuedAt,
			"development",
		);
		assert.ok(options, expiresAt);
		const cookieDiesAt = issuedAt + options.maxAge * 1000;
		assert.ok(
			cookieDiesAt >= Date.parse(expiresAt),
			`${expiresAt}: cookie 於 ${new Date(cookieDiesAt).toISOString()} 到期，早於工作階段 ⇒ 到期會被顯示成「無法確認」`,
		);
	}
});

test("cookie maxAge tracks the upstream expiry plus the fixed grace", () => {
	assert.equal(
		createDelegatedCookieOptions("2026-08-14T04:02:05.000Z", now, "development")
			?.maxAge,
		125 + DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS,
	);
	assert.equal(
		createDelegatedCookieOptions(
			"2026-08-14T04:02:00.000Z",
			now,
			"development",
			60,
		)?.maxAge,
		60 + DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS,
	);
});

test("serialized Set-Cookie carries every attribute the options declare", () => {
	const options = createDelegatedCookieOptions(
		"2026-08-14T04:30:00.000Z",
		now,
		"production",
	);
	assert.ok(options);
	const header = serializeDelegatedCookie(
		"__Secure-hk_delegated_operation",
		"dops_opaque_credential_value",
		options,
	);
	assert.equal(
		header,
		`__Secure-hk_delegated_operation=dops_opaque_credential_value; Path=/adm; Max-Age=${1800 + DELEGATED_COOKIE_EXPIRY_GRACE_SECONDS}; HttpOnly; SameSite=Lax; Secure`,
	);
});

test("credential only reads the exact cookie for the running environment", () => {
	const cookies = new Map([
		[
			"__Secure-hk_delegated_operation",
			{ value: "production-opaque-session-123" },
		],
		["hk_delegated_operation", { value: "local-opaque-session-456" }],
	]);
	assert.equal(
		readDelegatedCookie(cookies, "production")?.value,
		"production-opaque-session-123",
	);
	assert.equal(
		readDelegatedCookie(cookies, "development")?.value,
		"local-opaque-session-456",
	);
	assert.equal(
		readDelegatedCookie(
			new Map([
				["hk_delegated_operation", { value: "local-opaque-session-456" }],
			]),
			"production",
		),
		null,
	);
});

test("end clearing is permitted only when upstream proves the session is gone", () => {
	assert.equal(isDelegatedEndProven(204, null), true);
	assert.equal(
		isDelegatedEndProven(200, { code: 0, data: { ended: true } }),
		true,
	);
	assert.equal(isDelegatedEndProven(410, { code: 9270 }), true);
	assert.equal(isDelegatedEndProven(410, { code: 9271 }), true);

	for (const status of [0, 200, 401, 404, 410, 500, 503]) {
		assert.equal(isDelegatedEndProven(status, null), false, String(status));
	}
	assert.equal(isDelegatedEndProven(200, { code: 0, data: {} }), false);
	assert.equal(isDelegatedEndProven(404, { code: 9999 }), false);
});

/**
 * 除了 `/adm/enter` 與 `/adm/api/session/end`，全 repo 不得有第二處寫這個 cookie
 * 名（L1009）。一份憑證有兩個寫入點，就必然有一個在結束時忘了清。
 */
test("only the entry route and the end route may name the delegated cookie", () => {
	const root = fileURLToPath(new URL("..", import.meta.url));
	const allowed = new Set([
		"features/delegated-operations/server/session.ts",
		"app/(delegated)/adm/enter/route.ts",
		"app/(delegated)/adm/api/session/end/route.ts",
		"tests/delegated-operation-session.test.ts",
	]);
	const offenders: string[] = [];
	for (const dir of ["app", "components", "features", "lib", "tests"]) {
		for (const file of walk(join(root, dir), root)) {
			if (allowed.has(file)) continue;
			if (
				readFileSync(join(root, file), "utf8").includes(
					"hk_delegated_operation",
				)
			) {
				offenders.push(file);
			}
		}
	}
	assert.deepEqual(
		offenders,
		[],
		"a second writer of the delegated credential cookie is a second thing to forget to clear",
	);
});

function walk(dir: string, root: string): string[] {
	const out: string[] = [];
	let entries: string[];
	try {
		entries = readdirSync(dir);
	} catch {
		return out;
	}
	for (const entry of entries) {
		if (entry === "node_modules" || entry.startsWith(".")) continue;
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) out.push(...walk(full, root));
		else if (/\.(ts|tsx)$/.test(entry)) out.push(relative(root, full));
	}
	return out;
}
