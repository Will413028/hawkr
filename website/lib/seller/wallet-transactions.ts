import type { GetMyOnboardingResponse } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import { paymentAccountCompleted } from "./onboarding-summary";
import { isStoreCertified, type MyStore } from "./store";

/** 錢包頁顯示交易列表的條件：店家已認證，而且開通摘要的收款帳戶任務已完成（伺服器的答案，不讀 `PaymentAccount.status`）。錢包頁與 Link 的預取共用這一個判斷。 */
export function walletShowsTransactions(
	seller: MyStore | null | undefined,
	onboarding: GetMyOnboardingResponse | null | undefined,
): boolean {
	return isStoreCertified(seller) && paymentAccountCompleted(onboarding);
}
