import assert from "node:assert/strict";
import { test } from "node:test";
import { ClerkAPIResponseError } from "@clerk/nextjs/errors";

test("Clerk refusal keeps sign-out unsuccessful", async () => {
	const { logout, LogoutRevokeFailedError } = await import("../lib/api/auth");
	await assert.rejects(
		logout(() =>
			Promise.reject(
				new ClerkAPIResponseError("server error", { data: [], status: 500 }),
			),
		),
		(error: unknown) => {
			assert.ok(error instanceof LogoutRevokeFailedError);
			assert.equal(error.reason, "server");
			return true;
		},
	);
});

test("network failure keeps sign-out unsuccessful", async () => {
	const { logout, LogoutRevokeFailedError } = await import("../lib/api/auth");
	await assert.rejects(
		logout(() => Promise.reject(new TypeError("Load failed"))),
		(error: unknown) => {
			assert.ok(error instanceof LogoutRevokeFailedError);
			assert.equal(error.reason, "network");
			return true;
		},
	);
});

test("successful sign-out ends Clerk session once", async () => {
	const { logout } = await import("../lib/api/auth");
	let ended = 0;
	await logout(async () => {
		ended += 1;
	});
	assert.equal(ended, 1);
});
