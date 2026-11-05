import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/payment_account.dart';
import 'package:hawkr_seller/features/seller/data/payment_onboarding.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';

import '../../../support/fake_my_store.dart';
import '../../../support/kyc_fixtures.dart';

const _hosted = OnboardingSurface.ONBOARDING_SURFACE_HOSTED;
const _embedded = OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED;

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

/// 假的伺服器：記下 GetMyPaymentAccount 與 CreatePaymentOnboardingSession 的呼叫。
class _Server {
  int paymentCalls = 0;
  final sessions = <CreatePaymentOnboardingSessionRequest>[];
  final launched = <Uri>[];
  bool launchResult = true;

  /// 第 n 次（從 1 起）GetMyPaymentAccount 要等的門。
  final gates = <int, Completer<void>>{};
  CreatePaymentOnboardingSessionResponse sessionResponse =
      CreatePaymentOnboardingSessionResponse(
        surface: _hosted,
        url: 'https://connect.example.test/setup/abc',
      );

  late final store = FakeMyStoreServer();

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) {
          return GetMyKYCResponse(application: individualApplication());
        })
        .unary(SellerAppService.getMyPaymentAccount, (_, _) async {
          final call = ++paymentCalls;
          await gates[call]?.future;
          return GetMyPaymentAccountResponse(
            account: PaymentAccount(
              status:
                  PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED,
              onboardingSurface: _hosted,
            ),
          );
        })
        .unary(SellerAppService.createPaymentOnboardingSession, (req, _) {
          sessions.add(req);
          return sessionResponse;
        })
        .build(),
  );

  ProviderContainer container({String origin = 'https://web.example.test'}) {
    final container = ProviderContainer(
      overrides: [
        sessionProvider.overrideWith(_LoggedIn.new),
        sellerClientProvider.overrideWithValue(client),
        myStoreRepositoryProvider.overrideWithValue(store.repository),
        webOriginProvider.overrideWithValue(origin),
        externalBrowserLauncherProvider.overrideWithValue((uri) async {
          launched.add(uri);
          return launchResult;
        }),
      ],
    );
    addTearDown(container.dispose);
    final subs = [
      container.listen(currentStoreProvider, (_, _) {}),
      container.listen(currentKycProvider, (_, _) {}),
      container.listen(currentPaymentAccountProvider, (_, _) {}),
    ];
    for (final sub in subs) {
      addTearDown(sub.close);
    }
    return container;
  }
}

void main() {
  group('paymentReturnUri：website 的回跳頁', () {
    test(
      '<origin>/seller-app/payment-return?state=done|expired，預設語系沒有路徑前綴',
      () {
        expect(
          paymentReturnUri('https://web.example.test', PaymentReturnState.done),
          Uri.parse(
            'https://web.example.test/seller-app/payment-return?state=done',
          ),
        );
        expect(
          paymentReturnUri(
            'http://localhost:3000',
            PaymentReturnState.expired,
          ).toString(),
          'http://localhost:3000/seller-app/payment-return?state=expired',
        );
      },
    );
  });

  group('start', () {
    test('HOSTED：return_url 與 refresh_url 指向回跳頁，在外部瀏覽器開啟連結', () async {
      final server = _Server();
      final container = server.container();

      final outcome = await container
          .read(paymentOnboardingProvider.notifier)
          .start(_hosted);

      expect(outcome, PaymentOnboardingOutcome.launched);
      final request = server.sessions.single;
      expect(
        request.returnUrl,
        'https://web.example.test/seller-app/payment-return?state=done',
      );
      expect(
        request.refreshUrl,
        'https://web.example.test/seller-app/payment-return?state=expired',
      );
      expect(server.launched, [
        Uri.parse('https://connect.example.test/setup/abc'),
      ]);
    });

    test('EMBEDDED：App 沒有內嵌元件，不呼叫伺服器、不開瀏覽器，要到網頁版完成', () async {
      final server = _Server();
      final container = server.container();

      final outcome = await container
          .read(paymentOnboardingProvider.notifier)
          .start(_embedded);

      expect(outcome, PaymentOnboardingOutcome.needsWeb);
      expect(server.sessions, isEmpty);
      expect(server.launched, isEmpty);
    });

    test('沒有告知介面（UNSPECIFIED）：不猜預設介面，失敗', () async {
      final server = _Server();
      final container = server.container();

      final outcome = await container
          .read(paymentOnboardingProvider.notifier)
          .start(OnboardingSurface.ONBOARDING_SURFACE_UNSPECIFIED);

      expect(outcome, PaymentOnboardingOutcome.failed);
      expect(server.sessions, isEmpty);
    });

    test('伺服器回的介面不是託管頁面或連結為空：失敗，不開瀏覽器', () async {
      final server = _Server()
        ..sessionResponse = CreatePaymentOnboardingSessionResponse(
          surface: _embedded,
          clientSecret: 'secret',
        );
      final container = server.container();
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(_hosted),
        PaymentOnboardingOutcome.failed,
      );

      server.sessionResponse = CreatePaymentOnboardingSessionResponse(
        surface: _hosted,
      );
      expect(
        await container.read(paymentOnboardingProvider.notifier).start(_hosted),
        PaymentOnboardingOutcome.failed,
      );
      expect(server.launched, isEmpty);
    });

    test('瀏覽器開不起來：失敗，之後回到前景不當成從瀏覽器回來', () async {
      final server = _Server()..launchResult = false;
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);

      expect(await notifier.start(_hosted), PaymentOnboardingOutcome.failed);

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final first = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([first, resumed]);
      expect(
        server.paymentCalls,
        2,
        reason: '進行中的一輪共用：沒有第二輪（開瀏覽器失敗，沒有「從瀏覽器回來」）',
      );
    });
  });

  group('onResumed：回到前景重讀', () {
    test('重讀店家、認證流程與收款帳戶（GetMyPaymentAccount）', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final before = server.paymentCalls;

      await container.read(paymentOnboardingProvider.notifier).onResumed();

      expect(server.paymentCalls, before + 1);
    });

    test('從外部瀏覽器回來帶 fresh: true：進行中那一輪可能在狀態改變之前就開始讀了，之後再讀一輪', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      expect(await notifier.start(_hosted), PaymentOnboardingOutcome.launched);

      // 賣家在瀏覽器裡的時候，已經有一輪重讀在進行（還沒讀完）。
      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;

      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(
        server.paymentCalls,
        callsDuringRound + 1,
        reason: 'fresh: true：進行中那輪結束後，另起一輪',
      );
    });

    test('不是從瀏覽器回來（一般回前景）共用進行中的一輪', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;

      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(server.paymentCalls, callsDuringRound);
    });

    test('「從瀏覽器回來」只算一次：第二次回前景不再帶 fresh', () async {
      final server = _Server();
      final container = server.container();
      await container.read(accountPaymentAccountProvider('acc-1').future);
      final notifier = container.read(paymentOnboardingProvider.notifier);
      await notifier.start(_hosted);
      await notifier.onResumed();

      final gate = Completer<void>();
      server.gates[server.paymentCalls + 1] = gate;
      final inFlight = container
          .read(sellerStatusRefreshProvider.notifier)
          .refresh();
      await Future<void>.delayed(Duration.zero);
      final callsDuringRound = server.paymentCalls;
      final resumed = notifier.onResumed();
      gate.complete();
      await Future.wait([inFlight, resumed]);

      expect(server.paymentCalls, callsDuringRound);
    });
  });
}
