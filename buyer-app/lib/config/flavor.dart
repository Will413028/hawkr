import 'package:firebase_core/firebase_core.dart' show FirebaseOptions;

import 'firebase_config.dart';

const String _gatewayHostDefine = String.fromEnvironment('GATEWAY_HOST');
const String _mediaBaseHostDefine = String.fromEnvironment('MEDIA_BASE_HOST');
const String _sentryDsnDefine = String.fromEnvironment('SENTRY_DSN');

/// dev flavor 沒有提供 define 時連本機單體。
const String _localGatewayHost = 'http://localhost:8080';

enum Flavor {
  /// 開發環境
  dev,

  /// 預發布環境
  stage,

  /// 生產環境
  prod,
}

/// 解析 API 主機：[define] 非空就用它；dev 沒有提供時連本機單體；
/// stage 與 prod 沒有提供時拋出 [StateError]，不組出相對路徑的 API URL。
String resolveGatewayHost(Flavor flavor, String define) {
  if (define.isNotEmpty) return define;
  if (flavor == Flavor.dev) return _localGatewayHost;
  throw StateError(
    '缺少必要設定 GATEWAY_HOST：${flavor.name} 建置必須帶 '
    '--dart-define=GATEWAY_HOST=<API 主機>（見 config/dart-defines.example.json）',
  );
}

/// 全域 Flavor 狀態持有者
class F {
  static Flavor? _appFlavor;

  static Flavor get appFlavor => _appFlavor!;

  /// 初始化 Flavor，只能呼叫一次（防止狀態被意外覆寫）
  static void init(Flavor flavor) {
    assert(_appFlavor == null, 'F.init() 已經呼叫過，不允許重複初始化');
    _appFlavor = flavor;
  }

  /// 是否已初始化（用於判斷是否由入口點設定）
  static bool get isInitialized => _appFlavor != null;

  /// App 顯示名稱（用於 title bar / app icon badge）
  static String get title {
    switch (appFlavor) {
      case Flavor.dev:
        return 'Hawkr Dev';
      case Flavor.stage:
        return 'Hawkr Stage';
      case Flavor.prod:
        return 'Hawkr';
    }
  }

  /// 是否顯示 FCM Debug 浮動按鈕（dev 環境才開）
  static bool get showFcmDebugOverlay => _appFlavor == Flavor.dev;
}

// ============================================================
// Flavor Extension — 買家端
//
// 所有 flavor-dependent 的設定都集中在此 extension，
// 是 config 唯一的 source of truth。
// ============================================================

extension FlavorExtension on Flavor {
  /// 環境顯示名稱
  String get displayName {
    switch (this) {
      case Flavor.dev:
        return '開發環境';
      case Flavor.stage:
        return '預發布環境';
      case Flavor.prod:
        return '生產環境';
    }
  }

  /// Gateway 主機地址：`--dart-define=GATEWAY_HOST=...`；dev 沒有提供時連本機單體。
  /// stage 與 prod 是必要設定，沒有提供時拋出 [StateError]（見 [resolveGatewayHost]）。
  String get gatewayHost => resolveGatewayHost(this, _gatewayHostDefine);

  /// 啟動前檢查必要設定；缺少時拋出 [StateError]，訊息指出缺的 define。
  void requireCoreConfig() => resolveGatewayHost(this, _gatewayHostDefine);

  /// API Base URL（帶 /api/v1 前綴，對齊 OpenAPI spec server URL）
  String get apiBaseUrl => '$gatewayHost/api/v1';

  /// media 域名基底：`--dart-define=MEDIA_BASE_HOST=...`（49 §8.5 Image Gateway）；
  /// dev 沒有提供時與 [gatewayHost] 同主機，stage 與 prod 沒有預設值。圖片 URL 組裝走
  /// buildImageUrl（見 lib/config/media_image.dart）。
  String get mediaBaseHost {
    if (_mediaBaseHostDefine.isNotEmpty) return _mediaBaseHostDefine;
    return this == Flavor.dev ? _localGatewayHost : '';
  }

  /// API 超時時間
  Duration get apiTimeout {
    switch (this) {
      case Flavor.dev:
        return const Duration(seconds: 30);
      case Flavor.stage:
        return const Duration(seconds: 20);
      case Flavor.prod:
        return const Duration(seconds: 15);
    }
  }

  /// 是否啟用 API 請求日誌
  bool get enableApiLogging => this != Flavor.prod;

  /// 是否顯示詳細錯誤信息
  bool get enableDetailedErrors => this != Flavor.prod;

  /// Apple Pay merchant identifier
  ///
  /// 金流走 PAYUni，測試環境與正式環境各自綁不同的 Payment Processing
  /// Certificate，因此 dev/stage 必須用 `.sandbox` merchant id，付款 token 才會
  /// 用 PAYUni 測試環境的憑證加密、被導去測試環境（不碰真錢）。
  ///
  /// 注意：此 id 必須同時出現在對應 flavor 的已簽章 entitlements
  /// （dev → `RunnerDev.entitlements`）並掛在該 App ID 上，否則 Apple Pay
  /// 面板會直接失敗。
  String get applePayMerchantId {
    switch (this) {
      case Flavor.dev:
      case Flavor.stage:
        return 'merchant.io.github.will413028.hawkr.sandbox';
      case Flavor.prod:
        return 'merchant.io.github.will413028.hawkr';
    }
  }

  /// Firebase 配置；沒有提供 define 時為 `null`，Firebase 整體停用。
  FirebaseOptions? get firebaseOptions => definedFirebaseOptions;

  /// Firebase 是否已設定。
  bool get firebaseConfigured => firebaseOptions != null;

  /// Firebase App 名稱
  String get firebaseAppName => 'buyer-$name';

  /// Sentry DSN：`--dart-define=SENTRY_DSN=...`；沒有提供為空字串，
  /// SentryFlutter.init 不送任何事件。
  String get sentryDsn => _sentryDsnDefine;
}
