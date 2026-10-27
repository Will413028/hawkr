import 'dart:convert';

import 'dart:async';

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/session/clerk_session_provider.dart';
import 'package:hawkr_seller/core/session/post_login_side_effects.dart';
import 'package:hawkr_seller/features/auth/data/auth_repository.dart';
import 'package:hawkr_seller/features/seller/data/current_store.dart';
import 'package:hawkr_seller/features/seller/data/my_store_repository.dart';
import 'package:hawkr_auth/oauth.dart';
import 'package:dio/dio.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_riverpod/misc.dart' show Override;
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../support/fake_clerk_session.dart';
import '../../../support/fake_my_store.dart';
import '../../../support/my_store_fixtures.dart';

/// 建立已 override 四個 seam（picker、repository、Dio、Clerk）+ no-op 副作用的測試 container。[clerk] 沒有給時
/// 是一個登出的 [FakeClerkSession]。
///
/// 登入後的 `GetMyStore` 由假的 Connect 伺服器回答：[needsOnboarding] 為 true
/// 時回 not_found（沒有店家），否則回已建店的賣家（storeId [kSellerId]）；
/// [meFails] 時回 unavailable。[meGate] 不是 null 時，GetMyStore 等它完成才回答。
///
/// caller 負責先呼叫 [resetLoginStorage]（或在 setUp 中）以確保
/// SharedPreferences mock 乾淨。container 自動於測試結束 dispose。
ProviderContainer makeLoginContainer({
  required OAuthPicker picker,
  required AuthRepository repo,
  FakeClerkSession? clerk,
  bool needsOnboarding = false,
  bool meFails = false,
  Future<void>? meGate,
}) {
  final fakeClerk = clerk ?? FakeClerkSession();
  final container = ProviderContainer(
    overrides: [
      clerkSessionProvider.overrideWith((ref) async => fakeClerk),
      oauthPickerProvider.overrideWithValue(picker),
      authRepositoryProvider.overrideWithValue(repo),
      ...myStoreOverrides(
        needsOnboarding: needsOnboarding,
        meFails: meFails,
        meGate: meGate,
      ),
      postLoginSideEffectsProvider.overrideWithValue(
        const NoopPostLoginSideEffects(),
      ),
    ],
  );
  addTearDown(container.dispose);
  final store = container.listen(currentStoreProvider, (_, _) {});
  addTearDown(store.close);
  return container;
}

/// 回答 `GetMyStore` 的假伺服器（參數同 [makeLoginContainer]），加上不回任何資料的 Dio（其他 REST 呼叫不應該發生）。
List<Override> myStoreOverrides({
  bool needsOnboarding = false,
  bool meFails = false,
  Future<void>? meGate,
}) {
  final server = meFails
      ? FakeMyStoreServer.unavailable()
      : needsOnboarding
      ? FakeMyStoreServer.absent()
      : FakeMyStoreServer(response: myStoreResponse(id: kSellerId.toString()));
  server.gate = meGate;
  final dio = Dio(BaseOptions(baseUrl: 'https://example.test'))
    ..httpClientAdapter = _InertAdapter();
  addTearDown(dio.close);
  return [
    myStoreRepositoryProvider.overrideWithValue(server.repository),
    dioProvider.overrideWithValue(dio),
  ];
}

/// 冷啟動從 storage 恢復 session 後同步賣家（同 production 的 `onSessionRestored`），
/// 不做 chat polling 與 FCM。
class SyncSellerOnRestore implements PostLoginSideEffects {
  SyncSellerOnRestore(this._ref);

  final Ref _ref;

  @override
  void onLogin() {}

  @override
  Future<void> onSessionRestored() async {
    await _ref.read(currentStoreProvider.future);
  }
}

/// [makeLoginContainer] 的 `GetMyStore` 回的 storeId。
const int kSellerId = 2619961976;

/// 重設 SharedPreferences mock backing（同一測試 process 內的持久層）。
void resetLoginStorage([Map<String, Object> initial = const {}]) {
  SharedPreferences.setMockInitialValues(initial);
}

/// 以記憶體 map 頂替 flutter_secure_storage 的 method channel，回傳那個 map；測試直接讀寫它。在
/// `setUpAll` 呼叫一次，`setUp` 裡 `clear()`。
Map<String, String> mockSecureStorage() {
  final store = <String, String>{};
  TestDefaultBinaryMessengerBinding.instance.defaultBinaryMessenger
      .setMockMethodCallHandler(
        const MethodChannel('plugins.it_nomads.com/flutter_secure_storage'),
        (call) async {
          final args = (call.arguments as Map?)?.cast<String, dynamic>() ?? {};
          final key = args['key'] as String?;
          switch (call.method) {
            case 'write':
              store[key!] = args['value'] as String;
            case 'read':
              return store[key];
            case 'delete':
              store.remove(key);
            case 'readAll':
              return Map<String, String>.from(store);
            case 'deleteAll':
              store.clear();
            case 'containsKey':
              return store.containsKey(key);
          }
          return null;
        },
      );
  return store;
}

class _InertAdapter implements HttpClientAdapter {
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
