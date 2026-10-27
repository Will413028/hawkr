import assert from "node:assert/strict";
import { test } from "node:test";
import { renderToStaticMarkup } from "react-dom/server";
import { GET } from "../src/app/api/admin/logout/route";
import SessionExpiredPage from "../src/app/session-expired/page";

const route = "http://localhost:3000/api/admin/logout";

test("Access assertion 導向同源的 Access logout", () => {
	const response = GET(
		new Request(route, {
			headers: { "Cf-Access-Jwt-Assertion": "access-token" },
		}),
	);
	assert.equal(response.status, 303);
	assert.equal(
		response.headers.get("location"),
		"http://localhost:3000/cdn-cgi/access/logout",
	);
	assert.equal(response.headers.get("cache-control"), "no-store");
});

test("本機 dev token 模式停在獨立的失效頁，不重新查詢 admin session", () => {
	const response = GET(new Request(route));
	assert.equal(response.status, 303);
	assert.equal(
		response.headers.get("location"),
		"http://localhost:3000/session-expired",
	);
	const html = renderToStaticMarkup(SessionExpiredPage());
	assert.match(html, /role="alert"/);
	assert.match(html, /make dev-token/);
	assert.match(html, /ADMIN_DEV_TOKEN/);
	assert.match(html, /href="\/admin"/);
});
