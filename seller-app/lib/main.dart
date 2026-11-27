import 'dart:async';

import 'package:firebase_analytics/firebase_analytics.dart';
import 'package:firebase_core/firebase_core.dart';
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
import 'core/session/auth_session_provider.dart';
import 'package:hawkr_auth/logging.dart';
import 'features/app_update/application/force_update_provider.dart';
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
  flavor.requireCoreConfig();

  SentryFlutter.init(
    (options) {
      options.dsn = flavor.sentryDsn;
      options.environment = flavor.name;
      options.tracesSampleRate = 0;
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
  if (kDebugMode && _e2eModeSeller) {
    PushNotificationManager.e2eDisabled = true;
  }
  if (e2eAuthEnabled) {
    HKLog.logI('[E2E] auth token injected → bypass OAuth');
  }

  await initAppVersionInfo();

  F.init(_resolveFlavorFromEnv());
  HKLog.logI('[Main] Flavor: ${F.appFlavor}');
  HKLog.logI('[Main] Current Flavor: ${F.appFlavor.name}');
  HKLog.logD(F.appFlavor.environmentInfo);

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

  try {
    await container.read(forceUpdateGateProvider.notifier).checkOnStartup();
  } catch (e, stackTrace) {
    HKLog.logE('[Main] App 版本檢查失敗，fail-open 放行', e, stackTrace);
  }

  if (!_e2eModeSeller && isFirebaseInitialized) {
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
  }
}
