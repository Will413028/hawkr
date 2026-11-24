//
//  Generated code. Do not modify.
//  source: hawkr/payment/v1alpha1/payment.proto
//

import "package:connectrpc/connect.dart" as connect;
import "payment.pb.dart" as hawkrpaymentv1alpha1payment;
import "payment.connect.spec.dart" as specs;

/// PaymentAppService 只有登入的顧客帳號（買家）能呼叫；只讀寫呼叫者自己的付款（他人的訂單回 not_found）。
extension type PaymentAppServiceClient (connect.Transport _transport) {
  /// CreatePayment 對買家自己的一張待付款訂單建立付款（冪等：同一張訂單只有一筆付款、同時只有一個有效的 PaymentIntent，重複呼叫回目前的付款）。
  /// 訂單不是呼叫者的回 not_found；訂單不可付（已取消、已付款以外的狀態）、零元訂單、金額或店家與既有付款不同回 failed_precondition；沒有設定付款 provider 回 failed_precondition。
  /// provider 暫時失敗或逾時時回狀態 CREATING 的付款（沒有 session），背景補做之後以 GetPayment 重讀；provider 永久拒絕時回狀態 CREATE_FAILED（不是錯誤），訂單仍可付時再呼叫一次會換新的嘗試。
  /// 狀態 AWAITING_PAYMENT 時帶 session。
  Future<hawkrpaymentv1alpha1payment.CreatePaymentResponse> createPayment(
    hawkrpaymentv1alpha1payment.CreatePaymentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.PaymentAppService.createPayment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }

  /// GetPayment 讀買家自己的付款（沒有付款或不是呼叫者的回 not_found）。狀態 AWAITING_PAYMENT 時帶 session（client secret 是每次向 provider 讀的，不存）；
  /// provider 那邊已往前（買家剛付款）時不帶 session、狀態仍是 AWAITING_PAYMENT，背景同步後再讀會是 PROCESSING 或 SUCCEEDED。回應含 client secret，一律 POST，不可被快取。
  Future<hawkrpaymentv1alpha1payment.GetPaymentResponse> getPayment(
    hawkrpaymentv1alpha1payment.GetPaymentRequest input, {
    connect.Headers? headers,
    connect.AbortSignal? signal,
    Function(connect.Headers)? onHeader,
    Function(connect.Headers)? onTrailer,
  }) {
    return connect.Client(_transport).unary(
      specs.PaymentAppService.getPayment,
      input,
      signal: signal,
      headers: headers,
      onHeader: onHeader,
      onTrailer: onTrailer,
    );
  }
}
