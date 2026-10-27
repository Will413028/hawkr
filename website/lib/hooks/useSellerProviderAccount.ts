/**
 * AMEGO 憑證帳號（58 §6.1 端點 23–27，58A §S03）。
 *
 * 三層形狀照 `useSellerInvoiceRemedy.ts`：repo（唯一 import 傳輸層）→ 本檔 hook →
 * `features/seller-center/invoices/amego/**`。
 *
 * 🔴 **`9330` 不是錯誤態**：它代表「這個賣家還沒開始串接」，是每個新賣家的必經狀態。
 * 當成錯誤處理會把 S03 的入口變成一片紅字，而那正是他要進來設定的地方。
 * ⇒ hook 把它折成 `notStarted`，並用 `meta.isExpectedError` 擋掉 Sentry 上報。
 *
 * 🔴 **capability 是 `CapInvoiceManage`（停權 BLOCK）**：與 S02（`CapInvoiceTaxRemedy`，
 * 停權放行）不同族，所以 query key 也刻意分開——見 `lib/query/keys.ts` 的註解。
 */
"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuth } from "@/lib/auth/context";
import { isInvoiceBizCode } from "@/lib/invoices/errorCopy";
import {
	activateSellerProviderAccount,
	fetchSellerProviderAccount,
	putSellerProviderCredentials,
	rotateSellerProviderCredentials,
	SELLER_INVOICE_BIZ_CODE,
	verifySellerProviderAccount,
} from "@/lib/invoices/seller-repo";
import type { SellerProviderAccount } from "@/lib/invoices/seller-types";
import { queryKeys } from "@/lib/query/keys";
import { useQueryIdentity } from "./useQueryIdentity";

function isAccountMissing(error: unknown): boolean {
	return isInvoiceBizCode(
		error,
		SELLER_INVOICE_BIZ_CODE.PROVIDER_ACCOUNT_NOT_FOUND,
	);
}

export interface UseSellerProviderAccountResult {
	account: SellerProviderAccount | null;
	/** 後端 404 ＋ `9330` ⇒ 還沒開始串接。**不是錯誤**。 */
	notStarted: boolean;
	loading: boolean;
	/** 真正的失敗（5xx／網路）。`9330` 不走這裡。 */
	error: unknown;
	refresh: () => Promise<void>;
}

/** 端點 23（S03 唯一資料源）。 */
export function useSellerProviderAccount(): UseSellerProviderAccountResult {
	const { isLoggedIn, activeRole } = useAuth();
	const identity = useQueryIdentity();
	const enabled = isLoggedIn && activeRole === "seller";

	const query = useQuery({
		queryKey: queryKeys.seller.invoiceProviderAccount(identity),
		queryFn: fetchSellerProviderAccount,
		enabled,
		meta: { isExpectedError: isAccountMissing },
	});

	const notStarted = query.error != null && isAccountMissing(query.error);

	return {
		account: enabled ? (query.data ?? null) : null,
		notStarted,
		loading: enabled && query.isLoading,
		error: notStarted ? null : query.error,
		refresh: async () => {
			await query.refetch();
		},
	};
}

/**
 * `expectedVersion` 的預設值。
 *
 * 🔴 `notStarted`（還沒有列）時後端期待的是 **`"0"`**，不是空字串——樂觀鎖比的是
 * 數值。傳空字串會在 `strconv` 那層變成格式錯，而錯誤訊息會指向一個賣家看不懂的地方。
 */
export function providerExpectedVersion(
	account: SellerProviderAccount | null,
): string {
	return account?.version || "0";
}

/**
 * 四支 mutation。**成功後一律 invalidate 端點 23 ＋ 總覽**：S01 的卡 2 顯示的是
 * 同一份 `providerStatus`，不一起失效的話賣家串完回到總覽仍看到「尚未設定」。
 */
export interface UseProviderAccountActionsResult {
	saveCredential: (args: { sellerTaxId: string; appKey: string }) => void;
	savingCredential: boolean;
	credentialError: unknown;

	verify: () => void;
	verifying: boolean;
	verifyError: unknown;

	activate: (cutoverAt: string) => void;
	activating: boolean;
	activateError: unknown;

	rotate: (appKey: string) => void;
	rotating: boolean;
	rotateError: unknown;
}

export function useProviderAccountActions(
	expectedVersion: string,
): UseProviderAccountActionsResult {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();

	const invalidate = async () => {
		await Promise.all([
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.invoiceProviderAccount(identity),
			}),
			queryClient.invalidateQueries({
				queryKey: queryKeys.seller.invoiceOverview(identity),
			}),
		]);
	};

	const credential = useMutation({
		mutationFn: (args: { sellerTaxId: string; appKey: string }) =>
			putSellerProviderCredentials({ ...args, expectedVersion }),
		onSuccess: invalidate,
	});

	// 🔴 重試沿用同一把 idempotency key（58A §S03 CTA 表）：連線測試在服務商側
	// 是有副作用的呼叫，換 key 重試等於重打一次。key 綁在 mutation 實例上，
	// 每次「開始一輪新的測試」才換——由呼叫端在成功／改憑證後重掛。
	const verify = useMutation({
		mutationFn: (idempotencyKey: string) =>
			verifySellerProviderAccount({ expectedVersion, idempotencyKey }),
		onSuccess: invalidate,
	});

	const activate = useMutation({
		mutationFn: (cutoverAt: string) =>
			activateSellerProviderAccount({ cutoverAt, expectedVersion }),
		onSuccess: invalidate,
	});

	const rotate = useMutation({
		mutationFn: (appKey: string) =>
			rotateSellerProviderCredentials({ appKey, expectedVersion }),
		onSuccess: invalidate,
	});

	return {
		saveCredential: (args) => credential.mutate(args),
		savingCredential: credential.isPending,
		credentialError: credential.error,

		verify: () => verify.mutate(newVerifyKey()),
		verifying: verify.isPending,
		verifyError: verify.error,

		activate: (cutoverAt) => activate.mutate(cutoverAt),
		activating: activate.isPending,
		activateError: activate.error,

		rotate: (appKey) => rotate.mutate(appKey),
		rotating: rotate.isPending,
		rotateError: rotate.error,
	};
}

function newVerifyKey(): string {
	const random =
		typeof crypto !== "undefined" && "randomUUID" in crypto
			? crypto.randomUUID().replace(/-/g, "")
			: `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 12)}`;
	return `inv-verify-${random}`;
}
