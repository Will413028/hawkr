import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/firebase_core_platform_interface.dart';
import 'package:flutter_test/flutter_test.dart';

/// 用純 Dart 假物件頂替 `Firebase.app()`。
///
/// `PushNotificationManager`（單例）的欄位初始化式是
/// `final FirebaseMessaging _messaging = FirebaseMessaging.instance;`，
/// 而 `FirebaseMessaging.instance` 內部同步呼叫 `Firebase.app()`。測試環境
/// 沒有原生實作、沒跑過 `Firebase.initializeApp()`，第一次觸碰
/// `PushNotificationManager.instance` 就會丟
/// `[core/no-app] No Firebase App '[DEFAULT]' has been created`。
///
/// 這裡只需要滿足 `Firebase.app()` 回傳一個合法的 [FirebaseAppPlatform]——
/// 測試的 `Session` 路徑在 `PushNotificationManager.e2eDisabled` 下不會
/// 再碰任何真的 FirebaseMessaging 方法，不需要 mock 訊息通道。
void mockFirebaseCore() {
  Firebase.delegatePackingProperty = _FakeFirebasePlatform();
  // Firebase.delegatePackingProperty 是全域靜態欄位，跨測試檔存活（同一
  // isolate）。今天這個檔案只有一個測試用到它，但往後同檔若加測試、且刻意
  // 不需要 Firebase 的案例，沒有這行會悄悄繼承前一個測試設下的假物件。
  addTearDown(() => Firebase.delegatePackingProperty = null);
}

class _FakeFirebasePlatform extends FirebasePlatform {
  final FirebaseAppPlatform _app = FirebaseAppPlatform(
    defaultFirebaseAppName,
    const FirebaseOptions(
      apiKey: 'fake-api-key',
      appId: 'fake-app-id',
      messagingSenderId: 'fake-sender-id',
      projectId: 'fake-project-id',
    ),
  );

  @override
  FirebaseAppPlatform app([String name = defaultFirebaseAppName]) => _app;

  @override
  List<FirebaseAppPlatform> get apps => [_app];
}
