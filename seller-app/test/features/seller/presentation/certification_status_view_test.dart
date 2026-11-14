import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:protobuf/well_known_types/google/protobuf/timestamp.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/payment_onboarding.dart';
import 'package:hawkr_seller/features/seller/presentation/certification_status_view.dart';

import '../../../support/fake_my_store.dart';
import '../../../support/connect_errors.dart';
import '../../../support/kyc_fixtures.dart';
import '../../../support/onboarding_fixtures.dart';

const _actionRequired =
    PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ACTION_REQUIRED;
const _hosted = OnboardingSurface.ONBOARDING_SURFACE_HOSTED;
const _embedded = OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED;

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Server {
  /// GetMyPaymentAccount 的回應；null 是 not_found。
  PaymentAccount? payment;
  Object? paymentFailure;
  int paymentCalls = 0;

  /// GetMyOnboarding 的回應；null 時照申請狀態：核准是平台審核完成，其餘是還沒完成。
  GetMyOnboardingResponse? summary;
  Object? summaryFailure;
  bool approved = true;

  /// RefreshMyPaymentAccount 的失敗；null 是成功。
  Object? refreshFailure;

  /// 依序記下伺服器收到的呼叫，驗證「先重讀再讀」的順序。
  final calls = <String>[];
  int refreshCalls = 0;
  final sessions = <CreatePaymentOnboardingSessionRequest>[];
  final launched = <Uri>[];

  SellerAppServiceClient get client => SellerAppServiceClient(
    FakeTransportBuilder()
        .unary(SellerAppService.getMyKYC, (_, _) {
          return GetMyKYCResponse(application: individualApplication());
        })
        .unary(SellerAppService.getMyOnboarding, (_, _) {
          calls.add('getMyOnboarding');
          final failure = summaryFailure;
          if (failure != null) throw failure;
          return summary ??
              (approved ? paymentPendingSummary() : reviewPendingSummary());
        })
        .unary(SellerAppService.refreshMyPaymentAccount, (_, _) {
          calls.add('refreshMyPaymentAccount');
          refreshCalls++;
          final failure = refreshFailure;
          if (failure != null) throw failure;
          return RefreshMyPaymentAccountResponse();
        })
        .unary(SellerAppService.getMyPaymentAccount, (_, _) {
          calls.add('getMyPaymentAccount');
          paymentCalls++;
          final failure = paymentFailure;
          if (failure != null) throw failure;
          final account = payment;
          if (account == null) throw ConnectException(Code.notFound, 'none');
          return GetMyPaymentAccountResponse(account: account);
        })
        .unary(SellerAppService.createPaymentOnboardingSession, (req, _) {
          sessions.add(req);
          return CreatePaymentOnboardingSessionResponse(
            surface: _hosted,
            url: 'https://connect.example.test/setup',
          );
        })
        .build(),
  );
}

Future<void> _pump(
  WidgetTester tester,
  _Server server, {
  KYCStatus review = KYCStatus.KYC_STATUS_APPROVED,
  Timestamp? decideTime,
}) async {
  server.approved = review == KYCStatus.KYC_STATUS_APPROVED;
  final store = FakeMyStoreServer();
  final container = ProviderContainer(
    overrides: [
      sessionProvider.overrideWith(_LoggedIn.new),
      sellerClientProvider.overrideWithValue(server.client),
      myStoreRepositoryProvider.overrideWithValue(store.repository),
      webOriginProvider.overrideWithValue('https://web.example.test'),
      externalBrowserLauncherProvider.overrideWithValue((uri) async {
        server.launched.add(uri);
        return true;
      }),
    ],
  );
  addTearDown(container.dispose);
  final subs = [
    container.listen(currentStoreProvider, (_, _) {}),
    container.listen(currentKycProvider, (_, _) {}),
  ];
  for (final sub in subs) {
    addTearDown(sub.close);
  }
  final application = individualApplication(kycStatus: review);
  if (decideTime != null) application.decideTime = decideTime;
  final router = GoRouter(
    routes: [
      GoRoute(
        path: '/',
        builder: (_, _) =>
            Scaffold(body: CertificationStatusView(application: application)),
      ),
      GoRoute(
        path: SettingsRoutes.help,
        builder: (_, _) => const Scaffold(body: Text('help')),
      ),
    ],
  );
  addTearDown(router.dispose);
  tester.view.physicalSize = const Size(800, 3000);
  tester.view.devicePixelRatio = 2;
  addTearDown(tester.view.reset);
  await tester.pumpWidget(
    UncontrolledProviderScope(
      container: container,
      child: ScreenUtilInit(
        designSize: const Size(393, 852),
        builder: (_, _) => MaterialApp.router(routerConfig: router),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Finder _in(String key, String text) =>
    find.descendant(of: find.byKey(Key(key)), matching: find.text(text));

void main() {
  group('平台審核區：顯示 KYCApplication.status，文字取自目錄', () {
    testWidgets('審核中：徽章與送出時間；收款帳戶區是灰色的提示，不讀 GetMyPaymentAccount', (
      tester,
    ) async {
      final server = _Server();
      await _pump(tester, server, review: KYCStatus.KYC_STATUS_SUBMITTED);

      expect(_in('kyc_platform_review', '審核中'), findsOneWidget);
      expect(_in('kyc_payment_account', '平台審核通過後開放設定'), findsOneWidget);
      expect(find.byType(FilledButton), findsNothing);
      expect(server.paymentCalls, 0);
    });

    testWidgets('已通過：徽章與核准時間', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(status: _actionRequired);
      await _pump(
        tester,
        server,
        decideTime: Timestamp.fromDateTime(DateTime.utc(2026, 11, 3, 6)),
      );

      expect(_in('kyc_platform_review', '已通過'), findsOneWidget);
      expect(find.textContaining('核准時間：'), findsOneWidget);
    });

    testWidgets('收款帳戶區是否開放讀開通摘要，不看申請狀態：申請已核准但摘要說平台審核還沒完成，仍是灰色提示', (
      tester,
    ) async {
      final server = _Server()
        ..summary = reviewPendingSummary()
        ..payment = PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
          payoutsActive: true,
        );
      await _pump(tester, server);

      expect(_in('kyc_platform_review', '已通過'), findsOneWidget);
      expect(_in('kyc_payment_account', '平台審核通過後開放設定'), findsOneWidget);
      expect(server.paymentCalls, 0);
    });

    testWidgets('申請還是審核中但摘要說平台審核已完成：照摘要開放收款帳戶區', (tester) async {
      final server = _Server()
        ..summary = paymentPendingSummary()
        ..payment = PaymentAccount(status: _actionRequired);
      await _pump(tester, server, review: KYCStatus.KYC_STATUS_SUBMITTED);

      expect(_in('kyc_platform_review', '審核中'), findsOneWidget);
      expect(_in('kyc_payment_account', '待設定'), findsOneWidget);
    });

    testWidgets('摘要讀取失敗：顯示錯誤與重新整理，不當成「尚未開放」也不讀收款帳戶', (tester) async {
      final server = _Server()
        ..summaryFailure = ConnectException(Code.unavailable, 'down');
      await _pump(tester, server);

      expect(_in('kyc_payment_account', '重新整理'), findsOneWidget);
      expect(find.text('平台審核通過後開放設定'), findsNothing);
      expect(server.paymentCalls, 0);
    });

    testWidgets('不認得的申請狀態（UNSPECIFIED）：徽章是目錄的「未知」', (tester) async {
      await _pump(tester, _Server(), review: KYCStatus.KYC_STATUS_UNSPECIFIED);
      expect(_in('kyc_platform_review', '未知'), findsOneWidget);
    });
  });

  group('收款帳戶區：內容讀 GetMyPaymentAccount，文字取自目錄', () {
    testWidgets('待設定：說明不說逾期或停用；HOSTED 的按鈕在外部瀏覽器開連結，回跳網址指向回跳頁', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        );
      await _pump(tester, server);

      expect(_in('kyc_payment_account', '待設定'), findsOneWidget);
      expect(find.text('提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。'), findsOneWidget);
      expect(find.textContaining('逾期'), findsNothing);
      expect(find.textContaining('停用'), findsNothing);

      await tester.tap(find.byKey(const Key('payment_account_onboard')));
      await tester.pumpAndSettle();

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
        Uri.parse('https://connect.example.test/setup'),
      ]);
    });

    testWidgets('EMBEDDED：顯示「請到網頁版完成收款設定」，沒有開通按鈕，不呼叫伺服器', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _embedded,
        );
      await _pump(tester, server);

      expect(find.text('請到網頁版完成收款設定'), findsOneWidget);
      expect(find.byKey(const Key('payment_account_onboard')), findsNothing);
      expect(server.sessions, isEmpty);
    });

    testWidgets('待設定但沒有告知介面：沒有開通按鈕（不猜預設介面）', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(status: _actionRequired);
      await _pump(tester, server);

      expect(find.byKey(const Key('payment_account_onboard')), findsNothing);
      expect(find.text('請到網頁版完成收款設定'), findsNothing);
    });

    testWidgets(
      '平台審核完成、讀不到收款帳戶（not_found）：顯示錯誤與重新整理，不自己當成「準備中」；重新整理先呼叫 RefreshMyPaymentAccount 再讀',
      (tester) async {
        final server = _Server();
        await _pump(tester, server);

        expect(_in('kyc_payment_account', '準備中'), findsNothing);
        expect(_in('kyc_payment_account', '重新整理'), findsOneWidget);
        server.calls.clear();
        await tester.tap(find.text('重新整理'));
        await tester.pumpAndSettle();
        expect(server.calls.first, 'refreshMyPaymentAccount');
        expect(server.calls, contains('getMyOnboarding'));
        expect(server.calls, contains('getMyPaymentAccount'));
      },
    );

    testWidgets('已啟用但撥款還沒啟用：多一行撥款提醒', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
          payoutsActive: false,
        );
      await _pump(tester, server);

      expect(_in('kyc_payment_account', '已啟用'), findsOneWidget);
      expect(find.text('收款已啟用。'), findsOneWidget);
      expect(find.text('撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。'), findsOneWidget);
    });

    testWidgets('無法啟用：動作是聯絡客服，進幫助中心', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_DISABLED,
        );
      await _pump(tester, server);

      expect(_in('kyc_payment_account', '無法啟用'), findsOneWidget);
      await tester.tap(find.text('聯絡客服'));
      await tester.pumpAndSettle();
      expect(find.text('help'), findsOneWidget);
    });

    testWidgets('讀取失敗或不認得的狀態：顯示錯誤與重新整理，不當成任何有效狀態', (tester) async {
      final server = _Server()
        ..paymentFailure = ConnectException(Code.unavailable, 'down');
      await _pump(tester, server);

      expect(_in('kyc_payment_account', '重新整理'), findsOneWidget);
      expect(find.text('待設定'), findsNothing);

      server.paymentFailure = null;
      server.payment = PaymentAccount();
      await tester.tap(find.text('重新整理'));
      await tester.pumpAndSettle();
      expect(_in('kyc_payment_account', '重新整理'), findsOneWidget);
    });
  });

  group('重新整理與回到前景', () {
    testWidgets('「重新整理」重讀失敗：照樣讀，並顯示伺服器的句子', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
        )
        ..refreshFailure = reasonError(
          Code.failedPrecondition,
          'PAYMENT_NOT_CONFIGURED',
          message: '尚未設定收款',
        );
      await _pump(tester, server);
      server.calls.clear();

      await tester.tap(find.byKey(const Key('payment_account_refresh')));
      await tester.pumpAndSettle();

      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(server.calls, contains('getMyOnboarding'));
      expect(find.text('尚未設定收款'), findsOneWidget);
      await tester.pumpAndSettle(const Duration(seconds: 5));
    });

    testWidgets('一般回前景不呼叫 RefreshMyPaymentAccount', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        );
      await _pump(tester, server);

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();

      expect(server.refreshCalls, 0);
      expect(server.calls, contains('getMyOnboarding'));
    });

    testWidgets('從外部瀏覽器回來：先 RefreshMyPaymentAccount 再讀摘要與帳戶', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        );
      await _pump(tester, server);
      await tester.tap(find.byKey(const Key('payment_account_onboard')));
      await tester.pumpAndSettle();
      server.calls.clear();

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();

      expect(server.refreshCalls, 1);
      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(
        server.calls.skip(1),
        containsAll(['getMyOnboarding', 'getMyPaymentAccount']),
      );
    });

    testWidgets('從外部瀏覽器回來而重讀失敗：照樣讀，並顯示伺服器的句子', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        )
        ..refreshFailure = reasonError(
          Code.failedPrecondition,
          'PAYMENT_NOT_CONFIGURED',
          message: '尚未設定收款',
        );
      await _pump(tester, server);
      await tester.tap(find.byKey(const Key('payment_account_onboard')));
      await tester.pumpAndSettle();
      server.calls.clear();

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();

      expect(server.calls.first, 'refreshMyPaymentAccount');
      expect(server.calls, contains('getMyOnboarding'));
      expect(find.text('尚未設定收款'), findsOneWidget);
      await tester.pumpAndSettle(const Duration(seconds: 5));
    });

    testWidgets('AppLifecycleState.resumed 重讀收款帳戶（GetMyPaymentAccount）與開通摘要', (
      tester,
    ) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        );
      await _pump(tester, server);
      final before = server.paymentCalls;

      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();

      expect(server.paymentCalls, greaterThan(before));
    });

    testWidgets('從外部瀏覽器回來後，畫面顯示重讀到的新狀態', (tester) async {
      final server = _Server()
        ..payment = PaymentAccount(
          status: _actionRequired,
          onboardingSurface: _hosted,
        );
      await _pump(tester, server);
      expect(_in('kyc_payment_account', '待設定'), findsOneWidget);
      await tester.tap(find.byKey(const Key('payment_account_onboard')));
      await tester.pumpAndSettle();

      server.payment = PaymentAccount(
        status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_UNDER_REVIEW,
      );
      tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
      await tester.pumpAndSettle();

      expect(_in('kyc_payment_account', '審核中'), findsOneWidget);
    });
  });
}
