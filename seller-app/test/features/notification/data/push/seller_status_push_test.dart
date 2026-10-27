import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/app.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/router/app_router.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/notification/data/push/router.dart';
import 'package:hawkr_seller/features/notification/data/push/seller_status_push.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/seller/data/seller_status_refresh.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

import '../../../../support/fake_my_store.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

class _Reads {
  int kyc = 0;
  late final FakeMyStoreServer store = FakeMyStoreServer();

  List<Override> get overrides => [
    sessionProvider.overrideWith(_LoggedIn.new),
    myStoreRepositoryProvider.overrideWithValue(store.repository),
    sellerClientProvider.overrideWithValue(
      SellerAppServiceClient(
        FakeTransportBuilder().unary(SellerAppService.getMyKYC, (_, _) {
          kyc++;
          throw ConnectException(Code.notFound, 'none');
        }).build(),
      ),
    ),
  ];

  Future<void> warm(ProviderContainer container) async {
    container
      ..listen(currentStoreProvider, (_, _) {})
      ..listen(currentKycProvider, (_, _) {});
    await container.read(currentStoreProvider.future);
    await container.read(accountKycProvider('acc-1').future);
  }
}

void main() {
  test('認證與 PayUni 的推播類型才是觸發訊號', () {
    for (final type in [
      'PAYUNI_STATUS',
      'payuni_status',
      'SELLER_REVIEW_STATUS',
      'MERCHANT_VERIFICATION',
      'CERTIFICATION_APPROVED',
      'CERTIFICATION_REJECTED',
      'CERTIFICATION_PENDING_INFO',
      'MERCHANT.VERIFICATION.APPROVED',
      'MERCHANT.VERIFICATION.FORWARDED',
    ]) {
      expect(isSellerStatusPushType(type), isTrue, reason: type);
    }
    for (final type in [null, '', 'ORDER_STATUS', 'CHAT_MESSAGE', 'PAYOUT']) {
      expect(isSellerStatusPushType(type), isFalse, reason: '$type');
    }
  });

  test('推播只觸發重讀店家與認證流程（PayUni 狀態在認證流程裡），不帶任何推播內容進狀態', () async {
    final reads = _Reads();
    final container = ProviderContainer(overrides: reads.overrides);
    addTearDown(container.dispose);
    await reads.warm(container);
    final storeBefore = reads.store.calls;
    final kycBefore = reads.kyc;

    refreshSellerStatusFromPush(container);
    await container
        .read(sellerStatusRefreshProvider.notifier)
        .refresh(settledOnly: true);
    await Future<void>.delayed(Duration.zero);

    expect(reads.kyc, kycBefore + 1);
    expect(reads.store.calls, storeBefore + 1);
    expect(
      container.read(accountKycProvider('acc-1')).requireValue.status,
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    );
  });

  test('沒有 container 時略過，不丟錯', () {
    expect(() => refreshSellerStatusFromPush(null), returnsNormally);
  });

  testWidgets('NotificationRouter：認證與 PayUni 推播導向錢包（不重讀），其他類型不碰', (
    tester,
  ) async {
    final reads = _Reads();
    final router = GoRouter(
      navigatorKey: MyApp.navigatorKey,
      routes: [
        GoRoute(path: '/', builder: (_, _) => const Text('home')),
        GoRoute(
          path: MainRoutes.wallet,
          builder: (_, _) => const Text('wallet'),
        ),
      ],
    );
    addTearDown(router.dispose);
    await tester.pumpWidget(
      ProviderScope(
        overrides: reads.overrides,
        child: MaterialApp.router(routerConfig: router),
      ),
    );
    final container = ProviderScope.containerOf(
      tester.element(find.text('home')),
    );
    await tester.runAsync(() => reads.warm(container));

    NotificationRouter.navigate({'type': 'SYSTEM_MAINTENANCE'});
    await tester.pump();
    expect(find.text('home'), findsOneWidget);

    for (final type in ['PAYUNI_STATUS', 'CERTIFICATION_APPROVED']) {
      router.go('/');
      await tester.pumpAndSettle();
      final kycBefore = reads.kyc;
      NotificationRouter.navigate({'type': type});
      await tester.runAsync(
        () => Future<void>.delayed(const Duration(milliseconds: 50)),
      );
      await tester.pumpAndSettle();
      expect(find.text('wallet'), findsOneWidget, reason: type);
      expect(reads.kyc, kycBefore, reason: '$type：重讀由 manager 負責，導覽不重讀');
    }
  });
}
