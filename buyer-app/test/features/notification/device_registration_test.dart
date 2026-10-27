import 'dart:io';

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/clerk_session_provider.dart';
import 'package:hawkr_buyer/core/session/force_logout_provider.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/auth/data/auth_repository.dart';
import 'package:hawkr_buyer/features/notification/data/push/manager.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_clerk_session.dart';
import '../../support/firebase_core_mock.dart';
import '../../support/secure_storage_mock.dart';

const _devicesPath = '/api/v1/app/notifications/device-token';

class _AccountRepository extends Fake implements AuthRepository {
  @override
  Future<String> currentAccountId() async => 'acc_01J8Z';
}

/// 裝置註冊走中央 Dio：憑證只由 auth interceptor 從 Clerk 帶上，manager 不持有 session token。
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

  /// 以登入中的 Clerk 與 GetSession 還原 session；裝置註冊的請求回 [deviceStatus]。
  Future<
    ({ProviderContainer c, List<RequestOptions> sent, FakeClerkSession clerk})
  >
  restore({int deviceStatus = HttpStatus.ok}) async {
    mockSecureStorage({});
    final clerk = FakeClerkSession(signedIn: true);
    final container = ProviderContainer(
      overrides: [
        clerkSessionProvider.overrideWith((ref) async => clerk),
        authRepositoryProvider.overrideWith((ref) => _AccountRepository()),
      ],
    );
    addTearDown(container.dispose);
    final sent = <RequestOptions>[];
    container.read(dioProvider).httpClientAdapter = _RecordingAdapter(
      deviceStatus,
      sent,
    );
    manager.injectContainer(container);
    await container.read(sessionProvider.future);
    return (c: container, sent: sent, clerk: clerk);
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

    final register = h.sent.single;
    expect(register.method, 'POST');
    expect(register.uri.path, _devicesPath);
    expect(register.headers['Authorization'], 'Bearer live');
    expect(register.data, {'token': 'fcm-1', 'platform': 'ios'});
  });

  group('登出時註銷裝置', () {
    setUp(() {
      // 冷啟動不做 FCM；登入狀態與 FCM token 由下面各測試直接給。
      PushNotificationManager.e2eDisabled = true;
      addTearDown(() => PushNotificationManager.e2eDisabled = false);
    });

    test('先以登入中的 Clerk token 註銷，再登出 Clerk、清除 session；第二次登出不再送', () async {
      final h = await restore();
      manager.seedLifecycleStateForTest(
        fcmToken: 'fcm-1',
        isUserLoggedIn: true,
      );

      await h.c.read(sessionProvider.notifier).logout();

      final remove = h.sent.single;
      expect(remove.method, 'DELETE');
      expect(remove.uri.path, _devicesPath);
      expect(remove.headers['Authorization'], 'Bearer live');
      expect(remove.data, {'token': 'fcm-1'});
      expect(h.clerk.calls, ['signOut']);
      expect(h.c.read(sessionProvider).value, isNull);

      await h.c.read(sessionProvider.notifier).logout();
      expect(h.sent, hasLength(1));
    });

    test('強制登出（session 已失效，註銷收到 401）→ 照樣清除，不再觸發強制登出', () async {
      final h = await restore(deviceStatus: HttpStatus.unauthorized);
      manager.seedLifecycleStateForTest(
        fcmToken: 'fcm-1',
        isUserLoggedIn: true,
      );

      await h.c.read(sessionProvider.notifier).logout();

      expect(h.sent.single.method, 'DELETE');
      expect(h.c.read(forceLogoutProvider), isFalse);
      expect(h.c.read(sessionProvider).value, isNull);
    });

    test('沒有登入過 → 不送註銷', () async {
      final h = await restore();
      manager.seedLifecycleStateForTest(fcmToken: 'fcm-1');

      await h.c.read(sessionProvider.notifier).logout();

      expect(h.sent, isEmpty);
    });
  });
}

class _RecordingAdapter implements HttpClientAdapter {
  _RecordingAdapter(this.status, this.sent);

  final int status;
  final List<RequestOptions> sent;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<List<int>>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    sent.add(options);
    return ResponseBody.fromString(
      '{}',
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
