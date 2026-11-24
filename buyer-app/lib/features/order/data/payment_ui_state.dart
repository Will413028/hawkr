import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';

/// 一筆付款在買家端要做的下一步（只看 [pb.Payment]，不看 PaymentSheet 的回報）。
enum PaymentUiState {
  /// AWAITING_PAYMENT 且帶 Stripe session：開 PaymentSheet。
  presentSheet,

  /// CREATING、PROCESSING，或 AWAITING_PAYMENT 但 provider 已往前（沒有 session）：稍後以 GetPayment 重讀。
  reread,

  /// SUCCEEDED：唯一算成功的狀態。
  succeeded,

  /// CANCELED、CREATE_FAILED：這次嘗試結束，訂單仍可付時再 CreatePayment 會換新的嘗試。
  retryable,

  /// ANOMALY：買家不能重試，等營運處理。
  anomaly,

  /// UNSPECIFIED 或不認得的狀態：不算成功，也不開付款視窗。
  unknown,
}

PaymentUiState paymentUiStateOf(pb.Payment payment) {
  switch (payment.status) {
    case PaymentStatus.PAYMENT_STATUS_AWAITING_PAYMENT:
      return payment.hasSession() && payment.session.hasStripe()
          ? PaymentUiState.presentSheet
          : PaymentUiState.reread;
    case PaymentStatus.PAYMENT_STATUS_CREATING:
    case PaymentStatus.PAYMENT_STATUS_PROCESSING:
      return PaymentUiState.reread;
    case PaymentStatus.PAYMENT_STATUS_SUCCEEDED:
      return PaymentUiState.succeeded;
    case PaymentStatus.PAYMENT_STATUS_CANCELED:
    case PaymentStatus.PAYMENT_STATUS_CREATE_FAILED:
      return PaymentUiState.retryable;
    case PaymentStatus.PAYMENT_STATUS_ANOMALY:
      return PaymentUiState.anomaly;
    default:
      return PaymentUiState.unknown;
  }
}
