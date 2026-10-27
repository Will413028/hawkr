import { create } from "@bufbuild/protobuf";
import { DurationSchema } from "@bufbuild/protobuf/wkt";
import {
	Code,
	ConnectError,
	createClient,
	createRouterTransport,
	type Interceptor,
	type UnaryRequest,
} from "@connectrpc/connect";
import { describe, expect, it, vi } from "vitest";
import { retryInterceptor } from "../src/connect/retry";
import { RetryInfoSchema } from "../src/gen/google/rpc/error_details_pb";
import { ContentAdminService } from "../src/gen/hawkr/admin/content/v1alpha1/content_pb";
import {
	ContentPublicService,
	ListLiveBannersResponseSchema,
} from "../src/gen/hawkr/content/v1alpha1/content_pb";

/** 前 failures.length 次呼叫依序丟出這些錯誤，之後成功；回傳 client 與呼叫次數。 */
function fakeService(failures: ConnectError[], interceptors: Interceptor[]) {
	let calls = 0;
	const transport = createRouterTransport(
		(router) =>
			router.service(ContentPublicService, {
				listLiveBanners() {
					calls++;
					if (calls <= failures.length) throw failures[calls - 1];
					return create(ListLiveBannersResponseSchema, {});
				},
				checkAppVersion() {
					calls++;
					if (calls <= failures.length) throw failures[calls - 1];
					return {};
				},
			}),
		{ transport: { interceptors } },
	);
	return {
		client: createClient(ContentPublicService, transport),
		calls: () => calls,
	};
}

/** 後台的服務：deleteBanner 是非冪等的寫入。 */
function fakeAdmin(failures: ConnectError[], interceptors: Interceptor[]) {
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

describe("retryInterceptor", () => {
	it("瀏覽器：讀取遇到 unavailable 以退避重試，成功就停", async () => {
		const slept: number[] = [];
		const { client, calls } = fakeService(
			[unavailable(), unavailable()],
			[
				retryInterceptor({
					sleep: async (ms) => void slept.push(ms),
					jitter: () => 0,
				}),
			],
		);
		await client.listLiveBanners({});
		expect(calls()).toBe(3);
		expect(slept).toEqual([200, 400]);
	});

	it("瀏覽器：最多重試兩次", async () => {
		const { client, calls } = fakeService(
			[unavailable(), unavailable(), unavailable(), unavailable()],
			[retryInterceptor({ sleep: async () => {}, jitter: () => 0 })],
		);
		await expect(client.listLiveBanners({})).rejects.toMatchObject({
			code: Code.Unavailable,
		});
		expect(calls()).toBe(3);
	});

	it("RetryInfo 的時間優先於退避", async () => {
		const slept: number[] = [];
		const limited = new ConnectError(
			"slow down",
			Code.ResourceExhausted,
			undefined,
			[
				{
					desc: RetryInfoSchema,
					value: { retryDelay: create(DurationSchema, { seconds: BigInt(3) }) },
				},
			],
		);
		const { client, calls } = fakeService(
			[limited],
			[
				retryInterceptor({
					sleep: async (ms) => void slept.push(ms),
					jitter: () => 0,
				}),
			],
		);
		await client.listLiveBanners({});
		expect(calls()).toBe(2);
		expect(slept).toEqual([3000]);
	});

	it("fetch 的網路錯誤（interceptor 看到的是原本的 TypeError）也會重試", async () => {
		let attempts = 0;
		const flakyNetwork: Interceptor = (next) => async (req) => {
			attempts++;
			if (attempts === 1) throw new TypeError("fetch failed");
			return next(req);
		};
		const { client, calls } = fakeService(
			[],
			[
				retryInterceptor({ sleep: async () => {}, jitter: () => 0 }),
				flakyNetwork,
			],
		);
		await client.listLiveBanners({});
		expect(attempts).toBe(2);
		expect(calls()).toBe(1);
	});

	it("server：只立即重試一次，不等待", async () => {
		const sleep = vi.fn(async () => {});
		const { client, calls } = fakeService(
			[unavailable(), unavailable()],
			[retryInterceptor({ server: true, sleep })],
		);
		await expect(client.listLiveBanners({})).rejects.toMatchObject({
			code: Code.Unavailable,
		});
		expect(calls()).toBe(2);
		expect(sleep).not.toHaveBeenCalled();
	});

	it("server：需要等待的 RetryInfo 不重試", async () => {
		const limited = new ConnectError(
			"slow down",
			Code.ResourceExhausted,
			undefined,
			[
				{
					desc: RetryInfoSchema,
					value: { retryDelay: create(DurationSchema, { seconds: BigInt(1) }) },
				},
			],
		);
		const { client, calls } = fakeService(
			[limited],
			[retryInterceptor({ server: true })],
		);
		await expect(client.listLiveBanners({})).rejects.toMatchObject({
			code: Code.ResourceExhausted,
		});
		expect(calls()).toBe(1);
	});

	it("不可重試的錯誤不重試", async () => {
		const { client, calls } = fakeService(
			[new ConnectError("no", Code.InvalidArgument)],
			[retryInterceptor({ sleep: async () => {} })],
		);
		await expect(client.checkAppVersion({})).rejects.toMatchObject({
			code: Code.InvalidArgument,
		});
		expect(calls()).toBe(1);
	});

	it("寫入（非冪等）遇到 unavailable 不重試", async () => {
		const { client, calls } = fakeAdmin(
			[unavailable()],
			[retryInterceptor({ sleep: async () => {} })],
		);
		await expect(client.deleteBanner({ id: "x" })).rejects.toMatchObject({
			code: Code.Unavailable,
		});
		expect(calls()).toBe(1);
	});

	it("串流不重試（目前的 API 沒有串流方法，直接餵串流請求給 interceptor）", async () => {
		let calls = 0;
		const next = async () => {
			calls++;
			throw unavailable();
		};
		const sleep = vi.fn(async () => {});
		for (const server of [false, true]) {
			calls = 0;
			const handler = retryInterceptor({ server, sleep })(next as never);
			await expect(
				handler({ stream: true } as unknown as UnaryRequest),
			).rejects.toMatchObject({ code: Code.Unavailable });
			expect(calls).toBe(1);
		}
		expect(sleep).not.toHaveBeenCalled();
	});
});
