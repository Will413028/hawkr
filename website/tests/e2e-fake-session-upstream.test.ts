import assert from "node:assert/strict";
import type { AddressInfo } from "node:net";
import { after, before, test } from "node:test";
import { SESSION_COOKIE_NAME } from "../lib/auth/session-cookie";
import {
	createFakeSessionUpstream,
	decodeSessionValue,
	encodeSessionValue,
	GET_SESSION_PATH,
} from "./e2e/helpers/fake-session-upstream";

const server = createFakeSessionUpstream();
let origin = "";

before(async () => {
	await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
	origin = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
});

after(() => {
	server.close();
});

function getSession(cookie?: string) {
	return fetch(`${origin}${GET_SESSION_PATH}`, {
		headers: cookie ? { cookie } : {},
	});
}

test("the session value round-trips an identity and rejects anything else", () => {
	const identity = { accountId: "acct-7", roles: ["buyer", "seller"] as const };
	assert.deepEqual(decodeSessionValue(encodeSessionValue(identity)), identity);
	assert.equal(decodeSessionValue("e2e-session-not-base64-json"), null);
	assert.equal(decodeSessionValue("something-else"), null);
});

test("GetSession answers the account carried by the session cookie", async () => {
	const value = encodeSessionValue({
		accountId: "acct-7",
		roles: ["buyer", "seller"],
	});
	const res = await getSession(`${SESSION_COOKIE_NAME}=${value}`);
	assert.equal(res.status, 200);
	assert.deepEqual(await res.json(), {
		account: { id: "acct-7", roles: ["ROLE_SELLER"] },
	});
});

test("GetSession is unauthenticated without a usable session cookie", async () => {
	for (const cookie of [
		undefined,
		`${SESSION_COOKIE_NAME}=garbage`,
		"other=1",
	]) {
		const res = await getSession(cookie);
		assert.equal(res.status, 401);
		assert.equal(
			((await res.json()) as { code: string }).code,
			"unauthenticated",
		);
	}
});

test("every other path closes the connection like an unreachable upstream", async () => {
	await assert.rejects(fetch(`${origin}/api/v1/anything`));
});
