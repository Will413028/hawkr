type SearchParamsLike = {
	get(name: string): string | null;
};

/** 付款回跳網址上的訂單編號；它是要查哪一筆，不是結果。 */
export function readPaymentOrderNo(params: SearchParamsLike | null) {
	return params?.get("order")?.trim() ?? "";
}
