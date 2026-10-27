type SearchParamsLike = {
	get(name: string): string | null;
};

function firstParam(params: SearchParamsLike | null, keys: readonly string[]) {
	if (!params) return "";
	for (const key of keys) {
		const value = params.get(key)?.trim();
		if (value) return value;
	}
	return "";
}

export function readPaymentTradeNo(params: SearchParamsLike | null) {
	return firstParam(params, [
		"trade_no",
		"tradeNo",
		"TradeNo",
		"tradeNO",
		"merchantTradeNo",
		"MerchantTradeNo",
		"merTradeNo",
		"MerTradeNo",
		"providerTradeNo",
		"ProviderTradeNo",
	]);
}

export function readPaymentOrderNo(params: SearchParamsLike | null) {
	return firstParam(params, ["order", "order_no", "orderNo", "OrderNo"]);
}

export function readPaymentStatusHint(params: SearchParamsLike | null) {
	return firstParam(params, [
		"status",
		"Status",
		"payment_status",
		"paymentStatus",
		"code",
		"Code",
		"result",
		"Result",
	]);
}
