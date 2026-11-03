"use client";

import { useQuery } from "@tanstack/react-query";
import { fetchProfile } from "@/lib/profile/repo";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

/** 登入信箱（買家資料的 `email`）；登入方式沒有信箱，或讀取失敗時是 undefined。與買家資料頁共用快取。 */
export function useLoginEmail(): {
	email: string | undefined;
	loading: boolean;
} {
	const identity = useQueryIdentity();
	const query = useQuery({
		queryKey: queryKeys.buyer.profile(identity),
		queryFn: () => fetchProfile({ suppressAuthExpired: true }),
	});
	return { email: query.data?.email, loading: query.isLoading };
}
