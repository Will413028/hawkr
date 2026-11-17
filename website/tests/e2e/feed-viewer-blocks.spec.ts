import { expect, type Page, type Route, test } from "@playwright/test";
import { SESSION_COOKIE_NAME } from "../../lib/auth/session-cookie";
import {
	connectRequestMessage,
	fulfillConnect,
	seedStoredActiveRole,
	setupGatewayMocks,
	socialMethod,
} from "./helpers/mock-gateway";
import { gotoStreamed } from "./helpers/streamed-navigation";

/**
 * 依觀看者而定的列表（plan A7-4 「依觀看者而定的列表」）：買家封鎖了某家店之後，那家店的貼文不出現在
 * `/feed`，也不出現在以店為範圍的貼文列表。
 *
 * 封鎖由伺服器套用（ListPosts 在分頁之前過濾觀看者封鎖的店），所以這裡證明的是前端的一半：登入者的列表
 * 由瀏覽器帶 session cookie 重新讀，而不是用匿名的種子或匿名的共用快取答案。mock 以請求有沒有帶 session
 * cookie 決定回應：帶了才是「過濾後」的答案，所以只有真的以登入者讀取才會看不到被封鎖的店。
 *
 * 店家貼文分頁（`/sellers/{id}/posts`）的第一頁由伺服器端的 REST 讀店家資料，e2e 的 webServer 沒有後端可 mock，
 * 所以以店為範圍的列表用同一個 hook 的 `/feed?sellerId=` 驗；分頁路由本身的種子行為由
 * `useFeedPosts.viewer.unit.test.ts` 與 `FeedPage.unit.test.ts` 守。
 */

const BLOCKED_STORE = "7001";
const OTHER_STORE = "7002";
const BLOCKED_BODY = "被封鎖店家的貼文內容";
const OTHER_BODY = "其他店家的貼文內容";
const LOADER_KEY = "hawkr:initial-loader-seen";

function post(id: string, storeId: string, body: string) {
	return {
		id,
		store: { id: storeId, storeCode: storeId, name: `店 ${storeId}` },
		body,
		publishTime: "2026-11-01T00:00:00Z",
		likeCount: "0",
		commentCount: "0",
		viewCount: "1",
		shareCount: "0",
	};
}

const BLOCKED_POST = post(
	"5f0c9a52-7a4e-4f0b-8d2e-3b7c2a1d9e11",
	BLOCKED_STORE,
	BLOCKED_BODY,
);
const OTHER_POST = post(
	"5f0c9a52-7a4e-4f0b-8d2e-3b7c2a1d9e12",
	OTHER_STORE,
	OTHER_BODY,
);

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
			const all = [BLOCKED_POST, OTHER_POST].filter(
				(p) => storeId === undefined || p.store.id === storeId,
			);
			const visible = signedIn
				? all.filter((p) => p.store.id !== BLOCKED_STORE)
				: all;
			await fulfillConnect(route, { posts: visible, nextPageToken: "" });
		},
	);
	return calls;
}

async function openAsBuyer(page: Page, url: string): Promise<void> {
	await setupGatewayMocks(page, {
		identity: { accountId: "42", roles: ["buyer"] },
		session: "identity",
	});
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
		const calls = await mockBlockingFeed(page);
		await openAsBuyer(page, "/feed");

		await expect(page.getByText(OTHER_BODY).first()).toBeVisible();
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		expect(
			calls.some((c) => c.signedIn && c.storeId === undefined),
			"the signed-in list must be read by the browser with the session cookie",
		).toBe(true);
	});

	test("the same buyer does not see the blocked store's posts in a store-scoped list either", async ({
		page,
	}) => {
		const calls = await mockBlockingFeed(page);
		await openAsBuyer(page, `/feed?sellerId=${BLOCKED_STORE}`);

		await expect
			.poll(() => calls.some((c) => c.signedIn && c.storeId === BLOCKED_STORE))
			.toBe(true);
		await expect(page.getByText(BLOCKED_BODY)).toHaveCount(0);
		await expect(page.getByText(OTHER_BODY)).toHaveCount(0);
	});
});
