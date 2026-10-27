import 'package:hawkr_auth/clerk.dart';
import 'dart:convert';
import 'dart:io';

import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/session/clerk_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/core/session/session_provider.dart';
import 'package:hawkr_seller/features/chat/data/chat_repository.dart';
import 'package:dio/dio.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/test.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../support/fake_clerk_session.dart';

/// 401 代表 Clerk 的 session 已結束：直接登出，不重送。
///
/// 登出流程自己會發請求（裝置註銷），那個請求可能 401——若照常登出，logout
/// 又會再發一次同樣的請求。帶 [kSkipAuthInterceptorExtra] 的請求必須在**兩個**
/// 401 分支都提前返回：`onResponse`（validateStatus 放行 401）與 `onError`
/// （Dio 對 401 拋例外）。只補一邊等於沒修。
void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async {
    if (!F.isInitialized) F.init(Flavor.dev);
    setupFirebaseCoreMocks();
    await Firebase.initializeApp();
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

  /// 回傳已登入的 container：401 一旦被攔截器接手就登出 → `isLoggedIn`
  /// 變成 false、Clerk 登出，成為可觀測訊號。
  Future<(ProviderContainer, Dio, _Unauthorized401Adapter, FakeClerkSession)>
  buildContainer() async {
    final clerk = FakeClerkSession(signedIn: true, tokenValue: 'session-token');
    final container = ProviderContainer(
      overrides: [
        chatRepositoryProvider.overrideWithValue(_NoopChatRepository()),
        postLoginSideEffectsProvider.overrideWithValue(
          const NoopPostLoginSideEffects(),
        ),
        clerkSessionProvider.overrideWith((ref) async => clerk),
      ],
    );
    addTearDown(container.dispose);

    final adapter = _Unauthorized401Adapter();
    final dio = container.read(dioProvider)..httpClientAdapter = adapter;
    container.read(sessionProvider.notifier).state = const SessionModel(
      accountId: 'acc-1',
      isLoggedIn: true,
    );
    return (container, dio, adapter, clerk);
  }

  test('onResponse 分支：帶旗標的 401 不登出', () async {
    final (container, dio, adapter, _) = await buildContainer();

    final response = await dio.delete<dynamic>(
      '/api/v1/app/notifications/device-token',
      data: const {'token': 'device-token'},
      options: Options(
        extra: const {kSkipAuthInterceptorExtra: true},
        validateStatus: (_) => true,
      ),
    );

    expect(response.statusCode, 401);
    expect(adapter.calls, 1);
    expect(container.read(sessionProvider).isLoggedIn, isTrue);
  });

  test('onError 分支：帶旗標的 401 不登出', () async {
    final (container, dio, adapter, _) = await buildContainer();

    await expectLater(
      dio.delete<dynamic>(
        '/api/v1/app/notifications/device-token',
        data: const {'token': 'device-token'},
        options: Options(extra: const {kSkipAuthInterceptorExtra: true}),
      ),
      throwsA(
        isA<DioException>().having(
          (e) => e.response?.statusCode,
          'statusCode',
          401,
        ),
      ),
    );

    expect(adapter.calls, 1);
    expect(container.read(sessionProvider).isLoggedIn, isTrue);
  });

  /// 這個旗標是「請把 401 當成預期結果」——用錯地方的代價是：真正失效的
  /// session 不再觸發登出，使用者卡在一個每支 API 都 401 的死
  /// session 裡，而且沒有任何測試會失敗。它只有一個正當用途（登出流程中的
  /// 裝置註銷），所以用「哪些檔可以出現它」把使用面鎖死。App 版本 gate 改走
  /// Connect 的公開 service（不經過 Dio、不帶身分），不再需要它。
  test('架構 gate：kSkipAuthInterceptorExtra 只准出現在既定的路徑', () {
    const declaration = 'lib/core/networking/dio_provider.dart';
    const deviceRegistrar =
        'lib/features/notification/data/push/device_registrar.dart';
    const allowed = {declaration, deviceRegistrar};

    final users = <String>[];
    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final source = entity.readAsStringSync();
      if (source.contains('kSkipAuthInterceptorExtra') ||
          source.contains("'skipAuthInterceptor'")) {
        users.add(entity.path);
      }
    }

    expect(
      users.toSet().difference(allowed),
      isEmpty,
      reason:
          '新的使用點會讓該處的 401 不再回收 session。要新增請先確認那條路徑'
          '「本來就不預期帶得出有效憑證」，再把檔案加進本 gate 的白名單。',
    );
    // 白名單本身不得因改名而變成空 gate。
    expect(users.toSet(), allowed);
  });

  test('沒帶旗標的 401（onResponse）：直接登出並登出 Clerk，不重送', () async {
    final (container, dio, adapter, clerk) = await buildContainer();

    await dio.get<dynamic>(
      '/api/v1/app/chat/unread',
      options: Options(validateStatus: (_) => true),
    );

    expect(adapter.calls, 1);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
    expect(clerk.calls, ['signOut']);
  });

  test('沒帶旗標的 401（onError）：直接登出，不重送', () async {
    final (container, dio, adapter, _) = await buildContainer();

    await expectLater(
      dio.get<dynamic>('/api/v1/app/chat/unread'),
      throwsA(isA<DioException>()),
    );

    expect(adapter.calls, 1);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
  });

  test('Clerk refused 不送 REST，也會登出', () async {
    final (container, dio, adapter, clerk) = await buildContainer();
    clerk.tokenError = const SessionTokenRefused('revoked');
    await expectLater(
      dio.get<dynamic>('/app/chat/unread'),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          DioExceptionType.cancel,
        ),
      ),
    );
    await Future<void>.delayed(const Duration(milliseconds: 50));
    expect(adapter.lastHeaders, isNull);
    expect(clerk.isSignedIn, isFalse);
    expect(container.read(sessionProvider).isLoggedIn, isFalse);
  });

  test('取不到 token（連不上 Clerk）：不送出，以連線錯誤結束，不登出', () async {
    final (container, dio, adapter, clerk) = await buildContainer();
    clerk.tokenError = const SocketException('offline');

    await expectLater(
      dio.get<dynamic>('/api/v1/app/chat/unread'),
      throwsA(
        isA<DioException>().having(
          (e) => e.type,
          'type',
          DioExceptionType.connectionError,
        ),
      ),
    );

    expect(adapter.calls, 0);
    expect(container.read(sessionProvider).isLoggedIn, isTrue);
  });

  test('每個請求帶 X-Active-Role: SELLER 與 Clerk 的 session token', () async {
    final (_, dio, adapter, _) = await buildContainer();

    await dio.get<dynamic>(
      '/api/v1/app/orders',
      options: Options(
        extra: const {kSkipAuthInterceptorExtra: true},
        validateStatus: (_) => true,
      ),
    );

    expect(adapter.lastHeaders?['X-Active-Role'], 'SELLER');
    expect(adapter.lastHeaders?['Authorization'], 'Bearer session-token');
  });
}

class _Unauthorized401Adapter implements HttpClientAdapter {
  int calls = 0;
  Map<String, dynamic>? lastHeaders;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    calls++;
    lastHeaders = options.headers;
    return ResponseBody.fromString(
      jsonEncode({'code': 401, 'message': 'unauthorized'}),
      401,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}

class _NoopChatRepository extends ChatRepository {
  _NoopChatRepository() : super(Dio());

  @override
  void stopPolling() {}

  @override
  void startPolling({Duration interval = const Duration(seconds: 1)}) {}

  @override
  void useHttpClient(Dio dio) {}
}
