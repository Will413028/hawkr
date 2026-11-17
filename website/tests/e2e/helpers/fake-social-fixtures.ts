/**
 * The anonymous answers the e2e webServer's upstream gives for the server-rendered reads of the social lists:
 * ListPosts (the first page of /feed and of a store's posts tab) and the public seller lookup that tab resolves its
 * store_id from. Browser-side reads are mocked per test with `page.route`; these are what SSR sees, so a spec can
 * tell a server seed from a browser read.
 */

export interface FixtureStore {
	id: string;
	storeCode: string;
	name: string;
}

export const BLOCKED_STORE: FixtureStore = {
	id: "7001",
	storeCode: "2611000001",
	name: "店 7001",
};
export const OTHER_STORE: FixtureStore = {
	id: "7002",
	storeCode: "2611000002",
	name: "店 7002",
};
export const FIXTURE_STORES: readonly FixtureStore[] = [
	BLOCKED_STORE,
	OTHER_STORE,
];

export const BLOCKED_BODY = "被封鎖店家的貼文內容";
export const OTHER_BODY = "其他店家的貼文內容";

export function fixturePost(id: string, store: FixtureStore, body: string) {
	return {
		id,
		store: { id: store.id, storeCode: store.storeCode, name: store.name },
		body,
		publishTime: "2026-11-01T00:00:00Z",
		likeCount: "0",
		commentCount: "0",
		viewCount: "1",
		shareCount: "0",
	};
}

export const BLOCKED_POST = fixturePost(
	"5f0c9a52-7a4e-4f0b-8d2e-3b7c2a1d9e11",
	BLOCKED_STORE,
	BLOCKED_BODY,
);
export const OTHER_POST = fixturePost(
	"5f0c9a52-7a4e-4f0b-8d2e-3b7c2a1d9e12",
	OTHER_STORE,
	OTHER_BODY,
);

/** The anonymous posts of one fixture store; null for any other request, which the upstream refuses like an unreachable one. */
export function anonymousStorePosts(storeId: string | undefined) {
	if (!FIXTURE_STORES.some((s) => s.id === storeId)) return null;
	return [BLOCKED_POST, OTHER_POST].filter((p) => p.store.id === storeId);
}

/** The browser-side mock's view: every fixture post, or one store's. */
export function allPosts(storeId: string | undefined) {
	return [BLOCKED_POST, OTHER_POST].filter(
		(p) => storeId === undefined || p.store.id === storeId,
	);
}

export const LIST_POSTS_PATH =
	"/hawkr.social.v1alpha1.SocialPublicService/ListPosts";

const SELLER_PATH = /\/public\/sellers\/([^/]+)(\/reviews\/stats|\/stats)?$/;

/**
 * REST answers of the public seller endpoints for the fixture stores, keyed by store_code or store_id; null for any
 * other path or seller, which the upstream refuses like an unreachable one, so no other spec's server-side reads change.
 */
export function sellerRestAnswer(
	pathname: string,
): { status: number; body: unknown } | null {
	const match = SELLER_PATH.exec(pathname);
	if (!match) return null;
	const [, key, suffix] = match;
	const store = FIXTURE_STORES.find((s) => s.id === key || s.storeCode === key);
	if (!store) return null;
	if (suffix === "/reviews/stats") {
		return {
			status: 200,
			body: { code: 0, message: "ok", data: { avgRating: 0, totalCount: 0 } },
		};
	}
	if (suffix === "/stats") {
		return {
			status: 200,
			body: { code: 0, message: "ok", data: { followers: 0 } },
		};
	}
	return {
		status: 200,
		body: {
			code: 0,
			message: "ok",
			data: { storeId: store.id, storeCode: store.storeCode, name: store.name },
		},
	};
}
