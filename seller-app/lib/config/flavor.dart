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

abstract final class F {
  static Flavor? _appFlavor;

  static Flavor get appFlavor => _appFlavor!;

  static void init(Flavor flavor) {
    assert(_appFlavor == null, 'F.init() 已經呼叫過，不允許重複初始化');
    _appFlavor = flavor;
  }

  static bool get isInitialized => _appFlavor != null;

  static String get name => appFlavor.name;

  static String get title {
    switch (appFlavor) {
      case Flavor.dev:
        return 'Hawkr Seller Dev';
      case Flavor.stage:
        return 'Hawkr Seller Stage';
      case Flavor.prod:
        return 'Hawkr Seller';
    }
  }
}

/// Flavor Extension
///
/// 提供每個 flavor 對應的環境配置
extension FlavorExtension on Flavor {
  /// 獲取環境顯示名稱
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

  /// API Base URL (帶 /api/v1 前綴)
  String get apiBaseUrl => '$gatewayHost/api/v1';

  /// App: 需要登入
  String get appApi => '$apiBaseUrl/app';

  /// media 域名基底：`--dart-define=MEDIA_BASE_HOST=...`（49 §8.5 Image Gateway）；
  /// dev 沒有提供時與 [gatewayHost] 同主機，stage 與 prod 沒有預設值。圖片 URL 組裝走
  /// buildImageUrl（見 lib/config/media_image.dart）。
  String get mediaBaseHost {
    if (_mediaBaseHostDefine.isNotEmpty) return _mediaBaseHostDefine;
    return this == Flavor.dev ? _localGatewayHost : '';
  }

  /// 是否啟用 API 日誌
  bool get enableApiLogging => this != Flavor.prod;

  /// 是否啟用詳細錯誤信息
  bool get enableDetailedErrors => this != Flavor.prod;

  /// 是否為開發類環境
  bool get isDevEnvironment => this == Flavor.dev;

  /// API 超時時間（毫秒）
  int get apiTimeout {
    switch (this) {
      case Flavor.dev:
        return 30000; // 開發環境 30 秒
      case Flavor.stage:
        return 20000; // 預發布 20 秒
      case Flavor.prod:
        return 15000; // 生產環境 15 秒
    }
  }

  /// 獲取環境信息字符串（用於調試）
  String get environmentInfo {
    return '''
Flavor: $name
Display Name: $displayName
Gateway: $gatewayHost
API Base: $apiBaseUrl
API Logging: $enableApiLogging
API Timeout: ${apiTimeout}ms
''';
  }

  /// Firebase 配置；沒有提供 define 時為 `null`，Firebase 整體停用。
  FirebaseOptions? get firebaseOptions => definedFirebaseOptions;

  /// Firebase 是否已設定。
  bool get firebaseConfigured => firebaseOptions != null;

  /// 獲取對應環境的 Firebase App 名稱
  /// 用於 Firebase.initializeApp() 和 Firebase.app() 的 name 參數
  String get firebaseAppName => 'seller-$name';

  /// Sentry DSN：`--dart-define=SENTRY_DSN=...`；沒有提供為空字串，
  /// SentryFlutter.init 不送任何事件。
  String get sentryDsn => _sentryDsnDefine;
}
