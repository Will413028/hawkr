"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import * as React from "react";
import { ApiClientError } from "@/lib/api/client";
import {
	type AvailableShippingOption,
	type CheckoutItemInput,
	type CheckoutValidationError,
	validateCheckoutItems,
} from "@/lib/checkout/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

interface ShippingOptionsState {
	options: AvailableShippingOption[] | null;
	loading: boolean;
	error: string | null;
	/** Thrown ApiClientError's envelope `code`（例：46 §7.2 的 9241 賣家停權）。
	 * 未知/非 ApiClientError 的失敗維持 undefined，呼叫端不可誤判為某個特定碼。 */
	errorCode?: number;
	validationErrors: CheckoutValidationError[];
	deliveryTempIntersection: string[];
}

const EMPTY_STATE: ShippingOptionsState = {
	options: null,
	loading: false,
	error: null,
	errorCode: undefined,
	validationErrors: [],
	deliveryTempIntersection: [],
};

function delay(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		if (signal?.aborted) {
			reject(signal.reason);
			return;
		}
		const timer = window.setTimeout(resolve, ms);
		signal?.addEventListener(
			"abort",
			() => {
				window.clearTimeout(timer);
				reject(signal.reason);
			},
			{ once: true },
		);
	});
}

/**
 * Debounced wrapper over `/app/checkout/validate`. Returns null `options`
 * when disabled (guest / empty selection / items missing productId) so the
 * caller can fall back to a static method list.
 */
export function useShippingOptions(
	items: CheckoutItemInput[],
	enabled: boolean,
	refreshKey = 0,
): ShippingOptionsState {
	const identity = useQueryIdentity();
	const ready =
		enabled &&
		items.length > 0 &&
		items.every(
			(i) => i.skuId && i.productId != null && Number(i.quantity) > 0,
		);

	const itemSignature = React.useMemo(
		() =>
			ready
				? JSON.stringify(
						items.map((i) => `${i.skuId}|${i.productId}|${i.quantity}`).sort(),
					)
				: "",
		[items, ready],
	);

	const query = useQuery<ShippingOptionsState>({
		queryKey: queryKeys.checkout.shippingOptions(identity, {
			itemSignature,
			refreshKey,
		}),
		queryFn: async ({ signal }): Promise<ShippingOptionsState> => {
			await delay(400, signal);
			const result = await validateCheckoutItems(items);
			return {
				options: result.availableShippingOptions,
				loading: false,
				error: null,
				validationErrors: result.errors,
				deliveryTempIntersection: result.deliveryTempIntersection,
			};
		},
		enabled: ready,
		// keepPreviousData (not initialData / a static placeholder): every qty
		// edit changes `itemSignature` → a new query key. With a static
		// placeholder the data snapped back to EMPTY for the debounce + fetch
		// window, so the per-seller free-shipping meter and the summary's
		// shipping list collapsed and re-expanded on every +/- tap — visible
		// flicker + layout bounce on mobile. keepPreviousData holds the prior
		// options on screen while the new key revalidates (the consumers only
		// show their loading state when `options` is empty, so the previous
		// list stays put). It still triggers the fetch — unlike initialData,
		// which the 30s staleTime would treat as fresh and suppress entirely.
		placeholderData: keepPreviousData,
	});

	if (!ready) return EMPTY_STATE;
	// On error, placeholderData no longer applies and query.data is undefined.
	const data = query.data ?? EMPTY_STATE;
	return {
		...data,
		// Retain the list visually, but never treat a previous store/quantity
		// snapshot as ready for checkout during the debounce or revalidation.
		loading: query.isFetching || query.isPlaceholderData,
		error: query.error instanceof Error ? query.error.message : data.error,
		errorCode:
			query.error instanceof ApiClientError ? query.error.code : data.errorCode,
	};
}
