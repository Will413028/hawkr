import assert from "node:assert/strict";
import { type ChildProcessWithoutNullStreams, spawn } from "node:child_process";
import { once } from "node:events";
import { access } from "node:fs/promises";
import { createServer, type Server } from "node:http";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { chromium, type Page } from "@playwright/test";
import { hermeticEnv } from "./helpers/hermetic-env";
import { isLoopbackHost } from "./setup/loopback.mjs";

const RUN_SMOKE = process.env.RUN_SELLER_PREVIEW_SSR_SMOKE === "1";
const CONFIGURED_API_BASE =
	process.env.SELLER_PREVIEW_SSR_API_BASE_URL?.replace(/\/$/, "");
const CONFIGURED_SELLER_ID = process.env.SELLER_PREVIEW_SSR_SELLER_ID;
const CONFIGURED_SELLER_NAME = process.env.SELLER_PREVIEW_SSR_SELLER_NAME;
const ERROR_SHELL =
	/Application error|Internal Server Error|NEXT_NOT_FOUND|This page could not be found|糟糕，火箭偏離了航道|暫時迷航|這頁不在貨架上/i;

interface RouteExpectation {
	suffix: string;
	marker: string;
}

const ROUTES: RouteExpectation[] = [
	{ suffix: "", marker: "此賣家目前沒有商品" },
	{ suffix: "/posts", marker: "還沒有貼文" },
	{ suffix: "/messages", marker: "暫無留言" },
	{ suffix: "/reviews", marker: "0 則評價" },
];

function listen(server: Server, port = 0): Promise<number> {
	return new Promise((resolve, reject) => {
		server.once("error", reject);
		server.listen(port, "127.0.0.1", () => {
			server.off("error", reject);
			const address = server.address();
			assert(address && typeof address === "object");
			resolve(address.port);
		});
	});
}

async function close(server: Server): Promise<void> {
	if (!server.listening) return;
	server.close();
	await once(server, "close");
}

async function reservePort(): Promise<number> {
	const probe = createServer();
	const port = await listen(probe);
	await close(probe);
	return port;
}

function jsonResponse(data: unknown): string {
	return JSON.stringify({ code: 0, data });
}

function startApiStub(sellerId: string, seenRequests: string[]): Server {
	return createServer((request, response) => {
		const url = new URL(request.url ?? "/", "http://127.0.0.1");
		seenRequests.push(
			`${request.method ?? "GET"} ${url.pathname}${url.search}`,
		);

		const sellerRoot = `/api/v1/public/sellers/${sellerId}`;
		let data: unknown;
		switch (url.pathname) {
			case sellerRoot:
				// 全 camelCase（backend#2235）。`handler.PublicSellerInfo` 沒有 accountId，
				// 賣家的營運鍵就是 storeId（44 號設計書買賣家身分分離）。
				data = {
					storeId: sellerId,
					name: "SSR Stub 商店",
					storeName: "SSR Stub 商店",
					description: "Deterministic seller preview fixture",
					location: "臺灣",
				};
				break;
			case `${sellerRoot}/stats`:
				data = { storeId: sellerId, followers: 7 };
				break;
			case `${sellerRoot}/reviews/stats`:
				data = {
					avgRating: 0,
					totalCount: 0,
					rating1Count: 0,
					rating2Count: 0,
					rating3Count: 0,
					rating4Count: 0,
					rating5Count: 0,
				};
				break;
			case `${sellerRoot}/products`:
			case `${sellerRoot}/reviews`:
				data = { records: [], total: 0, totalPages: 0 };
				break;
			case `${sellerRoot}/posts`:
				data = { records: [], total: 0, page: 1, pageSize: 12 };
				break;
			case `${sellerRoot}/forum`:
				data = { records: [], total: 0, page: 1, pageSize: 20 };
				break;
			case "/api/v1/public/categories/tree":
				data = [];
				break;
			default:
				data = { records: [], total: 0, totalPages: 0 };
		}

		response.writeHead(200, {
			"content-type": "application/json; charset=utf-8",
			"cache-control": "no-store",
		});
		response.end(jsonResponse(data));
	});
}

/**
 * `/seller-center/*` is gated twice on the client before any route child — and
 * therefore before `StorefrontPreviewBoundary` — can mount:
 *
 *  1. `RoleZoneGuard` (components/shared/RoleZoneGuard.tsx:266) renders `null`
 *     until the session probe settles with the seller role.
 *  2. `SellerCenterShell` (features/seller-center/components/SellerCenterShell.tsx)
 *     redirects to `/seller/onboarding` whenever `useSellerIdentity()` reports
 *     `setupIncomplete`, and that flag is true for any `GetMyStore` response
 *     whose store has not completed onboarding (components/shared/useSellerIdentity.ts).
 *
 * A catch-all `{records: []}` therefore made the stubbed seller look like a
 * half-registered account, so the whole workspace was replaced by the 開店申請
 * form and the read-only boundary never mounted. Answer the two identity
 * endpoints with the shape the real services return (session over REST,
 * the store over the seller Connect API).
 */
function browserStubPayload(pathname: string, sellerId: string): unknown {
	if (pathname.endsWith("/app/auth/session")) {
		return { accountId: 2, accountType: 2, loginRole: 0 };
	}
	return { records: [], total: 0, totalPages: 0 };
}

async function installBrowserApiStub(
	page: Page,
	appOrigin: string,
	sellerId: string,
) {
	await page.route("**/api/v1/**", async (route) => {
		const request = route.request();
		const pathname = new URL(request.url()).pathname;
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			headers: {
				"access-control-allow-credentials": "true",
				"access-control-allow-origin": appOrigin,
			},
			body: jsonResponse(browserStubPayload(pathname, sellerId)),
		});
	});
	// ⚠️ 店家必須是完成新手引導的：否則 `useSellerIdentity` 判定 setupIncomplete →
	// `SellerCenterShell` 把整個工作區換成開店申請表單 → 唯讀邊界從未掛載，測試只會說
	// 「marker 沒出現」，看不出真正原因是 fixture。形狀是 proto3 JSON 的 GetMyStoreResponse。
	await page.route(/SellerAppService\/GetMyStore(?:\?|$)/, async (route) => {
		await route.fulfill({
			status: 200,
			contentType: "application/json",
			headers: {
				"access-control-allow-credentials": "true",
				"access-control-allow-origin": appOrigin,
			},
			body: JSON.stringify({
				store: {
					id: sellerId,
					storeName: "SSR Stub 商店",
					onboardingCompleted: true,
					status: "STORE_STATUS_ACTIVE",
					etag: "e1",
				},
				capabilities: [
					{ capability: "SELLER_CAPABILITY_PUBLIC_LISTING", active: true },
				],
			}),
		});
	});
}

interface ReadonlyProbe {
	insidePrevented: boolean | null;
	outsidePrevented: boolean | null;
}

/**
 * `StorefrontPreviewBoundary`'s contract is behavioural: its `onClickCapture`
 * cancels product / post / forum / pagination clicks before any child handler
 * runs, so a seller inspecting their own storefront cannot actually buy, follow
 * or comment. Asserting only that `data-storefront-preview-readonly` is
 * attached would stay green if someone deleted those capture handlers and left
 * the attribute behind — "the marker is still there, the guard is not" is
 * precisely the failure shape this file exists to catch.
 *
 * Synthetic anchors instead of fixture elements: under the empty-record stub
 * three of the four routes render zero interactive descendants, so a probe that
 * depended on real links would silently assert nothing on those routes. The
 * anchor is appended, clicked and removed within a single synchronous task, so
 * React never observes it.
 *
 * The outside probe is a control, not decoration: without it, "the click was
 * cancelled" cannot be distinguished from "nothing on this page is clickable".
 * It is mounted in `main.page-content`, which contains the boundary but is not
 * inside it.
 *
 * `<button>` rather than `<a href>` — both are in the guard's selector list, and
 * buttons are what the real buyer actions (加入購物車 / 追蹤 / 送出留言) use, but
 * only the anchor carries a navigating default action. A synthetic click on an
 * anchor runs "follow the hyperlink" even though the event is untrusted, so the
 * uncancelled control would navigate the page away mid-test. A button has no
 * such default action, which lets the verdict come straight from
 * `dispatchEvent`'s return value: false iff something cancelled the event.
 * Reading `defaultPrevented` from a listener instead would depend on where
 * React attaches its root listener and in which phase it dispatches — an
 * ordering assumption this test has no reason to take on.
 */
async function probeReadonlyBoundary(page: Page): Promise<ReadonlyProbe> {
	return page.evaluate(() => {
		const boundary = document.querySelector(
			'[data-storefront-preview-readonly="true"]',
		);
		const outside = document.querySelector("main.page-content");
		if (!boundary || !outside) {
			return { insidePrevented: null, outsidePrevented: null };
		}
		const clickProbe = (parent: Element): boolean => {
			const button = document.createElement("button");
			button.type = "button";
			button.textContent = "readonly probe";
			parent.appendChild(button);
			const delivered = button.dispatchEvent(
				new MouseEvent("click", { bubbles: true, cancelable: true }),
			);
			button.remove();
			return !delivered;
		};
		return {
			insidePrevented: clickProbe(boundary),
			outsidePrevented: clickProbe(outside),
		};
	});
}

/**
 * A bare `locator.waitFor` timeout reports only the selector, discarding the
 * `pageerror` / `console.error` output collected for exactly this moment and
 * hiding whatever the client gates rendered instead. Surface both before
 * rethrowing — that difference is what separates "the guard is missing" from
 * "the stub sent the workspace to onboarding".
 */
async function waitAttachedOrExplain(
	page: Page,
	selector: string,
	runtimeIssues: string[],
	pathname: string,
) {
	try {
		await page
			.locator(selector)
			.waitFor({ state: "attached", timeout: 15_000 });
	} catch (error) {
		const rendered = await page
			.locator("main.page-content")
			.innerText()
			.catch(() => "<main.page-content unavailable>");
		throw new Error(
			`${pathname} never attached ${selector}\nruntimeIssues:\n${
				runtimeIssues.join("\n") || "<none>"
			}\nmain.page-content text:\n${rendered.slice(0, 1_000)}`,
			{ cause: error },
		);
	}
}

async function stopProcess(
	child: ChildProcessWithoutNullStreams,
): Promise<void> {
	if (child.exitCode !== null || child.signalCode !== null) return;
	child.kill("SIGTERM");
	const exited = once(child, "exit");
	const timeout = new Promise<"timeout">((resolve) => {
		setTimeout(() => resolve("timeout"), 5_000).unref();
	});
	if ((await Promise.race([exited, timeout])) === "timeout") {
		child.kill("SIGKILL");
		await once(child, "exit");
	}
}

async function waitForNext(
	origin: string,
	child: ChildProcessWithoutNullStreams,
	logs: () => string,
): Promise<void> {
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		if (child.exitCode !== null) {
			throw new Error(
				`next start exited early with ${child.exitCode}\n${logs()}`,
			);
		}
		try {
			const response = await fetch(`${origin}/login`, {
				signal: AbortSignal.timeout(2_000),
			});
			if (response.status < 500) return;
		} catch {
			// next start has not opened its listener yet.
		}
		await new Promise((resolve) => setTimeout(resolve, 200));
	}
	throw new Error(`next start did not become ready\n${logs()}`);
}

test("production Next serves all four seller preview routes and the read-only guard holds after hydration", {
	skip: !RUN_SMOKE,
	timeout: 90_000,
}, async (context) => {
	await access(".next/BUILD_ID");

	// A fresh numeric ID prevents Next's cross-process `use cache` entries from
	// satisfying the request before it reaches this run's API stub. Supplying
	// SELLER_PREVIEW_SSR_API_BASE_URL + SELLER_PREVIEW_SSR_SELLER_ID switches
	// the same harness into an explicit, repeatable smoke against a backend you run
	// yourself. The `next start` child carries the hermetic guard (hermeticEnv), which
	// refuses every host that is not loopback, so that backend must be local.
	if (
		CONFIGURED_API_BASE &&
		!isLoopbackHost(new URL(CONFIGURED_API_BASE).hostname)
	) {
		throw new Error(
			`SELLER_PREVIEW_SSR_API_BASE_URL must be a loopback address (got ${CONFIGURED_API_BASE}): ` +
				"tests never reach a real host",
		);
	}
	const sellerId = CONFIGURED_SELLER_ID ?? `98${Date.now()}`;
	const sellerName = CONFIGURED_SELLER_NAME ?? "SSR Stub 商店";
	const seenRequests: string[] = [];
	const apiServer = CONFIGURED_API_BASE
		? null
		: startApiStub(sellerId, seenRequests);
	const apiPort = apiServer ? await listen(apiServer) : null;
	const apiBase = CONFIGURED_API_BASE ?? `http://127.0.0.1:${apiPort}/api/v1`;
	const appPort = await reservePort();
	const origin = `http://127.0.0.1:${appPort}`;

	const nextBin = fileURLToPath(
		new URL("../node_modules/next/dist/bin/next", import.meta.url),
	);
	const child = spawn(
		process.execPath,
		[nextBin, "start", "-H", "127.0.0.1", "-p", String(appPort)],
		{
			cwd: process.cwd(),
			env: hermeticEnv({
				API_BASE_URL: apiBase,
				NEXT_TELEMETRY_DISABLED: "1",
				PORT: String(appPort),
			}),
			stdio: ["ignore", "pipe", "pipe"],
		},
	);
	let childOutput = "";
	const appendOutput = (chunk: Buffer) => {
		childOutput = `${childOutput}${chunk.toString()}`.slice(-20_000);
	};
	child.stdout.on("data", appendOutput);
	child.stderr.on("data", appendOutput);

	context.after(async () => {
		await stopProcess(child);
		if (apiServer) await close(apiServer);
	});

	await waitForNext(origin, child, () => childOutput);

	// Document-level reachability + route-tree check. Deliberately NOT a claim
	// that these routes are server-rendered — they are not, and should not be.
	// `/seller-center/*` sits behind `RoleZoneGuard`
	// (components/shared/RoleZoneGuard.tsx:266), a client-only gate that renders
	// `null` during SSR so the seller workspace never emits its contents for an
	// anonymous document request. That is the correct design, so "SSR renders
	// the preview" is not a property this app has and not one this test asserts.
	//
	// Concretely: strip every <script> from these responses and the remaining
	// DOM is just the initial-loader shell — no 買家視角預覽, no seller-tab-frame,
	// no data-storefront-preview-readonly. The strings matched below live in the
	// serialised RSC flight payload, which is all this loop claims: the
	// production runtime answered, and it produced this route's layout tree.
	//
	// This is not a Cache Components postponement — next.config.ts does not
	// enable `cacheComponents` (an earlier comment here blamed it, and cost a
	// later reader a full investigation). The read-only guarantee that actually
	// matters is asserted against a live browser below.
	for (const { suffix } of ROUTES) {
		const pathname = `/seller-center/preview/${sellerId}${suffix}`;
		const response = await fetch(`${origin}${pathname}`, {
			headers: {
				accept: "text/html",
				cookie: "hk_session=opaque",
			},
			redirect: "manual",
			signal: AbortSignal.timeout(20_000),
		});
		const flight = await response.text();

		assert.equal(response.status, 200, `${pathname}\n${childOutput}`);
		assert.match(
			response.headers.get("content-type") ?? "",
			/^text\/html\b/,
			pathname,
		);
		assert.match(
			flight,
			/買家視角預覽/,
			`${pathname} flight payload carries no preview layout tree`,
		);
		assert(
			flight.includes(sellerName),
			`${pathname} flight payload never resolved the seller: ${sellerName}`,
		);
		assert.doesNotMatch(flight, ERROR_SHELL, pathname);
	}

	// System Chrome by default (#281). This was the only launch call in the repo
	// still on Playwright's bundled build, and that is why the test had never
	// actually executed anywhere: it is opt-in locally and CI never set
	// RUN_SELLER_PREVIEW_SSR_SMOKE, so the missing download surfaced as neither
	// a failure nor a skip — just a test nobody had ever run.
	//
	// `PW_CHANNEL=bundled` is the explicit opt-out CI uses: the runner installs
	// `playwright install --only-shell` and has no branded Chrome, and adding one
	// would burden every CI run for the sake of this test. A named sentinel
	// rather than an empty string — "" is falsy and would silently fall back to
	// "chrome", so the env var could never express "use the bundled browser".
	const rawChannel = process.env.PW_CHANNEL ?? "chrome";
	const channel = rawChannel === "bundled" ? undefined : rawChannel;
	const browser = await chromium.launch({ headless: true, channel });
	context.after(() => browser.close());
	for (const { suffix, marker } of ROUTES) {
		const pathname = `/seller-center/preview/${sellerId}${suffix}`;
		const page = await browser.newPage();
		const runtimeIssues: string[] = [];
		page.on("pageerror", (error) => {
			runtimeIssues.push(`pageerror: ${error.message}`);
		});
		page.on("console", (message) => {
			if (message.type() === "error") {
				runtimeIssues.push(`console.error: ${message.text()}`);
			}
		});
		await installBrowserApiStub(page, origin, sellerId);
		await page
			.context()
			.addCookies([{ name: "hk_session", value: "opaque", url: origin }]);
		await page.addInitScript(() => {
			localStorage.setItem("hkui:auth:active-role", "seller");
			sessionStorage.setItem("hawkr:initial-loader-seen", "1");
		});

		const response = await page.goto(`${origin}${pathname}`, {
			waitUntil: "domcontentloaded",
			timeout: 20_000,
		});
		assert.equal(response?.status(), 200, `${pathname}\n${childOutput}`);
		const previewContent = page.locator(
			'[data-storefront-preview-readonly="true"]',
		);
		await waitAttachedOrExplain(
			page,
			'[data-storefront-preview-readonly="true"]',
			runtimeIssues,
			pathname,
		);
		await waitAttachedOrExplain(
			page,
			'[data-auth-hydrated="true"]',
			runtimeIssues,
			pathname,
		);
		const pageText = await page.locator("main.page-content").innerText();
		const previewText = (await previewContent.innerText()).trim();
		assert(previewText.length > 0, `${pathname} rendered an empty preview tab`);
		if (!CONFIGURED_API_BASE) {
			assert(
				pageText.includes(marker),
				`${pathname} is missing its route content marker: ${marker}`,
			);
		}
		assert.doesNotMatch(pageText, ERROR_SHELL, pathname);

		// The attached check above is the precondition (is the subtree mounted at
		// all); this is the guarantee (does the guard actually cancel buyer
		// interactions). Keeping both is what separates "never mounted" from
		// "mounted but no longer intercepting".
		const probe = await probeReadonlyBoundary(page);
		assert.equal(
			probe.insidePrevented,
			true,
			`${pathname} did not cancel a click inside the read-only boundary — the preview is interactive`,
		);
		assert.equal(
			probe.outsidePrevented,
			false,
			`${pathname} cancelled a click outside the boundary too, so the probe proves nothing about the guard`,
		);

		for (const tabSuffix of ["", "/posts", "/messages", "/reviews"]) {
			const href = `/seller-center/preview/${sellerId}${tabSuffix}`;
			const tab = page.locator(`a[role="tab"][href="${href}"]`);
			await tab.waitFor({ state: "visible", timeout: 10_000 });
			assert.equal(
				await tab.getAttribute("aria-selected"),
				tabSuffix === suffix ? "true" : "false",
				`${pathname} has the wrong active tab for ${href}`,
			);
		}

		await page.waitForTimeout(300);
		assert.deepEqual(runtimeIssues, [], pathname);
		await page.close();
	}

	if (!CONFIGURED_API_BASE) {
		const seenPaths = new Set(
			seenRequests.map((entry) => entry.split(" ")[1]?.split("?")[0]),
		);
		for (const expectedPath of [
			`/api/v1/public/sellers/${sellerId}`,
			`/api/v1/public/sellers/${sellerId}/products`,
			`/api/v1/public/sellers/${sellerId}/posts`,
			`/api/v1/public/sellers/${sellerId}/forum`,
			`/api/v1/public/sellers/${sellerId}/reviews`,
			`/api/v1/public/sellers/${sellerId}/reviews/stats`,
			`/api/v1/public/sellers/${sellerId}/stats`,
			"/api/v1/public/products",
		]) {
			assert(
				seenPaths.has(expectedPath),
				`SSR never requested ${expectedPath}\nSeen:\n${seenRequests.join("\n")}`,
			);
		}
	}
});
