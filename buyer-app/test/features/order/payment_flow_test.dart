import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_buyer/core/payment/stripe_payment_sheet.dart';
import 'package:hawkr_buyer/features/order/data/payment_flow.dart';
import 'package:hawkr_buyer/features/order/data/payment_repository.dart';

pb.Payment _p(PaymentStatus status, {bool session = false}) => pb.Payment(
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

class _Sheet implements PaymentSheetPort {
  _Sheet(this.result);
  final PaymentSheetResult result;
  final List<pb.StripePaymentSession> presented = [];

  @override
  Future<PaymentSheetResult> present(pb.StripePaymentSession session) async {
    presented.add(session);
    return result;
  }
}

class _Server {
  _Server(this.created, this.reads);
  final pb.Payment created;
  final List<pb.Payment> reads;
  int createCalls = 0;
  int getCalls = 0;

  PaymentRepository repository() => PaymentRepository(
    PaymentAppServiceClient(
      FakeTransportBuilder()
          .unary(PaymentAppService.createPayment, (req, _) async {
            createCalls++;
            expect(req.orderNo, 'O1');
            return pb.CreatePaymentResponse(payment: created);
          })
          .unary(PaymentAppService.getPayment, (req, _) async {
            expect(req.orderNo, 'O1');
            final i = getCalls < reads.length ? getCalls : reads.length - 1;
            getCalls++;
            return pb.GetPaymentResponse(payment: reads[i]);
          })
          .build(),
    ),
  );
}

PaymentFlow _flow(_Server server, _Sheet sheet, {int maxReads = 3}) =>
    PaymentFlow(
      repository: server.repository(),
      sheet: sheet,
      maxReads: maxReads,
      delay: (_) async {},
    );

void main() {
  const awaiting = PaymentStatus.PAYMENT_STATUS_AWAITING_PAYMENT;

  test(
    'sheet completed then GetPayment SUCCEEDED is the only success',
    () async {
      final server = _Server(_p(awaiting, session: true), [
        _p(PaymentStatus.PAYMENT_STATUS_PROCESSING),
        _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
      ]);
      final sheet = _Sheet(PaymentSheetResult.completed);
      expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.succeeded);
      expect(sheet.presented.single.clientSecret, 'pi_1_secret');
      expect(sheet.presented.single.publishableKey, 'pk_test_1');
      expect(server.getCalls, 2);
    },
  );

  test(
    'sheet completed but never confirmed ends pending after the cap',
    () async {
      final server = _Server(_p(awaiting, session: true), [
        _p(PaymentStatus.PAYMENT_STATUS_PROCESSING),
      ]);
      final sheet = _Sheet(PaymentSheetResult.completed);
      expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.pending);
      expect(server.getCalls, 3);
      expect(sheet.presented, hasLength(1));
    },
  );

  test(
    'a re-read that still has a session does not reopen the sheet',
    () async {
      final server = _Server(_p(awaiting, session: true), [
        _p(awaiting, session: true),
        _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
      ]);
      final sheet = _Sheet(PaymentSheetResult.completed);
      expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.succeeded);
      expect(sheet.presented, hasLength(1));
    },
  );

  test('canceling the sheet stops without reading', () async {
    final server = _Server(_p(awaiting, session: true), [
      _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
    ]);
    final sheet = _Sheet(PaymentSheetResult.canceled);
    expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.canceled);
    expect(server.getCalls, 0);
  });

  test('a failed sheet is a failure even if nothing was read', () async {
    final server = _Server(_p(awaiting, session: true), [
      _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
    ]);
    final sheet = _Sheet(PaymentSheetResult.failed);
    expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.failed);
    expect(server.getCalls, 0);
  });

  test('CREATING is re-read until a session appears', () async {
    final server = _Server(_p(PaymentStatus.PAYMENT_STATUS_CREATING), [
      _p(PaymentStatus.PAYMENT_STATUS_CREATING),
      _p(awaiting, session: true),
      _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
    ]);
    final sheet = _Sheet(PaymentSheetResult.completed);
    expect(
      await _flow(server, sheet, maxReads: 5).pay('O1'),
      PaymentOutcome.succeeded,
    );
    expect(sheet.presented, hasLength(1));
  });

  test(
    'CREATE_FAILED, CANCELED and UNSPECIFIED are failures without a sheet',
    () async {
      for (final status in [
        PaymentStatus.PAYMENT_STATUS_CREATE_FAILED,
        PaymentStatus.PAYMENT_STATUS_CANCELED,
        PaymentStatus.PAYMENT_STATUS_UNSPECIFIED,
      ]) {
        final server = _Server(_p(status), [_p(status)]);
        final sheet = _Sheet(PaymentSheetResult.completed);
        expect(
          await _flow(server, sheet).pay('O1'),
          PaymentOutcome.failed,
          reason: status.name,
        );
        expect(sheet.presented, isEmpty);
      }
    },
  );

  test('ANOMALY is not retryable and opens no sheet', () async {
    final server = _Server(_p(PaymentStatus.PAYMENT_STATUS_ANOMALY), [
      _p(PaymentStatus.PAYMENT_STATUS_ANOMALY),
    ]);
    final sheet = _Sheet(PaymentSheetResult.completed);
    expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.anomaly);
    expect(sheet.presented, isEmpty);
  });

  test('already SUCCEEDED on create returns success without a sheet', () async {
    final server = _Server(_p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED), [
      _p(PaymentStatus.PAYMENT_STATUS_SUCCEEDED),
    ]);
    final sheet = _Sheet(PaymentSheetResult.completed);
    expect(await _flow(server, sheet).pay('O1'), PaymentOutcome.succeeded);
    expect(sheet.presented, isEmpty);
    expect(server.createCalls, 1);
  });
}
