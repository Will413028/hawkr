import {
	type APIRequestContext,
	expect,
	type Page,
	test,
} from "@playwright/test";
import { ACCESS_CLASSES, requiredAccessForPath } from "@/lib/auth/redirect";
import {
	discoverAppPageRoutes,
	discoverPermanentRedirects,
	discoverRedirectOnlyPages,
} from "./helpers/app-route-inventory";
import { canonicalSellerSegment } from "./helpers/canonical-seller";
import {
	DYNAMIC_ROUTE_CATALOG,
	type DynamicRouteCatalogEntry,
	materializeDynamicRoute,
	type PublicDynamicFixtureIds,
} from "./helpers/dynamic-route-catalog";

const STATIC_PAGE_ROUTES = discoverAppPageRoutes({ includeDynamic: false });
const DYNAMIC_PAGE_ROUTES = discoverAppPageRoutes().filter((route) =>
	route.includes("["),
);
const PERMANENT_REDIRECTS = discoverPermanentRedirects();
const PERMANENT_REDIRECT_SOURCES = new Set(
	PERMANENT_REDIRECTS.map((redirect) => redirect.source),
);
const PROTECTED_STATIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.flatMap((route) => {
	const access = requiredAccessForPath(route);
	return access ? [{ route, access }] : [];
});

// 訪客被導去 /login 時帶的 role：buyer／seller 帶工作區，`account` 不帶（登入頁用預設分頁）。
function expectedLoginRole(
	access: NonNullable<ReturnType<typeof requiredAccessForPath>>,
): string | null {
	return ACCESS_CLASSES[access].workspace;
}
// 只做 redirect() 的 page.tsx 永遠回 307，不是 200 —— 它們與 next.config 的
// permanent redirect 同類，只是宣告在頁面裡而不是設定檔裡（見
// discoverRedirectOnlyPages 的註解說明為什麼不把它們改成 config redirect）。
const REDIRECT_ONLY_PAGE_ROUTES = new Set(discoverRedirectOnlyPages());
const PUBLIC_STATIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.filter(
	(route) =>
		!requiredAccessForPath(route) &&
		!PERMANENT_REDIRECT_SOURCES.has(route) &&
		!REDIRECT_ONLY_PAGE_ROUTES.has(route),
);
// 上面那個排除把 redirect-only 的**公開**頁從 PUBLIC 拿掉了，於是它不再屬於
// 任何一個回應類別 —— 「每個靜態頁恰好屬於一類」那條斷言因此必然紅。正解不是
// 把它塞回 PUBLIC（它不回 200），而是承認它就是第四類，並且給這一類自己的
// HTTP 契約（見下面「每個 redirect-only 公開頁在 HTTP 層轉址」）。
//
// 為什麼要扣掉受角色保護與 next.config 永久轉址的那些：`/seller-center` 與
// `/seller-center/shipments/anomalies` 同時是 redirect-only **且**受保護，而
// middleware 先跑 —— 訪客拿到的是 307 → /login，不是頁面自己的 redirect()。
// 它們的契約歸 PROTECTED 那一類，重複列進來會讓分類集合出現重複項。
const REDIRECT_ONLY_PUBLIC_PAGE_ROUTES = STATIC_PAGE_ROUTES.filter(
	(route) =>
		REDIRECT_ONLY_PAGE_ROUTES.has(route) &&
		!requiredAccessForPath(route) &&
		!PERMANENT_REDIRECT_SOURCES.has(route),
);
// 「訪客導 /login」這條契約只適用於吃 JWT 角色的路由。代行工作區（access
// `delegated`）不在其中：它不看角色，`requiredAccessForPath` 對它一律回 null，
// 守門由工作階段承擔，因此它回的是 200 骨架而不是 307。混進來會讓下面那條
// 斷言把「設計如此」報成失敗。代行自己的契約在本檔尾段獨立斷言。
const PROTECTED_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.flatMap((route) =>
	route.access === "public" || route.access === "delegated"
		? []
		: [{ ...route, access: route.access }],
);
const DELEGATED_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.filter(
	(route) => route.access === "delegated",
);
const PUBLIC_DYNAMIC_ROUTES = DYNAMIC_ROUTE_CATALOG.filter(
	(route) => route.access === "public",
);
const PUBLIC_CALLBACK_DYNAMIC_ROUTES = PUBLIC_DYNAMIC_ROUTES.filter(
	(route) => route.fixture === "order-callback",
);
const PUBLIC_DATA_DYNAMIC_ROUTES = PUBLIC_DYNAMIC_ROUTES.filter(
	(route) => route.fixture !== "order-callback",
);
const CRAWL_SEEDS = [
	"/",
	"/feed",
	"/market/products",
	"/search",
	"/seller",
	"/about",
	"/faq",
	"/contact",
	"/terms",
	"/privacy",
	"/return-policy",
	"/user-policy",
	"/seller-terms",
	"/login",
] as const;
const RETRYABLE = (status: number) => status >= 500;
const FRAMEWORK_NOT_FOUND =
	/This page could not be found|NEXT_NOT_FOUND|"statusCode"\s*:\s*404/i;

interface LinkReference {
	href: string;
	sources: Set<string>;
}

interface JsonResponse {
	status: number;
	body: unknown;
}

interface DocumentResponse {
	status: number;
	contentType: string;
	body: string;
}

function asFixtureId(value: unknown): string {
	if (
		(typeof value === "string" || typeof value === "number") &&
		/^\d+$/.test(String(value))
	) {
		return String(value);
	}
	return "";
}

async function getJsonWithRetry(
	request: APIRequestContext,
	url: string,
): Promise<JsonResponse> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 3; attempt++) {
		try {
			const response = await request.get(url, {
				failOnStatusCode: false,
				timeout: 15_000,
			});
			const status = response.status();
			if (status >= 400 && status < 500) {
				throw new Error(`${status} ${url}`);
			}
			if (RETRYABLE(status)) {
				lastFailure = `${status} ${url}`;
				continue;
			}
			const body = await response.json().catch((error: unknown) => {
				throw new Error(
					`${url} returned invalid JSON: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			});
			return { status, body };
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
			if (/^4\d\d /.test(lastFailure)) throw error;
			if (attempt === 3) break;
		}
	}
	throw new Error(`test gateway unavailable after 3 attempts: ${lastFailure}`);
}

/**
 * Resolve fixture ids from the SAME backend the server under test renders
 * against: the local origin's /api/v1 proxy follows API_BASE_URL, so a
 * prod-flavored build is probed against the formal gateway instead of being
 * handed dev-only ids it can only 404 on. Returns `null` when that catalog is
 * legitimately empty (pre-launch prod) so data-backed checks can skip; a
 * record that omits its numeric ids, or a persistent 5xx, still fails.
 *
 * The seller id is canonicalized to its `store_code` (`canonicalSellerSegment`)
 * because the feed only exposes `store_id`, and `/sellers/{store_id}` answers
 * the `308` that consolidates the subtree — not the document these checks are
 * about. The redirect itself is asserted separately, below.
 */
async function discoverPublicDynamicFixtures(
	request: APIRequestContext,
): Promise<PublicDynamicFixtureIds | null> {
	const { body } = await getJsonWithRetry(
		request,
		"/api/v1/public/products?keyword=&page=1&page_size=1",
	);
	const data = body as {
		data?: {
			records?: Array<{
				id?: unknown;
				storeId?: unknown;
				account_id?: unknown;
				seller?: {
					id?: unknown;
					storeId?: unknown;
					store_id?: unknown;
				};
			}>;
		};
	};
	const record = data.data?.records?.[0];
	if (!record) return null;
	const productId = asFixtureId(record.id);
	const storeId = asFixtureId(
		record.storeId ??
			record.account_id ??
			record.seller?.storeId ??
			record.seller?.store_id ??
			record.seller?.id,
	);

	expect(
		productId,
		"gateway must expose a numeric id on its first product fixture",
	).not.toBe("");
	expect(
		storeId,
		"the first product fixture must expose its numeric seller account id",
	).not.toBe("");

	// `null` here means the backend answered and this seller has no store_code —
	// middleware only redirects a key it could resolve, so it serves its document
	// from this path. A lookup that never got an answer throws instead of
	// reaching this `??`: middleware would still resolve the code server-side and
	// 308 us, turning one flaky lookup into a page full of unexplained redirects.
	const sellerId = (await canonicalSellerSegment(request, storeId)) ?? storeId;
	return { productId, sellerId, sellerStoreId: storeId };
}

async function getDocumentWithRetry(
	request: APIRequestContext,
	route: string,
): Promise<DocumentResponse> {
	let lastFailure = "request was not attempted";
	for (let attempt = 1; attempt <= 2; attempt++) {
		try {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
				timeout: 15_000,
			});
			const status = response.status();
			const result = {
				status,
				contentType: response.headers()["content-type"] ?? "",
				body: await response.text(),
			};
			if (!RETRYABLE(status)) return result;
			lastFailure = `${status} ${route}`;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}
	throw new Error(`${route} failed after 2 attempts: ${lastFailure}`);
}

function publicDocumentFailure(
	entry: DynamicRouteCatalogEntry,
	route: string,
	response: DocumentResponse,
): string | null {
	if (response.status < 200 || response.status >= 300) {
		return `${response.status} ${route}`;
	}
	if (!response.contentType.toLowerCase().includes("text/html")) {
		return `${response.status} ${route} returned ${response.contentType || "no content-type"}`;
	}
	if (FRAMEWORK_NOT_FOUND.test(response.body)) {
		return `${response.status} ${route} rendered the framework not-found document`;
	}
	if (
		(entry.fixture === "product" || entry.fixture === "seller-product") &&
		response.body.includes("商品已下架")
	) {
		return `${response.status} ${route} rendered the unavailable-product shell`;
	}
	return null;
}

function normalizeInternalHref(
	rawHref: string,
	currentUrl: string,
): string | null {
	try {
		const current = new URL(currentUrl);
		const target = new URL(rawHref, current);
		if (target.origin !== current.origin) return null;
		if (!["http:", "https:"].includes(target.protocol)) return null;
		if (
			target.pathname.startsWith("/_next/") ||
			target.pathname.startsWith("/api/")
		) {
			return null;
		}
		target.hash = "";
		for (const key of [
			"_rsc",
			"dpl",
			"utm_source",
			"utm_medium",
			"utm_campaign",
		]) {
			target.searchParams.delete(key);
		}
		target.searchParams.sort();
		return `${target.pathname}${target.search}`;
	} catch {
		return null;
	}
}

async function collectRenderedLinks(
	page: Page,
	seed: string,
	links: Map<string, LinkReference>,
) {
	const response = await page.goto(seed, { waitUntil: "domcontentloaded" });
	expect(response, `${seed} returned no document response`).toBeTruthy();
	expect(
		response?.status() ?? 0,
		`${seed} failed before links could be collected`,
	).toBeLessThan(400);
	await page
		.waitForFunction(
			() => (document.body?.innerText.trim().length ?? 0) > 20,
			null,
			{ timeout: 15_000 },
		)
		.catch(() => {});
	await page.waitForTimeout(250);

	const hrefs = await page
		.locator("a[href]")
		.evaluateAll((anchors) =>
			anchors
				.map((anchor) => anchor.getAttribute("href"))
				.filter((href): href is string => Boolean(href)),
		);
	for (const rawHref of hrefs) {
		const href = normalizeInternalHref(rawHref, page.url());
		if (!href) continue;
		const existing = links.get(href);
		if (existing) existing.sources.add(seed);
		else links.set(href, { href, sources: new Set([seed]) });
	}
}

async function renderedLinkFailure(
	request: APIRequestContext,
	link: LinkReference,
): Promise<string | null> {
	const parsedLink = new URL(link.href, "http://route-contract.local");
	const requiredAccess = requiredAccessForPath(parsedLink.pathname);
	let lastFailure = "request was not attempted";

	for (let attempt = 1; attempt <= 2; attempt++) {
		try {
			const response = await request.get(link.href, {
				failOnStatusCode: false,
				maxRedirects: requiredAccess ? 0 : 5,
				timeout: 15_000,
			});
			if (RETRYABLE(response.status()) && attempt < 2) continue;

			if (requiredAccess) {
				const location = response.headers().location ?? "";
				const target = new URL(location, "http://route-contract.local");
				if (
					response.status() === 307 &&
					target.pathname === "/login" &&
					target.searchParams.get("role") ===
						expectedLoginRole(requiredAccess) &&
					target.searchParams.get("redirect") === link.href
				) {
					return null;
				}
				return `${response.status()} ${link.href} -> ${location || "(no location)"}`;
			}

			if (response.status() !== 200) {
				return `${response.status()} ${link.href}`;
			}
			const finalUrl = new URL(response.url());
			if (parsedLink.pathname !== "/login" && finalUrl.pathname === "/login") {
				return `${link.href} unexpectedly ended at ${finalUrl.pathname}${finalUrl.search}`;
			}
			return null;
		} catch (error) {
			lastFailure = error instanceof Error ? error.message : String(error);
		}
	}

	return `REQUEST_FAILED ${link.href}: ${lastFailure}`;
}

test.describe("whole-site route and link contract", () => {
	test.skip(
		({ browserName, isMobile }) => browserName !== "chromium" || !!isMobile,
		"HTTP route/link inventory only needs one browser project",
	);

	test("every canonical public static page serves 200", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		test.setTimeout(180_000);
		const failures: string[] = [];
		for (const route of PUBLIC_STATIC_PAGE_ROUTES) {
			try {
				const response = await request.get(route, {
					failOnStatusCode: false,
					maxRedirects: 0,
					timeout: 15_000,
				});
				if (response.status() !== 200) {
					failures.push(`${response.status()} ${route}`);
				}
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}
		expect(
			failures,
			`${PUBLIC_STATIC_PAGE_ROUTES.length} canonical public static routes checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("every protected static page sends guests to login", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const { route, access } of PROTECTED_STATIC_PAGE_ROUTES) {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 307 ||
				parsed.pathname !== "/login" ||
				parsed.searchParams.get("redirect") !== route ||
				parsed.searchParams.get("role") !== expectedLoginRole(access)
			) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(failures).toEqual([]);
	});

	test("every redirect-only public page redirects instead of rendering", {
		tag: ["@ui"],
	}, async ({ request }) => {
		// 這一類的存在理由在上面 REDIRECT_ONLY_PUBLIC_PAGE_ROUTES 的註解裡。這條
		// 斷言是它的入場費：沒有它，第四個類別就只是一個「分不出去的丟這裡」的
		// 桶子，新頁面只要被 discoverRedirectOnlyPages 認出來就再也沒有任何契約
		// 守著它。307 + 一個不指回自己的 Location = 它真的把人送去了別的地方。
		const failures: string[] = [];
		for (const route of REDIRECT_ONLY_PUBLIC_PAGE_ROUTES) {
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (response.status() !== 307 || !location || parsed.pathname === route) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(
			failures,
			`${REDIRECT_ONLY_PUBLIC_PAGE_ROUTES.length} redirect-only public routes checked`,
		).toEqual([]);
	});

	test("every static page belongs to one explicit response class", {
		tag: ["@ui"],
	}, () => {
		const classified = [
			...PUBLIC_STATIC_PAGE_ROUTES,
			...PROTECTED_STATIC_PAGE_ROUTES.map(({ route }) => route),
			...STATIC_PAGE_ROUTES.filter((route) =>
				PERMANENT_REDIRECT_SOURCES.has(route),
			),
			...REDIRECT_ONLY_PUBLIC_PAGE_ROUTES,
		].sort();
		expect(classified).toEqual(STATIC_PAGE_ROUTES);
	});

	test("dynamic route catalog exactly matches the App Router inventory", {
		tag: ["@ui"],
	}, () => {
		const catalogPatterns = DYNAMIC_ROUTE_CATALOG.map(
			(route) => route.pattern,
		).sort();
		expect(
			catalogPatterns,
			"every addressable dynamic page needs exactly one deliberate catalog entry",
		).toEqual(DYNAMIC_PAGE_ROUTES);
	});

	test("every protected dynamic page sends guests to login", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const entry of PROTECTED_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 307 ||
				parsed.pathname !== "/login" ||
				parsed.searchParams.get("redirect") !== route ||
				parsed.searchParams.get("role") !== expectedLoginRole(entry.access)
			) {
				failures.push(
					`${response.status()} ${route} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(failures).toEqual([]);
	});

	test("delegated workspace routes stay guest-safe and unindexed", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		// 代行工作區不吃 JWT 角色，所以匿名訪客拿得到文件——這是設計（守門在
		// client 端的工作階段）。真正要守的不變式是**那份文件裡沒有賣家資料**：
		// 每一頁都只是外框，內容全部由帶 operation credential 的 client 呼叫填入。
		//
		// 這條斷言存在的理由：把 /adm 登記進路由清冊只讓比對測試變綠，本身
		// 不保證任何事。哪天有人替某一頁加上 server 端取數（那在 Next 裡是一行
		// await 的事），骨架就會開始夾帶賣家資料，而清冊完全看不出來。
		const failures: string[] = [];
		for (const entry of DELEGATED_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			const response = await request.get(route, {
				failOnStatusCode: false,
				maxRedirects: 0,
			});
			if (response.status() >= 500) {
				failures.push(`${response.status()} ${route}`);
				continue;
			}
			const html = await response.text();
			if (!/<meta[^>]+name="robots"[^>]+noindex/i.test(html)) {
				failures.push(`${route} 缺 noindex`);
			}
			// 佔位 storeId 不存在，任何真實店名/商品名都不該出現在骨架裡。
			//
			// ⚠️ **不得**把樣式寫成帶等號的 `data-seller-name=`。這個子樹的 server
			// 端產物幾乎永遠不會變成 HTML 屬性：`DelegatedWorkspaceFrame` 在
			// `loading` 為真時只畫「正在確認代行工作階段…」而**不渲染 children**，
			// 而 SSR 那一拍 `loading` 必然為真。於是頁面（server component）的產物
			// 只會以 RSC flight payload 的形式躺在 `self.__next_f.push(...)` 裡，
			// 序列化成 **JSON**：`{"data-store-name":"範例門市-9000"}` —— 沒有
			// 等號。帶等號的樣式因此對它被寫來攔的那個洩漏形狀 100% 失明。
			// web#529 實測：在 `[storeId]/storefront/page.tsx` 加一行 server 端
			// await 取數並寫進屬性，匿名 curl 拿得到店名，而斷言命中 0、測試全綠。
			if (/data-(seller|store)-name/i.test(html)) {
				failures.push(`${route} 骨架含賣家資料屬性`);
			}
			// 上面那條是黑名單，只擋得住「剛好叫這兩個名字」的屬性；
			// `<h1>{seller.name}</h1>` 這種無屬性洩漏它抓不到。這一條是它的正面
			// 對偶：匿名訪客拿到的必須就是工作階段確認畫面本身。工作區外框只有在
			// 工作階段驗過之後才渲染（`DelegatedWorkspaceFrame` 的 `.seller-shell`
			// 帶著 `data-hk-seller-workspace-ready`），它出現在匿名骨架裡就代表
			// 整個工作區被搬到了 server 端 —— 那時洩漏的欄位叫什麼名字都無所謂。
			// 它是 client component 的**輸出**而非 props，不會被 flight payload 誤觸發。
			if (/data-hk-seller-workspace-ready/i.test(html)) {
				failures.push(`${route} 骨架已渲染工作區外框（工作階段閘門被繞過）`);
			}
		}
		expect(failures).toEqual([]);
	});

	test("工作區索引在 HTTP 層轉址，且不吃掉同層的具名路由", {
		tag: ["@ui"],
	}, async ({ request }) => {
		// `/adm/{storeId}` 沒有落地頁，導向固定 /products（與 /adm/enter 兌換成功的
		// 落點同一個），不需要任何 lookup ⇒ 佔位 id 隨便給，只要是數字。
		//
		// 這條要抓的是「200 包著 client 導向」那種降級：瀏覽器點連結時它看起來完全
		// 正常，只有真的發一次 HTTP 請求才分得出來。maxRedirects: 0 是重點。
		const index = await request.get("/adm/1", {
			failOnStatusCode: false,
			maxRedirects: 0,
		});
		expect(index.status()).toBe(307);
		expect(index.headers().location).toMatch(/\/adm\/1\/products$/);

		// redirects() 在 filesystem routes 之前求值：規則放寬成裸 `:storeId` 會把
		// 代行唯一入口 /adm/enter 一起吃掉，而那是靜默的（使用者只看到轉址）。
		const siblings = await Promise.all(
			["/adm/enter", "/adm/unavailable"].map(async (path) => {
				const res = await request.get(path, {
					failOnStatusCode: false,
					maxRedirects: 0,
				});
				return `${path} -> ${res.status()} ${res.headers().location ?? ""}`.trim();
			}),
		);
		expect(siblings.filter((s) => /\/products$/.test(s))).toEqual([]);
	});

	test("every callback public dynamic page serves a real guest document", {
		tag: ["@ui", "@live"],
	}, async ({ request }) => {
		// Order-callback routes materialize deterministic placeholders, so they
		// stay fully asserted even when the public catalog carries no data.
		const failures: string[] = [];

		for (const entry of PUBLIC_CALLBACK_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry);
			try {
				const response = await getDocumentWithRetry(request, route);
				const failure = publicDocumentFailure(entry, route, response);
				if (failure) failures.push(failure);
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}

		expect(PUBLIC_DYNAMIC_ROUTES).toHaveLength(9);
		expect(PUBLIC_CALLBACK_DYNAMIC_ROUTES).toHaveLength(2);
		expect(
			failures,
			`${PUBLIC_CALLBACK_DYNAMIC_ROUTES.length} callback public dynamic routes checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("every data-backed public dynamic page serves a real guest document", {
		tag: ["@contract", "@live"],
	}, async ({ request }) => {
		test.setTimeout(120_000);
		const fixtures = await discoverPublicDynamicFixtures(request);
		// With a legitimately empty catalog (pre-launch prod) there is no real
		// product/seller id to materialize these routes with; they resume as soon
		// as the catalog carries a product. Broken fixtures and 5xx still fail
		// inside discoverPublicDynamicFixtures.
		test.skip(
			fixtures === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (!fixtures) return;
		const failures: string[] = [];

		for (const entry of PUBLIC_DATA_DYNAMIC_ROUTES) {
			const route = materializeDynamicRoute(entry, fixtures);
			try {
				const response = await getDocumentWithRetry(request, route);
				const failure = publicDocumentFailure(entry, route, response);
				if (failure) failures.push(failure);
			} catch (error) {
				failures.push(
					`REQUEST_FAILED ${route}: ${
						error instanceof Error ? error.message : String(error)
					}`,
				);
			}
		}

		expect(PUBLIC_DATA_DYNAMIC_ROUTES).toHaveLength(7);
		expect(
			failures,
			`${PUBLIC_DATA_DYNAMIC_ROUTES.length} data-backed public dynamic routes checked with product ${fixtures.productId} / seller ${fixtures.sellerId}:\n${failures.join("\n")}`,
		).toEqual([]);
	});

	test("every permanent redirect preserves its destination and query", {
		tag: ["@ui"],
	}, async ({ request }) => {
		const failures: string[] = [];
		for (const redirect of PERMANENT_REDIRECTS) {
			const response = await request.get(
				`${redirect.source}?route_contract=1`,
				{
					failOnStatusCode: false,
					maxRedirects: 0,
				},
			);
			const location = response.headers().location ?? "";
			const parsed = new URL(location, "http://route-contract.local");
			if (
				response.status() !== 308 ||
				parsed.pathname !== redirect.destination ||
				parsed.searchParams.get("route_contract") !== "1"
			) {
				failures.push(
					`${response.status()} ${redirect.source} -> ${location || "(no location)"}`,
				);
			}
		}
		expect(PERMANENT_REDIRECTS.length).toBeGreaterThan(0);
		expect(failures).toEqual([]);
	});

	/**
	 * The storefront consolidation itself, on the wire. It cannot ride along in
	 * the document checks above (those enter canonically, by design), and it is
	 * the one part of the mechanism that no unit test can see: the tab pages are
	 * statically prerendered, so a redirect thrown from a page ships as a
	 * `<meta http-equiv="refresh">` inside a 200 — a status that satisfies every
	 * browser-level assertion while telling crawlers nothing. Asserting the
	 * status line is what distinguishes the two.
	 */
	test("a store_id storefront url is consolidated onto its store_code", {
		tag: ["@contract", "@live"],
	}, async ({ request }) => {
		const fixtures = await discoverPublicDynamicFixtures(request);
		test.skip(
			fixtures === null,
			"public catalog legitimately empty (pre-launch prod)",
		);
		if (!fixtures) return;
		const storeId = fixtures.sellerStoreId ?? fixtures.sellerId;
		const response = await request.get(
			`/sellers/${storeId}/reviews?route_contract=1`,
			{ failOnStatusCode: false, maxRedirects: 0 },
		);

		// A seller whose feed key IS its canonical segment has nothing to be sent
		// to; it must serve its document rather than redirect to itself.
		if (storeId === fixtures.sellerId) {
			expect(
				response.status(),
				`/sellers/${storeId}/reviews is already canonical and must not redirect`,
			).toBe(200);
			return;
		}

		const location = response.headers().location ?? "";
		const target = new URL(location, "http://route-contract.local");
		expect(
			`${response.status()} ${target.pathname}?${target.searchParams.toString()}`,
			`/sellers/${storeId}/reviews must 308 to its store_code with the query intact`,
		).toBe(`308 /sellers/${fixtures.sellerId}/reviews?route_contract=1`);
	});

	test("every rendered internal link reaches a valid document", {
		tag: ["@contract", "@live"],
	}, async ({ page, request }) => {
		test.setTimeout(180_000);
		const links = new Map<string, LinkReference>();
		for (const seed of CRAWL_SEEDS) {
			await collectRenderedLinks(page, seed, links);
		}

		const failures: string[] = [];
		for (const link of links.values()) {
			const failure = await renderedLinkFailure(request, link);
			if (failure) {
				failures.push(
					`${failure} (rendered on ${[...link.sources].join(", ")})`,
				);
			}
		}

		expect(links.size, "crawler found no internal links").toBeGreaterThan(0);
		expect(
			failures,
			`${links.size} rendered internal links checked:\n${failures.join("\n")}`,
		).toEqual([]);
	});
});
