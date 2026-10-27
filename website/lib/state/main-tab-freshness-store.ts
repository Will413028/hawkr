"use client";

import { create } from "zustand";
import { devtools } from "zustand/middleware";

export type MainTabRefreshStatus = "idle" | "refreshing" | "fresh" | "error";

export interface MainTabRuntimeState {
	status: MainTabRefreshStatus;
	lastSuccessAt: number;
	lastAttemptAt: number;
	requestId: number | null;
}

const EMPTY_RUNTIME: MainTabRuntimeState = {
	status: "idle",
	lastSuccessAt: 0,
	lastAttemptAt: 0,
	requestId: null,
};

type MainTabFreshnessState = {
	scopes: Record<string, MainTabRuntimeState>;
	nextRequestId: number;
	seedFreshness: (scope: string, freshAt: number) => void;
	beginRefresh: (scope: string, attemptedAt: number) => number | null;
	finishRefresh: (
		scope: string,
		requestId: number,
		succeededAt: number,
	) => void;
	failRefresh: (scope: string, requestId: number) => void;
	cancelRefresh: (scope: string, requestId: number) => void;
	removeScopes: (scopes: readonly string[]) => void;
	resetForTests: () => void;
};

function runtimeFor(
	scopes: Record<string, MainTabRuntimeState>,
	scope: string,
) {
	return scopes[scope] ?? EMPTY_RUNTIME;
}

export const useMainTabFreshnessStore = create<MainTabFreshnessState>()(
	devtools(
		(set, get) => ({
			scopes: {},
			nextRequestId: 1,
			seedFreshness: (scope, freshAt) => {
				if (!Number.isFinite(freshAt) || freshAt <= 0) return;
				const current = runtimeFor(get().scopes, scope);
				if (freshAt <= current.lastSuccessAt) return;
				set(
					(state) => ({
						scopes: {
							...state.scopes,
							[scope]: {
								...runtimeFor(state.scopes, scope),
								status:
									current.status === "refreshing" ? "refreshing" : "fresh",
								lastSuccessAt: freshAt,
							},
						},
					}),
					false,
					"main-tab-freshness/seed",
				);
			},
			beginRefresh: (scope, attemptedAt) => {
				const current = runtimeFor(get().scopes, scope);
				if (current.status === "refreshing") return null;
				const requestId = get().nextRequestId;
				set(
					(state) => ({
						nextRequestId: requestId + 1,
						scopes: {
							...state.scopes,
							[scope]: {
								...runtimeFor(state.scopes, scope),
								status: "refreshing",
								lastAttemptAt: attemptedAt,
								requestId,
							},
						},
					}),
					false,
					"main-tab-freshness/begin",
				);
				return requestId;
			},
			finishRefresh: (scope, requestId, succeededAt) => {
				const current = runtimeFor(get().scopes, scope);
				if (current.requestId !== requestId) return;
				set(
					(state) => ({
						scopes: {
							...state.scopes,
							[scope]: {
								...runtimeFor(state.scopes, scope),
								status: "fresh",
								lastSuccessAt: succeededAt,
								requestId: null,
							},
						},
					}),
					false,
					"main-tab-freshness/success",
				);
			},
			failRefresh: (scope, requestId) => {
				const current = runtimeFor(get().scopes, scope);
				if (current.requestId !== requestId) return;
				set(
					(state) => ({
						scopes: {
							...state.scopes,
							[scope]: {
								...runtimeFor(state.scopes, scope),
								status: "error",
								requestId: null,
							},
						},
					}),
					false,
					"main-tab-freshness/error",
				);
			},
			cancelRefresh: (scope, requestId) => {
				const current = runtimeFor(get().scopes, scope);
				if (current.requestId !== requestId) return;
				set(
					(state) => ({
						scopes: {
							...state.scopes,
							[scope]: {
								...runtimeFor(state.scopes, scope),
								status: current.lastSuccessAt > 0 ? "fresh" : "idle",
								requestId: null,
							},
						},
					}),
					false,
					"main-tab-freshness/cancel",
				);
			},
			removeScopes: (removedScopes) => {
				if (!removedScopes.some((scope) => scope in get().scopes)) return;
				set(
					(state) => {
						const scopes = { ...state.scopes };
						for (const scope of removedScopes) delete scopes[scope];
						return { scopes };
					},
					false,
					"main-tab-freshness/remove-scopes",
				);
			},
			resetForTests: () =>
				set(
					{ scopes: {}, nextRequestId: 1 },
					false,
					"main-tab-freshness/reset",
				),
		}),
		{
			name: "main-tab-freshness-store",
			enabled: process.env.NODE_ENV === "development",
		},
	),
);

export function readMainTabRuntime(scope: string): MainTabRuntimeState {
	return runtimeFor(useMainTabFreshnessStore.getState().scopes, scope);
}
