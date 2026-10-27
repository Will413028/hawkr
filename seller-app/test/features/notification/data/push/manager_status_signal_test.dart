import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.spec.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/connect_provider.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/notification/data/push/manager.dart';
import 'package:hawkr_seller/features/seller/data/current_kyc.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';

import '../../../../support/fake_my_store.dart';

class _LoggedIn extends Session {
  @override
  SessionModel build() =>
      const SessionModel(accountId: 'acc-1', isLoggedIn: true);
}

/// 每個通知事件只重讀一次：前景（onMessage）收到時重讀，之後點橫幅只導覽；背景點擊（onMessageOpenedApp）、冷啟動
/// （getInitialMessage 走同一個處理）與本地通知點擊沒有前景的處理，進來時重讀一次。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
  });

  late final PushNotificationManager manager;
  setUpAll(() => manager = PushNotificationManager.instance);
  tearDown(() => manager.seedContainerForTest(null));

  var kyc = 0;

  Future<ProviderContainer> container() async {
    kyc = 0;
    final store = FakeMyStoreServer();
    final overrides = <Override>[
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
    final c = ProviderContainer(overrides: overrides);
    addTearDown(c.dispose);
    c.listen(currentStoreProvider, (_, _) {});
    c.listen(currentKycProvider, (_, _) {});
    await c.read(currentStoreProvider.future);
    await c.read(accountKycProvider('acc-1').future);
    manager.seedContainerForTest(c);
    kyc = 0;
    return c;
  }

  Future<void> settle() async {
    for (var i = 0; i < 5; i++) {
      await Future<void>.delayed(Duration.zero);
    }
  }

  test('前景收到的認證狀態通知：重讀一次；之後點橫幅（只導覽）不再重讀', () async {
    await container();
    manager.handleForegroundMessageForTest(
      RemoteMessage(data: {'type': 'PAYUNI_STATUS', 'silent': 'true'}),
    );
    await settle();
    expect(kyc, 1);
  });

  test(
    '背景點擊通知（onMessageOpenedApp，冷啟動的 getInitialMessage 走同一個處理）：重讀一次',
    () async {
      await container();
      manager.handleMessageOpenedAppForTest(
        RemoteMessage(data: {'type': 'CERTIFICATION_APPROVED'}),
      );
      await settle();
      expect(kyc, 1);
    },
  );

  test('本地通知點擊（payload）：重讀一次', () async {
    await container();
    manager.navigateFromPayloadForTest(
      '{"type":"MERCHANT.VERIFICATION.FORWARDED"}',
    );
    await settle();
    expect(kyc, 1);
  });

  test('不是認證或 PayUni 的通知不重讀', () async {
    await container();
    manager.handleMessageOpenedAppForTest(
      RemoteMessage(data: {'type': 'CHAT_MESSAGE'}),
    );
    manager.handleForegroundMessageForTest(
      RemoteMessage(data: {'type': 'CHAT_MESSAGE', 'silent': 'true'}),
    );
    await settle();
    expect(kyc, 0);
  });
}
