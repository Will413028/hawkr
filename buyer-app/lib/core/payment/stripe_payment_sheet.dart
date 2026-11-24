import 'package:flutter_stripe/flutter_stripe.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart' as pb;

import '../../config/flavor.dart';

/// PaymentSheet 關閉時的結果。它只說明「視窗怎麼結束」，不是付款結果（付款結果只認 GetPayment）。
enum PaymentSheetResult { completed, canceled, failed }

/// 開 PaymentSheet 的介面，讓付款流程在測試裡不碰原生 SDK。
abstract interface class PaymentSheetPort {
  Future<PaymentSheetResult> present(pb.StripePaymentSession session);
}

/// `flutter_stripe` 的實作（ADR D64）：每次付款用 session 的 publishable key 初始化，3DS 與 Apple Pay 由視窗自己處理。
class StripePaymentSheet implements PaymentSheetPort {
  const StripePaymentSheet();

  static const _merchantDisplayName = 'Hawkr';
  static const _merchantCountryCode = 'TW';

  @override
  Future<PaymentSheetResult> present(pb.StripePaymentSession session) async {
    Stripe.publishableKey = session.publishableKey;
    Stripe.merchantIdentifier = F.appFlavor.applePayMerchantId;
    try {
      await Stripe.instance.applySettings();
      await Stripe.instance.initPaymentSheet(
        paymentSheetParameters: SetupPaymentSheetParameters(
          paymentIntentClientSecret: session.clientSecret,
          merchantDisplayName: _merchantDisplayName,
          applePay: const PaymentSheetApplePay(
            merchantCountryCode: _merchantCountryCode,
          ),
        ),
      );
      await Stripe.instance.presentPaymentSheet();
      return PaymentSheetResult.completed;
    } on StripeException catch (e) {
      return e.error.code == FailureCode.Canceled
          ? PaymentSheetResult.canceled
          : PaymentSheetResult.failed;
    }
  }
}
