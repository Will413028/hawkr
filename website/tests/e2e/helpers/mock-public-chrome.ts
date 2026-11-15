import type { Page, Route } from "@playwright/test";

/**
 * Wire shapes of the four reads the guest chrome client-fetches. Source of
 * truth: `backend/api/openapi/openapi.yaml` (`/api/v1/public/categories/tree`
 * → `Category[]`, `/public/hashtags/popular` → `GetPopularHashtagsResponse`,
 * `/public/tags/trending` → `TrendingPostTagsResponse`, `/public/posts` →
 * `PostListItemData`). Envelope is `{ code, message, data }`; int64 counts are
 * strings.
 */
const CATEGORY_TREE = [
	{
		id: "1",
		name: "蔬菜",
		parentId: "0",
		status: "ACTIVE",
		children: [
			{
				id: "11",
				name: "葉菜類",
				parentId: "1",
				parentName: "蔬菜",
				status: "ACTIVE",
				children: [
					{
						id: "111",
						name: "青江菜",
						parentId: "11",
						parentName: "葉菜類",
						status: "ACTIVE",
						children: [],
					},
				],
			},
		],
	},
	{
		id: "2",
		name: "水果",
		parentId: "0",
		status: "ACTIVE",
		children: [
			{
				id: "21",
				name: "柑橘類",
				parentId: "2",
				parentName: "水果",
				status: "ACTIVE",
				children: [
					{
						id: "211",
						name: "柳丁",
						parentId: "21",
						parentName: "柑橘類",
						status: "ACTIVE",
						children: [],
					},
				],
			},
		],
	},
];

const POPULAR_HASHTAGS = {
	records: [
		{ hashtag: "有機", count: "12" },
		{ hashtag: "產地直送", count: "8" },
	],
};

const TRENDING_TAGS = {
	items: [
		{ hashtag: "當季水果", count: "9" },
		{ hashtag: "產地直送", count: "5" },
	],
};

const LATEST_POSTS = {
	page: "1",
	pageSize: "20",
	total: "1",
	records: [
		{
			id: "9001",
			sellerId: "501",
			sellerName: "阿福農場",
			storeCode: "afu-farm",
			content: "本週柳丁開始採收。",
			tags: ["當季水果"],
			createdAt: 1760000000000,
			likeCount: 3,
			commentCount: 1,
			shareCount: 0,
			viewCount: 40,
			isLiked: false,
			imageKeys: [],
			imageUrls: [],
			logoKey: "",
			logoUrl: "",
		},
	],
};

function ok(data: unknown) {
	return (route: Route) =>
		route.fulfill({
			status: 200,
			contentType: "application/json",
			body: JSON.stringify({ code: 0, message: "", data }),
		});
}

const PUBLIC_CHROME_READS: Array<[string, unknown]> = [
	["/api/v1/public/categories/tree", CATEGORY_TREE],
	["/api/v1/public/hashtags/popular", POPULAR_HASHTAGS],
	["/api/v1/public/tags/trending", TRENDING_TAGS],
	["/api/v1/public/posts", LATEST_POSTS],
];

/**
 * Mocks the four guest-chrome reads and turns every other same-origin
 * `/api/**` request into a recorded 404, so nothing reaches a real backend and
 * a missed endpoint fails the spec (the returned array must stay empty).
 * Call before other `page.route` registrations: the last registered handler
 * wins, so this catch-all must be the lowest priority.
 */
export async function installPublicChromeMocks(page: Page): Promise<string[]> {
	const unmocked: string[] = [];
	await page.route(
		(url) => url.pathname.startsWith("/api/"),
		(route) => {
			const { pathname } = new URL(route.request().url());
			unmocked.push(`${route.request().method()} ${pathname}`);
			return route.fulfill({
				status: 404,
				contentType: "application/json",
				body: JSON.stringify({ code: 404, message: "unmocked in e2e" }),
			});
		},
	);
	for (const [pathname, data] of PUBLIC_CHROME_READS) {
		await page.route(
			(url) => url.pathname === pathname,
			(route) => {
				if (route.request().method() !== "GET") return route.fallback();
				return ok(data)(route);
			},
		);
	}
	return unmocked;
}
