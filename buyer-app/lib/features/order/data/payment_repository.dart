import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart' as pb;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/networking/connect_provider.dart';

part 'payment_repository.g.dart';

@riverpod
PaymentRepository paymentRepository(Ref ref) =>
    PaymentRepository(ref.watch(paymentAppClientProvider));

/// 買家對一張訂單的付款（`PaymentAppService`）。付款結果只認 [get]（webhook 驅動），不信任 PaymentSheet 的回報。
class PaymentRepository {
  PaymentRepository(this._client);

  final PaymentAppServiceClient _client;

  /// 對待付款訂單建立付款（冪等：重複呼叫回目前的付款；CANCELED、CREATE_FAILED 時會換新的嘗試）。
  Future<pb.Payment> create(String orderNo) async {
    final response = await _client.createPayment(
      pb.CreatePaymentRequest(orderNo: orderNo),
    );
    return response.payment;
  }

  /// 重讀付款；AWAITING_PAYMENT 且 provider 還在等時帶 session。
  Future<pb.Payment> get(String orderNo) async {
    final response = await _client.getPayment(
      pb.GetPaymentRequest(orderNo: orderNo),
    );
    return response.payment;
  }
}
