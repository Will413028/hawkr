"use client";

import { create, type StateCreator } from "zustand";
import {
	createJSONStorage,
	devtools,
	type PersistOptions,
	persist,
} from "zustand/middleware";
import {
	LOCAL_PRIVATE_KEYS,
	type LocalPrivateKeyName,
} from "@/lib/auth/local-private-keys";
import { registerLocalPrivateReset } from "@/lib/auth/local-private-resets";

/**
 * 建立一個綁定帳號身份的持久化 zustand store。
 *
 * 相對於各自手刻 devtools(persist(...))，這裡多做兩件事：
 *   1. storage key 只能來自 local-private-keys registry（未登記者編譯不過）
 *   2. 註冊一支 reset，讓 syncLocalPrivateState 在身份轉換時連記憶體副本一起丟
 *
 * 第 2 點是「只清 localStorage」會做錯的地方：zustand persist 不監聽 storage
 * 事件，賣家人在賣家中心時 store 是掛載狀態，把 localStorage 抽掉之後上一位的
 * 搜尋字串仍在記憶體，下一次 set() 會整包寫回去。
 */
export interface LocalPrivateStoreConfig<S, P> {
	/** registry 名稱，決定 storage key 與所在 storage。 */
	keyName: LocalPrivateKeyName;
	/** devtools 面板顯示名稱。 */
	devtoolsName: string;
	/** 身份轉換時要還原的持久欄位預設值。 */
	defaults: P;
	partialize: (state: S) => P;
	/** persist 的其餘選項（version / migrate / skipHydration）。 */
	persistOptions?: Omit<
		PersistOptions<S, P>,
		"name" | "partialize" | "storage"
	>;
}

type LocalPrivateInitializer<S, P> = StateCreator<
	S,
	[["zustand/devtools", never], ["zustand/persist", P]],
	[]
>;

export function createLocalPrivateStore<S extends object, P extends Partial<S>>(
	initializer: LocalPrivateInitializer<S, P>,
	config: LocalPrivateStoreConfig<S, P>,
) {
	const spec = LOCAL_PRIVATE_KEYS[config.keyName];
	// store 支撐的 key 一律只宣告一個 storage（橫跨兩者的只有結帳草稿，
	// 而它不是 store）。tests/local-private-registry.test.ts 會守住這個前提。
	const area = spec.in[0];
	const store = create<S>()(
		devtools(
			persist(
				initializer as StateCreator<
					S,
					[["zustand/devtools", never], ["zustand/persist", unknown]],
					[]
				>,
				{
					name: spec.key,
					partialize: config.partialize,
					storage: createJSONStorage(() =>
						area === "session" ? sessionStorage : localStorage,
					),
					...config.persistOptions,
				},
			),
			{
				name: config.devtoolsName,
				enabled: process.env.NODE_ENV === "development",
			},
		),
	);
	registerLocalPrivateReset(spec.key, () => {
		store.setState(config.defaults, false);
	});
	return store;
}
