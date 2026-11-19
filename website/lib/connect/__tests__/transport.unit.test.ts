import { create, toBinary } from "@bufbuild/protobuf";
import { DurationSchema } from "@bufbuild/protobuf/wkt";
import {
	Code,
	ConnectError,
	createClient,
	createContextValues,
	createRouterTransport,
	type Interceptor,
} from "@connectrpc/connect";
import { retryInterceptor } from "@hawkr/api/connect/retry";
import {
	ErrorInfoSchema,
	LocalizedMessageSchema,
	RetryInfoSchema,
} from "@hawkr/api/gen/google/rpc/error_details_pb";
import { BuyerAppService } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import {
	ContentPublicService,
	ListLiveBannersResponseSchema,
} from "@hawkr/api/gen/hawkr/content/v1alpha1/content_pb";
import { IdentityAppService } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import { SellerPublicService } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { SocialPublicService } from "@hawkr/api/gen/hawkr/social/v1alpha1/social_pb";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SELLER_SUSPENDED_EVENT } from "@/lib/api/client";
import {
	DelegatedRuntimeTransportError,
	enterDelegatedRuntime,
} from "@/lib/api/delegated-runtime";
import { ACTIVE_ROLE_STORAGE_KEY } from "@/lib/auth/active-role";
import { __setEnvForTest, getEnv } from "@/lib/config/env";
import { activeLocale, setActiveLocale } from "@/lib/intl/active-locale";
import {
	__resetConnectReportDedupForTest,
	acceptLanguageInterceptor,
	authExpiredInterceptor,
	authedTransport,
	browserTransport,
	connectBaseUrl,
	type FailureContext,
	FIRST_PARTY_CONNECT_PATH,
	optionalAuthTransport,
	publicTransport,
	reportInterceptor,
	SUPPRESS_AUTH_EXPIRED,
	suspensionInterceptor,
} from "../transport";

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

const unavailable = () => new ConnectError("down", Code.Unavailable);

describe("reportInterceptor", () => {
	beforeEach(() => __resetConnectReportDedupForTest());

	it("伺服器端的失敗上報一次，30 秒內同一個失敗不重複", async () => {
		const reported: FailureContext[] = [];
		const capture = (_: ConnectError, c: FailureContext) =>
			void reported.push(c);
		const { client } = fakeService(
			[
				new ConnectError("boom", Code.Internal),
				new ConnectError("boom", Code.Internal),
			],
			[reportInterceptor(capture)],
		);
		await expect(client.listLiveBanners({})).rejects.toThrow();
		await expect(client.listLiveBanners({})).rejects.toThrow();
		expect(reported).toEqual([
			{
				procedure:
					"/hawkr.content.v1alpha1.ContentPublicService/ListLiveBanners",
				method: "POST",
				code: "Internal",
				traceId: "no-trace",
			},
		]);
	});

	it("client 的錯誤與沒收到回應不上報", async () => {
		const reported: FailureContext[] = [];
		const capture = (_: ConnectError, c: FailureContext) =>
			void reported.push(c);
		// 網路失敗只發生在 client 端：以內層的 interceptor 模擬 fetch 丟出 TypeError。
		const networkDown: Interceptor = () => async () => {
			throw ConnectError.from(new TypeError("fetch failed"));
		};
		const notFound = fakeService(
			[new ConnectError("no", Code.NotFound)],
			[reportInterceptor(capture)],
		);
		await expect(notFound.client.listLiveBanners({})).rejects.toThrow();
		const offline = fakeService([], [reportInterceptor(capture), networkDown]);
		await expect(offline.client.listLiveBanners({})).rejects.toMatchObject({
			code: Code.Unknown,
		});
		expect(reported).toEqual([]);
	});
});

describe("connectBaseUrl", () => {
	const PAGE = "https://www.example.test";
	function withBases(apiBaseUrl: string, serverApiBase: string) {
		__setEnvForTest({ ...getEnv(), apiBaseUrl, serverApiBase });
	}
	afterEach(() => __setEnvForTest(null));

	it("瀏覽器、相對的 REST base：第一方路徑", () => {
		withBases("/api/v1", "https://api.example.test/api/v1");
		expect(connectBaseUrl(true, PAGE)).toBe(FIRST_PARTY_CONNECT_PATH);
		expect(FIRST_PARTY_CONNECT_PATH).toBe("/api/connect");
	});

	it("瀏覽器、與頁面同源的絕對 base：同一個 origin 的第一方路徑，不是網站的根目錄", () => {
		withBases(`${PAGE}/api/v1`, "https://api.example.test/api/v1");
		expect(connectBaseUrl(true, PAGE)).toBe(`${PAGE}/api/connect`);
		withBases(`${PAGE}/api/v1/`, "https://api.example.test/api/v1");
		expect(connectBaseUrl(true, PAGE)).toBe(`${PAGE}/api/connect`);
	});

	it("瀏覽器、明確設定跨來源的 base：那個 API host 的根目錄", () => {
		withBases(
			"https://api.example.test/api/v1",
			"https://ssr.example.test/api/v1",
		);
		expect(connectBaseUrl(true, PAGE)).toBe("https://api.example.test");
		// 同 host 不同 port 也是不同的 origin。
		withBases("http://localhost:8080/api/v1", "http://localhost:8080/api/v1");
		expect(connectBaseUrl(true, "http://localhost:3000")).toBe(
			"http://localhost:8080",
		);
	});

	it("瀏覽器、不知道頁面的 origin：當成跨來源", () => {
		withBases(`${PAGE}/api/v1`, "https://api.example.test/api/v1");
		expect(connectBaseUrl(true, undefined)).toBe(PAGE);
	});

	it("server 端（SSR）：serverApiBase 那個 API host 的根目錄，與瀏覽器的 base 無關", () => {
		withBases("/api/v1", "https://api.example.test/api/v1");
		expect(connectBaseUrl(false)).toBe("https://api.example.test");
		withBases(`${PAGE}/api/v1`, "http://127.0.0.1:9/api/v1");
		expect(connectBaseUrl(false, PAGE)).toBe("http://127.0.0.1:9");
	});
});

describe("publicTransport", () => {
	const fetchMock = vi.fn<typeof fetch>();
	beforeEach(() => {
		fetchMock.mockResolvedValue(
			new Response("{}", {
				status: 200,
				headers: { "content-type": "application/json" },
			}),
		);
		vi.stubGlobal("fetch", fetchMock);
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
	});

	it("讀取以 GET 打 gateway 的根目錄，不帶 cookie", async () => {
		await createClient(
			ContentPublicService,
			publicTransport({ locale: "zh-TW" }),
		).listLiveBanners({ pageSize: 5 });
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toMatch(
			new RegExp(
				`^${connectBaseUrl(false)}/hawkr\\.content\\.v1alpha1\\.ContentPublicService/ListLiveBanners\\?`,
			),
		);
		expect(connectBaseUrl(false)).not.toMatch(/\/api\/v1$/);
		expect(init?.method).toBe("GET");
		expect(init?.credentials).toBe("omit");
	});
});

describe("optionalAuthTransport", () => {
	const fetchMock = vi.fn<typeof fetch>();
	const send = () =>
		createClient(
			SellerPublicService,
			optionalAuthTransport({ locale: "zh-TW" }),
		).recordProductView({ productId: "p1" });
	beforeEach(() => {
		fetchMock.mockImplementation(
			async () =>
				new Response("{}", {
					status: 200,
					headers: { "content-type": "application/json" },
				}),
		);
		vi.stubGlobal("fetch", fetchMock);
		vi.stubGlobal("window", globalThis);
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
	});

	it("帶同源 session cookie，POST 寫入，不帶 Authorization（憑證是 HttpOnly cookie）", async () => {
		await send();
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [, init] = fetchMock.mock.calls[0];
		expect(init?.method).toBe("POST");
		expect(init?.credentials).toBe("include");
		expect(new Headers(init?.headers).has("Authorization")).toBe(false);
	});
});

describe("authExpiredInterceptor", () => {
	/** 一個 AppService、一個公開 service：每個方法都丟出 error。 */
	function identityServices(error: ConnectError, interceptors: Interceptor[]) {
		const transport = createRouterTransport(
			(router) => {
				router.service(IdentityAppService, {
					getSession() {
						throw error;
					},
				});
				router.service(ContentPublicService, {
					listLiveBanners() {
						throw error;
					},
				});
			},
			{ transport: { interceptors } },
		);
		return {
			app: createClient(IdentityAppService, transport),
			pub: createClient(ContentPublicService, transport),
		};
	}

	it("AppService 回 unauthenticated：session 不在了，廣播一次", async () => {
		const emitted: Array<{ path: string; message: string }> = [];
		const { app } = identityServices(
			new ConnectError("session gone", Code.Unauthenticated),
			[authExpiredInterceptor((d) => void emitted.push(d))],
		);
		await expect(app.getSession({})).rejects.toMatchObject({
			code: Code.Unauthenticated,
		});
		expect(emitted).toEqual([
			{
				path: "/hawkr.identity.v1alpha1.IdentityAppService/GetSession",
				message: "session gone",
			},
		]);
	});

	it("呼叫方以 SUPPRESS_AUTH_EXPIRED 抑制（讀取登入的帳號）", async () => {
		const emit = vi.fn();
		const { app } = identityServices(
			new ConnectError("guest", Code.Unauthenticated),
			[authExpiredInterceptor(emit)],
		);
		await expect(
			app.getSession(
				{},
				{
					contextValues: createContextValues().set(SUPPRESS_AUTH_EXPIRED, true),
				},
			),
		).rejects.toMatchObject({ code: Code.Unauthenticated });
		expect(emit).not.toHaveBeenCalled();
	});

	it("公開 service 的 unauthenticated 與其他 code 不廣播", async () => {
		const emit = vi.fn();
		const publicUnauthenticated = identityServices(
			new ConnectError("unauthenticated", Code.Unauthenticated),
			[authExpiredInterceptor(emit)],
		);
		await expect(
			publicUnauthenticated.pub.listLiveBanners({}),
		).rejects.toThrow();
		const down = identityServices(new ConnectError("down", Code.Unavailable), [
			authExpiredInterceptor(emit),
		]);
		await expect(down.app.getSession({})).rejects.toThrow();
		expect(emit).not.toHaveBeenCalled();
	});
});

describe("suspensionInterceptor: SELLER_SUSPENDED", () => {
	function denied(reason: string, domain = "hawkr") {
		return new ConnectError("suspended", Code.PermissionDenied, undefined, [
			{ desc: ErrorInfoSchema, value: { reason, domain } },
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "賣家已停權" },
			},
		]);
	}
	function services(error: ConnectError, interceptors: Interceptor[]) {
		const transport = createRouterTransport(
			(router) => {
				router.service(IdentityAppService, {
					getSession() {
						throw error;
					},
				});
				router.service(ContentPublicService, {
					listLiveBanners() {
						throw error;
					},
				});
			},
			{ transport: { interceptors } },
		);
		return {
			app: createClient(IdentityAppService, transport),
			pub: createClient(ContentPublicService, transport),
		};
	}

	it("AppService 回 SELLER_SUSPENDED：廣播一次，錯誤原樣丟出", async () => {
		const emit = vi.fn();
		const err = denied("SELLER_SUSPENDED");
		const { app } = services(err, [suspensionInterceptor(emit)]);
		await expect(app.getSession({})).rejects.toMatchObject({
			code: Code.PermissionDenied,
			rawMessage: "suspended",
		});
		expect(emit).toHaveBeenCalledTimes(1);
		expect(emit).toHaveBeenCalledWith({
			path: "/hawkr.identity.v1alpha1.IdentityAppService/GetSession",
			message: "賣家已停權",
		});
	});

	it("別的 reason、別的 domain、別的 code 都不廣播", async () => {
		for (const err of [
			denied("VERSION_CONFLICT"),
			denied("SELLER_SUSPENDED", "example.com"),
			new ConnectError("nope", Code.PermissionDenied),
		]) {
			const emit = vi.fn();
			const { app } = services(err, [suspensionInterceptor(emit)]);
			await expect(app.getSession({})).rejects.toBeInstanceOf(ConnectError);
			expect(emit).not.toHaveBeenCalled();
		}
	});

	it("公開的 service 不廣播", async () => {
		const emit = vi.fn();
		const { pub } = services(denied("SELLER_SUSPENDED"), [
			suspensionInterceptor(emit),
		]);
		await expect(pub.listLiveBanners({})).rejects.toBeInstanceOf(ConnectError);
		expect(emit).not.toHaveBeenCalled();
	});
});

describe("suspensionInterceptor: ACCOUNT_SUSPENDED", () => {
	function denied(reason: string, domain = "hawkr") {
		return new ConnectError("suspended", Code.PermissionDenied, undefined, [
			{ desc: ErrorInfoSchema, value: { reason, domain } },
			{
				desc: LocalizedMessageSchema,
				value: { locale: "zh-TW", message: "帳號已停權" },
			},
		]);
	}
	function clients(error: ConnectError, interceptors: Interceptor[]) {
		const transport = createRouterTransport(
			(router) => {
				router.service(IdentityAppService, {
					getSession() {
						throw error;
					},
				});
				router.service(SocialPublicService, {
					recordPostView() {
						throw error;
					},
				});
				router.service(BuyerAppService, {
					recordVisit() {
						throw error;
					},
				});
			},
			{ transport: { interceptors } },
		);
		return {
			identity: createClient(IdentityAppService, transport),
			social: createClient(SocialPublicService, transport),
			buyer: createClient(BuyerAppService, transport),
		};
	}

	it("a user-initiated call gets ACCOUNT_SUSPENDED: broadcast once, error thrown unchanged", async () => {
		const emit = vi.fn();
		const { identity } = clients(denied("ACCOUNT_SUSPENDED"), [
			suspensionInterceptor(undefined, emit),
		]);
		await expect(identity.getSession({})).rejects.toMatchObject({
			code: Code.PermissionDenied,
			rawMessage: "suspended",
		});
		expect(emit).toHaveBeenCalledTimes(1);
		expect(emit).toHaveBeenCalledWith({
			path: "/hawkr.identity.v1alpha1.IdentityAppService/GetSession",
			message: "帳號已停權",
		});
	});

	it("the background write RecordVisit is skipped silently", async () => {
		const emit = vi.fn();
		const { buyer } = clients(denied("ACCOUNT_SUSPENDED"), [
			suspensionInterceptor(undefined, emit),
		]);
		await expect(buyer.recordVisit({ productId: "1" })).rejects.toBeInstanceOf(
			ConnectError,
		);
		expect(emit).not.toHaveBeenCalled();
	});

	it("a public service never broadcasts (it runs as guest)", async () => {
		const emit = vi.fn();
		const { social } = clients(denied("ACCOUNT_SUSPENDED"), [
			suspensionInterceptor(undefined, emit),
		]);
		await expect(social.recordPostView({ postId: "1" })).rejects.toBeInstanceOf(
			ConnectError,
		);
		expect(emit).not.toHaveBeenCalled();
	});

	it("each reason goes to its own emitter, once", async () => {
		const emitSeller = vi.fn();
		const emitAccount = vi.fn();
		const a = clients(denied("ACCOUNT_SUSPENDED"), [
			suspensionInterceptor(emitSeller, emitAccount),
		]);
		await expect(a.identity.getSession({})).rejects.toBeInstanceOf(
			ConnectError,
		);
		expect(emitAccount).toHaveBeenCalledTimes(1);
		expect(emitSeller).not.toHaveBeenCalled();
	});

	it("a background write that fails for another reason behaves as before", async () => {
		const emit = vi.fn();
		const { buyer } = clients(denied("VERSION_CONFLICT"), [
			suspensionInterceptor(undefined, emit),
		]);
		await expect(buyer.recordVisit({ productId: "1" })).rejects.toBeInstanceOf(
			ConnectError,
		);
		expect(emit).not.toHaveBeenCalled();
	});

	it("another reason, another domain or another code does not broadcast", async () => {
		for (const err of [
			denied("SELLER_SUSPENDED"),
			denied("ACCOUNT_SUSPENDED", "example.com"),
			new ConnectError("nope", Code.PermissionDenied),
		]) {
			const emit = vi.fn();
			const { identity } = clients(err, [
				suspensionInterceptor(undefined, emit),
			]);
			await expect(identity.getSession({})).rejects.toBeInstanceOf(
				ConnectError,
			);
			expect(emit).not.toHaveBeenCalled();
		}
	});
});

describe("authedTransport", () => {
	const fetchMock = vi.fn<typeof fetch>();
	const storage = new Map<string, string>();
	beforeEach(() => {
		fetchMock.mockImplementation(
			async () =>
				new Response("{}", {
					status: 200,
					headers: { "content-type": "application/json" },
				}),
		);
		vi.stubGlobal("fetch", fetchMock);
		// 瀏覽器：vitest 的 node 環境沒有 window 與 localStorage。
		vi.stubGlobal("window", globalThis);
		vi.stubGlobal("localStorage", {
			getItem: (k: string) => storage.get(k) ?? null,
			setItem: (k: string, v: string) => void storage.set(k, v),
			removeItem: (k: string) => void storage.delete(k),
		});
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
		storage.clear();
	});

	it("讀取以 GET 打入口的根目錄，不用 HTTP 快取，存了賣家工作區也不帶 X-Active-Role（Connect 不讀工作區）", async () => {
		storage.set(ACTIVE_ROLE_STORAGE_KEY, "seller");
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		).getSession({});
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, init] = fetchMock.mock.calls[0];
		expect(String(url)).toMatch(
			new RegExp(
				`^${connectBaseUrl(true)}/hawkr\\.identity\\.v1alpha1\\.IdentityAppService/GetSession\\?`,
			),
		);
		expect(init?.method).toBe("GET");
		expect(init?.credentials).toBe("include");
		expect(init?.cache).toBe("no-store");
		expect(new Headers(init?.headers).has("X-Active-Role")).toBe(false);
	});

	it("沒有存工作區時也不帶 X-Active-Role", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		).getSession({});
		const [, init] = fetchMock.mock.calls[0];
		expect(new Headers(init?.headers).has("X-Active-Role")).toBe(false);
	});

	it("憑證是同源的 session cookie：credentials include，不帶 Authorization", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		).getSession({});
		const [, init] = fetchMock.mock.calls[0];
		expect(init?.credentials).toBe("include");
		expect(new Headers(init?.headers).has("Authorization")).toBe(false);
	});

	it("寫入（POST）也只靠 cookie，不帶 Authorization", async () => {
		await createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		).signOut({});
		const [, init] = fetchMock.mock.calls[0];
		expect(init?.method).toBe("POST");
		expect(init?.credentials).toBe("include");
		expect(new Headers(init?.headers).has("Authorization")).toBe(false);
	});

	it("伺服器回 401 就是答案：一次請求、不重送，呼叫端收到 unauthenticated", async () => {
		fetchMock.mockImplementation(
			async () =>
				new Response('{"code":"unauthenticated","message":"expired"}', {
					status: 401,
					headers: { "content-type": "application/json" },
				}),
		);
		await expect(
			createClient(
				IdentityAppService,
				authedTransport({ locale: "zh-TW" }),
			).getSession(
				{},
				{
					contextValues: createContextValues().set(SUPPRESS_AUTH_EXPIRED, true),
				},
			),
		).rejects.toMatchObject({ code: Code.Unauthenticated });
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it("代行工作區內直接拒絕，不發請求", async () => {
		const leave = enterDelegatedRuntime();
		try {
			const error = await createClient(
				IdentityAppService,
				authedTransport({ locale: "zh-TW" }),
			)
				.getSession({})
				.catch((e: unknown) => e);
			expect(error).toBeInstanceOf(DelegatedRuntimeTransportError);
			expect(fetchMock).not.toHaveBeenCalled();
		} finally {
			leave();
		}
	});

	it("browserTransport 依 service 分派：AppService 帶 cookie，公開 service 不帶", async () => {
		const transport = browserTransport({ locale: "zh-TW" });
		await createClient(IdentityAppService, transport).getSession({});
		await createClient(ContentPublicService, transport).listLiveBanners({});
		expect(fetchMock.mock.calls.map(([, init]) => init?.credentials)).toEqual([
			"include",
			"omit",
		]);
	});

	it("已接上 sellerSuspendedInterceptor：回應帶 SELLER_SUSPENDED 時 window 收到 SELLER_SUSPENDED_EVENT", async () => {
		const info = toBinary(
			ErrorInfoSchema,
			create(ErrorInfoSchema, {
				reason: "SELLER_SUSPENDED",
				domain: "hawkr",
			}),
		);
		const body = JSON.stringify({
			code: "permission_denied",
			message: "suspended",
			details: [
				{
					type: "google.rpc.ErrorInfo",
					value: Buffer.from(info).toString("base64"),
				},
			],
		});
		fetchMock.mockImplementation(
			async () =>
				new Response(body, {
					status: 403,
					headers: { "content-type": "application/json" },
				}),
		);
		const events: Event[] = [];
		// window 是 beforeEach 設的 globalThis。
		vi.stubGlobal("dispatchEvent", (e: Event) => void events.push(e));
		const app = createClient(
			IdentityAppService,
			authedTransport({ locale: "zh-TW" }),
		);
		await expect(app.getSession({})).rejects.toMatchObject({
			code: Code.PermissionDenied,
		});
		expect(events.map((e) => e.type)).toEqual([SELLER_SUSPENDED_EVENT]);
	});
});

describe("Accept-Language（ADR 2026-10-17 D1：UI 語系由 client transport 送，單一值）", () => {
	const fetchMock = vi.fn<typeof fetch>();
	/** 瀏覽器：vitest 的 node 環境沒有 window；有 window 才有「目前頁面的語系」。 */
	function browser(pageLang?: string) {
		vi.stubGlobal("window", globalThis);
		vi.stubGlobal("document", { documentElement: { lang: pageLang ?? "" } });
	}
	beforeEach(() => {
		fetchMock.mockImplementation(
			async () =>
				new Response("{}", {
					status: 200,
					headers: { "content-type": "application/json" },
				}),
		);
		vi.stubGlobal("fetch", fetchMock);
		setActiveLocale("zh-TW");
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
		setActiveLocale("zh-TW");
	});

	const acceptLanguage = () =>
		new Headers(fetchMock.mock.calls[0][1]?.headers).get("Accept-Language");

	it("三種 transport 都帶 Accept-Language，值是路由的語系（AppProviders 設的目前語系），單一值、沒有 q 值", async () => {
		browser();
		setActiveLocale("en");
		await createClient(ContentPublicService, publicTransport()).listLiveBanners(
			{},
		);
		expect(acceptLanguage()).toBe("en");

		fetchMock.mockClear();
		await createClient(IdentityAppService, authedTransport()).getSession({});
		expect(acceptLanguage()).toBe("en");

		setActiveLocale("zh-TW");
		fetchMock.mockClear();
		await createClient(ContentPublicService, publicTransport()).listLiveBanners(
			{},
		);
		expect(acceptLanguage()).toBe("zh-TW");
	});

	it("不讀 DOM：瀏覽器的翻譯把 <html lang> 改成 zh-TW，送出的仍是路由的語系", async () => {
		// Chrome 把英文頁翻成中文時會改寫 <html lang>；D1 的 preferred_locale 不能因此記成 zh-TW
		browser("zh-TW");
		setActiveLocale("en");
		await createClient(IdentityAppService, authedTransport()).getSession({});
		expect(acceptLanguage()).toBe("en");
		fetchMock.mockClear();
		browser("en");
		setActiveLocale("zh-TW");
		await createClient(IdentityAppService, authedTransport()).getSession({});
		expect(acceptLanguage()).toBe("zh-TW");
	});

	it("connect-query 用的 browserTransport 兩條路（公開、登入）都帶，語系取自 provider 傳進來的 locale", async () => {
		browser();
		// 目前語系是 zh-TW，但 provider 明確傳 en：明確的值優先（AppProviders 傳它自己的 locale）
		const transport = browserTransport({ locale: "en" });
		await createClient(ContentPublicService, transport).listLiveBanners({});
		expect(acceptLanguage()).toBe("en");
		fetchMock.mockClear();
		await createClient(IdentityAppService, transport).getSession({});
		expect(acceptLanguage()).toBe("en");
	});

	it("server 端沒有目前頁面：傳了 locale 就用傳的，沒傳就在送出前丟錯（不悄悄送 zh-TW）", async () => {
		await createClient(
			ContentPublicService,
			publicTransport({ locale: "en" }),
		).listLiveBanners({});
		expect(acceptLanguage()).toBe("en");
		fetchMock.mockClear();
		await expect(
			createClient(ContentPublicService, publicTransport()).listLiveBanners({}),
		).rejects.toThrow(/browser-only/);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("重試沿用同一個值：每次嘗試看到的 Accept-Language 都一樣", async () => {
		browser();
		setActiveLocale("en");
		const seen: Array<string | null> = [];
		const spy: Interceptor = (next) => async (req) => {
			seen.push(req.header.get("Accept-Language"));
			return next(req);
		};
		const { client, calls } = fakeService(
			[unavailable()],
			[
				acceptLanguageInterceptor(() => activeLocale()),
				retryInterceptor({ server: true }),
				spy,
			],
		);
		await client.listLiveBanners({});
		expect(calls()).toBe(2);
		expect(seen).toEqual(["en", "en"]);
	});
});
