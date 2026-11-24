import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_buyer/features/order/data/payment_ui_state.dart';

pb.Payment _payment(PaymentStatus status, {bool session = false}) => pb.Payment(
  orderNo: 'O1',
  status: status,
  session: session
      ? pb.PaymentSession(
          stripe: pb.StripePaymentSession(
            clientSecret: 'pi_1_secret',
            publishableKey: 'pk_test_1',
          ),
        )
      : null,
);

void main() {
  test(
    'AWAITING_PAYMENT presents the sheet only when a session is present',
    () {
      expect(
        paymentUiStateOf(
          _payment(
            PaymentStatus.PAYMENT_STATUS_AWAITING_PAYMENT,
            session: true,
          ),
        ),
        PaymentUiState.presentSheet,
      );
      expect(
        paymentUiStateOf(
          _payment(PaymentStatus.PAYMENT_STATUS_AWAITING_PAYMENT),
        ),
        PaymentUiState.reread,
      );
    },
  );

  test('CREATING and PROCESSING are re-read, never success', () {
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_CREATING)),
      PaymentUiState.reread,
    );
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_PROCESSING)),
      PaymentUiState.reread,
    );
  });

  test('only SUCCEEDED is success', () {
    for (final status in PaymentStatus.values) {
      expect(
        paymentUiStateOf(_payment(status)) == PaymentUiState.succeeded,
        status == PaymentStatus.PAYMENT_STATUS_SUCCEEDED,
        reason: status.name,
      );
    }
  });

  test('CANCELED and CREATE_FAILED are retryable, ANOMALY is not', () {
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_CANCELED)),
      PaymentUiState.retryable,
    );
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_CREATE_FAILED)),
      PaymentUiState.retryable,
    );
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_ANOMALY)),
      PaymentUiState.anomaly,
    );
  });

  test('UNSPECIFIED and unknown values are unknown', () {
    expect(
      paymentUiStateOf(_payment(PaymentStatus.PAYMENT_STATUS_UNSPECIFIED)),
      PaymentUiState.unknown,
    );
    expect(
      paymentUiStateOf(
        _payment(
          PaymentStatus.valueOf(99) ?? PaymentStatus.PAYMENT_STATUS_UNSPECIFIED,
        ),
      ),
      PaymentUiState.unknown,
    );
  });
}
