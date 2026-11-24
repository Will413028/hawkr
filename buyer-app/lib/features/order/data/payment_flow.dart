import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/payment/stripe_payment_sheet.dart';
import 'payment_repository.dart';
import 'payment_ui_state.dart';

part 'payment_flow.g.dart';

@riverpod
PaymentFlow paymentFlow(Ref ref) => PaymentFlow(
  repository: ref.watch(paymentRepositoryProvider),
  sheet: const StripePaymentSheet(),
);

/// 一次付款流程的結果；除 [succeeded] 以外都不是成功。
enum PaymentOutcome {
  /// GetPayment 回 SUCCEEDED。
  succeeded,

  /// 重讀次數用完仍在處理中（CREATING、PROCESSING 或 provider 已往前還沒同步）：結果以後由 webhook 決定。
  pending,

  /// 買家關掉 PaymentSheet：訂單仍待付款，可再試。
  canceled,

  /// 這次嘗試沒有成功（建立失敗、取消、PaymentSheet 失敗或狀態不明）：訂單仍可付時可再試。
  failed,

  /// ANOMALY：買家不能重試，等營運處理。
  anomaly,
}

/// CreatePayment、PaymentSheet 與 GetPayment 重讀串成一次付款。網路與伺服器錯誤原樣拋出（[ConnectException]）。
class PaymentFlow {
  PaymentFlow({
    required PaymentRepository repository,
    required PaymentSheetPort sheet,
    this.maxReads = 8,
    this.readInterval = const Duration(milliseconds: 1500),
    Future<void> Function(Duration)? delay,
  }) : _repository = repository,
       _sheet = sheet,
       _delay = delay ?? Future<void>.delayed;

  final PaymentRepository _repository;
  final PaymentSheetPort _sheet;

  /// 重讀 GetPayment 的次數上限（不含 CreatePayment 的回應）。
  final int maxReads;
  final Duration readInterval;
  final Future<void> Function(Duration) _delay;

  Future<PaymentOutcome> pay(String orderNo) async {
    var payment = await _repository.create(orderNo);
    var sheetPresented = false;
    var reads = 0;
    while (true) {
      switch (paymentUiStateOf(payment)) {
        case PaymentUiState.succeeded:
          return PaymentOutcome.succeeded;
        case PaymentUiState.anomaly:
          return PaymentOutcome.anomaly;
        case PaymentUiState.retryable:
        case PaymentUiState.unknown:
          return PaymentOutcome.failed;
        case PaymentUiState.presentSheet when !sheetPresented:
          sheetPresented = true;
          switch (await _sheet.present(payment.session.stripe)) {
            case PaymentSheetResult.completed:
              break;
            case PaymentSheetResult.canceled:
              return PaymentOutcome.canceled;
            case PaymentSheetResult.failed:
              return PaymentOutcome.failed;
          }
        case PaymentUiState.presentSheet:
        case PaymentUiState.reread:
          break;
      }
      if (reads >= maxReads) return PaymentOutcome.pending;
      reads++;
      await _delay(readInterval);
      payment = await _repository.get(orderNo);
    }
  }
}
