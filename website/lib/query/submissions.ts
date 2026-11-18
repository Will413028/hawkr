"use client";

import {
	type MutationKey,
	useMutationState,
	useQueryClient,
} from "@tanstack/react-query";
import * as React from "react";

export interface Submission<V, D> {
	variables: V;
	data: D | undefined;
	/** 毫秒；只給顯示相對時間用。 */
	submittedAt: number;
}

/** 這個 mutation key 底下進行中或已成功的送出；失敗的已經回滾，不在這裡。 */
export function useSubmissions<V, D>(
	mutationKey: MutationKey,
): Submission<V, D>[] {
	const states = useMutationState({
		filters: {
			mutationKey,
			predicate: (mutation) =>
				mutation.state.status === "pending" ||
				mutation.state.status === "success",
		},
		select: (mutation) => mutation.state,
	});
	return React.useMemo(
		() =>
			states.map((state) => ({
				variables: state.variables as V,
				data: state.data as D | undefined,
				submittedAt: state.submittedAt,
			})),
		[states],
	);
}

export interface SubmittedItem<T> {
	/** 從送出中到成功都不變，當 React key 用。 */
	key: string;
	item: T;
}

interface SubmittableItem {
	id: string;
	optimistic?: boolean;
}

/**
 * 自己剛送出、伺服器那份還不在已載入頁的項目（沒有操作按鈕，`optimistic: true`）。
 * 伺服器那份一進到已載入的頁，這筆送出就從 MutationCache 移除：之後那一則被刪除時
 * 不會再從成功的 mutation 重新出現。
 */
export function useSubmittedItems<
	V extends { requestId: string },
	T extends SubmittableItem,
>(options: {
	mutationKey: MutationKey;
	loaded: readonly T[];
	accepts?: (v: V) => boolean;
	build: (v: V, createdAt: number) => T;
	/** 送出成功後把伺服器回的那份補上只有送出時才知道的欄位（例如被回覆者名稱）。 */
	landed?: (data: T, v: V) => T;
}): SubmittedItem<T>[] {
	const { mutationKey, loaded, accepts, build, landed } = options;
	const queryClient = useQueryClient();
	const submissions = useSubmissions<V, T>(mutationKey);

	const landedIds = React.useMemo(() => {
		const loadedIds = new Set(loaded.map((c) => c.id));
		return submissions
			.filter((s) => s.data && loadedIds.has(s.data.id))
			.map((s) => s.variables.requestId);
	}, [submissions, loaded]);

	React.useEffect(() => {
		if (landedIds.length === 0) return;
		const cache = queryClient.getMutationCache();
		for (const mutation of cache.findAll({
			mutationKey,
			predicate: (m) =>
				m.state.status === "success" &&
				landedIds.includes((m.state.variables as V).requestId),
		})) {
			cache.remove(mutation);
		}
	}, [landedIds, mutationKey, queryClient]);

	return React.useMemo(() => {
		const loadedIds = new Set(loaded.map((c) => c.id));
		return submissions.flatMap((s) => {
			if (accepts && !accepts(s.variables)) return [];
			const key = `tmp-${s.variables.requestId}`;
			if (loadedIds.has(key) || (s.data && loadedIds.has(s.data.id))) return [];
			const item = s.data
				? {
						...(landed ? landed(s.data, s.variables) : s.data),
						optimistic: true,
					}
				: build(s.variables, Math.floor(s.submittedAt / 1000));
			return [{ key, item }];
		});
	}, [submissions, loaded, accepts, build, landed]);
}
