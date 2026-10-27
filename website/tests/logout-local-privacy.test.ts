import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { afterEach, beforeEach, test } from "node:test";

/**
 * hawkr-website#382：稍後購買清單與結帳草稿存在瀏覽器本機、沒有帳號綁定、
 * 任何登出路徑都不清除，於是同一台裝置換人登入會看到上一位使用者的商品，以及
 * （購物車 SKU 有重疊時）他的收件人姓名 / 電話 / 地址。
 *
 * 這裡驗的是「換一個擁有者之後，前一位的資料讀不出來」這個不變式，涵蓋登出、
 * 未登出就換人登入兩條路徑，並反向確認同一個人不會被誤清（修隱私不能把功能弄壞）。
 */

interface Store {
	store: Record<string, string>;
	getItem(k: string): string | null;
	setItem(k: string, v: string): void;
	removeItem(k: string): void;
}

function makeStorage(): Store {
	const store: Record<string, string> = {};
	return {
		store,
		getItem: (k) => (k in store ? store[k] : null),
		setItem: (k, v) => {
			store[k] = String(v);
		},
		removeItem: (k) => {
			delete store[k];
		},
	};
}

let ls = makeStorage();
let ss = makeStorage();
let dispatched: string[] = [];

class FakeStorageEvent {
	key: string | null;
	newValue: string | null;
	oldValue: string | null;
	constructor(
		_type: string,
		init: { key?: string; newValue?: string | null; oldValue?: string | null },
	) {
		this.key = init.key ?? null;
		this.newValue = init.newValue ?? null;
		this.oldValue = init.oldValue ?? null;
	}
}

beforeEach(() => {
	ls = makeStorage();
	ss = makeStorage();
	dispatched = [];
	const g = globalThis as Record<string, unknown>;
	g.localStorage = ls;
	g.sessionStorage = ss;
	g.StorageEvent = FakeStorageEvent;
	g.window = {
		localStorage: ls,
		sessionStorage: ss,
		dispatchEvent: (e: FakeStorageEvent) => {
			if (e.key) dispatched.push(e.key);
			return true;
		},
	};
});

afterEach(() => {
	const g = globalThis as Record<string, unknown>;
	g.localStorage = undefined;
	g.sessionStorage = undefined;
	g.StorageEvent = undefined;
	g.window = undefined;
});

const SAVED_KEY = "hawkr:cart:saved";
const DRAFT_KEY = "hawkr:checkout:draft";
const DRAFT_PII_KEY = "hawkr:checkout:draft-pii";

async function seedBuyerData(ownerId: string) {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { saveItem, getSavedItems } = await import("../lib/cart/saved");
	const { writeCheckoutDraft, readCheckoutDraft } = await import(
		"../lib/checkout/draft"
	);
	// 買家登入 → 本機私有狀態的擁有者換成他。
	syncLocalPrivateState(ownerId);
	saveItem({
		id: "sku-1",
		skuId: "sku-1",
		name: "有機小松菜",
		price: "NT$120",
		priceValue: 120,
		quantity: 1,
	});
	writeCheckoutDraft({
		version: 2,
		createdAt: 1,
		validateAt: 1,
		seller: { key: "s1", name: "陳記" },
		selectedSkuIds: ["sku-1"],
		shipping: { method: "home", deliveryTemp: "normal", label: "宅配", fee: 0 },
		recipient: { name: "王小明", phone: "0912000000", line: "台北市…" },
		note: "",
	});
	// 前提自檢：資料真的寫進去了，否則後面的「讀不到」是假通過。
	assert.equal(getSavedItems().length, 1, "seed: 稍後購買應有 1 筆");
	assert.equal(
		readCheckoutDraft()?.recipient?.phone,
		"0912000000",
		"seed: 草稿應含收件人電話",
	);
	return { getSavedItems, readCheckoutDraft, syncLocalPrivateState };
}

test("登出後讀不到上一位使用者的稍後購買與結帳草稿", async () => {
	const { getSavedItems, readCheckoutDraft, syncLocalPrivateState } =
		await seedBuyerData("1001");

	// 登出：AuthProvider 的 identity 事件帶著 null 擁有者進來。
	syncLocalPrivateState(null);

	assert.deepEqual(getSavedItems(), [], "登出後不該讀得到稍後購買清單");
	assert.equal(readCheckoutDraft(), null, "登出後不該讀得到結帳草稿");
	assert.equal(SAVED_KEY in ls.store, false, "稍後購買 key 應整個移除");
	assert.equal(DRAFT_KEY in ls.store, false, "草稿 key 應整個移除");
	assert.equal(DRAFT_PII_KEY in ss.store, false, "收件人 PII 應整個移除");
	assert.equal(
		JSON.stringify(ls.store).includes("0912000000"),
		false,
		"localStorage 不該殘留任何電話字串",
	);
});

test("沒有登出、同一裝置直接換另一個人登入，也讀不到前一位的資料", async () => {
	const { getSavedItems, readCheckoutDraft, syncLocalPrivateState } =
		await seedBuyerData("1001");

	// 關掉瀏覽器 → 沒有任何登出流程跑過，資料原封不動留在裝置上；
	// 下一位使用者登入是第一個能察覺的時機。
	syncLocalPrivateState("2002");

	assert.deepEqual(getSavedItems(), [], "換人登入後不該讀得到前一位的清單");
	assert.equal(readCheckoutDraft(), null, "換人登入後不該讀得到前一位的草稿");
	assert.equal(DRAFT_PII_KEY in ss.store, false, "前一位的收件人 PII 應消失");
});

test("同一個人再次登入 / 同帳號 identity 事件不會清掉自己的資料", async () => {
	const { getSavedItems, readCheckoutDraft, syncLocalPrivateState } =
		await seedBuyerData("1001");

	// cold load 的 session probe、以及 nickname/avatar 補值都會再送一次 identity
	// 事件（同一個 accountId）——這裡若誤清，等於為了修隱私把功能弄壞。
	syncLocalPrivateState("1001");
	syncLocalPrivateState("1001");

	assert.equal(getSavedItems().length, 1, "同帳號不該被清空");
	assert.equal(readCheckoutDraft()?.recipient?.phone, "0912000000");
});

test("清除時會通知已掛載的畫面，避免記憶體裡還留著上一位的副本", async () => {
	const { syncLocalPrivateState } = await seedBuyerData("1001");
	dispatched = [];

	syncLocalPrivateState("2002");

	// CartClient / CheckoutClient 都只在 mount 時讀一次，且那次讀取早於 cookie
	// probe 回答「你是誰」；不沿用它們既有的 storage 通道通知，畫面上會留著已被
	// 清掉的資料。
	assert.ok(dispatched.includes(SAVED_KEY), "應通知稍後購買清單已清除");
	assert.ok(dispatched.includes(DRAFT_KEY), "應通知結帳草稿已清除");
});

test("AuthProvider 在每次 identity 變動都呼叫本機私有狀態同步", async () => {
	const source = readFileSync(
		new URL("../lib/auth/context.tsx", import.meta.url),
		"utf8",
	);
	assert.match(
		source,
		/syncLocalPrivateState\(nextAccountId \?\? null\)/,
		"onIdentityChange 必須帶著新擁有者呼叫同步（登出時為 null）",
	);
	// 必須早於 shouldTeardownPrivateCache 的分支：後者刻意跳過「首次 identity
	// landing」，而那正是裝置上還留著前一位資料的情境。
	assert.ok(
		source.indexOf("syncLocalPrivateState(nextAccountId ?? null)") <
			source.indexOf("if (shouldTeardownPrivateCache("),
		"同步必須無條件執行，不能被 shouldTeardownPrivateCache 的條件擋掉",
	);
});

const MARKET_SNAPSHOT_KEY = "hk-market-list-snapshot";
const FEED_SNAPSHOT_KEY = "hk-feed-list-snapshot";

test("換帳號會清掉商城 / Feed 的返回列表快照", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	syncLocalPrivateState("1001");
	ss.setItem(
		MARKET_SNAPSHOT_KEY,
		JSON.stringify({ key: "/market", products: [{ id: "p1" }], page: 1 }),
	);
	ss.setItem(
		FEED_SNAPSHOT_KEY,
		JSON.stringify({ key: "/feed", posts: [{ id: "f1" }], page: 1 }),
	);
	// 前提自檢：快照真的寫進去了，否則後面的「讀不到」是假通過。
	assert.ok(ss.getItem(MARKET_SNAPSHOT_KEY), "seed: 商城快照應存在");
	assert.ok(ss.getItem(FEED_SNAPSHOT_KEY), "seed: Feed 快照應存在");

	syncLocalPrivateState("2002");

	assert.equal(
		ss.getItem(MARKET_SNAPSHOT_KEY),
		null,
		"換帳號後不該讀得到上一位的商城快照（含他的收藏 / 按讚狀態）",
	);
	assert.equal(
		ss.getItem(FEED_SNAPSHOT_KEY),
		null,
		"換帳號後不該讀得到上一位的 Feed 快照",
	);
});

test("同一個人再次登入不會清掉自己的列表快照", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	syncLocalPrivateState("1001");
	ss.setItem(MARKET_SNAPSHOT_KEY, JSON.stringify({ key: "/market" }));

	syncLocalPrivateState("1001");

	assert.ok(
		ss.getItem(MARKET_SNAPSHOT_KEY),
		"同帳號 identity 事件不該打斷返回列表的還原",
	);
});

test("結帳草稿的 sessionStorage PII 也在清除範圍內", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	syncLocalPrivateState("1001");
	ss.setItem(DRAFT_PII_KEY, JSON.stringify({ name: "王小明" }));

	syncLocalPrivateState("2002");

	assert.equal(ss.getItem(DRAFT_PII_KEY), null, "收件人 PII 不該留給下一位");
});

test("換帳號會清掉已掛載賣家 store 的記憶體副本，不只是 localStorage", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { useSellerCustomersUiStore } = await import(
		"../features/seller-center/customers/customersUiStore"
	);
	syncLocalPrivateState("1001");
	useSellerCustomersUiStore.getState().setSearch("上一位賣家的客戶關鍵字");
	useSellerCustomersUiStore.getState().setSortBy("order_count");
	// 前提自檢：值真的進了記憶體與 localStorage。
	assert.equal(
		useSellerCustomersUiStore.getState().search,
		"上一位賣家的客戶關鍵字",
		"seed: 搜尋字串應在記憶體",
	);
	assert.ok(
		ls.getItem("hawkr:seller-customers:ui"),
		"seed: 搜尋字串應已持久化",
	);

	syncLocalPrivateState("2002");

	assert.equal(
		ls.getItem("hawkr:seller-customers:ui"),
		null,
		"換帳號後 localStorage 不該留著上一位的篩選條件",
	);
	// zustand persist 不監聽 storage 事件：只清 localStorage 的話，記憶體裡
	// 還留著上一位的字串，下一次 set() 會把它整包寫回去，洩漏原地復活。
	assert.equal(
		useSellerCustomersUiStore.getState().search,
		"",
		"換帳號後記憶體副本必須回到預設值",
	);
	assert.equal(
		useSellerCustomersUiStore.getState().sortBy,
		"total_spent",
		"換帳號後排序必須回到預設值",
	);
});

test("其中一把 key 的 reset 失敗時，registry 順序在它後面的 key 仍會被清除，syncLocalPrivateState 不拋出例外", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { registerLocalPrivateReset } = await import(
		"../lib/auth/local-private-resets"
	);

	// 模擬某支 store 的 reset 因隱私模式 / quota exceeded 而 throw——實際成因
	// 是 store.setState → persist 的 setItem → storage.setItem 本來就可能拋
	// 例外，跟 storageFor() 在防的是同一類失敗。sellerProductsUi 目前沒有真的
	// zustand store 掛載，這裡直接註冊一支會 throw 的 reset 來重現同一種失敗
	// 形狀，不需要真的把某個 store 的 localStorage 逼到 quota exceeded。
	registerLocalPrivateReset("hawkr:seller-products:ui", () => {
		throw new Error("模擬隱私模式 / quota exceeded");
	});

	syncLocalPrivateState("1001");
	const LATER_KEY = "hawkr:seller-dashboard:ui";
	ls.setItem(LATER_KEY, JSON.stringify({ any: "data" }));
	// 前提自檢：排在失敗 key 後面的 key 真的有資料。
	assert.ok(ls.getItem(LATER_KEY), "seed: 排在後面的 key 應存在");

	assert.doesNotThrow(() => {
		syncLocalPrivateState("2002");
	}, "單一 key 的 reset 失敗不該讓 syncLocalPrivateState 拋出例外");

	assert.equal(
		ls.getItem(LATER_KEY),
		null,
		"registry 順序在失敗 key 後面的 key 仍應被清除，不能被中止的迴圈漏掉",
	);
});

test("換帳號會清掉購物車 UI state 的備註 / 優惠券 / 收件地址（含記憶體副本）", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { useCartUiStore } = await import("../features/cart/cartUiStore");
	syncLocalPrivateState("1001");
	// note 是自由文字欄位，實務上會出現「放管理室、按 3 樓電鈴」這類門牌代收
	// 指示——這把 key 是 #382 修復時漏掉的第三把同類 key，PII 等級與結帳草稿同級。
	useCartUiStore.getState().setNote("放管理室，按 3 樓電鈴找王小明");
	useCartUiStore.getState().setSelectedAddressId("addr-9527");
	useCartUiStore.getState().setCoupon({
		templateId: "t1",
		instanceId: "coupon-instance-only-this-buyer-holds",
		title: "滿 200 折 50",
		kind: "shop",
		estimatedDiscount: 50,
		storeId: "s1",
	});
	// 前提自檢：值真的進了記憶體與 localStorage。
	assert.equal(useCartUiStore.getState().note, "放管理室，按 3 樓電鈴找王小明");
	assert.ok(ls.getItem("hawkr:cart:ui"), "seed: 購物車 UI state 應已持久化");

	syncLocalPrivateState("2002");

	assert.equal(
		ls.getItem("hawkr:cart:ui"),
		null,
		"換帳號後 localStorage 不該留著上一位的購物車 UI state",
	);
	assert.equal(
		JSON.stringify(ls.store).includes("王小明"),
		false,
		"localStorage 不該殘留備註裡的姓名",
	);
	assert.equal(
		useCartUiStore.getState().note,
		"",
		"換帳號後備註的記憶體副本必須回到預設值",
	);
	assert.equal(
		useCartUiStore.getState().coupon,
		null,
		"換帳號後不該把上一位持有的券帶進新身份的試算",
	);
	assert.equal(
		useCartUiStore.getState().selectedAddressId,
		null,
		"換帳號後不該沿用上一位選定的收件地址",
	);
});

test("換帳號會清掉賣家訂單操作鎖（sessionStorage 背後的 store 也走同一條清除路徑）", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { useSellerShipmentOperationStore } = await import(
		"../features/seller-center/orders/shipmentOperationStore"
	);
	syncLocalPrivateState("1001");
	useSellerShipmentOperationStore.getState().startTcatReservation("O317257731");
	assert.ok(
		useSellerShipmentOperationStore.getState().operations.O317257731,
		"seed: 操作鎖應存在",
	);

	syncLocalPrivateState("2002");

	assert.deepEqual(
		useSellerShipmentOperationStore.getState().operations,
		{},
		"換帳號後不該讓下一位賣家看到上一位訂單編號被鎖住",
	);
	assert.equal(
		ss.getItem("hawkr:seller-shipment-operations"),
		null,
		"sessionStorage 也應清乾淨",
	);
});

/**
 * hawkr agent 搜尋 handoff 是 `clearOn: "leaving-a-signed-in-owner"` 的唯一
 * 實例，三條測試把它的三種轉換都釘住。少任何一條都會讓下一個人只看到半個需求：
 * 只驗「跨得過登入」會讓人以為它永不清除（那是 2026-08-07 第一版止血的錯誤），
 * 只驗「登出要清」則會再次打掉首頁帶入草稿的功能。
 */
test("handoff 跨得過「訪客 → 登入」（沒有這條，首頁帶入草稿的功能會被清掉）", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { writeHawkrAgentHandoff, consumeHawkrAgentHandoff } = await import(
		"../features/procurement-assistant/handoff"
	);

	writeHawkrAgentHandoff("台南水果禮盒 50 盒", "home-search");
	assert.ok(
		ss.getItem("hawkr:hawkr-agent:search-handoff:v1"),
		"seed: 草稿應已寫入 sessionStorage",
	);

	// 訪客寫入（owner marker 仍是 null）→ 登入：handoff 唯一要跨越的轉換。
	syncLocalPrivateState("1001");

	assert.equal(
		consumeHawkrAgentHandoff()?.query,
		"台南水果禮盒 50 盒",
		"登入後草稿必須還在，否則 Agent 工作區會開出空白 composer",
	);
});

test("handoff 跨不過登出（A 登入中寫的草稿不留給下一個人）", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { writeHawkrAgentHandoff, consumeHawkrAgentHandoff } = await import(
		"../features/procurement-assistant/handoff"
	);

	// 關鍵前提：先讓 A 成為擁有者，草稿才是「已登入帳號寫的」。
	syncLocalPrivateState("1001");
	writeHawkrAgentHandoff("A 登入後打的搜尋字串", "search-page");

	syncLocalPrivateState(null);

	assert.equal(
		ss.getItem("hawkr:hawkr-agent:search-handoff:v1"),
		null,
		"登出後 sessionStorage 不該留著 A 的搜尋字串",
	);
	// storage 之外還有 module 級記憶體副本，沒有 reset 註冊的話 consume() 仍會吐出來。
	assert.equal(
		consumeHawkrAgentHandoff(),
		null,
		"登出後記憶體 fallback 也必須清掉",
	);
});

test("handoff 跨不過換帳號（A 沒登出，B 直接在同裝置登入）", async () => {
	const { syncLocalPrivateState } = await import(
		"../lib/auth/local-private-state"
	);
	const { writeHawkrAgentHandoff, consumeHawkrAgentHandoff } = await import(
		"../features/procurement-assistant/handoff"
	);

	syncLocalPrivateState("1001");
	writeHawkrAgentHandoff("A 沒消費完的搜尋字串", "home-search");

	syncLocalPrivateState("2002");

	assert.equal(
		consumeHawkrAgentHandoff(),
		null,
		"換帳號後不該把 A 的搜尋字串帶進 B 的 AI 助手",
	);
});
