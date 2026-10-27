import assert from "node:assert/strict";
import { afterEach, test } from "node:test";
import { adminSession } from "../src/lib/session";

const originalDevToken = process.env.ADMIN_DEV_TOKEN;
const originalNodeEnv = process.env.NODE_ENV;

afterEach(() => {
	if (originalDevToken === undefined) delete process.env.ADMIN_DEV_TOKEN;
	else process.env.ADMIN_DEV_TOKEN = originalDevToken;
	if (originalNodeEnv === undefined)
		Reflect.deleteProperty(process.env, "NODE_ENV");
	else Reflect.set(process.env, "NODE_ENV", originalNodeEnv);
});

test("Access assertion 優先於本機 token，BFF 不讀瀏覽器 Authorization", async () => {
	process.env.ADMIN_DEV_TOKEN = "dev-token";
	const request = new Request("https://admin.example.test/api/bff/x", {
		headers: {
			"Cf-Access-Jwt-Assertion": "access-jwt",
			Authorization: "Bearer browser-token",
		},
	});
	assert.equal(await adminSession.token(request), "access-jwt");
});

test("沒有 Access assertion 時使用設定的本機 token", async () => {
	process.env.ADMIN_DEV_TOKEN = "dev-token";
	Reflect.set(process.env, "NODE_ENV", "production");
	assert.equal(
		await adminSession.token(new Request("http://localhost:3000/api/bff/x")),
		"dev-token",
	);
});

test("沒有 Access assertion 或本機 token 時回 401 所需的空憑證", async () => {
	delete process.env.ADMIN_DEV_TOKEN;
	const request = new Request("https://admin.example.test/api/bff/x", {
		headers: { Authorization: "Bearer browser-token" },
	});
	assert.equal(await adminSession.token(request), null);
});
