import 'dart:async';
import 'dart:convert';

import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/session/auth_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:hawkr_seller/features/notification/data/push/device_registrar.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_seller/features/notification/data/push/manager.dart';
import 'package:hawkr_seller/features/photo_editor/data/ai_edit_history_local_service.dart';
import 'package:hawkr_seller/features/photo_editor/data/ai_edit_history_model.dart';
import 'package:dio/dio.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_auth_session.dart';
import '../../support/fake_my_store.dart';
import '../../support/my_store_fixtures.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  /// hawkr_seller#106：登出自己會發 API 請求（裝置註銷走中央 dio），該
  /// 請求 401 後又會回頭呼叫 logout，於是 `logout → 401 → logout` 自我循環，
  /// dev 環境 9 小時打了 14 萬次。守衛在 [Session.logout] 的 `_loggingOut`
  /// 旗標——它是目前唯一擋住這場風暴的東西，這兩個測試是它的回歸防線。
  group('Session.logout 可重入保護', () {
    setUpAll(() async {
      if (!F.isInitialized) F.init(Flavor.dev);

      // logout() → PushNotificationManager.instance 的欄位初始化會呼叫
      // FirebaseMessaging.instance（→ Firebase.app()），沒有註冊 app 會拋
      // `core/no-app`。
      setupFirebaseCoreMocks();
      await Firebase.initializeApp();

      // AuthStorage 走 FlutterSecureStorage → native MethodChannel。
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
            const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
            (call) async {
              switch (call.method) {
                case 'readAll':
                  return <String, String>{};
                case 'containsKey':
                  return false;
                default:
                  return null;
              }
            },
          );
    });

    setUp(() {
      SharedPreferences.setMockInitialValues({});
    });

    ProviderContainer containerWith(
      _CountingChatRepository chat, {
      FakeAuthSession? auth,
      List<String>? events,
    }) {
      final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
        ..httpClientAdapter = _OkAdapter();
      final container = ProviderContainer(
        overrides: [
          dioProvider.overrideWithValue(dio),
          myStoreRepositoryProvider.overrideWithValue(
            FakeMyStoreServer().repository,
          ),
          chatRepositoryProvider.overrideWithValue(chat),
          postLoginSideEffectsProvider.overrideWithValue(
            const NoopPostLoginSideEffects(),
          ),
          authSessionProvider.overrideWith(
            (ref) async => auth ?? FakeAuthSession(signedIn: true),
          ),
          authRepositoryProvider.overrideWithValue(
            _SignOutRecorder(events ?? <String>[]),
          ),
        ],
      );
      addTearDown(container.dispose);
      addTearDown(dio.close);
      return container;
    }

    test('並行的第二次 logout 直接返回，登出副作用只執行一次', () async {
      final chat = _CountingChatRepository();
      final container = containerWith(chat);
      final notifier = container.read(sessionProvider.notifier);
      await notifier.login(accountId: 'acc-1');

      // 第二次呼叫落在第一次的第一個 await 之前 → 必須被旗標擋下。
      final first = notifier.logout();
      final second = notifier.logout();
      await Future.wait([first, second]);

      expect(chat.stopPollingCalls, 1);
      expect(container.read(sessionProvider).isLoggedIn, isFalse);
    });

    test('登出流程中被重入觸發的 logout 不會遞迴，副作用仍只執行一次', () async {
      final chat = _CountingChatRepository();
      final container = containerWith(chat);
      final notifier = container.read(sessionProvider.notifier);
      // 模擬 401 interceptor：在第一輪登出的副作用執行途中再次呼叫 logout。
      // 上限 5 次只是避免守衛失效時測試無限跑，斷言仍是「恰好一次」。
      chat.onStopPolling = () {
        if (chat.stopPollingCalls < 5) unawaited(notifier.logout());
      };
      await notifier.login(accountId: 'acc-1');

      await notifier.logout();
      await Future<void>.delayed(Duration.zero);

      expect(chat.stopPollingCalls, 1);
      expect(container.read(sessionProvider).isLoggedIn, isFalse);
    });

    /// 迴圈的每一輪都是「state 已清空後，某個 401 再觸發一次 logout」——
    /// `_loggingOut` 只擋同一輪重入，擋不住下一輪。擋住下一輪的是這裡：沒有
    /// 登入就不要送那個注定 401、注定再觸發 logout 的 DELETE。
    test('沒有登入時登出不送裝置註銷（#106 迴圈的下一輪防線）', () async {
      final gateway = _RecordingDeviceGateway();
      PushNotificationManager.instance.seedLifecycleStateForTest(
        registration: DeviceRegistrationCoordinator(
          gateway: gateway,
          retryDelays: const [],
        ),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );
      addTearDown(
        () => PushNotificationManager.instance.seedLifecycleStateForTest(),
      );
      final container = containerWith(_CountingChatRepository());

      // 沒有 login()：session 沒有登入，等同「state 已被清空後再被觸發」。
      await container.read(sessionProvider.notifier).logout();

      expect(gateway.removeCalls, isEmpty);
    });

    test('登入中登出：先送出裝置註銷一次，再登出 session token（對照組，確保上一條不是空測）', () async {
      final events = <String>[];
      final gateway = _RecordingDeviceGateway(events);
      PushNotificationManager.instance.seedLifecycleStateForTest(
        registration: DeviceRegistrationCoordinator(
          gateway: gateway,
          retryDelays: const [],
        ),
        fcmToken: 'fcm-token',
        isUserLoggedIn: true,
      );
      addTearDown(
        () => PushNotificationManager.instance.seedLifecycleStateForTest(),
      );
      final container = containerWith(
        _CountingChatRepository(),
        auth: FakeAuthSession(signedIn: true, calls: events),
        events: events,
      );
      final notifier = container.read(sessionProvider.notifier);
      await notifier.login(accountId: 'acc-1');

      await notifier.logout();

      expect(gateway.removeCalls, ['fcm-token']);
      expect(events, ['removeDevice', 'serverSignOut', 'clear']);
    });

    test('登出時登出 session token，再清除本機', () async {
      final auth = FakeAuthSession(signedIn: true);
      final container = containerWith(_CountingChatRepository(), auth: auth);
      final notifier = container.read(sessionProvider.notifier);

      await notifier.login(accountId: 'acc-1');
      await notifier.logout();

      expect(auth.calls, ['clear']);
      expect(container.read(sessionProvider).isLoggedIn, isFalse);
    });

    test('登出後可以再次登入並正常登出（旗標有復位）', () async {
      final chat = _CountingChatRepository();
      final container = containerWith(chat);
      final notifier = container.read(sessionProvider.notifier);

      await notifier.login(accountId: 'acc-1');
      await notifier.logout();
      await notifier.login(accountId: 'acc-2');
      await notifier.logout();

      expect(chat.stopPollingCalls, 2);
      expect(container.read(sessionProvider).isLoggedIn, isFalse);
    });

    /// hawkr_seller#115：AI 修圖歷史本地快取無帳號 scope，同裝置換
    /// 帳號（含刪店重開店，該流程本身就是強制 logout+重新 login）會讓
    /// 下一位使用者看到上一位的修圖動作名稱/次數/時間。
    test('登出流程一併清除 AI 修圖歷史（#115）', () async {
      final container = containerWith(_CountingChatRepository());
      final historyService = container.read(aiEditHistoryLocalServiceProvider);
      await historyService.save([
        AiEditHistoryEntry(
          action: '背景去除',
          timestamp: DateTime.utc(2026, 7, 27),
          count: 1,
        ),
      ]);
      expect(await historyService.load(), hasLength(1));

      final notifier = container.read(sessionProvider.notifier);
      await notifier.login(accountId: 'acc-1');
      await notifier.logout();

      expect(await historyService.load(), isEmpty);
    });
  });

  group('店家 ID 由店家推導、accountId 不被覆寫（issue #114）', () {
    setUpAll(() async {
      if (!F.isInitialized) F.init(Flavor.dev);
      if (Firebase.apps.isEmpty) {
        setupFirebaseCoreMocks();
        await Firebase.initializeApp();
      }
      TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
          .setMockMethodCallHandler(
            const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
            (call) async {
              switch (call.method) {
                case 'readAll':
                  return <String, String>{};
                case 'containsKey':
                  return false;
                default:
                  return null;
              }
            },
          );
    });

    setUp(() {
      SharedPreferences.setMockInitialValues({});
    });

    ProviderContainer containerWithStore(FakeMyStoreServer server) {
      final container = ProviderContainer(
        overrides: [
          myStoreRepositoryProvider.overrideWithValue(server.repository),
          chatRepositoryProvider.overrideWithValue(_CountingChatRepository()),
          postLoginSideEffectsProvider.overrideWithValue(
            const NoopPostLoginSideEffects(),
          ),
        ],
      );
      addTearDown(container.dispose);
      return container;
    }

    test('GetMyStore 回店家時 storeId 由店家推導，accountId 維持登入當下的真值不變', () async {
      final container = containerWithStore(
        FakeMyStoreServer(response: myStoreResponse(id: '555')),
      );
      final notifier = container.read(sessionProvider.notifier);
      await notifier.login(accountId: 'acc-111');
      await container.read(currentStoreProvider.future);

      expect(container.read(sellerStoreIdProvider), '555');
      expect(container.read(sessionProvider).accountId, 'acc-111');
    });

    test('GetMyStore 回的店家沒有 ID 時，storeId 是 null（不是空字串），accountId 不變', () async {
      final container = containerWithStore(
        FakeMyStoreServer(response: myStoreResponse(id: '')),
      );
      final notifier = container.read(sessionProvider.notifier);
      await notifier.login(accountId: 'acc-111');
      final result = await container.read(currentStoreProvider.future);

      expect(result!.failed, isFalse);
      expect(container.read(sellerStoreIdProvider), isNull);
      expect(container.read(sessionProvider).accountId, 'acc-111');
    });
  });
}

/// 記錄裝置註銷是否真的送到 gateway 的假實作。
/// 只記錄伺服器端的 `SignOut`。
class _SignOutRecorder extends Fake implements AuthRepository {
  _SignOutRecorder(this.events);

  final List<String> events;

  @override
  Future<void> signOut() async => events.add('serverSignOut');
}

class _RecordingDeviceGateway implements DeviceRegistrationGateway {
  _RecordingDeviceGateway([List<String>? events]) : _events = events ?? [];

  final List<String> removeCalls = [];
  final List<String> _events;

  @override
  Future<DeviceMutationOutcome> registerDevice({required String fcmToken}) =>
      Future.value(const DeviceMutationOutcome.success(statusCode: 200));

  @override
  Future<DeviceMutationOutcome> removeDevice({required String fcmToken}) {
    removeCalls.add(fcmToken);
    _events.add('removeDevice');
    return Future.value(const DeviceMutationOutcome.success(statusCode: 204));
  }
}

/// 只用來計次的 [ChatRepository]：`stopPolling` 是 logout 唯一可觀測、且
/// 每次登出必經一次的副作用。
class _CountingChatRepository extends ChatRepository {
  _CountingChatRepository() : super(Dio());

  int stopPollingCalls = 0;
  void Function()? onStopPolling;

  @override
  void stopPolling() {
    stopPollingCalls++;
    onStopPolling?.call();
  }

  @override
  void startPolling({Duration interval = const Duration(seconds: 1)}) {}

  @override
  void useHttpClient(Dio dio) {}
}

class _OkAdapter implements HttpClientAdapter {
  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    return ResponseBody.fromString(
      jsonEncode({'code': 0}),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
