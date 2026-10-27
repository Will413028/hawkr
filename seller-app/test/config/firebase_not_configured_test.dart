import 'package:hawkr_seller/config/firebase_config.dart';
import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/core/analytics/analytics_providers.dart';
import 'package:hawkr_seller/features/notification/data/push/manager.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_core_platform_interface/firebase_core_platform_interface.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

/// 假平台：沒有任何 app，並記錄 [Firebase.initializeApp] 與 [Firebase.app] 的呼叫。
/// 真實平台在沒有 app 時 `Firebase.app()` 會拋 `core/no-app`，這裡同樣照做。
class _RecordingFirebasePlatform extends FirebasePlatform {
  int initializeCalls = 0;
  int appLookups = 0;

  @override
  Future<FirebaseAppPlatform> initializeApp({
    String? name,
    FirebaseOptions? options,
  }) async {
    initializeCalls++;
    throw StateError('Firebase.initializeApp must not be called');
  }

  @override
  FirebaseAppPlatform app([String name = defaultFirebaseAppName]) {
    appLookups++;
    throw FirebaseException(plugin: 'core', code: 'no-app');
  }

  @override
  List<FirebaseAppPlatform> get apps => const [];
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  late _RecordingFirebasePlatform platform;

  setUp(() {
    platform = _RecordingFirebasePlatform();
    Firebase.delegatePackingProperty = platform;
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  tearDown(() {
    Firebase.delegatePackingProperty = null;
    PushNotificationManager.instance.seedLifecycleStateForTest();
  });

  group('buildFirebaseOptions', () {
    FirebaseOptions? build({
      String apiKey = 'k',
      String appId = 'a',
      String senderId = 's',
      String projectId = 'p',
    }) => buildFirebaseOptions(
      apiKey: apiKey,
      appId: appId,
      messagingSenderId: senderId,
      projectId: projectId,
    );

    test('四個必填欄位齊全就組出設定', () {
      final options = build();
      expect(options, isNotNull);
      expect(options!.projectId, 'p');
      expect(options.storageBucket, isNull);
    });

    test('任一必填欄位為空白就是沒有設定', () {
      expect(build(apiKey: ''), isNull);
      expect(build(appId: ''), isNull);
      expect(build(senderId: ' '), isNull);
      expect(build(projectId: ''), isNull);
    });

    test('沒有帶 FIREBASE_* define 時 flavor 的設定為 null', () {
      for (final flavor in Flavor.values) {
        expect(flavor.firebaseOptions, isNull, reason: flavor.name);
        expect(flavor.firebaseConfigured, isFalse, reason: flavor.name);
      }
    });
  });

  group('Firebase 未設定', () {
    test('initializeFirebase 不初始化任何 app', () async {
      final ready = await initializeFirebase(options: null, appName: 'unused');

      expect(ready, isFalse);
      expect(platform.initializeCalls, 0);
      expect(isFirebaseInitialized, isFalse);
    });

    test('推播管理器各入口走停用路徑，不碰 Firebase 也不拋例外', () async {
      final manager = PushNotificationManager.instance;

      await manager.init(requestPermissionOnInit: false);
      expect(
        await manager.requestPermission(),
        AuthorizationStatus.notDetermined,
      );
      expect(await manager.getToken(), isNull);
      await manager.onUserLoggedIn();
      await manager.onAppResumed();

      expect(platform.appLookups, 0);
      expect(platform.initializeCalls, 0);
    });

    test('分析服務停用且不取 FirebaseAnalytics.instance', () async {
      final container = ProviderContainer();
      addTearDown(container.dispose);

      final service = container.read(analyticsServiceProvider);
      await service.logEvent('any', const {});
      await service.setUserId('u');

      expect(service.enabled, isFalse);
      expect(platform.appLookups, 0);
    });
  });
}
