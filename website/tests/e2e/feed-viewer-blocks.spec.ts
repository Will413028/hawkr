import { expect, type Page, type Route, test } from "@playwright/test";
import { SESSION_COOKIE_NAME } from "../../lib/auth/session-cookie";
import {
	allPosts,
	BLOCKED_BODY,
	BLOCKED_STORE,
	OTHER_BODY,
	OTHER_STORE,
} from "./helpers/fake-social-fixtures";
import {
	connectRequestMessage,
	fulfillConnect,
	seedStoredActiveRole,
	setupGatewayMocks,
	socialMethod,
} from "./helpers/mock-gateway";
import { gotoStreamed } from "./helpers/streamed-navigation";

/**
 * 店家貼文頁（`/sellers/<store_code>/posts`）的第一頁是伺服器端讀的匿名答案，由 e2e 的 upstream（`fake-social-fixtures.ts`）回，
 * 內含被封鎖店家的貼文；瀏覽器端的讀取由 `page.route` 回，帶 session cookie 才是「過濾後」的答案。
 */

const LOADER_KEY = "hawkr:initial-loader-seen";

interface ListCall {
	signedIn: boolean;
	storeId: string | undefined;
}

async function mockBlockingFeed(page: Page): Promise<ListCall[]> {
	const calls: ListCall[] = [];
	await page.route(
		socialMethod("SocialPublicService", "ListPosts"),
		async (route: Route) => {
			const cookie = (await route.request().allHeaders()).cookie ?? "";
			const signedIn = cookie.includes(SESSION_COOKIE_NAME);
			const storeId = connectRequestMessage(route).storeId as
				| string
				| undefined;
			calls.push({ signedIn, storeId });
			const all = allPosts(storeId);
			const visible = signedIn
				? all.filter((p) => p.store.id !== BLOCKED_STORE.id)
				: all;
			await fulfillConnect(route, { posts: visible, nextPageToken: "" });
		},
	);
	return calls;
}

async function openAsBuyer(
	page: Page,
	url: string,
	registerAfterDefaults: () => Promise<void>,
): Promise<void> {
	await setupGatewayMocks(page, {
		identity: { accountId: "42", roles: ["buyer"] },
		session: "identity",
	});
	// Later routes win in Playwright, so the feed mock must follow the defaults.
	await registerAfterDefaults();
	await seedStoredActiveRole(page, "buyer");
	await page.addInitScript((loaderKey) => {
		sessionStorage.setItem(loaderKey, "1");
	}, LOADER_KEY);
	await gotoStreamed(page, url);
}

test.describe("feed lists depend on the viewer", { tag: ["@ui"] }, () => {
	test("a buyer who blocked a store does not see its posts on /feed", async ({
		page,
	}) => {
		let calls: ListCall[] = [];
		await openAsBuyer(page, "/feed", async () => {
			calls = await mockBlockingFeed(page);
		});

		await expect(page.getByText(OTHER_BODY).first()).toBeVisible();
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		expect(
			calls.some((c) => c.signedIn && c.storeId === undefined),
			"the signed-in list must be read by the browser with the session cookie",
		).toBe(true);
	});

	test("a signed-in buyer's store-scoped list on /feed hides the blocked store", async ({
		page,
	}) => {
		let calls: ListCall[] = [];
		await openAsBuyer(page, `/feed?sellerId=${BLOCKED_STORE.id}`, async () => {
			calls = await mockBlockingFeed(page);
		});

		await expect
			.poll(() =>
				calls.some((c) => c.signedIn && c.storeId === BLOCKED_STORE.id),
			)
			.toBe(true);
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		await expect(page.getByText(OTHER_BODY)).toHaveCount(0);
	});

	test("a guest's store posts tab at /sellers/<store_code>/posts renders the posts server-read under that store's id", async ({
		page,
	}) => {
		const calls = await mockBlockingFeed(page);
		await gotoStreamed(page, `/sellers/${OTHER_STORE.storeCode}/posts`);

		await expect(page.getByText(OTHER_BODY).first()).toBeVisible();
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		expect(
			calls.every((c) => c.storeId !== OTHER_STORE.storeCode),
			"the URL store_code must never be sent as store_id",
		).toBe(true);
	});

	test("a signed-in buyer's store posts tab drops the anonymous seed and reads by the store id, not the URL store_code", async ({
		page,
	}) => {
		let calls: ListCall[] = [];
		await openAsBuyer(
			page,
			`/sellers/${BLOCKED_STORE.storeCode}/posts`,
			async () => {
				calls = await mockBlockingFeed(page);
			},
		);

		await expect
			.poll(() =>
				calls.some((c) => c.signedIn && c.storeId === BLOCKED_STORE.id),
			)
			.toBe(true);
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		expect(
			calls.every((c) => c.storeId !== BLOCKED_STORE.storeCode),
			"the URL store_code must never be sent as store_id",
		).toBe(true);
	});
});
