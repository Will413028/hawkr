import 'dart:async';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:sentry_flutter/sentry_flutter.dart';

import 'app.dart';
import 'config/firebase_config.dart';
import 'config/flavor.dart';
import 'core/analytics/analytics_providers.dart';
import 'core/layout/app_text_size.dart';
import 'core/networking/app_version_headers.dart';
import 'core/observability/sentry_scrub.dart';
import 'core/session/clerk_session_provider.dart';
import 'package:hawkr_auth/logging.dart';
import 'features/app_update/application/force_update_provider.dart';
import 'features/notification/data/push/background_handler.dart';
import 'features/notification/data/push/manager.dart';
import 'shared/utils/hk_log.dart';
import 'shared/widgets/brand_startup_splash.dart';

const bool _e2eSuppressPromptsSeller = bool.fromEnvironment(
  'E2E_SUPPRESS_PROMPTS',
  defaultValue: false,
);
bool get _e2eModeSeller =>
    _e2eSuppressPromptsSeller || kE2EAuthToken.isNotEmpty;

/// 讀取 flavor：native 用 FLUTTER_APP_FLAVOR（由 --flavor 自動設定），
/// fallback 到 FLUTTER_FLAVOR（由 --dart-define=FLUTTER_FLAVOR=xxx 設定），
/// 最後 fallback 到 APP_FLAVOR（web），預設 dev
const String appFlavor = String.fromEnvironment(
  'FLUTTER_APP_FLAVOR',
  defaultValue: String.fromEnvironment(
    'FLUTTER_FLAVOR',
    defaultValue: String.fromEnvironment('APP_FLAVOR', defaultValue: 'dev'),
  ),
);

/// main() 與 _bootstrapApplication 共用同一份 flavor 解析邏輯。
Flavor _resolveFlavorFromEnv() =>
    Flavor.values.firstWhere((element) => element.name == appFlavor);

void main() {
  WidgetsFlutterBinding.ensureInitialized();

  final flavor = _resolveFlavorFromEnv();
  // API 主機是必要設定：stage 與 prod 缺少 GATEWAY_HOST 時在 runApp 前直接失敗。
  flavor.requireCoreConfig();

  SentryFlutter.init(
    (options) {
      options.dsn = flavor.sentryDsn;
      options.environment = flavor.name;
      options.tracesSampleRate = 0; // error-only rollout；不開 performance tracing
      options.sendDefaultPii = false;
      options.beforeSend = (event, hint) => scrubSentryEvent(event, hint);
    },
    appRunner: () => runApp(
      BrandStartupGate(
        bootstrap: _bootstrapApplication,
        onComplete: _finishNotificationStartup,
      ),
    ),
  );
}

void _finishNotificationStartup() {
  if (_e2eModeSeller) return;
  unawaited(_requestNotificationPermissionAndToken());
}

Future<void> _requestNotificationPermissionAndToken() async {
  final manager = PushNotificationManager.instance;
  await manager.requestPermission();
  await manager.getToken();
}

Future<Widget> _bootstrapApplication() async {
  // ============ E2E auth inject（debug build only）============
  // `--dart-define=E2E_AUTH_TOKEN=<token>` 以固定的 Bearer 取代 Clerk 的 session token（見
  // clerk_session_provider.dart），GetSession 讀取對應的帳號。release build 不讀它。
  if (kDebugMode && _e2eModeSeller) {
    PushNotificationManager.e2eDisabled = true;
  }
  if (e2eAuthEnabled) {
    HKLog.logI('[E2E] auth token injected → bypass OAuth');
  }

  // bare semver 快取一次供同步讀取（Dio BaseOptions 的 header 與版本 gate
  // 都要用，`packages/hawkr_app_update/docs/spec.md`）。必須早於任何 Dio instance 建立。
  await initAppVersionInfo();

  // 從 --dart-define=FLUTTER_FLAVOR=xxx 讀取環境配置
  F.init(_resolveFlavorFromEnv());
  HKLog.logI('[Main] Flavor: ${F.appFlavor}');
  HKLog.logI('[Main] Current Flavor: ${F.appFlavor.name}');
  HKLog.logD(F.appFlavor.environmentInfo);

  // Firebase 沒有設定（缺 FIREBASE_* define）時整體停用：不初始化、不註冊推播與分析，
  // App 仍可啟動與登入。
  try {
    final firebaseReady = await initializeFirebase(
      options: F.appFlavor.firebaseOptions,
      appName: F.appFlavor.firebaseAppName,
    );
    if (firebaseReady) {
      HKLog.logI('Firebase 初始化成功 - App: ${Firebase.app().options.projectId}');
    } else {
      HKLog.logW('Firebase 未設定，推播與分析停用');
    }
  } catch (e, stackTrace) {
    HKLog.logE('❌ Firebase 初始化失敗', e, stackTrace);
    rethrow;
  }

  // GA4: prod + dev 啟用 analytics 收集（stage 不用）。各 flavor 連各自的
  // Firebase 專案 → 各自的 GA property，dev 不會污染 prod 報表。複用單一
  // predicate，與 analyticsServiceProvider 的 enabled gate 同步，避免 SDK
  // master switch 與 app 層 gate 不一致。
  if (isFirebaseInitialized) {
    await FirebaseAnalytics.instance.setAnalyticsCollectionEnabled(
      analyticsEnabledForFlavor(F.appFlavor),
    );
  }

  final container = ProviderContainer(
    overrides: [
      authLoggerProvider.overrideWithValue((level, message, error, stack) {
        if (level == AuthLogLevel.error) {
          HKLog.logE(message, error, stack);
        } else {
          HKLog.logW(message, error, stack);
        }
      }),
    ],
  );
  await container.read(appTextSizeProvider.notifier).restore();
  PushNotificationManager.instance.injectContainer(container);

  // App 版本 gate（`packages/hawkr_app_update/docs/spec.md`）：**await** 完成才進路由，否則會先進
  // 首頁再彈更新畫面，出現可操作空窗。位置刻意早於下面的推播初始化——
  // 通知導頁在 gate 攔截時還沒 arm，不需另建抑制機制。
  //
  // ⚠️ 例外必須在這裡自己吃掉：`BrandStartupGate._runBootstrap` 的 catch 只
  // reportError、不 setState 也不解除 splash（`brand_startup_splash.dart:75-84`），
  // 例外冒上去就是永遠卡在啟動畫面。fail-open 的具體落地點就是這個 try。
  try {
    await container.read(forceUpdateGateProvider.notifier).checkOnStartup();
  } catch (e, stackTrace) {
    HKLog.logE('[Main] App 版本檢查失敗，fail-open 放行', e, stackTrace);
  }

  if (!_e2eModeSeller && isFirebaseInitialized) {
    // 註冊背景訊息處理器（必須在 Firebase 初始化之後）
    FirebaseMessaging.onBackgroundMessage(firebaseMessagingBackgroundHandler);
    // 初始化 Firebase Messaging
    await _initializeFirebaseMessaging();
  }

  return UncontrolledProviderScope(container: container, child: const MyApp());
}

/// 初始化 Firebase Messaging
Future<void> _initializeFirebaseMessaging() async {
  try {
    await PushNotificationManager.instance.init(requestPermissionOnInit: false);
    HKLog.logI('Firebase Messaging 初始化成功');
  } catch (e, stackTrace) {
    HKLog.logE('Firebase Messaging 初始化失敗', e, stackTrace);
    // Messaging 失敗不阻止 app 運行
  }
}
