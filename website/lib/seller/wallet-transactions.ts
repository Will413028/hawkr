import {
	type PaymentAccount,
	PaymentAccountStatus,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { isStoreCertified, type MyStore } from "./store";

/** 錢包頁顯示交易列表的條件：店家已認證，而且收款帳戶可以收款（`PaymentAccount.status` 是 ENABLED）。錢包頁與 Link 的預取共用這一個判斷。 */
export function walletShowsTransactions(
	seller: MyStore | null | undefined,
	account: PaymentAccount | null | undefined,
): boolean {
	return (
		isStoreCertified(seller) && account?.status === PaymentAccountStatus.ENABLED
	);
}
