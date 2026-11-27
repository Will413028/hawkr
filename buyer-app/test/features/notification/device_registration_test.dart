import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pb.dart'
    as pb;
import 'package:hawkr_api/gen/hawkr/notification/v1alpha1/notification.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_buyer/core/networking/connect_provider.dart';
import 'package:hawkr_buyer/core/session/auth_session_provider.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_auth_session.dart';
import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';

class _AccountRepository extends Fake implements AuthRepository {
  @override
  Future<AccountSnapshot> currentAccount() async =>
      const AccountSnapshot(accountId: 'acc_01J8Z');
}

/// 裝置註冊走 Connect 的 `NotificationAppService`：憑證只由 auth transport 帶上，manager 不持有 session token。
/// 請求不帶 `platform`，只帶 App 類型。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late PushNotificationManager manager;

  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  setUp(() {
    mockFirebaseCore();
    SharedPreferences.setMockInitialValues({});
    manager = PushNotificationManager.instance;
    addTearDown(manager.seedLifecycleStateForTest);
  });

  /// 以登入中的 session token 與 GetSession 還原 session；裝置 token 的請求記進 [registered] 與 [deleted]。
  Future<
    ({
      ProviderContainer c,
      List<pb.RegisterDeviceTokenRequest> registered,
      List<pb.DeleteDeviceTokenRequest> deleted,
      FakeAuthSession auth,
    })
  >
  restore({bool deleteFails = false}) async {
    mockSecureStorage({});
    final auth = FakeAuthSession(signedIn: true);
    final registered = <pb.RegisterDeviceTokenRequest>[];
    final deleted = <pb.DeleteDeviceTokenRequest>[];
    final transport = FakeTransportBuilder()
        .unary(NotificationAppService.registerDeviceToken, (req, _) {
          registered.add(req);
          return pb.RegisterDeviceTokenResponse();
        })
        .unary(NotificationAppService.deleteDeviceToken, (req, _) {
          deleted.add(req);
          auth.calls.add('deleteDeviceToken');
          if (deleteFails) {
            throw ConnectException(Code.unauthenticated, 'session revoked');
          }
          return pb.DeleteDeviceTokenResponse();
        })
        .build();
    final container = ProviderContainer(
      overrides: [
        authSessionProvider.overrideWith((ref) async => auth),
        authRepositoryProvider.overrideWith((ref) => _AccountRepository()),
        notificationAppClientProvider.overrideWithValue(
          NotificationAppServiceClient(transport),
        ),
      ],
    );
    addTearDown(container.dispose);
    manager.injectContainer(container);
    await container.read(sessionProvider.future);
    return (c: container, registered: registered, deleted: deleted, auth: auth);
  }

  test('冷啟動恢復 session → 以恢復的 session 憑證註冊裝置', () async {
    TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
        .setMockMethodCallHandler(
          const MethodChannel('plugins.flutter.io/firebase_messaging'),
          (call) async =>
              call.method == 'Messaging#getToken' ? {'token': 'fcm-1'} : null,
        );

    final h = await restore();
    await pumpEventQueue();

    final register = h.registered.single;
    expect(register.token, 'fcm-1');
    expect(register.app, NotificationApp.NOTIFICATION_APP_BUYER_APP);
    expect(register.toProto3Json(), {
      'token': 'fcm-1',
      'app': 'NOTIFICATION_APP_BUYER_APP',
    });
  });

  group('登出時註銷裝置', () {
    setUp(() {
      PushNotificationManager.e2eDisabled = true;
      addTearDown(() => PushNotificationManager.e2eDisabled = false);
    });

    test(
      '先以登入中的 session token token 註銷，再登出 session token、清除 session；第二次登出不再送',
      () async {
        final h = await restore();
        manager.seedLifecycleStateForTest(
          fcmToken: 'fcm-1',
          isUserLoggedIn: true,
        );

        await h.c.read(sessionProvider.notifier).logout();

        expect(h.deleted.single.token, 'fcm-1');
        expect(h.auth.calls, ['deleteDeviceToken', 'clear']);
        expect(h.c.read(sessionProvider).value, isNull);

        await h.c.read(sessionProvider.notifier).logout();
        expect(h.deleted, hasLength(1));
      },
    );

    test('強制登出（session 已失效，註銷收到 401）→ 照樣清除，不再觸發強制登出', () async {
      final h = await restore(deleteFails: true);
      manager.seedLifecycleStateForTest(
        fcmToken: 'fcm-1',
        isUserLoggedIn: true,
      );

      await h.c.read(sessionProvider.notifier).logout();

      expect(h.deleted.single.token, 'fcm-1');
      expect(h.c.read(forceLogoutProvider), isFalse);
      expect(h.c.read(sessionProvider).value, isNull);
    });

    test('沒有登入過 → 不送註銷', () async {
      final h = await restore();
      manager.seedLifecycleStateForTest(fcmToken: 'fcm-1');

      await h.c.read(sessionProvider.notifier).logout();

      expect(h.deleted, isEmpty);
    });
  });
}
