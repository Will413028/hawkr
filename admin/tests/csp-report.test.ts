import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { POST } from "../src/app/api/csp-report/route";

const testDir = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(testDir);

// Rolled back to report-only: enforcement blocked the presigned-R2 upload PUT
// on every image flow (connect-src only allows 'self'). Restore the `= false;`
// expectation together with the connect-src fix.
test("CSP ships as report-only while the R2 connect-src gap is open", () => {
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	assert.match(config, /const CSP_REPORT_ONLY = true;/);
});

test("enforced CSP carries the required hardening directives", () => {
	const config = readFileSync(join(projectRoot, "next.config.ts"), "utf8");
	const policy = config.match(
		/const CONTENT_SECURITY_POLICY = \[([\s\S]*?)\]\.join\("; "\)/,
	)?.[1];
	assert.ok(policy);
	for (const directive of [
		"object-src 'none'",
		"base-uri 'self'",
		"frame-ancestors 'none'",
		"report-uri /api/csp-report",
	]) {
		assert.match(policy, new RegExp(directive.replace(/'/g, "['\"]")));
	}
});

test("csp-report endpoint accepts a violation report and responds 204", async () => {
	const res = await POST(
		new Request("https://admin.example.com/api/csp-report", {
			method: "POST",
			body: JSON.stringify({
				"csp-report": { "violated-directive": "script-src" },
			}),
		}) as never,
	);
	assert.equal(res.status, 204);
});

test("csp-report endpoint tolerates a malformed body without throwing", async () => {
	const res = await POST(
		new Request("https://admin.example.com/api/csp-report", {
			method: "POST",
			body: "not json",
		}) as never,
	);
	assert.equal(res.status, 204);
});
