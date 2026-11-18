"use client";

import { type MutationKey, useMutationState } from "@tanstack/react-query";
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
