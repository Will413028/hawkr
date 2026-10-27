import {
	type KYCFlow,
	PayUniState,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { isStoreCertified, type MyStore } from "./store";

/** 錢包頁顯示交易列表的條件：店家已認證，而且 PayUni 可以收款（`flow.payuni.state` 是 APPROVED）。錢包頁與 Link 的預取共用這一個判斷。 */
export function walletShowsTransactions(
	seller: MyStore | null | undefined,
	flow: KYCFlow | null | undefined,
): boolean {
	return (
		isStoreCertified(seller) && flow?.payuni?.state === PayUniState.APPROVED
	);
}
