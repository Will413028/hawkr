"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { queryKeys } from "@/lib/query/keys";
import { pushHistory } from "@/lib/visits/repo";
import { useQueryIdentity } from "./useQueryIdentity";

/**
 * Records a product visit into the logged-in buyer's browse history
 * (BuyerAppService.RecordVisit). The product-view beacon
 * (SellerPublicService.RecordProductView) only feeds aggregate counts — this
 * is what populates the 瀏覽紀錄 page.
 *
 * `enabled` mirrors the buyer-query gate (`hydrated && isLoggedIn`) so callers
 * can skip recording for guests. On success the history query is invalidated so
 * an already-mounted 瀏覽紀錄 page reflects the new visit.
 */
export function useRecordHistory(): {
	enabled: boolean;
	record: (productId: string) => void;
} {
	const { hydrated, activeRole, isLoggedIn } = useAuth();
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const mutation = useMutation({
		mutationFn: (productId: string) => pushHistory(productId),
		onSuccess: () => {
			void queryClient.invalidateQueries({
				queryKey: queryKeys.buyer.history(identity, {
					activeRole: activeRole ?? "",
				}),
			});
		},
	});
	return { enabled: hydrated && isLoggedIn, record: mutation.mutate };
}
