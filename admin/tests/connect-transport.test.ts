import assert from "node:assert/strict";
import test from "node:test";
import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
	type Interceptor,
} from "@connectrpc/connect";
import { retryInterceptor } from "@hawkr/api/connect/retry";
import { ContentAdminService } from "@hawkr/api/gen/hawkr/admin/content/v1alpha1/content_pb";
import { sessionInterceptor } from "../src/lib/connect/transport";

/** 前 failures.length 次呼叫依序丟出這些錯誤，之後成功。 */
function fakeAdmin(failures: Error[], interceptors: Interceptor[]) {
	let calls = 0;
	const handle = () => {
		calls++;
		if (calls <= failures.length) throw failures[calls - 1];
		return {};
	};
	const transport = createRouterTransport(
		(router) =>
			router.service(ContentAdminService, {
				listBanners: handle,
				deleteBanner: handle,
			}),
		{ transport: { interceptors } },
	);
	return {
		client: createClient(ContentAdminService, transport),
		calls: () => calls,
	};
}

const unavailable = () => new ConnectError("down", Code.Unavailable);

const unauthenticated = () => new ConnectError("gone", Code.Unauthenticated);

test("unauthenticated：通知 session 失效一次，錯誤照樣交給呼叫端，不重試", async () => {
	let expired = 0;
	const { client, calls } = fakeAdmin(
		[unauthenticated()],
		[
			sessionInterceptor(() => expired++),
			retryInterceptor({ sleep: async () => {} }),
		],
	);
	await assert.rejects(
		client.listBanners({}),
		(e: unknown) =>
			e instanceof ConnectError && e.code === Code.Unauthenticated,
	);
	assert.equal(expired, 1);
	assert.equal(calls(), 1);
});

test("其他錯誤與重試後成功的呼叫不當成 session 失效", async () => {
	let expired = 0;
	const interceptors = [
		sessionInterceptor(() => expired++),
		retryInterceptor({ sleep: async () => {}, jitter: () => 0 }),
	];
	await fakeAdmin([unavailable()], interceptors).client.listBanners({});
	await assert.rejects(
		fakeAdmin(
			[new ConnectError("no", Code.PermissionDenied)],
			interceptors,
		).client.deleteBanner({ id: "x" }),
	);
	assert.equal(expired, 0);
});

test("query 層不重試 Connect 的錯誤（transport 已依 D7 處理），舊 REST 的錯誤維持一次", async () => {
	const { shouldRetryQuery } = await import("../src/lib/query/client");
	assert.equal(
		shouldRetryQuery(0, new ConnectError("down", Code.Unavailable)),
		false,
	);
	assert.equal(shouldRetryQuery(0, new Error("REST 失敗")), true);
	assert.equal(shouldRetryQuery(1, new Error("REST 失敗")), false);
});
